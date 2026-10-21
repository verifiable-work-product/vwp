package builder_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
)

const legacyGolden = "../testdata/golden/asc606-default-memo.md"

// openFixtureCopy copies the ASC 606 fixture into a temp dir and opens it.
func openFixtureCopy(t *testing.T) (*builder.Builder, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(asc606Fixture)); err != nil {
		t.Fatal(err)
	}
	b, err := builder.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b, dir
}

func customFormat() vwp.MemoFormat {
	return vwp.MemoFormat{
		Version: 1,
		Name:    "Acme house style",
		Header: []vwp.HeaderField{
			{Label: "Re", Source: vwp.HeaderSourceTitle},
			{Label: "Entity", Source: vwp.HeaderSourceEntity},
			{Label: "Period", Source: vwp.HeaderSourcePeriod},
			{Label: "Prepared by", Source: vwp.HeaderSourcePreparer},
			{Label: "Reviewed by", Source: vwp.HeaderSourceApprovers},
			{Label: "Classification", Source: vwp.HeaderSourceLiteral, Value: "Confidential"},
		},
		Sections: []vwp.MemoFormatSection{
			{Id: "purpose", Heading: "Purpose and scope", Level: 1, Kind: vwp.SectionKindNarrative, Required: true},
			{Id: "guidance", Heading: "Authoritative guidance", Level: 1, Kind: vwp.SectionKindBoilerplate, Body: "ASC 606-10-25-1 sets out the contract criteria."},
			{Id: "analysis", Heading: "Analysis", Level: 1, Kind: vwp.SectionKindNarrative, TemplateSection: "s2"},
			{Id: "alternatives", Heading: "Alternatives considered", Level: 2, Kind: vwp.SectionKindAlternatives},
			{Id: "calcs", Heading: "Supporting calculations", Level: 1, Kind: vwp.SectionKindCalculations},
			{Id: "conclusion", Heading: "Our conclusion", Level: 1, Kind: vwp.SectionKindConclusion, Required: true},
			{Id: "sources", Heading: "Documents relied upon", Level: 1, Kind: vwp.SectionKindSources},
		},
		Footer: "Prepared with Wavie.",
	}
}

func TestSetMemoFormatAndMemoFormat(t *testing.T) {
	b, dir := openFixtureCopy(t)

	f, configured, err := b.MemoFormat()
	if err != nil || configured {
		t.Fatalf("MemoFormat before set: configured=%v err=%v", configured, err)
	}
	if f.Name != vwp.DefaultMemoFormat().Name {
		t.Fatalf("expected the default format, got %q", f.Name)
	}

	if err := b.SetMemoFormat(vwp.MemoFormat{Version: 2}); !errors.Is(err, vwp.ErrInvalidMemoFormat) {
		t.Fatalf("invalid format accepted: %v", err)
	}
	if err := b.SetMemoFormat(customFormat()); err != nil {
		t.Fatalf("SetMemoFormat: %v", err)
	}
	f, configured, err = b.MemoFormat()
	if err != nil || !configured || f.Name != "Acme house style" {
		t.Fatalf("MemoFormat after set: %+v configured=%v err=%v", f.Name, configured, err)
	}
	m := b.Manifest()
	part, ok := m.FindPart(vwp.MemoFormatPath)
	if !ok || part.Role != builder.RoleMemoFormat || part.MediaType != "application/json" {
		t.Fatalf("format part not indexed: %+v %v", part, ok)
	}
	raw := mustRead(t, filepath.Join(dir, filepath.FromSlash(vwp.MemoFormatPath)))
	if vwp.SHA256Hex(raw) != part.SHA256 {
		t.Fatal("format part hash not recorded")
	}

	// Overwrite is allowed: formats are configuration.
	g := customFormat()
	g.Name = "Acme house style v2"
	if err := b.SetMemoFormat(g); err != nil {
		t.Fatalf("second SetMemoFormat: %v", err)
	}
	if f, _, _ := b.MemoFormat(); f.Name != "Acme house style v2" {
		t.Fatalf("format not overwritten: %q", f.Name)
	}
	// A format that demands prose sections is not satisfied until they are
	// written: narrative.sections is the only failure.
	assertOnlyFailure(t, verifyDir(t, dir), vwp.ValidatorNarrativeSections)
}

