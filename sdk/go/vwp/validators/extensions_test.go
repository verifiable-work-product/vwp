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

// bitwaveFixtures are the packages fixturegen/main.go produces, with the
// extension validator ids each must pass.
var bitwaveFixtures = map[string][]string{
	"asc350-60-crypto-assets": {
		validators.ValidatorASC35060ScopeRegisterPresent, validators.ValidatorASC35060ScopeSchemaValid,
		validators.ValidatorASC35060ScopeConsistent, validators.ValidatorASC35060ScopeJudgmentLinked,
		validators.ValidatorASC35060RollForwardSchemaValid, validators.ValidatorASC35060RollForwardTieOut,
		validators.ValidatorASC35060RollForwardAssetsInScope, validators.ValidatorASC35060HoldingsSchemaValid,
		validators.ValidatorASC35060HoldingsTieOut, validators.ValidatorASC35060HoldingsAssetsInScope, validators.ValidatorASC35060HoldingsTiesToRollForward,
		validators.ValidatorASC35060AccountingScheduleTieOut,
	},
	"asc350-30-impairment": {
		validators.ValidatorASC35030ImpairmentSchemaValid, validators.ValidatorASC35030ImpairmentRecompute,
		validators.ValidatorASC35030ImpairmentNoReversal, validators.ValidatorASC35030ImpairmentFairValueEvidence,
		validators.ValidatorASC35030ImpairmentContinuity, validators.ValidatorASC35030AccountingScheduleTieOut,
	},
	"staking-rewards": {
		validators.ValidatorStakingRewardsSchemaValid, validators.ValidatorStakingRewardsRecompute,
		validators.ValidatorStakingRewardsChainTieOut, validators.ValidatorStakingRewardsPricingEvidence,
		validators.ValidatorStakingRewardsPopulation, validators.ValidatorStakingAccountingScheduleTieOut,
	},
	"digital-asset-lending": {
		validators.ValidatorLendingAccrualSchemaValid, validators.ValidatorLendingAccrualRecompute,
		validators.ValidatorLendingCollateralSchemaValid, validators.ValidatorLendingCollateralRecompute,
		validators.ValidatorLendingDerecognitionClaimJudge, validators.ValidatorLendingAccountingScheduleTieOut,
	},
	"cost-basis-lot-relief": {
		validators.ValidatorCostBasisLotReliefSchemaValid, validators.ValidatorCostBasisLotReliefUnitsConserved,
		validators.ValidatorCostBasisLotReliefRecompute, validators.ValidatorCostBasisLotReliefMethodOrder,
		validators.ValidatorCostBasisWalletAllocSchemaValid, validators.ValidatorCostBasisWalletAllocTieOut, validators.ValidatorCostBasisWalletAllocEligibility,
		validators.ValidatorCostBasisAccountingScheduleTieOut,
	},
	"stablecoin-classification": {
		validators.ValidatorStablecoinRegisterPresent, validators.ValidatorStablecoinSchemaValid,
		validators.ValidatorStablecoinJudgmentLinked, validators.ValidatorStablecoinRedemptionEvidence,
		validators.ValidatorStablecoinClassificationConsistent, validators.ValidatorStablecoinCashEquivalentChallenged,
	},
}

func TestBitwaveFixturesVerifyClean(t *testing.T) {
	for fixture, ids := range bitwaveFixtures {
		t.Run(fixture, func(t *testing.T) {
			report := verifyDir(t, filepath.Join(repoRoot, "examples", fixture, "package"))
			if !report.Valid {
				for _, r := range report.Results {
					if r.Status == vwp.StatusFail {
						t.Errorf("FAIL %s: %s", r.ValidatorId, r.Message)
					}
				}
				t.Fatal("fixture must verify clean")
			}
			for _, id := range ids {
				if res := result(t, report, id); res.Status != vwp.StatusPass {
					t.Errorf("%s = %s %q, want pass", id, res.Status, res.Message)
				}
			}
		})
	}
}

// The presence validators report on success too, so a skill's completion
// contract can require them to pass.
func TestPresenceValidatorsPassOnValidFixtures(t *testing.T) {
	for fixture, id := range map[string]string{
		"asc820-fair-value":               validators.ValidatorASC820MeasurementsPresent,
		"digital-assets-wallet-ownership": validators.ValidatorWalletsRegisterPresent,
	} {
		report := verifyDir(t, filepath.Join(repoRoot, "examples", fixture, "package"))
		if res := result(t, report, id); res.Status != vwp.StatusPass {
			t.Errorf("%s: %s = %s %q", fixture, id, res.Status, res.Message)
		}
	}
}

func TestRegistryCoversEveryExtension(t *testing.T) {
	reg := validators.Registry()
	for _, ext := range []string{
		vwp.ExtensionASC606, vwp.ExtensionASC820, vwp.ExtensionWalletControl,
		vwp.ExtensionASC35060, vwp.ExtensionASC35030, vwp.ExtensionStakingReward,
		vwp.ExtensionLending, vwp.ExtensionCostBasis, vwp.ExtensionStablecoin,
	} {
		if reg[ext] == nil {
			t.Errorf("registry missing %s", ext)
		}
	}
}

func TestRegisterSchemaLookup(t *testing.T) {
	for ext, names := range map[string][]string{
		vwp.ExtensionASC820:        {validators.RegisterFairValueMeasurements},
		vwp.ExtensionWalletControl: {validators.RegisterWallets},
		vwp.ExtensionASC35060:      {validators.RegisterCryptoAssets},
		vwp.ExtensionStablecoin:    {validators.RegisterStablecoinHoldings},
	} {
		if got := validators.RegisterNames(ext); strings.Join(got, ",") != strings.Join(names, ",") {
			t.Errorf("RegisterNames(%s) = %v, want %v", ext, got, names)
		}
		for _, n := range names {
			ref, ok := validators.RegisterSchema(ext, n)
			if !ok || !strings.HasPrefix(ref, "extensions/") {
				t.Errorf("RegisterSchema(%s, %s) = %q, %v", ext, n, ref, ok)
			}
			if validators.RegisterPath(n) != "facts/"+n+".json" {
				t.Errorf("RegisterPath(%s) = %q", n, validators.RegisterPath(n))
			}
		}
	}
	if _, ok := validators.RegisterSchema(vwp.ExtensionASC606, "anything"); ok {
		t.Error("asc606 defines no register")
	}
	if names := validators.RegisterNames(vwp.ExtensionLending); len(names) != 0 {
		t.Errorf("lending defines no register, got %v", names)
	}
}

