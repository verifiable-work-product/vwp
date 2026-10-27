package validators_test

import (
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func TestASC740FailureModes(t *testing.T) {
	fixture := []string{"examples", "asc740-income-taxes", "package"}
	const calc = "s3_tax_provision"
	t.Run("taxable income and current tax recompute from the differences", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editCalc(t, dir, calc, func(doc map[string]any) {
			td := doc["inputs"].(map[string]any)["temporaryDifferences"].([]any)[0].(map[string]any)
			td["closingDifference"] = 170000.0 // +20,000 more deductible difference
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC740ProvisionRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "taxableIncome declared 940000 but recomputes to 960000") || !strings.Contains(res.Message, "currentTaxExpense declared 235000 but recomputes to 240000") {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
		expectFail(t, dir, validators.ValidatorASC740ProvisionDeferredRollForward, "deferredTaxAssetClosing declared 37500 but recomputes to 42500")
		expectFail(t, dir, validators.ValidatorASC740AccountingScheduleTieOut, "cannot be booked")
	})
	t.Run("pretax income must equal its fact", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s2_f_pretax_income" {
					fm["value"].(map[string]any)["amount"] = 1100000.0
				}
			}
		})
		expectFail(t, dir, validators.ValidatorASC740ProvisionRecompute, `pretaxIncome is 1000000 but fact "s2_f_pretax_income" says 1100000`)
	})
	t.Run("rate reconciliation must start from statutory and sum to the total", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editCalc(t, dir, calc, func(doc map[string]any) {
			rows := doc["outputs"].(map[string]any)["rateReconciliation"].([]any)
			rows[1].(map[string]any)["amount"] = 8000.0
		})
		expectFail(t, dir, validators.ValidatorASC740ProvisionRateReconciliation, "rows sum to 248000 but the total provision is 247500")
		editCalc(t, dir, calc, func(doc map[string]any) {
			rows := doc["outputs"].(map[string]any)["rateReconciliation"].([]any)
			rows[1].(map[string]any)["amount"] = 7500.0
			rows[0].(map[string]any)["item"] = "expected"
		})
		expectFail(t, dir, validators.ValidatorASC740ProvisionRateReconciliation, "exactly one 'statutory' row")
	})
	t.Run("valuation allowance needs a judgment and cannot exceed the gross asset", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editCalc(t, dir, calc, func(doc map[string]any) {
			va := doc["inputs"].(map[string]any)["valuationAllowance"].(map[string]any)
			delete(va, "judgmentRef")
		})
		expectFail(t, dir, validators.ValidatorASC740ProvisionValuationAllowanceJudged, "valuation allowance is carried without a realizability judgment (ASC 740-10-30-5(e))")
		editCalc(t, dir, calc, func(doc map[string]any) {
			va := doc["inputs"].(map[string]any)["valuationAllowance"].(map[string]any)
			va["judgmentRef"] = "s4_j_valuation_allowance"
			va["opening"] = 50000.0
		})
		expectFail(t, dir, validators.ValidatorASC740ProvisionDeferredRollForward, "valuation allowance (50000 opening, 0 closing) exceeds the gross deferred tax asset (45000, 37500)")
	})
	t.Run("uncertain tax position judgment must resolve", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editCalc(t, dir, calc, func(doc map[string]any) {
			doc["inputs"].(map[string]any)["uncertainTaxPositions"].(map[string]any)["judgmentRef"] = "j_missing"
		})
		expectFail(t, dir, validators.ValidatorASC740ProvisionValuationAllowanceJudged, `uncertainTaxPositions.judgmentRef "j_missing" does not resolve (ASC 740-10-25-6)`)
	})
	t.Run("deferred entry direction follows the balance changes", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		// Booking the deferred tax asset change on the wrong side balances
		// only if the expense flips too; the tie-out catches both.
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[2].Postings = []accountingschedule.Posting{
				{AccountRole: validators.RoleDeferredTaxAsset, Credit: "12500.00"},
				{AccountRole: validators.RoleIncomeTaxExpense, Debit: "27500.00"},
				{AccountRole: validators.RoleDeferredTaxLiability, Credit: "15000.00"},
			}
			a.TotalAmount = "272500.00"
		})
		expectFail(t, dir, validators.ValidatorASC740AccountingScheduleTieOut, "expected debit deferred_tax_asset 12500")
	})
	t.Run("a current benefit is booked on the opposite sides", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editCalc(t, dir, calc, func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			out := doc["outputs"].(map[string]any)
			in["pretaxIncome"] = -1000000.0
			// taxable = -1,000,000 + 30,000 + 50,000 - 60,000 - 80,000 = -1,060,000; current = -265,000
			out["taxableIncome"] = -1060000.0
			out["currentTaxExpense"] = -265000.0
			out["totalTaxExpense"] = -252500.0
			out["effectiveRate"] = 0.2525
			out["rateReconciliation"] = []any{
				map[string]any{"item": "statutory", "amount": -250000.0},
				map[string]any{"item": "permanent differences", "amount": 7500.0},
				map[string]any{"item": "valuation allowance release", "amount": -20000.0},
				map[string]any{"item": "unrecognized tax benefits", "amount": 10000.0},
			}
		})
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s2_f_pretax_income" {
					fm["value"].(map[string]any)["amount"] = -1000000.0
				}
			}
		})
		expectPass(t, dir, validators.ValidatorASC740ProvisionRecompute)
		expectFail(t, dir, validators.ValidatorASC740AccountingScheduleTieOut, "expected debit income_taxes_payable 265000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{
				{AccountRole: validators.RoleIncomeTaxesPayable, Debit: "265000.00"},
				{AccountRole: validators.RoleIncomeTaxExpense, Credit: "265000.00"},
			}
			a.TotalAmount = "290000.00"
		})
		expectPass(t, dir, validators.ValidatorASC740AccountingScheduleTieOut)
	})
	t.Run("provision missing", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		removePart(t, dir, "calculations/"+calc+".json")
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorASC740ProvisionSchemaValid); res.Status != vwp.StatusNotRun {
			t.Errorf("schemaValid = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorASC740AccountingScheduleTieOut); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "no schema-valid income tax provision") {
			t.Errorf("tieOut = %s %q", res.Status, res.Message)
		}
	})
}
