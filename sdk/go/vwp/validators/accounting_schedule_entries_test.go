package validators_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// editSchedule mutates the executable artifact and rewrites its manifest
// hash, so a mutated package fails only for the reason under test.
func editSchedule(t *testing.T, dir string, edit func(a *accountingschedule.Artifact)) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(accountingschedule.ArtifactPath)))
	if err != nil {
		t.Fatal(err)
	}
	var a accountingschedule.Artifact
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	edit(&a)
	addPart(t, dir, accountingschedule.ArtifactPath, "calculation", a)
}

func expectFail(t *testing.T, dir, id, fragment string) {
	t.Helper()
	res := result(t, verifyDir(t, dir), id)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, fragment) {
		t.Errorf("%s = %s %q; want fail mentioning %q", id, res.Status, res.Message, fragment)
	}
}

func expectPass(t *testing.T, dir, id string) {
	t.Helper()
	res := result(t, verifyDir(t, dir), id)
	if res.Status != vwp.StatusPass {
		t.Errorf("%s = %s %q; want pass", id, res.Status, res.Message)
	}
}

// Every schedule-bearing fixture reports each validator its required-output
// contract names, passing.
func TestScheduleFixturesSatisfyTheirRequiredOutputContract(t *testing.T) {
	for fixture, extension := range map[string]string{
		"asc350-60-crypto-assets": vwp.ExtensionASC35060,
		"asc350-30-impairment":    vwp.ExtensionASC35030,
		"staking-rewards":         vwp.ExtensionStakingReward,
		"digital-asset-lending":   vwp.ExtensionLending,
		"cost-basis-lot-relief":   vwp.ExtensionCostBasis,
	} {
		t.Run(fixture, func(t *testing.T) {
			report := verifyDir(t, copyFixture(t, "examples", fixture, "package"))
			ids := []string{vwp.ValidatorRequiredOutputsExist}
			for _, o := range vwp.RequiredOutputs(extension) {
				ids = append(ids, o.Validators...)
			}
			for _, id := range ids {
				if res := result(t, report, id); res.Status != vwp.StatusPass {
					t.Errorf("%s = %s %q", id, res.Status, res.Message)
				}
			}
		})
	}
}

func TestASC35060ScheduleTieOut(t *testing.T) {
	id := validators.ValidatorASC35060AccountingScheduleTieOut
	t.Run("missing when the roll-forward books a remeasurement", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, id, "is required")
	})
	t.Run("absent and nothing to book passes", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		removePart(t, dir, accountingschedule.ArtifactPath)
		editCalc(t, dir, "s3_roll_forward", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			for _, l := range in["lines"].([]any) {
				line := l.(map[string]any)
				line["gains"], line["losses"] = 0.0, 0.0
				line["closingBalance"] = line["openingBalance"].(float64) - line["dispositions"].(float64)
			}
			totals := doc["outputs"].(map[string]any)["totals"].(map[string]any)
			totals["gains"], totals["losses"], totals["closingBalance"] = 0.0, 0.0, 1600000.0
		})
		res := result(t, verifyDir(t, dir), id)
		if res.Status != vwp.StatusPass || !strings.Contains(res.Message, "book nothing") {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	})
	t.Run("gain booked as a loss", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{{AccountRole: validators.RoleCryptoAssetGainLoss, Debit: "50000.00"}, {AccountRole: validators.RoleCryptoAsset, Credit: "50000.00"}}
		})
		expectFail(t, dir, id, "expected debit crypto_asset 50000")
	})
	t.Run("amount off by a cent", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[1].Postings[0].Debit, a.Entries[1].Postings[1].Credit, a.TotalAmount = "100000.01", "100000.01", "150000.01"
		})
		expectFail(t, dir, id, "expected debit crypto_asset_gain_loss 100000")
	})
	t.Run("entry no calculation explains", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries = append(a.Entries, accountingschedule.Entry{ID: "SOL-2026-06-30", Date: "2026-06-30", Description: "SOL", ObligationID: "SOL",
				Postings: []accountingschedule.Posting{{AccountRole: validators.RoleCryptoAsset, Debit: "10.00"}, {AccountRole: validators.RoleCryptoAssetGainLoss, Credit: "10.00"}}})
			a.TotalAmount = "150010.00"
		})
		expectFail(t, dir, id, `not derived from any calculation: SOL-2026-06-30 (obligationId "SOL", 2026-06-30)`)
	})
	t.Run("wrong kind, wrong currency, wrong date", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Kind = accountingschedule.KindRevenueRecognition })
		expectFail(t, dir, id, `kind "revenue_recognition"; this package books fair_value_remeasurement entries`)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Kind, a.Currency = accountingschedule.KindFairValueRemeasurement, "EUR"
		})
		expectFail(t, dir, id, "currency EUR but the calculations are in USD")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Currency = "USD"; a.Entries[0].Date = "2026-06-29" })
		expectFail(t, dir, id, `no entry with obligationId "BTC" dated 2026-06-30`)
	})
	t.Run("gain and loss on one asset may share an entry", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s3_roll_forward", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			btc := in["lines"].([]any)[0].(map[string]any)
			btc["gains"], btc["losses"] = 70000.0, 20000.0
			doc["outputs"].(map[string]any)["totals"].(map[string]any)["gains"] = 70000.0
			doc["outputs"].(map[string]any)["totals"].(map[string]any)["losses"] = 120000.0
		})
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{
				{AccountRole: validators.RoleCryptoAsset, Debit: "70000.00"}, {AccountRole: validators.RoleCryptoAssetGainLoss, Credit: "70000.00"},
				{AccountRole: validators.RoleCryptoAssetGainLoss, Debit: "20000.00"}, {AccountRole: validators.RoleCryptoAsset, Credit: "20000.00"},
			}
			a.TotalAmount = "190000.00"
		})
		expectPass(t, dir, id)
	})
}

