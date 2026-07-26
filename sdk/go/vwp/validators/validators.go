// Package validators implements the built-in VWP extension validators
// (ASC 606, ASC 820, digital-asset wallet ownership) with the same validator
// ids and semantics as the reference TypeScript CLI, and provides the
// batteries-included Verify entry point.
package validators

import (
	"math"
	"strconv"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
)

// Registry returns the built-in extension validators keyed by the manifest
// `extension` identifier they apply to.
func Registry() map[string]vwp.ExtensionValidator {
	return map[string]vwp.ExtensionValidator{
		vwp.ExtensionASC606:        ASC606,
		vwp.ExtensionASC820:        ASC820,
		vwp.ExtensionWalletControl: WalletOwnership,
	}
}

// Verify runs full package verification: core validators, profile
// validators, and every built-in extension validator applicable to the
// package's declared extension.
func Verify(pkg vwp.Source) *vwp.Report {
	return vwp.Verify(pkg, vwp.WithExtensionValidators(Registry()))
}

// near reports whether two amounts agree within the shared rounding
// tolerance used by the tie-out validators. NaN never compares near.
func near(a, b float64) bool {
	return math.Abs(a-b) <= 0.01
}

// num formats a float the way JavaScript renders numbers in template
// strings, so messages match the reference CLI (300000 not 300000.000000).
func num(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// jsNumber mirrors JavaScript Number(x ?? 0): nil -> 0, numbers pass
// through, numeric strings parse, anything else is NaN.
func jsNumber(v any) float64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return t
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return math.NaN()
		}
		return f
	case bool:
		if t {
			return 1
		}
		return 0
	default:
		return math.NaN()
	}
}

func getString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func getSlice(m map[string]any, key string) []any {
	s, _ := m[key].([]any)
	return s
}

func getMap(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func getMaps(m map[string]any, key string) []map[string]any {
	items := getSlice(m, key)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mm, ok := item.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

// stringRefs returns the string elements of an array field, and whether the
// field was a non-empty array of resolvable strings is up to the caller.
func stringRefs(m map[string]any, key string) ([]string, bool) {
	raw, isArray := m[key].([]any)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, "")
		}
	}
	return out, isArray
}

func idSet(items []map[string]any) map[string]bool {
	ids := map[string]bool{}
	for _, item := range items {
		if id := getString(item, "id"); id != "" {
			ids[id] = true
		}
	}
	return ids
}

func joinCapped(items []string, limit int) string {
	if len(items) > limit {
		items = items[:limit]
	}
	return strings.Join(items, "; ")
}

// readJSONValue reads a JSON part as a decoded value; ok is false when the
// part is missing or not valid JSON (mirroring the reference CLI's
// undefined). The value may be any JSON shape; schema checks reject
// non-objects where an object is required.
func readJSONValue(pkg vwp.Source, relPath string) (any, bool) {
	var doc any
	if err := vwp.ReadJSON(pkg, relPath, &doc); err != nil {
		return nil, false
	}
	return doc, true
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
