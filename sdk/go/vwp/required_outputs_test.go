package vwp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

func TestRequiredOutputsContract(t *testing.T) {
	want := []vwp.RequiredOutput{
		{Kind: "sspAllocation", Path: vwp.ASC606SSPAllocationPath, MediaType: "application/json", CalculationType: vwp.CalcTypeSSPAllocation,
			Validators: []string{"asc606.allocation.schemaValid", "asc606.allocation.tieOut"}},
		{Kind: "revenueSchedule", Path: vwp.ASC606RevenueSchedulePath, MediaType: "text/csv", CalculationType: vwp.CalcTypeRevenueSchedule,
			Validators: []string{"asc606.revenueSchedule.total"}},
		{Kind: "accountingSchedule", Path: accountingschedule.ArtifactPath, MediaType: "application/json", CalculationType: vwp.CalcTypeAccountingSchedule, SchemaVersion: 1,
			Validators: []string{vwp.ValidatorAccountingScheduleValid, vwp.ValidatorASC606AccountingScheduleTieOut}},
	}
	got := vwp.RequiredOutputs(vwp.ExtensionASC606)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("required outputs = %+v, want %+v", got, want)
	}
	got[0].Path = "changed"
	if !reflect.DeepEqual(vwp.RequiredOutputs(vwp.ExtensionASC606), want) {
		t.Fatal("caller changed the required output contract")
	}
	for _, extension := range []string{"", "asc606", vwp.ExtensionASC820, vwp.ExtensionWalletControl, vwp.ExtensionStablecoin} {
		if got := vwp.RequiredOutputs(extension); len(got) != 0 {
			t.Errorf("unexpected required outputs for %q: %+v", extension, got)
		}
		if vwp.RequiresAccountingSchedule(extension) {
			t.Errorf("%q must not require an accounting schedule", extension)
		}
	}
}

// The schedule-bearing extensions require the executable artifact
// conditionally: the contract names the condition and the validators that
// prove it, and only the extension tie-out may excuse its absence.
func TestScheduleBearingExtensionsRequireTheArtifactConditionally(t *testing.T) {
	tieOuts := map[string]string{
		vwp.ExtensionASC35060:      vwp.ValidatorASC35060AccountingScheduleTieOut,
		vwp.ExtensionASC35030:      vwp.ValidatorASC35030AccountingScheduleTieOut,
		vwp.ExtensionStakingReward: vwp.ValidatorStakingAccountingScheduleTieOut,
		vwp.ExtensionLending:       vwp.ValidatorLendingAccountingScheduleTieOut,
		vwp.ExtensionCostBasis:     vwp.ValidatorCostBasisAccountingScheduleTieOut,
	}
	for extension, tieOut := range tieOuts {
		outputs := vwp.RequiredOutputs(extension)
		if len(outputs) != 1 {
			t.Fatalf("%s: outputs = %+v", extension, outputs)
		}
		o := outputs[0]
		if o.Kind != "accountingSchedule" || o.Path != accountingschedule.ArtifactPath || o.MediaType != "application/json" ||
			o.CalculationType != vwp.CalcTypeAccountingSchedule || o.SchemaVersion != accountingschedule.SchemaVersion ||
			o.Condition == "" || !reflect.DeepEqual(o.Validators, []string{vwp.ValidatorAccountingScheduleValid, tieOut}) {
			t.Errorf("%s: output = %+v", extension, o)
		}
		if !vwp.RequiresAccountingSchedule(extension) {
			t.Errorf("%s must require an accounting schedule", extension)
		}
	}
	// ASC 606 stays unconditional.
	for _, o := range vwp.RequiredOutputs(vwp.ExtensionASC606) {
		if o.Condition != "" {
			t.Errorf("ASC 606 output %s must be unconditional", o.Kind)
		}
	}
}