// assertOnlyFailure asserts that id is the one failing validator.
func assertOnlyFailure(t *testing.T, report *vwp.Report, id string) {
	t.Helper()
	seen := false
	for _, r := range report.Results {
		if r.ValidatorId == id {
			seen = true
			if r.Status != vwp.StatusFail {
				t.Fatalf("%s = %s, want fail", id, r.Status)
			}
			continue
		}
		if r.Status == vwp.StatusFail {
			t.Fatalf("unexpected failure %s: %s", r.ValidatorId, r.Message)
		}
	}
	if !seen {
		t.Fatalf("%s did not run", id)
	}
}

func purposeSection() vwp.NarrativeSection {
	return vwp.NarrativeSection{
		Id: "purpose",
		Paragraphs: []vwp.NarrativeParagraph{{
			Text: "This memo documents the revenue recognition for the Acme order form.",
			Role: vwp.ParagraphRoleAnalysis, Cites: vwp.CiteIds("f_contract_parties_date", "ev_order_form"),
		}},
	}
}

func conclusionSection() vwp.NarrativeSection {
	return vwp.NarrativeSection{
		Id: "conclusion",
		Paragraphs: []vwp.NarrativeParagraph{
			{Text: "The contract contains three performance obligations.", Role: vwp.ParagraphRoleConclusion, Cites: vwp.CiteIds("c_step2_performance_obligations")},
			{Text: "The transaction price of 300,000 USD is allocated on a relative SSP basis.", Role: vwp.ParagraphRoleConclusion, Cites: vwp.CiteIds("c_step4_allocation", "calc_ssp_allocation")},
		},
	}
}

