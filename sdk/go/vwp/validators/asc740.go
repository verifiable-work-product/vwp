package validators

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// ASC 740 extension validator ids.
const (
	ValidatorASC740ProvisionSchemaValid              = "asc740.provision.schemaValid"
	ValidatorASC740ProvisionRecompute                = "asc740.provision.recompute"
	ValidatorASC740ProvisionRateReconciliation       = "asc740.provision.rateReconciliation"
	ValidatorASC740ProvisionDeferredRollForward      = "asc740.provision.deferredRollForward"
	ValidatorASC740ProvisionValuationAllowanceJudged = "asc740.provision.valuationAllowanceJudged"
	ValidatorASC740AccountingScheduleTieOut          = vwp.ValidatorASC740AccountingScheduleTieOut
)

// Data contract locations consumed by the ASC 740 validators.
const (
	incomeTaxProvisionType      = "incomeTaxProvision"
	incomeTaxProvisionSchemaRef = "extensions/accounting-asc740/income-tax-provision.schema.json"
	asc740DefaultTol            = 0.01
	asc740RateTol               = 1e-6
)

// Account roles the ASC 740 executable schedule uses. deferred_tax_asset is
// bound to the deferred tax asset NET of its valuation allowance (the
// schedule books the change in the net asset); unrecognized_tax_benefits
// to the liability for uncertain tax positions (ASC 740-10-45-11), which
// is not income taxes payable.
const (
	RoleIncomeTaxExpense        = "income_tax_expense"
	RoleIncomeTaxesPayable      = "income_taxes_payable"
	RoleDeferredTaxAsset        = "deferred_tax_asset"
	RoleDeferredTaxLiability    = "deferred_tax_liability"
	RoleUnrecognizedTaxBenefits = "unrecognized_tax_benefits"
)

// Fixed obligation ids of the provision's schedule groups.
const (
	asc740GroupCurrent  = "current"
	asc740GroupUTP      = "uncertainTaxPositions"
	asc740GroupDeferred = "deferred"
)

