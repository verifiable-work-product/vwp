package validators_test

import (
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func defiFixture(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "examples", "defi-positions", "package")
}

func TestDeFiPositionsFailureModes(t *testing.T) {
	t.Run("register missing", func(t *testing.T) {
		dir := defiFixture(t)
		removePart(t, dir, "facts/defi-positions.json")
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorDeFiRegisterPresent); res.Status != vwp.StatusFail {
			t.Errorf("registerPresent = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorDeFiValuationTiesToRegister); res.Status != vwp.StatusNotRun {
			t.Errorf("tiesToRegister = %s, want notRun", res.Status)
		}
		if res := result(t, report, validators.ValidatorDeFiValuationRecompute); res.Status != vwp.StatusPass {
			t.Errorf("recompute = %s %q; the valuation stands on its own", res.Status, res.Message)
		}
	})
	t.Run("derecognized but control retained", func(t *testing.T) {
		dir := defiFixture(t)
		editNamedRegister(t, dir, "defi-positions", func(doc map[string]any) {
			doc["positions"].([]any)[0].(map[string]any)["controlRetained"] = true
		})
		expectFail(t, dir, validators.ValidatorDeFiRegisterJudgmentLinked, "pos_lp_eth_dusd derecognizes assets it says it still controls")
	})
	t.Run("judgment and custody evidence must resolve", func(t *testing.T) {
		dir := defiFixture(t)
		editNamedRegister(t, dir, "defi-positions", func(doc map[string]any) {
			p := doc["positions"].([]any)[1].(map[string]any)
			p["treatmentJudgmentRef"] = "s2_j_nope"
			p["custodyEvidenceRef"] = "ev_nope"
		})
		expectFail(t, dir, validators.ValidatorDeFiRegisterJudgmentLinked, `pos_lend_eth (treatmentJudgmentRef "s2_j_nope" does not resolve)`)
		expectFail(t, dir, validators.ValidatorDeFiRegisterJudgmentLinked, `pos_lend_eth (custodyEvidenceRef "ev_nope" is neither evidence nor a fact in the package)`)
	})
	t.Run("fair value must be units times price", func(t *testing.T) {
		dir := defiFixture(t)
		editCalc(t, dir, "s3_position_valuation", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["positions"].([]any)[2].(map[string]any)["fairValue"] = 640000.0
		})
		expectFail(t, dir, validators.ValidatorDeFiValuationRecompute, "pos_wrapped_btc fairValue 640000 does not equal Σ units × price = 650000")
		editCalc(t, dir, "s3_position_valuation", func(doc map[string]any) {
			doc["outputs"].(map[string]any)["totalFairValue"] = 2015000.0
		})
		expectFail(t, dir, validators.ValidatorDeFiValuationRecompute, "totalFairValue 2015000 does not equal the sum of positions 2025000")
	})
	t.Run("price fact must state the price on the valuation date", func(t *testing.T) {
		dir := defiFixture(t)
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s3_f_price_btc" {
					fm["value"] = map[string]any{"price": 65000.0, "date": "2026-06-29"}
				}
			}
		})
		expectFail(t, dir, validators.ValidatorDeFiValuationPricingEvidence, `fact "s3_f_price_btc" is a price as of 2026-06-29, not the valuation date 2026-06-30`)
		editCalc(t, dir, "s3_position_valuation", func(doc map[string]any) {
			c := doc["inputs"].(map[string]any)["positions"].([]any)[1].(map[string]any)["components"].([]any)[0].(map[string]any)
			c["unitPrice"] = 2600.0
			doc["outputs"].(map[string]any)["positions"].([]any)[1].(map[string]any)["fairValue"] = 260000.0
			doc["outputs"].(map[string]any)["totalFairValue"] = 2035000.0
		})
		expectFail(t, dir, validators.ValidatorDeFiValuationPricingEvidence, `unitPrice is 2600 but fact "s3_f_price_eth" says 2500`)
	})
	t.Run("valuation must carry the register's units and cover every position", func(t *testing.T) {
		dir := defiFixture(t)
		editCalc(t, dir, "s3_position_valuation", func(doc map[string]any) {
			c := doc["inputs"].(map[string]any)["positions"].([]any)[0].(map[string]any)["components"].([]any)[0].(map[string]any)
			c["units"] = 240.0
			doc["outputs"].(map[string]any)["positions"].([]any)[0].(map[string]any)["fairValue"] = 1100000.0
			doc["outputs"].(map[string]any)["totalFairValue"] = 2000000.0
		})
		expectPass(t, dir, validators.ValidatorDeFiValuationRecompute)
		expectFail(t, dir, validators.ValidatorDeFiValuationTiesToRegister, "pos_lp_eth_dusd values 240 ETH but the register holds 250")
		editCalc(t, dir, "s3_position_valuation", func(doc map[string]any) {
			in := doc["inputs"].(map[string]any)
			in["positions"] = in["positions"].([]any)[:2]
			out := doc["outputs"].(map[string]any)
			out["positions"] = out["positions"].([]any)[:2]
			out["totalFairValue"] = 1350000.0
		})
		expectFail(t, dir, validators.ValidatorDeFiValuationTiesToRegister, "register position pos_wrapped_btc is not valued")
	})
	t.Run("valuation date and currency must match the register", func(t *testing.T) {
		dir := defiFixture(t)
		editCalc(t, dir, "s3_position_valuation", func(doc map[string]any) {
			doc["inputs"].(map[string]any)["currency"] = "EUR"
		})
		expectFail(t, dir, validators.ValidatorDeFiValuationTiesToRegister, "currency EUR but the register is in USD")
	})
}
