package builder_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
)

func manifestDigests(t *testing.T, dir string) *vwp.Digests {
	t.Helper()
	var m vwp.Manifest
	readJSONInto(t, filepath.Join(dir, "manifest.json"), &m)
	if m.Digests == nil {
		t.Fatal("manifest carries no digests block")
	}
	return m.Digests
}

// A human review of a judgment is review state: it moves the review root
// and nothing else.
func TestSetJudgmentStatusMovesOnlyTheReviewRoot(t *testing.T) {
	b, dir := openFixtureCopy(t)
	before := manifestDigests(t, dir)
	if err := b.SetJudgmentStatus("j_impl_distinct", builder.JudgmentStatusApproved); err != nil {
		t.Fatal(err)
	}
	after := manifestDigests(t, dir)
	if after.AnalysisRoot != before.AnalysisRoot || after.ContentRoot != before.ContentRoot {
		t.Fatal("judgment status moved the analysis or content root")
	}
	if after.Collections[vwp.KindJudgment] != before.Collections[vwp.KindJudgment] {
		t.Fatal("judgment status moved the judgment collection root")
	}
	if after.ReviewRoot == before.ReviewRoot {
		t.Fatal("judgment status did not move the review root")
	}
	if report := verifyDir(t, dir); !report.Valid {
		t.Fatalf("package invalid after review: %v", statuses(report))
	}
}