func TestSetNarrativeSectionWritesPartAndConclusionSummary(t *testing.T) {
	b, dir := openFixtureCopy(t)
	if err := b.SetMemoFormat(customFormat()); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := b.Narrative(); ok || err != nil {
		t.Fatalf("Narrative before write: ok=%v err=%v", ok, err)
	}

	// Structural problems and unknown or non-prose section ids are rejected.
	bad := purposeSection()
	bad.Paragraphs[0].Cites = nil
	if err := b.SetNarrativeSection(bad); !errors.Is(err, vwp.ErrInvalidNarrative) {
		t.Fatalf("section without cites accepted: %v", err)
	}
	unknown := purposeSection()
	unknown.Id = "nonexistent"
	if err := b.SetNarrativeSection(unknown); err == nil {
		t.Fatal("unknown format section accepted")
	}
	notProse := purposeSection()
	notProse.Id = "calcs"
	if err := b.SetNarrativeSection(notProse); err == nil {
		t.Fatal("non-prose format section accepted")
	}

	if err := b.SetNarrativeSection(purposeSection()); err != nil {
		t.Fatalf("SetNarrativeSection purpose: %v", err)
	}
	n, ok, err := b.Narrative()
	if err != nil || !ok {
		t.Fatalf("Narrative: ok=%v err=%v", ok, err)
	}
	formatRaw := mustRead(t, filepath.Join(dir, filepath.FromSlash(vwp.MemoFormatPath)))
	formatHash, err := vwp.SelfHash(vwp.KindMemoFormat, json.RawMessage(formatRaw))
	if err != nil {
		t.Fatal(err)
	}
	if n.FormatHash != formatHash {
		t.Errorf("FormatHash = %s, want self hash of the format part", n.FormatHash)
	}
	digests, err := b.Digests()
	if err != nil {
		t.Fatal(err)
	}
	if n.AnalysisRoot != digests.AnalysisRoot {
		t.Errorf("AnalysisRoot = %s, want %s", n.AnalysisRoot, digests.AnalysisRoot)
	}
	// Cites were passed by id and come back stamped with closure hashes.
	g, err := b.Graph()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range n.Sections[0].Paragraphs[0].Cites {
		obj, ok := g.Lookup(c.Id)
		if !ok || c.Hash != obj.ClosureHash {
			t.Errorf("cite %s stamped %q, want closure hash %q", c.Id, c.Hash, obj.ClosureHash)
		}
	}
	if len(n.Sections) != 1 || n.Sections[0].Id != "purpose" {
		t.Fatalf("sections = %+v", n.Sections)
	}
	m := b.Manifest()
	part, ok := m.FindPart(vwp.NarrativePath)
	if !ok || part.Role != builder.RoleNarrative {
		t.Fatalf("narrative part not indexed: %+v %v", part, ok)
	}

	// Conclusion-kind sections fill the workproduct's conclusionSummary and
	// the digest stamped on the narrative reflects that rewrite.
	if err := b.SetNarrativeSection(conclusionSection()); err != nil {
		t.Fatalf("SetNarrativeSection conclusion: %v", err)
	}
	var wp vwp.Workproduct
	readJSONInto(t, filepath.Join(dir, "workproduct.json"), &wp)
	want := "The contract contains three performance obligations.\n\nThe transaction price of 300,000 USD is allocated on a relative SSP basis."
	if wp.ConclusionSummary != want {
		t.Errorf("ConclusionSummary = %q", wp.ConclusionSummary)
	}
	n, _, _ = b.Narrative()
	digests, _ = b.Digests()
	if n.AnalysisRoot != digests.AnalysisRoot {
		t.Errorf("AnalysisRoot after conclusion = %s, want %s", n.AnalysisRoot, digests.AnalysisRoot)
	}
	if len(n.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(n.Sections))
	}

	// Upsert replaces the whole section, subsections included.
	replaced := purposeSection()
	replaced.Paragraphs[0].Text = "Rewritten purpose."
	replaced.Subsections = []vwp.NarrativeSection{{Id: "scope", Heading: "Scope", Paragraphs: []vwp.NarrativeParagraph{{Text: "Scope text.", Role: vwp.ParagraphRoleAnalysis, Cites: vwp.CiteIds("f_contract_term")}}}}
	if err := b.SetNarrativeSection(replaced); err != nil {
		t.Fatal(err)
	}
	n, _, _ = b.Narrative()
	if len(n.Sections) != 2 || n.Sections[0].Paragraphs[0].Text != "Rewritten purpose." || len(n.Sections[0].Subsections) != 1 {
		t.Fatalf("upsert did not replace the section: %+v", n.Sections[0])
	}
	if report := verifyDir(t, dir); !report.Valid {
		t.Fatalf("package invalid after narrative writes; statuses: %v", statuses(report))
	}

	// ClearNarrative removes the part and the summary it set.
	if err := b.ClearNarrative(); err != nil {
		t.Fatalf("ClearNarrative: %v", err)
	}
	if _, ok, _ := b.Narrative(); ok {
		t.Fatal("narrative still present after clear")
	}
	if m := b.Manifest(); func() bool { _, ok := m.FindPart(vwp.NarrativePath); return ok }() {
		t.Fatal("narrative part still indexed after clear")
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(vwp.NarrativePath))); !os.IsNotExist(err) {
		t.Fatalf("narrative file still on disk: %v", err)
	}
	var cleared vwp.Workproduct
	readJSONInto(t, filepath.Join(dir, "workproduct.json"), &cleared)
	if cleared.ConclusionSummary != "" {
		t.Errorf("ConclusionSummary not cleared: %q", cleared.ConclusionSummary)
	}
	if err := b.ClearNarrative(); err != nil {
		t.Fatalf("second ClearNarrative: %v", err)
	}
	// The format still demands its required prose sections.
	assertOnlyFailure(t, verifyDir(t, dir), vwp.ValidatorNarrativeSections)
}

func TestSetNarrativeSectionAgainstDefaultFormat(t *testing.T) {
	b, _ := openFixtureCopy(t)
	// No snapshot: the default format decides which ids are prose.
	if err := b.SetNarrativeSection(conclusionSection()); err != nil {
		t.Fatalf("SetNarrativeSection: %v", err)
	}
	n, _, _ := b.Narrative()
	want, _ := vwp.SelfHash(vwp.KindMemoFormat, vwp.DefaultMemoFormat())
	if n.FormatHash != want {
		t.Errorf("FormatHash = %s, want self hash of the default format", n.FormatHash)
	}
	if err := b.SetNarrativeSection(purposeSection()); err == nil {
		t.Fatal("purpose is not a default format section; should be rejected")
	}
}

