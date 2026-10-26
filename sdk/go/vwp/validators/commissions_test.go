package validators_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// A commission package: the statement calculation values each period's
// commission (earned in CC) into USD, and the executable schedule books one
// accrual entry per period — expense against the liability to the payee.
func commissionPackage(t *testing.T, withArtifact bool, entries []map[string]any, total string) string {
	t.Helper()
	dir := t.TempDir()
	b, err := builder.Scaffold(dir, builder.ScaffoldConfig{
		PackageId: "pkg_commission_test",
		CreatedAt: "2026-10-26T12:00:00Z",
		Profile:   vwp.ProfileFinance,
		Extension: vwp.ExtensionCommissions,
		Title:     "Commissions — super validator rewards",
		Workproduct: vwp.Workproduct{
			Id: "wp_commission_test", Title: "Commissions — super validator rewards",
			Domain: "accounting.commissions", Profile: vwp.ProfileFinance, Extension: vwp.ExtensionCommissions, Status: "draft",
		},
		Provenance: vwp.Provenance{
			CreatedAt: "2026-10-26T12:00:00Z",
			CreatedBy: vwp.ProvenanceAgent{Type: "system", Name: "commissions-test"},
		},
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	rows := make([]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, e)
	}
	if err := b.AddCalculation(vwp.Calculation{
		Id: "s3_commission_statement", Type: "commissionStatement",
		Inputs: map[string]any{
			"currency":     "USD",
			"denomination": map[string]any{"asset": "CC", "scale": 10},
			"planId":       "cp-1", "statementId": "cs-1",
			"rates":           map[string]any{"2026-01": "0.0500", "2026-02": "0.0400"},
			"valuationSource": "month-end CC/USD close supplied by finance",
		},
		Outputs: map[string]any{
			"schedule": map[string]any{"currency": "USD", "amountScale": 2, "entries": rows, "total": total},
		},
	}); err != nil {
		t.Fatalf("AddCalculation: %v", err)
	}
	if withArtifact {
		artifactEntries := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			amount := e["amount"].(string)
			artifactEntries = append(artifactEntries, map[string]any{
				"id": "commission-" + e["period"].(string), "date": e["date"], "obligationId": e["period"],
				"description": "Commission earned " + e["period"].(string),
				"postings": []map[string]any{
					{"accountRole": validators.RoleCommissionExpense, "debit": amount},
					{"accountRole": validators.RoleAccruedCommissionsPayable, "credit": amount},
				},
			})
		}
		artifact := map[string]any{
			"schemaVersion": 1, "name": "Commission accrual", "kind": accountingschedule.KindCommissionAccrual,
			"currency": "USD", "amountScale": 2, "startDate": "2026-01-31", "endDate": "2026-02-28",
			"totalAmount": total, "entries": artifactEntries,
		}
		raw, _ := json.Marshal(artifact)
		if err := b.AddFilePart(accountingschedule.ArtifactPath, "application/json", builder.RoleCalculation, raw); err != nil {
			t.Fatalf("AddFilePart: %v", err)
		}
	}
	return dir
}

var commissionEntries = []map[string]any{
	{"period": "2026-01", "date": "2026-01-31", "commission": "209769.83029005", "rate": "0.0500", "amount": "10488.49"},
	{"period": "2026-02", "date": "2026-02-28", "commission": "66844.1043897", "rate": "0.0400", "amount": "2673.76"},
}

func TestCommissionKindIsAcceptedByTheScheduleContract(t *testing.T) {
	found := false
	for _, k := range accountingschedule.Kinds() {
		if k == accountingschedule.KindCommissionAccrual {
			found = true
		}
	}
	if !found {
		t.Fatalf("Kinds() = %v, want commission_accrual", accountingschedule.Kinds())
	}
	outputs := vwp.RequiredOutputs(vwp.ExtensionCommissions)
	if len(outputs) != 1 || outputs[0].Path != accountingschedule.ArtifactPath || outputs[0].Condition == "" ||
		outputs[0].Validators[1] != vwp.ValidatorCommissionsAccountingScheduleTieOut {
		t.Fatalf("required outputs = %+v", outputs)
	}
	if !vwp.RequiresAccountingSchedule(vwp.ExtensionCommissions) {
		t.Fatal("the commissions extension carries the executable schedule")
	}
}

func TestCommissionStatementBooksOneAccrualPerPeriod(t *testing.T) {
	dir := commissionPackage(t, true, commissionEntries, "13162.25")
	report := verifyDir(t, dir)
	for _, id := range []string{
		vwp.ValidatorRequiredOutputsExist, vwp.ValidatorAccountingScheduleValid,
		validators.ValidatorCommissionsStatementSchemaValid, validators.ValidatorCommissionsStatementRecompute,
		validators.ValidatorCommissionsAccountingScheduleTieOut,
	} {
		if res := result(t, report, id); res.Status != vwp.StatusPass {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	}
}

func TestCommissionScheduleMustMatchTheStatement(t *testing.T) {
	id := validators.ValidatorCommissionsAccountingScheduleTieOut
	t.Run("a wrong amount fails", func(t *testing.T) {
		dir := commissionPackage(t, true, commissionEntries, "13162.25")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Entries[0].Postings[0].Debit = "10488.50" })
		expectFail(t, dir, id, "2026-01")
	})
	t.Run("a wrong kind fails", func(t *testing.T) {
		dir := commissionPackage(t, true, commissionEntries, "13162.25")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) { a.Kind = accountingschedule.KindInterestAccrual })
		expectFail(t, dir, id, "commission_accrual")
	})
	t.Run("a missing schedule fails when periods earn a commission", func(t *testing.T) {
		dir := commissionPackage(t, false, commissionEntries, "13162.25")
		expectFail(t, dir, id, "required")
	})
	t.Run("nothing earned, nothing to book", func(t *testing.T) {
		dir := commissionPackage(t, false, []map[string]any{
			{"period": "2026-01", "date": "2026-01-31", "commission": "0", "rate": "0.0500", "amount": "0.00"},
		}, "0.00")
		expectPass(t, dir, id)
	})
	t.Run("the statement must recompute: amount is commission × rate at the schedule scale", func(t *testing.T) {
		dir := commissionPackage(t, true, []map[string]any{
			{"period": "2026-01", "date": "2026-01-31", "commission": "100", "rate": "0.0500", "amount": "6.00"},
		}, "6.00")
		expectFail(t, dir, validators.ValidatorCommissionsStatementRecompute, "2026-01")
	})
}

// The fixture must be a real package the SDK would sign: no part left unhashed.
func TestCommissionFixtureIsWellFormed(t *testing.T) {
	dir := commissionPackage(t, true, commissionEntries, "13162.25")
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), vwp.ExtensionCommissions) || !strings.Contains(string(raw), accountingschedule.ArtifactPath) {
		t.Fatalf("manifest = %s", raw)
	}
}
