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

const asc450Register = "facts/contingencies.json"

// editRegister mutates a register part and rewrites its manifest hash.
func editASC450Register(t *testing.T, dir, rel string, edit func(doc map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	addPart(t, dir, rel, "facts", doc)
}

func asc450Item(doc map[string]any, id string) map[string]any {
	for _, it := range doc["items"].([]any) {
		if m := it.(map[string]any); m["id"] == id {
			return m
		}
	}
	return nil
}

func TestASC450FailureModes(t *testing.T) {
	fixture := []string{"examples", "asc450-contingencies", "package"}
	t.Run("probable and estimable must accrue the low end when no best estimate", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			asc450Item(doc, "c_supplier_litigation")["accrued"] = 650000.0
		})
		expectFail(t, dir, validators.ValidatorASC450RegisterAccrualConsistent, "accrued 650000 but a probable, estimable loss accrues the low end of the range (ASC 450-20-30-1, no best estimate): 400000")
		// An inconsistent register cannot be booked, whatever the artifact says.
		expectFail(t, dir, validators.ValidatorASC450AccountingScheduleTieOut, "cannot be booked")
	})
	t.Run("a best estimate inside the range is what accrues", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			it := asc450Item(doc, "c_supplier_litigation")
			it["bestEstimate"] = 600000.0
			it["accrued"] = 600000.0
		})
		// A best estimate is evidence too: the cited fact must state it.
		expectFail(t, dir, validators.ValidatorASC450RegisterAccrualConsistent, `bestEstimate cites fact "s1_f_litigation_range" which carries no numeric value`)
		editFacts(t, dir, func(doc map[string]any) {
			for _, f := range doc["facts"].([]any) {
				if fm := f.(map[string]any); fm["id"] == "s1_f_litigation_range" {
					fm["value"].(map[string]any)["best"] = 600000.0
				}
			}
		})
		expectPass(t, dir, validators.ValidatorASC450RegisterAccrualConsistent)
		expectFail(t, dir, validators.ValidatorASC450AccountingScheduleTieOut, "expected debit contingency_loss 600000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings[0].Debit, a.Entries[0].Postings[1].Credit, a.TotalAmount = "600000.00", "600000.00", "600000.00"
		})
		expectPass(t, dir, validators.ValidatorASC450AccountingScheduleTieOut)
	})
	t.Run("reasonably possible losses are not accrued and are disclosed", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			it := asc450Item(doc, "c_regulatory_inquiry")
			it["accrued"] = 50000.0
			it["disclosureRequired"] = false
		})
		expectFail(t, dir, validators.ValidatorASC450RegisterAccrualConsistent, "accrued 50000 although the loss is reasonablyPossible")
		expectFail(t, dir, validators.ValidatorASC450RegisterDisclosureConsistent, "reasonably possible but disclosureRequired is false (ASC 450-20-50-3)")
		expectFail(t, dir, validators.ValidatorASC450AccountingScheduleTieOut, "cannot be booked")
	})
	t.Run("probable but not estimable: disclose, do not accrue", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			it := asc450Item(doc, "c_supplier_litigation")
			it["estimable"] = false
		})
		expectFail(t, dir, validators.ValidatorASC450RegisterAccrualConsistent, "not reasonably estimable")
	})
	t.Run("remote guarantees are still disclosed", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			asc450Item(doc, "c_affiliate_guarantee")["disclosureRequired"] = false
		})
		expectFail(t, dir, validators.ValidatorASC450RegisterDisclosureConsistent, "remote guarantee-type contingency is still disclosed (ASC 450-20-50-6)")
	})
	t.Run("range must equal the cited estimate fact", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			it := asc450Item(doc, "c_supplier_litigation")
			it["estimateLow"] = 300000.0
			it["accrued"] = 300000.0
		})
		expectFail(t, dir, validators.ValidatorASC450RegisterAccrualConsistent, `estimateLow is 300000 but fact "s1_f_litigation_range" says 400000`)
	})
	t.Run("judgment and evidence must resolve", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			asc450Item(doc, "c_regulatory_inquiry")["judgmentRef"] = "j_missing"
			asc450Item(doc, "c_affiliate_guarantee")["evidenceRef"] = "ev_missing"
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC450RegisterJudgmentLinked)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `judgmentRef "j_missing"`) || !strings.Contains(res.Message, `evidenceRef "ev_missing"`) {
			t.Errorf("judgmentLinked = %s %q", res.Status, res.Message)
		}
	})
	t.Run("going concern evaluation is required", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		editJSON(t, filepath.Join(dir, "judgments", "judgments.json"), func(doc map[string]any) {
			for _, j := range doc["judgments"].([]any) {
				if jm := j.(map[string]any); jm["id"] == "s4_j_going_concern" {
					jm["topic"] = "Liquidity"
					jm["question"] = "Is cash sufficient?"
				}
			}
		})
		expectFail(t, dir, validators.ValidatorASC450GoingConcernJudged, "no judgment addresses going concern / substantial doubt")
	})
	t.Run("nothing accrued means no schedule", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, validators.ValidatorASC450AccountingScheduleTieOut, "is required")
		editASC450Register(t, dir, asc450Register, func(doc map[string]any) {
			it := asc450Item(doc, "c_supplier_litigation")
			it["likelihood"] = "reasonablyPossible"
			it["accrued"] = 0.0
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorASC450AccountingScheduleTieOut)
		if res.Status != vwp.StatusPass || !strings.Contains(res.Message, "book nothing") {
			t.Errorf("tieOut = %s %q", res.Status, res.Message)
		}
	})
	t.Run("register missing", func(t *testing.T) {
		dir := copyFixture(t, fixture...)
		removePart(t, dir, asc450Register)
		report := verifyDir(t, dir)
		if res := result(t, report, validators.ValidatorASC450RegisterPresent); res.Status != vwp.StatusFail {
			t.Errorf("registerPresent = %s %q", res.Status, res.Message)
		}
		if _, ok := report.Result(validators.ValidatorASC450RegisterSchemaValid); ok {
			t.Error("schemaValid should not run when the register is missing")
		}
	})
}
