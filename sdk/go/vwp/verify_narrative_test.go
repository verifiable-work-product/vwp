package vwp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

var narrativeValidatorIds = []string{
	vwp.ValidatorNarrativeSchemaValid,
	vwp.ValidatorNarrativeSections,
	vwp.ValidatorNarrativeCitations,
	vwp.ValidatorNarrativeRoles,
	vwp.ValidatorNarrativeCitesFresh,
	vwp.ValidatorNarrativeDrift,
}

func houseFormat(templateSection string) vwp.MemoFormat {
	return vwp.MemoFormat{
		Version: 1,
		Name:    "House style",
		Sections: []vwp.MemoFormatSection{
			{Id: "background", Heading: "Background", Level: 1, Kind: vwp.SectionKindNarrative, Required: true},
			{Id: "analysis", Heading: "Analysis", Level: 1, Kind: vwp.SectionKindNarrative, TemplateSection: templateSection},
			{Id: "claims", Heading: "Claims", Level: 1, Kind: vwp.SectionKindClaims},
			{Id: "conclusion", Heading: "Conclusion", Level: 1, Kind: vwp.SectionKindConclusion, Required: true},
		},
	}
}

func para(role, text string, cites ...string) vwp.NarrativeParagraph {
	return vwp.NarrativeParagraph{Text: text, Role: role, Cites: vwp.CiteIds(cites...)}
}

// narrativePackage copies the ASC 606 fixture, snapshots a format and writes
// a narrative citing every claim, so every narrative validator passes.
func narrativePackage(t *testing.T, format vwp.MemoFormat) (*builder.Builder, string) {
	t.Helper()
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	b, err := builder.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetMemoFormat(format); err != nil {
		t.Fatal(err)
	}
	sections := []vwp.NarrativeSection{
		{Id: "background", Paragraphs: []vwp.NarrativeParagraph{
			para(vwp.ParagraphRoleAnalysis, "DemoCo signed an order form with Acme.", "f_contract_parties_date", "ev_order_form"),
		}},
		{Id: "analysis", Paragraphs: []vwp.NarrativeParagraph{
			para(vwp.ParagraphRoleAnalysis, "A contract exists.", "c_step1_contract_identified"),
			para(vwp.ParagraphRoleAnalysis, "Three promises were identified.", "c_step2_performance_obligations", "j_impl_distinct"),
			para(vwp.ParagraphRoleAnalysis, "The price is fixed.", "c_step3_transaction_price"),
			para(vwp.ParagraphRoleAnalysis, "Allocation on relative SSP.", "c_step4_allocation", "calc_ssp_allocation"),
		}},
		{Id: "conclusion", Paragraphs: []vwp.NarrativeParagraph{
			para(vwp.ParagraphRoleConclusion, "Revenue is recognized ratably, implementation at completion.", "c_step5_recognition"),
		}},
	}
	for _, s := range sections {
		if err := b.SetNarrativeSection(s); err != nil {
			t.Fatalf("SetNarrativeSection %s: %v", s.Id, err)
		}
	}
	return b, dir
}

func verifyNarrativeDir(t *testing.T, dir string) *vwp.Report {
	t.Helper()
	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return validators.Verify(pkg)
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
}

