// Command vwp-api is the Cloud Run backend for vwp.ac. It wraps the sdk/go
// verifier behind a small HTTP API: upload a .vwp and get its validation
// report, inspect a manifest, and browse pre-verified example packages.
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

//go:embed examples/*.vwp
var examplesFS embed.FS

// maxUpload caps request bodies so a hostile upload can't exhaust memory.
// .vwp packages are small; 25 MiB is generous.
const maxUpload = 25 << 20

// Caps on inlined part content in verify responses, so one oversized exhibit
// can't balloon the JSON payload.
const (
	maxRenderingBytes = 200 << 10
	maxCsvBytes       = 100 << 10
)

// example describes a pre-loaded package the marketing site can link to.
type example struct {
	Id          string `json:"id"`
	File        string `json:"-"` // internal embed path, not exposed
	Title       string `json:"title"`
	Description string `json:"description"`
}

var examples = []example{
	{Id: "asc606-revenue", File: "examples/acme-asc606.vwp", Title: "ASC 606 Revenue Recognition", Description: "A five-step revenue workpaper with an SSP allocation that ties out and a distinctness judgment left in review."},
	{Id: "asc820-fair-value", File: "examples/asc820-fair-value.vwp", Title: "ASC 820 Fair Value", Description: "A level 1 exchange-priced position and a level 3 restricted token with a marketability discount."},
	{Id: "wallet-ownership", File: "examples/wallet-ownership.vwp", Title: "Digital Asset Wallet Ownership", Description: "Proof-of-control evidence, entity mapping, and balance tie-outs for a treasury entity."},
	{Id: "dtcc-milestone", File: "examples/dtcc-milestone.vwp", Title: "DTCC / Canton Milestone Consideration", Description: "Noncash milestone consideration measured at each Accountability Committee approval; consumes a linked ASC 820 fair value package."},
	{Id: "canton-coin-fv", File: "examples/canton-coin-fv.vwp", Title: "Canton Coin Fair Value", Description: "Per-coin fair value at signing from thin OTC evidence; rejects the network fee price as a non-exit price."},
}

func exampleById(id string) (example, bool) {
	for _, e := range examples {
		if e.Id == id {
			return e, true
		}
	}
	return example{}, false
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/verify", handleVerify)
	mux.HandleFunc("/inspect", handleInspect)
	mux.HandleFunc("/examples", handleExamples)
	mux.HandleFunc("/examples/", handleExampleVerify) // /examples/{id}/verify

	handler := withCORS(withLogging(mux))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("vwp-api listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// --- handlers ---

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "vwp-api"})
}

// handleVerify accepts a .vwp via multipart form field "package" (or a raw
// body) and returns the full verification report.
func handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	data, origin, err := readPackage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	report, err := verifyBytes(data, origin)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("could not open package: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// handleInspect returns just the manifest summary for a package.
func handleInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	data, origin, err := readPackage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pkg, err := vwp.OpenPackageBytes(data, origin)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("could not open package: %v", err))
		return
	}
	raw, err := pkg.Read(vwp.ManifestPath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "package has no manifest.json")
		return
	}
	manifest, err := vwp.ParseManifest(raw)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("could not parse manifest: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, manifest)
}

func handleExamples(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"examples": examples})
}

// handleExampleVerify serves GET /examples/{id}/verify — the report for a
// bundled example — and GET /examples/{id}/download — the raw .vwp bytes.
func handleExampleVerify(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/examples/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "expected /examples/{id}/verify or /examples/{id}/download")
		return
	}
	id, action := parts[0], parts[1]
	ex, ok := exampleById(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown example %q", id))
		return
	}
	data, err := examplesFS.ReadFile(ex.File)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "example unreadable")
		return
	}
	switch action {
	case "verify":
		report, err := verifyBytes(data, ex.Id+".vwp")
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("example failed to open: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"example": ex, "report": report})
	case "download":
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+".vwp"))
		w.Write(data)
	default:
		writeError(w, http.StatusNotFound, "expected action 'verify' or 'download'")
	}
}

// --- helpers ---

// verifyResponse is a verification report plus the package's human-readable
// chain — facts, claims, judgments, calculations, and the workproduct
// conclusion — so callers can show what was asserted, not just whether it
// checks out. Report fields marshal at the top level for compatibility.
type verifyResponse struct {
	*vwp.Report
	Components map[string]any `json:"components,omitempty"`
}

func verifyBytes(data []byte, origin string) (*verifyResponse, error) {
	pkg, err := vwp.OpenPackageBytes(data, origin)
	if err != nil {
		return nil, err
	}
	return &verifyResponse{
		Report:     validators.Verify(pkg),
		Components: extractComponents(pkg),
	}, nil
}

