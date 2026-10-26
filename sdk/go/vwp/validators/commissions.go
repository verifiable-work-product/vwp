package validators

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// Commissions extension validator ids.
const (
	ValidatorCommissionsStatementSchemaValid     = "commissions.statement.schemaValid"
	ValidatorCommissionsStatementRecompute       = "commissions.statement.recompute"
	ValidatorCommissionsAccountingScheduleTieOut = vwp.ValidatorCommissionsAccountingScheduleTieOut
)

const (
	commissionStatementType      = "commissionStatement"
	commissionStatementSchemaRef = "extensions/accounting-commissions/commission-statement.schema.json"
)

// Commissions implements the vwp-accounting-commissions extension validators
// over every commission statement in the package:
//
//   - recompute: each period's schedule amount is its commission times the
//     period's valuation rate, rounded to the schedule scale, and the total
//     is their sum;
//   - accountingSchedule.tieOut: the executable schedule books exactly one
//     commission_accrual group per period on the period's last day —
//     commission_expense debited and accrued_commissions_payable credited
//     for that amount — in the statement's currency, and nothing else. A
//     statement that earns nothing needs no schedule.
func Commissions(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	_ = ctx
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	all := []string{ValidatorCommissionsStatementRecompute, ValidatorCommissionsAccountingScheduleTieOut}

	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		if id := getString(in, "statementId"); id != "" {
			return id
		}
		return getString(doc, "id")
	}
	hits, found := locateCalculations(pkg, add, ValidatorCommissionsStatementSchemaValid, commissionStatementType, commissionStatementSchemaRef, scope)
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid commission statement", all...)
		return results
	}

	exp := scheduleExpectation{kind: accountingschedule.KindCommissionAccrual}
	var wrong []string
	for _, hit := range hits {
		exp.sources++
		schedule := getMap(getMap(hit.Doc, "outputs"), "schedule")
		currency := getString(schedule, "currency")
		exp.currency(currency)
		if in := getString(getMap(hit.Doc, "inputs"), "currency"); in != "" && in != currency {
			wrong = append(wrong, fmt.Sprintf("%s: inputs.currency %s but the schedule is in %s", hit.Path, in, currency))
		}
		scale := int(jsNumber(schedule["amountScale"]))
		tol := scheduleTolerance(scale)
		var total float64
		seen := map[string]bool{}
		for i, e := range getMaps(schedule, "entries") {
			period := getString(e, "period")
			label := fmt.Sprintf("%s: %s", hit.Path, period)
			if seen[period] {
				wrong = append(wrong, label+" appears twice")
				continue
			}
			seen[period] = true
			commission, okC := exactDecimal(getString(e, "commission"))
			rate, okR := exactDecimal(getString(e, "rate"))
			amount, okA := exactDecimal(getString(e, "amount"))
			if !okC || !okR || !okA {
				wrong = append(wrong, label+": commission, rate and amount must be decimal strings")
				continue
			}
			want := roundHalfUp(new(big.Rat).Mul(commission, rate), scale)
			if want.Cmp(amount) != 0 {
				wrong = append(wrong, fmt.Sprintf("%s: amount %s but commission %s × rate %s = %s", label, getString(e, "amount"), getString(e, "commission"), getString(e, "rate"), want.FloatString(scale)))
			}
			f, _ := amount.Float64()
			total += f
			if _, ok := parseWhen(getString(e, "date")); !ok {
				wrong = append(wrong, label+": date is not a parseable date")
			}
			exp.group(scheduleGroup{
				obligationId: period, date: getString(e, "date"), label: fmt.Sprintf("entry %d (%s)", i+1, period),
				postings: []schedulePosting{debit(f, RoleCommissionExpense), credit(f, RoleAccruedCommissionsPayable)},
			})
		}
		if declared, ok := exactDecimal(getString(schedule, "total")); !ok {
			wrong = append(wrong, hit.Path+": total must be a decimal string")
		} else if d, _ := declared.Float64(); math.Abs(d-total) > tol {
			wrong = append(wrong, fmt.Sprintf("%s: total %s does not equal the sum of the entries %s", hit.Path, getString(schedule, "total"), num(total)))
		}
	}

	if len(wrong) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorCommissionsStatementRecompute, Status: vwp.StatusFail, Message: "statement does not recompute: " + joinCapped(wrong, 5)})
		exp.problem("the statement does not recompute")
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorCommissionsStatementRecompute, Status: vwp.StatusPass, Message: strings.Join(paths(hits), ", ")})
	}
	add(tieOutAccountingSchedule(pkg, ValidatorCommissionsAccountingScheduleTieOut, exp))
	return results
}

// exactDecimal parses a decimal string exactly (no float rounding).
func exactDecimal(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || strings.ContainsAny(s, "/eE") {
		return nil, false
	}
	return r, true
}

// roundHalfUp rounds r to scale decimal places, halves away from zero.
func roundHalfUp(r *big.Rat, scale int) *big.Rat {
	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(pow))
	num := new(big.Int).Set(scaled.Num())
	den := new(big.Int).Set(scaled.Denom())
	neg := num.Sign() < 0
	if neg {
		num.Neg(num)
	}
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if neg {
		q.Neg(q)
	}
	return new(big.Rat).SetFrac(q, pow)
}
