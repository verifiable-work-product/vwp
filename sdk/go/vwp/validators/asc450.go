package validators

import (
	"fmt"
	"strconv"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// ASC 450 extension validator ids.
const (
	ValidatorASC450RegisterPresent              = "asc450.register.present"
	ValidatorASC450RegisterSchemaValid          = "asc450.register.schemaValid"
	ValidatorASC450RegisterJudgmentLinked       = "asc450.register.judgmentLinked"
	ValidatorASC450RegisterAccrualConsistent    = "asc450.register.accrualConsistent"
	ValidatorASC450RegisterDisclosureConsistent = "asc450.register.disclosureConsistent"
	ValidatorASC450GoingConcernJudged           = "asc450.goingConcern.judged"
	ValidatorASC450AccountingScheduleTieOut     = vwp.ValidatorASC450AccountingScheduleTieOut
)

// Data contract locations consumed by the ASC 450 validators.
const (
	contingenciesPath      = "facts/contingencies.json"
	contingenciesSchemaRef = "extensions/accounting-asc450/contingencies.schema.json"
	asc450DefaultTol       = 0.01
)

// Account roles the ASC 450 executable schedule uses: the loss recognized
// in income against the liability accrued for it (ASC 450-20-25-2).
const (
	RoleContingencyLoss             = "contingency_loss"
	RoleAccruedContingencyLiability = "accrued_contingency_liability"
)

// ASC450 implements the vwp-accounting-asc450 extension validators over the
// loss contingency register (facts/contingencies.json) and the package's
// judgments:
//
//   - register.judgmentLinked: every item links a resolvable judgment, and
//     its evidenceRef (when given) resolves to evidence or a fact;
//   - register.accrualConsistent: the accrual follows ASC 450-20-25-2 and
//     30-1 — probable and estimable accrues the best estimate, else the low
//     end of the range, and nothing else accrues; the range the item states
//     equals the cited estimate fact;
//   - register.disclosureConsistent: reasonably possible losses, probable
//     losses that cannot be estimated and remote guarantee-type items are
//     disclosed (ASC 450-20-50-3 to 50-6);
//   - goingConcern.judged: the package carries a going concern evaluation —
//     a judgment on substantial doubt and a claim citing ASC 205-40;
//   - accountingSchedule.tieOut: the executable schedule books one
//     contingency_accrual group per accrued item on the assessment date for
//     the amount the register's rules recompute, and nothing else.
//
// None of this decides likelihood: that is the preparer's judgment, which
// the validators only require to exist and to be applied consistently.
func ASC450(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	registerDoc, present := readJSONValue(pkg, contingenciesPath)
	reg := asMap(registerDoc)
	items := getMaps(reg, "items")
	if !present || len(items) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC450RegisterPresent, Status: vwp.StatusFail, Message: contingenciesPath + " missing or contains no items"})
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorASC450RegisterPresent, Status: vwp.StatusPass, Message: strconv.Itoa(len(items)) + " contingencies in register"})
	errs, err := schema.ValidateExtension(contingenciesSchemaRef, registerDoc)
	if err != nil {
		errs = []string{err.Error()}
	}
	if len(errs) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC450RegisterSchemaValid, Status: vwp.StatusFail, Message: joinCapped(errs, 5) + " (remaining ASC 450 validators not run)"})
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorASC450RegisterSchemaValid, Status: vwp.StatusPass})

	tol := tolerance(reg, "roundingTolerance", asc450DefaultTol)
	asOf, okDate := isoDate(getString(reg, "asOf"))
	facts := factIndex(ctx)
	judgmentIds := idSet(ctx.Judgments)
	evidenceIds := idSet(ctx.Evidence)
	factIds := idSet(ctx.Facts)

	exp := scheduleExpectation{kind: accountingschedule.KindContingencyAccrual, sources: 1}
	exp.currency(getString(reg, "currency"))
	if !okDate {
		exp.problem(contingenciesPath + ": asOf is not a date")
	}

	var unlinked, accrual, disclosure []string
	seen := map[string]bool{}
	for _, it := range items {
		id := getString(it, "id")
		pfx := id + ": "
		if seen[id] {
			accrual = append(accrual, pfx+"listed twice")
			continue
		}
		seen[id] = true
		if ref := getString(it, "judgmentRef"); ref == "" || !judgmentIds[ref] {
			unlinked = append(unlinked, pfx+"judgmentRef "+strconv.Quote(ref)+" does not resolve")
		}
		if ref := getString(it, "evidenceRef"); ref != "" && !evidenceIds[ref] && !factIds[ref] {
			unlinked = append(unlinked, pfx+"evidenceRef "+strconv.Quote(ref)+" is neither evidence nor a fact in the package")
		}

		likelihood := getString(it, "likelihood")
		estimable, _ := it["estimable"].(bool)
		low, high := jsNumber(it["estimateLow"]), jsNumber(it["estimateHigh"])
		_, hasLow := it["estimateLow"]
		_, hasHigh := it["estimateHigh"]
		best, hasBest := it["bestEstimate"]
		accrued := jsNumber(it["accrued"])

		// The range the item states is the range its fact states.
		if ref := getString(it, "estimateFact"); ref != "" {
			if msg := checkFactValue(facts, ref, low, tol, pfx+"estimateLow", "low", "estimateLow", "minimum"); msg != "" {
				accrual = append(accrual, msg)
			}
			if msg := checkFactValue(facts, ref, high, tol, pfx+"estimateHigh", "high", "estimateHigh", "maximum"); msg != "" {
				accrual = append(accrual, msg)
			}
			if hasBest {
				if msg := checkFactValue(facts, ref, jsNumber(best), tol, pfx+"bestEstimate", "best", "bestEstimate"); msg != "" {
					accrual = append(accrual, msg)
				}
			}
		}

		// ASC 450-20-25-2: accrue when probable AND reasonably estimable;
		// ASC 450-20-30-1: the best estimate, else the low end of the range.
		expected := 0.0
		switch {
		case likelihood == "probable" && estimable:
			if !hasLow || !hasHigh {
				accrual = append(accrual, pfx+"probable and estimable but no estimateLow/estimateHigh range")
				break
			}
			if low > high+tol {
				accrual = append(accrual, fmt.Sprintf("%sestimateLow %s exceeds estimateHigh %s", pfx, num(low), num(high)))
			}
			expected = low
			if hasBest {
				b := jsNumber(best)
				if b < low-tol || b > high+tol {
					accrual = append(accrual, fmt.Sprintf("%sbestEstimate %s lies outside the range %s to %s", pfx, num(b), num(low), num(high)))
				}
				expected = b
			}
			if !nearTol(accrued, expected, tol) {
				which := "the low end of the range (ASC 450-20-30-1, no best estimate)"
				if hasBest {
					which = "the best estimate (ASC 450-20-30-1)"
				}
				accrual = append(accrual, fmt.Sprintf("%saccrued %s but a probable, estimable loss accrues %s: %s", pfx, num(accrued), which, num(expected)))
			}
		case likelihood == "probable":
			if accrued > tol {
				accrual = append(accrual, fmt.Sprintf("%saccrued %s although the loss is not reasonably estimable (ASC 450-20-25-2(b) not met: disclose, do not accrue)", pfx, num(accrued)))
			}
		case likelihood == "reasonablyPossible" || likelihood == "remote":
			if accrued > tol {
				accrual = append(accrual, fmt.Sprintf("%saccrued %s although the loss is %s (ASC 450-20-25-2(a) not met)", pfx, num(accrued), likelihood))
			}
		}

		// ASC 450-20-50-3 to 50-6: what must be disclosed.
		disclosed, _ := it["disclosureRequired"].(bool)
		switch {
		case likelihood == "reasonablyPossible" && !disclosed:
			disclosure = append(disclosure, pfx+"reasonably possible but disclosureRequired is false (ASC 450-20-50-3)")
		case likelihood == "probable" && !estimable && !disclosed:
			disclosure = append(disclosure, pfx+"probable but not estimable and disclosureRequired is false (ASC 450-20-50-3 to 50-5)")
		case likelihood == "remote" && getString(it, "type") == "guarantee" && !disclosed:
			disclosure = append(disclosure, pfx+"a remote guarantee-type contingency is still disclosed (ASC 450-20-50-6)")
		}

		if okDate && expected >= tol && likelihood == "probable" && estimable {
			exp.group(scheduleGroup{obligationId: id, date: asOf, label: contingenciesPath + " " + id,
				postings: []schedulePosting{debit(expected, RoleContingencyLoss), credit(expected, RoleAccruedContingencyLiability)}})
		}
	}
	failList(add, ValidatorASC450RegisterJudgmentLinked,
		"every contingency links a resolvable judgment and its evidence resolves",
		"contingencies with unresolvable references: ", unlinked)
	failList(add, ValidatorASC450RegisterAccrualConsistent,
		"accruals follow ASC 450-20-25-2 and 30-1 from each item's likelihood, estimability and range",
		"accruals inconsistent with the likelihood and range: ", accrual)
	if len(accrual) > 0 {
		exp.problem("the register's accruals do not follow from the likelihood and range")
	}
	failList(add, ValidatorASC450RegisterDisclosureConsistent,
		"disclosure follows ASC 450-20-50 from each item's likelihood and type",
		"disclosure inconsistent with the likelihood: ", disclosure)

	// ASC 205-40-50-1: management evaluates whether conditions raise
	// substantial doubt within one year after the issuance date. The
	// package must carry that evaluation as a judgment and conclude on it
	// in a claim citing ASC 205-40.
	var goingConcern []string
	judged := false
	for _, j := range ctx.Judgments {
		text := strings.ToLower(getString(j, "topic") + " " + getString(j, "question"))
		if strings.Contains(text, "going concern") || strings.Contains(text, "substantial doubt") {
			judged = true
		}
	}
	if !judged {
		goingConcern = append(goingConcern, "no judgment addresses going concern / substantial doubt")
	}
	claimed := false
	for _, c := range ctx.Claims {
		refs, _ := stringRefs(c, "standardRefs")
		for _, r := range refs {
			if strings.Contains(r, "205-40") {
				claimed = true
			}
		}
	}
	if !claimed {
		goingConcern = append(goingConcern, "no claim cites ASC 205-40")
	}
	failList(add, ValidatorASC450GoingConcernJudged,
		"the going concern evaluation is a judgment and a claim citing ASC 205-40 concludes on it",
		"going concern evaluation incomplete (ASC 205-40-50-1): ", goingConcern)

	add(tieOutAccountingSchedule(pkg, ValidatorASC450AccountingScheduleTieOut, exp))
	return results
}
