// Package builder incrementally assembles a VWP package as an unpacked
// directory. Every operation validates the object against its JSON schema
// before writing, re-hashes touched parts, and rewrites the manifest, so the
// package on disk is always integrity-consistent.
//
// Operations are idempotent by caller-supplied object id: re-adding an
// identical object is a no-op success, while re-using an id with different
// content returns ErrConflict. This makes the operations safe to retry, e.g.
// as Temporal activities.
package builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// ErrConflict is returned when an operation re-uses an existing object id
// (or part path) with different content.
var ErrConflict = errors.New("vwp/builder: id already exists with different content")

// Builder edits an unpacked VWP package directory.
type Builder struct {
	dir      string
	manifest *vwp.Manifest
}

// Part roles used in manifest part references.
const (
	RoleWorkproduct     = "workproduct"
	RoleProvenance      = "provenance"
	RoleClaims          = "claims"
	RoleEvidenceIndex   = "evidence-index"
	RoleEvidence        = "evidence"
	RoleFacts           = "facts"
	RoleJudgments       = "judgments"
	RoleCalculation     = "calculation"
	RoleValidations     = "validations"
	RoleApprovals       = "approvals"
	RoleAttestation     = "attestation"
	RoleRendering       = "rendering"
	RoleReadable        = "readable"
	RolePrimaryArtifact = "primary-artifact"
)

// ScaffoldConfig describes the package identity for Scaffold.
type ScaffoldConfig struct {
	PackageId string
	// CreatedAt is an RFC 3339 timestamp. Leave empty to use the current
	// time; supply it explicitly when Scaffold must be retry-idempotent.
	CreatedAt   string
	Profile     string
	Extension   string
	Title       string
	Subject     map[string]any
	Workproduct vwp.Workproduct
	Provenance  vwp.Provenance
}

