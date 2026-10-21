package vwp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func statuses(r *vwp.Report) map[string]vwp.Status {
	out := map[string]vwp.Status{}
	for _, res := range r.Results {
		out[res.ValidatorId] = res.Status
	}
	return out
}

// repoRoot is where the fixture trees live (examples/<name>/package,
// testdata/valid, testdata/invalid): the repository root in the canonical VWP
// checkout, the mirrored testdata/ tree in the Bitwave monorepo. The test
// files are identical in both apart from this constant.
const repoRoot = "../../.."

func fixturePath(parts ...string) string {
	return filepath.Join(append([]string{repoRoot}, parts...)...)
}

func openFixture(t *testing.T, parts ...string) vwp.Source {
	t.Helper()
	pkg, err := vwp.OpenPackage(fixturePath(parts...))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	return pkg
}

// copyFixture copies a fixture package into a temp dir for mutation.
func copyFixture(t *testing.T, parts ...string) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(fixturePath(parts...))); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func mustResult(t *testing.T, r *vwp.Report, validatorId string) vwp.ValidationResult {
	t.Helper()
	res, ok := r.Result(validatorId)
	if !ok {
		t.Fatalf("no result for %s; got %v", validatorId, statuses(r))
	}
	return res
}

func assertStatus(t *testing.T, r *vwp.Report, validatorId string, want vwp.Status) {
	t.Helper()
	res := mustResult(t, r, validatorId)
	if res.Status != want {
		t.Errorf("%s = %s (message %q), want %s", validatorId, res.Status, res.Message, want)
	}
}

func TestGoldenASC606Example(t *testing.T) {
	report := validators.Verify(openFixture(t, "examples", "asc606-revenue-contract", "package"))
	if !report.Valid {
		t.Fatalf("asc606 example should be valid; statuses: %v", statuses(report))
	}
	assertStatus(t, report, vwp.ValidatorPackageSafePaths, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorManifestSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorRequiredPartsExist, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorPartHashesMatch, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorWorkproductSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorClaimsSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorProvenanceSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorEvidenceSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorFactsSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorJudgmentsSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorApprovalsSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorValidationsSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorAttestationsSchemaOK, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorEvidenceRefsResolve, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorClaimRefsResolve, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorSignaturesPresent, vwp.StatusWarning)
	assertStatus(t, report, vwp.ValidatorFinanceClaimsHaveAssertions, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorFinanceClaimsHaveSupport, vwp.StatusPass)
	// The implementation-services judgment is intentionally left in review.
	assertStatus(t, report, vwp.ValidatorFinanceRequiredJudgmentsDone, vwp.StatusReviewRequired)

	anchors := mustResult(t, report, vwp.ValidatorFactAnchorsQuotesMatch)
	if anchors.Status != vwp.StatusPass {
		t.Errorf("factAnchors = %s (%s), want pass", anchors.Status, anchors.Message)
	}
	if anchors.Message != "10 quote anchors verified" {
		t.Errorf("factAnchors message = %q", anchors.Message)
	}

	assertStatus(t, report, "asc606.allocation.schemaValid", vwp.StatusPass)
	tieOut := mustResult(t, report, "asc606.allocation.tieOut")
	if tieOut.Status != vwp.StatusPass || tieOut.Message != "allocated 300000 equals transaction price 300000" {
		t.Errorf("tieOut = %s %q", tieOut.Status, tieOut.Message)
	}
	schedule := mustResult(t, report, "asc606.revenueSchedule.total")
	if schedule.Status != vwp.StatusPass || schedule.Message != "schedule total 300000 equals allocated consideration 300000" {
		t.Errorf("revenueSchedule = %s %q", schedule.Status, schedule.Message)
	}
}

func TestGoldenASC606ZipForm(t *testing.T) {
	report := validators.Verify(openFixture(t, "examples", "asc606-revenue-contract", "acme-asc606.vwp"))
	if !report.Valid {
		t.Fatalf("asc606 .vwp should be valid; statuses: %v", statuses(report))
	}
	assertStatus(t, report, vwp.ValidatorPartHashesMatch, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorAttestationsSchemaOK, vwp.StatusPass)
	assertStatus(t, report, "asc606.allocation.tieOut", vwp.StatusPass)
}

