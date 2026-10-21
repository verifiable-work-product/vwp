package vwp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// verifyNarrative runs the narrative.* validators. They hold the memo prose
// to the structured parts it was written against: shape, the format it
// follows, citation resolution and coverage, role discipline, and whether
// what the prose cites (or the analysis as a whole) moved on since it was
// written. Packages without a narrative part emit nothing beyond the
// format's own section demands. g and digests are the package's integrity
// graph and recomputed roots, nil when they could not be built.
func verifyNarrative(pkg Source, g *Graph, digests *Digests, add func(ValidationResult), evidence, facts, claims, judgments []any) {
	if !pkg.Has(NarrativePath) {
		// No prose yet. A memo format snapshot that demands prose sections
		// still has to be honored: the memo is not complete without them.
		if pkg.Has(MemoFormatPath) {
			format := memoFormatInUse(pkg)
			verifyNarrativeSections(add, Narrative{FormatHash: format.hash}, format)
		}
		return
	}

	// narrative.schemaValid: the embedded schema plus the shape rules the
	// schema cannot express (single-line headings, plain text, depth).
	var narrative Narrative
	var value any
	raw, err := pkg.Read(NarrativePath)
	if err == nil {
		err = json.Unmarshal(raw, &value)
	}
	if err == nil {
		err = json.Unmarshal(raw, &narrative)
	}
	if err != nil {
		add(ValidationResult{
			ValidatorId: ValidatorNarrativeSchemaValid,
			Status:      StatusFail,
			Message:     fmt.Sprintf("%s is not a valid narrative: %v", NarrativePath, err),
		})
		return
	}
	errs, serr := schema.Validate(schema.Narrative, value)
	if serr != nil {
		errs = []string{serr.Error()}
	}
	errs = append(errs, narrativeShapeErrors(narrative)...)
	if len(errs) > 0 {
		add(ValidationResult{ValidatorId: ValidatorNarrativeSchemaValid, Status: StatusFail, Message: joinCapped(errs, 5)})
	} else {
		add(ValidationResult{ValidatorId: ValidatorNarrativeSchemaValid, Status: StatusPass})
	}

	// narrative.sections
	format := memoFormatInUse(pkg)
	verifyNarrativeSections(add, narrative, format)

	// narrative.citations
	kinds := citableKinds(pkg, evidence, facts, claims, judgments)
	verifyNarrativeCitations(add, narrative, kinds, claims)

	// narrative.roles
	verifyNarrativeRoles(add, narrative, format.format, kinds)

	// narrative.cites.fresh and narrative.drift need the graph; when it
	// could not be built core.graph.refsResolve already failed.
	notRun := func(id, why string) {
		add(ValidationResult{ValidatorId: id, Status: StatusNotRun, Message: why})
	}
	if g == nil {
		notRun(ValidatorNarrativeCitesFresh, "integrity graph could not be built")
		notRun(ValidatorNarrativeDrift, "integrity graph could not be built")
		return
	}
	fresh := verifyNarrativeCitesFresh(add, narrative, g)
	switch {
	case !fresh:
		notRun(ValidatorNarrativeDrift, "cites are stale (narrative.cites.fresh)")
	case narrative.AnalysisRoot == "":
		notRun(ValidatorNarrativeDrift, "narrative records no analysisRoot")
	case digests == nil:
		notRun(ValidatorNarrativeDrift, "roots cannot be computed (see core.graph.refsResolve and core.graph.acyclic)")
	case narrative.AnalysisRoot == digests.AnalysisRoot:
		add(ValidationResult{ValidatorId: ValidatorNarrativeDrift, Status: StatusPass})
	default:
		add(ValidationResult{
			ValidatorId: ValidatorNarrativeDrift,
			Status:      StatusReviewRequired,
			Message:     "the analysis changed since the narrative was written (analysis root moved): something new and uncited exists; review the narrative",
		})
	}
}