// Scaffold creates the minimum valid package in dir: manifest.json,
// workproduct.json, an empty claims/claims.json, and
// provenance/provenance.json. If dir already contains an identical
// scaffolded package the call is a no-op; a different existing package
// returns ErrConflict.
func Scaffold(dir string, cfg ScaffoldConfig) (*Builder, error) {
	if cfg.CreatedAt == "" {
		cfg.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if cfg.Workproduct.Profile == "" {
		cfg.Workproduct.Profile = cfg.Profile
	}
	if cfg.Workproduct.Extension == "" {
		cfg.Workproduct.Extension = cfg.Extension
	}
	if err := validateObject(schema.Workproduct, cfg.Workproduct); err != nil {
		return nil, err
	}
	if err := validateObject(schema.Provenance, cfg.Provenance); err != nil {
		return nil, err
	}

	workproductBytes, err := vwp.EncodeJSON(cfg.Workproduct)
	if err != nil {
		return nil, err
	}
	provenanceBytes, err := vwp.EncodeJSON(cfg.Provenance)
	if err != nil {
		return nil, err
	}
	claimsBytes, err := vwp.EncodeJSON(map[string]any{"claims": []any{}})
	if err != nil {
		return nil, err
	}

	manifest := &vwp.Manifest{
		VwpVersion: vwp.VwpVersion,
		PackageId:  cfg.PackageId,
		CreatedAt:  cfg.CreatedAt,
		Profile:    cfg.Profile,
		Extension:  cfg.Extension,
		Title:      cfg.Title,
		Subject:    cfg.Subject,
		Workproduct: &vwp.PartRef{
			Path: vwp.WorkproductPath, MediaType: "application/json",
			Role: RoleWorkproduct, SHA256: vwp.SHA256Hex(workproductBytes),
		},
		Provenance: &vwp.PartRef{
			Path: vwp.ProvenancePath, MediaType: "application/json",
			Role: RoleProvenance, SHA256: vwp.SHA256Hex(provenanceBytes),
		},
		Parts: []vwp.PartRef{{
			Path: vwp.ClaimsPath, MediaType: "application/json",
			Role: RoleClaims, SHA256: vwp.SHA256Hex(claimsBytes),
		}},
	}
	if err := validateObject(schema.Manifest, manifest); err != nil {
		return nil, err
	}

	if _, err := os.Stat(filepath.Join(dir, vwp.ManifestPath)); err == nil {
		return reopenScaffold(dir, manifest, cfg)
	}

	b := &Builder{dir: dir, manifest: manifest}
	if err := b.writePart(vwp.WorkproductPath, workproductBytes); err != nil {
		return nil, err
	}
	if err := b.writePart(vwp.ProvenancePath, provenanceBytes); err != nil {
		return nil, err
	}
	if err := b.writePart(vwp.ClaimsPath, claimsBytes); err != nil {
		return nil, err
	}
	if err := b.saveManifest(); err != nil {
		return nil, err
	}
	return b, nil
}

// reopenScaffold handles Scaffold on a directory that already has a
// manifest: identical identity fields make it an idempotent reopen,
// anything else is a conflict.
func reopenScaffold(dir string, want *vwp.Manifest, cfg ScaffoldConfig) (*Builder, error) {
	b, err := Open(dir)
	if err != nil {
		return nil, err
	}
	got := b.manifest
	same := got.PackageId == want.PackageId &&
		got.CreatedAt == want.CreatedAt &&
		got.Profile == want.Profile &&
		got.Extension == want.Extension &&
		got.Title == want.Title &&
		jsonEqual(got.Subject, want.Subject)
	if !same {
		return nil, fmt.Errorf("%w: scaffold of %s with different package identity", ErrConflict, dir)
	}
	var existingWp any
	if err := readJSONFile(b.path(vwp.WorkproductPath), &existingWp); err != nil {
		return nil, err
	}
	if !jsonEqual(existingWp, cfg.Workproduct) {
		return nil, fmt.Errorf("%w: scaffold workproduct differs for %s", ErrConflict, cfg.PackageId)
	}
	return b, nil
}

// SetIdentity renames the package and records an extension it was scaffolded
// without, in both the manifest and workproduct.json. It is how a package
// started before its template was chosen takes on the template's identity.
// An empty title keeps the current one; an extension can be added but never
// swapped for another (it decides which validators define done), so a
// different existing extension is ErrConflict.
func (b *Builder) SetIdentity(title, extension string) error {
	if extension != "" && b.manifest.Extension != "" && b.manifest.Extension != extension {
		return fmt.Errorf("%w: package extension is %s, not %s", ErrConflict, b.manifest.Extension, extension)
	}
	var wp vwp.Workproduct
	if err := readJSONFile(b.path(vwp.WorkproductPath), &wp); err != nil {
		return err
	}
	if title != "" {
		b.manifest.Title = title
		wp.Title = title
	}
	if extension != "" {
		b.manifest.Extension = extension
		wp.Extension = extension
	}
	if err := validateObject(schema.Workproduct, wp); err != nil {
		return err
	}
	data, err := vwp.EncodeJSON(wp)
	if err != nil {
		return err
	}
	if err := b.writePart(vwp.WorkproductPath, data); err != nil {
		return err
	}
	b.upsertManifestPart(vwp.WorkproductPath, "application/json", RoleWorkproduct, vwp.SHA256Hex(data))
	return b.saveManifest()
}

// Open opens an existing unpacked package directory for editing. The
// directory must contain a parseable manifest.json.
func Open(dir string) (*Builder, error) {
	raw, err := os.ReadFile(filepath.Join(dir, vwp.ManifestPath))
	if err != nil {
		return nil, fmt.Errorf("vwp/builder: open %s: %w", dir, err)
	}
	manifest, err := vwp.ParseManifest(raw)
	if err != nil {
		return nil, err
	}
	return &Builder{dir: dir, manifest: manifest}, nil
}

// Dir returns the package directory being built.
func (b *Builder) Dir() string { return b.dir }

// Manifest returns a copy of the current manifest.
func (b *Builder) Manifest() vwp.Manifest {
	m := *b.manifest
	return m
}

// AddEvidence records an evidence object in evidence/evidence.json and, when
// content is non-nil, writes the evidence file at ev.Path (computing its
// manifest SHA-256). Pass nil content for external-reference evidence.
func (b *Builder) AddEvidence(ev vwp.Evidence, content []byte) error {
	if content != nil && ev.Path == "" {
		return fmt.Errorf("vwp/builder: evidence %s: content given but Path is empty", ev.Id)
	}
	if ev.Path != "" && vwp.IsUnsafePath(ev.Path) {
		return fmt.Errorf("vwp/builder: evidence %s: unsafe path %s", ev.Id, ev.Path)
	}
	if err := validateObject(schema.EvidenceSchema, ev); err != nil {
		return err
	}
	if content != nil {
		if err := b.putFilePart(ev.Path, mediaTypeFor(ev.Path), RoleEvidence, content); err != nil {
			return err
		}
	}
	return b.addCollectionObject(vwp.EvidenceIndex, "evidence", RoleEvidenceIndex, ev.Id, ev)
}

// AddEvidenceFromFile is AddEvidence with the file content read from srcPath
// (outside the package).
func (b *Builder) AddEvidenceFromFile(ev vwp.Evidence, srcPath string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("vwp/builder: evidence %s: %w", ev.Id, err)
	}
	return b.AddEvidence(ev, content)
}

