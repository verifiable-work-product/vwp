package validators_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// repoRoot is where the fixture trees live (see verify_test.go in the root package).
const repoRoot = "../../../.."

func copyFixture(t *testing.T, parts ...string) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(append([]string{repoRoot}, parts...)...))); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func verifyDir(t *testing.T, dir string) *vwp.Report {
	t.Helper()
	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return validators.Verify(pkg)
}

func result(t *testing.T, r *vwp.Report, id string) vwp.ValidationResult {
	t.Helper()
	res, ok := r.Result(id)
	if !ok {
		t.Fatalf("no result for %s", id)
	}
	return res
}

func editJSON(t *testing.T, path string, edit func(doc map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	updated, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestASC606TieOutFailure(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	editJSON(t, filepath.Join(dir, "calculations", "ssp-allocation.json"), func(doc map[string]any) {
		outputs := doc["outputs"].(map[string]any)
		allocs := outputs["allocations"].([]any)
		first := allocs[0].(map[string]any)
		first["allocatedAmount"] = first["allocatedAmount"].(float64) + 100
	})

	report := verifyDir(t, dir)
	if report.Valid {
		t.Fatal("mutated allocation should fail verification")
	}
	tieOut := result(t, report, "asc606.allocation.tieOut")
	if tieOut.Status != vwp.StatusFail ||
		tieOut.Message != "allocated total 300100 does not equal transaction price 300000" {
		t.Errorf("tieOut = %s %q", tieOut.Status, tieOut.Message)
	}
	// Schedule now ties against the (mutated) allocated total and fails too.
	schedule := result(t, report, "asc606.revenueSchedule.total")
	if schedule.Status != vwp.StatusFail {
		t.Errorf("revenueSchedule = %s %q", schedule.Status, schedule.Message)
	}
}

func TestASC606FailsWithoutAllocation(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	if err := os.Remove(filepath.Join(dir, "calculations", "ssp-allocation.json")); err != nil {
		t.Fatal(err)
	}

	report := verifyDir(t, dir)
	for id, wantMsg := range map[string]string{
		"asc606.allocation.schemaValid": "calculations/ssp-allocation.json not present",
		"asc606.allocation.tieOut":      "calculations/ssp-allocation.json not present",
		"asc606.revenueSchedule.total":  "requires a schema-valid calculations/ssp-allocation.json to tie against",
	} {
		res := result(t, report, id)
		if res.Status != vwp.StatusFail || res.Message != wantMsg {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	}
}

func TestASC606RequiredSchedulesCannotBeSkipped(t *testing.T) {
	for _, tc := range []struct {
		path      string
		validator string
	}{
		{"calculations/revenue-schedule.csv", validators.ValidatorASC606RevenueScheduleTotal},
		{"calculations/accounting-schedule.json", "asc606.accountingSchedule.tieOut"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
			if err := os.Remove(filepath.Join(dir, tc.path)); err != nil {
				t.Fatal(err)
			}
			pkg, err := vwp.OpenPackage(dir)
			if err != nil {
				t.Fatal(err)
			}
			// Call the extension directly: failure must not depend on a
			// separate core manifest or hash check noticing the missing file.
			for _, res := range validators.ASC606(pkg, nil) {
				if res.ValidatorId == tc.validator {
					if res.Status != vwp.StatusFail {
						t.Fatalf("missing mandatory output: %+v", res)
					}
					return
				}
			}
			t.Fatalf("required validator %s did not run", tc.validator)
		})
	}
}

func TestASC606ScheduleWithoutTotalColumn(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	csv := "month,subscription\n2026-01,8000\n"
	if err := os.WriteFile(filepath.Join(dir, "calculations", "revenue-schedule.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	report := verifyDir(t, dir)
	res := result(t, report, "asc606.revenueSchedule.total")
	if res.Status != vwp.StatusFail || res.Message != "revenue-schedule.csv has no 'total' column" {
		t.Errorf("revenueSchedule = %s %q", res.Status, res.Message)
	}
}

func TestASC606AllocationSchemaFailure(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	editJSON(t, filepath.Join(dir, "calculations", "ssp-allocation.json"), func(doc map[string]any) {
		inputs := doc["inputs"].(map[string]any)
		delete(inputs, "transactionPrice")
	})
	report := verifyDir(t, dir)
	if res := result(t, report, "asc606.allocation.schemaValid"); res.Status != vwp.StatusFail {
		t.Errorf("allocation.schemaValid = %s %q", res.Status, res.Message)
	}
	tieOut := result(t, report, "asc606.allocation.tieOut")
	if tieOut.Status != vwp.StatusFail || tieOut.Message != "allocation failed extension schema validation" {
		t.Errorf("tieOut = %s %q", tieOut.Status, tieOut.Message)
	}
}

func TestASC820MissingRegister(t *testing.T) {
	dir := copyFixture(t, "examples", "asc820-fair-value", "package")
	if err := os.Remove(filepath.Join(dir, "facts", "fair-value-measurements.json")); err != nil {
		t.Fatal(err)
	}
	report := verifyDir(t, dir)
	res := result(t, report, "asc820.measurements.present")
	if res.Status != vwp.StatusFail {
		t.Errorf("measurements.present = %s %q", res.Status, res.Message)
	}
	if _, ok := report.Result("asc820.measurements.schemaValid"); ok {
		t.Error("schemaValid should not run when the register is missing")
	}
}

func TestASC820Level3WithoutJudgment(t *testing.T) {
	dir := copyFixture(t, "examples", "asc820-fair-value", "package")
	editJSON(t, filepath.Join(dir, "facts", "fair-value-measurements.json"), func(doc map[string]any) {
		for _, m := range doc["measurements"].([]any) {
			mm := m.(map[string]any)
			if mm["level"] == "level3" {
				delete(mm, "judgmentRef")
			}
		}
	})
	report := verifyDir(t, dir)
	res := result(t, report, "asc820.level3.judgmentLinked")
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "m_lck_2026_06_30") {
		t.Errorf("level3.judgmentLinked = %s %q", res.Status, res.Message)
	}
	if len(res.Refs) != 1 || res.Refs[0] != "m_lck_2026_06_30" {
		t.Errorf("Refs = %v", res.Refs)
	}
}

func TestASC820UnresolvablePricingSource(t *testing.T) {
	dir := copyFixture(t, "examples", "asc820-fair-value", "package")
	editJSON(t, filepath.Join(dir, "facts", "fair-value-measurements.json"), func(doc map[string]any) {
		m := doc["measurements"].([]any)[0].(map[string]any)
		m["pricingSourceEvidence"] = []any{"ev_missing"}
	})
	report := verifyDir(t, dir)
	res := result(t, report, "asc820.pricingSources.resolve")
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "m_eth_2026_06_30") {
		t.Errorf("pricingSources.resolve = %s %q", res.Status, res.Message)
	}
}

func TestWalletValidatorsFailureModes(t *testing.T) {
	t.Run("register missing", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-assets-wallet-ownership", "package")
		if err := os.Remove(filepath.Join(dir, "facts", "wallets.json")); err != nil {
			t.Fatal(err)
		}
		report := verifyDir(t, dir)
		res := result(t, report, "digitalAssets.wallets.registerPresent")
		if res.Status != vwp.StatusFail {
			t.Errorf("registerPresent = %s %q", res.Status, res.Message)
		}
	})
	t.Run("excluded wallet without rationale", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-assets-wallet-ownership", "package")
		editJSON(t, filepath.Join(dir, "facts", "wallets.json"), func(doc map[string]any) {
			for _, w := range doc["wallets"].([]any) {
				ww := w.(map[string]any)
				if inc, _ := ww["included"].(bool); !inc {
					delete(ww, "exclusionRationale")
				}
			}
		})
		report := verifyDir(t, dir)
		res := result(t, report, "digitalAssets.wallets.exclusionRationale")
		if res.Status != vwp.StatusFail {
			t.Errorf("exclusionRationale = %s %q", res.Status, res.Message)
		}
	})
}

func TestRegistryCoversBuiltInExtensions(t *testing.T) {
	reg := validators.Registry()
	for _, ext := range []string{vwp.ExtensionASC606, vwp.ExtensionASC820, vwp.ExtensionWalletControl} {
		if reg[ext] == nil {
			t.Errorf("registry missing %s", ext)
		}
	}
}
