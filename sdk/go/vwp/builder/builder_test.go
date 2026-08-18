package builder_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

const asc606Fixture = "../../../../examples/asc606-revenue-contract/package"

func readJSONInto(t *testing.T, path string, out any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func verifyDir(t *testing.T, dir string) *vwp.Report {
	t.Helper()
	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return validators.Verify(pkg)
}

func statuses(r *vwp.Report) map[string]vwp.Status {
	out := map[string]vwp.Status{}
	for _, res := range r.Results {
		out[res.ValidatorId] = res.Status
	}
	return out
}

func minimalScaffold(t *testing.T, dir string) *builder.Builder {
	t.Helper()
	b, err := builder.Scaffold(dir, builder.ScaffoldConfig{
		PackageId: "pkg_builder_test",
		CreatedAt: "2026-07-19T12:00:00Z",
		Profile:   "vwp-core",
		Workproduct: vwp.Workproduct{
			Id: "wp_builder_test", Title: "Builder test package",
			Domain: "test", Status: "draft",
		},
		Provenance: vwp.Provenance{
			CreatedAt: "2026-07-19T12:00:00Z",
			CreatedBy: vwp.ProvenanceAgent{Type: "system", Name: "builder-test"},
		},
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	return b
}

// rebuildASC606 reconstructs the ASC 606 example package from builder
// operations, using the fixture's parts as op inputs.
func rebuildASC606(t *testing.T, dir string) *builder.Builder {
	t.Helper()

	var manifest vwp.Manifest
	readJSONInto(t, filepath.Join(asc606Fixture, "manifest.json"), &manifest)
	var workproduct vwp.Workproduct
	readJSONInto(t, filepath.Join(asc606Fixture, "workproduct.json"), &workproduct)
	var provenance vwp.Provenance
	readJSONInto(t, filepath.Join(asc606Fixture, "provenance/provenance.json"), &provenance)

	b, err := builder.Scaffold(dir, builder.ScaffoldConfig{
		PackageId:   manifest.PackageId,
		CreatedAt:   manifest.CreatedAt,
		Profile:     manifest.Profile,
		Extension:   manifest.Extension,
		Title:       manifest.Title,
		Subject:     manifest.Subject,
		Workproduct: workproduct,
		Provenance:  provenance,
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	var evidenceDoc struct {
		Evidence []vwp.Evidence `json:"evidence"`
	}
	readJSONInto(t, filepath.Join(asc606Fixture, "evidence/evidence.json"), &evidenceDoc)
	for _, ev := range evidenceDoc.Evidence {
		if err := b.AddEvidence(ev, mustRead(t, filepath.Join(asc606Fixture, ev.Path))); err != nil {
			t.Fatalf("AddEvidence %s: %v", ev.Id, err)
		}
	}

	var factsDoc struct {
		Facts []vwp.Fact `json:"facts"`
	}
	readJSONInto(t, filepath.Join(asc606Fixture, "facts/facts.json"), &factsDoc)
	for _, f := range factsDoc.Facts {
		if err := b.AddFact(f); err != nil {
			t.Fatalf("AddFact %s: %v", f.Id, err)
		}
	}

	var claimsDoc struct {
		Claims []vwp.Claim `json:"claims"`
	}
	readJSONInto(t, filepath.Join(asc606Fixture, "claims/claims.json"), &claimsDoc)
	for _, c := range claimsDoc.Claims {
		if err := b.AddClaim(c); err != nil {
			t.Fatalf("AddClaim %s: %v", c.Id, err)
		}
	}

	var judgmentsDoc struct {
		Judgments []vwp.Judgment `json:"judgments"`
	}
	readJSONInto(t, filepath.Join(asc606Fixture, "judgments/judgments.json"), &judgmentsDoc)
	for _, j := range judgmentsDoc.Judgments {
		if err := b.AddJudgment(j); err != nil {
			t.Fatalf("AddJudgment %s: %v", j.Id, err)
		}
	}

	var calc vwp.Calculation
	readJSONInto(t, filepath.Join(asc606Fixture, "calculations/ssp-allocation.json"), &calc)
	if err := b.AddCalculation(calc, builder.WithCalculationPath("calculations/ssp-allocation.json")); err != nil {
		t.Fatalf("AddCalculation: %v", err)
	}
	if err := b.AddFilePart("calculations/revenue-schedule.csv", "text/csv", builder.RoleCalculation,
		mustRead(t, filepath.Join(asc606Fixture, "calculations/revenue-schedule.csv"))); err != nil {
		t.Fatalf("AddFilePart schedule: %v", err)
	}

	var validationsDoc struct {
		Validations []vwp.Validation `json:"validations"`
	}
	readJSONInto(t, filepath.Join(asc606Fixture, "validations/validation-report.json"), &validationsDoc)
	for _, v := range validationsDoc.Validations {
		if err := b.AddValidation(v); err != nil {
			t.Fatalf("AddValidation %s: %v", v.Id, err)
		}
	}

	var approvalsDoc struct {
		Approvals []vwp.Approval `json:"approvals"`
	}
	readJSONInto(t, filepath.Join(asc606Fixture, "approvals/approvals.json"), &approvalsDoc)
	for _, a := range approvalsDoc.Approvals {
		if err := b.RecordApproval(a); err != nil {
			t.Fatalf("RecordApproval %s: %v", a.Id, err)
		}
	}

	var attestation map[string]any
	readJSONInto(t, filepath.Join(asc606Fixture, "attestations/att-001.json"), &attestation)
	if err := b.AddAttestation(attestation); err != nil {
		t.Fatalf("AddAttestation: %v", err)
	}

	if err := b.AddFilePart("renderings/memo.md", "text/markdown", builder.RoleRendering,
		mustRead(t, filepath.Join(asc606Fixture, "renderings/memo.md"))); err != nil {
		t.Fatalf("AddFilePart memo: %v", err)
	}
	return b
}

func TestBuilderRebuildsASC606Example(t *testing.T) {
	dir := t.TempDir()
	rebuildASC606(t, dir)

	// The rebuilt package verifies clean with the same validator outcomes.
	rebuilt := verifyDir(t, dir)
	if !rebuilt.Valid {
		t.Fatalf("rebuilt package invalid; statuses: %v", statuses(rebuilt))
	}
	original := verifyDir(t, asc606Fixture)
	if !reflect.DeepEqual(statuses(original), statuses(rebuilt)) {
		t.Errorf("validator outcomes differ:\noriginal: %v\nrebuilt:  %v", statuses(original), statuses(rebuilt))
	}

	// Every fixture part exists in the rebuild and is semantically equal:
	// JSON parts JSON-equal, opaque parts byte-equal.
	err := filepath.WalkDir(asc606Fixture, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(asc606Fixture, path)
		if err != nil {
			return err
		}
		if rel == "manifest.json" {
			return nil // compared field-wise below
		}
		want := mustRead(t, path)
		got := mustRead(t, filepath.Join(dir, rel))
		if filepath.Ext(rel) == ".json" {
			var wantV, gotV any
			if err := json.Unmarshal(want, &wantV); err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			if err := json.Unmarshal(got, &gotV); err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			if !reflect.DeepEqual(wantV, gotV) {
				t.Errorf("part %s not JSON-equal to fixture", rel)
			}
		} else if !bytes.Equal(want, got) {
			t.Errorf("part %s not byte-equal to fixture", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Manifest: identity fields equal; part index equal as a set of
	// path -> (mediaType, role); hashes proven consistent by the clean
	// core.partHashes.match above.
	var want, got vwp.Manifest
	readJSONInto(t, filepath.Join(asc606Fixture, "manifest.json"), &want)
	readJSONInto(t, filepath.Join(dir, "manifest.json"), &got)
	if got.VwpVersion != want.VwpVersion || got.PackageId != want.PackageId ||
		got.CreatedAt != want.CreatedAt || got.Profile != want.Profile ||
		got.Extension != want.Extension || got.Title != want.Title ||
		!reflect.DeepEqual(got.Subject, want.Subject) {
		t.Errorf("manifest identity fields differ: got %+v", got)
	}
	type partMeta struct{ MediaType, Role string }
	partSet := func(m vwp.Manifest) map[string]partMeta {
		out := map[string]partMeta{}
		for _, p := range m.AllPartRefs() {
			out[p.Path] = partMeta{p.MediaType, p.Role}
		}
		return out
	}
	if !reflect.DeepEqual(partSet(want), partSet(got)) {
		t.Errorf("manifest part sets differ:\nwant: %v\ngot:  %v", partSet(want), partSet(got))
	}
}

func TestBuilderOpsAreIdempotent(t *testing.T) {
	dir := t.TempDir()
	rebuildASC606(t, dir)
	manifestBefore := mustRead(t, filepath.Join(dir, "manifest.json"))

	// Replaying the entire op sequence must be a pure no-op.
	rebuildASC606(t, dir)
	manifestAfter := mustRead(t, filepath.Join(dir, "manifest.json"))
	if !bytes.Equal(manifestBefore, manifestAfter) {
		t.Error("replaying identical ops changed the manifest")
	}
	if report := verifyDir(t, dir); !report.Valid {
		t.Fatalf("package invalid after replay; statuses: %v", statuses(report))
	}
}

func TestBuilderConflictsOnDifferentContent(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)

	fact := vwp.Fact{
		Id: "f_1", Statement: "The fee is 100 USD.",
		EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_1"}},
	}
	if err := b.AddFact(fact); err != nil {
		t.Fatalf("AddFact: %v", err)
	}
	if err := b.AddFact(fact); err != nil {
		t.Fatalf("idempotent re-add should succeed: %v", err)
	}
	changed := fact
	changed.Statement = "The fee is 200 USD."
	if err := b.AddFact(changed); !errors.Is(err, builder.ErrConflict) {
		t.Errorf("AddFact with changed content = %v, want ErrConflict", err)
	}

	ev := vwp.Evidence{Id: "ev_1", Type: "doc", Title: "Contract", Path: "evidence/contract.txt"}
	if err := b.AddEvidence(ev, []byte("total fee: 100 USD")); err != nil {
		t.Fatalf("AddEvidence: %v", err)
	}
	if err := b.AddEvidence(ev, []byte("total fee: 100 USD")); err != nil {
		t.Fatalf("idempotent evidence re-add should succeed: %v", err)
	}
	if err := b.AddEvidence(ev, []byte("DIFFERENT BYTES")); !errors.Is(err, builder.ErrConflict) {
		t.Errorf("AddEvidence with changed bytes = %v, want ErrConflict", err)
	}

	if err := b.AddFilePart("renderings/memo.md", "text/markdown", builder.RoleRendering, []byte("# a\n")); err != nil {
		t.Fatal(err)
	}
	if err := b.AddFilePart("renderings/memo.md", "text/markdown", builder.RoleRendering, []byte("# b\n")); !errors.Is(err, builder.ErrConflict) {
		t.Errorf("AddFilePart with changed bytes = %v, want ErrConflict", err)
	}
}

func TestBuilderRejectsSchemaInvalidObjects(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)

	// A fact without evidenceRefs violates the fact schema and must not be
	// written.
	if err := b.AddFact(vwp.Fact{Id: "f_bad", Statement: "unsupported"}); err == nil {
		t.Error("schema-invalid fact should be rejected")
	}
	if _, err := os.Stat(filepath.Join(dir, "facts", "facts.json")); !os.IsNotExist(err) {
		t.Error("rejected fact must not create facts.json")
	}

	if err := b.AddJudgment(vwp.Judgment{Id: "j_bad", Topic: "t", Question: "q", RiskLevel: "extreme"}); err == nil {
		t.Error("judgment with invalid riskLevel should be rejected")
	}
}

func TestBuilderPrimaryArtifact(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)
	content := []byte("region,revenue\nNYC,125000\n")
	if err := b.AddPrimaryArtifact("artifacts/revenue-allocation.csv", "text/csv", content); err != nil {
		t.Fatalf("AddPrimaryArtifact: %v", err)
	}
	if err := b.AddPrimaryArtifact("artifacts/revenue-allocation.csv", "text/csv", content); err != nil {
		t.Fatalf("idempotent AddPrimaryArtifact: %v", err)
	}
	if err := b.AddPrimaryArtifact("artifacts/other.csv", "text/csv", content); !errors.Is(err, builder.ErrConflict) {
		t.Fatalf("second primary artifact = %v, want ErrConflict", err)
	}
	if err := b.AddPrimaryArtifact("renderings/report.csv", "text/csv", content); err == nil {
		t.Fatal("primary artifact outside artifacts/ should fail")
	}

	var manifest vwp.Manifest
	readJSONInto(t, filepath.Join(dir, vwp.ManifestPath), &manifest)
	found := false
	for _, part := range manifest.Parts {
		if part.Role == builder.RolePrimaryArtifact {
			found = true
			if part.Path != "artifacts/revenue-allocation.csv" || part.SHA256 != vwp.SHA256Hex(content) {
				t.Fatalf("primary artifact part = %#v", part)
			}
		}
	}
	if !found {
		t.Fatal("manifest does not index primary artifact")
	}
}

func TestScaffoldIdempotencyAndConflict(t *testing.T) {
	dir := t.TempDir()
	cfg := builder.ScaffoldConfig{
		PackageId: "pkg_x",
		CreatedAt: "2026-07-19T12:00:00Z",
		Profile:   "vwp-core",
		Workproduct: vwp.Workproduct{
			Id: "wp_x", Title: "X", Domain: "test", Status: "draft",
		},
		Provenance: vwp.Provenance{
			CreatedAt: "2026-07-19T12:00:00Z",
			CreatedBy: vwp.ProvenanceAgent{Type: "system", Name: "t"},
		},
	}
	if _, err := builder.Scaffold(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Scaffold(dir, cfg); err != nil {
		t.Errorf("identical re-scaffold should be a no-op, got %v", err)
	}
	other := cfg
	other.PackageId = "pkg_y"
	if _, err := builder.Scaffold(dir, other); !errors.Is(err, builder.ErrConflict) {
		t.Errorf("re-scaffold with different identity = %v, want ErrConflict", err)
	}
	if report := verifyDir(t, dir); !report.Valid {
		t.Fatalf("scaffolded package invalid; statuses: %v", statuses(report))
	}
}

func TestLinkSupport(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)
	if err := b.AddEvidence(vwp.Evidence{Id: "ev_1", Type: "doc", Title: "Contract", Path: "evidence/contract.txt"},
		[]byte("total fee: 100 USD")); err != nil {
		t.Fatal(err)
	}
	if err := b.AddFact(vwp.Fact{Id: "f_1", Statement: "Fee is 100 USD.",
		EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_1"}}}); err != nil {
		t.Fatal(err)
	}
	if err := b.AddClaim(vwp.Claim{Id: "c_1", Statement: "The fee is complete and accurate."}); err != nil {
		t.Fatal(err)
	}

	if err := b.LinkSupport("c_1", builder.SupportFacts, "f_1"); err != nil {
		t.Fatalf("LinkSupport: %v", err)
	}
	if err := b.LinkSupport("c_1", builder.SupportEvidence, "ev_1"); err != nil {
		t.Fatalf("LinkSupport evidence: %v", err)
	}
	// Idempotent: relinking changes nothing.
	before := mustRead(t, filepath.Join(dir, "claims", "claims.json"))
	if err := b.LinkSupport("c_1", builder.SupportFacts, "f_1"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, mustRead(t, filepath.Join(dir, "claims", "claims.json"))) {
		t.Error("idempotent relink rewrote claims.json")
	}

	// Unknown refs and claims are rejected.
	if err := b.LinkSupport("c_1", builder.SupportFacts, "f_missing"); err == nil {
		t.Error("linking an unknown fact should fail")
	}
	if err := b.LinkSupport("c_missing", builder.SupportFacts, "f_1"); err == nil {
		t.Error("linking on an unknown claim should fail")
	}
	if err := b.LinkSupport("c_1", "nonsense", "f_1"); err == nil {
		t.Error("unknown support kind should fail")
	}

	report := verifyDir(t, dir)
	if !report.Valid {
		t.Fatalf("package invalid after linking; statuses: %v", statuses(report))
	}
	res, _ := report.Result(vwp.ValidatorClaimRefsResolve)
	if res.Status != vwp.StatusPass {
		t.Errorf("claimRefs = %s %q", res.Status, res.Message)
	}
}

func TestSetProvenanceAndAddModel(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)

	model := vwp.ModelRef{Provider: "anthropic", Name: "claude-fable-5", Version: "2026-01", Role: "fact extraction"}
	if err := b.AddProvenanceModel(model); err != nil {
		t.Fatalf("AddProvenanceModel: %v", err)
	}
	if err := b.AddProvenanceModel(model); err != nil {
		t.Fatalf("idempotent model re-add: %v", err)
	}
	var p vwp.Provenance
	readJSONInto(t, filepath.Join(dir, "provenance", "provenance.json"), &p)
	if len(p.Models) != 1 || p.Models[0] != model {
		t.Errorf("models = %+v", p.Models)
	}

	// Invalid provenance is rejected before touching disk.
	bad := p
	bad.CreatedBy = vwp.ProvenanceAgent{Type: "robot", Name: "x"}
	if err := b.SetProvenance(bad); err == nil {
		t.Error("invalid createdBy.type should be rejected")
	}

	if report := verifyDir(t, dir); !report.Valid {
		t.Fatalf("package invalid; statuses: %v", statuses(report))
	}
}

func TestRunValidatorsAndRender(t *testing.T) {
	dir := t.TempDir()
	rebuildASC606(t, dir)
	b, err := builder.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	report, err := b.RunValidators(nil)
	if err != nil {
		t.Fatalf("RunValidators: %v", err)
	}
	if !report.Valid {
		t.Fatalf("report invalid; statuses: %v", statuses(report))
	}

	if err := b.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	memo := mustRead(t, filepath.Join(dir, "renderings", "memo.md"))
	for _, want := range []string{
		"# ASC 606 Revenue Recognition Workpaper - Acme Contract",
		"c_step4_allocation",
		"j_impl_distinct",
		"asc606.allocation.tieOut",
		"structured package is the source of truth",
	} {
		if !bytes.Contains(memo, []byte(want)) {
			t.Errorf("rendering missing %q", want)
		}
	}

	// After shipping the report and re-rendering, the package still
	// verifies clean end to end (hashes rewritten for touched parts).
	after := verifyDir(t, dir)
	if !after.Valid {
		t.Fatalf("package invalid after RunValidators+Render; statuses: %v", statuses(after))
	}
	res, _ := after.Result(vwp.ValidatorValidationsSchemaValid)
	if res.Status != vwp.StatusPass {
		t.Errorf("validations.schemaValid = %s %q", res.Status, res.Message)
	}
	hashRes, _ := after.Result(vwp.ValidatorPartHashesMatch)
	if hashRes.Status != vwp.StatusPass {
		t.Errorf("partHashes = %s %q", hashRes.Status, hashRes.Message)
	}
}

func TestBuilderPackRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rebuildASC606(t, dir)
	vwpPath := filepath.Join(t.TempDir(), "rebuilt.vwp")
	if err := vwp.Pack(dir, vwpPath); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	pkg, err := vwp.OpenPackage(vwpPath)
	if err != nil {
		t.Fatal(err)
	}
	if report := validators.Verify(pkg); !report.Valid {
		t.Fatalf("packed rebuild invalid; statuses: %v", statuses(report))
	}
}

func TestBuilderRejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)
	err := b.AddEvidence(vwp.Evidence{Id: "ev_evil", Type: "doc", Title: "Evil", Path: "../evil.txt"}, []byte("x"))
	if err == nil {
		t.Error("evidence with traversal path should be rejected")
	}
	if err := b.AddFilePart("/abs.txt", "text/plain", builder.RoleEvidence, []byte("x")); err == nil {
		t.Error("absolute part path should be rejected")
	}
}

// Render on a package whose memo was written as a raw part: Render
// overwrites rather than conflicts, because renderings are derived views.
func TestRenderOverwritesExistingRendering(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)
	if err := b.AddFilePart(builder.RenderPath, "text/markdown", builder.RoleRendering, []byte("# stale\n")); err != nil {
		t.Fatal(err)
	}
	if err := b.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	memo := mustRead(t, filepath.Join(dir, "renderings", "memo.md"))
	if bytes.Contains(memo, []byte("stale")) {
		t.Error("Render should replace the previous rendering")
	}
	if report := verifyDir(t, dir); !report.Valid {
		t.Fatalf("package invalid after render; statuses: %v", statuses(report))
	}
}