// Core polices the manifest binding of a conditional output once it is
// written, and says nothing about its absence — the extension's tie-out
// owns that decision.
func TestCoreConditionalOutputAbsentOrBound(t *testing.T) {
	t.Run("absent: core passes, no artifact check runs", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		removeFixturePart(t, dir, accountingschedule.ArtifactPath)
		report := verifyCoreDir(t, dir)
		assertStatus(t, report, vwp.ValidatorRequiredOutputsExist, vwp.StatusPass)
		if _, ok := report.Result(vwp.ValidatorAccountingScheduleValid); ok {
			t.Error("accountingSchedule.valid ran without an artifact to validate")
		}
	})
	t.Run("present: must be a calculation part with the declared media type", func(t *testing.T) {
		dir := copyFixture(t, "examples", "staking-rewards", "package")
		editManifest(t, dir, func(m *vwp.Manifest) {
			for i := range m.Parts {
				if m.Parts[i].Path == accountingschedule.ArtifactPath {
					m.Parts[i].Role = "evidence"
				}
			}
		})
		report := verifyCoreDir(t, dir)
		assertStatus(t, report, vwp.ValidatorRequiredOutputsExist, vwp.StatusFail)
		assertStatus(t, report, vwp.ValidatorAccountingScheduleValid, vwp.StatusPass)
	})
	t.Run("present but invalid", func(t *testing.T) {
		dir := copyFixture(t, "examples", "digital-asset-lending", "package")
		path := filepath.Join(dir, filepath.FromSlash(accountingschedule.ArtifactPath))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw = []byte(strings.Replace(string(raw), `"kind": "interest_accrual"`, `"kind": "interest"`, 1))
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		editManifest(t, dir, func(m *vwp.Manifest) {
			for i := range m.Parts {
				if m.Parts[i].Path == accountingschedule.ArtifactPath {
					m.Parts[i].SHA256 = vwp.SHA256Hex(raw)
				}
			}
		})
		report := verifyCoreDir(t, dir)
		assertStatus(t, report, vwp.ValidatorRequiredOutputsExist, vwp.StatusPass)
		assertStatus(t, report, vwp.ValidatorAccountingScheduleValid, vwp.StatusFail)
	})
}

func editManifest(t *testing.T, dir string, edit func(*vwp.Manifest)) {
	t.Helper()
	path := filepath.Join(dir, vwp.ManifestPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := vwp.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	edit(manifest)
	raw, err = manifest.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

// removeFixturePart deletes a part and its manifest reference.
func removeFixturePart(t *testing.T, dir, relPath string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(relPath))); err != nil {
		t.Fatal(err)
	}
	editManifest(t, dir, func(m *vwp.Manifest) {
		kept := m.Parts[:0]
		for _, p := range m.Parts {
			if p.Path != relPath {
				kept = append(kept, p)
			}
		}
		m.Parts = kept
	})
}

func verifyCoreDir(t *testing.T, dir string) *vwp.Report {
	t.Helper()
	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return vwp.Verify(pkg)
}

