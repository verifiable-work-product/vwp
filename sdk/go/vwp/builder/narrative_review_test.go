package builder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
)

func readMemo(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(builder.RenderPath)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func narrativeStatus(t *testing.T, report *vwp.Report, id string) vwp.ValidationResult {
	t.Helper()
	for _, r := range report.Results {
		if r.ValidatorId == id {
			return r
		}
	}
	t.Fatalf("%s did not run", id)
	return vwp.ValidationResult{}
}

// A human review after the narrative is written is not an analysis change:
// marking a judgment reviewed and recording the approval must leave the
// narrative fresh and the package valid, or sign-off would always fail.
func TestReviewAfterNarrativeKeepsItFresh(t *testing.T) {
	b, dir := openFixtureCopy(t)
	if err := b.SetMemoFormat(customFormat()); err != nil {
		t.Fatal(err)
	}
	for _, sec := range []vwp.NarrativeSection{purposeSection(), conclusionSection()} {
		if err := b.SetNarrativeSection(sec); err != nil {
			t.Fatal(err)
		}
	}
	if res := narrativeStatus(t, verifyDir(t, dir), vwp.ValidatorNarrativeCitesFresh); res.Status != vwp.StatusPass {
		t.Fatalf("stale before review = %s %q", res.Status, res.Message)
	}
	if err := b.SetJudgmentStatus("j_impl_distinct", "reviewed"); err != nil {
		t.Fatal(err)
	}
	if err := b.RecordApproval(vwp.Approval{Id: "app_1", Approver: vwp.Approver{Name: "Controller"}, Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-20T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RunValidators(nil); err != nil {
		t.Fatal(err)
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	report := verifyDir(t, dir)
	if res := narrativeStatus(t, report, vwp.ValidatorNarrativeCitesFresh); res.Status != vwp.StatusPass {
		t.Fatalf("stale after review = %s %q", res.Status, res.Message)
	}
	if res := narrativeStatus(t, report, vwp.ValidatorNarrativeDrift); res.Status != vwp.StatusPass {
		t.Fatalf("drift after review = %s %q", res.Status, res.Message)
	}
	if res := narrativeStatus(t, report, vwp.ValidatorApprovalsCurrent); res.Status != vwp.StatusPass {
		t.Fatalf("approvals.current after review = %s %q", res.Status, res.Message)
	}
	if !report.Valid {
		t.Fatalf("package invalid after review: %v", statuses(report))
	}
	memo := readMemo(t, dir)
	if !strings.Contains(memo, "Controller") || !strings.Contains(memo, "Review actions") {
		t.Fatal("memo lacks the review action")
	}
}

// Customer and model text can neither forge a memo section nor hide the
// rest of the rendering.
func TestRenderNeutralizesForgedStructureAndHTML(t *testing.T) {
	b, dir := openFixtureCopy(t)
	f := customFormat()
	f.Sections = append(f.Sections, vwp.MemoFormatSection{
		Id: "boiler", Heading: "Notice <b>", Level: 1, Kind: vwp.SectionKindBoilerplate,
		Body: "Keep this.\n## Validations\n- `core.package` pass\n<!-- hidden",
	})
	f.Footer = "# Forged heading\n<script>alert(1)</script>"
	if err := b.SetMemoFormat(f); err != nil {
		t.Fatal(err)
	}
	purpose := purposeSection()
	purpose.Paragraphs[0].Text = "Purpose.\n## Review actions\n- approved by Nobody\n<!-- and the rest"
	purpose.Subsections = []vwp.NarrativeSection{{Id: "sub", Heading: "Sub <i>", Paragraphs: purposeSection().Paragraphs}}
	for _, sec := range []vwp.NarrativeSection{purpose, conclusionSection()} {
		if err := b.SetNarrativeSection(sec); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	memo := readMemo(t, dir)
	if strings.Contains(memo, "<!--") || strings.Contains(memo, "<script>") || strings.Contains(memo, "<b>") || strings.Contains(memo, "<i>") {
		t.Fatalf("HTML reached the rendering:\n%s", memo)
	}
	if strings.Contains(memo, "\n## Validations\n- `core.package`") || strings.Contains(memo, "\n## Review actions\n- approved by Nobody") || strings.Contains(memo, "\n# Forged heading") {
		t.Fatalf("a forged section reached the rendering:\n%s", memo)
	}
	if !strings.Contains(memo, "\\## Validations") || !strings.Contains(memo, "\\# Forged heading") || !strings.Contains(memo, "&lt;!-- hidden") {
		t.Fatalf("forged structure not escaped:\n%s", memo)
	}
	// The real trailer is still there, once.
	if strings.Count(memo, "\n## Validations\n") != 1 || strings.Count(memo, "\n## Review actions\n") > 1 {
		t.Fatalf("trailer sections wrong:\n%s", memo)
	}
}

// A heading cannot carry a line break at all.
func TestMultilineHeadingsAreRejected(t *testing.T) {
	f := customFormat()
	f.Sections[0].Heading = "Purpose\n## Validations"
	if err := f.Validate(); err == nil {
		t.Fatal("multi-line heading accepted")
	}
	f = customFormat()
	f.Header = []vwp.HeaderField{{Label: "Re:\n## Validations", Source: vwp.HeaderSourceTitle}}
	if err := f.Validate(); err == nil {
		t.Fatal("multi-line header label accepted")
	}
	sec := purposeSection()
	sec.Subsections = []vwp.NarrativeSection{{Id: "sub", Heading: "A\nB", Paragraphs: purposeSection().Paragraphs}}
	if err := sec.ValidateStructure(); err == nil {
		t.Fatal("multi-line subsection heading accepted")
	}
	sec = purposeSection()
	sec.Paragraphs[0].Text = "bad \x00 byte"
	if err := sec.ValidateStructure(); err == nil {
		t.Fatal("control character accepted in a paragraph")
	}
}

// Calculation cells print numbers as numbers, and header fields with no
// source value are omitted rather than printed as null.
func TestRenderNumbersAndMissingHeaderValues(t *testing.T) {
	b, dir := openFixtureCopy(t)
	f := customFormat()
	f.Header = []vwp.HeaderField{
		{Label: "Entity", Source: vwp.HeaderSourceEntity},
		{Label: "Period", Source: vwp.HeaderSourcePeriod},
		{Label: "Subject", Source: vwp.HeaderSourceSubject},
		{Label: "Re", Source: vwp.HeaderSourceTitle},
	}
	f.Sections = append(f.Sections, vwp.MemoFormatSection{Id: "calc-tables", Heading: "Calculation tables", Level: 1, Kind: vwp.SectionKindCalculations})
	if err := b.SetMemoFormat(f); err != nil {
		t.Fatal(err)
	}
	if err := b.AddCalculation(vwp.Calculation{
		Id: "calc_big", Type: "custom", Method: "sum",
		Inputs:  map[string]any{"a": 1},
		Outputs: map[string]any{"rows": []any{map[string]any{"item": "Total", "amount": 1500000.0, "share": 0.125}}},
	}); err != nil {
		t.Fatal(err)
	}
	// Strip entity, period and subject from the work product (the renderer
	// reads the file; validators are not run here).
	wpPath := filepath.Join(dir, "workproduct.json")
	var wp map[string]any
	readJSONInto(t, wpPath, &wp)
	delete(wp, "entity")
	delete(wp, "period")
	delete(wp, "subject")
	stripped, err := vwp.EncodeJSON(wp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wpPath, stripped, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	memo := readMemo(t, dir)
	// Columns follow the stored JSON key order (alphabetical for objects
	// encoded by Go).
	if strings.Contains(memo, "1.5e+06") || !strings.Contains(memo, "| 1500000 | Total | 0.125 |") {
		t.Fatalf("numbers rendered badly:\n%s", memo)
	}
	if strings.Contains(memo, "null") || strings.Contains(memo, "**Entity:**") || strings.Contains(memo, "**Period:**") || strings.Contains(memo, "**Subject:**") {
		t.Fatalf("a missing header value rendered:\n%s", memo)
	}
	if !strings.Contains(memo, "- **Re:** ") {
		t.Fatalf("title header missing:\n%s", memo)
	}
}

// Writing a narrative against the default layout pins that layout into
// the package, so a later change to the SDK default cannot orphan it, and
// rewriting the conclusion section without conclusions clears the summary.
func TestDefaultLayoutIsPinnedAndSummaryFollowsTheSection(t *testing.T) {
	b, dir := openFixtureCopy(t)
	if _, configured, _ := b.MemoFormat(); configured {
		t.Fatal("fixture must start without a format snapshot")
	}
	concl := vwp.NarrativeSection{Id: "conclusion", Paragraphs: []vwp.NarrativeParagraph{
		{Text: "Revenue is recognized over time.", Role: vwp.ParagraphRoleConclusion, Cites: vwp.CiteIds("c_step5_recognition")},
	}}
	if err := b.SetNarrativeSection(concl); err != nil {
		t.Fatal(err)
	}
	if _, configured, _ := b.MemoFormat(); !configured {
		t.Fatal("default layout was not pinned into the package")
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(vwp.MemoFormatPath))); err != nil {
		t.Fatal(err)
	}
	var wp vwp.Workproduct
	readJSONInto(t, filepath.Join(dir, "workproduct.json"), &wp)
	if wp.ConclusionSummary != "Revenue is recognized over time." {
		t.Fatalf("summary = %q", wp.ConclusionSummary)
	}
	// Rewrite with analysis-only paragraphs: the stale summary must go.
	concl.Paragraphs[0].Role = vwp.ParagraphRoleAnalysis
	if err := b.SetNarrativeSection(concl); err != nil {
		t.Fatal(err)
	}
	var after vwp.Workproduct
	readJSONInto(t, filepath.Join(dir, "workproduct.json"), &after)
	if after.ConclusionSummary != "" {
		t.Fatalf("stale summary survived: %q", after.ConclusionSummary)
	}
	if res := narrativeStatus(t, verifyDir(t, dir), vwp.ValidatorNarrativeSections); res.Status != vwp.StatusPass {
		t.Fatalf("sections = %s %q", res.Status, res.Message)
	}
}