// AddFact appends a fact to facts/facts.json.
func (b *Builder) AddFact(f vwp.Fact) error {
	if err := validateObject(schema.Fact, f); err != nil {
		return err
	}
	return b.addCollectionObject(vwp.FactsPath, "facts", RoleFacts, f.Id, f)
}

// AddClaim appends a claim to claims/claims.json.
func (b *Builder) AddClaim(c vwp.Claim) error {
	if err := validateObject(schema.Claim, c); err != nil {
		return err
	}
	return b.addCollectionObject(vwp.ClaimsPath, "claims", RoleClaims, c.Id, c)
}

// AddJudgment appends a judgment to judgments/judgments.json.
func (b *Builder) AddJudgment(j vwp.Judgment) error {
	if err := validateObject(schema.Judgment, j); err != nil {
		return err
	}
	return b.addCollectionObject(vwp.JudgmentsPath, "judgments", RoleJudgments, j.Id, j)
}

// RecordApproval appends an approval to approvals/approvals.json.
func (b *Builder) RecordApproval(a vwp.Approval) error {
	if err := validateObject(schema.Approval, a); err != nil {
		return err
	}
	return b.addCollectionObject(vwp.ApprovalsPath, "approvals", RoleApprovals, a.Id, a)
}

// AddValidation appends a shipped validation result to
// validations/validation-report.json.
func (b *Builder) AddValidation(v vwp.Validation) error {
	if err := validateObject(schema.Validation, v); err != nil {
		return err
	}
	return b.addCollectionObject(vwp.ValidationsPath, "validations", RoleValidations, v.Id, v)
}

// CalculationOption configures AddCalculation.
type CalculationOption func(*calcConfig)

type calcConfig struct {
	path    string
	replace bool
}

// WithCalculationPath overrides the calculation part path (default
// calculations/<id>.json).
func WithCalculationPath(relPath string) CalculationOption {
	return func(c *calcConfig) { c.path = relPath }
}

// ReplaceExisting lets AddCalculation overwrite a calculation already stored
// at its path instead of returning ErrConflict: a corrected recompute
// supersedes the earlier attempt rather than sitting beside it.
func ReplaceExisting() CalculationOption {
	return func(c *calcConfig) { c.replace = true }
}