// replaceNarrative writes an edited narrative back without restamping.
func replaceNarrative(t *testing.T, b *builder.Builder, n *vwp.Narrative) {
	t.Helper()
	raw, err := vwp.CanonicalJSON(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.ReplaceFilePart(vwp.NarrativePath, "application/json", builder.RoleNarrative, raw); err != nil {
		t.Fatal(err)
	}
}

func TestNarrativeValidatorsAbsentWithoutNarrative(t *testing.T) {
	fixtures := [][]string{
		{"examples", "asc606-revenue-contract", "package"},
		{"examples", "asc820-fair-value", "package"},
		{"examples", "digital-assets-wallet-ownership", "package"},
		{"testdata", "valid", "minimal-package"},
	}
	for _, fixture := range fixtures {
		report := validators.Verify(openFixture(t, fixture...))
		for _, r := range report.Results {
			if strings.HasPrefix(r.ValidatorId, "narrative.") {
				t.Errorf("%s: unexpected %s without a narrative part", strings.Join(fixture, "/"), r.ValidatorId)
			}
		}
	}
}

func TestNarrativeValidatorsPassOnWellFormedNarrative(t *testing.T) {
	_, dir := narrativePackage(t, houseFormat(""))
	report := verifyNarrativeDir(t, dir)
	for _, id := range narrativeValidatorIds {
		assertStatus(t, report, id, vwp.StatusPass)
	}
	assertStatus(t, report, vwp.ValidatorDigestsMatch, vwp.StatusPass)
	if !report.Valid {
		t.Fatalf("package should be valid; statuses: %v", statuses(report))
	}
	// Narrative validators run with the core set, before profile checks.
	var order []string
	for _, r := range report.Results {
		order = append(order, r.ValidatorId)
	}
	idx := func(id string) int {
		for i, v := range order {
			if v == id {
				return i
			}
		}
		return -1
	}
	if idx(vwp.ValidatorNarrativeDrift) > idx(vwp.ValidatorFinanceClaimsHaveAssertions) || idx(vwp.ValidatorNarrativeSchemaValid) < idx(vwp.ValidatorSignaturesPresent) {
		t.Errorf("narrative validators out of order: %v", order)
	}
}

func TestNarrativeValidatorsPassAgainstDefaultFormat(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	b, err := builder.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "conclusion", Paragraphs: []vwp.NarrativeParagraph{
		para(vwp.ParagraphRoleConclusion, "All five steps hold.", "c_step1_contract_identified", "c_step2_performance_obligations", "c_step3_transaction_price", "c_step4_allocation", "c_step5_recognition"),
	}}); err != nil {
		t.Fatal(err)
	}
	report := verifyNarrativeDir(t, dir)
	for _, id := range narrativeValidatorIds {
		assertStatus(t, report, id, vwp.StatusPass)
	}
}

func TestNarrativeSchemaValidFails(t *testing.T) {
	b, dir := narrativePackage(t, houseFormat(""))
	n, _, err := b.Narrative()
	if err != nil {
		t.Fatal(err)
	}
	n.Sections[0].Paragraphs[0].Role = "summary"
	replaceNarrative(t, b, n)
	report := verifyNarrativeDir(t, dir)
	res := mustResult(t, report, vwp.ValidatorNarrativeSchemaValid)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "summary") {
		t.Errorf("schemaValid = %s %q", res.Status, res.Message)
	}

	// The embedded schema is enforced too: a cite hash that is not hex.
	n, _, _ = b.Narrative()
	n.Sections[0].Paragraphs[0].Role = vwp.ParagraphRoleAnalysis
	n.Sections[0].Paragraphs[0].Cites[0].Hash = "not-a-hash"
	replaceNarrative(t, b, n)
	report = verifyNarrativeDir(t, dir)
	res = mustResult(t, report, vwp.ValidatorNarrativeSchemaValid)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "hash") {
		t.Errorf("schemaValid with bad hash = %s %q", res.Status, res.Message)
	}

	if err := b.ReplaceFilePart(vwp.NarrativePath, "application/json", builder.RoleNarrative, []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	report = verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeSchemaValid, vwp.StatusFail)
	if _, ok := report.Result(vwp.ValidatorNarrativeCitations); ok {
		t.Error("citations should not run on an unreadable narrative")
	}
}

func TestNarrativeSectionsFails(t *testing.T) {
	// Format changed after the narrative was written: formatHash mismatch.
	b, dir := narrativePackage(t, houseFormat(""))
	f := houseFormat("")
	f.Sections[0].Heading = "Context"
	if err := b.SetMemoFormat(f); err != nil {
		t.Fatal(err)
	}
	report := verifyNarrativeDir(t, dir)
	res := mustResult(t, report, vwp.ValidatorNarrativeSections)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "formatHash") {
		t.Errorf("sections after format change = %s %q", res.Status, res.Message)
	}

	// A snapshot that violates the memo-format schema is reported here.
	if err := b.ReplaceFilePart(vwp.MemoFormatPath, "application/json", builder.RoleMemoFormat, []byte(`{"version": 1, "sections": [], "extra": true}`)); err != nil {
		t.Fatal(err)
	}
	report = verifyNarrativeDir(t, dir)
	res = mustResult(t, report, vwp.ValidatorNarrativeSections)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, vwp.MemoFormatPath) {
		t.Errorf("sections with schema-invalid format = %s %q", res.Status, res.Message)
	}

	// Unknown top-level section id and a missing required section.
	b, dir = narrativePackage(t, houseFormat(""))
	n, _, _ := b.Narrative()
	n.Sections[0].Id = "preface"
	n.Sections = n.Sections[:2] // drops the required conclusion
	replaceNarrative(t, b, n)
	report = verifyNarrativeDir(t, dir)
	res = mustResult(t, report, vwp.ValidatorNarrativeSections)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "preface") || !strings.Contains(res.Message, "conclusion") || !strings.Contains(res.Message, "background") {
		t.Errorf("sections = %s %q", res.Status, res.Message)
	}
}

