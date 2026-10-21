package vwp

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Memo format and narrative: the two package parts that turn a verifiable
// work product into a memo in the organization's own house style.
//
// The memo format (narrative/format.json, role memo-format) is the
// organization's section list: headings in their order and words, which
// kind of package content fills each, static boilerplate they keep
// verbatim, and the header block. It is configuration the organization
// approved, snapshotted into the package so the package renders the same
// offline.
//
// The narrative (narrative/narrative.json, role narrative) is the prose the
// model writes against that format as the last step, after the analysis
// validates. Every paragraph cites the package objects it rests on, so the
// narrative validators can hold the prose to the structured parts. It is a
// content part: it is hashed like claims and facts, so a human approving the
// package approves the prose they read.

const (
	// NarrativePath is the package-relative path of the narrative part.
	NarrativePath = "narrative/narrative.json"
	// MemoFormatPath is the package-relative path of the memo format snapshot.
	MemoFormatPath = "narrative/format.json"
)

// Section kinds of a memo format: what fills the section when rendered.
const (
	// SectionKindNarrative is model-written prose (analysis paragraphs,
	// which may end in conclusion paragraphs).
	SectionKindNarrative = "narrative"
	// SectionKindConclusion is model-written conclusion prose; it also
	// fills Workproduct.ConclusionSummary.
	SectionKindConclusion = "conclusion"
	// SectionKindAlternatives renders the alternatives recorded on judgments.
	SectionKindAlternatives = "alternatives"
	// SectionKindClaims renders the claims list.
	SectionKindClaims = "claims"
	// SectionKindJudgments renders the judgments list.
	SectionKindJudgments = "judgments"
	// SectionKindCalculations renders the calculations as tables.
	SectionKindCalculations = "calculations"
	// SectionKindSources renders the numbered evidence list.
	SectionKindSources = "sources"
	// SectionKindSignoff renders the approvals (review actions).
	SectionKindSignoff = "signoff"
	// SectionKindValidations renders the shipped validation results.
	SectionKindValidations = "validations"
	// SectionKindAttestations renders the analysis attestations.
	SectionKindAttestations = "attestations"
	// SectionKindBoilerplate renders the section's Body verbatim.
	SectionKindBoilerplate = "boilerplate"
)

// Header field sources: where a header block value comes from.
const (
	HeaderSourceTitle     = "title"
	HeaderSourceEntity    = "entity"
	HeaderSourcePeriod    = "period"
	HeaderSourceSubject   = "subject"
	HeaderSourceStatus    = "status"
	HeaderSourcePreparer  = "preparer"
	HeaderSourceApprovers = "approvers"
	HeaderSourceLiteral   = "literal"
)

// Paragraph roles of the narrative.
const (
	ParagraphRoleAnalysis   = "analysis"
	ParagraphRoleConclusion = "conclusion"
)

// Limits of a memo format and a narrative.
const (
	MaxMemoFormatSections = 40
	MaxMemoFormatHeader   = 12
	MaxMemoFormatGuidance = 4 << 10
	MaxMemoFormatBody     = 16 << 10
	MaxMemoFormatFooter   = 4 << 10
	MaxMemoFormatLevel    = 3
	MaxNarrativeParagraph = 8 << 10
	MaxNarrativeDepth     = 1
)

// ErrInvalidMemoFormat wraps every memo format validation failure.
var ErrInvalidMemoFormat = errors.New("invalid memo format")

// ErrInvalidNarrative wraps every structural narrative validation failure.
var ErrInvalidNarrative = errors.New("invalid narrative")

// MemoFormat is an organization's memo layout.
type MemoFormat struct {
	Version  int                 `json:"version"`
	Name     string              `json:"name,omitempty"`
	Source   *MemoFormatSource   `json:"source,omitempty"`
	Header   []HeaderField       `json:"header,omitempty"`
	Sections []MemoFormatSection `json:"sections"`
	Footer   string              `json:"footer,omitempty"`
}