// --- failure modes ---

// editCalc mutates a calculation part AND rewrites its manifest hash, so a
// mutated package fails only for the substantive reason under test, never
// for core.partHashes.match.
func editCalc(t *testing.T, dir, name string, edit func(doc map[string]any)) {
	t.Helper()
	rel := "calculations/" + name + ".json"
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	addPart(t, dir, rel, "calculation", doc)
}

// editFacts mutates facts/facts.json and rewrites its manifest hash.
func editFacts(t *testing.T, dir string, edit func(doc map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "facts", "facts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	addPart(t, dir, "facts/facts.json", "facts", doc)
}

func TestASC35060FailureModes(t *testing.T) {
	t.Run("inScope disagrees with criteria", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editJSON(t, filepath.Join(dir, "facts", "crypto-assets.json"), func(doc map[string]any) {
			doc["assets"].([]any)[0].(map[string]any)["inScope"] = false
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC35060ScopeConsistent)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "ca_btc") {
			t.Errorf("scope.consistent = %s %q", res.Status, res.Message)
		}
	})
	t.Run("out-of-scope asset without judgment", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editJSON(t, filepath.Join(dir, "facts", "crypto-assets.json"), func(doc map[string]any) {
			delete(doc["assets"].([]any)[2].(map[string]any), "scopeJudgmentRef")
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC35060ScopeJudgmentLinked)
		if res.Status != vwp.StatusFail || len(res.Refs) != 1 || res.Refs[0] != "ca_wdemo" {
			t.Errorf("scope.judgmentLinked = %s %q %v", res.Status, res.Message, res.Refs)
		}
	})
	t.Run("roll-forward line does not tie", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s3_roll_forward", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lines"].([]any)[0].(map[string]any)["gains"] = 60000.0
		})
		report := verifyDir(t, dir)
		res := result(t, report, validators.ValidatorASC35060RollForwardTieOut)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "BTC (expected closing 660000, declared 650000)") {
			t.Errorf("rollForward.tieOut = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorASC35060HoldingsTiesToRollForward); res.Status != vwp.StatusNotRun {
			t.Errorf("tiesToRollForward = %s %q", res.Status, res.Message)
		}
	})
	t.Run("roll-forward for out-of-scope asset", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s3_roll_forward", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lines"].([]any)[0].(map[string]any)["asset"] = "WDEMO"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC35060RollForwardAssetsInScope)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "WDEMO") {
			t.Errorf("assetsInScope = %s %q", res.Status, res.Message)
		}
	})
	t.Run("holdings do not tie to total", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s5_significant_holdings", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["totalFairValue"] = 1600000.0
		})
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorASC35060HoldingsTieOut); res.Status != vwp.StatusFail {
			t.Errorf("holdings.tieOut = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorASC35060HoldingsTiesToRollForward); res.Status != vwp.StatusFail {
			t.Errorf("tiesToRollForward = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: an out-of-scope asset in the disclosure table.
	t.Run("holdings table lists an out-of-scope asset", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s5_significant_holdings", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["holdings"].([]any)[0].(map[string]any)["asset"] = "WDEMO"
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("a disclosure table with an out-of-scope asset must not verify")
		}
		if res := result(t, report, validators.ValidatorASC35060HoldingsAssetsInScope); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "WDEMO") {
			t.Errorf("holdings.assetsInScope = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorASC35060HoldingsTiesToRollForward); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "BTC has a roll-forward closing balance but is not in the holdings table") {
			t.Errorf("tiesToRollForward = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: a $1 non-significant holding reported as $650,000 of other holdings.
	t.Run("other holdings declared independently of the table", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s5_significant_holdings", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			btc := in["holdings"].([]any)[0].(map[string]any)
			btc["significant"] = false
			btc["fairValue"] = 1.0
			in["totalFairValue"] = 1550000.0
			out := doc["outputs"].(map[string]any)
			out["significantFairValue"] = 900000.0
			out["otherFairValue"] = 650000.0
			out["significantCostBasis"] = 800000.0
			out["otherCostBasis"] = 450000.0
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("a table whose other-holdings total is not the sum of its rows must not verify")
		}
		if res := result(t, report, validators.ValidatorASC35060HoldingsTieOut); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "otherFairValue declared 650000 but the holdings sum to 1") {
			t.Errorf("holdings.tieOut = %s %q", res.Status, res.Message)
		}
	})
	// Round-2 reproduction: two consecutive quarters each closing at 1.55m.
	// The correct 1.55m holdings table must pass and a doubled 3.1m one fail.
	t.Run("consecutive roll-forwards chain and are not summed", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		raw, _ := os.ReadFile(filepath.Join(dir, "calculations", "s3_roll_forward.json"))
		var q3 map[string]any
		_ = json.Unmarshal(raw, &q3)
		q3["id"] = "s3_roll_forward_q3"
		in := q3["inputs"].(map[string]any)
		in["periodStart"] = "2026-07-01"
		in["periodEnd"] = "2026-09-30"
		for _, l := range in["lines"].([]any) {
			lm := l.(map[string]any)
			lm["openingBalance"] = lm["closingBalance"]
			lm["additions"], lm["dispositions"], lm["gains"], lm["losses"] = 0.0, 0.0, 0.0, 0.0
		}
		q3["outputs"] = map[string]any{"totals": map[string]any{"openingBalance": 1550000.0, "additions": 0.0, "dispositions": 0.0, "gains": 0.0, "losses": 0.0, "closingBalance": 1550000.0}}
		addPart(t, dir, "calculations/s3_roll_forward_q3.json", "calculation", q3)
		// Holdings table dated Q3 end, correct at 1.55m.
		editCalc(t, dir, "s5_significant_holdings", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["asOf"] = "2026-09-30"
		})
		report := verifyDir(t, dir)
		for _, id := range []string{validators.ValidatorASC35060RollForwardTieOut, validators.ValidatorASC35060HoldingsTiesToRollForward} {
			if res := result(t, report, id); res.Status != vwp.StatusPass {
				t.Errorf("%s = %s %q", id, res.Status, res.Message)
			}
		}
		// Doubled table must fail.
		editCalc(t, dir, "s5_significant_holdings", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["totalFairValue"] = 3100000.0
			for _, h := range in["holdings"].([]any) {
				hm := h.(map[string]any)
				hm["fairValue"] = hm["fairValue"].(float64) * 2
			}
			doc["outputs"].(map[string]any)["significantFairValue"] = 3100000.0
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorASC35060HoldingsTiesToRollForward); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "BTC holdings fair value 1300000 vs roll-forward closing 650000") {
			t.Errorf("tiesToRollForward = %s %q", res.Status, res.Message)
		}
		// A Q3 schedule that does not open where Q2 closed fails the chain.
		dir = copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		in["lines"].([]any)[0].(map[string]any)["openingBalance"] = 700000.0
		in["lines"].([]any)[0].(map[string]any)["closingBalance"] = 700000.0
		q3["outputs"].(map[string]any)["totals"].(map[string]any)["openingBalance"] = 1600000.0
		q3["outputs"].(map[string]any)["totals"].(map[string]any)["closingBalance"] = 1600000.0
		addPart(t, dir, "calculations/s3_roll_forward_q3.json", "calculation", q3)
		if res := result(t, verifyDir(t, dir), validators.ValidatorASC35060RollForwardTieOut); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "opens BTC at 700000 but calculations/s3_roll_forward.json closed it at 650000") {
			t.Errorf("rollForward.tieOut = %s %q", res.Status, res.Message)
		}
	})
	t.Run("holdings dated off any roll-forward close fail", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s5_significant_holdings", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["asOf"] = "2026-05-31"
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorASC35060HoldingsTiesToRollForward); res.Status != vwp.StatusFail || !strings.Contains(res.Message, `no roll-forward for scope "" closes on 2026-05-31`) {
			t.Errorf("tiesToRollForward = %s %q", res.Status, res.Message)
		}
	})
	t.Run("per-asset fair value must equal that asset's roll-forward closing", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		editCalc(t, dir, "s5_significant_holdings", func(doc map[string]any) {
			hs := doc["inputs"].(map[string]any)["holdings"].([]any)
			hs[0].(map[string]any)["fairValue"] = 900000.0
			hs[1].(map[string]any)["fairValue"] = 650000.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC35060HoldingsTiesToRollForward)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "BTC holdings fair value 900000 vs roll-forward closing 650000") {
			t.Errorf("tiesToRollForward = %s %q", res.Status, res.Message)
		}
	})
	t.Run("no roll-forward", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-60-crypto-assets", "package")
		removePart(t, dir, "calculations/s3_roll_forward.json")
		report := verifyDir(t, dir)
		for _, id := range []string{validators.ValidatorASC35060RollForwardTieOut, validators.ValidatorASC35060HoldingsTiesToRollForward} {
			if res := result(t, report, id); res.Status != vwp.StatusNotRun {
				t.Errorf("%s = %s %q", id, res.Status, res.Message)
			}
		}
	})
}