// ASC740 implements the vwp-accounting-asc740 extension validators over
// every income tax provision in the package (one per jurisdiction and
// period):
//
//   - recompute: taxable income = pretax income (equal to its fact) +
//     permanent differences + the period's change in deductible temporary
//     differences − the change in taxable ones; current = taxable income ×
//     statutory rate; the unrecognized-tax-benefit expense is the change in
//     that liability; deferred = net deferred (opening) − net deferred
//     (closing); total = current + UTB + deferred; effective rate = total /
//     pretax;
//   - deferredRollForward: gross deferred tax assets and liabilities at
//     each date recompute from the cumulative differences × rate
//     (ASC 740-10-30-8), the allowance never exceeds the gross asset, and
//     net deferred = assets − allowance − liabilities;
//   - rateReconciliation: a 'statutory' row equal to pretax × statutory
//     rate, no duplicate rows, and the rows sum to the total provision
//     (ASC 740-10-50-12);
//   - valuationAllowanceJudged: the more-likely-than-not judgment
//     (ASC 740-10-30-5(e)) resolves whenever an allowance exists, and the
//     uncertain-tax-position judgment (ASC 740-10-25-6) whenever one is
//     stated;
//   - accountingSchedule.tieOut: the executable schedule books, on the
//     period end, a 'current' group (income_tax_expense against
//     income_taxes_payable), an 'uncertainTaxPositions' group
//     (income_tax_expense against unrecognized_tax_benefits) and a
//     'deferred' group (the change in the net deferred tax asset, the change
//     in the deferred tax liability and the deferred expense that balances
//     them), each only when nonzero, for the recomputed amounts.
func ASC740(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	all := []string{ValidatorASC740ProvisionRecompute, ValidatorASC740ProvisionRateReconciliation, ValidatorASC740ProvisionDeferredRollForward, ValidatorASC740ProvisionValuationAllowanceJudged}

	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "jurisdiction") + "|" + getString(in, "periodEnd")
	}
	hits, found := locateCalculations(pkg, add, ValidatorASC740ProvisionSchemaValid, incomeTaxProvisionType, incomeTaxProvisionSchemaRef, scope)
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid income tax provision", all...)
		exp := scheduleExpectation{kind: accountingschedule.KindIncomeTaxProvision}
		exp.problem("no schema-valid income tax provision to derive the schedule from")
		add(tieOutAccountingSchedule(pkg, ValidatorASC740AccountingScheduleTieOut, exp))
		return results
	}

	facts := factIndex(ctx)
	judgmentIds := idSet(ctx.Judgments)
	exp := scheduleExpectation{kind: accountingschedule.KindIncomeTaxProvision}
	var recompute, rollForward, reconciliation, judged []string
	for _, hit := range hits {
		pfx := hit.Path + ": "
		tol := tolerance(hit.Doc, "roundingTolerance", asc740DefaultTol)
		in := getMap(hit.Doc, "inputs")
		out := getMap(hit.Doc, "outputs")
		exp.sources++
		exp.currency(getString(in, "currency"))
		periodEnd, okDate := isoDate(getString(in, "periodEnd"))
		if !okDate {
			exp.problem(pfx + "periodEnd is not a date")
		}

		rate := jsNumber(in["statutoryRate"])
		pretax := jsNumber(in["pretaxIncome"])
		if msg := checkFactValue(facts, getString(in, "pretaxIncomeFact"), pretax, tol, pfx+"pretaxIncome", "pretaxIncome", "amount", "income"); msg != "" {
			recompute = append(recompute, msg)
		}
		if ref := getString(in, "statutoryRateFact"); ref != "" {
			if msg := checkFactValue(facts, ref, rate, asc740RateTol, pfx+"statutoryRate", "rate", "statutoryRate"); msg != "" {
				recompute = append(recompute, msg)
			}
		}

		// Taxable income.
		taxable := pretax
		seenIds := map[string]bool{}
		for _, p := range getMaps(in, "permanentDifferences") {
			id := getString(p, "id")
			if seenIds[id] {
				recompute = append(recompute, pfx+"permanent difference "+id+" listed twice")
			}
			seenIds[id] = true
			taxable += jsNumber(p["amount"])
		}
		var dtaOpen, dtaClose, dtlOpen, dtlClose float64
		for _, t := range getMaps(in, "temporaryDifferences") {
			id := getString(t, "id")
			if seenIds[id] {
				recompute = append(recompute, pfx+"temporary difference "+id+" listed twice")
			}
			seenIds[id] = true
			opening, closing, r := jsNumber(t["openingDifference"]), jsNumber(t["closingDifference"]), jsNumber(t["rate"])
			switch getString(t, "kind") {
			case "deductible":
				taxable += closing - opening
				dtaOpen += opening * r
				dtaClose += closing * r
			case "taxable":
				taxable -= closing - opening
				dtlOpen += opening * r
				dtlClose += closing * r
			}
		}
		current := taxable * rate
		va := getMap(in, "valuationAllowance")
		vaOpen, vaClose := jsNumber(va["opening"]), jsNumber(va["closing"])
		netOpen := dtaOpen - vaOpen - dtlOpen
		netClose := dtaClose - vaClose - dtlClose
		deferred := netOpen - netClose
		utp := getMap(in, "uncertainTaxPositions")
		utpExpense := 0.0
		if len(utp) > 0 {
			utpExpense = jsNumber(utp["closing"]) - jsNumber(utp["opening"])
		}
		total := current + utpExpense + deferred

		check := func(list *[]string, key string, want float64, t float64) {
			if got := jsNumber(out[key]); !nearTol(got, want, t) {
				*list = append(*list, fmt.Sprintf("%s%s declared %s but recomputes to %s", pfx, key, num(got), num(want)))
			}
		}
		check(&recompute, "taxableIncome", taxable, tol)
		check(&recompute, "currentTaxExpense", current, tol)
		if len(utp) > 0 || out["uncertainTaxPositionsExpense"] != nil {
			check(&recompute, "uncertainTaxPositionsExpense", utpExpense, tol)
		}
		check(&recompute, "deferredTaxExpense", deferred, tol)
		check(&recompute, "totalTaxExpense", total, tol)
		if pretax != 0 {
			check(&recompute, "effectiveRate", total/pretax, asc740RateTol)
		}

		// Deferred balances.
		check(&rollForward, "deferredTaxAssetOpening", dtaOpen, tol)
		check(&rollForward, "deferredTaxAssetClosing", dtaClose, tol)
		check(&rollForward, "deferredTaxLiabilityOpening", dtlOpen, tol)
		check(&rollForward, "deferredTaxLiabilityClosing", dtlClose, tol)
		if out["netDeferredOpening"] != nil {
			check(&rollForward, "netDeferredOpening", netOpen, tol)
		}
		if out["netDeferredClosing"] != nil {
			check(&rollForward, "netDeferredClosing", netClose, tol)
		}
		if vaOpen > dtaOpen+tol || vaClose > dtaClose+tol {
			rollForward = append(rollForward, fmt.Sprintf("%svaluation allowance (%s opening, %s closing) exceeds the gross deferred tax asset (%s, %s)", pfx, num(vaOpen), num(vaClose), num(dtaOpen), num(dtaClose)))
		}

		// Rate reconciliation (ASC 740-10-50-12).
		var sum float64
		statutoryRows := 0
		seenRows := map[string]bool{}
		for _, row := range getMaps(out, "rateReconciliation") {
			item := strings.TrimSpace(getString(row, "item"))
			if seenRows[item] {
				reconciliation = append(reconciliation, pfx+"rate reconciliation row "+strconv.Quote(item)+" appears twice")
			}
			seenRows[item] = true
			amount := jsNumber(row["amount"])
			sum += amount
			if strings.EqualFold(item, "statutory") {
				statutoryRows++
				if !nearTol(amount, pretax*rate, tol) {
					reconciliation = append(reconciliation, fmt.Sprintf("%sstatutory row %s but pretax %s × rate %s = %s", pfx, num(amount), num(pretax), num(rate), num(pretax*rate)))
				}
			}
		}
		if statutoryRows != 1 {
			reconciliation = append(reconciliation, pfx+"rate reconciliation needs exactly one 'statutory' row")
		}
		if !nearTol(sum, total, tol) {
			reconciliation = append(reconciliation, fmt.Sprintf("%srate reconciliation rows sum to %s but the total provision is %s", pfx, num(sum), num(total)))
		}

		// Judgments.
		if ref := getString(va, "judgmentRef"); ref != "" {
			if !judgmentIds[ref] {
				judged = append(judged, pfx+"valuationAllowance.judgmentRef "+strconv.Quote(ref)+" does not resolve")
			}
		} else if vaOpen > tol || vaClose > tol {
			judged = append(judged, pfx+"a valuation allowance is carried without a realizability judgment (ASC 740-10-30-5(e))")
		}
		if len(utp) > 0 {
			if ref := getString(utp, "judgmentRef"); ref == "" || !judgmentIds[ref] {
				judged = append(judged, pfx+"uncertainTaxPositions.judgmentRef "+strconv.Quote(ref)+" does not resolve (ASC 740-10-25-6)")
			}
		}

		// Executable entries on the period end, for the recomputed amounts.
		if okDate {
			if math.Abs(current) >= tol {
				g := scheduleGroup{obligationId: asc740GroupCurrent, date: periodEnd, label: pfx + "current tax"}
				if current > 0 {
					g.postings = []schedulePosting{debit(current, RoleIncomeTaxExpense), credit(current, RoleIncomeTaxesPayable)}
				} else {
					g.postings = []schedulePosting{debit(-current, RoleIncomeTaxesPayable), credit(-current, RoleIncomeTaxExpense)}
				}
				exp.group(g)
			}
			if math.Abs(utpExpense) >= tol {
				g := scheduleGroup{obligationId: asc740GroupUTP, date: periodEnd, label: pfx + "unrecognized tax benefits"}
				if utpExpense > 0 {
					g.postings = []schedulePosting{debit(utpExpense, RoleIncomeTaxExpense), credit(utpExpense, RoleUnrecognizedTaxBenefits)}
				} else {
					g.postings = []schedulePosting{debit(-utpExpense, RoleUnrecognizedTaxBenefits), credit(-utpExpense, RoleIncomeTaxExpense)}
				}
				exp.group(g)
			}
			dDTA := (dtaClose - vaClose) - (dtaOpen - vaOpen)
			dDTL := dtlClose - dtlOpen
			var postings []schedulePosting
			switch {
			case dDTA >= tol:
				postings = append(postings, debit(dDTA, RoleDeferredTaxAsset))
			case dDTA <= -tol:
				postings = append(postings, credit(-dDTA, RoleDeferredTaxAsset))
			}
			switch {
			case dDTL >= tol:
				postings = append(postings, credit(dDTL, RoleDeferredTaxLiability))
			case dDTL <= -tol:
				postings = append(postings, debit(-dDTL, RoleDeferredTaxLiability))
			}
			switch {
			case deferred >= tol:
				postings = append(postings, debit(deferred, RoleIncomeTaxExpense))
			case deferred <= -tol:
				postings = append(postings, credit(-deferred, RoleIncomeTaxExpense))
			}
			if len(postings) > 0 {
				exp.group(scheduleGroup{obligationId: asc740GroupDeferred, date: periodEnd, label: pfx + "deferred tax", postings: postings})
			}
		}
	}

	emit := func(id, passMsg, failPrefix string, errs []string) {
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusFail, Message: failPrefix + joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusPass, Message: passMsg})
		}
	}
	emit(ValidatorASC740ProvisionRecompute, fmt.Sprintf("%d provision(s): taxable income, current, deferred and total tax recompute from the differences and the pretax income fact", len(hits)), "provision does not recompute: ", recompute)
	emit(ValidatorASC740ProvisionDeferredRollForward, "deferred tax assets and liabilities recompute from the cumulative differences at the enacted rates; the allowance never exceeds the gross asset", "deferred balances do not recompute: ", rollForward)
	emit(ValidatorASC740ProvisionRateReconciliation, "the rate reconciliation starts from the statutory expectation and sums to the total provision", "rate reconciliation does not tie: ", reconciliation)
	emit(ValidatorASC740ProvisionValuationAllowanceJudged, "valuation allowance and uncertain tax position conclusions rest on resolvable judgments", "", judged)
	if len(recompute) > 0 {
		exp.problem("the provision does not recompute")
	}
	add(tieOutAccountingSchedule(pkg, ValidatorASC740AccountingScheduleTieOut, exp))
	return results
}