// AddCalculation writes a calculation as its own JSON part under
// calculations/.
func (b *Builder) AddCalculation(c vwp.Calculation, opts ...CalculationOption) error {
	cfg := calcConfig{path: "calculations/" + c.Id + ".json"}
	for _, o := range opts {
		o(&cfg)
	}
	if err := validateObject(schema.Calculation, c); err != nil {
		return err
	}
	data, err := vwp.EncodeJSON(c)
	if err != nil {
		return err
	}
	if cfg.replace {
		return b.ReplaceFilePart(cfg.path, "application/json", RoleCalculation, data)
	}
	return b.putFilePart(cfg.path, "application/json", RoleCalculation, data)
}

// AddAttestation writes an analysis attestation at attestations/<id>.json.
// The document must carry a string "id" and validate against the
// analysis-attestation schema.
func (b *Builder) AddAttestation(att map[string]any) error {
	id, _ := att["id"].(string)
	if id == "" {
		return fmt.Errorf("vwp/builder: attestation requires a string id")
	}
	if err := validateObject(schema.AnalysisAttestation, att); err != nil {
		return err
	}
	data, err := vwp.EncodeJSON(att)
	if err != nil {
		return err
	}
	return b.putFilePart("attestations/"+id+".json", "application/json", RoleAttestation, data)
}

// AddFilePart writes an opaque part (rendering, CSV schedule, signature,
// derived text layer) and indexes it in the manifest. Identical existing
// content is a no-op; different existing content is ErrConflict.
func (b *Builder) AddFilePart(relPath, mediaType, role string, content []byte) error {
	return b.putFilePart(relPath, mediaType, role, content)
}

// ReplaceFilePart writes an opaque part, overwriting any existing content at
// relPath, and indexes it in the manifest with the new digest.
func (b *Builder) ReplaceFilePart(relPath, mediaType, role string, content []byte) error {
	if vwp.IsUnsafePath(relPath) {
		return fmt.Errorf("vwp/builder: unsafe part path: %s", relPath)
	}
	if err := b.writePart(relPath, content); err != nil {
		return err
	}
	b.upsertManifestPart(relPath, mediaType, role, vwp.SHA256Hex(content))
	return b.saveManifest()
}

// AddPrimaryArtifact embeds the native file a human considers the work
// product (for example a PDF, spreadsheet, or document). A package has at
// most one primary artifact; derived renderings can still be added separately.
// The artifact lives under artifacts/ and is covered by the manifest hash.
func (b *Builder) AddPrimaryArtifact(relPath, mediaType string, content []byte) error {
	if !strings.HasPrefix(relPath, "artifacts/") || relPath == "artifacts/" {
		return fmt.Errorf("vwp/builder: primary artifact must be under artifacts/")
	}
	if mediaType == "" {
		return fmt.Errorf("vwp/builder: primary artifact requires a media type")
	}
	for _, part := range b.manifest.Parts {
		if part.Role == RolePrimaryArtifact && part.Path != relPath {
			return fmt.Errorf("%w: package already has primary artifact %s", ErrConflict, part.Path)
		}
	}
	return b.putFilePart(relPath, mediaType, RolePrimaryArtifact, content)
}