func TestNarrativeCitations(t *testing.T) {
	// A bogus cite fails citations and the graph's reference check.
	b, dir := narrativePackage(t, houseFormat(""))
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "background", Paragraphs: []vwp.NarrativeParagraph{
		para(vwp.ParagraphRoleAnalysis, "DemoCo signed an order form with Acme.", "f_contract_parties_date", "f_does_not_exist"),
	}}); err == nil {
		t.Fatal("the builder stamped a cite of an unknown object")
	}
	n, _, _ := b.Narrative()
	n.Sections[0].Paragraphs[0].Cites = append(n.Sections[0].Paragraphs[0].Cites, vwp.Cite{Id: "f_does_not_exist", Hash: strings.Repeat("0", 64)})
	replaceNarrative(t, b, n)
	report := verifyNarrativeDir(t, dir)
	res := mustResult(t, report, vwp.ValidatorNarrativeCitations)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "f_does_not_exist") {
		t.Errorf("citations with bogus cite = %s %q", res.Status, res.Message)
	}
	res = mustResult(t, report, vwp.ValidatorGraphRefsResolve)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "narrative background: unknown cites ref f_does_not_exist") {
		t.Errorf("graph.refsResolve with bogus cite = %s %q", res.Status, res.Message)
	}

	// A paragraph without cites fails.
	n, _, _ = b.Narrative()
	n.Sections[0].Paragraphs[0].Cites = nil
	replaceNarrative(t, b, n)
	report = verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeCitations, vwp.StatusFail)

	// Uncited claims are review-required, with the ids in the message.
	b, dir = narrativePackage(t, houseFormat(""))
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "analysis", Paragraphs: []vwp.NarrativeParagraph{
		para(vwp.ParagraphRoleAnalysis, "A contract exists.", "c_step1_contract_identified"),
	}}); err != nil {
		t.Fatal(err)
	}
	report = verifyNarrativeDir(t, dir)
	res = mustResult(t, report, vwp.ValidatorNarrativeCitations)
	if res.Status != vwp.StatusReviewRequired || !strings.Contains(res.Message, "c_step4_allocation") || !strings.Contains(res.Message, "c_step2_performance_obligations") {
		t.Errorf("citations with uncited claims = %s %q", res.Status, res.Message)
	}
	if len(res.Refs) != 3 {
		t.Errorf("refs = %v, want the three uncited claim ids", res.Refs)
	}
}

func TestNarrativeRoles(t *testing.T) {
	// A conclusion that cites only a fact fails.
	b, dir := narrativePackage(t, houseFormat(""))
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "conclusion", Paragraphs: []vwp.NarrativeParagraph{
		para(vwp.ParagraphRoleConclusion, "Revenue is recognized ratably.", "f_contract_term"),
	}}); err != nil {
		t.Fatal(err)
	}
	report := verifyNarrativeDir(t, dir)
	res := mustResult(t, report, vwp.ValidatorNarrativeRoles)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "conclusion") {
		t.Errorf("roles with fact-only conclusion = %s %q", res.Status, res.Message)
	}

	// A section mapped to template section s2 that cites nothing from s2_
	// is review-required; one that does passes.
	_, dir = narrativePackage(t, houseFormat("s2"))
	report = verifyNarrativeDir(t, dir)
	res = mustResult(t, report, vwp.ValidatorNarrativeRoles)
	if res.Status != vwp.StatusReviewRequired || !strings.Contains(res.Message, "s2") {
		t.Errorf("roles without s2_ cite = %s %q", res.Status, res.Message)
	}

	b, dir = narrativePackage(t, houseFormat("s2"))
	if err := b.AddFact(vwp.Fact{Id: "s2_fact_promises", Statement: "Three promises are listed.", EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_order_form"}}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "analysis", Paragraphs: []vwp.NarrativeParagraph{
		para(vwp.ParagraphRoleAnalysis, "Three promises were identified.", "s2_fact_promises", "c_step1_contract_identified", "c_step2_performance_obligations", "c_step3_transaction_price", "c_step4_allocation"),
	}}); err != nil {
		t.Fatal(err)
	}
	report = verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeRoles, vwp.StatusPass)
}

