package validators

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// calcHit is one calculation part located by its declared type.
type calcHit struct {
	Path string
	Doc  map[string]any
}

// calculationsByType returns every calculations/*.json part whose `type`
// equals calcType, in path order. Extension validators locate their data
// contracts by type rather than by a fixed path so that a template's
// section id prefix (s4_roll_forward) never hides the calculation from the
// validator that recomputes it.
func calculationsByType(pkg vwp.Source, calcType string) []calcHit {
	var hits []calcHit
	for _, p := range pkg.List() {
		if !strings.HasPrefix(p, "calculations/") || !strings.HasSuffix(p, ".json") {
			continue
		}
		doc, ok := readJSONValue(pkg, p)
		if !ok {
			continue
		}
		m := asMap(doc)
		if getString(m, "type") == calcType {
			hits = append(hits, calcHit{Path: p, Doc: m})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Path < hits[j].Path })
	return hits
}

// locateCalculations finds every calculation of calcType, schema-checks each
// against schemaRef and reports ONE result under validatorId: notRun when
// none exists, fail when any is invalid or two declare the same scope key,
// pass listing the parts otherwise. A portfolio ordinarily carries several
// schedules of one type (one lot relief per pool, one accrual per loan);
// scopeKey names the scope each declares so overlaps are caught instead of
// silently summed. It returns the schema-valid hits and whether any exist.
func locateCalculations(pkg vwp.Source, add func(vwp.ValidationResult), validatorId, calcType, schemaRef string, scopeKey func(map[string]any) string) ([]calcHit, bool) {
	hits := calculationsByType(pkg, calcType)
	if len(hits) == 0 {
		add(vwp.ValidationResult{ValidatorId: validatorId, Status: vwp.StatusNotRun, Message: "no calculation of type " + calcType + " under calculations/"})
		return nil, false
	}
	var valid []calcHit
	var problems []string
	seen := map[string]string{}
	for _, h := range hits {
		errs, err := schema.ValidateExtension(schemaRef, h.Doc)
		if err != nil {
			errs = []string{err.Error()}
		}
		if len(errs) > 0 {
			problems = append(problems, h.Path+": "+joinCapped(errs, 3))
			continue
		}
		if scopeKey != nil {
			key := scopeKey(h.Doc)
			if prev, dup := seen[key]; dup {
				problems = append(problems, fmt.Sprintf("%s declares the same scope (%s) as %s", h.Path, key, prev))
				continue
			}
			seen[key] = h.Path
		}
		valid = append(valid, h)
	}
	if len(problems) > 0 {
		add(vwp.ValidationResult{ValidatorId: validatorId, Status: vwp.StatusFail, Message: joinCapped(problems, 5)})
	} else {
		add(vwp.ValidationResult{ValidatorId: validatorId, Status: vwp.StatusPass, Message: strings.Join(paths(valid), ", ")})
	}
	return valid, true
}

func paths(hits []calcHit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Path)
	}
	return out
}

// tolerance reads a declared tolerance off a calculation, falling back to
// def when absent or unusable.
func tolerance(doc map[string]any, key string, def float64) float64 {
	v, ok := doc[key]
	if !ok {
		return def
	}
	f := jsNumber(v)
	if math.IsNaN(f) || f < 0 {
		return def
	}
	return f
}

// nearTol reports whether two amounts agree within tol. NaN never agrees.
func nearTol(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// daysBetween returns the whole days from start (inclusive) to end
// (exclusive) for YYYY-MM-DD dates; ok is false on a parse failure.
func daysBetween(start, end string) (int, bool) {
	s, err := time.Parse("2006-01-02", start)
	if err != nil {
		return 0, false
	}
	e, err := time.Parse("2006-01-02", end)
	if err != nil {
		return 0, false
	}
	return int(e.Sub(s).Hours() / 24), true
}

// parseWhen accepts the timestamp shapes a subledger export carries — a
// date, an RFC 3339 instant, or a date-time without zone — so chronology
// is compared as time, never as text.
func parseWhen(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// notRun emits a notRun result for every id in ids with the same message.
func notRun(add func(vwp.ValidationResult), msg string, ids ...string) {
	for _, id := range ids {
		add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusNotRun, Message: msg})
	}
}

// failList emits a pass when refs is empty, else a fail naming them.
func failList(add func(vwp.ValidationResult), id, passMsg, failPrefix string, refs []string) {
	if len(refs) == 0 {
		add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusPass, Message: passMsg})
		return
	}
	add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusFail, Message: failPrefix + joinCapped(refs, 8), Refs: refs})
}

// factIndex maps fact id → fact object for value cross-checks.
func factIndex(ctx *vwp.ExtensionContext) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, f := range ctx.Facts {
		if id := getString(f, "id"); id != "" {
			out[id] = f
		}
	}
	return out
}

// factNumber extracts the numeric value a calculation input claims to take
// from a fact: the fact's `value` when it is a number, else the first of
// keys found on an object value. A fact referenced by a calculation must
// carry a typed value — a prose-only fact cannot be compared and fails.
func factNumber(f map[string]any, keys ...string) (float64, bool) {
	switch v := f["value"].(type) {
	case float64:
		return v, true
	case string:
		n := jsNumber(v)
		return n, !math.IsNaN(n)
	case map[string]any:
		for _, k := range keys {
			if raw, ok := v[k]; ok {
				n := jsNumber(raw)
				if !math.IsNaN(n) {
					return n, true
				}
			}
		}
	}
	return 0, false
}

// checkFactValue compares a calculation input against the fact it cites.
// It returns a description of the mismatch, or "" when the fact resolves
// and agrees within tol.
func checkFactValue(facts map[string]map[string]any, ref string, want float64, tol float64, label string, keys ...string) string {
	f, ok := facts[ref]
	if !ok {
		return fmt.Sprintf("%s cites fact %q which is not in the package", label, ref)
	}
	got, ok := factNumber(f, keys...)
	if !ok {
		return fmt.Sprintf("%s cites fact %q which carries no numeric value (keys tried: %s)", label, ref, strings.Join(keys, ", "))
	}
	if !nearTol(got, want, tol) {
		return fmt.Sprintf("%s is %s but fact %q says %s", label, num(want), ref, num(got))
	}
	return ""
}

// factDate extracts the date a fact's typed value is stated as of: a date
// string value, or `date` / `asOf` / `recordedOn` on an object value.
func factDate(f map[string]any) (time.Time, bool) {
	switch v := f["value"].(type) {
	case string:
		return parseWhen(v)
	case map[string]any:
		for _, k := range []string{"date", "asOf", "recordedOn"} {
			if s, ok := v[k].(string); ok {
				if t, ok := parseWhen(s); ok {
					return t, true
				}
			}
		}
	}
	return time.Time{}, false
}

// indexByID builds id → row for an output collection, reporting duplicate
// ids instead of letting a later row silently overwrite an earlier one.
func indexByID(rows []map[string]any, key string) (map[string]map[string]any, []string) {
	out := map[string]map[string]any{}
	var dups []string
	for _, r := range rows {
		id := getString(r, key)
		if _, seen := out[id]; seen {
			dups = append(dups, id)
			continue
		}
		out[id] = r
	}
	return out, dups
}

// periodsOverlap reports whether [aStart,aEnd] and [bStart,bEnd] share a day.
func periodsOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return !aEnd.Before(bStart) && !bEnd.Before(aStart)
}
