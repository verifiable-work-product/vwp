package validators

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// ASC 350-60 extension validator ids.
const (
	ValidatorASC35060ScopeRegisterPresent      = "asc35060.scope.registerPresent"
	ValidatorASC35060ScopeSchemaValid          = "asc35060.scope.schemaValid"
	ValidatorASC35060ScopeConsistent           = "asc35060.scope.consistent"
	ValidatorASC35060ScopeJudgmentLinked       = "asc35060.scope.judgmentLinked"
	ValidatorASC35060RollForwardSchemaValid    = "asc35060.rollForward.schemaValid"
	ValidatorASC35060RollForwardTieOut         = "asc35060.rollForward.tieOut"
	ValidatorASC35060RollForwardAssetsInScope  = "asc35060.rollForward.assetsInScope"
	ValidatorASC35060HoldingsSchemaValid       = "asc35060.significantHoldings.schemaValid"
	ValidatorASC35060HoldingsTieOut            = "asc35060.significantHoldings.tieOut"
	ValidatorASC35060HoldingsAssetsInScope     = "asc35060.significantHoldings.assetsInScope"
	ValidatorASC35060HoldingsTiesToRollForward = "asc35060.significantHoldings.tiesToRollForward"
	ValidatorASC35060AccountingScheduleTieOut  = vwp.ValidatorASC35060AccountingScheduleTieOut
)

// Data contract locations consumed by the ASC 350-60 validators.
const (
	cryptoAssetsPath          = "facts/crypto-assets.json"
	cryptoAssetsSchemaRef     = "extensions/accounting-asc350-60/crypto-assets.schema.json"
	rollForwardCalcType       = "cryptoAssetRollForward"
	rollForwardSchemaRef      = "extensions/accounting-asc350-60/crypto-asset-roll-forward.schema.json"
	significantHoldingsType   = "significantCryptoHoldings"
	significantHoldingsSchema = "extensions/accounting-asc350-60/significant-holdings.schema.json"
	scopeCriteriaCount        = 6
	rollForwardDefaultTol     = 0.01
)