func TestGoldenASC820Example(t *testing.T) {
	report := validators.Verify(openFixture(t, "examples", "asc820-fair-value", "package"))
	if !report.Valid {
		t.Fatalf("asc820 example should be valid; statuses: %v", statuses(report))
	}
	assertStatus(t, report, vwp.ValidatorPartHashesMatch, vwp.StatusPass)
	assertStatus(t, report, "asc820.measurements.schemaValid", vwp.StatusPass)
	pricing := mustResult(t, report, "asc820.pricingSources.resolve")
	if pricing.Status != vwp.StatusPass || pricing.Message != "2 measurements have resolvable pricing source evidence" {
		t.Errorf("pricingSources = %s %q", pricing.Status, pricing.Message)
	}
	level3 := mustResult(t, report, "asc820.level3.judgmentLinked")
	if level3.Status != vwp.StatusPass || level3.Message != "1 level3 measurement links a hierarchy/unobservable-inputs judgment" {
		t.Errorf("level3 = %s %q", level3.Status, level3.Message)
	}
	// Unlike the reference CLI, presence is reported on success as well so
	// a completion contract can require it.
	if res, ok := report.Result("asc820.measurements.present"); !ok || res.Status != vwp.StatusPass {
		t.Errorf("measurements.present = %+v, want pass", res)
	}
}

func TestGoldenWalletOwnershipExample(t *testing.T) {
	report := validators.Verify(openFixture(t, "examples", "digital-assets-wallet-ownership", "package"))
	if !report.Valid {
		t.Fatalf("wallet example should be valid; statuses: %v", statuses(report))
	}
	for _, id := range []string{
		"digitalAssets.wallets.schemaValid",
		"digitalAssets.wallets.entityMapping",
		"digitalAssets.wallets.proofOfControl",
		"digitalAssets.wallets.exclusionRationale",
		"digitalAssets.wallets.balanceEvidence",
	} {
		assertStatus(t, report, id, vwp.StatusPass)
	}
}

func TestMinimalPackage(t *testing.T) {
	report := validators.Verify(openFixture(t, "testdata", "valid", "minimal-package"))
	if !report.Valid {
		t.Fatalf("minimal package should be valid; statuses: %v", statuses(report))
	}
	assertStatus(t, report, vwp.ValidatorPartHashesMatch, vwp.StatusWarning)
	assertStatus(t, report, vwp.ValidatorSignaturesPresent, vwp.StatusWarning)
	assertStatus(t, report, vwp.ValidatorClaimRefsResolve, vwp.StatusPass)
	// No optional collections, no attestations: those validators emit nothing.
	for _, id := range []string{
		vwp.ValidatorEvidenceSchemaValid,
		vwp.ValidatorFactsSchemaValid,
		vwp.ValidatorAttestationsSchemaOK,
		vwp.ValidatorEvidenceRefsResolve,
		vwp.ValidatorFactAnchorsQuotesMatch,
	} {
		if _, ok := report.Result(id); ok {
			t.Errorf("%s should not be reported for the minimal package", id)
		}
	}
}

func TestInvalidFixtures(t *testing.T) {
	t.Run("bad-hash", func(t *testing.T) {
		report := validators.Verify(openFixture(t, "testdata", "invalid", "bad-hash"))
		if report.Valid {
			t.Fatal("bad-hash should be invalid")
		}
		assertStatus(t, report, vwp.ValidatorPartHashesMatch, vwp.StatusFail)
	})
	t.Run("missing-provenance", func(t *testing.T) {
		report := validators.Verify(openFixture(t, "testdata", "invalid", "missing-provenance"))
		if report.Valid {
			t.Fatal("missing-provenance should be invalid")
		}
		assertStatus(t, report, vwp.ValidatorRequiredPartsExist, vwp.StatusFail)
	})
	t.Run("bad-anchor-quote", func(t *testing.T) {
		report := validators.Verify(openFixture(t, "testdata", "invalid", "bad-anchor-quote"))
		if report.Valid {
			t.Fatal("bad-anchor-quote should be invalid")
		}
		assertStatus(t, report, vwp.ValidatorFactAnchorsQuotesMatch, vwp.StatusFail)
		// The refs themselves resolve; only the quoted text is absent.
		assertStatus(t, report, vwp.ValidatorEvidenceRefsResolve, vwp.StatusPass)
	})
	t.Run("bad-workproduct-schema", func(t *testing.T) {
		report := validators.Verify(openFixture(t, "testdata", "invalid", "bad-workproduct-schema"))
		if report.Valid {
			t.Fatal("bad-workproduct-schema should be invalid")
		}
		assertStatus(t, report, vwp.ValidatorWorkproductSchemaValid, vwp.StatusFail)
	})
}

func TestTamperedPartFailsHashCheck(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	memoPath := filepath.Join(dir, "renderings", "memo.md")
	memo, err := os.ReadFile(memoPath)
	if err != nil {
		t.Fatal(err)
	}
	memo[0] ^= 0x01 // flip one bit
	if err := os.WriteFile(memoPath, memo, 0o644); err != nil {
		t.Fatal(err)
	}

	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	report := validators.Verify(pkg)
	if report.Valid {
		t.Fatal("tampered package should be invalid")
	}
	res := mustResult(t, report, vwp.ValidatorPartHashesMatch)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "hash mismatch: renderings/memo.md") {
		t.Errorf("partHashes = %s %q", res.Status, res.Message)
	}
}

