package validators_test

import (
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func tokenIssuanceFixture(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "examples", "token-issuance", "package")
}

func editNamedRegister(t *testing.T, dir, name string, edit func(doc map[string]any)) {
	t.Helper()
	path := filepath.Join(dir, "facts", name+".json")
	editJSON(t, path, edit)
	// editJSON rewrites the part; the manifest hash must follow for the
	// package to fail only for the reason under test.
	var doc map[string]any
	editJSON(t, path, func(d map[string]any) { doc = d })
	addPart(t, dir, "facts/"+name+".json", "facts", doc)
}

func TestTokenIssuanceFailureModes(t *testing.T) {
	t.Run("register missing", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		removePart(t, dir, "facts/issuance-instruments.json")
		res := result(t, verifyDir(t, dir), validators.ValidatorTokenIssuanceRegisterPresent)
		if res.Status != vwp.StatusFail {
			t.Errorf("registerPresent = %s %q", res.Status, res.Message)
		}
		if res := result(t, verifyDir(t, dir), validators.ValidatorTokenIssuanceAllocationTiesToRegister); res.Status != vwp.StatusNotRun {
			t.Errorf("tiesToRegister = %s, want notRun without a register", res.Status)
		}
	})
	t.Run("instrument without a resolvable judgment", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		editNamedRegister(t, dir, "issuance-instruments", func(doc map[string]any) {
			doc["instruments"].([]any)[0].(map[string]any)["classificationJudgmentRef"] = "s2_j_missing"
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceRegisterJudgmentLinked, `SAFT-1 (classificationJudgmentRef "s2_j_missing" does not resolve)`)
	})
	t.Run("proceeds received but classified none", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		editNamedRegister(t, dir, "issuance-instruments", func(doc map[string]any) {
			doc["instruments"].([]any)[1].(map[string]any)["classification"] = "none"
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceRegisterJudgmentLinked, "PUB-1 received 500000 but is classified none")
	})
	t.Run("components do not sum to proceeds", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		editCalc(t, dir, "s3_proceeds_allocation", func(doc map[string]any) {
			inst := doc["inputs"].(map[string]any)["instruments"].([]any)[0].(map[string]any)
			inst["components"].([]any)[0].(map[string]any)["amount"] = 900000.0
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceAllocationRecompute, "SAFT-1 components sum to 900000, proceeds 1000000")
		expectFail(t, dir, validators.ValidatorTokenIssuanceAllocationRecompute, "byClassification.liability 1000000 does not equal the components' sum 900000")
		// The schedule still books the component amounts, so it no longer ties either.
		expectFail(t, dir, validators.ValidatorTokenIssuanceAccountingScheduleTieOut, "expected credit token_liability 900000")
	})
	t.Run("allocation proceeds differ from the register and its fact", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		editCalc(t, dir, "s3_proceeds_allocation", func(doc map[string]any) {
			inst := doc["inputs"].(map[string]any)["instruments"].([]any)[1].(map[string]any)
			inst["proceeds"] = 600000.0
			inst["components"].([]any)[0].(map[string]any)["amount"] = 600000.0
			doc["outputs"].(map[string]any)["byClassification"].(map[string]any)["deferredRevenue"] = 600000.0
			doc["outputs"].(map[string]any)["total"] = 1600000.0
		})
		expectPass(t, dir, validators.ValidatorTokenIssuanceAllocationRecompute)
		expectFail(t, dir, validators.ValidatorTokenIssuanceAllocationTiesToRegister, "PUB-1 allocates 600000 but the register records proceeds of 500000")
		editNamedRegister(t, dir, "issuance-instruments", func(doc map[string]any) {
			doc["instruments"].([]any)[1].(map[string]any)["proceeds"] = 600000.0
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceAllocationTiesToRegister, `proceeds is 600000 but fact "s1_f_public_proceeds" says 500000`)
	})
	t.Run("register instrument with proceeds left unallocated", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		editNamedRegister(t, dir, "issuance-instruments", func(doc map[string]any) {
			tre := doc["instruments"].([]any)[2].(map[string]any)
			tre["proceeds"] = 10.0
			tre["proceedsFact"] = "s1_f_saft_proceeds"
			tre["classification"] = "liability"
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceAllocationTiesToRegister, "register instrument TREAS-1 received proceeds but is not allocated")
	})
	t.Run("schedule: wrong role for the classification, wrong date, extra entry", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[1].Postings[1].AccountRole = validators.RoleTokenLiability
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceAccountingScheduleTieOut, "expected credit deferred_revenue 500000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[1].Postings[1].AccountRole = validators.RoleDeferredRevenue
			a.Entries[0].Date = "2026-04-16"
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceAccountingScheduleTieOut, `no entry with obligationId "SAFT-1" dated 2026-04-15`)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Date = "2026-04-15"
			a.Entries = append(a.Entries, accountingschedule.Entry{ID: "AIR-1", Date: "2026-06-01", Description: "airdrop", ObligationID: "AIR-1",
				Postings: []accountingschedule.Posting{{AccountRole: validators.RoleConsiderationReceived, Debit: "1.00"}, {AccountRole: validators.RoleTokenLiability, Credit: "1.00"}}})
			a.TotalAmount = "1500001.00"
		})
		expectFail(t, dir, validators.ValidatorTokenIssuanceAccountingScheduleTieOut, `not derived from any calculation: AIR-1 (obligationId "AIR-1", 2026-06-01)`)
	})
	t.Run("hybrid instrument books one credit per component", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		editCalc(t, dir, "s3_proceeds_allocation", func(doc map[string]any) {
			inst := doc["inputs"].(map[string]any)["instruments"].([]any)[0].(map[string]any)
			inst["components"] = []any{
				map[string]any{"classification": "liability", "amount": 800000.0},
				map[string]any{"classification": "derivativeLiability", "amount": 200000.0, "basis": "fair value of the token forward at inception"},
			}
			by := doc["outputs"].(map[string]any)["byClassification"].(map[string]any)
			by["liability"], by["derivativeLiability"] = 800000.0, 200000.0
		})
		expectPass(t, dir, validators.ValidatorTokenIssuanceAllocationRecompute)
		expectFail(t, dir, validators.ValidatorTokenIssuanceAccountingScheduleTieOut, "expected credit token_liability 800000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{
				{AccountRole: validators.RoleConsiderationReceived, Debit: "1000000.00"},
				{AccountRole: validators.RoleTokenLiability, Credit: "800000.00"},
				{AccountRole: validators.RoleDerivativeLiability, Credit: "200000.00"},
			}
		})
		expectPass(t, dir, validators.ValidatorTokenIssuanceAccountingScheduleTieOut)
	})
	t.Run("nothing received: no schedule required", func(t *testing.T) {
		dir := tokenIssuanceFixture(t)
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, validators.ValidatorTokenIssuanceAccountingScheduleTieOut, "is required")
		editCalc(t, dir, "s3_proceeds_allocation", func(doc map[string]any) {
			for _, i := range doc["inputs"].(map[string]any)["instruments"].([]any) {
				inst := i.(map[string]any)
				inst["proceeds"] = 0.0
				inst["components"] = []any{}
			}
			out := doc["outputs"].(map[string]any)
			out["byClassification"] = map[string]any{"liability": 0.0, "equity": 0.0, "deferredRevenue": 0.0, "derivativeLiability": 0.0}
			out["total"] = 0.0
		})
		editNamedRegister(t, dir, "issuance-instruments", func(doc map[string]any) {
			for _, i := range doc["instruments"].([]any) {
				inst := i.(map[string]any)
				inst["proceeds"] = 0.0
				delete(inst, "proceedsFact")
			}
		})
		res := result(t, verifyDir(t, dir), validators.ValidatorTokenIssuanceAccountingScheduleTieOut)
		if res.Status != vwp.StatusPass || !strings.Contains(res.Message, "book nothing") {
			t.Errorf("tieOut = %s %q", res.Status, res.Message)
		}
	})
}
