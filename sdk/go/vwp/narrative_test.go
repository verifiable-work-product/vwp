package vwp

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func validFormat() MemoFormat {
	return MemoFormat{
		Version: 1,
		Header:  []HeaderField{{Label: "Re:", Source: HeaderSourceTitle}, {Label: "Prepared for", Source: HeaderSourceLiteral, Value: "Audit committee"}},
		Sections: []MemoFormatSection{
			{Id: "purpose", Heading: "Purpose", Level: 1, Kind: SectionKindNarrative},
			{Id: "guidance", Heading: "Authoritative guidance", Level: 1, Kind: SectionKindBoilerplate, Body: "ASC 606-10-25-1 ..."},
			{Id: "conclusion", Heading: "Conclusion", Level: 1, Kind: SectionKindConclusion, Required: true},
		},
		Footer: "Confidential",
	}
}

func TestMemoFormatValidate(t *testing.T) {
	if err := validFormat().Validate(); err != nil {
		t.Fatalf("valid format rejected: %v", err)
	}
	if err := DefaultMemoFormat().Validate(); err != nil {
		t.Fatalf("default format rejected: %v", err)
	}

	cases := map[string]func(f *MemoFormat){
		"version":              func(f *MemoFormat) { f.Version = 2 },
		"no sections":          func(f *MemoFormat) { f.Sections = nil },
		"bad id":               func(f *MemoFormat) { f.Sections[0].Id = "Purpose!" },
		"duplicate id":         func(f *MemoFormat) { f.Sections[1].Id = "purpose" },
		"no heading":           func(f *MemoFormat) { f.Sections[0].Heading = " " },
		"level":                func(f *MemoFormat) { f.Sections[0].Level = 4 },
		"kind":                 func(f *MemoFormat) { f.Sections[0].Kind = "prose" },
		"boilerplate body":     func(f *MemoFormat) { f.Sections[1].Body = "" },
		"body on narrative":    func(f *MemoFormat) { f.Sections[0].Body = "x" },
		"no prose":             func(f *MemoFormat) { f.Sections = f.Sections[1:2] },
		"header source":        func(f *MemoFormat) { f.Header[0].Source = "random" },
		"literal no value":     func(f *MemoFormat) { f.Header[1].Value = "" },
		"value on non-literal": func(f *MemoFormat) { f.Header[0].Value = "x" },
		"guidance too long":    func(f *MemoFormat) { f.Sections[0].Guidance = strings.Repeat("g", MaxMemoFormatGuidance+1) },
		"footer too long":      func(f *MemoFormat) { f.Footer = strings.Repeat("f", MaxMemoFormatFooter+1) },
		"template section id":  func(f *MemoFormat) { f.Sections[0].TemplateSection = "S 2" },
		"header over cap": func(f *MemoFormat) {
			for i := 0; i <= MaxMemoFormatHeader; i++ {
				f.Header = append(f.Header, HeaderField{Label: "L", Source: HeaderSourceTitle})
			}
		},
		"roles on prose kind":  func(f *MemoFormat) { f.Sections[0].Roles = []string{"Global Controller"} },
		"roles on boilerplate": func(f *MemoFormat) { f.Sections[1].Roles = []string{"Global Controller"} },
		"empty role": func(f *MemoFormat) {
			f.Sections = append(f.Sections, MemoFormatSection{Id: "signoff", Heading: "Sign-off", Level: 1, Kind: SectionKindSignoff, Roles: []string{"Global Controller", " "}})
		},
		"multi-line role": func(f *MemoFormat) {
			f.Sections = append(f.Sections, MemoFormatSection{Id: "signoff", Heading: "Sign-off", Level: 1, Kind: SectionKindSignoff, Roles: []string{"Global\nController"}})
		},
		"too many roles": func(f *MemoFormat) {
			roles := make([]string, MaxMemoFormatRoles+1)
			for i := range roles {
				roles[i] = fmt.Sprintf("Role %d", i)
			}
			f.Sections = append(f.Sections, MemoFormatSection{Id: "signoff", Heading: "Sign-off", Level: 1, Kind: SectionKindSignoff, Roles: roles})
		},
	}
	for name, mutate := range cases {
		f := validFormat()
		mutate(&f)
		err := f.Validate()
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if !errors.Is(err, ErrInvalidMemoFormat) {
			t.Errorf("%s: error %v does not wrap ErrInvalidMemoFormat", name, err)
		}
	}
}

