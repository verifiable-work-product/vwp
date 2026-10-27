package validators

import (
	"fmt"
	"math"
	"sort"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// Reconciliation extension validator ids.
const (
	ValidatorReconciliationBalanceSchemaValid       = "reconciliation.balance.schemaValid"
	ValidatorReconciliationBalanceRecompute         = "reconciliation.balance.recompute"
	ValidatorReconciliationBalanceItemsSupported    = "reconciliation.balance.itemsSupported"
	ValidatorReconciliationFluxSchemaValid          = "reconciliation.flux.schemaValid"
	ValidatorReconciliationFluxRecompute            = "reconciliation.flux.recompute"
	ValidatorReconciliationFluxVariancesExplained   = "reconciliation.flux.variancesExplained"
	ValidatorReconciliationAccountingScheduleTieOut = vwp.ValidatorReconciliationAccountingScheduleTieOut
)

// Data contract locations consumed by the Reconciliation validators.
const (
	balanceReconciliationType      = "balanceReconciliation"
	balanceReconciliationSchemaRef = "extensions/accounting-reconciliation/balance-reconciliation.schema.json"
	fluxAnalysisType               = "fluxAnalysis"
	fluxAnalysisSchemaRef          = "extensions/accounting-reconciliation/flux-analysis.schema.json"
	reconciliationDefaultTol       = 0.01
)

// bookableCategories are the reconciling items the ledger must record: a
// timing difference clears itself, an error or an omission does not.
var bookableCategories = map[string]bool{"error": true, "omission": true}

// Reconciliation implements the vwp-accounting-reconciliation extension
// validators over every balance reconciliation (one per scope and date) and
// every flux analysis (one per pair of period ends) in the package:
//
//   - balance.recompute: difference = source − ledger, explained = Σ items
//     with status explained, unexplained = difference − explained; the
//     source and ledger balances equal the values of the facts they cite,
//     and those facts are stated as of the reconciliation date;
//   - balance.itemsSupported: every item cites a supportRef that resolves
//     in the package; every error or omission is bookable; every bookable
//     item declares both roles; an unexplained residual needs a judgment
//     that names the scope;
//   - flux.recompute: variance = current − prior, variancePct = variance /
//     |prior| (null when prior is zero), totals sum;
//   - flux.variancesExplained: every line whose variance exceeds either
//     threshold names an explanation claim present in the package;
//   - accountingSchedule.tieOut: the executable schedule books one
//     reconciling_adjustment group per bookable item on the reconciliation
//     date — the item's debitRole against its creditRole for |amount| — and
//     nothing else. A package with no bookable item needs no schedule.
func Reconciliation(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	facts := factIndex(ctx)
	supportIds := supportIndex(pkg, ctx)
	judgments := ctx.Judgments
	claimIds := idSet(ctx.Claims)

	// --- balance reconciliations ---
	balScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "scope") + "|" + getString(in, "asOf")
	}
	exp := scheduleExpectation{kind: accountingschedule.KindReconcilingAdjustment}
	balances, found := locateCalculations(pkg, add, ValidatorReconciliationBalanceSchemaValid, balanceReconciliationType, balanceReconciliationSchemaRef, balScope)
	if !found || len(balances) == 0 {
		notRun(add, "no schema-valid balance reconciliation", ValidatorReconciliationBalanceRecompute, ValidatorReconciliationBalanceItemsSupported)
	} else {
		var recompute, support []string
		for _, hit := range balances {
			pfx := hit.Path + ": "
			tol := tolerance(hit.Doc, "roundingTolerance", reconciliationDefaultTol)
			in := getMap(hit.Doc, "inputs")
			out := getMap(hit.Doc, "outputs")
			scope := getString(in, "scope")
			asOf, okDate := parseWhen(getString(in, "asOf"))
			asOfISO, _ := isoDate(getString(in, "asOf"))
			exp.sources++
			exp.currency(getString(in, "currency"))
			if !okDate {
				recompute = append(recompute, pfx+"asOf is not a parseable date")
			}

			source := jsNumber(in["sourceBalance"])
			ledger := jsNumber(in["ledgerBalance"])
			difference := source - ledger
			var explained float64
			seen := map[string]bool{}
			for _, item := range getMaps(in, "items") {
				id := getString(item, "id")
				label := pfx + "item " + id
				if seen[id] {
					support = append(support, label+" appears twice")
					continue
				}
				seen[id] = true
				amount := jsNumber(item["amount"])
				if getString(item, "status") == "explained" {
					explained += amount
				}
				category := getString(item, "category")
				bookable, _ := item["bookable"].(bool)
				debitRole, creditRole := getString(item, "debitRole"), getString(item, "creditRole")
				if bookableCategories[category] && !bookable {
					support = append(support, fmt.Sprintf("%s is an %s but is not bookable: the ledger must record it", label, category))
				}
				if bookable && (debitRole == "" || creditRole == "") {
					support = append(support, label+" is bookable but declares no debitRole/creditRole")
				}
				if ref := getString(item, "supportRef"); ref == "" {
					support = append(support, label+" cites no supportRef")
				} else if !supportIds[ref] {
					support = append(support, fmt.Sprintf("%s cites %q which is not a fact, evidence or calculation in the package", label, ref))
				}
				if bookable && debitRole != "" && creditRole != "" && okDate {
					exp.group(scheduleGroup{obligationId: id, date: asOfISO, label: label,
						postings: []schedulePosting{debit(math.Abs(amount), debitRole), credit(math.Abs(amount), creditRole)}})
				}
			}
			unexplained := difference - explained
			for _, chk := range []struct {
				key  string
				want float64
			}{{"difference", difference}, {"explained", explained}, {"unexplained", unexplained}} {
				if got := jsNumber(out[chk.key]); !nearTol(got, chk.want, tol) {
					recompute = append(recompute, fmt.Sprintf("%s%s declared %s but recomputes to %s", pfx, chk.key, num(got), num(chk.want)))
				}
			}
			// The balances are what their facts say, as of the reconciliation date.
			for _, chk := range []struct {
				label, ref string
				want       float64
			}{{"sourceBalance", getString(in, "sourceFact"), source}, {"ledgerBalance", getString(in, "ledgerFact"), ledger}} {
				if msg := checkFactValue(facts, chk.ref, chk.want, tol, pfx+chk.label, "amount", "balance", "units", "quantity"); msg != "" {
					recompute = append(recompute, msg)
				} else if fd, ok := factDate(facts[chk.ref]); !ok {
					recompute = append(recompute, fmt.Sprintf("%s%s cites fact %q which states no date (value needs a date)", pfx, chk.label, chk.ref))
				} else if okDate && !fd.Equal(asOf) {
					recompute = append(recompute, fmt.Sprintf("%s%s cites fact %q stated as of %s, not the reconciliation date %s", pfx, chk.label, chk.ref, fd.Format("2006-01-02"), asOf.Format("2006-01-02")))
				}
			}
			// An unexplained residual is a finding a human must own.
			if math.Abs(unexplained) > tol && !judgmentMentions(judgments, scope) {
				support = append(support, fmt.Sprintf("%sunexplained residual %s but no judgment names scope %q", pfx, num(unexplained), scope))
			}
		}
		if len(recompute) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationBalanceRecompute, Status: vwp.StatusFail, Message: "reconciliation does not recompute: " + joinCapped(recompute, 6)})
			exp.problem("the balance reconciliation does not recompute")
		} else {
			add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationBalanceRecompute, Status: vwp.StatusPass, Message: fmt.Sprintf("%d reconciliation(s): difference, explained and unexplained recompute from balances their facts state as of the reconciliation date", len(balances))})
		}
		if len(support) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationBalanceItemsSupported, Status: vwp.StatusFail, Message: "reconciling items are not supported: " + joinCapped(support, 6)})
			exp.problem("reconciling items are not supported")
		} else {
			add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationBalanceItemsSupported, Status: vwp.StatusPass, Message: "every reconciling item is supported, errors and omissions are bookable with roles, and any unexplained residual is judged"})
		}
	}
	add(tieOutAccountingSchedule(pkg, ValidatorReconciliationAccountingScheduleTieOut, exp))

	// --- flux analyses ---
	fluxScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "priorPeriodEnd") + "|" + getString(in, "currentPeriodEnd")
	}
	fluxes, found := locateCalculations(pkg, add, ValidatorReconciliationFluxSchemaValid, fluxAnalysisType, fluxAnalysisSchemaRef, fluxScope)
	if !found || len(fluxes) == 0 {
		notRun(add, "no schema-valid flux analysis", ValidatorReconciliationFluxRecompute, ValidatorReconciliationFluxVariancesExplained)
		return results
	}
	var recompute, unexplained []string
	for _, hit := range fluxes {
		pfx := hit.Path + ": "
		tol := tolerance(hit.Doc, "roundingTolerance", reconciliationDefaultTol)
		in := getMap(hit.Doc, "inputs")
		out := getMap(hit.Doc, "outputs")
		prior, okP := parseWhen(getString(in, "priorPeriodEnd"))
		current, okC := parseWhen(getString(in, "currentPeriodEnd"))
		if !okP || !okC || !current.After(prior) {
			recompute = append(recompute, pfx+"priorPeriodEnd/currentPeriodEnd are not an ordered date pair")
		}
		thresholdAmount := jsNumber(in["thresholdAmount"])
		thresholdPct := jsNumber(in["thresholdPct"])
		rows, dups := indexByID(getMaps(out, "rows"), "account")
		for _, d := range dups {
			recompute = append(recompute, pfx+"duplicate output row for account "+d)
		}
		seen := map[string]bool{}
		var totalPrior, totalCurrent float64
		for _, l := range getMaps(in, "lines") {
			account := getString(l, "account")
			label := pfx + account
			if seen[account] {
				recompute = append(recompute, label+" listed twice")
				continue
			}
			seen[account] = true
			p, c := jsNumber(l["prior"]), jsNumber(l["current"])
			totalPrior += p
			totalCurrent += c
			variance := c - p
			row, ok := rows[account]
			if !ok {
				recompute = append(recompute, label+" (no output row)")
			} else {
				if got := jsNumber(row["variance"]); !nearTol(got, variance, tol) {
					recompute = append(recompute, fmt.Sprintf("%s: variance declared %s but current %s − prior %s = %s", label, num(got), num(c), num(p), num(variance)))
				}
				if p == 0 {
					if row["variancePct"] != nil {
						recompute = append(recompute, label+": variancePct must be null when the prior balance is zero")
					}
				} else if got := jsNumber(row["variancePct"]); !nearTol(got, variance/math.Abs(p), 1e-6) {
					recompute = append(recompute, fmt.Sprintf("%s: variancePct declared %s but recomputes to %s", label, num(got), num(variance/math.Abs(p))))
				}
			}
			above := math.Abs(variance) > thresholdAmount+tol || (p != 0 && math.Abs(variance/math.Abs(p)) > thresholdPct+1e-9)
			if above {
				if ref := getString(l, "explanationClaim"); ref == "" {
					unexplained = append(unexplained, fmt.Sprintf("%s: variance %s exceeds a threshold but names no explanationClaim", label, num(variance)))
				} else if !claimIds[ref] {
					unexplained = append(unexplained, fmt.Sprintf("%s: explanationClaim %q is not a claim in the package", label, ref))
				}
			}
		}
		for account := range rows {
			if !seen[account] {
				recompute = append(recompute, pfx+"output row for account "+account+" which is not an input line")
			}
		}
		if got := jsNumber(out["totalPrior"]); !nearTol(got, totalPrior, tol) {
			recompute = append(recompute, fmt.Sprintf("%stotalPrior declared %s but the lines sum to %s", pfx, num(got), num(totalPrior)))
		}
		if got := jsNumber(out["totalCurrent"]); !nearTol(got, totalCurrent, tol) {
			recompute = append(recompute, fmt.Sprintf("%stotalCurrent declared %s but the lines sum to %s", pfx, num(got), num(totalCurrent)))
		}
	}
	sort.Strings(unexplained)
	if len(recompute) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationFluxRecompute, Status: vwp.StatusFail, Message: "flux analysis does not recompute: " + joinCapped(recompute, 6)})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationFluxRecompute, Status: vwp.StatusPass, Message: fmt.Sprintf("%d flux analysis(es): variances, percentages and totals recompute from the lines", len(fluxes))})
	}
	if len(unexplained) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationFluxVariancesExplained, Status: vwp.StatusFail, Message: "variances above threshold without an explanation claim: " + joinCapped(unexplained, 6)})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorReconciliationFluxVariancesExplained, Status: vwp.StatusPass, Message: "every variance above either threshold is explained by a claim in the package"})
	}
	return results
}

// supportIndex is the set of ids a reconciling item may cite as support:
// every fact and evidence id the core parsed, plus every calculation id
// under calculations/.
func supportIndex(pkg vwp.Source, ctx *vwp.ExtensionContext) map[string]bool {
	ids := idSet(ctx.Facts)
	for id := range idSet(ctx.Evidence) {
		ids[id] = true
	}
	for _, p := range pkg.List() {
		if !strings.HasPrefix(p, "calculations/") || !strings.HasSuffix(p, ".json") {
			continue
		}
		if doc, ok := readJSONValue(pkg, p); ok {
			if id := getString(asMap(doc), "id"); id != "" {
				ids[id] = true
			}
		}
	}
	return ids
}

// judgmentMentions reports whether any judgment names the text in its topic
// or question (case-insensitive).
func judgmentMentions(judgments []map[string]any, text string) bool {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return false
	}
	for _, j := range judgments {
		if strings.Contains(strings.ToLower(getString(j, "topic")), needle) || strings.Contains(strings.ToLower(getString(j, "question")), needle) {
			return true
		}
	}
	return false
}
