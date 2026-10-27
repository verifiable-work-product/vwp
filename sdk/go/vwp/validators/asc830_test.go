package validators_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func fxItems(doc map[string]any) []any {
	return doc["inputs"].(map[string]any)["items"].([]any)
}

func TestASC830FailureModes(t *testing.T) {
	t.Run("remeasured amount is foreign amount times spot", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc830-foreign-currency", "package")
		editCalc(t, dir, "s4_remeasurement", func(doc map[string]any) {
			rows := doc["outputs"].(map[string]any)["rows"].([]any)
			rows[0].(map[string]any)["remeasured"] = 109000.0
		})
		expectFail(t, dir, validators.ValidatorASC830RemeasurementRecompute, "item cash_eur: remeasured 109000 but 100000 × 1.1 = 110000")
	})
	t.Run("a liability's increase is a loss", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc830-foreign-currency", "package")
		editCalc(t, dir, "s4_remeasurement", func(doc map[string]any) {
			rows := doc["outputs"].(map[string]any)["rows"].([]any)
			rows[1].(map[string]any)["gainLoss"] = 1000.0
			doc["outputs"].(map[string]any)["netGainLoss"] = 3000.0
		})
		expectFail(t, dir, validators.ValidatorASC830RemeasurementRecompute, "item ap_gbp: gainLoss 1000 but the credit-balance item moved from 63000 to 64000, a loss of 1000")
		// The schedule then books it the wrong way round too.
		expectFail(t, dir, validators.ValidatorASC830AccountingScheduleTieOut, "cannot be booked")
	})
	t.Run("non-monetary items carry no difference", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc830-foreign-currency", "package")
		editCalc(t, dir, "s4_remeasurement", func(doc map[string]any) {
			rows := doc["outputs"].(map[string]any)["rows"].([]any)
			rows[2].(map[string]any)["remeasured"] = 22000.0
			rows[2].(map[string]any)["gainLoss"] = 400.0
			doc["outputs"].(map[string]any)["netGainLoss"] = 1400.0
		})
		expectFail(t, dir, validators.ValidatorASC830RemeasurementRecompute, "item prepaid_eur: non-monetary items stay at carrying amount 21600, not 22000")
		expectFail(t, dir, validators.ValidatorASC830RemeasurementMonetaryClassified, "item prepaid_eur: a non-monetary item carries a remeasurement difference of 400")
	})
	t.Run("rate fact must state the rate as of the remeasurement date", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc830-foreign-currency", "package")
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s3_f_gbp_rate" {
					fm["value"] = map[string]any{"rate": 1.28, "date": "2026-06-29", "pair": "GBP/USD"}
				}
			}
		})
		expectFail(t, dir, validators.ValidatorASC830RemeasurementRateEvidence, `item ap_gbp: fact "s3_f_gbp_rate" is a rate as of 2026-06-29, not the remeasurement date 2026-06-30`)
		editCalc(t, dir, "s4_remeasurement", func(doc map[string]any) {
			fxItems(doc)[0].(map[string]any)["spotRate"] = 1.12
		})
		expectFail(t, dir, validators.ValidatorASC830RemeasurementRateEvidence, `item cash_eur spotRate is 1.12 but fact "s3_f_eur_rate" says 1.1`)
	})
	t.Run("functional currency must be judged", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc830-foreign-currency", "package")
		editJudgments(t, dir, func(doc map[string]any) {
			for _, j := range doc["judgments"].([]any) {
				if jm := j.(map[string]any); jm["id"] == "s1_j_functional_currency" {
					jm["topic"] = "Reporting basis"
					jm["question"] = "Which basis applies?"
				}
			}
		})
		expectFail(t, dir, validators.ValidatorASC830RemeasurementMonetaryClassified, "no judgment in the package concludes on the functional currency")
	})
	t.Run("schedule direction and item roles", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc830-foreign-currency", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{{AccountRole: validators.RoleFXGainLoss, Debit: "2000.00"}, {AccountRole: "cash_eur", Credit: "2000.00"}}
		})
		expectFail(t, dir, validators.ValidatorASC830AccountingScheduleTieOut, "item cash_eur on 2026-06-30: expected debit cash_eur 2000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{{AccountRole: "cash_usd", Debit: "2000.00"}, {AccountRole: validators.RoleFXGainLoss, Credit: "2000.00"}}
		})
		expectFail(t, dir, validators.ValidatorASC830AccountingScheduleTieOut, "expected debit cash_eur 2000")
	})
	t.Run("nothing to remeasure needs no schedule", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc830-foreign-currency", "package")
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, validators.ValidatorASC830AccountingScheduleTieOut, "is required")
		editCalc(t, dir, "s4_remeasurement", func(doc map[string]any) {
			items := fxItems(doc)
			items[0].(map[string]any)["carryingAmount"] = 110000.0
			items[1].(map[string]any)["carryingAmount"] = 64000.0
			rows := doc["outputs"].(map[string]any)["rows"].([]any)
			rows[0].(map[string]any)["gainLoss"] = 0.0
			rows[1].(map[string]any)["gainLoss"] = 0.0
			doc["outputs"].(map[string]any)["netGainLoss"] = 0.0
		})
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				switch fm := f.(map[string]any); fm["id"] {
				case "s2_f_cash_eur":
					fm["value"].(map[string]any)["carryingAmount"] = 110000.0
				case "s2_f_ap_gbp":
					fm["value"].(map[string]any)["carryingAmount"] = 64000.0
				}
			}
		})
		expectPass(t, dir, validators.ValidatorASC830AccountingScheduleTieOut)
	})
}

// editJudgments mutates judgments/judgments.json and rewrites its manifest hash.
func editJudgments(t *testing.T, dir string, edit func(doc map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "judgments", "judgments.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	addPart(t, dir, "judgments/judgments.json", "judgments", doc)
}
