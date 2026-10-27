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

func custodyFixture(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "examples", "custody-safeguarding", "package")
}

// editCustodyRegister mutates facts/custodied-assets.json and rewrites its
// manifest hash.
func editCustodyRegister(t *testing.T, dir string, edit func(doc map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "facts", "custodied-assets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	addPart(t, dir, "facts/custodied-assets.json", "facts", doc)
}

func custodyHolding(doc map[string]any, id string) map[string]any {
	for _, h := range doc["holdings"].([]any) {
		if hm := h.(map[string]any); hm["id"] == id {
			return hm
		}
	}
	return nil
}

func TestCustodySafeguardingFailureModes(t *testing.T) {
	t.Run("register missing", func(t *testing.T) {
		dir := custodyFixture(t)
		removePart(t, dir, "facts/custodied-assets.json")
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorCustodyRegisterPresent); res.Status != vwp.StatusFail {
			t.Errorf("present = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorCustodyRegisterJudgmentLinked); res.Status != vwp.StatusNotRun {
			t.Errorf("judgmentLinked = %s, want notRun", res.Status)
		}
		// The artifact still exists but nothing derives it: the tie-out fails
		// (the shared matcher reports the absent currency declaration first).
		if res := result(t, report, validators.ValidatorCustodyAccountingScheduleTieOut); res.Status != vwp.StatusFail {
			t.Errorf("tieOut = %s %q, want fail", res.Status, res.Message)
		}
	})
	t.Run("liability recognized without a probable and estimable loss", func(t *testing.T) {
		dir := custodyFixture(t)
		editCustodyRegister(t, dir, func(doc map[string]any) {
			custodyHolding(doc, "h_btc_omnibus")["recognizedLiability"] = 1000.0
		})
		expectFail(t, dir, validators.ValidatorCustodyRegisterJudgmentLinked, "h_btc_omnibus recognizes 1000 but the loss is not declared both probable and estimable")
	})
	t.Run("probable and estimable loss without an accrual", func(t *testing.T) {
		dir := custodyFixture(t)
		editCustodyRegister(t, dir, func(doc map[string]any) {
			custodyHolding(doc, "h_eth_incident")["recognizedLiability"] = 0.0
		})
		expectFail(t, dir, validators.ValidatorCustodyRegisterJudgmentLinked, "h_eth_incident declares the loss probable and estimable but recognizes no liability")
	})
	t.Run("judgment reference does not resolve", func(t *testing.T) {
		dir := custodyFixture(t)
		editCustodyRegister(t, dir, func(doc map[string]any) {
			custodyHolding(doc, "h_eth_subcustody")["judgmentRef"] = "s4_j_missing"
		})
		expectFail(t, dir, validators.ValidatorCustodyRegisterJudgmentLinked, "holdings without a resolvable judgmentRef: h_eth_subcustody")
	})
	t.Run("valuation: fair value, price and price date", func(t *testing.T) {
		dir := custodyFixture(t)
		editCustodyRegister(t, dir, func(doc map[string]any) {
			custodyHolding(doc, "h_btc_omnibus")["fairValue"] = 6400000.0
		})
		expectFail(t, dir, validators.ValidatorCustodyRegisterValuationRecompute, "h_btc_omnibus fairValue 6400000 does not equal units 100 × unitPrice 65000 = 6500000")
		editCustodyRegister(t, dir, func(doc map[string]any) {
			h := custodyHolding(doc, "h_btc_omnibus")
			h["fairValue"], h["unitPrice"] = 6600000.0, 66000.0
		})
		expectFail(t, dir, validators.ValidatorCustodyRegisterValuationRecompute, `h_btc_omnibus unitPrice is 66000 but fact "s3_f_btc_price" says 65000`)
		editCustodyRegister(t, dir, func(doc map[string]any) {
			h := custodyHolding(doc, "h_btc_omnibus")
			h["fairValue"], h["unitPrice"] = 6500000.0, 65000.0
		})
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s3_f_btc_price" {
					fm["value"] = map[string]any{"price": 65000.0, "date": "2026-06-29", "currency": "USD"}
				}
			}
		})
		expectFail(t, dir, validators.ValidatorCustodyRegisterValuationRecompute, `fact "s3_f_btc_price" is a price as of 2026-06-29, not the register date 2026-06-30`)
	})
	t.Run("control evidence missing or unresolvable, and an agent with a return obligation", func(t *testing.T) {
		dir := custodyFixture(t)
		editCustodyRegister(t, dir, func(doc map[string]any) {
			delete(custodyHolding(doc, "h_btc_omnibus"), "controlEvidenceRef")
			custodyHolding(doc, "h_eth_subcustody")["controlEvidenceRef"] = "ev_nowhere"
			custodyHolding(doc, "h_eth_incident")["holderRole"] = "agent"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorCustodyRegisterControlEvidence)
		for _, want := range []string{"h_btc_omnibus is obligated to return the assets but cites no controlEvidenceRef", `h_eth_subcustody cites controlEvidenceRef "ev_nowhere" which is neither evidence nor a fact`, "h_eth_incident is held as agent yet declares an obligation to return"} {
			if res.Status != vwp.StatusFail || !strings.Contains(res.Message, want) {
				t.Errorf("controlEvidence = %s %q; want %q", res.Status, res.Message, want)
			}
		}
	})
	t.Run("schema: a holding without the loss assessment fields", func(t *testing.T) {
		dir := custodyFixture(t)
		editCustodyRegister(t, dir, func(doc map[string]any) {
			delete(custodyHolding(doc, "h_btc_omnibus"), "lossProbable")
		})
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorCustodyRegisterSchemaValid); res.Status != vwp.StatusFail {
			t.Errorf("schemaValid = %s %q", res.Status, res.Message)
		}
		if res := result(t, report, validators.ValidatorCustodyRegisterValuationRecompute); res.Status != vwp.StatusNotRun {
			t.Errorf("valuationRecompute = %s, want notRun", res.Status)
		}
	})
}

