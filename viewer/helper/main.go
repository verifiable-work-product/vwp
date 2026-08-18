// vwp-viewer-helper opens and verifies an untrusted .vwp package, extracts
// only its selected readable and primary artifact to a caller-owned temporary
// directory, and writes a JSON view model for the native desktop viewer.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

const (
	rolePrimaryArtifact = "primary-artifact"
	roleArtifact        = "artifact"
	roleReadable        = "readable"
	roleRendering       = "rendering"
	maxInlineTextBytes  = 5 << 20
)

type embeddedFile struct {
	Path          string `json:"path"`
	DisplayName   string `json:"displayName"`
	MediaType     string `json:"mediaType"`
	Role          string `json:"role"`
	ExtractedPath string `json:"extractedPath,omitempty"`
	Content       string `json:"content,omitempty"`
}

type partPayload struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Data      any    `json:"data,omitempty"`
	Content   string `json:"content,omitempty"`
}

type document struct {
	SourcePath      string         `json:"sourcePath"`
	Title           string         `json:"title"`
	Manifest        any            `json:"manifest"`
	Workproduct     any            `json:"workproduct"`
	Readable        *embeddedFile  `json:"readable,omitempty"`
	PrimaryArtifact *embeddedFile  `json:"primaryArtifact,omitempty"`
	Artifacts       []embeddedFile `json:"artifacts"`
	Renderings      []embeddedFile `json:"renderings"`
	Facts           []any          `json:"facts"`
	Claims          []any          `json:"claims"`
	Judgments       []any          `json:"judgments"`
	Calculations    []partPayload  `json:"calculations"`
	Evidence        []any          `json:"evidence"`
	Approvals       []any          `json:"approvals"`
	Attestations    []partPayload  `json:"attestations"`
	Provenance      any            `json:"provenance"`
	Verification    *vwp.Report    `json:"verification"`
}