// extractComponents reads the chain parts out of a package by manifest role.
// Extraction is best-effort: a part that is missing or malformed is skipped
// rather than failing the response — the validators report those problems.
func extractComponents(pkg vwp.Source) map[string]any {
	raw, err := pkg.Read(vwp.ManifestPath)
	if err != nil {
		return nil
	}
	manifest, err := vwp.ParseManifest(raw)
	if err != nil {
		return nil
	}

	out := map[string]any{}
	var calcs, registers, renderings []map[string]any
	for _, part := range manifest.AllPartRefs() {
		switch part.Role {
		case "evidence-index":
			var doc map[string]any
			if vwp.ReadJSON(pkg, part.Path, &doc) != nil {
				continue
			}
			if items, ok := doc["evidence"].([]any); ok {
				prev, _ := out["evidence"].([]any)
				out["evidence"] = append(prev, items...)
			}
		case "provenance":
			var doc map[string]any
			if vwp.ReadJSON(pkg, part.Path, &doc) == nil {
				out["provenance"] = doc
			}
		case "readable", "rendering":
			if !strings.EqualFold(path.Ext(part.Path), ".md") {
				continue
			}
			raw, err := pkg.Read(part.Path)
			if err != nil || len(raw) > maxRenderingBytes {
				continue
			}
			renderings = append(renderings, map[string]any{"path": part.Path, "markdown": string(raw)})
		case "facts", "claims", "judgments":
			// Each of these parts is a JSON document keyed by its role,
			// e.g. facts/facts.json is {"facts": [...]}. A part may also
			// carry domain registers under other keys (measurements,
			// wallets, ...) — surface those as named registers.
			var doc map[string]any
			if vwp.ReadJSON(pkg, part.Path, &doc) != nil {
				continue
			}
			for key, v := range doc {
				items, ok := v.([]any)
				if !ok || len(items) == 0 {
					continue
				}
				if key == part.Role {
					prev, _ := out[part.Role].([]any)
					out[part.Role] = append(prev, items...)
					continue
				}
				registers = append(registers, map[string]any{"name": key, "path": part.Path, "items": items})
			}
		case "calculation":
			calc := map[string]any{"path": part.Path, "mediaType": part.MediaType}
			if part.MediaType == "application/json" {
				var doc map[string]any
				if vwp.ReadJSON(pkg, part.Path, &doc) == nil {
					doc["path"] = part.Path
					calc = doc
				}
			} else if part.MediaType == "text/csv" {
				// Schedules ship as CSV exhibits; include the raw text so
				// the report can render the table itself.
				if raw, err := pkg.Read(part.Path); err == nil && len(raw) <= maxCsvBytes {
					calc["csv"] = string(raw)
				}
			}
			calcs = append(calcs, calc)
		case "workproduct":
			var doc map[string]any
			if vwp.ReadJSON(pkg, part.Path, &doc) == nil {
				out["workproduct"] = doc
			}
		}
	}
	if len(calcs) > 0 {
		out["calculations"] = calcs
	}
	if len(registers) > 0 {
		out["registers"] = registers
	}
	if len(renderings) > 0 {
		out["renderings"] = renderings
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// readPackage extracts package bytes from a multipart "package" field, or falls
// back to the raw request body (for simple curl / fetch uploads).
func readPackage(r *http.Request) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxUpload)
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUpload); err != nil {
			return nil, "", fmt.Errorf("invalid multipart form: %w", err)
		}
		file, header, err := r.FormFile("package")
		if err != nil {
			return nil, "", errors.New("missing form field 'package' (the .vwp file)")
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, "", fmt.Errorf("could not read upload: %w", err)
		}
		return data, header.Filename, nil
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "", fmt.Errorf("could not read body: %w", err)
	}
	if len(data) == 0 {
		return nil, "", errors.New("empty request: POST a .vwp as multipart field 'package' or as the raw body")
	}
	return data, "upload.vwp", nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- middleware ---

// withCORS allows the vwp.ac frontend (and local dev) to call the API from the
// browser. Origins are allowlisted; VWP_ALLOWED_ORIGINS overrides the default.
func withCORS(next http.Handler) http.Handler {
	allowed := map[string]bool{
		"https://vwp.ac":         true,
		"https://www.vwp.ac":     true,
		"http://localhost:5000":  true,
		"http://localhost:8080":  true,
		"http://127.0.0.1:5000":  true,
	}
	if env := os.Getenv("VWP_ALLOWED_ORIGINS"); env != "" {
		for _, o := range strings.Split(env, ",") {
			allowed[strings.TrimSpace(o)] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
