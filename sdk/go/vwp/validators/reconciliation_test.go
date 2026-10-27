package validators_test

import (
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

const reconciliationFixture = "period-end-reconciliation"

func reconciliationDir(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "examples", reconciliationFixture, "package")
}

func TestReconciliationFixturePassesEveryValidator(t *testing.T) {
	report := verifyDir(t, filepath.Join(repoRoot, "examples", reconciliationFixture, "package"))
	if !report.Valid {
		for _, r := range report.Results {
			if r.Status == vwp.StatusFail {
				t.Errorf("FAIL %s: %s", r.ValidatorId, r.Message)
			}
		}
	}
	ids := []string{vwp.ValidatorRequiredOutputsExist, vwp.ValidatorAccountingScheduleValid}
	for _, o := range vwp.RequiredOutputs(vwp.ExtensionReconciliation) {
		ids = append(ids, o.Validators...)
	}
	for _, id := range ids {
		if res := result(t, report, id); res.Status != vwp.StatusPass {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	}
}

func TestReconciliationBalanceRecompute(t *testing.T) {
	id := validators.ValidatorReconciliationBalanceRecompute
	t.Run("unexplained residual must be declared", func(t *testing.T) {
		dir := reconciliationDir(t)
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["ledgerBalance"] = 1247000.0
		})
		expectFail(t, dir, id, "difference declared 2650 but recomputes to 3000")
	})
	t.Run("balance must equal its cited fact", func(t *testing.T) {
		dir := reconciliationDir(t)
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s1_f_source_balance" {
					fm["value"] = map[string]any{"amount": 1250100.0, "date": "2026-06-30"}
				}
			}
		})
		expectFail(t, dir, id, `sourceBalance is 1250000 but fact "s1_f_source_balance" says 1250100`)
	})
	t.Run("balance fact must be stated as of the reconciliation date", func(t *testing.T) {
		dir := reconciliationDir(t)
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s1_f_ledger_balance" {
					fm["value"] = map[string]any{"amount": 1247350.0, "date": "2026-06-29"}
				}
			}
		})
		expectFail(t, dir, id, "stated as of 2026-06-29, not the reconciliation date 2026-06-30")
	})
	t.Run("a fact without a date cannot support a balance", func(t *testing.T) {
		dir := reconciliationDir(t)
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s1_f_ledger_balance" {
					fm["value"] = 1247350.0
				}
			}
		})
		expectFail(t, dir, id, "states no date")
	})
}

func TestReconciliationItemsSupported(t *testing.T) {
	id := validators.ValidatorReconciliationBalanceItemsSupported
	items := func(doc map[string]any) []any { return doc["inputs"].(map[string]any)["items"].([]any) }
	t.Run("an omission must be bookable with both roles", func(t *testing.T) {
		dir := reconciliationDir(t)
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			items(doc)[1].(map[string]any)["bookable"] = false
		})
		expectFail(t, dir, id, "item RI-2 is an omission but is not bookable")
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			items(doc)[1].(map[string]any)["bookable"] = true
			delete(items(doc)[1].(map[string]any), "creditRole")
		})
		expectFail(t, dir, id, "declares no debitRole/creditRole")
	})
	t.Run("support must resolve", func(t *testing.T) {
		dir := reconciliationDir(t)
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			items(doc)[0].(map[string]any)["supportRef"] = "s2_f_missing"
		})
		expectFail(t, dir, id, `cites "s2_f_missing" which is not a fact, evidence or calculation`)
	})
	t.Run("an unexplained residual needs a judgment naming the scope", func(t *testing.T) {
		dir := reconciliationDir(t)
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			items(doc)[0].(map[string]any)["status"] = "unexplained"
			doc["outputs"].(map[string]any)["explained"] = -350.0
			doc["outputs"].(map[string]any)["unexplained"] = 3000.0
		})
		expectPass(t, dir, validators.ValidatorReconciliationBalanceRecompute)
		// The fixture's judgment names scope DEMO-EX-01, so a residual on
		// that scope is judged; a residual on another scope is not.
		expectPass(t, dir, id)
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["scope"] = "DEMO-EX-02"
		})
		expectFail(t, dir, id, `unexplained residual 3000 but no judgment names scope "DEMO-EX-02"`)
	})
	t.Run("duplicate item ids fail", func(t *testing.T) {
		dir := reconciliationDir(t)
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			items(doc)[0].(map[string]any)["id"] = "RI-2"
		})
		expectFail(t, dir, id, "item RI-2 appears twice")
	})
}