func TestASC35030FailureModes(t *testing.T) {
	t.Run("loss does not recompute", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		editCalc(t, dir, "s4_impairment_test", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["lots"].([]any)[0].(map[string]any)["impairmentLoss"] = 10000.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC35030ImpairmentRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "NFT-001") {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("write-up is a reversal", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		editCalc(t, dir, "s4_impairment_test", func(doc map[string]any) {
			lot := doc["outputs"].(map[string]any)["lots"].([]any)[1].(map[string]any)
			lot["impairmentLoss"] = -3000.0
			lot["carryingAmountAfter"] = 18000.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC35030ImpairmentNoReversal)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "NFT-002") {
			t.Errorf("noReversal = %s %q", res.Status, res.Message)
		}
	})
	t.Run("fair value fact missing", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		editCalc(t, dir, "s4_impairment_test", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["lots"].([]any)[0].(map[string]any)["fairValueFact"] = "f_nope"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC35030ImpairmentFairValueEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `fact "f_nope" which is not in the package`) {
			t.Errorf("fairValueEvidence = %s %q", res.Status, res.Message)
		}
	})
	// Round-2: a later test revisits a lot from its reduced carrying amount
	// (allowed) or from a different one (continuity failure).
	t.Run("a later test carries the earlier result forward", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		raw, _ := os.ReadFile(filepath.Join(dir, "calculations", "s4_impairment_test.json"))
		var later map[string]any
		_ = json.Unmarshal(raw, &later)
		later["id"] = "s4_impairment_test_q3"
		in := later["inputs"].(map[string]any)
		in["testDate"] = "2026-09-30"
		lot := in["lots"].([]any)[0].(map[string]any)
		lot["carryingAmount"] = 25000.0 // what the Q2 test left
		lot["fairValue"] = 25000.0
		in["lots"] = []any{lot}
		out := later["outputs"].(map[string]any)
		out["lots"] = []any{map[string]any{"lotId": "NFT-001", "impairmentLoss": 0.0, "carryingAmountAfter": 25000.0}}
		out["totalImpairmentLoss"] = 0.0
		addPart(t, dir, "calculations/s4_impairment_test_q3.json", "calculation", later)
		report := verifyDir(t, dir)
		for _, id := range []string{validators.ValidatorASC35030ImpairmentSchemaValid, validators.ValidatorASC35030ImpairmentRecompute, validators.ValidatorASC35030ImpairmentContinuity} {
			if res := result(t, report, id); res.Status != vwp.StatusPass {
				t.Errorf("%s = %s %q", id, res.Status, res.Message)
			}
		}
		lot["carryingAmount"] = 40000.0
		lot["fairValue"] = 40000.0
		out["lots"] = []any{map[string]any{"lotId": "NFT-001", "impairmentLoss": 0.0, "carryingAmountAfter": 40000.0}}
		addPart(t, dir, "calculations/s4_impairment_test_q3.json", "calculation", later)
		if res := result(t, verifyDir(t, dir), validators.ValidatorASC35030ImpairmentContinuity); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "lot NFT-001 starts calculations/s4_impairment_test_q3.json at carrying amount 40000 but the 2026-06-30 test left it at 25000") {
			t.Errorf("continuity = %s %q", res.Status, res.Message)
		}
	})
	// Round-2 reproduction: a duplicate output row with an arbitrary amount
	// ahead of the correct one.
	t.Run("duplicate output rows are rejected", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		editCalc(t, dir, "s4_impairment_test", func(doc map[string]any) {
			out := doc["outputs"].(map[string]any)
			out["lots"] = append([]any{map[string]any{"lotId": "NFT-001", "impairmentLoss": 99999999.0, "carryingAmountAfter": 0.0}}, out["lots"].([]any)...)
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("a duplicate output row must not verify")
		}
		if res := result(t, report, validators.ValidatorASC35030ImpairmentRecompute); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "duplicate output row for lot NFT-001") {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: the calculation's fair value disagrees with the
	// fact it cites (evidence and facts untouched).
	t.Run("fair value contradicts its fact", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc350-30-impairment", "package")
		editCalc(t, dir, "s4_impairment_test", func(doc map[string]any) {
			lot := doc["inputs"].(map[string]any)["lots"].([]any)[0].(map[string]any)
			lot["fairValue"] = 39000.0
			out := doc["outputs"].(map[string]any)
			out["lots"].([]any)[0].(map[string]any)["impairmentLoss"] = 1000.0
			out["lots"].([]any)[0].(map[string]any)["carryingAmountAfter"] = 39000.0
			out["totalImpairmentLoss"] = 1000.0
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("a test priced off a number its fact does not state must not verify")
		}
		res := result(t, report, validators.ValidatorASC35030ImpairmentFairValueEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `NFT-001 fairValue is 39000 but fact "s3_f_lot1_fv" says 25000`) {
			t.Errorf("fairValueEvidence = %s %q", res.Status, res.Message)
		}
		// Arithmetic on the (wrong) inputs is internally consistent, so recompute passes: the fact check is what catches it.
		if res := result(t, report, validators.ValidatorASC35030ImpairmentRecompute); res.Status != vwp.StatusPass {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
}

func TestStakingFailureModes(t *testing.T) {
	t.Run("units do not tie to chain", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["chainObservedUnits"] = 1.75
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsChainTieOut)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `chainObservedUnits is 1.75 but fact "s4_f_chain_total" says 1.5`) {
			t.Errorf("chainTieOut = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: rewards inflated tenfold with the chain fact unchanged.
	t.Run("rewards inflated against the chain fact", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			out := doc["outputs"].(map[string]any)
			for i, r := range in["rewards"].([]any) {
				r.(map[string]any)["units"] = 5.0
				row := out["rows"].([]any)[i].(map[string]any)
				row["units"] = 5.0
				row["fairValue"] = 5.0 * r.(map[string]any)["unitPrice"].(float64)
			}
			in["chainObservedUnits"] = 15.0
			out["totalUnits"] = 15.0
			out["totalFairValue"] = 37500.0
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("a schedule contradicting its chain fact must not verify")
		}
		if res := result(t, report, validators.ValidatorStakingRewardsChainTieOut); res.Status != vwp.StatusFail || !strings.Contains(res.Message, `chainObservedUnits is 15 but fact "s4_f_chain_total" says 1.5`) {
			t.Errorf("chainTieOut = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: prices inflated tenfold with the price facts unchanged.
	t.Run("prices inflated against the price facts", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			out := doc["outputs"].(map[string]any)
			for i, r := range in["rewards"].([]any) {
				rm := r.(map[string]any)
				rm["unitPrice"] = rm["unitPrice"].(float64) * 10
				row := out["rows"].([]any)[i].(map[string]any)
				row["fairValue"] = row["fairValue"].(float64) * 10
			}
			out["totalFairValue"] = 37500.0
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("a schedule priced off numbers its facts do not state must not verify")
		}
		if res := result(t, report, validators.ValidatorStakingRewardsPricingEvidence); res.Status != vwp.StatusFail || !strings.Contains(res.Message, `row 1 (2026-04-10) unitPrice is 24000 but fact "s4_f_price_0410" says 2400`) {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorStakingRewardsRecompute); res.Status != vwp.StatusPass {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("two schedules with the same scope are rejected, different scopes aggregate", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		raw, _ := os.ReadFile(filepath.Join(dir, "calculations", "s4_rewards_schedule.json"))
		var dup map[string]any
		_ = json.Unmarshal(raw, &dup)
		dup["id"] = "s4_rewards_schedule_2"
		addPart(t, dir, "calculations/s4_rewards_schedule_2.json", "calculation", dup)
		if res := result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsSchemaValid); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "declares the same scope") {
			t.Errorf("schemaValid = %s %q", res.Status, res.Message)
		}
		dup["inputs"].(map[string]any)["arrangement"] = "validator-0xOTHER"
		addPart(t, dir, "calculations/s4_rewards_schedule_2.json", "calculation", dup)
		report := verifyDir(t, dir)
		for _, id := range []string{validators.ValidatorStakingRewardsSchemaValid, validators.ValidatorStakingRewardsRecompute, validators.ValidatorStakingRewardsChainTieOut, validators.ValidatorStakingRewardsPricingEvidence, validators.ValidatorStakingRewardsPopulation} {
			if res := result(t, report, id); res.Status != vwp.StatusPass {
				t.Errorf("%s = %s %q", id, res.Status, res.Message)
			}
		}
	})
	// Round-2 reproduction: the same April rewards duplicated into a second
	// schedule whose period starts a day later (same arrangement, references
	// and chain fact) must not pass.
	t.Run("overlapping schedule double counts rewards", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		raw, _ := os.ReadFile(filepath.Join(dir, "calculations", "s4_rewards_schedule.json"))
		var dup map[string]any
		_ = json.Unmarshal(raw, &dup)
		dup["id"] = "s4_rewards_schedule_2"
		dup["inputs"].(map[string]any)["periodStart"] = "2026-04-02"
		addPart(t, dir, "calculations/s4_rewards_schedule_2.json", "calculation", dup)
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("duplicated rewards must not verify")
		}
		res := result(t, report, validators.ValidatorStakingRewardsPopulation)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `reward "epoch-281000" already appears in calculations/s4_rewards_schedule.json`) || !strings.Contains(res.Message, "overlapping periods") {
			t.Errorf("population = %s %q", res.Status, res.Message)
		}
	})
	t.Run("rewards must lie in the schedule period", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["periodStart"] = "2026-05-01"
			in["periodEnd"] = "2026-05-31"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsPopulation)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "recognition date outside the schedule period 2026-05-01 to 2026-05-31") {
			t.Errorf("population = %s %q", res.Status, res.Message)
		}
	})
	// Round-2 reproduction: switching the pricing basis to contract
	// inception while every row is still priced on its receipt date.
	t.Run("pricing basis is enforced against the rows and their facts", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["pricingDateBasis"] = "contractInception"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsPricingEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "contractInception requires inceptionOn on every row") {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
		// With inception dates that differ from the measurement date, the
		// measurement date and the price fact's date are both checked.
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			for _, r := range doc["inputs"].(map[string]any)["rewards"].([]any) {
				r.(map[string]any)["inceptionOn"] = "2026-04-01"
			}
		})
		res = result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsPricingEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "measurementDate 2026-04-10 is not the inceptionOn 2026-04-01 the contractInception basis requires") {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			for _, r := range doc["inputs"].(map[string]any)["rewards"].([]any) {
				r.(map[string]any)["measurementDate"] = "2026-04-01"
			}
		})
		res = result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsPricingEvidence)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `fact "s4_f_price_0410" is a price as of 2026-04-10, not the measurement date 2026-04-01`) {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
	})
	// Round-2: rewards earned at period end but received next month
	// reconcile through the unreceived closing balance.
	t.Run("earned before receipt reconciles through unreceived balances", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			out := doc["outputs"].(map[string]any)
			in["rewards"] = append(in["rewards"].([]any), map[string]any{
				"date": "2026-04-30", "receivedOn": "2026-05-02", "measurementDate": "2026-04-30", "units": 0.3, "unitPrice": 2600.0, "priceFact": "s4_f_price_0430", "reference": "epoch-285750",
			})
			out["rows"] = append(out["rows"].([]any), map[string]any{"date": "2026-04-30", "units": 0.3, "fairValue": 780.0})
			out["totalUnits"] = 1.8
			out["totalFairValue"] = 4530.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsChainTieOut)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "recognized 1.8 + unreceived opening 0 − unreceived closing 0 − other 0 = 1.8 does not equal chain-observed receipts 1.5") {
			t.Errorf("chainTieOut = %s %q", res.Status, res.Message)
		}
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["unreceivedClosingUnits"] = 0.3
			in["reconciliationNote"] = "0.3 ETH earned for the 2026-04-30 duty period, credited 2026-05-02."
		})
		// A reward recognized before receipt is a receivable on its
		// recognition date and moves to the crypto asset on receipt.
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.EndDate, a.TotalAmount = "2026-05-02", "5310.00"
			a.Entries = append(a.Entries,
				accountingschedule.Entry{ID: "epoch-285750", Date: "2026-04-30", Description: "0.3 ETH earned, unreceived", ObligationID: "epoch-285750",
					Postings: []accountingschedule.Posting{{AccountRole: validators.RoleStakingRewardReceivable, Debit: "780.00"}, {AccountRole: validators.RoleStakingRewardIncome, Credit: "780.00"}}},
				accountingschedule.Entry{ID: "epoch-285750-receipt", Date: "2026-05-02", Description: "0.3 ETH received", ObligationID: "epoch-285750",
					Postings: []accountingschedule.Posting{{AccountRole: validators.RoleCryptoAsset, Debit: "780.00"}, {AccountRole: validators.RoleStakingRewardReceivable, Credit: "780.00"}}})
		})
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorStakingRewardsChainTieOut); res.Status != vwp.StatusPass {
			t.Errorf("chainTieOut = %s %q", res.Status, res.Message)
		}
		if !report.Valid {
			for _, r := range report.Results {
				if r.Status == vwp.StatusFail {
					t.Errorf("FAIL %s: %s", r.ValidatorId, r.Message)
				}
			}
			t.Error("earned-before-receipt schedule with a reconciliation and recognition-date pricing must verify")
		}
		// Under a receipt-date basis an unreceived reward has no measurement
		// date yet: the basis is what such a policy must change.
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["pricingDateBasis"] = "receiptDate"
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsPricingEvidence); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "measurementDate 2026-04-30 is not the receivedOn 2026-05-02 the receiptDate basis requires") {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
	})
	t.Run("row fair value wrong", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["rows"].([]any)[1].(map[string]any)["fairValue"] = 1300.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStakingRewardsRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "row 2 (2026-04-20)") {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("schema failure stops the rest", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editCalc(t, dir, "s4_rewards_schedule", func(doc map[string]any) {
			delete(doc["inputs"].(map[string]any)["rewards"].([]any)[0].(map[string]any), "priceFact")
		})
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorStakingRewardsSchemaValid); res.Status != vwp.StatusFail {
			t.Errorf("schemaValid = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorStakingRewardsPricingEvidence); res.Status != vwp.StatusNotRun {
			t.Errorf("pricingEvidence = %s %q", res.Status, res.Message)
		}
	})
}