// verifyNarrativeCitesFresh compares every cite's recorded hash to the
// cited object's current closure hash. It returns true when nothing is
// stale (including when no cite carries a hash).
func verifyNarrativeCitesFresh(add func(ValidationResult), n Narrative, g *Graph) bool {
	stale := map[string][]string{} // object id -> sections citing it
	hashed := 0
	eachParagraph(n, func(sectionId string, p NarrativeParagraph) {
		for _, c := range p.Cites {
			if c.Hash == "" {
				continue
			}
			obj, ok := g.Objects[c.Id]
			if !ok || obj.Tainted {
				continue // narrative.citations or core.graph.refsResolve reports it
			}
			hashed++
			if obj.ClosureHash != c.Hash {
				if !containsString(stale[c.Id], sectionId) {
					stale[c.Id] = append(stale[c.Id], sectionId)
				}
			}
		}
	})
	switch {
	case hashed == 0:
		add(ValidationResult{ValidatorId: ValidatorNarrativeCitesFresh, Status: StatusNotRun, Message: "no cite hashes recorded"})
		return true
	case len(stale) == 0:
		add(ValidationResult{ValidatorId: ValidatorNarrativeCitesFresh, Status: StatusPass})
		return true
	}
	ids := make([]string, 0, len(stale))
	for id := range stale {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var items []string
	sections := map[string]bool{}
	for _, id := range ids {
		items = append(items, fmt.Sprintf("%s (cited by %s)", id, strings.Join(stale[id], ", ")))
		for _, s := range stale[id] {
			sections[s] = true
		}
	}
	refs := make([]string, 0, len(sections))
	for s := range sections {
		refs = append(refs, s)
	}
	sort.Strings(refs)
	add(ValidationResult{
		ValidatorId: ValidatorNarrativeCitesFresh,
		Status:      StatusFail,
		Message:     "cited objects changed after the narrative was written; rewrite the sections citing them: " + joinCapped(items, 5),
		Refs:        refs,
	})
	return false
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// narrativeShapeErrors checks the narrative's shape: section ids, paragraph
// roles, text limits and nesting depth. Cite presence is the citations
// validator's concern.
func narrativeShapeErrors(n Narrative) []string {
	var errs []string
	var walk func(s NarrativeSection, depth int)
	walk = func(s NarrativeSection, depth int) {
		if !sectionIdRe.MatchString(s.Id) {
			errs = append(errs, fmt.Sprintf("section id %q must match %s", s.Id, sectionIdRe))
		}
		if depth > MaxNarrativeDepth {
			errs = append(errs, fmt.Sprintf("section %q nests deeper than %d", s.Id, MaxNarrativeDepth))
			return
		}
		for i, p := range s.Paragraphs {
			if strings.TrimSpace(p.Text) == "" {
				errs = append(errs, fmt.Sprintf("section %q paragraph %d is empty", s.Id, i+1))
			}
			if len(p.Text) > MaxNarrativeParagraph {
				errs = append(errs, fmt.Sprintf("section %q paragraph %d exceeds %d bytes", s.Id, i+1, MaxNarrativeParagraph))
			}
			if p.Role != ParagraphRoleAnalysis && p.Role != ParagraphRoleConclusion {
				errs = append(errs, fmt.Sprintf("section %q paragraph %d has unknown role %q", s.Id, i+1, p.Role))
			}
			if !PlainText(p.Text) {
				errs = append(errs, fmt.Sprintf("section %q paragraph %d contains control characters", s.Id, i+1))
			}
		}
		if !SingleLine(s.Heading) {
			errs = append(errs, fmt.Sprintf("section %q heading must be a single line", s.Id))
		}
		for _, sub := range s.Subsections {
			walk(sub, depth+1)
		}
	}
	seen := map[string]bool{}
	for _, s := range n.Sections {
		if seen[s.Id] {
			errs = append(errs, fmt.Sprintf("duplicate section id %q", s.Id))
		}
		seen[s.Id] = true
		walk(s, 0)
	}
	return errs
}

// formatInUse is the memo format a package renders with: the snapshot when
// present (configured), else the default. hash is the self hash the
// narrative's formatHash must equal; problems lists why the snapshot was
// rejected (it then falls back to the default).
type formatInUse struct {
	format     MemoFormat
	hash       string
	configured bool
	problems   []string
}

// memoFormatInUse resolves the format a package renders with. A snapshot
// that does not parse, fails the memo-format schema or fails Validate is
// reported through problems and the default is returned in its place.
func memoFormatInUse(pkg Source) formatInUse {
	fallback := func(problems ...string) formatInUse {
		f := DefaultMemoFormat()
		hash, err := SelfHash(KindMemoFormat, f)
		if err != nil {
			problems = append(problems, err.Error())
		}
		return formatInUse{format: f, hash: hash, problems: problems}
	}
	if !pkg.Has(MemoFormatPath) {
		return fallback()
	}
	raw, err := pkg.Read(MemoFormatPath)
	if err != nil {
		return fallback(fmt.Sprintf("failed to read %s: %v", MemoFormatPath, err))
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fallback(fmt.Sprintf("%s is not a valid memo format: %v", MemoFormatPath, err))
	}
	errs, serr := schema.Validate(schema.MemoFormat, value)
	if serr != nil {
		errs = []string{serr.Error()}
	}
	if len(errs) > 0 {
		for i := range errs {
			errs[i] = MemoFormatPath + ": " + errs[i]
		}
		return fallback(errs...)
	}
	var f MemoFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return fallback(fmt.Sprintf("%s is not a valid memo format: %v", MemoFormatPath, err))
	}
	if err := f.Validate(); err != nil {
		return fallback(fmt.Sprintf("%s: %v", MemoFormatPath, err))
	}
	hash, err := SelfHash(KindMemoFormat, json.RawMessage(raw))
	if err != nil {
		return fallback(fmt.Sprintf("%s: %v", MemoFormatPath, err))
	}
	return formatInUse{format: f, hash: hash, configured: true}
}

func verifyNarrativeSections(add func(ValidationResult), n Narrative, format formatInUse) {
	var problems, refs []string
	if len(format.problems) > 0 {
		problems = append(problems, format.problems...)
	} else if n.FormatHash != format.hash {
		problems = append(problems, "narrative was written against a different memo format (formatHash mismatch)")
	}
	for _, s := range n.Sections {
		fs, ok := format.format.Section(s.Id)
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("section %q is not a section of the memo format", s.Id))
			refs = append(refs, s.Id)
		case !IsProseKind(fs.Kind):
			problems = append(problems, fmt.Sprintf("section %q is of kind %s, not prose", s.Id, fs.Kind))
			refs = append(refs, s.Id)
		}
	}
	for _, fs := range format.format.Sections {
		if !fs.Required || !IsProseKind(fs.Kind) {
			continue
		}
		s, ok := n.Section(fs.Id)
		if !ok || countParagraphs(s) == 0 {
			problems = append(problems, fmt.Sprintf("required section %q has no paragraphs", fs.Id))
			refs = append(refs, fs.Id)
		}
	}
	if len(problems) == 0 {
		add(ValidationResult{ValidatorId: ValidatorNarrativeSections, Status: StatusPass})
		return
	}
	add(ValidationResult{ValidatorId: ValidatorNarrativeSections, Status: StatusFail, Message: joinCapped(problems, 5), Refs: refs})
}