// ASC35060 implements the vwp-accounting-asc350-60 extension validators:
// the scope register (criteria ⇔ inScope, out-of-scope assets judged and
// routed), every holdings roll-forward (each line and the totals tie, only
// in-scope assets), and every significant-holdings table (significant and
// other fair value AND cost basis recompute from the holdings, only
// in-scope assets, and per-asset fair value equals the roll-forward's
// closing balance for that asset).
func ASC35060(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	inScope := map[string]bool{}
	registerDoc, present := readJSONValue(pkg, cryptoAssetsPath)
	assets := getMaps(asMap(registerDoc), "assets")
	registerValid := false
	if !present || len(assets) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC35060ScopeRegisterPresent, Status: vwp.StatusFail, Message: cryptoAssetsPath + " missing or contains no assets"})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC35060ScopeRegisterPresent, Status: vwp.StatusPass, Message: strconv.Itoa(len(assets)) + " assets in scope register"})
		errs, err := schema.ValidateExtension(cryptoAssetsSchemaRef, registerDoc)
		if err != nil {
			errs = []string{err.Error()}
		}
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorASC35060ScopeSchemaValid, Status: vwp.StatusFail, Message: joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: ValidatorASC35060ScopeSchemaValid, Status: vwp.StatusPass})
			registerValid = true
		}
	}

	if registerValid {
		judgmentIds := idSet(ctx.Judgments)
		var inconsistent, unrouted, unjudged []string
		seenAsset := map[string]bool{}
		for _, a := range assets {
			id := getString(a, "id")
			symbol := getString(a, "asset")
			if seenAsset[symbol] {
				inconsistent = append(inconsistent, id+" (asset "+symbol+" registered twice)")
			}
			seenAsset[symbol] = true
			criteria := getMap(a, "criteria")
			allTrue := len(criteria) == scopeCriteriaCount
			for _, v := range criteria {
				if b, ok := v.(bool); !ok || !b {
					allTrue = false
				}
			}
			declared, _ := a["inScope"].(bool)
			if declared != allTrue {
				inconsistent = append(inconsistent, id)
			}
			if declared {
				inScope[symbol] = true
				inScope[id] = true
				continue
			}
			if strings.TrimSpace(getString(a, "outOfScopeTreatment")) == "" {
				unrouted = append(unrouted, id)
			}
			if ref := getString(a, "scopeJudgmentRef"); ref == "" || !judgmentIds[ref] {
				unjudged = append(unjudged, id)
			}
		}
		switch {
		case len(inconsistent) > 0:
			add(vwp.ValidationResult{ValidatorId: ValidatorASC35060ScopeConsistent, Status: vwp.StatusFail, Message: "inScope disagrees with the six criteria for: " + strings.Join(inconsistent, ", "), Refs: inconsistent})
		case len(unrouted) > 0:
			add(vwp.ValidationResult{ValidatorId: ValidatorASC35060ScopeConsistent, Status: vwp.StatusFail, Message: "out-of-scope assets without an outOfScopeTreatment: " + strings.Join(unrouted, ", "), Refs: unrouted})
		default:
			add(vwp.ValidationResult{ValidatorId: ValidatorASC35060ScopeConsistent, Status: vwp.StatusPass, Message: fmt.Sprintf("%d assets: inScope agrees with the criteria; out-of-scope assets name their treatment", len(assets))})
		}
		failList(add, ValidatorASC35060ScopeJudgmentLinked,
			"every out-of-scope asset links a resolvable scoping judgment",
			"out-of-scope assets without a resolvable scopeJudgmentRef: ", unjudged)
	} else {
		notRun(add, "requires a schema-valid "+cryptoAssetsPath, ValidatorASC35060ScopeConsistent, ValidatorASC35060ScopeJudgmentLinked)
	}

	// Roll-forwards: one per scope and period. Each keeps its own per-asset
	// closing balances, keyed by scope and period end, so a holdings table
	// reconciles to the schedule that closes on its date — consecutive
	// quarters roll into each other, they are never added together.
	type rfClose struct {
		path           string
		scope          string
		start, end     time.Time
		closingByAsset map[string]float64
		openingByAsset map[string]float64
		closingTotal   float64
	}
	var closes []rfClose
	rollForwardOK := false
	rfScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "scope") + "|" + getString(in, "periodStart") + "|" + getString(in, "periodEnd")
	}
	rollForwards, found := locateCalculations(pkg, add, ValidatorASC35060RollForwardSchemaValid, rollForwardCalcType, rollForwardSchemaRef, rfScope)
	if !found || len(rollForwards) == 0 {
		notRun(add, "no schema-valid roll-forward", ValidatorASC35060RollForwardTieOut, ValidatorASC35060RollForwardAssetsInScope)
	} else {
		var broken, outside []string
		cols := []string{"openingBalance", "additions", "dispositions", "gains", "losses", "closingBalance"}
		for _, rf := range rollForwards {
			pfx := rf.Path + ": "
			tol := tolerance(rf.Doc, "roundingTolerance", rollForwardDefaultTol)
			in := getMap(rf.Doc, "inputs")
			lines := getMaps(in, "lines")
			totals := getMap(getMap(rf.Doc, "outputs"), "totals")
			sums := map[string]float64{}
			seen := map[string]bool{}
			start, okS := parseWhen(getString(in, "periodStart"))
			end, okE := parseWhen(getString(in, "periodEnd"))
			if !okS || !okE || end.Before(start) {
				broken = append(broken, pfx+"periodStart/periodEnd are not an ordered date pair")
			}
			rc := rfClose{path: rf.Path, scope: getString(in, "scope"), start: start, end: end, closingByAsset: map[string]float64{}, openingByAsset: map[string]float64{}}
			for _, l := range lines {
				asset := getString(l, "asset")
				if seen[asset] {
					broken = append(broken, pfx+asset+" appears twice")
				}
				seen[asset] = true
				for _, c := range cols {
					sums[c] += jsNumber(l[c])
				}
				expect := jsNumber(l["openingBalance"]) + jsNumber(l["additions"]) - jsNumber(l["dispositions"]) + jsNumber(l["gains"]) - jsNumber(l["losses"])
				if !nearTol(expect, jsNumber(l["closingBalance"]), tol) {
					broken = append(broken, fmt.Sprintf("%s%s (expected closing %s, declared %s)", pfx, asset, num(expect), num(jsNumber(l["closingBalance"]))))
				}
				rc.closingByAsset[asset] = jsNumber(l["closingBalance"])
				rc.openingByAsset[asset] = jsNumber(l["openingBalance"])
				if registerValid && !inScope[asset] {
					outside = append(outside, pfx+asset)
				}
			}
			for _, c := range cols {
				if !nearTol(sums[c], jsNumber(totals[c]), tol) {
					broken = append(broken, fmt.Sprintf("%s%s (lines sum %s, declared %s)", pfx, c, num(sums[c]), num(jsNumber(totals[c]))))
				}
			}
			rc.closingTotal = jsNumber(totals["closingBalance"])
			closes = append(closes, rc)
		}
		// Consecutive periods of one scope must chain: the later period's
		// opening balance per asset is the earlier period's closing balance,
		// with no gap and no overlap between them.
		sort.Slice(closes, func(i, j int) bool {
			if closes[i].scope != closes[j].scope {
				return closes[i].scope < closes[j].scope
			}
			return closes[i].start.Before(closes[j].start)
		})
		for i := 1; i < len(closes); i++ {
			prev, cur := closes[i-1], closes[i]
			if prev.scope != cur.scope || prev.start.IsZero() || cur.start.IsZero() {
				continue
			}
			if periodsOverlap(prev.start, prev.end, cur.start, cur.end) {
				broken = append(broken, fmt.Sprintf("%s and %s overlap in time for scope %q", prev.path, cur.path, cur.scope))
				continue
			}
			if !cur.start.Equal(prev.end.AddDate(0, 0, 1)) {
				broken = append(broken, fmt.Sprintf("%s starts %s but %s ends %s: periods of one scope must be consecutive", cur.path, cur.start.Format("2006-01-02"), prev.path, prev.end.Format("2006-01-02")))
				continue
			}
			for asset, opening := range cur.openingByAsset {
				if closing := prev.closingByAsset[asset]; !nearTol(opening, closing, rollForwardDefaultTol) {
					broken = append(broken, fmt.Sprintf("%s opens %s at %s but %s closed it at %s", cur.path, asset, num(opening), prev.path, num(closing)))
				}
			}
		}
		if len(broken) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorASC35060RollForwardTieOut, Status: vwp.StatusFail, Message: "roll-forward does not tie: " + joinCapped(broken, 6)})
		} else {
			rollForwardOK = true
			add(vwp.ValidationResult{ValidatorId: ValidatorASC35060RollForwardTieOut, Status: vwp.StatusPass, Message: fmt.Sprintf("%d roll-forward(s) tie and chain by scope and period", len(rollForwards))})
		}
		if !registerValid {
			notRun(add, "requires a schema-valid "+cryptoAssetsPath, ValidatorASC35060RollForwardAssetsInScope)
		} else {
			failList(add, ValidatorASC35060RollForwardAssetsInScope, "every roll-forward line is an in-scope asset", "roll-forward lines for assets not in scope under ASC 350-60: ", outside)
		}
	}

	// Executable remeasurement entries: one per asset, period end and
	// direction, from each roll-forward line's gross gains and losses.
	// Additions and dispositions are the subledger's own transactions and
	// are not booked here.
	remeasure := scheduleExpectation{kind: accountingschedule.KindFairValueRemeasurement}
	for _, rf := range rollForwards {
		in := getMap(rf.Doc, "inputs")
		remeasure.sources++
		remeasure.currency(getString(in, "currency"))
		end, ok := isoDate(getString(in, "periodEnd"))
		if !ok {
			remeasure.problem(rf.Path + ": periodEnd is not a date")
			continue
		}
		for _, l := range getMaps(in, "lines") {
			asset := getString(l, "asset")
			gains, losses := jsNumber(l["gains"]), jsNumber(l["losses"])
			g := scheduleGroup{obligationId: asset, date: end, label: rf.Path + " " + asset}
			if gains > 0 {
				g.postings = append(g.postings, debit(gains, RoleCryptoAsset), credit(gains, RoleCryptoAssetGainLoss))
			}
			if losses > 0 {
				g.postings = append(g.postings, debit(losses, RoleCryptoAssetGainLoss), credit(losses, RoleCryptoAsset))
			}
			remeasure.group(g)
		}
	}
	add(tieOutAccountingSchedule(pkg, ValidatorASC35060AccountingScheduleTieOut, remeasure))

	// Significant holdings.
	shScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "scope") + "|" + getString(in, "asOf")
	}
	holdings, found := locateCalculations(pkg, add, ValidatorASC35060HoldingsSchemaValid, significantHoldingsType, significantHoldingsSchema, shScope)
	if !found || len(holdings) == 0 {
		notRun(add, "no schema-valid significant holdings table", ValidatorASC35060HoldingsTieOut, ValidatorASC35060HoldingsAssetsInScope, ValidatorASC35060HoldingsTiesToRollForward)
		return results
	}
	var tie, outside, mismatch []string
	var fvTotal float64
	for _, sh := range holdings {
		pfx := sh.Path + ": "
		tol := tolerance(sh.Doc, "roundingTolerance", rollForwardDefaultTol)
		inputs := getMap(sh.Doc, "inputs")
		outputs := getMap(sh.Doc, "outputs")
		total := jsNumber(inputs["totalFairValue"])
		fvByAsset := map[string]float64{}
		var sigFV, otherFV, sigCB, otherCB float64
		seen := map[string]bool{}
		for _, h := range getMaps(inputs, "holdings") {
			asset := getString(h, "asset")
			if seen[asset] {
				tie = append(tie, pfx+asset+" listed twice")
			}
			seen[asset] = true
			fv := jsNumber(h["fairValue"])
			cb := jsNumber(h["costBasis"])
			if s, _ := h["significant"].(bool); s {
				sigFV += fv
				sigCB += cb
			} else {
				otherFV += fv
				otherCB += cb
			}
			fvByAsset[asset] = fv
			if registerValid && !inScope[asset] {
				outside = append(outside, pfx+asset)
			}
		}
		fvTotal += total
		// Reconcile to the roll-forward of the same scope that closes on asOf.
		if rollForwardOK {
			asOf, okA := parseWhen(getString(inputs, "asOf"))
			var match *rfClose
			for i := range closes {
				if closes[i].scope == getString(inputs, "scope") && okA && closes[i].end.Equal(asOf) {
					match = &closes[i]
				}
			}
			switch {
			case !okA:
				mismatch = append(mismatch, pfx+"asOf is not a parseable date")
			case match == nil:
				mismatch = append(mismatch, fmt.Sprintf("%sno roll-forward for scope %q closes on %s", pfx, getString(inputs, "scope"), getString(inputs, "asOf")))
			default:
				for asset, fv := range fvByAsset {
					closing, ok := match.closingByAsset[asset]
					if !ok {
						mismatch = append(mismatch, pfx+asset+" is in the holdings table but has no line in "+match.path)
					} else if !nearTol(fv, closing, tol) {
						mismatch = append(mismatch, fmt.Sprintf("%s%s holdings fair value %s vs roll-forward closing %s", pfx, asset, num(fv), num(closing)))
					}
				}
				for asset, closing := range match.closingByAsset {
					if _, ok := fvByAsset[asset]; !ok && closing > tol {
						mismatch = append(mismatch, pfx+asset+" has a roll-forward closing balance but is not in the holdings table")
					}
				}
				if !nearTol(total, match.closingTotal, tol) {
					mismatch = append(mismatch, fmt.Sprintf("%stotal fair value %s does not equal roll-forward closing balance %s", pfx, num(total), num(match.closingTotal)))
				}
			}
		}
		for _, chk := range []struct {
			key  string
			want float64
		}{
			{"significantFairValue", sigFV}, {"otherFairValue", otherFV},
			{"significantCostBasis", sigCB}, {"otherCostBasis", otherCB},
		} {
			if got := jsNumber(outputs[chk.key]); !nearTol(got, chk.want, tol) {
				tie = append(tie, fmt.Sprintf("%s%s declared %s but the holdings sum to %s", pfx, chk.key, num(got), num(chk.want)))
			}
		}
		if !nearTol(sigFV+otherFV, total, tol) {
			tie = append(tie, fmt.Sprintf("%sholdings fair value %s does not equal totalFairValue %s", pfx, num(sigFV+otherFV), num(total)))
		}
	}
	if len(tie) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC35060HoldingsTieOut, Status: vwp.StatusFail, Message: joinCapped(tie, 6)})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC35060HoldingsTieOut, Status: vwp.StatusPass, Message: fmt.Sprintf("significant and other fair value and cost basis recompute from the holdings; total fair value %s", num(fvTotal))})
	}
	if !registerValid {
		notRun(add, "requires a schema-valid "+cryptoAssetsPath, ValidatorASC35060HoldingsAssetsInScope)
	} else {
		failList(add, ValidatorASC35060HoldingsAssetsInScope, "every holding is an in-scope asset", "holdings for assets not in scope under ASC 350-60: ", outside)
	}
	if !rollForwardOK {
		notRun(add, "requires a tied roll-forward to compare against", ValidatorASC35060HoldingsTiesToRollForward)
		return results
	}
	sortStrings(mismatch)
	if len(mismatch) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC35060HoldingsTiesToRollForward, Status: vwp.StatusFail, Message: joinCapped(mismatch, 6)})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorASC35060HoldingsTiesToRollForward, Status: vwp.StatusPass, Message: fmt.Sprintf("every holdings table reconciles per asset to the roll-forward closing on its date; total %s", num(fvTotal))})
	}
	return results
}