// registerNameRe bounds register names to one lower-case path segment.
var registerNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// AddRegister writes a structured fact register at facts/<name>.json — a
// typed collection of facts an extension's validators consume (the wallet
// population, the fair value measurement table). When the package's
// extension defines the named register the document must satisfy its
// extension schema; when the extension defines registers but not this
// name the write is refused so a typo never produces a part the validators
// silently ignore. Extensions without registers accept any object.
func (b *Builder) AddRegister(name string, doc any) error {
	name = strings.TrimSpace(name)
	if !registerNameRe.MatchString(name) {
		return fmt.Errorf("vwp/builder: invalid register name %q: use lower-case letters, digits and dashes", name)
	}
	value, err := toJSONValue(doc)
	if err != nil {
		return fmt.Errorf("vwp/builder: register %s: %w", name, err)
	}
	if _, isObject := value.(map[string]any); !isObject {
		return fmt.Errorf("vwp/builder: register %s: document must be a JSON object", name)
	}
	ext := b.manifest.Extension
	if schemaRef, ok := validators.RegisterSchema(ext, name); ok {
		errs, err := schema.ValidateExtension(schemaRef, value)
		if err != nil {
			return fmt.Errorf("vwp/builder: register %s: %w", name, err)
		}
		if len(errs) > 0 {
			if len(errs) > 8 {
				errs = append(errs[:8], fmt.Sprintf("… %d more", len(errs)-8))
			}
			return fmt.Errorf("vwp/builder: register %s does not satisfy the %s schema: %s", name, ext, strings.Join(errs, "; "))
		}
	} else if known := validators.RegisterNames(ext); len(known) > 0 {
		return fmt.Errorf("vwp/builder: extension %s defines no register named %q (known: %s)", ext, name, strings.Join(known, ", "))
	}
	data, err := vwp.EncodeJSON(value)
	if err != nil {
		return err
	}
	return b.putFilePart(validators.RegisterPath(name), "application/json", RoleFacts, data)
}

// SupportKind names a claim support link field.
type SupportKind string

// Claim support link kinds.
const (
	SupportFacts        SupportKind = "supportingFacts"
	SupportEvidence     SupportKind = "supportingEvidence"
	SupportCalculations SupportKind = "supportingCalculations"
	SupportJudgments    SupportKind = "judgments"
)

// LinkSupport adds support references to an existing claim. Referenced ids
// must already exist in the package; already-linked ids are skipped, so the
// operation is idempotent.
func (b *Builder) LinkSupport(claimId string, kind SupportKind, refIds ...string) error {
	switch kind {
	case SupportFacts, SupportEvidence, SupportCalculations, SupportJudgments:
	default:
		return fmt.Errorf("vwp/builder: unknown support kind %q", kind)
	}
	known, err := b.knownIdsFor(kind)
	if err != nil {
		return err
	}
	for _, ref := range refIds {
		if !known[ref] {
			return fmt.Errorf("vwp/builder: claim %s: unknown %s ref %s", claimId, kind, ref)
		}
	}

	doc, err := b.readCollectionDoc(vwp.ClaimsPath)
	if err != nil {
		return err
	}
	items, _ := doc["claims"].([]any)
	found := false
	changed := false
	for _, item := range items {
		claim, ok := item.(map[string]any)
		if !ok || claim["id"] != claimId {
			continue
		}
		found = true
		existing, _ := claim[string(kind)].([]any)
		for _, ref := range refIds {
			already := false
			for _, e := range existing {
				if e == ref {
					already = true
					break
				}
			}
			if !already {
				existing = append(existing, ref)
				changed = true
			}
		}
		claim[string(kind)] = existing
		if err := validateObject(schema.Claim, claim); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("vwp/builder: claim %s not found", claimId)
	}
	if !changed {
		return nil
	}
	return b.writeCollectionDoc(vwp.ClaimsPath, RoleClaims, doc)
}

// knownIdsFor collects the ids a support kind may reference.
func (b *Builder) knownIdsFor(kind SupportKind) (map[string]bool, error) {
	ids := map[string]bool{}
	collect := func(relPath, key string) error {
		doc, err := b.readCollectionDoc(relPath)
		if err != nil {
			return err
		}
		items, _ := doc[key].([]any)
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				if id, ok := m["id"].(string); ok {
					ids[id] = true
				}
			}
		}
		return nil
	}
	switch kind {
	case SupportFacts:
		return ids, collect(vwp.FactsPath, "facts")
	case SupportEvidence:
		return ids, collect(vwp.EvidenceIndex, "evidence")
	case SupportJudgments:
		return ids, collect(vwp.JudgmentsPath, "judgments")
	case SupportCalculations:
		entries, err := os.ReadDir(filepath.Join(b.dir, "calculations"))
		if err != nil {
			if os.IsNotExist(err) {
				return ids, nil
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			var doc map[string]any
			if err := readJSONFile(filepath.Join(b.dir, "calculations", entry.Name()), &doc); err != nil {
				continue
			}
			if id, ok := doc["id"].(string); ok {
				ids[id] = true
			}
		}
		return ids, nil
	}
	return ids, nil
}