func countParagraphs(s NarrativeSection) int {
	n := len(s.Paragraphs)
	for _, sub := range s.Subsections {
		n += countParagraphs(sub)
	}
	return n
}

// Kinds of objects a narrative paragraph may cite.
const (
	citeKindFact        = KindFact
	citeKindClaim       = KindClaim
	citeKindJudgment    = KindJudgment
	citeKindCalculation = KindCalculation
	citeKindEvidence    = KindEvidence
)

// citableKinds maps every citable id in the package to its kind.
func citableKinds(pkg Source, evidence, facts, claims, judgments []any) map[string]string {
	kinds := map[string]string{}
	for id := range idSet(evidence) {
		kinds[id] = citeKindEvidence
	}
	for id := range idSet(facts) {
		kinds[id] = citeKindFact
	}
	for id := range idSet(claims) {
		kinds[id] = citeKindClaim
	}
	for id := range idSet(judgments) {
		kinds[id] = citeKindJudgment
	}
	for id := range calculationIds(pkg) {
		kinds[id] = citeKindCalculation
	}
	return kinds
}

// eachParagraph visits every paragraph of the narrative with the id of the
// top-level section it belongs to.
func eachParagraph(n Narrative, visit func(sectionId string, p NarrativeParagraph)) {
	var walk func(top string, s NarrativeSection)
	walk = func(top string, s NarrativeSection) {
		for _, p := range s.Paragraphs {
			visit(top, p)
		}
		for _, sub := range s.Subsections {
			walk(top, sub)
		}
	}
	for _, s := range n.Sections {
		walk(s.Id, s)
	}
}