func TestProvenanceModelMissingVersionFailsSchema(t *testing.T) {
	dir := copyFixture(t, "testdata", "valid", "minimal-package")
	provPath := filepath.Join(dir, "provenance", "provenance.json")
	var prov map[string]any
	raw, err := os.ReadFile(provPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &prov); err != nil {
		t.Fatal(err)
	}
	prov["models"] = []any{map[string]any{"provider": "demo", "name": "some-model"}} // no version
	updated, err := json.Marshal(prov)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provPath, updated, 0o644); err != nil {
		t.Fatal(err)
	}

	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	report := validators.Verify(pkg)
	if report.Valid {
		t.Fatal("package with version-less provenance model should be invalid")
	}
	assertStatus(t, report, vwp.ValidatorProvenanceSchemaValid, vwp.StatusFail)
}

func TestSchemaInvalidFactFails(t *testing.T) {
	dir := copyFixture(t, "testdata", "valid", "minimal-package")
	if err := os.MkdirAll(filepath.Join(dir, "facts"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A fact with no evidenceRefs violates the fact schema.
	bad := `{"facts": [{"id": "f_bad", "statement": "unsupported"}]}`
	if err := os.WriteFile(filepath.Join(dir, "facts", "facts.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	report := validators.Verify(pkg)
	if report.Valid {
		t.Fatal("package with schema-invalid fact should be invalid")
	}
	res := mustResult(t, report, vwp.ValidatorFactsSchemaValid)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "fact[f_bad]") {
		t.Errorf("facts.schemaValid = %s %q", res.Status, res.Message)
	}
}

// TestMultiAnchorQuoteVerification covers the unified anchor vocabulary:
// every anchor in a ref's anchors[] array is checked independently, so one
// bad quote fails the ref even when a sibling anchor's quote matches.
func TestMultiAnchorQuoteVerification(t *testing.T) {
	writeFacts := func(t *testing.T, dir string, anchors []map[string]any) {
		t.Helper()
		doc := map[string]any{"facts": []any{map[string]any{
			"id":        "f_multi",
			"statement": "The total fee is USD 250,000.",
			"evidenceRefs": []any{map[string]any{
				"evidenceId": "ev_contract",
				"anchors":    anchors,
			}},
		}}}
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "facts", "facts.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The fixture's evidence/contract.txt reads "The total fee is USD
	// 100,000, payable net thirty (30) days from invoice."
	goodAnchor := map[string]any{"quote": "The total fee is USD 100,000", "section": "2(a)"}
	badAnchor := map[string]any{"quote": "text that is not in the evidence"}
	t.Run("good quote alone passes", func(t *testing.T) {
		dir := copyFixture(t, "testdata", "invalid", "bad-anchor-quote")
		writeFacts(t, dir, []map[string]any{goodAnchor})
		pkg, err := vwp.OpenPackage(dir)
		if err != nil {
			t.Fatal(err)
		}
		report := validators.Verify(pkg)
		res := mustResult(t, report, vwp.ValidatorFactAnchorsQuotesMatch)
		if res.Status != vwp.StatusPass || res.Message != "1 quote anchors verified" {
			t.Errorf("factAnchors = %s %q", res.Status, res.Message)
		}
	})
	t.Run("one bad quote among good fails", func(t *testing.T) {
		dir := copyFixture(t, "testdata", "invalid", "bad-anchor-quote")
		writeFacts(t, dir, []map[string]any{goodAnchor, badAnchor})
		pkg, err := vwp.OpenPackage(dir)
		if err != nil {
			t.Fatal(err)
		}
		report := validators.Verify(pkg)
		if report.Valid {
			t.Fatal("one bad quote must fail the package")
		}
		res := mustResult(t, report, vwp.ValidatorFactAnchorsQuotesMatch)
		if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "fact f_multi: quote not found in evidence/contract.txt") {
			t.Errorf("factAnchors = %s %q", res.Status, res.Message)
		}
	})
}

func TestNormalizeForQuoteMatch(t *testing.T) {
	in := "  “Total  fee” — USD\t250,000 ‘net’ "
	want := `"Total fee" - USD 250,000 'net'`
	if got := vwp.NormalizeForQuoteMatch(in); got != want {
		t.Errorf("NormalizeForQuoteMatch = %q, want %q", got, want)
	}
}

func TestIsUnsafePath(t *testing.T) {
	unsafe := []string{"../evil.txt", "a/../../evil.txt", "/etc/passwd", "C:evil.txt", `a\b.txt`, "", "a\x00b"}
	for _, p := range unsafe {
		if !vwp.IsUnsafePath(p) {
			t.Errorf("IsUnsafePath(%q) = false, want true", p)
		}
	}
	safe := []string{"manifest.json", "claims/claims.json", "evidence/a.b.txt", "a/b/c"}
	for _, p := range safe {
		if vwp.IsUnsafePath(p) {
			t.Errorf("IsUnsafePath(%q) = true, want false", p)
		}
	}
}