// SetProvenance replaces provenance/provenance.json.
func (b *Builder) SetProvenance(p vwp.Provenance) error {
	if err := validateObject(schema.Provenance, p); err != nil {
		return err
	}
	data, err := vwp.EncodeJSON(p)
	if err != nil {
		return err
	}
	if err := b.writePart(vwp.ProvenancePath, data); err != nil {
		return err
	}
	if b.manifest.Provenance == nil {
		b.manifest.Provenance = &vwp.PartRef{Path: vwp.ProvenancePath, MediaType: "application/json", Role: RoleProvenance}
	}
	b.manifest.Provenance.SHA256 = vwp.SHA256Hex(data)
	return b.saveManifest()
}

// AddProvenanceModel appends a model record to provenance. An
// already-recorded identical entry is a no-op.
func (b *Builder) AddProvenanceModel(m vwp.ModelRef) error {
	var p vwp.Provenance
	if err := readJSONFile(b.path(vwp.ProvenancePath), &p); err != nil {
		return err
	}
	for _, existing := range p.Models {
		if existing == m {
			return nil
		}
	}
	p.Models = append(p.Models, m)
	return b.SetProvenance(p)
}

// RunValidators verifies the package (core, profile, and extension
// validators; extensions defaults to the built-in registry when nil) and
// ships the outcomes as validations/validation-report.json. It returns the
// verification report.
func (b *Builder) RunValidators(extensions map[string]vwp.ExtensionValidator) (*vwp.Report, error) {
	if extensions == nil {
		extensions = validators.Registry()
	}
	pkg, err := vwp.OpenPackage(b.dir)
	if err != nil {
		return nil, err
	}
	report := vwp.Verify(pkg, vwp.WithExtensionValidators(extensions))
	shipped := make([]vwp.Validation, 0, len(report.Results))
	for i, r := range report.Results {
		shipped = append(shipped, vwp.Validation{
			Id:        fmt.Sprintf("v_%03d", i+1),
			Validator: r.ValidatorId,
			Scope:     "package",
			Status:    r.Status,
			Message:   r.Message,
		})
	}
	doc := map[string]any{"validations": shipped}
	if err := b.writeCollectionDoc(vwp.ValidationsPath, RoleValidations, doc); err != nil {
		return nil, err
	}
	return report, nil
}

// --- internals ---

func (b *Builder) path(relPath string) string {
	return filepath.Join(b.dir, filepath.FromSlash(relPath))
}

