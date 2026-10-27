package validators

import (
	"fmt"
	"math"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// ASC 842 extension validator ids.
const (
	ValidatorASC842LeaseSchemaValid          = "asc842.lease.schemaValid"
	ValidatorASC842LeasePvRecompute          = "asc842.lease.pvRecompute"
	ValidatorASC842LeaseRollForward          = "asc842.lease.rollForward"
	ValidatorASC842LeaseClassificationJudged = "asc842.lease.classificationJudged"
	ValidatorASC842LeaseRateEvidence         = "asc842.lease.rateEvidence"
	ValidatorASC842AccountingScheduleTieOut  = vwp.ValidatorASC842AccountingScheduleTieOut
)

// Data contract locations consumed by the ASC 842 validators.
const (
	leaseScheduleType      = "leaseSchedule"
	leaseScheduleSchemaRef = "extensions/accounting-asc842/lease-schedule.schema.json"
	leaseDefaultTol        = 0.01
)

// Account roles the ASC 842 executable schedule (kind lease_cost) uses.
// lease_payable is the obligation to pay each period's payment (cash or
// accounts payable); lease_liability is the discounted ASC 842 liability.
const (
	RoleLeaseExpense           = "lease_expense"
	RoleLeaseInterestExpense   = "interest_expense"
	RoleRouAmortizationExpense = "amortization_expense"
	RoleLeaseLiability         = "lease_liability"
	RoleRouAsset               = "rou_asset"
	RoleLeasePayable           = "lease_payable"
)

// ASC842 implements the vwp-accounting-asc842 extension validators over
// every lease schedule in the package (one per lease):
//
//   - pvRecompute: the initial liability is the present value of the
//     payments at discountRate / periodsPerYear with the declared timing
//     (ASC 842-20-30-1), and the right-of-use asset is that liability plus
//     prepaid payments and initial direct costs less incentives received
//     (842-20-30-5);
//   - rollForward: each period's interest is the opening liability (less
//     the payment, when paid in advance) times the per-period rate,
//     principal = payment − interest, closing = opening − principal; the
//     final period retires the liability exactly (its principal is the
//     remaining balance and its interest absorbs the accumulated rounding,
//     bounded by one rounding unit per period); an operating lease's cost
//     is the straight-line single lease cost (842-20-25-6) and its ROU
//     amortization the plug (842-20-35-3), a finance lease's ROU
//     amortization is straight-line over the term (842-20-35-7) and its
//     lease cost is interest plus amortization;
//   - classificationJudged: the classification rests on a judgment that
//     exists in the package (842-10-25-2 is judged, never computed);
//   - rateEvidence: the discount rate equals the value of the fact it cites;
//   - accountingSchedule.tieOut: the executable schedule books the
//     commencement entry and one lease_cost group per period from these
//     figures, under the classification's roles.
func ASC842(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	all := []string{ValidatorASC842LeasePvRecompute, ValidatorASC842LeaseRollForward, ValidatorASC842LeaseClassificationJudged, ValidatorASC842LeaseRateEvidence}

	scope := func(doc map[string]any) string {
		return getString(getMap(doc, "inputs"), "leaseId")
	}
	hits, found := locateCalculations(pkg, add, ValidatorASC842LeaseSchemaValid, leaseScheduleType, leaseScheduleSchemaRef, scope)
	exp := scheduleExpectation{kind: accountingschedule.KindLeaseCost}
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid lease schedule", all...)
		add(tieOutAccountingSchedule(pkg, ValidatorASC842AccountingScheduleTieOut, exp))
		return results
	}

	facts := factIndex(ctx)
	judgmentIds := idSet(ctx.Judgments)
	var pvErrs, rollErrs, judgeErrs, rateErrs []string
	for _, hit := range hits {
		pfx := hit.Path + ": "
		tol := tolerance(hit.Doc, "roundingTolerance", leaseDefaultTol)
		in := getMap(hit.Doc, "inputs")
		out := getMap(hit.Doc, "outputs")
		leaseId := getString(in, "leaseId")
		exp.sources++
		exp.currency(getString(in, "currency"))

		classification := getString(in, "classification")
		if ref := getString(in, "classificationJudgmentRef"); !judgmentIds[ref] {
			judgeErrs = append(judgeErrs, fmt.Sprintf("%sclassificationJudgmentRef %q is not a judgment in the package", pfx, ref))
		}
		rate := jsNumber(in["discountRate"])
		if msg := checkFactValue(facts, getString(in, "rateFact"), rate, 1e-9, pfx+"discountRate", "discountRate", "rate", "annualRate", "incrementalBorrowingRate"); msg != "" {
			rateErrs = append(rateErrs, msg)
		}
		perYear := jsNumber(in["periodsPerYear"])
		if math.IsNaN(perYear) || perYear <= 0 {
			perYear = 12
		}
		r := rate / perYear
		advance := getString(in, "paymentTiming") == "advance"
		payments := getMaps(in, "payments")
		idc := zeroIfNaN(jsNumber(in["initialDirectCosts"]))
		incentives := zeroIfNaN(jsNumber(in["leaseIncentives"]))
		prepaid := zeroIfNaN(jsNumber(in["prepaidLeasePayments"]))

		// Present value at commencement.
		var pv, totalPayments float64
		for i, p := range payments {
			amount := jsNumber(p["amount"])
			totalPayments += amount
			periods := float64(i + 1)
			if advance {
				periods = float64(i)
			}
			pv += amount / math.Pow(1+r, periods)
		}
		liability := jsNumber(out["initialLiability"])
		rou := jsNumber(out["initialRouAsset"])
		if !nearTol(liability, pv, tol) {
			pvErrs = append(pvErrs, fmt.Sprintf("%sinitialLiability %s but the %d payments discounted at %s per period (%s) have a present value of %s", pfx, num(liability), len(payments), num(r), getString(in, "paymentTiming"), num(pv)))
		}
		if wantRou := liability + idc + prepaid - incentives; !nearTol(rou, wantRou, tol) {
			pvErrs = append(pvErrs, fmt.Sprintf("%sinitialRouAsset %s but liability %s + initial direct costs %s + prepaid %s − incentives %s = %s", pfx, num(rou), num(liability), num(idc), num(prepaid), num(incentives), num(wantRou)))
		}

		// Commencement entry.
		commencement, okC := isoDate(getString(in, "commencementDate"))
		if !okC {
			exp.problem(pfx + "commencementDate is not a date")
		} else {
			g := scheduleGroup{obligationId: leaseId, date: commencement, label: pfx + "commencement",
				postings: []schedulePosting{debit(rou, RoleRouAsset), credit(liability, RoleLeaseLiability)}}
			if idc+prepaid > 0 {
				g.postings = append(g.postings, credit(idc+prepaid, RoleLeasePayable))
			}
			if incentives > 0 {
				g.postings = append(g.postings, debit(incentives, RoleLeasePayable))
			}
			exp.group(g)
		}

		// Roll-forward.
		periods := getMaps(out, "periods")
		n := len(periods)
		if n != len(payments) {
			rollErrs = append(rollErrs, fmt.Sprintf("%s%d payments but %d periods", pfx, len(payments), n))
		}
		straightLine := (totalPayments + idc - incentives) / float64(max(n, 1))
		if declared, ok := out["straightLineCost"]; ok && classification == "operating" && !nearTol(jsNumber(declared), straightLine, tol) {
			rollErrs = append(rollErrs, fmt.Sprintf("%sstraightLineCost %s but (payments %s + initial direct costs %s − incentives %s) / %d = %s", pfx, num(jsNumber(declared)), num(totalPayments), num(idc), num(incentives), n, num(straightLine)))
		}
		financeAmort := rou / float64(max(n, 1))
		expectedOpening := liability
		for i, p := range periods {
			label := fmt.Sprintf("%speriod %d (%s)", pfx, i+1, getString(p, "periodEnd"))
			opening := jsNumber(p["openingLiability"])
			payment := jsNumber(p["payment"])
			interest := jsNumber(p["interest"])
			principal := jsNumber(p["principal"])
			closing := jsNumber(p["closingLiability"])
			leaseCost := jsNumber(p["leaseCost"])
			amort := jsNumber(p["rouAmortization"])
			last := i == n-1

			if !nearTol(opening, expectedOpening, tol) {
				rollErrs = append(rollErrs, fmt.Sprintf("%s: openingLiability %s but the prior period closed at %s", label, num(opening), num(expectedOpening)))
			}
			if i < len(payments) && !nearTol(payment, jsNumber(payments[i]["amount"]), tol) {
				rollErrs = append(rollErrs, fmt.Sprintf("%s: payment %s but inputs.payments[%d] is %s", label, num(payment), i, num(jsNumber(payments[i]["amount"]))))
			}
			base := opening
			if advance {
				base = opening - payment
			}
			wantInterest := base * r
			if last {
				// The final period retires the liability exactly; its interest
				// absorbs the rounding the earlier periods accumulated.
				if !nearTol(closing, 0, tol) {
					rollErrs = append(rollErrs, fmt.Sprintf("%s: the final period must close the liability at zero, not %s", label, num(closing)))
				}
				if !nearTol(principal, opening, tol) {
					rollErrs = append(rollErrs, fmt.Sprintf("%s: final principal %s must equal the opening liability %s", label, num(principal), num(opening)))
				}
				if !nearTol(interest, wantInterest, tol*float64(n)) {
					rollErrs = append(rollErrs, fmt.Sprintf("%s: final interest %s is further than the accumulated rounding allowance from %s × %s = %s", label, num(interest), num(base), num(r), num(wantInterest)))
				}
			} else if !nearTol(interest, wantInterest, tol) {
				rollErrs = append(rollErrs, fmt.Sprintf("%s: interest %s but %s × %s = %s", label, num(interest), num(base), num(r), num(wantInterest)))
			}
			if !nearTol(principal, payment-interest, tol) {
				rollErrs = append(rollErrs, fmt.Sprintf("%s: principal %s but payment %s − interest %s = %s", label, num(principal), num(payment), num(interest), num(payment-interest)))
			}
			if !nearTol(closing, opening-principal, tol) {
				rollErrs = append(rollErrs, fmt.Sprintf("%s: closingLiability %s but opening %s − principal %s = %s", label, num(closing), num(opening), num(principal), num(opening-principal)))
			}
			expectedOpening = closing

			switch classification {
			case "operating":
				if !nearTol(leaseCost, straightLine, tol) {
					rollErrs = append(rollErrs, fmt.Sprintf("%s: leaseCost %s but the straight-line single lease cost is %s", label, num(leaseCost), num(straightLine)))
				}
				if !nearTol(amort, leaseCost-interest, tol) {
					rollErrs = append(rollErrs, fmt.Sprintf("%s: rouAmortization %s but lease cost %s − interest %s = %s", label, num(amort), num(leaseCost), num(interest), num(leaseCost-interest)))
				}
			case "finance":
				if !nearTol(amort, financeAmort, tol) {
					rollErrs = append(rollErrs, fmt.Sprintf("%s: rouAmortization %s but the right-of-use asset %s over %d periods is %s", label, num(amort), num(rou), n, num(financeAmort)))
				}
				if !nearTol(leaseCost, interest+amort, tol) {
					rollErrs = append(rollErrs, fmt.Sprintf("%s: leaseCost %s but interest %s + amortization %s = %s", label, num(leaseCost), num(interest), num(amort), num(interest+amort)))
				}
			}

			date, ok := isoDate(getString(p, "periodEnd"))
			if !ok {
				exp.problem(label + ": periodEnd is not a date")
				continue
			}
			g := scheduleGroup{obligationId: leaseId, date: date, label: label}
			switch classification {
			case "operating":
				g.postings = []schedulePosting{debit(leaseCost, RoleLeaseExpense), debit(principal, RoleLeaseLiability), credit(payment, RoleLeasePayable), credit(amort, RoleRouAsset)}
			default:
				g.postings = []schedulePosting{debit(interest, RoleLeaseInterestExpense), debit(principal, RoleLeaseLiability), credit(payment, RoleLeasePayable), debit(amort, RoleRouAmortizationExpense), credit(amort, RoleRouAsset)}
			}
			exp.group(g)
		}
	}

	emit := func(id, passMsg, failPrefix string, errs []string) {
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusFail, Message: failPrefix + joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusPass, Message: passMsg})
		}
	}
	emit(ValidatorASC842LeasePvRecompute, fmt.Sprintf("%d lease(s): initial liability is the present value of the payments and the right-of-use asset follows from it", len(hits)), "initial measurement does not recompute: ", pvErrs)
	emit(ValidatorASC842LeaseRollForward, "every period's interest, principal, closing liability, lease cost and amortization recompute and the liability retires at the end of the term", "liability roll-forward does not tie: ", rollErrs)
	emit(ValidatorASC842LeaseClassificationJudged, "every lease's classification rests on a judgment in the package", "classification is not judged: ", judgeErrs)
	emit(ValidatorASC842LeaseRateEvidence, "every discount rate equals the value of the fact it cites", "discount rates disagree with their cited facts: ", rateErrs)
	if len(pvErrs)+len(rollErrs) > 0 {
		exp.problem("the lease schedule does not recompute")
	}
	add(tieOutAccountingSchedule(pkg, ValidatorASC842AccountingScheduleTieOut, exp))
	return results
}

func zeroIfNaN(f float64) float64 {
	if math.IsNaN(f) {
		return 0
	}
	return f
}
