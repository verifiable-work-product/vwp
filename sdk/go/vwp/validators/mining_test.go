package validators_test

import (
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func miningFixture(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "examples", "mining-revenue", "package")
}

func TestMiningRevenueFailureModes(t *testing.T) {
	t.Run("fair value does not recompute and fees exceed units", func(t *testing.T) {
		dir := miningFixture(t)
		editCalc(t, dir, "s4_mining_schedule", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["rows"].([]any)[0].(map[string]any)["fairValue"] = 222000.0
			doc["inputs"].(map[string]any)["rewards"].([]any)[1].(map[string]any)["feeUnits"] = 4.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorMiningRewardsRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "expected 3.175 units / 222250, declared 3.175 units / 222000") || !strings.Contains(res.Message, "feeUnits 4 exceed the reward's units 3.205") {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("price fact states another price or another date", func(t *testing.T) {
		dir := miningFixture(t)
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s4_f_price_0418" {
					fm["value"] = map[string]any{"price": 72000.0, "date": "2026-04-17", "currency": "USD"}
				}
			}
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorMiningRewardsPricingEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `fact "s4_f_price_0418" is a price as of 2026-04-17, not the measurement date 2026-04-18`) {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
		editCalc(t, dir, "s4_mining_schedule", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["rewards"].([]any)[1].(map[string]any)["unitPrice"] = 71500.0
		})
		res = result(t, verifyDir(t, dir), validators.ValidatorMiningRewardsPricingEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `unitPrice is 71500 but fact "s4_f_price_0418" says 72000`) {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
	})
	t.Run("receipt-date basis requires receivedOn as the measurement date", func(t *testing.T) {
		dir := miningFixture(t)
		editCalc(t, dir, "s4_mining_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["pricingDateBasis"] = "receiptDate"
			delete(in["rewards"].([]any)[2].(map[string]any), "receivedOn")
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorMiningRewardsPricingEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "pricingDateBasis receiptDate requires receivedOn on every row") {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
	})
	t.Run("recognized units do not tie to the chain record", func(t *testing.T) {
		dir := miningFixture(t)
		editCalc(t, dir, "s4_mining_schedule", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["chainObservedUnits"] = 9.5
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorMiningRewardsChainTieOut)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `chainObservedUnits is 9.5 but fact "s4_f_chain_total" says 9.545`) {
			t.Errorf("chainTieOut = %s %q", res.Status, res.Message)
		}
		// An orphaned block reversed at source is a reconciling item that
		// must be explained.
		editCalc(t, dir, "s4_mining_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["chainObservedUnits"] = 9.545
			in["rewards"] = append(in["rewards"].([]any), map[string]any{
				"date": "2026-04-30", "receivedOn": "2026-04-30", "measurementDate": "2026-04-30", "units": 3.125, "unitPrice": 71000.0, "priceFact": "s4_f_price_0429", "reference": "block-891800",
			})
			doc["outputs"].(map[string]any)["rows"] = append(doc["outputs"].(map[string]any)["rows"].([]any), map[string]any{"date": "2026-04-30", "units": 3.125, "fairValue": 221875.0})
			doc["outputs"].(map[string]any)["totalUnits"] = 12.67
			doc["outputs"].(map[string]any)["totalFairValue"] = 899600.0
			in["otherReconcilingUnits"] = 3.125
		})
		res = result(t, verifyDir(t, dir), validators.ValidatorMiningRewardsChainTieOut)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "reconciling items declared without a reconciliationNote") {
			t.Errorf("chainTieOut = %s %q", res.Status, res.Message)
		}
	})
	t.Run("duplicate block reference and a reward outside the period", func(t *testing.T) {
		dir := miningFixture(t)
		editCalc(t, dir, "s4_mining_schedule", func(doc map[string]any) {
			rewards := doc["inputs"].(map[string]any)["rewards"].([]any)
			rewards[1].(map[string]any)["reference"] = "block-890100"
			rewards[2].(map[string]any)["date"] = "2026-05-01"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorMiningRewardsPopulation)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `reward "block-890100" already appears in`) || !strings.Contains(res.Message, "recognition date outside the schedule period") {
			t.Errorf("population = %s %q", res.Status, res.Message)
		}
	})
	t.Run("no schedule at all fails the tie-out, not just notRun", func(t *testing.T) {
		dir := miningFixture(t)
		removePart(t, dir, "calculations/s4_mining_schedule.json")
		// The artifact still exists but nothing derives it: the tie-out fails
		// (the shared matcher reports the absent currency declaration first).
		if res := result(t, verifyDir(t, dir), validators.ValidatorMiningAccountingScheduleTieOut); res.Status != vwp.StatusFail {
			t.Errorf("tieOut = %s %q, want fail", res.Status, res.Message)
		}
	})
}

func TestMiningScheduleTieOut(t *testing.T) {
	id := validators.ValidatorMiningAccountingScheduleTieOut
	t.Run("pool payout received later is a receivable then a transfer", func(t *testing.T) {
		dir := miningFixture(t)
		editCalc(t, dir, "s4_mining_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["operatorType"] = "poolParticipant"
			in["rewards"].([]any)[2].(map[string]any)["receivedOn"] = "2026-05-02"
			in["unreceivedClosingUnits"] = 3.165
			in["chainObservedUnits"] = 6.38
			in["reconciliationNote"] = "payout for block 891700 settled by the pool on 2026-05-02"
		})
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s4_f_chain_total" {
					fm["value"] = map[string]any{"units": 6.38, "periodStart": "2026-04-01", "periodEnd": "2026-04-30"}
				}
			}
		})
		expectFail(t, dir, id, "expected debit mining_reward_receivable 224715")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[2].Postings[0].AccountRole = validators.RoleMiningRewardReceivable
		})
		expectFail(t, dir, id, `receipt: no entry with obligationId "block-891700" dated 2026-05-02`)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.EndDate, a.TotalAmount = "2026-05-02", "902440.00"
			a.Entries = append(a.Entries, accountingschedule.Entry{ID: "block-891700-receipt", Date: "2026-05-02", Description: "payout received", ObligationID: "block-891700",
				Postings: []accountingschedule.Posting{{AccountRole: validators.RoleCryptoAsset, Debit: "224715.00"}, {AccountRole: validators.RoleMiningRewardReceivable, Credit: "224715.00"}}})
		})
		expectPass(t, dir, id)
		if report := verifyDir(t, dir); !report.Valid {
			for _, r := range report.Results {
				if r.Status == vwp.StatusFail {
					t.Errorf("FAIL %s: %s", r.ValidatorId, r.Message)
				}
			}
		}
	})
	t.Run("revenue booked to the wrong role or amount", func(t *testing.T) {
		dir := miningFixture(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Entries[0].Postings[1].AccountRole = "revenue" })
		expectFail(t, dir, id, "expected credit mining_revenue 222250")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings[1].AccountRole = validators.RoleMiningRevenue
			a.Entries[0].Postings[0].Debit, a.Entries[0].Postings[1].Credit, a.TotalAmount = "222250.01", "222250.01", "677725.01"
		})
		expectFail(t, dir, id, "expected debit crypto_asset 222250")
	})
	t.Run("wrong kind and missing artifact", func(t *testing.T) {
		dir := miningFixture(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Kind = accountingschedule.KindStakingRewards })
		expectFail(t, dir, id, `kind "staking_rewards"; this package books mining_revenue entries`)
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, id, "is required")
	})
}
