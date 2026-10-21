package validators

import (
	"fmt"
	"math"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// Digital asset lending extension validator ids.
const (
	ValidatorLendingAccrualSchemaValid       = "lending.accrual.schemaValid"
	ValidatorLendingAccrualRecompute         = "lending.accrual.recompute"
	ValidatorLendingCollateralSchemaValid    = "lending.collateral.schemaValid"
	ValidatorLendingCollateralRecompute      = "lending.collateral.recompute"
	ValidatorLendingDerecognitionClaimJudge  = "lending.derecognition.claimJudged"
	ValidatorLendingAccountingScheduleTieOut = vwp.ValidatorLendingAccountingScheduleTieOut
)

const (
	interestAccrualType      = "digitalAssetLoanInterestAccrual"
	interestAccrualSchemaRef = "extensions/digital-assets-lending/interest-accrual.schema.json"
	collateralCoverageType   = "collateralCoverage"
	collateralCoverageSchema = "extensions/digital-assets-lending/collateral-coverage.schema.json"
	lendingDefaultUnitTol    = 1e-6
	lendingDefaultRatioTol   = 0.0001
	lendingDefaultAmountTol  = 0.01
)

// derecognitionRefs are the reference families under which a transfer of a
// lent digital asset is concluded on: ASC 860-10-40 for crypto assets that
// are financial assets, ASC 610-20-40 (derecognition of nonfinancial
// assets), ASC 350-30-40 (intangibles), or — for the SEC staff / AICPA
// Digital Assets Practice Aid analysis of crypto intangibles, which has no
// Codification paragraph — any reference that names derecognition
// ("AICPA Digital Assets Practice Aid — derecognition of lent crypto
// assets"). The validator requires the conclusion to exist and to rest on
// a judgment; it does not prescribe which framework the preparer applies.
// Matching is case-insensitive.
var derecognitionRefs = []string{"860-10-40", "610-20-40", "350-30-40", "derecogni"}

// Lending implements the vwp-digital-assets-lending extension validators
// over every loan schedule in the package: each in-kind accrual recomputes
// from principal × rate × days / basis AND those terms are what the cited
// terms fact says; each collateral coverage ratio, margin shortfall and
// unsecured portion (the ASC 326-20-35-6 loss basis) recompute;
// and a derecognition conclusion exists as a claim citing a derecognition
// paragraph family with a linked judgment — whether control passed is
// never arithmetic.
func Lending(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	facts := factIndex(ctx)

	accScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "loanId") + "|" + getString(in, "asset") + "|" + getString(in, "accrualStart") + "|" + getString(in, "accrualEnd")
	}
	accruals, found := locateCalculations(pkg, add, ValidatorLendingAccrualSchemaValid, interestAccrualType, interestAccrualSchemaRef, accScope)
	// Executable accrual entries: the fee accrued in kind, at the fair value
	// the priced accrual states, on the last day it accrued.
	accrualExp := scheduleExpectation{kind: accountingschedule.KindInterestAccrual}
	if !found || len(accruals) == 0 {
		notRun(add, "no schema-valid interest accrual", ValidatorLendingAccrualRecompute)
	} else {
		var errs []string
		for _, acc := range accruals {
			pfx := acc.Path + ": "
			unitTol := tolerance(acc.Doc, "unitTolerance", lendingDefaultUnitTol)
			in := getMap(acc.Doc, "inputs")
			out := getMap(acc.Doc, "outputs")
			days, ok := daysBetween(getString(in, "accrualStart"), getString(in, "accrualEnd"))
			principal := jsNumber(in["principalUnits"])
			rate := jsNumber(in["annualRate"])
			basis := jsNumber(in["dayCountBasis"])
			expected := principal * rate * float64(days) / basis
			termsFact := getString(in, "termsFact")
			switch {
			case !ok || days < 0:
				errs = append(errs, pfx+"accrualStart/accrualEnd are not an ordered YYYY-MM-DD pair")
			case jsNumber(out["days"]) != float64(days):
				errs = append(errs, fmt.Sprintf("%sdays %s declared but %d elapse between %s and %s", pfx, num(jsNumber(out["days"])), days, getString(in, "accrualStart"), getString(in, "accrualEnd")))
			case !nearTol(jsNumber(out["accruedUnits"]), expected, unitTol):
				errs = append(errs, fmt.Sprintf("%saccruedUnits %s does not equal %s × %s × %d/%s = %s", pfx, num(jsNumber(out["accruedUnits"])), num(principal), num(rate), days, num(basis), num(expected)))
			}
			// An in-kind fee is booked at the fair value of the units accrued:
			// a priced accrual states unitPrice, the fact that price comes
			// from (as of the last accrued day) and accruedFairValue.
			accrualExp.sources++
			accrualExp.currency(getString(in, "currency"))
			_, hasPrice := in["unitPrice"]
			_, hasFV := out["accruedFairValue"]
			priceRef := getString(in, "priceFact")
			if !hasPrice && !hasFV && priceRef == "" {
				accrualExp.problem(pfx + "not priced: add inputs.unitPrice, inputs.priceFact (a price fact dated the last accrued day), inputs.currency and outputs.accruedFairValue")
			} else if ok && days >= 0 {
				amountTol := tolerance(acc.Doc, "roundingTolerance", lendingDefaultAmountTol)
				price := jsNumber(in["unitPrice"])
				fv := expected * price
				endDay, _ := parseWhen(getString(in, "accrualEnd"))
				measured := endDay.AddDate(0, 0, -1)
				switch {
				case math.IsNaN(price) || price < 0:
					errs = append(errs, pfx+"unitPrice is not a non-negative number")
				case !nearTol(jsNumber(out["accruedFairValue"]), fv, amountTol):
					errs = append(errs, fmt.Sprintf("%saccruedFairValue %s does not equal accrued units %s × unitPrice %s = %s", pfx, num(jsNumber(out["accruedFairValue"])), num(expected), num(price), num(fv)))
				}
				if getString(in, "currency") == "" {
					errs = append(errs, pfx+"a priced accrual must declare inputs.currency")
				}
				if msg := checkFactValue(facts, priceRef, price, amountTol, pfx+"unitPrice", "unitPrice", "price", "amount", "close"); msg != "" {
					errs = append(errs, msg)
				} else if fd, okDate := factDate(facts[priceRef]); !okDate {
					errs = append(errs, fmt.Sprintf("%sfact %q states no date for its price (value needs a date)", pfx, priceRef))
				} else if !fd.Equal(measured) {
					errs = append(errs, fmt.Sprintf("%sfact %q is a price as of %s, not the last accrued day %s", pfx, priceRef, fd.Format("2006-01-02"), measured.Format("2006-01-02")))
				}
				accrualExp.group(scheduleGroup{obligationId: getString(in, "loanId"), date: measured.Format("2006-01-02"), label: pfx + "accrual",
					postings: []schedulePosting{debit(fv, RoleInterestReceivable), credit(fv, RoleInterestIncome)}})
			}
			// The terms the accrual runs on must be the terms the cited fact extracted.
			for _, chk := range []struct {
				label string
				want  float64
				tol   float64
				keys  []string
			}{
				{"principalUnits", principal, unitTol, []string{"principalUnits", "principal", "units"}},
				{"annualRate", rate, 1e-9, []string{"annualRate", "rate"}},
				{"dayCountBasis", basis, 0, []string{"dayCountBasis", "basis"}},
			} {
				if msg := checkFactValue(facts, termsFact, chk.want, chk.tol, pfx+chk.label, chk.keys...); msg != "" {
					errs = append(errs, msg)
				}
			}
		}
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorLendingAccrualRecompute, Status: vwp.StatusFail, Message: joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: ValidatorLendingAccrualRecompute, Status: vwp.StatusPass, Message: fmt.Sprintf("%d accrual(s) recompute from the terms their facts state", len(accruals))})
		}
	}
	add(tieOutAccountingSchedule(pkg, ValidatorLendingAccountingScheduleTieOut, accrualExp))

	covScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "loanId") + "|" + getString(in, "asOf")
	}
	coverages, found := locateCalculations(pkg, add, ValidatorLendingCollateralSchemaValid, collateralCoverageType, collateralCoverageSchema, covScope)
	if !found || len(coverages) == 0 {
		notRun(add, "no schema-valid collateral coverage", ValidatorLendingCollateralRecompute)
	} else {
		var errs []string
		for _, cov := range coverages {
			pfx := cov.Path + ": "
			tol := tolerance(cov.Doc, "roundingTolerance", 0.01)
			in := getMap(cov.Doc, "inputs")
			out := getMap(cov.Doc, "outputs")
			loan := jsNumber(in["loanFairValue"])
			collateral := jsNumber(in["collateralFairValue"])
			required := jsNumber(in["requiredCoverageRatio"])
			ratio := collateral / loan
			shortfall := math.Max(0, loan*required-collateral)
			unsecured := math.Max(0, loan-collateral)
			adequate := ratio+lendingDefaultRatioTol >= required
			declaredAdequate, _ := out["adequatelyCollateralized"].(bool)
			switch {
			case !nearTol(jsNumber(out["unsecuredPortion"]), unsecured, tol):
				errs = append(errs, fmt.Sprintf("%sunsecuredPortion %s does not equal max(0, %s − %s) = %s (the 326-20-35-6 loss basis is carrying amount less collateral, not the margin shortfall)", pfx, num(jsNumber(out["unsecuredPortion"])), num(loan), num(collateral), num(unsecured)))
			case !nearTol(jsNumber(out["coverageRatio"]), ratio, lendingDefaultRatioTol):
				errs = append(errs, fmt.Sprintf("%scoverageRatio %s does not equal collateral %s / loan %s = %s", pfx, num(jsNumber(out["coverageRatio"])), num(collateral), num(loan), num(math.Round(ratio*1e6)/1e6)))
			case !nearTol(jsNumber(out["shortfall"]), shortfall, tol):
				errs = append(errs, fmt.Sprintf("%sshortfall %s does not equal max(0, %s × %s − %s) = %s", pfx, num(jsNumber(out["shortfall"])), num(loan), num(required), num(collateral), num(shortfall)))
			case declaredAdequate != adequate:
				errs = append(errs, fmt.Sprintf("%sadequatelyCollateralized %t disagrees with ratio %s against required %s", pfx, declaredAdequate, num(math.Round(ratio*1e4)/1e4), num(required)))
			}
			for _, chk := range []struct {
				label, factKey string
				want           float64
				keys           []string
			}{
				{"loanFairValue", "loanFairValueFact", loan, []string{"loanFairValue", "fairValue", "amount"}},
				{"collateralFairValue", "collateralFairValueFact", collateral, []string{"collateralFairValue", "fairValue", "amount"}},
				{"requiredCoverageRatio", "requiredCoverageRatioFact", required, []string{"requiredCoverageRatio", "ratio"}},
			} {
				if ref := getString(in, chk.factKey); ref != "" {
					if msg := checkFactValue(facts, ref, chk.want, tol, pfx+chk.label, chk.keys...); msg != "" {
						errs = append(errs, msg)
					}
				}
			}
		}
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorLendingCollateralRecompute, Status: vwp.StatusFail, Message: joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: ValidatorLendingCollateralRecompute, Status: vwp.StatusPass, Message: fmt.Sprintf("%d coverage calculation(s) recompute", len(coverages))})
		}
	}

	var derecognition, unjudged []string
	for _, c := range ctx.Claims {
		refs, _ := stringRefs(c, "standardRefs")
		cites := false
		for _, r := range refs {
			for _, fam := range derecognitionRefs {
				if strings.Contains(strings.ToLower(r), fam) {
					cites = true
				}
			}
		}
		if !cites {
			continue
		}
		derecognition = append(derecognition, getString(c, "id"))
		if len(getSlice(c, "judgments")) == 0 {
			unjudged = append(unjudged, getString(c, "id"))
		}
	}
	switch {
	case len(derecognition) == 0:
		add(vwp.ValidationResult{ValidatorId: ValidatorLendingDerecognitionClaimJudge, Status: vwp.StatusFail,
			Message: "no claim concludes on derecognition (none cites ASC 860-10-40, ASC 610-20-40, ASC 350-30-40, or a reference naming derecognition such as the AICPA Practice Aid analysis)"})
	case len(unjudged) > 0:
		add(vwp.ValidationResult{ValidatorId: ValidatorLendingDerecognitionClaimJudge, Status: vwp.StatusFail,
			Message: "derecognition claims without a linked judgment: " + strings.Join(unjudged, ", "), Refs: unjudged})
	default:
		add(vwp.ValidationResult{ValidatorId: ValidatorLendingDerecognitionClaimJudge, Status: vwp.StatusPass,
			Message: fmt.Sprintf("%d derecognition claim(s) link a judgment", len(derecognition))})
	}
	return results
}