func TestCoreRequiresManifestBoundASC606OutputsWithoutExtensionValidators(t *testing.T) {
	valid := vwp.Verify(openFixture(t, "examples", "asc606-revenue-contract", "package"))
	if !valid.Valid {
		t.Fatalf("complete package failed core verification: %+v", valid.Results)
	}
	assertStatus(t, valid, vwp.ValidatorRequiredOutputsExist, vwp.StatusPass)
	assertStatus(t, valid, vwp.ValidatorAccountingScheduleValid, vwp.StatusPass)
	for _, output := range vwp.RequiredOutputs(vwp.ExtensionASC606) {
		for _, mutation := range []string{"missing", "wrong path", "unmanifested", "wrong role", "wrong media type", "unhashed", "duplicate", "signature instead of part"} {
			t.Run(output.Kind+"/"+mutation, func(t *testing.T) {
				dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
				editManifest(t, dir, func(m *vwp.Manifest) {
					for i := range m.Parts {
						if m.Parts[i].Path != output.Path {
							continue
						}
						switch mutation {
						case "missing":
							if err := os.Remove(filepath.Join(dir, output.Path)); err != nil {
								t.Fatal(err)
							}
							// Remove the reference too: ordinary hash verification
							// alone must not conceal a missing required output.
							m.Parts = append(m.Parts[:i], m.Parts[i+1:]...)
						case "wrong path":
							if err := os.Rename(filepath.Join(dir, output.Path), filepath.Join(dir, output.Path+".other")); err != nil {
								t.Fatal(err)
							}
							m.Parts[i].Path += ".other"
						case "unmanifested":
							m.Parts = append(m.Parts[:i], m.Parts[i+1:]...)
						case "wrong role":
							m.Parts[i].Role = "evidence"
						case "wrong media type":
							m.Parts[i].MediaType = "text/plain"
						case "unhashed":
							m.Parts[i].SHA256 = ""
						case "duplicate":
							m.Parts = append(m.Parts, m.Parts[i])
						case "signature instead of part":
							m.Signatures = append(m.Signatures, m.Parts[i])
							m.Parts = append(m.Parts[:i], m.Parts[i+1:]...)
						}
						return
					}
					t.Fatal("fixture does not declare required output")
				})
				report := verifyCoreDir(t, dir)
				if report.Valid {
					t.Fatal("incomplete output contract passed core verification")
				}
				assertStatus(t, report, vwp.ValidatorRequiredOutputsExist, vwp.StatusFail)
				assertStatus(t, report, vwp.ValidatorPartHashesMatch, vwp.StatusPass)
				if output.Kind == "accountingSchedule" && (mutation == "missing" || mutation == "wrong path") {
					assertStatus(t, report, vwp.ValidatorAccountingScheduleValid, vwp.StatusFail)
				}
			})
		}
	}
}

func TestCoreRejectsInvalidRequiredAccountingSchedule(t *testing.T) {
	for _, mutation := range []string{"unbalanced", "wrong schema version"} {
		t.Run(mutation, func(t *testing.T) {
			dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
			path := filepath.Join(dir, accountingschedule.ArtifactPath)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var artifact accountingschedule.Artifact
			if err := json.Unmarshal(raw, &artifact); err != nil {
				t.Fatal(err)
			}
			if mutation == "unbalanced" {
				artifact.Entries[0].Postings[0].Debit = "1"
			} else {
				artifact.SchemaVersion = 2
			}
			raw, err = json.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			editManifest(t, dir, func(m *vwp.Manifest) {
				for i := range m.Parts {
					if m.Parts[i].Path == accountingschedule.ArtifactPath {
						m.Parts[i].SHA256 = vwp.SHA256Hex(raw)
					}
				}
			})
			report := verifyCoreDir(t, dir)
			if report.Valid {
				t.Fatal("invalid required accounting schedule passed core verification")
			}
			assertStatus(t, report, vwp.ValidatorPartHashesMatch, vwp.StatusPass)
			assertStatus(t, report, vwp.ValidatorAccountingScheduleValid, vwp.StatusFail)
		})
	}
}

func TestOtherExtensionsDoNotRequireAccountingSchedule(t *testing.T) {
	for name, parts := range map[string][]string{
		"minimal-package":                 {"testdata", "valid", "minimal-package"},
		"asc820-fair-value":               {"examples", "asc820-fair-value", "package"},
		"digital-assets-wallet-ownership": {"examples", "digital-assets-wallet-ownership", "package"},
	} {
		t.Run(name, func(t *testing.T) {
			report := vwp.Verify(openFixture(t, parts...))
			if !report.Valid {
				t.Fatalf("other package failed core verification: %+v", report.Results)
			}
			for _, id := range []string{vwp.ValidatorRequiredOutputsExist, vwp.ValidatorAccountingScheduleValid} {
				if _, ok := report.Result(id); ok {
					t.Errorf("unexpected required accounting output check %s", id)
				}
			}
		})
	}
}
