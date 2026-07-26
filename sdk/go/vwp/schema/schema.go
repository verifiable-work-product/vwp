// Package schema validates VWP objects against the vendored VWP v0.1 JSON
// Schemas (draft 2020-12). Schemas are embedded from vwpschemas/, a vendored
// copy of the reference repository's schemas/v0.1 tree; see
// vwpschemas/README.md for sync provenance.
package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed vwpschemas
var schemaFS embed.FS

// Version is the vendored VWP schema version.
const Version = "v0.1"

// baseURL is the canonical $id prefix of the vendored schemas.
const baseURL = "https://vwp.dev/schemas/v0.1/"

// Core schema names, matching the reference CLI's schema registry.
const (
	Manifest            = "manifest"
	Workproduct         = "workproduct"
	EvidenceSchema      = "evidence"
	Fact                = "fact"
	Claim               = "claim"
	Judgment            = "judgment"
	Calculation         = "calculation"
	Validation          = "validation"
	Approval            = "approval"
	Provenance          = "provenance"
	Signature           = "signature"
	AnalysisAttestation = "analysis-attestation"
)

// Names returns the core schema names accepted by Validate.
func Names() []string {
	return []string{
		Manifest, Workproduct, EvidenceSchema, Fact, Claim, Judgment,
		Calculation, Validation, Approval, Provenance, Signature,
		AnalysisAttestation,
	}
}

var (
	mu       sync.Mutex
	compiler *jsonschema.Compiler
	compiled = map[string]*jsonschema.Schema{}
	printer  = message.NewPrinter(language.English)
)

// compileLocked compiles (and caches) the schema at the given path relative
// to schemas/v0.1, e.g. "manifest.schema.json" or
// "extensions/accounting-asc606/ssp-allocation.schema.json".
func compileLocked(relPath string) (*jsonschema.Schema, error) {
	if sch, ok := compiled[relPath]; ok {
		return sch, nil
	}
	if compiler == nil {
		compiler = jsonschema.NewCompiler()
		compiler.AssertFormat()
	}
	raw, err := schemaFS.ReadFile("vwpschemas/v0.1/" + relPath)
	if err != nil {
		return nil, fmt.Errorf("vwp/schema: schema not found: %s", relPath)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("vwp/schema: parse schema %s: %w", relPath, err)
	}
	url := baseURL + relPath
	if err := compiler.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("vwp/schema: add schema %s: %w", relPath, err)
	}
	sch, err := compiler.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("vwp/schema: compile schema %s: %w", relPath, err)
	}
	compiled[relPath] = sch
	return sch, nil
}

func validateAgainst(relPath string, doc any) ([]string, error) {
	mu.Lock()
	sch, err := compileLocked(relPath)
	mu.Unlock()
	if err != nil {
		return nil, err
	}
	err = sch.Validate(doc)
	if err == nil {
		return nil, nil
	}
	var ve *jsonschema.ValidationError
	if ok := errorsAs(err, &ve); ok {
		return flatten(ve), nil
	}
	return []string{err.Error()}, nil
}

// errorsAs is a tiny local wrapper to avoid importing errors just for As.
func errorsAs(err error, target **jsonschema.ValidationError) bool {
	ve, ok := err.(*jsonschema.ValidationError)
	if ok {
		*target = ve
	}
	return ok
}

// flatten renders leaf validation causes as "<instancePath> <message>"
// strings, mirroring the reference CLI's error formatting.
func flatten(ve *jsonschema.ValidationError) []string {
	if len(ve.Causes) == 0 {
		loc := "(root)"
		if len(ve.InstanceLocation) > 0 {
			loc = "/" + strings.Join(ve.InstanceLocation, "/")
		}
		return []string{loc + " " + ve.ErrorKind.LocalizedString(printer)}
	}
	var out []string
	for _, cause := range ve.Causes {
		out = append(out, flatten(cause)...)
	}
	return out
}

// Validate checks an already-decoded JSON value against a named core schema
// and returns human-readable error strings (empty means valid). Go structs
// are accepted and round-tripped through JSON first.
func Validate(name string, doc any) ([]string, error) {
	if !isCoreName(name) {
		return nil, fmt.Errorf("vwp/schema: unknown schema: %s", name)
	}
	value, err := normalize(doc)
	if err != nil {
		return nil, err
	}
	return validateAgainst(name+".schema.json", value)
}

// ValidateBytes checks raw JSON bytes against a named core schema. It
// returns an error when the bytes are not valid JSON or the schema name is
// unknown; schema violations come back as error strings.
func ValidateBytes(name string, raw []byte) ([]string, error) {
	if !isCoreName(name) {
		return nil, fmt.Errorf("vwp/schema: unknown schema: %s", name)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("vwp/schema: invalid JSON: %w", err)
	}
	return validateAgainst(name+".schema.json", doc)
}

// ValidateExtension checks a value against an extension schema addressed by
// its path relative to schemas/v0.1, e.g.
// "extensions/accounting-asc606/ssp-allocation.schema.json".
func ValidateExtension(relPath string, doc any) ([]string, error) {
	value, err := normalize(doc)
	if err != nil {
		return nil, err
	}
	return validateAgainst(relPath, value)
}

func isCoreName(name string) bool {
	for _, n := range Names() {
		if n == name {
			return true
		}
	}
	return false
}

// normalize converts arbitrary Go values (including structs) into the plain
// map/slice/scalar shape the validator expects.
func normalize(doc any) (any, error) {
	switch doc.(type) {
	case nil, bool, string, float64, json.Number,
		map[string]any, []any:
		return doc, nil
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("vwp/schema: marshal value: %w", err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("vwp/schema: normalize value: %w", err)
	}
	return value, nil
}