func TestLendingFailureModes(t *testing.T) {
	t.Run("accrual days wrong", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editCalc(t, dir, "s3_interest_accrual", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["days"] = 90.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorLendingAccrualRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "91 elapse") {
			t.Errorf("accrual.recompute = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: the accrual runs on a rate the terms fact does not state.
	t.Run("accrual rate contradicts the terms fact", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editCalc(t, dir, "s3_interest_accrual", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["annualRate"] = 0.06
			doc["outputs"].(map[string]any)["accruedUnits"] = 100 * 0.06 * 91 / 365.0
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("an accrual on terms its fact does not state must not verify")
		}
		if res := result(t, report, validators.ValidatorLendingAccrualRecompute); res.Status != vwp.StatusFail || !strings.Contains(res.Message, `annualRate is 0.06 but fact "s1_f_terms" says 0.05`) {
			t.Errorf("accrual.recompute = %s %q", res.Status, res.Message)
		}
	})
	// Review (angel.guevara.flores): the margin shortfall is not the
	// allowance — the 35-6 loss basis is carrying amount less collateral.
	t.Run("unsecured portion is carrying amount less collateral, not the margin shortfall", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		// Coverage 1.6x against a 1.5x requirement: no shortfall and no
		// unsecured portion. Reporting a margin-style number as the
		// unsecured portion is the error under test.
		editCalc(t, dir, "s4_collateral_coverage", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["unsecuredPortion"] = 25000.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorLendingCollateralRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "unsecuredPortion 25000 does not equal max(0, 250000 − 400000) = 0") {
			t.Errorf("collateral.recompute = %s %q", res.Status, res.Message)
		}
		editCalc(t, dir, "s4_collateral_coverage", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["unsecuredPortion"] = 0.0
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorLendingCollateralRecompute); res.Status != vwp.StatusPass {
			t.Errorf("collateral.recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("collateral contradicts the collateral fact", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editCalc(t, dir, "s4_collateral_coverage", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["collateralFairValue"] = 500000.0
			out := doc["outputs"].(map[string]any)
			out["coverageRatio"] = 2.0
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorLendingCollateralRecompute); res.Status != vwp.StatusFail || !strings.Contains(res.Message, `collateralFairValue is 500000 but fact "s4_f_collateral" says 400000`) {
			t.Errorf("collateral.recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("derecognition may cite the AICPA Practice Aid / SEC staff analysis alone", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editJSON(t, filepath.Join(dir, "claims", "claims.json"), func(doc map[string]any) {
			for _, c := range doc["claims"].([]any) {
				cm := c.(map[string]any)
				if cm["id"] == "s2_c_derecognition" {
					cm["standardRefs"] = []any{"AICPA Digital Assets Practice Aid, derecognition of lent crypto assets (SEC staff view)"}
				}
			}
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorLendingDerecognitionClaimJudge); res.Status != vwp.StatusPass {
			t.Errorf("claimJudged = %s %q", res.Status, res.Message)
		}
	})
	t.Run("derecognition may cite ASC 610-20 instead of 860", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editJSON(t, filepath.Join(dir, "claims", "claims.json"), func(doc map[string]any) {
			for _, c := range doc["claims"].([]any) {
				cm := c.(map[string]any)
				if cm["id"] == "s2_c_derecognition" {
					cm["standardRefs"] = []any{"ASC 610-20-40-1"}
				}
			}
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorLendingDerecognitionClaimJudge); res.Status != vwp.StatusPass {
			t.Errorf("claimJudged = %s %q", res.Status, res.Message)
		}
	})
	t.Run("collateral adequacy flag wrong", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editCalc(t, dir, "s4_collateral_coverage", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["collateralFairValue"] = 300000.0
			out := doc["outputs"].(map[string]any)
			out["coverageRatio"] = 1.2
			out["shortfall"] = 75000.0
			out["adequatelyCollateralized"] = true
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorLendingCollateralRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "adequatelyCollateralized true disagrees") {
			t.Errorf("collateral.recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("derecognition claim without judgment", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		editJSON(t, filepath.Join(dir, "claims", "claims.json"), func(doc map[string]any) {
			for _, c := range doc["claims"].([]any) {
				cm := c.(map[string]any)
				if cm["id"] == "s2_c_derecognition" {
					delete(cm, "judgments")
				}
			}
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorLendingDerecognitionClaimJudge)
		if res.Status != vwp.StatusFail || len(res.Refs) != 1 || res.Refs[0] != "s2_c_derecognition" {
			t.Errorf("derecognition.claimJudged = %s %q %v", res.Status, res.Message, res.Refs)
		}
	})
	t.Run("no accrual calculation", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		removePart(t, dir, "calculations/s3_interest_accrual.json")
		res := result(t, verifyDir(t, dir), validators.ValidatorLendingAccrualRecompute)
		if res.Status != vwp.StatusNotRun {
			t.Errorf("accrual.recompute = %s %q", res.Status, res.Message)
		}
	})
}

func TestCostBasisFailureModes(t *testing.T) {
	t.Run("relief out of FIFO order", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			d := doc["inputs"].(map[string]any)["disposals"].([]any)[0].(map[string]any)
			d["reliefs"] = []any{map[string]any{"lotId": "L2", "units": 1.0}, map[string]any{"lotId": "L1", "units": 1.5}}
			doc["outputs"].(map[string]any)["disposals"].([]any)[0].(map[string]any)["basisRelieved"] = 110000.0
			doc["outputs"].(map[string]any)["disposals"].([]any)[0].(map[string]any)["realizedGainLoss"] = 52500.0
			doc["outputs"].(map[string]any)["totalRealizedGainLoss"] = 52500.0
			doc["outputs"].(map[string]any)["remainingLots"] = []any{
				map[string]any{"lotId": "L1", "units": 0.5, "costBasis": 20000.0},
				map[string]any{"lotId": "L3", "units": 1.0, "costBasis": 60000.0},
			}
		})
		report := verifyDir(t, dir)
		res := result(t, report, validators.ValidatorCostBasisLotReliefMethodOrder)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "D1 relieved L2 but FIFO picks L1") {
			t.Errorf("methodOrder = %s %q", res.Status, res.Message)
		}
		// The arithmetic itself still recomputes for the lots actually relieved.
		if res := result(t, report, validators.ValidatorCostBasisLotReliefRecompute); res.Status != vwp.StatusPass {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("declared method LIFO but FIFO relief", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["method"] = "LIFO"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisLotReliefMethodOrder)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "LIFO picks L2") {
			t.Errorf("methodOrder = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: a disposal relieving a lot acquired after it, under
	// specific identification (which has no ordering constraint).
	t.Run("future lot relieved under specific identification", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["method"] = "specificIdentification"
			in["lots"].([]any)[0].(map[string]any)["acquiredAt"] = "2027-01-15"
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("relieving a lot bought after the sale must not verify")
		}
		if res := result(t, report, validators.ValidatorCostBasisLotReliefUnitsConserved); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "relieves lot L1 acquired later (2027-01-15)") {
			t.Errorf("unitsConserved = %s %q", res.Status, res.Message)
		}
	})
	// Review reproduction: a remaining lot that was never an input.
	t.Run("invented remaining lot", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			out := doc["outputs"].(map[string]any)
			out["remainingLots"] = append(out["remainingLots"].([]any), map[string]any{"lotId": "L-GHOST", "units": 100.0, "costBasis": 10000000.0})
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("inventing remaining inventory must not verify")
		}
		if res := result(t, report, validators.ValidatorCostBasisLotReliefUnitsConserved); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "L-GHOST which is not an input lot") {
			t.Errorf("unitsConserved = %s %q", res.Status, res.Message)
		}
	})
	t.Run("timestamps compare chronologically not lexically", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["lots"].([]any)[0].(map[string]any)["acquiredAt"] = "2024-06-15T09:30:00Z"
			in["disposals"].([]any)[0].(map[string]any)["date"] = "2025-03-20T16:00:00Z"
		})
		report := verifyDir(t, dir)
		for _, id := range []string{validators.ValidatorCostBasisLotReliefUnitsConserved, validators.ValidatorCostBasisLotReliefMethodOrder} {
			if res := result(t, report, id); res.Status != vwp.StatusPass {
				t.Errorf("%s = %s %q", id, res.Status, res.Message)
			}
		}
	})
	t.Run("two pools validate; a duplicated pool is rejected", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorCostBasisLotReliefSchemaValid); res.Status != vwp.StatusPass || !strings.Contains(res.Message, "s3_lot_relief_wb") {
			t.Errorf("schemaValid = %s %q", res.Status, res.Message)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, "calculations", "s3_lot_relief_wb.json"))
		var dup map[string]any
		_ = json.Unmarshal(raw, &dup)
		dup["id"] = "s3_lot_relief_wb2"
		addPart(t, dir, "calculations/s3_lot_relief_wb2.json", "calculation", dup)
		report = verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorCostBasisLotReliefSchemaValid); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "declares the same scope (BTC|W-B)") {
			t.Errorf("schemaValid = %s %q", res.Status, res.Message)
		}
	})
	// Round-2 reproduction: a duplicate disposal output row ahead of the
	// correct one.
	t.Run("duplicate disposal output rows are rejected", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			out := doc["outputs"].(map[string]any)
			out["disposals"] = append([]any{map[string]any{"disposalId": "D1", "basisRelieved": 1.0, "realizedGainLoss": 162499.0}}, out["disposals"].([]any)...)
		})
		report := verifyDir(t, dir)
		if report.Valid {
			t.Fatal("a duplicate disposal output row must not verify")
		}
		if res := result(t, report, validators.ValidatorCostBasisLotReliefRecompute); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "duplicate output row for disposal D1") {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	// Round-2 reproduction: the method record must resolve, be dated, and be
	// dated when the Rev. Proc. requires.
	t.Run("method record evidence and deadlines", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s5_wallet_allocation", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["methodRecordFact"] = "f_nope"
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisWalletAllocEligibility); res.Status != vwp.StatusFail || !strings.Contains(res.Message, `methodRecordFact "f_nope" does not resolve`) {
			t.Errorf("eligibility = %s %q", res.Status, res.Message)
		}
		// A global rule recorded on 2025-01-02 is too late.
		dir = copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s5_f_allocation_record" {
					fm["value"] = map[string]any{"recordedOn": "2025-01-02", "method": "globalAllocation"}
				}
			}
		})
		editCalc(t, dir, "s5_wallet_allocation", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["allocationMethod"] = "globalAllocation"
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisWalletAllocEligibility); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "global allocation rule recorded 2025-01-02; Rev. Proc. 2024-28 §5.02(5) requires the rule in the books and records before 2025-01-01") {
			t.Errorf("eligibility = %s %q", res.Status, res.Message)
		}
		// A specific unit allocation recorded after the first 2025 disposition is too late.
		dir = copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s5_f_allocation_record" {
					fm["value"] = map[string]any{"recordedOn": "2025-04-01", "method": "specificUnit"}
				}
			}
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisWalletAllocEligibility); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "specific unit allocation recorded 2025-04-01, after the first disposition of BTC on or after 2025-01-01 (2025-03-20)") {
			t.Errorf("eligibility = %s %q", res.Status, res.Message)
		}
	})
	t.Run("safe harbor allocation must be as of 2025-01-01 and tie to pre-2025 lots", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s5_wallet_allocation", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["asOf"] = "2026-03-31"
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisWalletAllocEligibility); res.Status != vwp.StatusFail || !strings.Contains(res.Message, `asOf "2026-03-31" is not the Rev. Proc. 2024-28 safe harbor date`) {
			t.Errorf("eligibility = %s %q", res.Status, res.Message)
		}
		dir = copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			// L1 now bought in 2025: it is not in the safe harbor population.
			doc["inputs"].(map[string]any)["lots"].([]any)[0].(map[string]any)["acquiredAt"] = "2025-02-01"
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisWalletAllocEligibility); res.Status != vwp.StatusFail || !strings.Contains(res.Message, "hold 2 units / 110000 basis but the allocation covers 4 units / 190000 basis") {
			t.Errorf("eligibility = %s %q", res.Status, res.Message)
		}
	})
	t.Run("specific identification has no order constraint", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["method"] = "specificIdentification"
		})
		if res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisLotReliefMethodOrder); res.Status != vwp.StatusPass {
			t.Errorf("methodOrder = %s %q", res.Status, res.Message)
		}
	})
	t.Run("units not conserved", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			d := doc["inputs"].(map[string]any)["disposals"].([]any)[0].(map[string]any)
			d["reliefs"].([]any)[1].(map[string]any)["units"] = 0.75
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisLotReliefUnitsConserved)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "reliefs total 2.75 units, disposal is 2.5") {
			t.Errorf("unitsConserved = %s %q", res.Status, res.Message)
		}
	})
	t.Run("gain does not recompute", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s3_lot_relief", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["totalRealizedGainLoss"] = 60000.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisLotReliefRecompute)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "totalRealizedGainLoss 60000") {
			t.Errorf("recompute = %s %q", res.Status, res.Message)
		}
	})
	t.Run("wallet allocation does not tie", func(t *testing.T) {
		dir := copyFixture(t, "examples", "cost-basis-lot-relief", "package")
		editCalc(t, dir, "s5_wallet_allocation", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["allocations"].([]any)[0].(map[string]any)["allocatedBasis"] = 30000.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorCostBasisWalletAllocTieOut)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "allocated basis 90000") {
			t.Errorf("walletAllocation.tieOut = %s %q", res.Status, res.Message)
		}
	})
}