// Recording an approval moves the review root only, and the approval is
// stamped with the roots as they stood when it was recorded.
func TestRecordApprovalStampsRootsAndMovesOnlyTheReviewRoot(t *testing.T) {
	b, dir := openFixtureCopy(t)
	before := manifestDigests(t, dir)
	if err := b.RecordApproval(vwp.Approval{Id: "app_ctrl", Approver: vwp.Approver{Name: "Controller"}, Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-20T10:00:00Z", Scope: "package"}); err != nil {
		t.Fatal(err)
	}
	after := manifestDigests(t, dir)
	if after.AnalysisRoot != before.AnalysisRoot || after.ContentRoot != before.ContentRoot {
		t.Fatal("an approval moved the analysis or content root")
	}
	if after.ReviewRoot == before.ReviewRoot {
		t.Fatal("an approval did not move the review root")
	}
	var doc struct {
		Approvals []vwp.Approval `json:"approvals"`
	}
	readJSONInto(t, filepath.Join(dir, "approvals", "approvals.json"), &doc)
	var stamped vwp.Approval
	for _, a := range doc.Approvals {
		if a.Id == "app_ctrl" {
			stamped = a
		}
	}
	if stamped.ContentRoot != before.ContentRoot {
		t.Errorf("approval contentRoot = %s, want the content root at recording time %s", stamped.ContentRoot, before.ContentRoot)
	}
	if stamped.ReviewRoot != before.ReviewRoot {
		t.Errorf("approval reviewRoot = %s, want the review root before the approval %s", stamped.ReviewRoot, before.ReviewRoot)
	}
	report := verifyDir(t, dir)
	if res, ok := report.Result(vwp.ValidatorApprovalsCurrent); !ok || res.Status != vwp.StatusPass {
		t.Fatalf("approvals.current = %+v", res)
	}

	// Re-recording the same approval without the stamps is the idempotent
	// no-op; with different stamps it is a conflict.
	if err := b.RecordApproval(vwp.Approval{Id: "app_ctrl", Approver: vwp.Approver{Name: "Controller"}, Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-20T10:00:00Z", Scope: "package"}); err != nil {
		t.Fatalf("idempotent re-record: %v", err)
	}
	if err := b.RecordApproval(vwp.Approval{Id: "app_ctrl", Approver: vwp.Approver{Name: "Controller"}, Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-20T10:00:00Z", Scope: "package", ContentRoot: strings.Repeat("1", 64)}); !errors.Is(err, builder.ErrConflict) {
		t.Fatalf("re-record with other stamps: %v", err)
	}

	// Content that changes after the approval supersedes it.
	if err := b.AddFact(vwp.Fact{Id: "f_after_approval", Statement: "Added after approval.", EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_order_form"}}}); err != nil {
		t.Fatal(err)
	}
	report = verifyDir(t, dir)
	res, ok := report.Result(vwp.ValidatorApprovalsCurrent)
	if !ok || res.Status != vwp.StatusReviewRequired || !strings.Contains(res.Message, "app_ctrl") {
		t.Fatalf("approvals.current after content change = %+v", res)
	}
	if !report.Valid {
		t.Fatalf("a superseded approval is a review item, not a failure: %v", statuses(report))
	}
}

// Every builder mutation leaves the manifest digests current, including
// from a fresh scaffold, and a package with unresolved references carries
// none rather than stale ones.
func TestBuilderKeepsManifestDigestsCurrent(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)
	d := manifestDigests(t, dir)
	want, err := b.Digests()
	if err != nil {
		t.Fatal(err)
	}
	if !d.Equal(want) {
		t.Fatalf("scaffold digests %+v, want %+v", d, want)
	}
	if err := b.AddClaim(vwp.Claim{Id: "c_1", Statement: "A claim."}); err != nil {
		t.Fatal(err)
	}
	if again := manifestDigests(t, dir); again.Equal(d) {
		t.Fatal("digests did not move after AddClaim")
	}
	if report := verifyDir(t, dir); report.Valid == false {
		t.Fatalf("package invalid: %v", statuses(report))
	} else if res, _ := report.Result(vwp.ValidatorDigestsMatch); res.Status != vwp.StatusPass {
		t.Fatalf("digests.match = %+v", res)
	}

	// A fact citing evidence that is not in the package: the graph is
	// incomplete, so the manifest drops its digests and the verifier
	// reports the dangling reference instead of a digest mismatch.
	if err := b.AddFact(vwp.Fact{Id: "f_dangling", Statement: "Cites nothing in the package.", EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_missing"}}}); err != nil {
		t.Fatal(err)
	}
	var m vwp.Manifest
	readJSONInto(t, filepath.Join(dir, "manifest.json"), &m)
	if m.Digests != nil {
		t.Fatal("manifest carries digests for an incomplete graph")
	}
	if _, err := b.Digests(); err == nil {
		t.Fatal("Digests should fail on an incomplete graph")
	}
	if err := b.RefreshDigests(); err == nil {
		t.Fatal("RefreshDigests should fail on an incomplete graph")
	}
	report := verifyDir(t, dir)
	res, _ := report.Result(vwp.ValidatorGraphRefsResolve)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "fact f_dangling: unknown evidenceRefs ref ev_missing") {
		t.Fatalf("graph.refsResolve = %+v", res)
	}
	if res, _ := report.Result(vwp.ValidatorDigestsMatch); res.Status != vwp.StatusNotRun {
		t.Fatalf("digests.match without a digests block = %+v, want notRun", res)
	}
}

// An id used by two kinds makes every reference ambiguous: the graph
// cannot be built and the message names the id and both kinds.
func TestDuplicateIdAcrossCollectionsFailsRefsResolve(t *testing.T) {
	b, dir := openFixtureCopy(t)
	if err := b.AddClaim(vwp.Claim{Id: "f_contract_term", Statement: "A claim reusing a fact id."}); err != nil {
		t.Fatal(err)
	}
	report := verifyDir(t, dir)
	res, _ := report.Result(vwp.ValidatorGraphRefsResolve)
	if res.Status != vwp.StatusFail || res.Message != "claim f_contract_term: id already used by a fact" {
		t.Fatalf("graph.refsResolve = %s %q", res.Status, res.Message)
	}
	if res, _ := report.Result(vwp.ValidatorDigestsMatch); res.Status != vwp.StatusNotRun {
		t.Fatalf("digests.match = %+v, want notRun", res)
	}
	if res, _ := report.Result(vwp.ValidatorApprovalsCurrent); res.Status != vwp.StatusNotRun {
		t.Fatalf("approvals.current = %+v, want notRun", res)
	}
}

// approvals.current is not run when no approval carries a contentRoot, and
// absent when there is no approvals part.
func TestApprovalsCurrentNotRunWithoutStamps(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)
	if res, ok := verifyDir(t, dir).Result(vwp.ValidatorApprovalsCurrent); ok {
		t.Fatalf("approvals.current without an approvals part = %+v", res)
	}
	if err := b.RecordApproval(vwp.Approval{Id: "app_1", Approver: vwp.Approver{Name: "R"}, Action: vwp.ApprovalActionAcknowledged, Timestamp: "2026-10-20T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	// Strip the stamps the builder wrote.
	raw := mustRead(t, filepath.Join(dir, "approvals", "approvals.json"))
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, a := range doc["approvals"].([]any) {
		delete(a.(map[string]any), "contentRoot")
		delete(a.(map[string]any), "reviewRoot")
	}
	stripped, _ := vwp.EncodeJSON(doc)
	if err := b.ReplaceFilePart(vwp.ApprovalsPath, "application/json", builder.RoleApprovals, stripped); err != nil {
		t.Fatal(err)
	}
	res, _ := verifyDir(t, dir).Result(vwp.ValidatorApprovalsCurrent)
	if res.Status != vwp.StatusNotRun || !strings.Contains(res.Message, "contentRoot") {
		t.Fatalf("approvals.current without stamps = %+v, want notRun", res)
	}
}

// Declared inputs: a calculation whose inputs name a fact it did not list
// in dependsOn, and a judgment whose basis mentions a fact outside its
// basisFacts, are warned about; declaring them clears the warning and the
// edges enter the closure hashes.
func TestDeclaredInputsWarningAndClosureEdges(t *testing.T) {
	b, dir := openFixtureCopy(t)
	if err := b.AddCalculation(vwp.Calculation{
		Id: "calc_term_months", Type: "custom", Method: "months in term",
		Inputs:  map[string]any{"termFact": "f_contract_term", "nested": []any{map[string]any{"valueFact": "f_contract_value"}}},
		Outputs: map[string]any{"months": 24},
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.AddJudgment(vwp.Judgment{Id: "j_term", Topic: "Term", Question: "Is the term fixed?", ProposedConclusion: "Yes",
		Basis: "f_contract_term states a non-cancellable 24-month term."}); err != nil {
		t.Fatal(err)
	}
	// Mentions are whole tokens: a basis naming f_x10 does not mention
	// f_x1, while punctuation around an id still bounds it.
	for _, id := range []string{"f_x1", "f_x10"} {
		if err := b.AddFact(vwp.Fact{Id: id, Statement: "Token test fact " + id + ".", EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_order_form"}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.AddJudgment(vwp.Judgment{Id: "j_token", Topic: "Tokens", Question: "Which fact?", ProposedConclusion: "f_x10",
		Basis: "Per f_x10 (see also f_x10-note and xf_x1), the longer id is the one weighed.", BasisFacts: []string{"f_x10"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.AddJudgment(vwp.Judgment{Id: "j_token_punct", Topic: "Tokens", Question: "Which fact?", ProposedConclusion: "f_x1",
		Basis: "Weighed (f_x1)."}); err != nil {
		t.Fatal(err)
	}
	report := verifyDir(t, dir)
	res, _ := report.Result(vwp.ValidatorGraphDeclaredInputs)
	if res.Status != vwp.StatusWarning ||
		!strings.Contains(res.Message, "calculation calc_term_months: inputs reference f_contract_term, f_contract_value not declared in dependsOn") ||
		!strings.Contains(res.Message, "judgment j_term: basis mentions f_contract_term not declared in basisFacts") ||
		!strings.Contains(res.Message, "judgment j_token_punct: basis mentions f_x1 not declared in basisFacts") ||
		strings.Contains(res.Message, "j_token:") {
		t.Fatalf("declaredInputs = %s %q", res.Status, res.Message)
	}

	// Declared: no warning, and the closure hashes now depend on the facts.
	b2, dir2 := openFixtureCopy(t)
	if err := b2.AddCalculation(vwp.Calculation{
		Id: "calc_term_months", Type: "custom", Method: "months in term",
		Inputs:    map[string]any{"termFact": "f_contract_term", "nested": []any{map[string]any{"valueFact": "f_contract_value"}}},
		Outputs:   map[string]any{"months": 24},
		DependsOn: []string{"f_contract_term", "f_contract_value"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := b2.AddJudgment(vwp.Judgment{Id: "j_term", Topic: "Term", Question: "Is the term fixed?", ProposedConclusion: "Yes",
		Basis: "f_contract_term states a non-cancellable 24-month term.", BasisFacts: []string{"f_contract_term"}}); err != nil {
		t.Fatal(err)
	}
	report = verifyDir(t, dir2)
	if res, _ := report.Result(vwp.ValidatorGraphDeclaredInputs); res.Status != vwp.StatusPass {
		t.Fatalf("declaredInputs with declarations = %s %q", res.Status, res.Message)
	}
	g1, err := b.Graph()
	if err != nil {
		t.Fatal(err)
	}
	g2, err := b2.Graph()
	if err != nil {
		t.Fatal(err)
	}
	c1, _ := g1.Lookup("calc_term_months")
	c2, _ := g2.Lookup("calc_term_months")
	if c1.ClosureHash == c2.ClosureHash {
		t.Fatal("declaring dependsOn did not change the calculation's closure hash")
	}
	if strings.Join(c2.Deps, ",") != "f_contract_term,f_contract_value" {
		t.Fatalf("deps = %v", c2.Deps)
	}
	j2, _ := g2.Lookup("j_term")
	if strings.Join(j2.Deps, ",") != "f_contract_term" {
		t.Fatalf("judgment deps = %v", j2.Deps)
	}
}

// A dependency cycle is reported, not recursed into.
func TestGraphCycleIsReported(t *testing.T) {
	dir := t.TempDir()
	b := minimalScaffold(t, dir)
	if err := b.AddCalculation(vwp.Calculation{Id: "calc_a", Type: "custom", Inputs: map[string]any{}, Outputs: map[string]any{}, DependsOn: []string{"calc_b"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.AddCalculation(vwp.Calculation{Id: "calc_b", Type: "custom", Inputs: map[string]any{}, Outputs: map[string]any{}, DependsOn: []string{"calc_a"}}); err != nil {
		t.Fatal(err)
	}
	g, err := b.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Cycles) == 0 || g.Complete() {
		t.Fatalf("cycle not detected: %+v", g.Cycles)
	}
	report := verifyDir(t, dir)
	res, _ := report.Result(vwp.ValidatorGraphAcyclic)
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "calc_a -> calc_b -> calc_a") {
		t.Fatalf("graph.acyclic = %s %q", res.Status, res.Message)
	}
	if res, _ := report.Result(vwp.ValidatorGraphRefsResolve); res.Status != vwp.StatusPass {
		t.Fatalf("refsResolve on a cycle = %s %q", res.Status, res.Message)
	}
}
