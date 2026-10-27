package validators

import (
	"fmt"
	"math"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// ASC 250 extension validator ids.
const (
	ValidatorASC250CorrectionSchemaValid       = "asc250.correction.schemaValid"
	ValidatorASC250CorrectionRecompute         = "asc250.correction.recompute"
	ValidatorASC250CorrectionDualMethod        = "asc250.correction.dualMethod"
	ValidatorASC250CorrectionMaterialityJudged = "asc250.correction.materialityJudged"
	ValidatorASC250AccountingScheduleTieOut    = vwp.ValidatorASC250AccountingScheduleTieOut
)

// Data contract locations consumed by the ASC 250 validators.
const (
	errorCorrectionType      = "errorCorrectionAnalysis"
	errorCorrectionSchemaRef = "extensions/accounting-asc250/error-correction-analysis.schema.json"
	asc250DefaultTol         = 0.01
)

// Account roles the ASC 250 executable schedule uses. The misstated
// account's own role is declared on each line; the offset is either the
// prior-period adjustment to opening retained earnings (a restatement) or
// the income statement role the line declares (an out-of-period correction).
const (
	RolePriorPeriodAdjustment = "prior_period_adjustment"
)

// dispositionedStatuses are the judgment statuses that show a human has
// been asked to own (or has owned) a material conclusion; open and empty
// are not.
var dispositionedStatuses = map[string]bool{"reviewRequired": true, "reviewed": true, "approved": true}

// ASC250 implements the vwp-accounting-asc250 extension validators over
// every error correction analysis in the package (one per correction date
// and scope):
//
//   - correction.recompute: per line and period (listed chronologically)
//     asRestated = asReported + the cumulative correction through that
//     period, since the account's balance carries earlier corrections
//     forward; the line's cumulative correction is the sum of the periods; the
//     rollover figure is the largest absolute single-period misstatement
//     across lines; the iron-curtain figure is the absolute cumulative
//     misstatement across lines; both percentages are of a basis that
//     equals the fact it cites;
//   - correction.dualMethod: both SAB 108 quantifications are present and
//     recomputed, and the threshold is declared, so neither method can be
//     skipped;
//   - correction.materialityJudged: the materiality judgment resolves, and
//     when either method reaches the threshold it has a proposed
//     conclusion and a status a human owns (reviewRequired, reviewed or
//     approved — never open);
//   - accountingSchedule.tieOut: the executable schedule books one
//     error_correction group per line with a nonzero cumulative correction
//     on the correction date, under the line's own account role (on its
//     natural side for a positive correction) against its offset role.
//     Lines whose corrections net to zero need no entry.
func ASC250(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	facts := factIndex(ctx)
	judgmentById := map[string]map[string]any{}
	for _, j := range ctx.Judgments {
		judgmentById[getString(j, "id")] = j
	}

	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "correctionDate") + "|" + getString(in, "scope")
	}
	exp := scheduleExpectation{kind: accountingschedule.KindErrorCorrection}
	hits, found := locateCalculations(pkg, add, ValidatorASC250CorrectionSchemaValid, errorCorrectionType, errorCorrectionSchemaRef, scope)
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid error correction analysis", ValidatorASC250CorrectionRecompute, ValidatorASC250CorrectionDualMethod, ValidatorASC250CorrectionMaterialityJudged)
		add(tieOutAccountingSchedule(pkg, ValidatorASC250AccountingScheduleTieOut, exp))
		return results
	}

	var wrong, dual, judged []string
	for _, hit := range hits {
		pfx := hit.Path + ": "
		tol := tolerance(hit.Doc, "roundingTolerance", asc250DefaultTol)
		in := getMap(hit.Doc, "inputs")
		out := getMap(hit.Doc, "outputs")
		exp.sources++
		exp.currency(getString(in, "currency"))
		correctionDate, okDate := isoDate(getString(in, "correctionDate"))
		if !okDate {
			wrong = append(wrong, pfx+"correctionDate is not a parseable date")
		}

		outLines, dups := indexByID(getMaps(out, "lines"), "id")
		for _, d := range dups {
			wrong = append(wrong, pfx+"duplicate output line "+d)
		}
		seen := map[string]bool{}
		periodTotals := map[string]float64{}
		var cumulativeAll float64
		for _, l := range getMaps(in, "lines") {
			id := getString(l, "id")
			label := pfx + "line " + id
			if seen[id] {
				wrong = append(wrong, label+" listed twice")
				continue
			}
			seen[id] = true
			o, ok := outLines[id]
			if !ok {
				wrong = append(wrong, label+" (no output line)")
			}
			outPeriods, pdups := indexByID(getMaps(o, "periods"), "period")
			for _, d := range pdups {
				wrong = append(wrong, label+": duplicate output period "+d)
			}
			// Periods are listed chronologically; the account's balance
			// carries every earlier period's correction forward, so the
			// restated balance is as reported plus the running cumulative.
			seenPeriod := map[string]bool{}
			var cumulative float64
			for _, p := range getMaps(l, "periods") {
				period := getString(p, "period")
				if seenPeriod[period] {
					wrong = append(wrong, label+": period "+period+" listed twice")
					continue
				}
				seenPeriod[period] = true
				correction := jsNumber(p["correction"])
				cumulative += correction
				periodTotals[period] += correction
				restated := jsNumber(p["asReported"]) + cumulative
				if op, ok := outPeriods[period]; !ok {
					wrong = append(wrong, label+": no output for period "+period)
				} else if got := jsNumber(op["asRestated"]); !nearTol(got, restated, tol) {
					wrong = append(wrong, fmt.Sprintf("%s %s: asRestated declared %s but asReported %s + cumulative correction %s = %s", label, period, num(got), num(jsNumber(p["asReported"])), num(cumulative), num(restated)))
				}
			}
			for period := range outPeriods {
				if !seenPeriod[period] {
					wrong = append(wrong, label+": output period "+period+" which is not an input period")
				}
			}
			if ok {
				if got := jsNumber(o["cumulativeCorrection"]); !nearTol(got, cumulative, tol) {
					wrong = append(wrong, fmt.Sprintf("%s: cumulativeCorrection declared %s but the periods sum to %s", label, num(got), num(cumulative)))
				}
			}
			cumulativeAll += cumulative
			// The entry that corrects the balance at the correction date.
			if okDate {
				accountRole, offsetRole := getString(l, "accountRole"), getString(l, "offsetRole")
				increase := cumulative > 0
				accountDebit := (getString(l, "balanceSide") == "debit") == increase
				amount := math.Abs(cumulative)
				g := scheduleGroup{obligationId: id, date: correctionDate, label: label}
				if accountDebit {
					g.postings = []schedulePosting{debit(amount, accountRole), credit(amount, offsetRole)}
				} else {
					g.postings = []schedulePosting{credit(amount, accountRole), debit(amount, offsetRole)}
				}
				exp.group(g)
			}
		}
		for id := range outLines {
			if !seen[id] {
				wrong = append(wrong, pfx+"output line "+id+" which is not an input line")
			}
		}

		// SAB 108: quantify under both methods against the basis.
		materiality := getMap(in, "materiality")
		basis := jsNumber(materiality["basisAmount"])
		threshold := jsNumber(materiality["quantitativeThreshold"])
		if msg := checkFactValue(facts, getString(materiality, "basisFact"), basis, tol, pfx+"materiality.basisAmount", "amount", "basisAmount", "pretaxIncome", "revenue", "totalAssets"); msg != "" {
			wrong = append(wrong, msg)
		}
		var maxPeriod float64
		for _, v := range periodTotals {
			maxPeriod = math.Max(maxPeriod, math.Abs(v))
		}
		ironCurtain := math.Abs(cumulativeAll)
		rollover := getMap(out, "rollover")
		iron := getMap(out, "ironCurtain")
		if rollover == nil || iron == nil {
			dual = append(dual, pfx+"both rollover and ironCurtain quantifications are required (SAB 108)")
		}
		if threshold <= 0 {
			dual = append(dual, pfx+"materiality.quantitativeThreshold must be declared and positive")
		}
		var rolloverPct, ironPct float64
		if basis > 0 {
			rolloverPct, ironPct = maxPeriod/basis, ironCurtain/basis
		}
		for _, chk := range []struct {
			section map[string]any
			key     string
			want    float64
			tol     float64
			name    string
		}{
			{rollover, "maxPeriodCorrection", maxPeriod, tol, "rollover.maxPeriodCorrection"},
			{rollover, "pctOfBasis", rolloverPct, 1e-6, "rollover.pctOfBasis"},
			{iron, "cumulativeCorrection", ironCurtain, tol, "ironCurtain.cumulativeCorrection"},
			{iron, "pctOfBasis", ironPct, 1e-6, "ironCurtain.pctOfBasis"},
		} {
			if chk.section == nil {
				continue
			}
			if got := jsNumber(chk.section[chk.key]); !nearTol(got, chk.want, chk.tol) {
				wrong = append(wrong, fmt.Sprintf("%s%s declared %s but recomputes to %s", pfx, chk.name, num(got), num(chk.want)))
			}
		}

		// The materiality conclusion is a human's; a material result must be
		// dispositioned, never left open.
		ref := getString(in, "materialityJudgmentRef")
		j, ok := judgmentById[ref]
		switch {
		case !ok:
			judged = append(judged, fmt.Sprintf("%smaterialityJudgmentRef %q is not a judgment in the package", pfx, ref))
		case threshold > 0 && (rolloverPct >= threshold || ironPct >= threshold):
			if getString(j, "proposedConclusion") == "" || !dispositionedStatuses[getString(j, "status")] {
				judged = append(judged, fmt.Sprintf("%sthe misstatement reaches the %s threshold (rollover %s, iron curtain %s of the basis) but judgment %q has no conclusion or is still open", pfx, num(threshold), num(rolloverPct), num(ironPct), ref))
			}
		}
	}

	if len(wrong) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC250CorrectionRecompute, Status: vwp.StatusFail, Message: "error correction does not recompute: " + joinCapped(wrong, 6)})
		exp.problem("the error correction analysis does not recompute")
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC250CorrectionRecompute, Status: vwp.StatusPass, Message: fmt.Sprintf("%d analysis(es): restated amounts, cumulative corrections and both SAB 108 quantifications recompute against a basis their fact states", len(hits))})
	}
	failList(add, ValidatorASC250CorrectionDualMethod, "both the rollover and the iron-curtain quantifications are present against a declared threshold", "SAB 108 dual method incomplete: ", dual)
	failList(add, ValidatorASC250CorrectionMaterialityJudged, "the materiality judgment resolves and a material result is dispositioned", "materiality not judged: ", judged)
	add(tieOutAccountingSchedule(pkg, ValidatorASC250AccountingScheduleTieOut, exp))
	return results
}