func TestStablecoinFailureModes(t *testing.T) {
	t.Run("same token classified differently", func(t *testing.T) {
		dir := copyFixture(t, "examples", "stablecoin-classification", "package")
		editJSON(t, filepath.Join(dir, "facts", "stablecoin-holdings.json"), func(doc map[string]any) {
			doc["holdings"].([]any)[1].(map[string]any)["classification"] = "cash"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStablecoinClassificationConsistent)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "DUSD (financialAssetReceivable vs cash)") {
			t.Errorf("classification.consistent = %s %q", res.Status, res.Message)
		}
	})
	t.Run("cash-like without redemption evidence", func(t *testing.T) {
		dir := copyFixture(t, "examples", "stablecoin-classification", "package")
		editJSON(t, filepath.Join(dir, "facts", "stablecoin-holdings.json"), func(doc map[string]any) {
			h := doc["holdings"].([]any)[0].(map[string]any)
			h["redemptionRight"] = "none"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStablecoinRedemptionEvidence)
		if res.Status != vwp.StatusFail || len(res.Refs) != 1 || res.Refs[0] != "h_dusd_treasury" {
			t.Errorf("redemption.evidence = %s %q %v", res.Status, res.Message, res.Refs)
		}
	})
	// A cash-equivalent conclusion is never silently passed: it has no
	// maturity, and the Practice Aid view is that stablecoins generally are
	// not cash equivalents, so the package surfaces it for a reviewer.
	t.Run("cash equivalent conclusions are surfaced for review", func(t *testing.T) {
		dir := copyFixture(t, "examples", "stablecoin-classification", "package")
		if res := result(t, verifyDir(t, dir), validators.ValidatorStablecoinCashEquivalentChallenged); res.Status != vwp.StatusPass {
			t.Errorf("fixture (receivable classification) = %s %q", res.Status, res.Message)
		}
		editJSON(t, filepath.Join(dir, "facts", "stablecoin-holdings.json"), func(doc map[string]any) {
			for _, h := range doc["holdings"].([]any) {
				if hm := h.(map[string]any); hm["token"] == "DUSD" {
					hm["classification"] = "cashEquivalent"
				}
			}
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStablecoinCashEquivalentChallenged)
		if res.Status != vwp.StatusReviewRequired || !strings.Contains(res.Message, "no maturity") || len(res.Refs) != 2 {
			t.Errorf("cashEquivalentChallenged = %s %q %v", res.Status, res.Message, res.Refs)
		}
	})
	t.Run("judgment missing", func(t *testing.T) {
		dir := copyFixture(t, "examples", "stablecoin-classification", "package")
		editJSON(t, filepath.Join(dir, "facts", "stablecoin-holdings.json"), func(doc map[string]any) {
			doc["holdings"].([]any)[2].(map[string]any)["classificationJudgmentRef"] = "j_missing"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorStablecoinJudgmentLinked)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "h_xeur_vault") {
			t.Errorf("judgmentLinked = %s %q", res.Status, res.Message)
		}
	})
	t.Run("register missing", func(t *testing.T) {
		dir := copyFixture(t, "examples", "stablecoin-classification", "package")
		removePart(t, dir, "facts/stablecoin-holdings.json")
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorStablecoinRegisterPresent); res.Status != vwp.StatusFail {
			t.Errorf("registerPresent = %s %q", res.Status, res.Message)
		}
		if _, ok := report.Result(validators.ValidatorStablecoinSchemaValid); ok {
			t.Error("schemaValid should not run when the register is missing")
		}
	})
}

// ASC 606 locates its data contracts by type when a template prefix renamed
// them, and accepts a JSON revenue schedule when no CSV could be written.
func TestASC606LocatesPrefixedAllocationByType(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	renamePart(t, dir, "calculations/ssp-allocation.json", "calculations/s4_ssp_allocation.json")
	report := verifyDir(t, dir)
	if res := result(t, report, "asc606.allocation.tieOut"); res.Status != vwp.StatusPass {
		t.Errorf("tieOut = %s %q", res.Status, res.Message)
	}
	if res := result(t, report, "asc606.revenueSchedule.total"); res.Status != vwp.StatusPass {
		t.Errorf("revenueSchedule = %s %q", res.Status, res.Message)
	}
}

func TestASC606AcceptsJSONRevenueSchedule(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	removePart(t, dir, "calculations/revenue-schedule.csv")
	addPart(t, dir, "calculations/s5_revenue_schedule.json", "calculation", map[string]any{
		"id": "s5_revenue_schedule", "type": "revenueSchedule",
		"inputs":  map[string]any{"allocation": "calc_ssp_allocation"},
		"outputs": map[string]any{"rows": []any{map[string]any{"month": "2026-01", "total": 100000}, map[string]any{"month": "2026-02", "total": 200000}}},
	})
	if res := result(t, verifyDir(t, dir), "asc606.revenueSchedule.total"); res.Status != vwp.StatusPass {
		t.Errorf("revenueSchedule = %s %q", res.Status, res.Message)
	}
	addPart(t, dir, "calculations/s5_revenue_schedule.json", "calculation", map[string]any{
		"id": "s5_revenue_schedule", "type": "revenueSchedule",
		"inputs":  map[string]any{},
		"outputs": map[string]any{"rows": []any{map[string]any{"month": "2026-01", "total": 100000}}},
	})
	if res := result(t, verifyDir(t, dir), "asc606.revenueSchedule.total"); res.Status != vwp.StatusFail || res.Message != "schedule total 100000 does not equal allocated consideration 300000" {
		t.Errorf("revenueSchedule = %s %q", res.Status, res.Message)
	}
}

// --- manifest-aware fixture surgery ---

// editManifestParts rewrites the manifest part index by hand. The integrity
// digests block is dropped with it: these edits bypass the builder, so the
// roots it recorded no longer describe the package (core.digests.match
// skips a manifest without the block).
func editManifestParts(t *testing.T, dir string, edit func(parts []any) []any) {
	t.Helper()
	editJSON(t, filepath.Join(dir, "manifest.json"), func(doc map[string]any) {
		doc["parts"] = edit(doc["parts"].([]any))
		delete(doc, "digests")
	})
}

func removePart(t *testing.T, dir, relPath string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(relPath))); err != nil {
		t.Fatal(err)
	}
	editManifestParts(t, dir, func(parts []any) []any {
		kept := parts[:0]
		for _, p := range parts {
			if p.(map[string]any)["path"] != relPath {
				kept = append(kept, p)
			}
		}
		return kept
	})
}

func renamePart(t *testing.T, dir, from, to string) {
	t.Helper()
	if err := os.Rename(filepath.Join(dir, filepath.FromSlash(from)), filepath.Join(dir, filepath.FromSlash(to))); err != nil {
		t.Fatal(err)
	}
	editManifestParts(t, dir, func(parts []any) []any {
		for _, p := range parts {
			if pm := p.(map[string]any); pm["path"] == from {
				pm["path"] = to
			}
		}
		return parts
	})
}

func addPart(t *testing.T, dir, relPath, role string, doc any) {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(relPath)), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sha := vwp.SHA256Hex(data)
	editManifestParts(t, dir, func(parts []any) []any {
		for _, p := range parts {
			if pm := p.(map[string]any); pm["path"] == relPath {
				pm["sha256"] = sha
				return parts
			}
		}
		return append(parts, map[string]any{"path": relPath, "mediaType": "application/json", "role": role, "sha256": sha})
	})
}