func TestClearNarrativeKeepsForeignConclusionSummary(t *testing.T) {
	b, dir := openFixtureCopy(t)
	var before vwp.Workproduct
	readJSONInto(t, filepath.Join(dir, "workproduct.json"), &before)
	if before.ConclusionSummary == "" {
		t.Fatal("fixture should carry a conclusionSummary")
	}
	if err := b.SetMemoFormat(customFormat()); err != nil {
		t.Fatal(err)
	}
	if err := b.SetNarrativeSection(purposeSection()); err != nil {
		t.Fatal(err)
	}
	if err := b.ClearNarrative(); err != nil {
		t.Fatal(err)
	}
	var after vwp.Workproduct
	readJSONInto(t, filepath.Join(dir, "workproduct.json"), &after)
	if after.ConclusionSummary != before.ConclusionSummary {
		t.Errorf("a summary the narrative did not write was cleared: %q", after.ConclusionSummary)
	}
}

func TestDigestsMatchTheGraphAndTrackContent(t *testing.T) {
	b, _ := openFixtureCopy(t)
	got, err := b.Digests()
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := vwp.OpenPackage(b.Dir())
	if err != nil {
		t.Fatal(err)
	}
	want, err := vwp.ComputeDigests(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatalf("Digests = %+v, want %+v", got, want)
	}
	// Shipping validations and rendering are derived parts: the roots hold,
	// and the manifest on disk carries them.
	if _, err := b.RunValidators(nil); err != nil {
		t.Fatal(err)
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	if again, _ := b.Digests(); !again.Equal(got) {
		t.Fatalf("roots moved after RunValidators+Render: %+v", again)
	}
	var manifest vwp.Manifest
	readJSONInto(t, filepath.Join(b.Dir(), "manifest.json"), &manifest)
	if manifest.Digests == nil || !manifest.Digests.Equal(got) {
		t.Fatalf("manifest digests = %+v, want %+v", manifest.Digests, got)
	}
	// A new fact moves the analysis and content roots but not the review root.
	if err := b.AddFact(vwp.Fact{Id: "f_new", Statement: "A new observation.", EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev_order_form"}}}); err != nil {
		t.Fatal(err)
	}
	after, _ := b.Digests()
	if after.AnalysisRoot == got.AnalysisRoot || after.ContentRoot == got.ContentRoot {
		t.Fatal("analysis or content root did not move after AddFact")
	}
	if after.ReviewRoot != got.ReviewRoot {
		t.Fatal("review root moved after AddFact")
	}
}

// TestRenderDefaultFormatMatchesLegacy pins the rendering produced before
// memo formats existed: RenderWith(DefaultMemoFormat()) and a plain Render()
// on a package without a format snapshot reproduce it byte for byte.
func TestRenderDefaultFormatMatchesLegacy(t *testing.T) {
	want := mustRead(t, legacyGolden)

	b, dir := openFixtureCopy(t)
	if err := b.RenderWith(vwp.DefaultMemoFormat()); err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	got := mustRead(t, filepath.Join(dir, "renderings", "memo.md"))
	if !bytes.Equal(got, want) {
		t.Fatalf("RenderWith(DefaultMemoFormat()) differs from the legacy golden:\n%s", diffHint(got, want))
	}

	b2, dir2 := openFixtureCopy(t)
	if err := b2.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got2 := mustRead(t, filepath.Join(dir2, "renderings", "memo.md"))
	if !bytes.Equal(got2, want) {
		t.Fatalf("Render() without a format snapshot differs from the legacy golden:\n%s", diffHint(got2, want))
	}
}

func diffHint(got, want []byte) string {
	g := strings.Split(string(got), "\n")
	w := strings.Split(string(want), "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return fmt.Sprintf("first difference at line %d:\n got: %s\nwant: %s", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("line counts differ: got %d, want %d", len(g), len(w))
}