func verifyNarrativeCitations(add func(ValidationResult), n Narrative, kinds map[string]string, claims []any) {
	var problems, refs []string
	cited := map[string]bool{}
	eachParagraph(n, func(sectionId string, p NarrativeParagraph) {
		if len(p.Cites) == 0 {
			problems = append(problems, fmt.Sprintf("section %q: a paragraph cites nothing", sectionId))
			refs = append(refs, sectionId)
		}
		for _, c := range p.Cites {
			if _, ok := kinds[c.Id]; !ok {
				problems = append(problems, fmt.Sprintf("section %q: unknown cite %s", sectionId, c.Id))
				refs = append(refs, c.Id)
				continue
			}
			cited[c.Id] = true
		}
	})
	if len(problems) > 0 {
		add(ValidationResult{ValidatorId: ValidatorNarrativeCitations, Status: StatusFail, Message: joinCapped(problems, 5), Refs: refs})
		return
	}
	var uncited []string
	for _, item := range claims {
		if id := getString(asMap(item), "id"); id != "" && !cited[id] {
			uncited = append(uncited, id)
		}
	}
	if len(uncited) == 0 {
		add(ValidationResult{ValidatorId: ValidatorNarrativeCitations, Status: StatusPass})
		return
	}
	add(ValidationResult{
		ValidatorId: ValidatorNarrativeCitations,
		Status:      StatusReviewRequired,
		Message:     "claims not cited by the narrative: " + strings.Join(uncited, ", "),
		Refs:        uncited,
	})
}

func verifyNarrativeRoles(add func(ValidationResult), n Narrative, format MemoFormat, kinds map[string]string) {
	var failures, reviews, refs []string
	eachParagraph(n, func(sectionId string, p NarrativeParagraph) {
		if p.Role != ParagraphRoleConclusion {
			return
		}
		for _, c := range p.Cites {
			if k := kinds[c.Id]; k == citeKindClaim || k == citeKindJudgment {
				return
			}
		}
		failures = append(failures, fmt.Sprintf("section %q: a conclusion paragraph cites no claim or judgment", sectionId))
		refs = append(refs, sectionId)
	})
	for _, s := range n.Sections {
		fs, ok := format.Section(s.Id)
		if !ok || fs.TemplateSection == "" {
			continue
		}
		prefix := fs.TemplateSection + "_"
		found := false
		eachParagraph(Narrative{Sections: []NarrativeSection{s}}, func(_ string, p NarrativeParagraph) {
			for _, c := range p.Cites {
				if strings.HasPrefix(c.Id, prefix) {
					found = true
				}
			}
		})
		if !found {
			reviews = append(reviews, fmt.Sprintf("section %q covers template section %s but cites nothing with id prefix %s", s.Id, fs.TemplateSection, prefix))
			refs = append(refs, s.Id)
		}
	}
	sort.Strings(refs)
	switch {
	case len(failures) > 0:
		add(ValidationResult{ValidatorId: ValidatorNarrativeRoles, Status: StatusFail, Message: joinCapped(append(failures, reviews...), 5), Refs: refs})
	case len(reviews) > 0:
		add(ValidationResult{ValidatorId: ValidatorNarrativeRoles, Status: StatusReviewRequired, Message: joinCapped(reviews, 5), Refs: refs})
	default:
		add(ValidationResult{ValidatorId: ValidatorNarrativeRoles, Status: StatusPass})
	}
}