func TestNarrativeCitesFreshAndDrift(t *testing.T) {
	b, dir := narrativePackage(t, houseFormat(""))
	// Review and derived parts neither stale the cites nor move the
	// analysis root.
	if _, err := b.RunValidators(nil); err != nil {
		t.Fatal(err)
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	if err := b.SetJudgmentStatus("j_impl_distinct", builder.JudgmentStatusReviewed); err != nil {
		t.Fatal(err)
	}
	if err := b.RecordApproval(vwp.Approval{Id: "app_reviewer", Approver: vwp.Approver{Name: "Reviewer"}, Action: vwp.ApprovalActionApproved, Timestamp: "2026-07-08T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	report := verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeCitesFresh, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorNarrativeDrift, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorApprovalsCurrent, vwp.StatusPass)

	// A fact added afterwards is not cited by anything: the cites stay
	// fresh but the analysis drifted, and the approval is superseded.
	if err := b.AddFact(vwp.Fact{Id: "f_late", Statement: "Added after the narrative.", EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_order_form"}}}); err != nil {
		t.Fatal(err)
	}
	report = verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeCitesFresh, vwp.StatusPass)
	res := mustResult(t, report, vwp.ValidatorNarrativeDrift)
	if res.Status != vwp.StatusReviewRequired || !strings.Contains(res.Message, "analysis root moved") {
		t.Errorf("drift = %s %q", res.Status, res.Message)
	}
	res = mustResult(t, report, vwp.ValidatorApprovalsCurrent)
	if res.Status != vwp.StatusReviewRequired || !strings.Contains(res.Message, "app_reviewer") {
		t.Errorf("approvals.current = %s %q", res.Status, res.Message)
	}
	if !report.Valid {
		t.Errorf("drift and a superseded approval are review items, not failures: %v", statuses(report))
	}

	// A new approval over the current content makes the package current
	// again; the earlier one is history, listed for the reader.
	if err := b.RecordApproval(vwp.Approval{Id: "app_reviewer_2", Approver: vwp.Approver{Name: "Reviewer"}, Action: vwp.ApprovalActionApproved, Timestamp: "2026-07-09T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	res = mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorApprovalsCurrent)
	if res.Status != vwp.StatusPass || !strings.Contains(res.Message, "app_reviewer") {
		t.Errorf("approvals.current after re-approval = %s %q", res.Status, res.Message)
	}

	// Rewriting any section restamps the analysis root, which clears the
	// drift (the other sections' cites were fresh all along).
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "background", Paragraphs: []vwp.NarrativeParagraph{
		para(vwp.ParagraphRoleAnalysis, "DemoCo signed an order form with Acme.", "f_contract_parties_date", "f_late"),
	}}); err != nil {
		t.Fatal(err)
	}
	report = verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeCitesFresh, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorNarrativeDrift, vwp.StatusPass)

	// A cited object that changes stales every section citing it, by
	// name: the evidence bytes behind ev_order_form move its closure hash
	// and, through the facts anchored to it, the closure of everything
	// above.
	evPath := filepath.Join(dir, "evidence", "order-form.txt")
	raw, err := os.ReadFile(evPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.ReplaceFilePart("evidence/order-form.txt", "text/plain", builder.RoleEvidence, append(raw, []byte("\nAmended.\n")...)); err != nil {
		t.Fatal(err)
	}
	// The evidence index still declares the old sha256: the entry's
	// closure is unchanged, so only the index itself can stale it. Rewrite
	// the index with the new byte hash.
	var evidenceDoc map[string]any
	readJSONFile(t, filepath.Join(dir, "evidence", "evidence.json"), &evidenceDoc)
	for _, item := range evidenceDoc["evidence"].([]any) {
		ev := item.(map[string]any)
		if ev["id"] == "ev_order_form" {
			ev["sha256"] = vwp.SHA256Hex(append(raw, []byte("\nAmended.\n")...))
		}
	}
	encoded, err := vwp.EncodeJSON(evidenceDoc)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.ReplaceFilePart(vwp.EvidenceIndex, "application/json", builder.RoleEvidenceIndex, encoded); err != nil {
		t.Fatal(err)
	}
	report = verifyNarrativeDir(t, dir)
	res = mustResult(t, report, vwp.ValidatorNarrativeCitesFresh)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "c_step1_contract_identified (cited by analysis)") || !strings.Contains(res.Message, "f_contract_parties_date (cited by background)") {
		t.Errorf("cites.fresh after evidence change = %s %q", res.Status, res.Message)
	}
	if strings.Join(res.Refs, ",") != "analysis,background,conclusion" {
		t.Errorf("every section cites something downstream of the evidence; refs = %v", res.Refs)
	}
	if res, _ := report.Result(vwp.ValidatorNarrativeDrift); res.Status != vwp.StatusNotRun || !strings.Contains(res.Message, "stale") {
		t.Errorf("drift while cites are stale = %s %q, want notRun", res.Status, res.Message)
	}
	if report.Valid {
		t.Error("stale cites must invalidate the package")
	}

	// Rewriting every section against the changed package restores it.
	n, _, err := b.Narrative()
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range n.Sections {
		if err := b.SetNarrativeSection(sec); err != nil {
			t.Fatal(err)
		}
	}
	report = verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeCitesFresh, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorNarrativeDrift, vwp.StatusPass)
}

func TestNarrativeCitesWithoutHashesAreNotChecked(t *testing.T) {
	b, dir := narrativePackage(t, houseFormat(""))
	n, _, _ := b.Narrative()
	for i := range n.Sections {
		for j := range n.Sections[i].Paragraphs {
			for k := range n.Sections[i].Paragraphs[j].Cites {
				n.Sections[i].Paragraphs[j].Cites[k].Hash = ""
			}
		}
	}
	replaceNarrative(t, b, n)
	report := verifyNarrativeDir(t, dir)
	// The schema demands the hash, so schemaValid fails; freshness has
	// nothing to compare and says so rather than guessing.
	assertStatus(t, report, vwp.ValidatorNarrativeSchemaValid, vwp.StatusFail)
	assertStatus(t, report, vwp.ValidatorNarrativeCitesFresh, vwp.StatusNotRun)
}

func TestNarrativeValidatorsRunOnPackedForm(t *testing.T) {
	_, dir := narrativePackage(t, houseFormat(""))
	vwpPath := filepath.Join(t.TempDir(), "narrated.vwp")
	if err := vwp.Pack(dir, vwpPath); err != nil {
		t.Fatal(err)
	}
	pkg, err := vwp.OpenPackage(vwpPath)
	if err != nil {
		t.Fatal(err)
	}
	report := validators.Verify(pkg)
	for _, id := range narrativeValidatorIds {
		assertStatus(t, report, id, vwp.StatusPass)
	}
	if _, err := os.Stat(vwpPath); err != nil {
		t.Fatal(err)
	}
}

// A literature paragraph reproduces the authoritative text the memo rests
// on: it must cite a claim or judgment and name one of its standardRefs.
func TestNarrativeLiteratureRole(t *testing.T) {
	set := func(t *testing.T, b *builder.Builder, text string, cites ...string) {
		t.Helper()
		if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "analysis", Paragraphs: []vwp.NarrativeParagraph{
			para(vwp.ParagraphRoleLiterature, text, cites...),
			para(vwp.ParagraphRoleAnalysis, "Applicability to the entity.", "c_step1_contract_identified", "c_step2_performance_obligations", "c_step3_transaction_price", "c_step4_allocation"),
		}}); err != nil {
			t.Fatal(err)
		}
	}

	// Verbatim reference in the text: pass, and the role is schema-valid.
	b, dir := narrativePackage(t, houseFormat(""))
	set(t, b, "ASC 606-10-25-1: An entity shall account for a contract with a customer only when all of the following criteria are met ...", "c_step1_contract_identified")
	report := verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeSchemaValid, vwp.StatusPass)
	assertStatus(t, report, vwp.ValidatorNarrativeRoles, vwp.StatusPass)

	// The reference without its "ASC " prefix also names it.
	set(t, b, "Per 606-10-25-1, an entity shall account for a contract ...", "c_step1_contract_identified")
	assertStatus(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeRoles, vwp.StatusPass)

	// A judgment cite carries the standardRefs of the claims listing it.
	set(t, b, "ASC 606-10-25-14 lists the promises in a contract ...", "j_impl_distinct")
	assertStatus(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeRoles, vwp.StatusPass)

	// Text that names none of the cited references fails. (Matching is
	// containment, so a text naming ASC 606-10-25-14 would name the cited
	// ASC 606-10-25-1 as well; pick a reference that does not.)
	set(t, b, "ASC 606-10-32-31 allocates the transaction price ...", "c_step1_contract_identified")
	res := mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeRoles)
	if res.Status != vwp.StatusFail || res.Message != `section "analysis": a literature paragraph names none of its cited references` {
		t.Errorf("roles with unnamed reference = %s %q", res.Status, res.Message)
	}
	if strings.Join(res.Refs, ",") != "analysis" {
		t.Errorf("refs = %v", res.Refs)
	}

	// A cited claim without standardRefs contributes nothing: the
	// reference set is empty and the paragraph cannot name one.
	if err := b.AddClaim(vwp.Claim{Id: "c_unreferenced", Statement: "A claim with no literature."}); err != nil {
		t.Fatal(err)
	}
	set(t, b, "ASC 606-10-25-1: An entity shall ...", "c_unreferenced")
	res = mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeRoles)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `section "analysis": a literature paragraph names none of its cited references`) {
		t.Errorf("roles with reference-less claim = %s %q", res.Status, res.Message)
	}

	// A literature paragraph that cites only a fact fails, whatever it says.
	set(t, b, "ASC 606-10-25-1: An entity shall ...", "f_contract_term")
	res = mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeRoles)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `section "analysis": a literature paragraph cites no claim or judgment`) {
		t.Errorf("roles with fact-only literature = %s %q", res.Status, res.Message)
	}
}

