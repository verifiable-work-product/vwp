package validators

import (
	"strconv"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// ASC 820 extension validator ids.
const (
	ValidatorASC820MeasurementsPresent     = "asc820.measurements.present"
	ValidatorASC820MeasurementsSchemaValid = "asc820.measurements.schemaValid"
	ValidatorASC820PricingSourcesResolve   = "asc820.pricingSources.resolve"
	ValidatorASC820Level3JudgmentLinked    = "asc820.level3.judgmentLinked"
)

// Data contract paths consumed by the ASC 820 validators.
const (
	fairValueRegisterPath      = "facts/fair-value-measurements.json"
	fairValueRegisterSchemaRef = "extensions/accounting-asc820/fair-value-measurements.schema.json"
)

// ASC820 implements the vwp-accounting-asc820 extension validators, run
// against the fair value measurement register at
// facts/fair-value-measurements.json.
func ASC820(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	registerDoc, present := readJSONValue(pkg, fairValueRegisterPath)
	if !present {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC820MeasurementsPresent,
			Status:      vwp.StatusFail,
			Message:     "facts/fair-value-measurements.json missing or unreadable",
		})
		return results
	}
	errs, err := schema.ValidateExtension(fairValueRegisterSchemaRef, registerDoc)
	if err != nil {
		errs = []string{err.Error()}
	}
	if len(errs) > 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC820MeasurementsSchemaValid,
			Status:      vwp.StatusFail,
			Message:     joinCapped(errs, 5) + " (remaining asc820 validators not run)",
		})
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorASC820MeasurementsSchemaValid, Status: vwp.StatusPass})

	measurements := getMaps(asMap(registerDoc), "measurements")
	evidenceIds := idSet(ctx.Evidence)
	judgmentIds := idSet(ctx.Judgments)

	var badSources []string
	for _, m := range measurements {
		refs, _ := stringRefs(m, "pricingSourceEvidence")
		for _, ref := range refs {
			if !evidenceIds[ref] {
				badSources = append(badSources, getString(m, "id"))
				break
			}
		}
	}
	if len(badSources) == 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC820PricingSourcesResolve,
			Status:      vwp.StatusPass,
			Message:     strconv.Itoa(len(measurements)) + " measurements have resolvable pricing source evidence",
		})
	} else {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC820PricingSourcesResolve,
			Status:      vwp.StatusFail,
			Message:     "measurements with unresolvable pricing source evidence: " + strings.Join(badSources, ", "),
			Refs:        badSources,
		})
	}

	var level3 []map[string]any
	for _, m := range measurements {
		if getString(m, "level") == "level3" {
			level3 = append(level3, m)
		}
	}
	var unjudged []string
	for _, m := range level3 {
		if ref := getString(m, "judgmentRef"); ref == "" || !judgmentIds[ref] {
			unjudged = append(unjudged, getString(m, "id"))
		}
	}
	if len(unjudged) == 0 {
		n := len(level3)
		linkVerb := "link"
		if n == 1 {
			linkVerb = "links"
		}
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC820Level3JudgmentLinked,
			Status:      vwp.StatusPass,
			Message: strconv.Itoa(n) + " level3 " + plural(n, "measurement", "measurements") +
				" " + linkVerb + " a hierarchy/unobservable-inputs judgment",
		})
	} else {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC820Level3JudgmentLinked,
			Status:      vwp.StatusFail,
			Message:     "level3 measurements without a resolvable judgmentRef: " + strings.Join(unjudged, ", "),
			Refs:        unjudged,
		})
	}
	return results
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