// The DTCC short-form memo: a 24-line header table with the package date
// and the authoritative literature, and a sign-off table by role.
func TestMemoFormatAcceptsDTCCShapes(t *testing.T) {
	f := validFormat()
	f.Header = nil
	for i := 0; i < MaxMemoFormatHeader-2; i++ {
		f.Header = append(f.Header, HeaderField{Label: fmt.Sprintf("Line %d", i), Source: HeaderSourceLiteral, Value: "x"})
	}
	f.Header = append(f.Header,
		HeaderField{Label: "Date", Source: HeaderSourceDate},
		HeaderField{Label: "Authoritative literature", Source: HeaderSourceStandardRefs},
	)
	if len(f.Header) != 24 || MaxMemoFormatHeader != 24 {
		t.Fatalf("header cap = %d, want 24", MaxMemoFormatHeader)
	}
	f.Sections = append(f.Sections, MemoFormatSection{
		Id: "signoff", Heading: "Sign-off", Level: 1, Kind: SectionKindSignoff,
		Roles: []string{"Global Controller", "Technical Accounting Manager", "Financial Reporting and Accounting", "Regional Controller", "Legal Entity Controller"},
	})
	if err := f.Validate(); err != nil {
		t.Fatalf("DTCC-shaped format rejected: %v", err)
	}
	if MaxMemoFormatRoles != 24 {
		t.Fatalf("roles cap = %d, want 24", MaxMemoFormatRoles)
	}
	// A value on the new sources is rejected like any non-literal source.
	f.Header[23].Value = "ASC 606"
	if err := f.Validate(); !errors.Is(err, ErrInvalidMemoFormat) {
		t.Fatalf("value on standardRefs source accepted: %v", err)
	}
}

func TestMemoFormatLookups(t *testing.T) {
	f := validFormat()
	if s, ok := f.Section("guidance"); !ok || s.Kind != SectionKindBoilerplate {
		t.Fatalf("Section lookup failed: %+v %v", s, ok)
	}
	if _, ok := f.Section("missing"); ok {
		t.Fatal("missing section found")
	}
	if !f.HasKind(SectionKindConclusion) || f.HasKind(SectionKindSignoff) {
		t.Fatal("HasKind wrong")
	}
	if !IsProseKind(SectionKindNarrative) || IsProseKind(SectionKindClaims) {
		t.Fatal("IsProseKind wrong")
	}
	if !IsTrailerKind(SectionKindSignoff) || IsTrailerKind(SectionKindSources) {
		t.Fatal("IsTrailerKind wrong")
	}
}

