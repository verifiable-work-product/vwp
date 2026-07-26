package vwp

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// VwpVersion is the specification version this SDK implements.
const VwpVersion = "0.1"

// Well-known package part paths.
const (
	ManifestPath    = "manifest.json"
	WorkproductPath = "workproduct.json"
	ClaimsPath      = "claims/claims.json"
	ProvenancePath  = "provenance/provenance.json"
	EvidenceIndex   = "evidence/evidence.json"
	FactsPath       = "facts/facts.json"
	JudgmentsPath   = "judgments/judgments.json"
	ApprovalsPath   = "approvals/approvals.json"
	ValidationsPath = "validations/validation-report.json"
)

// PartRef is a package part reference in the manifest: a package-relative
// path with a media type and an optional integrity digest.
type PartRef struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	SHA256    string `json:"sha256,omitempty"`
	Role      string `json:"role,omitempty"`
}

// Manifest is the package manifest (manifest.json), the entry point of every
// VWP package.
type Manifest struct {
	VwpVersion  string         `json:"vwpVersion"`
	PackageId   string         `json:"packageId"`
	CreatedAt   string         `json:"createdAt"`
	Profile     string         `json:"profile"`
	Extension   string         `json:"extension,omitempty"`
	Title       string         `json:"title,omitempty"`
	Subject     map[string]any `json:"subject,omitempty"`
	Workproduct *PartRef       `json:"workproduct"`
	Parts       []PartRef      `json:"parts,omitempty"`
	Provenance  *PartRef       `json:"provenance"`
	Signatures  []PartRef      `json:"signatures,omitempty"`
}

// ParseManifest decodes manifest.json bytes.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("vwp: parse manifest: %w", err)
	}
	return &m, nil
}

// AllPartRefs returns every part reference the manifest declares, in the
// order workproduct, provenance, parts, signatures. References without a
// path are skipped.
func (m *Manifest) AllPartRefs() []PartRef {
	var refs []PartRef
	if m.Workproduct != nil && m.Workproduct.Path != "" {
		refs = append(refs, *m.Workproduct)
	}
	if m.Provenance != nil && m.Provenance.Path != "" {
		refs = append(refs, *m.Provenance)
	}
	for _, p := range m.Parts {
		if p.Path != "" {
			refs = append(refs, p)
		}
	}
	for _, s := range m.Signatures {
		if s.Path != "" {
			refs = append(refs, s)
		}
	}
	return refs
}

// FindPart returns the part reference for the given package-relative path,
// searching workproduct, provenance, parts, and signatures.
func (m *Manifest) FindPart(path string) (PartRef, bool) {
	for _, ref := range m.AllPartRefs() {
		if ref.Path == path {
			return ref, true
		}
	}
	return PartRef{}, false
}

// EncodeJSON renders a JSON document the way the reference tooling does:
// two-space indentation, no HTML escaping, trailing newline.
func EncodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Encode renders the manifest as manifest.json bytes.
func (m *Manifest) Encode() ([]byte, error) {
	return EncodeJSON(m)
}