func TestCustodyScheduleTieOut(t *testing.T) {
	id := validators.ValidatorCustodyAccountingScheduleTieOut
	t.Run("nothing accrued: the schedule is not required", func(t *testing.T) {
		dir := custodyFixture(t)
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, id, "is required")
		editCustodyRegister(t, dir, func(doc map[string]any) {
			h := custodyHolding(doc, "h_eth_incident")
			h["lossProbable"], h["lossEstimable"], h["recognizedLiability"] = false, false, 0.0
		})
		res := result(t, verifyDir(t, dir), id)
		if res.Status != vwp.StatusPass || !strings.Contains(res.Message, "book nothing") {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	})
	t.Run("accrual amount, side and date", func(t *testing.T) {
		dir := custodyFixture(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings[0].Debit, a.Entries[0].Postings[1].Credit, a.TotalAmount = "22000.00", "22000.00", "22000.00"
		})
		expectFail(t, dir, id, "expected debit safeguarding_loss 22500")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{{AccountRole: validators.RoleSafeguardingLiability, Debit: "22500.00"}, {AccountRole: validators.RoleSafeguardingLoss, Credit: "22500.00"}}
			a.TotalAmount = "22500.00"
		})
		expectFail(t, dir, id, "expected debit safeguarding_loss 22500")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{{AccountRole: validators.RoleSafeguardingLoss, Debit: "22500.00"}, {AccountRole: validators.RoleSafeguardingLiability, Credit: "22500.00"}}
			a.StartDate, a.Entries[0].Date = "2026-06-12", "2026-06-12"
		})
		expectFail(t, dir, id, `no entry with obligationId "h_eth_incident" dated 2026-06-30`)
	})
	t.Run("an accrual for a population that recognizes nothing", func(t *testing.T) {
		dir := custodyFixture(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries = append(a.Entries, accountingschedule.Entry{ID: "h_btc_omnibus-2026-06-30", Date: "2026-06-30", Description: "general reserve", ObligationID: "h_btc_omnibus",
				Postings: []accountingschedule.Posting{{AccountRole: validators.RoleSafeguardingLoss, Debit: "100.00"}, {AccountRole: validators.RoleSafeguardingLiability, Credit: "100.00"}}})
			a.TotalAmount = "22600.00"
		})
		expectFail(t, dir, id, `entries not derived from any calculation: h_btc_omnibus-2026-06-30 (obligationId "h_btc_omnibus", 2026-06-30)`)
	})
	t.Run("currency must match the register", func(t *testing.T) {
		dir := custodyFixture(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Currency = "EUR" })
		expectFail(t, dir, id, "currency EUR but the calculations are in USD")
	})
}