func (b *Builder) writePart(relPath string, data []byte) error {
	if vwp.IsUnsafePath(relPath) {
		return fmt.Errorf("vwp/builder: unsafe part path: %s", relPath)
	}
	target := b.path(relPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("vwp/builder: write %s: %w", relPath, err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("vwp/builder: write %s: %w", relPath, err)
	}
	return nil
}

// putFilePart writes a raw part with id-by-path idempotency, indexes it in
// the manifest, and saves the manifest.
func (b *Builder) putFilePart(relPath, mediaType, role string, content []byte) error {
	if vwp.IsUnsafePath(relPath) {
		return fmt.Errorf("vwp/builder: unsafe part path: %s", relPath)
	}
	existing, err := os.ReadFile(b.path(relPath))
	switch {
	case err == nil:
		if vwp.SHA256Hex(existing) != vwp.SHA256Hex(content) {
			return fmt.Errorf("%w: part %s", ErrConflict, relPath)
		}
	case os.IsNotExist(err):
		if err := b.writePart(relPath, content); err != nil {
			return err
		}
	default:
		return fmt.Errorf("vwp/builder: read %s: %w", relPath, err)
	}
	b.upsertManifestPart(relPath, mediaType, role, vwp.SHA256Hex(content))
	return b.saveManifest()
}

// addCollectionObject appends obj (identified by id) to the `{key: [...]}`
// collection at relPath, enforcing id idempotency.
func (b *Builder) addCollectionObject(relPath, key, role, id string, obj any) error {
	if id == "" {
		return fmt.Errorf("vwp/builder: %s object requires an id", key)
	}
	doc, err := b.readCollectionDoc(relPath)
	if err != nil {
		return err
	}
	items, _ := doc[key].([]any)
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok || m["id"] != id {
			continue
		}
		if jsonEqual(item, obj) {
			return nil // idempotent re-add
		}
		return fmt.Errorf("%w: %s %s", ErrConflict, key, id)
	}
	value, err := toJSONValue(obj)
	if err != nil {
		return err
	}
	doc[key] = append(items, value)
	return b.writeCollectionDoc(relPath, role, doc)
}

// readCollectionDoc loads a collection document, returning an empty object
// when the part does not exist yet.
func (b *Builder) readCollectionDoc(relPath string) (map[string]any, error) {
	var doc map[string]any
	err := readJSONFile(b.path(relPath), &doc)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func (b *Builder) writeCollectionDoc(relPath, role string, doc map[string]any) error {
	data, err := vwp.EncodeJSON(doc)
	if err != nil {
		return err
	}
	if err := b.writePart(relPath, data); err != nil {
		return err
	}
	b.upsertManifestPart(relPath, "application/json", role, vwp.SHA256Hex(data))
	return b.saveManifest()
}

// upsertManifestPart updates the digest of an already-indexed part or
// appends a new part reference. workproduct.json and
// provenance/provenance.json update their dedicated manifest references.
func (b *Builder) upsertManifestPart(relPath, mediaType, role, sha string) {
	switch relPath {
	case vwp.WorkproductPath:
		if b.manifest.Workproduct == nil {
			b.manifest.Workproduct = &vwp.PartRef{Path: relPath, MediaType: mediaType, Role: role}
		}
		b.manifest.Workproduct.SHA256 = sha
		return
	case vwp.ProvenancePath:
		if b.manifest.Provenance == nil {
			b.manifest.Provenance = &vwp.PartRef{Path: relPath, MediaType: mediaType, Role: role}
		}
		b.manifest.Provenance.SHA256 = sha
		return
	}
	for i := range b.manifest.Parts {
		if b.manifest.Parts[i].Path == relPath {
			b.manifest.Parts[i].SHA256 = sha
			return
		}
	}
	b.manifest.Parts = append(b.manifest.Parts, vwp.PartRef{
		Path: relPath, MediaType: mediaType, Role: role, SHA256: sha,
	})
}

func (b *Builder) saveManifest() error {
	if err := validateObject(schema.Manifest, b.manifest); err != nil {
		return err
	}
	data, err := b.manifest.Encode()
	if err != nil {
		return err
	}
	return b.writePart(vwp.ManifestPath, data)
}

func validateObject(schemaName string, obj any) error {
	errs, err := schema.Validate(schemaName, obj)
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return fmt.Errorf("vwp/builder: %s schema violation: %s", schemaName, strings.Join(errs, "; "))
	}
	return nil
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("vwp/builder: %s: %w", path, err)
	}
	return nil
}

// toJSONValue converts a Go value to its plain decoded-JSON shape.
func toJSONValue(obj any) (any, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// jsonEqual compares two values by their decoded-JSON shape.
func jsonEqual(a, b any) bool {
	av, errA := toJSONValue(a)
	bv, errB := toJSONValue(b)
	if errA != nil || errB != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

func mediaTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".md":
		return "text/markdown"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}