// MemoFormatSource records the customer document the format was derived from.
type MemoFormatSource struct {
	Filename  string `json:"filename,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

// HeaderField is one line of the memo's header block (To, From, Re ...).
type HeaderField struct {
	Label  string `json:"label"`
	Source string `json:"source"`
	Value  string `json:"value,omitempty"`
}

// MemoFormatSection is one section of the memo in the customer's words.
type MemoFormatSection struct {
	Id              string `json:"id"`
	Heading         string `json:"heading"`
	Level           int    `json:"level"`
	Kind            string `json:"kind"`
	TemplateSection string `json:"templateSection,omitempty"`
	Guidance        string `json:"guidance,omitempty"`
	Body            string `json:"body,omitempty"`
	Required        bool   `json:"required,omitempty"`
}

// Narrative is the model-written prose of the memo, section by section.
// FormatHash is the self hash of the memo format snapshot it follows and
// AnalysisRoot the package analysis root when it was last written
// (spec/integrity.md section 6); the builder stamps both.
type Narrative struct {
	FormatHash   string             `json:"formatHash"`
	AnalysisRoot string             `json:"analysisRoot"`
	Sections     []NarrativeSection `json:"sections"`
}

// NarrativeSection holds the paragraphs of one memo section. Top-level
// sections are named by a format section id; subsections are model-named.
type NarrativeSection struct {
	Id          string               `json:"id"`
	Heading     string               `json:"heading,omitempty"`
	Paragraphs  []NarrativeParagraph `json:"paragraphs"`
	Subsections []NarrativeSection   `json:"subsections,omitempty"`
}

// NarrativeParagraph is one paragraph of prose and the package objects it
// cites.
type NarrativeParagraph struct {
	Text  string `json:"text"`
	Role  string `json:"role"`
	Cites []Cite `json:"cites"`
}

// Cite names a package object a paragraph rests on and the object's closure
// hash at the time of writing. Callers of the builder pass the id only; the
// builder resolves and stamps the hash.
type Cite struct {
	Id   string `json:"id"`
	Hash string `json:"hash"`
}

// CiteIds builds cites (hash unset) from object ids, for callers of the
// builder's SetNarrativeSection.
func CiteIds(ids ...string) []Cite {
	out := make([]Cite, 0, len(ids))
	for _, id := range ids {
		out = append(out, Cite{Id: id})
	}
	return out
}

// CiteIdsOf returns the ids a paragraph cites, in order.
func CiteIdsOf(cites []Cite) []string {
	out := make([]string, 0, len(cites))
	for _, c := range cites {
		out = append(out, c.Id)
	}
	return out
}

var sectionIdRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// SingleLine reports whether s has no line breaks or other control
// characters: the shape a heading, label or header value must have so it
// cannot smuggle markdown structure into the rendering.
func SingleLine(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// PlainText reports whether s has no control characters other than line
// breaks and tabs: the shape of prose, boilerplate and footers.
func PlainText(s string) bool {
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\r' && r != '\t') || r == 0x7f {
			return false
		}
	}
	return true
}

var sectionKinds = map[string]bool{
	SectionKindNarrative: true, SectionKindConclusion: true, SectionKindAlternatives: true,
	SectionKindClaims: true, SectionKindJudgments: true, SectionKindCalculations: true,
	SectionKindSources: true, SectionKindSignoff: true, SectionKindValidations: true,
	SectionKindAttestations: true, SectionKindBoilerplate: true,
}

var headerSources = map[string]bool{
	HeaderSourceTitle: true, HeaderSourceEntity: true, HeaderSourcePeriod: true, HeaderSourceSubject: true,
	HeaderSourceStatus: true, HeaderSourcePreparer: true, HeaderSourceApprovers: true, HeaderSourceLiteral: true,
}

// IsProseKind reports whether a section kind is filled by the narrative.
func IsProseKind(kind string) bool {
	return kind == SectionKindNarrative || kind == SectionKindConclusion
}

// IsTrailerKind reports whether a section kind is one the renderer always
// emits (the verifiability trailer), whether or not the format lists it.
func IsTrailerKind(kind string) bool {
	return kind == SectionKindValidations || kind == SectionKindSignoff || kind == SectionKindAttestations
}

// Validate checks the format's shape and limits.
func (f MemoFormat) Validate() error {
	if f.Version != 1 {
		return fmt.Errorf("%w: version must be 1", ErrInvalidMemoFormat)
	}
	if len(f.Sections) == 0 {
		return fmt.Errorf("%w: at least one section is required", ErrInvalidMemoFormat)
	}
	if len(f.Sections) > MaxMemoFormatSections {
		return fmt.Errorf("%w: at most %d sections are allowed", ErrInvalidMemoFormat, MaxMemoFormatSections)
	}
	if len(f.Header) > MaxMemoFormatHeader {
		return fmt.Errorf("%w: at most %d header fields are allowed", ErrInvalidMemoFormat, MaxMemoFormatHeader)
	}
	if len(f.Footer) > MaxMemoFormatFooter {
		return fmt.Errorf("%w: footer exceeds %d bytes", ErrInvalidMemoFormat, MaxMemoFormatFooter)
	}
	if !PlainText(f.Footer) || !SingleLine(f.Name) {
		return fmt.Errorf("%w: footer must be plain text and the name a single line", ErrInvalidMemoFormat)
	}
	for i, h := range f.Header {
		if strings.TrimSpace(h.Label) == "" {
			return fmt.Errorf("%w: header field %d has no label", ErrInvalidMemoFormat, i+1)
		}
		if !SingleLine(h.Label) || !SingleLine(h.Value) {
			return fmt.Errorf("%w: header field %q must be a single line", ErrInvalidMemoFormat, h.Label)
		}
		if !headerSources[h.Source] {
			return fmt.Errorf("%w: header field %q has unknown source %q", ErrInvalidMemoFormat, h.Label, h.Source)
		}
		if h.Source == HeaderSourceLiteral && strings.TrimSpace(h.Value) == "" {
			return fmt.Errorf("%w: header field %q is literal but has no value", ErrInvalidMemoFormat, h.Label)
		}
		if h.Source != HeaderSourceLiteral && h.Value != "" {
			return fmt.Errorf("%w: header field %q carries a value but its source is %q", ErrInvalidMemoFormat, h.Label, h.Source)
		}
	}
	seen := map[string]bool{}
	prose := false
	for i, s := range f.Sections {
		if !sectionIdRe.MatchString(s.Id) {
			return fmt.Errorf("%w: section %d id %q must match %s", ErrInvalidMemoFormat, i+1, s.Id, sectionIdRe)
		}
		if seen[s.Id] {
			return fmt.Errorf("%w: duplicate section id %q", ErrInvalidMemoFormat, s.Id)
		}
		seen[s.Id] = true
		if strings.TrimSpace(s.Heading) == "" {
			return fmt.Errorf("%w: section %q has no heading", ErrInvalidMemoFormat, s.Id)
		}
		if !SingleLine(s.Heading) {
			return fmt.Errorf("%w: section %q heading must be a single line", ErrInvalidMemoFormat, s.Id)
		}
		if !PlainText(s.Body) || !PlainText(s.Guidance) {
			return fmt.Errorf("%w: section %q body and guidance must be plain text", ErrInvalidMemoFormat, s.Id)
		}
		if s.Level < 1 || s.Level > MaxMemoFormatLevel {
			return fmt.Errorf("%w: section %q level must be 1..%d", ErrInvalidMemoFormat, s.Id, MaxMemoFormatLevel)
		}
		if !sectionKinds[s.Kind] {
			return fmt.Errorf("%w: section %q has unknown kind %q", ErrInvalidMemoFormat, s.Id, s.Kind)
		}
		if s.Kind == SectionKindBoilerplate && strings.TrimSpace(s.Body) == "" {
			return fmt.Errorf("%w: boilerplate section %q has no body", ErrInvalidMemoFormat, s.Id)
		}
		if s.Kind != SectionKindBoilerplate && s.Body != "" {
			return fmt.Errorf("%w: section %q of kind %s must not carry a body", ErrInvalidMemoFormat, s.Id, s.Kind)
		}
		if len(s.Body) > MaxMemoFormatBody {
			return fmt.Errorf("%w: section %q body exceeds %d bytes", ErrInvalidMemoFormat, s.Id, MaxMemoFormatBody)
		}
		if len(s.Guidance) > MaxMemoFormatGuidance {
			return fmt.Errorf("%w: section %q guidance exceeds %d bytes", ErrInvalidMemoFormat, s.Id, MaxMemoFormatGuidance)
		}
		if s.TemplateSection != "" && !sectionIdRe.MatchString(s.TemplateSection) {
			return fmt.Errorf("%w: section %q template section %q is not a valid id", ErrInvalidMemoFormat, s.Id, s.TemplateSection)
		}
		if IsProseKind(s.Kind) {
			prose = true
		}
	}
	if !prose {
		return fmt.Errorf("%w: at least one section of kind %s or %s is required", ErrInvalidMemoFormat, SectionKindNarrative, SectionKindConclusion)
	}
	return nil
}

// Section returns the section with the given id.
func (f MemoFormat) Section(id string) (MemoFormatSection, bool) {
	for _, s := range f.Sections {
		if s.Id == id {
			return s, true
		}
	}
	return MemoFormatSection{}, false
}

// HasKind reports whether the format lists a section of the given kind.
func (f MemoFormat) HasKind(kind string) bool {
	for _, s := range f.Sections {
		if s.Kind == kind {
			return true
		}
	}
	return false
}

// DefaultMemoFormat is the layout of a package whose organization never
// configured a format: exactly the rendering the builder produced before
// memo formats existed.
func DefaultMemoFormat() MemoFormat {
	return MemoFormat{
		Version: 1,
		Name:    "Bitwave default",
		Sections: []MemoFormatSection{
			{Id: "conclusion", Heading: "Conclusion", Level: 1, Kind: SectionKindConclusion},
			{Id: "claims", Heading: "Claims", Level: 1, Kind: SectionKindClaims},
			{Id: "judgments", Heading: "Judgments", Level: 1, Kind: SectionKindJudgments},
			{Id: "validations", Heading: "Validations", Level: 1, Kind: SectionKindValidations},
			{Id: "review-actions", Heading: "Review actions", Level: 1, Kind: SectionKindSignoff},
			{Id: "attestations", Heading: "Analysis attestations", Level: 1, Kind: SectionKindAttestations},
		},
	}
}

// CanonicalJSON renders a value the way the package stores it, so hashes of
// a format or a narrative are stable across writers.
func CanonicalJSON(v any) ([]byte, error) {
	return EncodeJSON(v)
}

// ValidateStructure checks a narrative section's shape and limits (roles,
// cite presence, lengths, depth). Citation resolution is the validators' job.
func (s NarrativeSection) ValidateStructure() error {
	return s.validate(0)
}

func (s NarrativeSection) validate(depth int) error {
	if !sectionIdRe.MatchString(s.Id) {
		return fmt.Errorf("%w: section id %q must match %s", ErrInvalidNarrative, s.Id, sectionIdRe)
	}
	if depth > MaxNarrativeDepth {
		return fmt.Errorf("%w: section %q nests deeper than %d", ErrInvalidNarrative, s.Id, MaxNarrativeDepth)
	}
	if depth > 0 && strings.TrimSpace(s.Heading) == "" {
		return fmt.Errorf("%w: subsection %q has no heading", ErrInvalidNarrative, s.Id)
	}
	if !SingleLine(s.Heading) {
		return fmt.Errorf("%w: section %q heading must be a single line", ErrInvalidNarrative, s.Id)
	}
	if len(s.Paragraphs) == 0 && len(s.Subsections) == 0 {
		return fmt.Errorf("%w: section %q is empty", ErrInvalidNarrative, s.Id)
	}
	for i, p := range s.Paragraphs {
		if strings.TrimSpace(p.Text) == "" {
			return fmt.Errorf("%w: section %q paragraph %d is empty", ErrInvalidNarrative, s.Id, i+1)
		}
		if len(p.Text) > MaxNarrativeParagraph {
			return fmt.Errorf("%w: section %q paragraph %d exceeds %d bytes", ErrInvalidNarrative, s.Id, i+1, MaxNarrativeParagraph)
		}
		if !PlainText(p.Text) {
			return fmt.Errorf("%w: section %q paragraph %d contains control characters", ErrInvalidNarrative, s.Id, i+1)
		}
		if p.Role != ParagraphRoleAnalysis && p.Role != ParagraphRoleConclusion {
			return fmt.Errorf("%w: section %q paragraph %d has unknown role %q", ErrInvalidNarrative, s.Id, i+1, p.Role)
		}
		if len(p.Cites) == 0 {
			return fmt.Errorf("%w: section %q paragraph %d cites nothing", ErrInvalidNarrative, s.Id, i+1)
		}
		for _, c := range p.Cites {
			if strings.TrimSpace(c.Id) == "" {
				return fmt.Errorf("%w: section %q paragraph %d has an empty cite", ErrInvalidNarrative, s.Id, i+1)
			}
		}
	}
	seen := map[string]bool{}
	for _, sub := range s.Subsections {
		if seen[sub.Id] {
			return fmt.Errorf("%w: duplicate subsection id %q under %q", ErrInvalidNarrative, sub.Id, s.Id)
		}
		seen[sub.Id] = true
		if err := sub.validate(depth + 1); err != nil {
			return err
		}
	}
	return nil
}

// Section returns the top-level narrative section with the given id.
func (n Narrative) Section(id string) (NarrativeSection, bool) {
	for _, s := range n.Sections {
		if s.Id == id {
			return s, true
		}
	}
	return NarrativeSection{}, false
}
