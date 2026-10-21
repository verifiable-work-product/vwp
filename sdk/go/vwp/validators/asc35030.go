package validators

import (
	"fmt"
	"math"
	"sort"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// ASC 350-30 extension validator ids.
const (
	ValidatorASC35030ImpairmentSchemaValid       = "asc35030.impairment.schemaValid"
	ValidatorASC35030ImpairmentRecompute         = "asc35030.impairment.recompute"
	ValidatorASC35030ImpairmentNoReversal        = "asc35030.impairment.noReversal"
	ValidatorASC35030ImpairmentFairValueEvidence = "asc35030.impairment.fairValueEvidence"
	ValidatorASC35030ImpairmentContinuity        = "asc35030.impairment.continuity"
	ValidatorASC35030AccountingScheduleTieOut    = vwp.ValidatorASC35030AccountingScheduleTieOut
)

const (
	impairmentTestType      = "indefiniteLivedIntangibleImpairmentTest"
	impairmentTestSchemaRef = "extensions/accounting-asc350-30/impairment-test.schema.json"
)

type lotTest struct {
	path     string
	testDate time.Time
	before   float64
	after    float64
}

// ASC35030 implements the vwp-accounting-asc350-30 extension validators for
// the indefinite-lived intangible impairment test applied to digital assets
// outside ASC 350-60, over every test in the package (one per test date
// and scope): per lot, loss = max(0, carrying - fair value) with no lot
// listed twice in one test and no duplicate output rows; the new carrying
// amount never exceeds the old one (ASC 350-30-35-20 forbids reversal);
// each lot's fair value equals the value of the fact it cites; and when a
// lot is tested again later, it starts from the carrying amount the
// earlier test left it at.
func ASC35030(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	all := []string{ValidatorASC35030ImpairmentRecompute, ValidatorASC35030ImpairmentNoReversal, ValidatorASC35030ImpairmentFairValueEvidence, ValidatorASC35030ImpairmentContinuity}

	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "testDate") + "|" + getString(in, "scope")
	}
	hits, found := locateCalculations(pkg, add, ValidatorASC35030ImpairmentSchemaValid, impairmentTestType, impairmentTestSchemaRef, scope)
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid impairment test", all...)
		return results
	}

	facts := factIndex(ctx)
	var wrong, reversed, badFact, continuity []string
	history := map[string][]lotTest{}
	var total float64
	var lots int
	for _, hit := range hits {
		tol := tolerance(hit.Doc, "roundingTolerance", 0.01)
		inputs := getMap(hit.Doc, "inputs")
		inLots := getMaps(inputs, "lots")
		outputs := getMap(hit.Doc, "outputs")
		pfx := hit.Path + ": "
		testDate, okDate := parseWhen(getString(inputs, "testDate"))
		if !okDate {
			continuity = append(continuity, pfx+"testDate is not a parseable date")
		}
		outByLot, dups := indexByID(getMaps(outputs, "lots"), "lotId")
		for _, d := range dups {
			wrong = append(wrong, pfx+"duplicate output row for lot "+d)
		}
		seen := map[string]bool{}
		var calcTotal float64
		for _, l := range inLots {
			lots++
			id := getString(l, "lotId")
			if seen[id] {
				wrong = append(wrong, pfx+"lot "+id+" listed twice in one test")
				continue
			}
			seen[id] = true
			carrying := jsNumber(l["carryingAmount"])
			fv := jsNumber(l["fairValue"])
			loss := math.Max(0, carrying-fv)
			calcTotal += loss
			o, ok := outByLot[id]
			if !ok {
				wrong = append(wrong, pfx+id+" (no output row)")
				continue
			}
			declaredLoss := jsNumber(o["impairmentLoss"])
			after := jsNumber(o["carryingAmountAfter"])
			if !nearTol(declaredLoss, loss, tol) || !nearTol(after, carrying-loss, tol) {
				wrong = append(wrong, fmt.Sprintf("%s%s (expected loss %s / after %s, declared %s / %s)", pfx, id, num(loss), num(carrying-loss), num(declaredLoss), num(after)))
			}
			if declaredLoss < -tol || after > carrying+tol {
				reversed = append(reversed, id)
			}
			if msg := checkFactValue(facts, getString(l, "fairValueFact"), fv, tol, pfx+id+" fairValue", "fairValue", "amount", "price", "lowestObservedPrice"); msg != "" {
				badFact = append(badFact, msg)
			}
			if okDate {
				history[id] = append(history[id], lotTest{path: hit.Path, testDate: testDate, before: carrying, after: after})
			}
		}
		for id := range outByLot {
			if !seen[id] {
				wrong = append(wrong, pfx+"output row for lot "+id+" which is not tested in this calculation")
			}
		}
		if declared := jsNumber(outputs["totalImpairmentLoss"]); !nearTol(declared, calcTotal, tol) {
			wrong = append(wrong, fmt.Sprintf("%stotalImpairmentLoss %s does not equal the sum of lot losses %s", pfx, num(declared), num(calcTotal)))
		}
		total += calcTotal
	}
	// A lot tested more than once must carry forward the earlier result.
	for id, tests := range history {
		sort.Slice(tests, func(i, j int) bool { return tests[i].testDate.Before(tests[j].testDate) })
		for i := 1; i < len(tests); i++ {
			prev, cur := tests[i-1], tests[i]
			if cur.testDate.Equal(prev.testDate) {
				continuity = append(continuity, fmt.Sprintf("lot %s tested twice on %s (%s, %s)", id, cur.testDate.Format("2006-01-02"), prev.path, cur.path))
				continue
			}
			if !nearTol(cur.before, prev.after, 0.01) {
				continuity = append(continuity, fmt.Sprintf("lot %s starts %s at carrying amount %s but the %s test left it at %s", id, cur.path, num(cur.before), prev.testDate.Format("2006-01-02"), num(prev.after)))
			}
		}
	}
	sortStrings(continuity)

	emit := func(id, passMsg, failPrefix string, errs []string) {
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusFail, Message: failPrefix + joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusPass, Message: passMsg})
		}
	}
	emit(ValidatorASC35030ImpairmentRecompute, fmt.Sprintf("%d lot tests recompute; total impairment loss %s", lots, num(total)), "lots do not recompute: ", wrong)
	failList(add, ValidatorASC35030ImpairmentNoReversal,
		"no lot is written up above its carrying amount",
		"lots whose carrying amount would increase (ASC 350-30-35-20 forbids reversal): ", reversed)
	emit(ValidatorASC35030ImpairmentFairValueEvidence, "every lot's fair value equals the value of the fact it cites", "fair values disagree with their cited facts: ", badFact)
	emit(ValidatorASC35030ImpairmentContinuity, "lots tested more than once carry forward the earlier test's carrying amount", "carrying amounts do not carry forward between tests: ", continuity)

	// Executable impairment entries: one per impaired lot on its test date,
	// for the loss the test recomputes (never the declared figure).
	impair := scheduleExpectation{kind: accountingschedule.KindImpairment}
	for _, hit := range hits {
		inputs := getMap(hit.Doc, "inputs")
		impair.sources++
		impair.currency(getString(inputs, "currency"))
		testDate, ok := isoDate(getString(inputs, "testDate"))
		if !ok {
			impair.problem(hit.Path + ": testDate is not a date")
			continue
		}
		for _, l := range getMaps(inputs, "lots") {
			id := getString(l, "lotId")
			loss := math.Max(0, jsNumber(l["carryingAmount"])-jsNumber(l["fairValue"]))
			impair.group(scheduleGroup{obligationId: id, date: testDate, label: hit.Path + " lot " + id,
				postings: []schedulePosting{debit(loss, RoleImpairmentLoss), credit(loss, RoleIntangibleAsset)}})
		}
	}
	add(tieOutAccountingSchedule(pkg, ValidatorASC35030AccountingScheduleTieOut, impair))
	return results
}
