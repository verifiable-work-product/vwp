package builder_test

import (
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
)

// dtccFormat is the shape of DTCC's short-form Accounting Technical Memo:
// a header table, a technical analysis section that alternates literature
// and applicability, sections that may not apply, and a sign-off table by
// role.
func dtccFormat() vwp.MemoFormat {
	return vwp.MemoFormat{
		Version: 1,
		Name:    "DTCC short-form technical memo",
		Header: []vwp.HeaderField{
			{Label: "Memo number", Source: vwp.HeaderSourceLiteral, Value: "ATM-2026-014"},
			{Label: "Memo name", Source: vwp.HeaderSourceTitle},
			{Label: "Date", Source: vwp.HeaderSourceDate},
			{Label: "Business unit", Source: vwp.HeaderSourceSubject},
			{Label: "Legal entity", Source: vwp.HeaderSourceEntity},
			{Label: "Preparer", Source: vwp.HeaderSourcePreparer},
			{Label: "Authoritative literature", Source: vwp.HeaderSourceStandardRefs},
			{Label: "Other sources", Source: vwp.HeaderSourceLiteral, Value: "None"},
			{Label: "Parties consulted", Source: vwp.HeaderSourceApprovers},
		},
		Sections: []vwp.MemoFormatSection{
			{Id: "purpose", Heading: "Purpose", Level: 1, Kind: vwp.SectionKindNarrative, Required: true},
			{Id: "analysis", Heading: "Technical accounting analysis", Level: 1, Kind: vwp.SectionKindNarrative, Required: true},
			{Id: "tax", Heading: "Tax and treasury considerations", Level: 1, Kind: vwp.SectionKindNarrative, Required: true},
			{Id: "conclusion", Heading: "Conclusion", Level: 1, Kind: vwp.SectionKindConclusion, Required: true},
			{Id: "signoff", Heading: "Sign-off", Level: 1, Kind: vwp.SectionKindSignoff,
				Roles: []string{"Global Controller", "Technical Accounting Manager", "Financial Reporting and Accounting", "Regional Controller", "Legal Entity Controller"}},
		},
	}
}

func dtccPackage(t *testing.T) (*builder.Builder, string) {
	t.Helper()
	b, dir := openFixtureCopy(t)
	if err := b.SetMemoFormat(dtccFormat()); err != nil {
		t.Fatal(err)
	}
	sections := []vwp.NarrativeSection{
		purposeSection(),
		{Id: "analysis", Subsections: []vwp.NarrativeSection{
			{Id: "contract", Heading: "ASC 606-10-25-1 Identifying the contract", Paragraphs: []vwp.NarrativeParagraph{
				{Text: "ASC 606-10-25-1: An entity shall account for a contract with a customer only when all of the following criteria are met:\n(a) the parties have approved the contract ...", Role: vwp.ParagraphRoleLiterature, Cites: vwp.CiteIds("c_step1_contract_identified")},
				{Text: "Applicability to DTCC: the order form is approved by both parties.", Role: vwp.ParagraphRoleAnalysis, Cites: vwp.CiteIds("c_step1_contract_identified", "f_contract_parties_date")},
			}},
		}},
		{Id: "tax", NotApplicable: "No tax or treasury impact identified."},
		conclusionSection(),
	}
	for _, s := range sections {
		if err := b.SetNarrativeSection(s); err != nil {
			t.Fatalf("SetNarrativeSection %s: %v", s.Id, err)
		}
	}
	return b, dir
}