func TestReconciliationFlux(t *testing.T) {
	t.Run("variances and totals recompute", func(t *testing.T) {
		dir := reconciliationDir(t)
		editCalc(t, dir, "s3_flux_analysis", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["rows"].([]any)[1].(map[string]any)["variance"] = 770000.0
		})
		expectFail(t, dir, validators.ValidatorReconciliationFluxRecompute, "digital_assets: variance declared 770000 but current 2627000 − prior 1850000 = 777000")
	})
	t.Run("a zero prior balance has no percentage", func(t *testing.T) {
		dir := reconciliationDir(t)
		editCalc(t, dir, "s3_flux_analysis", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lines"].([]any)[2].(map[string]any)["prior"] = 0.0
			row := doc["outputs"].(map[string]any)["rows"].([]any)[2].(map[string]any)
			row["variance"] = 1150.0
			doc["outputs"].(map[string]any)["totalPrior"] = 3080000.0
		})
		expectFail(t, dir, validators.ValidatorReconciliationFluxRecompute, "variancePct must be null when the prior balance is zero")
		editCalc(t, dir, "s3_flux_analysis", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["rows"].([]any)[2].(map[string]any)["variancePct"] = nil
		})
		expectPass(t, dir, validators.ValidatorReconciliationFluxRecompute)
	})
	t.Run("a variance above threshold needs a claim that exists", func(t *testing.T) {
		id := validators.ValidatorReconciliationFluxVariancesExplained
		dir := reconciliationDir(t)
		editCalc(t, dir, "s3_flux_analysis", func(doc map[string]any) {
			delete(doc["inputs"].(map[string]any)["lines"].([]any)[1].(map[string]any), "explanationClaim")
		})
		expectFail(t, dir, id, "digital_assets: variance 777000 exceeds a threshold but names no explanationClaim")
		editCalc(t, dir, "s3_flux_analysis", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lines"].([]any)[1].(map[string]any)["explanationClaim"] = "s3_c_nowhere"
		})
		expectFail(t, dir, id, `explanationClaim "s3_c_nowhere" is not a claim in the package`)
		// The percentage threshold alone triggers an explanation.
		editCalc(t, dir, "s3_flux_analysis", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lines"].([]any)[1].(map[string]any)["explanationClaim"] = "s3_c_digital_assets_flux"
			doc["inputs"].(map[string]any)["thresholdPct"] = 0.01
		})
		expectFail(t, dir, id, "exchange_cash: variance 17350 exceeds a threshold")
	})
}

func TestReconciliationScheduleTieOut(t *testing.T) {
	id := validators.ValidatorReconciliationAccountingScheduleTieOut
	t.Run("books the omission under its declared roles", func(t *testing.T) {
		dir := reconciliationDir(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Entries[0].Postings[1].AccountRole = "cash" })
		expectFail(t, dir, id, "expected credit exchange_cash 350")
	})
	t.Run("a timing difference is not booked", func(t *testing.T) {
		dir := reconciliationDir(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries = append(a.Entries, accountingschedule.Entry{ID: "RI-1", Date: "2026-06-30", Description: "deposit in transit", ObligationID: "RI-1",
				Postings: []accountingschedule.Posting{{AccountRole: "exchange_cash", Debit: "3000.00"}, {AccountRole: "deposits_in_transit", Credit: "3000.00"}}})
			a.TotalAmount = "3350.00"
		})
		expectFail(t, dir, id, `not derived from any calculation: RI-1 (obligationId "RI-1", 2026-06-30)`)
	})
	t.Run("nothing bookable, nothing to book", func(t *testing.T) {
		dir := reconciliationDir(t)
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, id, "is required")
		editCalc(t, dir, "s2_balance_reconciliation", func(doc map[string]any) {
			items := doc["inputs"].(map[string]any)["items"].([]any)
			doc["inputs"].(map[string]any)["items"] = items[:1]
			doc["inputs"].(map[string]any)["ledgerBalance"] = 1247000.0
			doc["outputs"].(map[string]any)["difference"] = 3000.0
			doc["outputs"].(map[string]any)["explained"] = 3000.0
		})
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s1_f_ledger_balance" {
					fm["value"] = map[string]any{"amount": 1247000.0, "date": "2026-06-30"}
				}
			}
		})
		res := result(t, verifyDir(t, dir), id)
		if res.Status != vwp.StatusPass || !strings.Contains(res.Message, "book nothing") {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	})
	t.Run("wrong kind", func(t *testing.T) {
		dir := reconciliationDir(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Kind = accountingschedule.KindImpairment })
		expectFail(t, dir, id, "this package books reconciling_adjustment entries")
	})
}