func TestASC35030ScheduleTieOut(t *testing.T) {
	id := validators.ValidatorASC35030AccountingScheduleTieOut
	t.Run("no impaired lot: schedule not required", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, id, "is required")
		editCalc(t, dir, "s4_impairment_test", func(doc map[string]any) {
			lot := doc["inputs"].(map[string]any)["lots"].([]any)[0].(map[string]any)
			lot["fairValue"] = 40000.0
			out := doc["outputs"].(map[string]any)
			out["lots"].([]any)[0].(map[string]any)["impairmentLoss"] = 0.0
			out["lots"].([]any)[0].(map[string]any)["carryingAmountAfter"] = 40000.0
			out["totalImpairmentLoss"] = 0.0
		})
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s3_f_lot1_fv" {
					fm["value"] = 40000.0
				}
			}
		})
		expectPass(t, dir, id)
	})
	t.Run("entry for an unimpaired lot", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries = append(a.Entries, accountingschedule.Entry{ID: "NFT-002", Date: "2026-06-30", Description: "not impaired", ObligationID: "NFT-002",
				Postings: []accountingschedule.Posting{{AccountRole: validators.RoleImpairmentLoss, Debit: "1.00"}, {AccountRole: validators.RoleIntangibleAsset, Credit: "1.00"}}})
			a.TotalAmount = "15001.00"
		})
		expectFail(t, dir, id, "lot NFT-002: nothing to book on 2026-06-30 but entries NFT-002 exist")
	})
	t.Run("loss uses the recomputed amount, not the declared one", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		editCalc(t, dir, "s4_impairment_test", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lots"].([]any)[0].(map[string]any)["fairValue"] = 20000.0
		})
		expectFail(t, dir, id, "expected debit impairment_loss 20000")
	})
}

func TestStakingScheduleTieOut(t *testing.T) {
	id := validators.ValidatorStakingAccountingScheduleTieOut
	t.Run("reward received later must be a receivable on recognition", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["rewards"].([]any)[2].(map[string]any)["receivedOn"] = "2026-05-02"
			doc["inputs"].(map[string]any)["unreceivedClosingUnits"] = 0.5
			doc["inputs"].(map[string]any)["chainObservedUnits"] = 1.0
			doc["inputs"].(map[string]any)["reconciliationNote"] = "epoch 285500 credited 2026-05-02"
		})
		expectFail(t, dir, id, "expected debit staking_reward_receivable 1300")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[2].Postings[0].AccountRole = validators.RoleStakingRewardReceivable
		})
		expectFail(t, dir, id, `receipt: no entry with obligationId "epoch-285500" dated 2026-05-02`)
	})
	t.Run("unreceived reward has no receipt entry", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			delete(doc["inputs"].(map[string]any)["rewards"].([]any)[2].(map[string]any), "receivedOn")
			doc["inputs"].(map[string]any)["unreceivedClosingUnits"] = 0.5
			doc["inputs"].(map[string]any)["chainObservedUnits"] = 1.0
			doc["inputs"].(map[string]any)["reconciliationNote"] = "epoch 285500 not yet credited"
		})
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[2].Postings[0].AccountRole = validators.RoleStakingRewardReceivable
		})
		expectPass(t, dir, id)
	})
	t.Run("income on the wrong role", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Entries[0].Postings[1].AccountRole = "revenue" })
		expectFail(t, dir, id, "expected credit staking_reward_income 1200")
	})
}