func main() {
	if len(os.Args) != 4 || os.Args[1] != "inspect" {
		fmt.Fprintln(os.Stderr, "usage: vwp-viewer-helper inspect <package.vwp> <extraction-directory>")
		os.Exit(2)
	}
	doc, err := inspectPackage(os.Args[2], os.Args[3])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func inspectPackage(packagePath, extractionDir string) (*document, error) {
	pkg, err := vwp.OpenPackage(packagePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(extractionDir, 0o700); err != nil {
		return nil, fmt.Errorf("create preview directory: %w", err)
	}

	manifestBytes, err := pkg.Read(vwp.ManifestPath)
	if err != nil {
		return nil, err
	}
	manifest, err := vwp.ParseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	var manifestValue any
	if err := json.Unmarshal(manifestBytes, &manifestValue); err != nil {
		return nil, err
	}
	workproduct := readJSONValue(pkg, vwp.WorkproductPath)
	title := stringField(workproduct, "title")
	if title == "" {
		title = manifest.Title
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(packagePath), filepath.Ext(packagePath))
	}

	doc := &document{
		SourcePath:   packagePath,
		Title:        title,
		Manifest:     manifestValue,
		Workproduct:  workproduct,
		Facts:        readCollection(pkg, vwp.FactsPath, "facts"),
		Claims:       readCollection(pkg, vwp.ClaimsPath, "claims"),
		Judgments:    readCollection(pkg, vwp.JudgmentsPath, "judgments"),
		Evidence:     readCollection(pkg, vwp.EvidenceIndex, "evidence"),
		Approvals:    readCollection(pkg, vwp.ApprovalsPath, "approvals"),
		Provenance:   readJSONValue(pkg, vwp.ProvenancePath),
		Verification: validators.Verify(pkg),
		Artifacts:    []embeddedFile{},
		Renderings:   []embeddedFile{},
		Calculations: []partPayload{},
		Attestations: []partPayload{},
	}

	var primaryRef *vwp.PartRef
	var readableRef *vwp.PartRef
	for _, part := range manifest.AllPartRefs() {
		switch part.Role {
		case rolePrimaryArtifact:
			ref := part
			primaryRef = &ref
			doc.Artifacts = append(doc.Artifacts, fileMetadata(part))
		case roleArtifact:
			doc.Artifacts = append(doc.Artifacts, fileMetadata(part))
		case roleReadable, roleRendering:
			item := fileMetadata(part)
			if raw, err := pkg.Read(part.Path); err == nil && isTextMediaType(part.MediaType) && len(raw) <= maxInlineTextBytes {
				item.Content = string(raw)
			}
			doc.Renderings = append(doc.Renderings, item)
			if part.Role == roleReadable {
				ref := part
				readableRef = &ref
			}
		case "calculation":
			doc.Calculations = append(doc.Calculations, readPartPayload(pkg, part))
		case "attestation":
			doc.Attestations = append(doc.Attestations, readPartPayload(pkg, part))
		}
	}

	// Be liberal with early packages that used the directory convention but
	// omitted role hints.
	if len(doc.Renderings) == 0 {
		for _, part := range manifest.AllPartRefs() {
			if strings.HasPrefix(part.Path, "renderings/") {
				item := fileMetadata(part)
				if raw, err := pkg.Read(part.Path); err == nil && isTextMediaType(part.MediaType) && len(raw) <= maxInlineTextBytes {
					item.Content = string(raw)
				}
				doc.Renderings = append(doc.Renderings, item)
			}
		}
	}

	if readableRef == nil {
		readableRef = preferredRendering(manifest.AllPartRefs())
	}
	// A few experimental v0.1 packages contain only a primary artifact. Keep
	// them readable until v0.2's required canonical Markdown part is standard.
	if readableRef == nil {
		readableRef = primaryRef
	}
	if primaryRef != nil {
		primary, err := extractPreview(pkg, *primaryRef, extractionDir, "primary")
		if err != nil {
			return nil, err
		}
		doc.PrimaryArtifact = primary
	}
	if readableRef != nil {
		if primaryRef != nil && readableRef.Path == primaryRef.Path {
			doc.Readable = doc.PrimaryArtifact
		} else {
			readable, err := extractPreview(pkg, *readableRef, extractionDir, "readable")
			if err != nil {
				return nil, err
			}
			doc.Readable = readable
		}
	}

	sort.Slice(doc.Artifacts, func(i, j int) bool { return doc.Artifacts[i].Path < doc.Artifacts[j].Path })
	sort.Slice(doc.Renderings, func(i, j int) bool { return doc.Renderings[i].Path < doc.Renderings[j].Path })
	return doc, nil
}

func readJSONValue(pkg vwp.Source, relPath string) any {
	var value any
	if err := vwp.ReadJSON(pkg, relPath, &value); err != nil {
		return nil
	}
	return value
}

func readCollection(pkg vwp.Source, relPath, key string) []any {
	value := readJSONValue(pkg, relPath)
	object, _ := value.(map[string]any)
	items, _ := object[key].([]any)
	if items == nil {
		return []any{}
	}
	return items
}

func readPartPayload(pkg vwp.Source, part vwp.PartRef) partPayload {
	item := partPayload{Path: part.Path, MediaType: part.MediaType}
	raw, err := pkg.Read(part.Path)
	if err != nil {
		return item
	}
	if part.MediaType == "application/json" {
		_ = json.Unmarshal(raw, &item.Data)
	} else if isTextMediaType(part.MediaType) && len(raw) <= maxInlineTextBytes {
		item.Content = string(raw)
	}
	return item
}

func fileMetadata(part vwp.PartRef) embeddedFile {
	return embeddedFile{
		Path:        part.Path,
		DisplayName: path.Base(part.Path),
		MediaType:   part.MediaType,
		Role:        part.Role,
	}
}

func preferredRendering(parts []vwp.PartRef) *vwp.PartRef {
	preference := map[string]int{
		"text/markdown":   0,
		"application/pdf": 1,
		"text/html":       2,
		"text/plain":      3,
		"text/csv":        4,
	}
	candidates := make([]vwp.PartRef, 0)
	for _, part := range parts {
		if part.Role == roleReadable || part.Role == roleRendering || strings.HasPrefix(part.Path, "renderings/") {
			candidates = append(candidates, part)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, lok := preference[candidates[i].MediaType]
		right, rok := preference[candidates[j].MediaType]
		if !lok {
			left = 100
		}
		if !rok {
			right = 100
		}
		if left != right {
			return left < right
		}
		return candidates[i].Path < candidates[j].Path
	})
	if len(candidates) == 0 {
		return nil
	}
	return &candidates[0]
}

func extractPreview(pkg vwp.Source, part vwp.PartRef, extractionDir, prefix string) (*embeddedFile, error) {
	raw, err := pkg.Read(part.Path)
	if err != nil {
		return nil, err
	}
	name := path.Base(part.Path)
	if name == "." || name == "/" || name == "" {
		return nil, errors.New("embedded file has no safe file name")
	}
	outPath := filepath.Join(extractionDir, safePreviewName(name, prefix))
	file, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("preview file already exists: %s", outPath)
		}
		return nil, err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	item := fileMetadata(part)
	item.ExtractedPath = outPath
	if isTextMediaType(part.MediaType) && len(raw) <= maxInlineTextBytes {
		item.Content = string(raw)
	}
	return &item, nil
}

var safeExtension = regexp.MustCompile(`^\.[A-Za-z0-9]{1,12}$`)

func safePreviewName(original, prefix string) string {
	extension := path.Ext(original)
	if !safeExtension.MatchString(extension) {
		extension = ""
	}
	return prefix + strings.ToLower(extension)
}

func isTextMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "text/") || mediaType == "application/json"
}

func stringField(value any, key string) string {
	object, _ := value.(map[string]any)
	text, _ := object[key].(string)
	return text
}