// The header renders every field in order, the package date as a calendar
// date and the literature as the sorted union of the claims' standardRefs;
// a field whose source yields nothing keeps its slot with an empty value.
func TestRenderHeaderDateAndStandardRefs(t *testing.T) {
	b, dir := dtccPackage(t)
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	memo := readMemo(t, dir)
	want := strings.Join([]string{
		"- **Memo number:** ATM-2026-014",
		"- **Memo name:** ASC 606 Revenue Recognition Workpaper - Acme Contract",
		"- **Date:** 2026-07-07",
		"- **Business unit:** customerContract contract_acme_001",
		"- **Legal entity:** DemoCo",
		"- **Preparer:** vwp-demo-agent",
		"- **Authoritative literature:** ASC 606-10-25-1; ASC 606-10-25-14; ASC 606-10-25-23; ASC 606-10-32-2; ASC 606-10-32-31",
		"- **Other sources:** None",
		"- **Parties consulted:** Demo Preparer",
		"",
	}, "\n")
	if !strings.Contains(memo, want) {
		t.Fatalf("header block differs; want:\n%s\ngot:\n%s", want, memo)
	}

	// No claims at all: the literature slot is kept, empty.
	b2, dir2 := openFixtureCopy(t)
	f := dtccFormat()
	f.Sections = f.Sections[:4]
	if err := b2.SetMemoFormat(f); err != nil {
		t.Fatal(err)
	}
	if err := b2.ReplaceFilePart(vwp.ClaimsPath, "application/json", builder.RoleClaims, []byte(`{"claims": []}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := b2.Render(); err != nil {
		t.Fatal(err)
	}
	if memo := readMemo(t, dir2); !strings.Contains(memo, "\n- **Authoritative literature:** \n") {
		t.Fatalf("empty literature slot missing:\n%s", memo)
	}
}

// A sign-off section with roles renders a table with one row per role,
// filled from the latest approved action in that role; approvals that fill
// no row are listed after the table.
func TestRenderSignoffRolesTable(t *testing.T) {
	b, dir := dtccPackage(t)
	approvals := []vwp.Approval{
		{Id: "app_gc_1", Approver: vwp.Approver{Name: "Grace Chen"}, Role: "global controller", Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-19T09:00:00Z"},
		{Id: "app_tam_reject", Approver: vwp.Approver{Name: "Tom Alvarez"}, Role: "Technical Accounting Manager", Action: vwp.ApprovalActionRejected, Timestamp: "2026-10-19T10:00:00Z", Comments: "Please expand step 2."},
		{Id: "app_tam", Approver: vwp.Approver{Name: "Tom Alvarez"}, Role: " Technical Accounting Manager ", Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-20T10:00:00Z"},
		{Id: "app_gc_2", Approver: vwp.Approver{Id: "user_gc2"}, Role: "Global Controller", Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-21T11:30:00Z"},
		{Id: "app_audit", Approver: vwp.Approver{Name: "External Auditor"}, Role: "auditor", Action: vwp.ApprovalActionApproved, Timestamp: "2026-10-21T12:00:00Z"},
	}
	for _, a := range approvals {
		if err := b.RecordApproval(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	memo := readMemo(t, dir)
	want := strings.Join([]string{
		"## Sign-off",
		"",
		"| Role | Name | Date |",
		"|---|---|---|",
		"| Global Controller | user_gc2 | 2026-10-21 |",
		"| Technical Accounting Manager | Tom Alvarez | 2026-10-20 |",
		"| Financial Reporting and Accounting |  |  |",
		"| Regional Controller |  |  |",
		"| Legal Entity Controller |  |  |",
		"",
		"### Other review actions",
		"",
		"- acknowledged by Demo Preparer (preparer) at 2026-07-07T15:05:00Z — scope: package — Draft prepared and submitted for review. Judgment j_impl_distinct pending revenue-manager review.",
		"- approved by Grace Chen (global controller) at 2026-10-19T09:00:00Z",
		"- rejected by Tom Alvarez (Technical Accounting Manager) at 2026-10-19T10:00:00Z — Please expand step 2.",
		"- approved by External Auditor (auditor) at 2026-10-21T12:00:00Z",
		"",
	}, "\n")
	if !strings.Contains(memo, want) {
		t.Fatalf("sign-off table differs; want:\n%s\ngot:\n%s", want, memo)
	}
	// The trailer is not duplicated: the format's sign-off section is it.
	if strings.Count(memo, "| Role | Name | Date |") != 1 || strings.Contains(memo, "## Review actions") {
		t.Fatalf("sign-off rendered twice:\n%s", memo)
	}

	// With no approvals at all the table still shows every pending role
	// and nothing follows it.
	b2, dir2 := dtccPackage(t)
	if err := b2.ReplaceFilePart(vwp.ApprovalsPath, "application/json", builder.RoleApprovals, []byte(`{"approvals": []}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := b2.Render(); err != nil {
		t.Fatal(err)
	}
	memo = readMemo(t, dir2)
	if !strings.Contains(memo, "| Global Controller |  |  |\n") || strings.Contains(memo, "Other review actions") {
		t.Fatalf("pending-only table wrong:\n%s", memo)
	}

	// Without roles today's list rendering stands.
	f := dtccFormat()
	f.Sections[4].Roles = nil
	if err := b.SetMemoFormat(f); err != nil {
		t.Fatal(err)
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	memo = readMemo(t, dir)
	if strings.Contains(memo, "| Role |") || strings.Contains(memo, "Other review actions") || !strings.Contains(memo, "## Sign-off\n\n- acknowledged by Demo Preparer (preparer)") {
		t.Fatalf("roleless sign-off changed:\n%s", memo)
	}
}

// Literature paragraphs render as a blockquote without the conclusion
// marker, and a not-applicable section as a plain sentence under its
// heading.
func TestRenderLiteratureAndNotApplicable(t *testing.T) {
	b, dir := dtccPackage(t)
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	memo := readMemo(t, dir)
	want := strings.Join([]string{
		"## Technical accounting analysis",
		"",
		"### ASC 606-10-25-1 Identifying the contract",
		"",
		"> ASC 606-10-25-1: An entity shall account for a contract with a customer only when all of the following criteria are met:",
		"> (a) the parties have approved the contract ... [3]",
		"",
		"Applicability to DTCC: the order form is approved by both parties. [3] [1]",
		"",
		"## Tax and treasury considerations",
		"",
		"Not applicable: No tax or treasury impact identified.",
		"",
		"## Conclusion",
		"",
		"> **Conclusion.** The contract contains three performance obligations. [4]",
		"",
	}, "\n")
	if !strings.Contains(memo, want) {
		t.Fatalf("analysis rendering differs; want:\n%s\ngot:\n%s", want, memo)
	}
	// The reason is untrusted text: markdown structure cannot open a tag.
	if err := b.SetNarrativeSection(vwp.NarrativeSection{Id: "tax", NotApplicable: "No impact <!-- hidden"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Render(); err != nil {
		t.Fatal(err)
	}
	if memo := readMemo(t, dir); !strings.Contains(memo, "Not applicable: No impact &lt;!-- hidden\n") {
		t.Fatalf("not-applicable reason not escaped:\n%s", memo)
	}
	if report := verifyDir(t, dir); !report.Valid {
		t.Fatalf("package invalid: %v", statuses(report))
	}
}