func TestNarrativeSectionValidateStructure(t *testing.T) {
	good := NarrativeSection{Id: "purpose", Paragraphs: []NarrativeParagraph{{Text: "We analysed the MSA.", Role: ParagraphRoleAnalysis, Cites: CiteIds("s1_fact_msa")}},
		Subsections: []NarrativeSection{{Id: "po-platform", Heading: "Platform subscription", Paragraphs: []NarrativeParagraph{{Text: "Distinct.", Role: ParagraphRoleConclusion, Cites: CiteIds("s2_claim_platform")}}}}}
	if err := good.ValidateStructure(); err != nil {
		t.Fatalf("valid section rejected: %v", err)
	}
	literature := NarrativeSection{Id: "analysis", Paragraphs: []NarrativeParagraph{{Text: "ASC 606-10-25-1 A contract exists when ...", Role: ParagraphRoleLiterature, Cites: CiteIds("s2_claim_contract")}}}
	if err := literature.ValidateStructure(); err != nil {
		t.Fatalf("literature paragraph rejected: %v", err)
	}
	notApplicable := NarrativeSection{Id: "tax", NotApplicable: "No tax or treasury impact identified."}
	if err := notApplicable.ValidateStructure(); err != nil {
		t.Fatalf("not-applicable section rejected: %v", err)
	}
	// A subsection may be marked not applicable too (same checks).
	nested := NarrativeSection{Id: "analysis", Paragraphs: good.Paragraphs, Subsections: []NarrativeSection{{Id: "tax", Heading: "Tax", NotApplicable: "No tax impact."}}}
	if err := nested.ValidateStructure(); err != nil {
		t.Fatalf("not-applicable subsection rejected: %v", err)
	}
	if MaxNarrativeNotApplicable != 512 {
		t.Fatalf("notApplicable cap = %d, want 512", MaxNarrativeNotApplicable)
	}
	cases := map[string]NarrativeSection{
		"na with paragraphs":  {Id: "tax", NotApplicable: "No tax impact.", Paragraphs: good.Paragraphs},
		"na with subsections": {Id: "tax", NotApplicable: "No tax impact.", Subsections: good.Subsections},
		"na blank":            {Id: "tax", NotApplicable: " "},
		"na multi-line":       {Id: "tax", NotApplicable: "No tax\nimpact."},
		"na too long":         {Id: "tax", NotApplicable: strings.Repeat("n", MaxNarrativeNotApplicable+1)},
		"bad id":              {Id: "Purpose", Paragraphs: good.Paragraphs},
		"empty":               {Id: "purpose"},
		"empty text":          {Id: "purpose", Paragraphs: []NarrativeParagraph{{Text: " ", Role: ParagraphRoleAnalysis, Cites: CiteIds("x")}}},
		"bad role":            {Id: "purpose", Paragraphs: []NarrativeParagraph{{Text: "t", Role: "summary", Cites: CiteIds("x")}}},
		"no cites":            {Id: "purpose", Paragraphs: []NarrativeParagraph{{Text: "t", Role: ParagraphRoleAnalysis}}},
		"blank cite":          {Id: "purpose", Paragraphs: []NarrativeParagraph{{Text: "t", Role: ParagraphRoleAnalysis, Cites: CiteIds(" ")}}},
		"too long":            {Id: "purpose", Paragraphs: []NarrativeParagraph{{Text: strings.Repeat("t", MaxNarrativeParagraph+1), Role: ParagraphRoleAnalysis, Cites: CiteIds("x")}}},
		"sub no heading":      {Id: "purpose", Subsections: []NarrativeSection{{Id: "a", Paragraphs: good.Paragraphs}}},
		"too deep":            {Id: "purpose", Subsections: []NarrativeSection{{Id: "a", Heading: "A", Subsections: []NarrativeSection{{Id: "b", Heading: "B", Paragraphs: good.Paragraphs}}}}},
		"dup sub":             {Id: "purpose", Subsections: []NarrativeSection{{Id: "a", Heading: "A", Paragraphs: good.Paragraphs}, {Id: "a", Heading: "A2", Paragraphs: good.Paragraphs}}},
	}
	for name, sec := range cases {
		err := sec.ValidateStructure()
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if !errors.Is(err, ErrInvalidNarrative) {
			t.Errorf("%s: error %v does not wrap ErrInvalidNarrative", name, err)
		}
	}
}

func TestCanonicalJSONIsStable(t *testing.T) {
	a, err := CanonicalJSON(DefaultMemoFormat())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := CanonicalJSON(DefaultMemoFormat())
	if string(a) != string(b) {
		t.Fatal("canonical JSON differs between calls")
	}
	if !strings.HasSuffix(string(a), "\n") || !strings.Contains(string(a), "\"sections\": [") {
		t.Fatalf("unexpected encoding: %s", a)
	}
}