func TestLendingScheduleTieOut(t *testing.T) {
	id := validators.ValidatorLendingAccountingScheduleTieOut
	t.Run("unpriced accrual cannot be booked", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editCalc(t, dir, "s3_interest_accrual", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			delete(in, "unitPrice")
			delete(in, "priceFact")
			delete(doc["outputs"].(map[string]any), "accruedFairValue")
		})
		expectFail(t, dir, id, "not priced")
	})
	t.Run("price fact must be dated the last accrued day", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s3_f_fee_price" {
					fm["value"] = map[string]any{"price": 2500.0, "date": "2026-07-01"}
				}
			}
		})
		expectFail(t, dir, validators.ValidatorLendingAccrualRecompute, `is a price as of 2026-07-01, not the last accrued day 2026-06-30`)
	})
	t.Run("accruedFairValue must be units times price", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editCalc(t, dir, "s3_interest_accrual", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["accruedFairValue"] = 3200.0
		})
		expectFail(t, dir, validators.ValidatorLendingAccrualRecompute, "accruedFairValue 3200 does not equal")
	})
	t.Run("entry on the accrual end date instead of the last accrued day", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.EndDate, a.Entries[0].Date = "2026-07-01", "2026-07-01" })
		expectFail(t, dir, id, `no entry with obligationId "L-2026-04" dated 2026-06-30`)
	})
	t.Run("amount is the recomputed fair value rounded to scale", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings[0].Debit, a.Entries[0].Postings[1].Credit, a.TotalAmount = "3116.45", "3116.45", "3116.45"
		})
		expectFail(t, dir, id, "expected debit interest_receivable 3116.438")
	})
}

func TestCostBasisScheduleTieOut(t *testing.T) {
	id := validators.ValidatorCostBasisAccountingScheduleTieOut
	t.Run("loss disposal debits realized gain loss", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			d := doc["inputs"].(map[string]any)["disposals"].([]any)[0].(map[string]any)
			d["proceeds"] = 100000.0
			out := doc["outputs"].(map[string]any)
			out["disposals"].([]any)[0].(map[string]any)["realizedGainLoss"] = -5000.0
			out["totalRealizedGainLoss"] = -5000.0
		})
		expectFail(t, dir, id, "expected debit disposal_proceeds 100000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{
				{AccountRole: validators.RoleDisposalProceeds, Debit: "100000.00"},
				{AccountRole: validators.RoleRealizedGainLoss, Debit: "5000.00"},
				{AccountRole: validators.RoleCryptoAsset, Credit: "105000.00"},
			}
			a.TotalAmount = "105000.00"
		})
		expectPass(t, dir, id)
	})
	t.Run("basis relieved is recomputed from the lots", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lots"].([]any)[0].(map[string]any)["costBasis"] = 90000.0
		})
		expectFail(t, dir, id, "expected credit crypto_asset 115000")
	})
	t.Run("no disposals anywhere: schedule not required", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, id, "is required")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["disposals"] = []any{}
			out := doc["outputs"].(map[string]any)
			out["disposals"] = []any{}
			out["totalRealizedGainLoss"] = 0.0
			out["remainingLots"] = []any{map[string]any{"lotId": "L1", "units": 2.0, "costBasis": 80000.0}, map[string]any{"lotId": "L2", "units": 1.0, "costBasis": 50000.0}}
		})
		expectPass(t, dir, id)
	})
	t.Run("sub-unit residue rounds away at scale", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			d := doc["inputs"].(map[string]any)["disposals"].([]any)[0].(map[string]any)
			d["proceeds"] = 162500.004
			doc["outputs"].(map[string]any)["disposals"].([]any)[0].(map[string]any)["realizedGainLoss"] = 57500.004
			doc["outputs"].(map[string]any)["totalRealizedGainLoss"] = 57500.004
		})
		expectPass(t, dir, id)
	})
}