// A prose section that does not apply carries a reason instead of
// paragraphs: it satisfies required, cites nothing, and is shape-checked.
func TestNarrativeNotApplicableSection(t *testing.T) {
	b, dir := narrativePackage(t, houseFormat(""))
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "background", NotApplicable: "No background beyond the order form is needed."}); err != nil {
		t.Fatalf("SetNarrativeSection notApplicable: %v", err)
	}
	n, _, err := b.Narrative()
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := n.Section("background"); !ok || s.NotApplicable == "" || len(s.Paragraphs) != 0 {
		t.Fatalf("section = %+v", s)
	}
	report := verifyNarrativeDir(t, dir)
	for _, id := range narrativeValidatorIds {
		assertStatus(t, report, id, vwp.StatusPass)
	}
	if !report.Valid {
		t.Fatalf("package should be valid; statuses: %v", statuses(report))
	}

	// The builder stamps nothing for it: no cites, and the part round-trips
	// with notApplicable and an empty paragraphs array.
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(vwp.NarrativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"notApplicable": "No background beyond the order form is needed."`) {
		t.Fatalf("notApplicable not written:\n%s", raw)
	}

	// A not-applicable section bound to a template section is exempt from
	// the prefix check: it cannot cite anything by design.
	b2, dir2 := narrativePackage(t, houseFormat("s2"))
	if err := b2.SetNarrativeSection(vwp.NarrativeSection{Id: "analysis", NotApplicable: "No analysis beyond the conclusion is required."}); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, verifyNarrativeDir(t, dir2), vwp.ValidatorNarrativeRoles, vwp.StatusPass)

	// Shape: a reason alongside paragraphs or subsections, a multi-line
	// reason, or one over 512 bytes fails schemaValid.
	filler := vwp.NarrativeParagraph{Text: "Filler.", Role: vwp.ParagraphRoleAnalysis, Cites: []vwp.Cite{{Id: "f_contract_term", Hash: strings.Repeat("0", 64)}}}
	n.Sections[0].Paragraphs = []vwp.NarrativeParagraph{filler}
	replaceNarrative(t, b, n)
	res := mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeSchemaValid)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `section "background": notApplicable sections carry no paragraphs`) {
		t.Errorf("schemaValid with notApplicable and paragraphs = %s %q", res.Status, res.Message)
	}
	n, _, _ = b.Narrative()
	n.Sections[0].Paragraphs = []vwp.NarrativeParagraph{}
	n.Sections[0].Subsections = []vwp.NarrativeSection{{Id: "sub", Heading: "Sub", Paragraphs: []vwp.NarrativeParagraph{filler}}}
	replaceNarrative(t, b, n)
	res = mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeSchemaValid)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `section "background": notApplicable sections carry no paragraphs`) {
		t.Errorf("schemaValid with notApplicable and subsections = %s %q", res.Status, res.Message)
	}
	n, _, _ = b.Narrative()
	n.Sections[0].NotApplicable = "No background\nneeded."
	replaceNarrative(t, b, n)
	res = mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeSchemaValid)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `section "background" notApplicable must be a single line`) {
		t.Errorf("schemaValid with multi-line notApplicable = %s %q", res.Status, res.Message)
	}
	n, _, _ = b.Narrative()
	n.Sections[0].NotApplicable = strings.Repeat("é", 300) // 600 bytes, 300 characters
	replaceNarrative(t, b, n)
	res = mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeSchemaValid)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `section "background" notApplicable exceeds 512 bytes`) {
		t.Errorf("schemaValid with long notApplicable = %s %q", res.Status, res.Message)
	}

	// A not-applicable subsection is accepted, but only a top-level
	// section satisfies required.
	n, _, _ = b.Narrative()
	n.Sections[0].NotApplicable = ""
	n.Sections[0].Paragraphs = []vwp.NarrativeParagraph{}
	n.Sections[0].Subsections = []vwp.NarrativeSection{{Id: "sub", Heading: "Sub", Paragraphs: []vwp.NarrativeParagraph{}, NotApplicable: "Nope."}}
	replaceNarrative(t, b, n)
	report = verifyNarrativeDir(t, dir)
	assertStatus(t, report, vwp.ValidatorNarrativeSchemaValid, vwp.StatusPass)
	res = mustResult(t, report, vwp.ValidatorNarrativeSections)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `required section "background" has no paragraphs`) {
		t.Errorf("sections with an empty required section = %s %q", res.Status, res.Message)
	}
}

// Roles on a non-signoff section and malformed roles are reported through
// narrative.sections with the exact wording the TypeScript SDK emits.
func TestNarrativeSectionsReportsRoleProblems(t *testing.T) {
	b, dir := narrativePackage(t, houseFormat(""))
	f := houseFormat("")
	f.Sections[0].Roles = []string{"Global Controller"}
	raw, err := vwp.CanonicalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.ReplaceFilePart(vwp.MemoFormatPath, "application/json", builder.RoleMemoFormat, raw); err != nil {
		t.Fatal(err)
	}
	res := mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeSections)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `narrative/format.json: section "background" roles are allowed only on signoff sections`) {
		t.Errorf("sections with roles on prose = %s %q", res.Status, res.Message)
	}
	f = houseFormat("")
	f.Sections = append(f.Sections, vwp.MemoFormatSection{Id: "signoff", Heading: "Sign-off", Level: 1, Kind: vwp.SectionKindSignoff, Roles: []string{"Global Controller", "Regional\nController"}})
	if raw, err = vwp.CanonicalJSON(f); err != nil {
		t.Fatal(err)
	}
	if err := b.ReplaceFilePart(vwp.MemoFormatPath, "application/json", builder.RoleMemoFormat, raw); err != nil {
		t.Fatal(err)
	}
	res = mustResult(t, verifyNarrativeDir(t, dir), vwp.ValidatorNarrativeSections)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, `narrative/format.json: section "signoff" role 2 must be a non-empty single line`) {
		t.Errorf("sections with multi-line role = %s %q", res.Status, res.Message)
	}
}
