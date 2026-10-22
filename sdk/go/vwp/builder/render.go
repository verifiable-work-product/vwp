package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
)

// RenderPath is where Render writes the package's markdown rendering.
const RenderPath = "renderings/memo.md"

// maxNotesLine caps the one-line summary of a cited object in the Notes
// section, in runes.
const maxNotesLine = 160

// maxOutputsJSON caps the compact JSON shown for a calculation's outputs
// when they are not a row table, in bytes.
const maxOutputsJSON = 2 << 10

// Render writes a deterministic markdown rendering of the package to
// renderings/memo.md, laid out by the memo format snapshot when the package
// carries one and by vwp.DefaultMemoFormat() otherwise. Renderings are
// derived views and are regenerated (overwritten) on every call; the
// structured package remains the source of truth.
func (b *Builder) Render() error {
	f, _, err := b.MemoFormat()
	if err != nil {
		return fmt.Errorf("vwp/builder: render: %w", err)
	}
	return b.RenderWith(f)
}

// RenderWith renders the package with the given memo format: the package
// header, the format's header fields, its sections in order (prose from the
// narrative, structural kinds from the package parts), the verifiability
// trailer (validations, review actions, attestations; always present), the
// Notes section resolving the cite markers, and the footer.
func (b *Builder) RenderWith(f vwp.MemoFormat) error {
	if err := f.Validate(); err != nil {
		return fmt.Errorf("vwp/builder: render: %w", err)
	}
	var wp vwp.Workproduct
	if err := readJSONFile(b.path(vwp.WorkproductPath), &wp); err != nil {
		return fmt.Errorf("vwp/builder: render: %w", err)
	}
	narrative, _, err := b.Narrative()
	if err != nil {
		return fmt.Errorf("vwp/builder: render: %w", err)
	}

	r := &memoRenderer{b: b, wp: wp, narrative: narrative, markers: map[string]int{}}
	sb := &r.sb

	title := wp.Title
	if title == "" {
		title = b.manifest.PackageId
	}
	fmt.Fprintf(sb, "# %s\n\n", title)
	fmt.Fprintf(sb, "- Package: `%s`\n", b.manifest.PackageId)
	fmt.Fprintf(sb, "- Profile: `%s`\n", b.manifest.Profile)
	if b.manifest.Extension != "" {
		fmt.Fprintf(sb, "- Extension: `%s`\n", b.manifest.Extension)
	}
	if wp.Status != "" {
		fmt.Fprintf(sb, "- Status: %s\n", wp.Status)
	}
	sb.WriteString("\n> Derived rendering. The structured package is the source of truth.\n")

	r.renderHeaderFields(f.Header)

	present := map[string]bool{}
	for _, s := range f.Sections {
		present[s.Kind] = true
		r.renderSection(s)
	}
	for _, s := range vwp.DefaultMemoFormat().Sections {
		if vwp.IsTrailerKind(s.Kind) && !present[s.Kind] {
			r.renderSection(s)
		}
	}
	r.renderNotes()
	if f.Footer != "" {
		fmt.Fprintf(sb, "\n%s\n", safeProse(f.Footer))
	}

	content := []byte(sb.String())
	// Renderings are derived: overwrite rather than conflict on change.
	if err := b.writePart(RenderPath, content); err != nil {
		return err
	}
	b.upsertManifestPart(RenderPath, "text/markdown", RoleRendering, vwp.SHA256Hex(content))
	return b.saveManifest()
}

// memoRenderer carries the state of one rendering: the cite markers handed
// out so far, numbered globally in order of first citation.
type memoRenderer struct {
	b         *Builder
	wp        vwp.Workproduct
	narrative *vwp.Narrative
	sb        strings.Builder
	markers   map[string]int
	order     []string
}

func headingLine(level int, text string) string {
	if level < 1 {
		level = 1
	}
	return strings.Repeat("#", level+1) + " " + text
}

// safeHeading keeps customer or model heading text on one line and unable
// to open HTML.
func safeHeading(text string) string {
	return strings.ReplaceAll(oneLine(text), "<", "&lt;")
}

// safeProse neutralizes markdown structure and HTML in customer or model
// text so it can never forge or hide a section of the memo: a line that
// would start a heading is escaped, and "<" is escaped so no tag or
// comment can open (an unclosed comment would swallow the rest of the
// rendering in a viewer).
func safeProse(text string) string {
	text = strings.ReplaceAll(strings.TrimRight(text, "\n"), "<", "&lt;")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
			lines[i] = "\\" + strings.TrimLeft(l, " \t")
		}
	}
	return strings.Join(lines, "\n")
}

// renderHeaderFields writes every header field in format order. A field
// whose source yields nothing keeps its slot with an empty value, so the
// reader sees what the organization's header asks for and that it is
// unfilled.
func (r *memoRenderer) renderHeaderFields(fields []vwp.HeaderField) {
	if len(fields) == 0 {
		return
	}
	r.sb.WriteString("\n")
	for _, h := range fields {
		value := strings.TrimSpace(r.headerValue(h))
		fmt.Fprintf(&r.sb, "- **%s:** %s\n", safeHeading(h.Label), safeHeading(value))
	}
}

// calendarDate returns the YYYY-MM-DD part of an RFC 3339 timestamp (the
// date in the timestamp's own offset, as written); a value not shaped like
// one is returned as is, so nothing is lost.
func calendarDate(ts string) string {
	ts = strings.TrimSpace(ts)
	if len(ts) >= 10 && isDigits(ts[0:4]) && ts[4] == '-' && isDigits(ts[5:7]) && ts[7] == '-' && isDigits(ts[8:10]) {
		return ts[:10]
	}
	return ts
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// standardRefsHeader is the sorted (byte order), deduplicated union of the
// claims' standardRefs joined by "; ".
func (b *Builder) standardRefsHeader() string {
	doc, err := b.readCollectionDoc(vwp.ClaimsPath)
	if err != nil {
		return ""
	}
	seen := map[string]bool{}
	var refs []string
	for _, item := range getSliceAny(doc, "claims") {
		claim, _ := item.(map[string]any)
		for _, ref := range getSliceAny(claim, "standardRefs") {
			s, _ := ref.(string)
			s = strings.TrimSpace(s)
			if s != "" && !seen[s] {
				seen[s] = true
				refs = append(refs, s)
			}
		}
	}
	sort.Strings(refs)
	return strings.Join(refs, "; ")
}

func (r *memoRenderer) headerValue(h vwp.HeaderField) string {
	switch h.Source {
	case vwp.HeaderSourceTitle:
		if r.wp.Title != "" {
			return r.wp.Title
		}
		return r.b.manifest.Title
	case vwp.HeaderSourceEntity:
		if name, _ := r.wp.Entity["name"].(string); name != "" {
			return name
		}
		if len(r.wp.Entity) == 0 {
			return ""
		}
		return compactJSON(r.wp.Entity, maxNotesLine)
	case vwp.HeaderSourcePeriod:
		start, _ := r.wp.Period["start"].(string)
		end, _ := r.wp.Period["end"].(string)
		switch {
		case start != "" && end != "":
			return start + " to " + end
		case start != "":
			return "from " + start
		case end != "":
			return "to " + end
		}
		if len(r.wp.Period) == 0 {
			return ""
		}
		return compactJSON(r.wp.Period, maxNotesLine)
	case vwp.HeaderSourceSubject:
		id, _ := r.wp.Subject["id"].(string)
		typ, _ := r.wp.Subject["type"].(string)
		switch {
		case id != "" && typ != "":
			return typ + " " + id
		case id != "":
			return id
		}
		if len(r.wp.Subject) == 0 {
			return ""
		}
		return compactJSON(r.wp.Subject, maxNotesLine)
	case vwp.HeaderSourceStatus:
		return r.wp.Status
	case vwp.HeaderSourcePreparer:
		var p vwp.Provenance
		if err := readJSONFile(r.b.path(vwp.ProvenancePath), &p); err == nil && p.CreatedBy.Name != "" {
			return p.CreatedBy.Name
		}
		name, _ := r.wp.CreatedBy["name"].(string)
		return name
	case vwp.HeaderSourceApprovers:
		doc, err := r.b.readCollectionDoc(vwp.ApprovalsPath)
		if err != nil {
			return ""
		}
		var names []string
		seen := map[string]bool{}
		for _, item := range getSliceAny(doc, "approvals") {
			a, _ := item.(map[string]any)
			approver, _ := a["approver"].(map[string]any)
			name, _ := approver["name"].(string)
			if name == "" {
				name, _ = approver["id"].(string)
			}
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		return strings.Join(names, ", ")
	case vwp.HeaderSourceDate:
		return calendarDate(r.b.manifest.CreatedAt)
	case vwp.HeaderSourceStandardRefs:
		return r.b.standardRefsHeader()
	case vwp.HeaderSourceLiteral:
		return h.Value
	}
	return ""
}

func (r *memoRenderer) renderSection(s vwp.MemoFormatSection) {
	sb := &r.sb
	switch s.Kind {
	case vwp.SectionKindNarrative, vwp.SectionKindConclusion:
		if r.narrative != nil {
			if ns, ok := r.narrative.Section(s.Id); ok {
				fmt.Fprintf(sb, "\n%s\n", headingLine(s.Level, safeHeading(s.Heading)))
				r.renderProse(ns, s.Level)
				return
			}
		}
		if s.Kind == vwp.SectionKindConclusion && r.wp.ConclusionSummary != "" {
			fmt.Fprintf(sb, "\n%s\n\n%s\n", headingLine(s.Level, safeHeading(s.Heading)), safeProse(r.wp.ConclusionSummary))
		}
	case vwp.SectionKindClaims:
		renderClaims(sb, r.b, s.Level, safeHeading(s.Heading))
	case vwp.SectionKindJudgments:
		renderJudgments(sb, r.b, s.Level, safeHeading(s.Heading))
	case vwp.SectionKindValidations:
		renderValidations(sb, r.b, s.Level, safeHeading(s.Heading))
	case vwp.SectionKindSignoff:
		renderApprovals(sb, r.b, s.Level, safeHeading(s.Heading), s.Roles)
	case vwp.SectionKindAttestations:
		renderAttestations(sb, r.b, s.Level, safeHeading(s.Heading))
	case vwp.SectionKindAlternatives:
		renderAlternatives(sb, r.b, s.Level, safeHeading(s.Heading))
	case vwp.SectionKindCalculations:
		renderCalculations(sb, r.b, s.Level, safeHeading(s.Heading))
	case vwp.SectionKindSources:
		renderSources(sb, r.b, s.Level, safeHeading(s.Heading))
	case vwp.SectionKindBoilerplate:
		fmt.Fprintf(sb, "\n%s\n\n%s\n", headingLine(s.Level, safeHeading(s.Heading)), safeProse(s.Body))
	}
}

// renderProse writes a narrative section's paragraphs, each followed by its
// cite markers, then its subsections one heading level deeper. A section
// that does not apply renders its reason as a plain sentence instead.
// Conclusion paragraphs are blockquotes with a bold marker; literature
// paragraphs are plain blockquotes (the authoritative text, set apart).
func (r *memoRenderer) renderProse(ns vwp.NarrativeSection, level int) {
	sb := &r.sb
	if ns.NotApplicable != "" {
		fmt.Fprintf(sb, "\nNot applicable: %s\n", safeProse(oneLine(ns.NotApplicable)))
	}
	for _, p := range ns.Paragraphs {
		text := safeProse(strings.TrimSpace(p.Text))
		switch p.Role {
		case vwp.ParagraphRoleConclusion:
			text = "> **Conclusion.** " + strings.ReplaceAll(text, "\n", "\n> ")
		case vwp.ParagraphRoleLiterature:
			text = "> " + strings.ReplaceAll(text, "\n", "\n> ")
		}
		fmt.Fprintf(sb, "\n%s%s\n", text, r.markersFor(p.Cites))
	}
	for _, sub := range ns.Subsections {
		fmt.Fprintf(sb, "\n%s\n", headingLine(level+1, safeHeading(sub.Heading)))
		r.renderProse(sub, level+1)
	}
}

// markersFor returns " [n] [m]" for the cites, numbering each distinct id
// once across the memo in order of first citation.
func (r *memoRenderer) markersFor(cites []vwp.Cite) string {
	var out strings.Builder
	for _, c := range cites {
		id := strings.TrimSpace(c.Id)
		if id == "" {
			continue
		}
		n, ok := r.markers[id]
		if !ok {
			n = len(r.order) + 1
			r.markers[id] = n
			r.order = append(r.order, id)
		}
		fmt.Fprintf(&out, " [%d]", n)
	}
	return out.String()
}

// renderNotes resolves every cite marker to one line describing the object.
func (r *memoRenderer) renderNotes() {
	if len(r.order) == 0 {
		return
	}
	index := r.b.citedObjectIndex()
	r.sb.WriteString("\n## Notes\n\n")
	for i, id := range r.order {
		obj, ok := index[id]
		if !ok {
			fmt.Fprintf(&r.sb, "[%d] unresolved %s\n", i+1, id)
			continue
		}
		fmt.Fprintf(&r.sb, "[%d] %s %s: %s\n", i+1, obj.kind, id, truncateRunes(obj.line, maxNotesLine))
	}
}

// citedObject is what a cite marker resolves to in the Notes section.
type citedObject struct {
	kind string
	line string
}

// citedObjectIndex maps every citable id (facts, claims, judgments,
// calculations, evidence) to its kind and one-line summary.
func (b *Builder) citedObjectIndex() map[string]citedObject {
	index := map[string]citedObject{}
	collect := func(relPath, key, kind string, line func(map[string]any) string) {
		doc, err := b.readCollectionDoc(relPath)
		if err != nil {
			return
		}
		for _, item := range getSliceAny(doc, key) {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := m["id"].(string); id != "" {
				index[id] = citedObject{kind: kind, line: oneLine(line(m))}
			}
		}
	}
	collect(vwp.FactsPath, "facts", "fact", func(m map[string]any) string {
		name, _ := m["name"].(string)
		if value, ok := m["value"]; ok && name != "" {
			return name + " = " + compactJSON(value, maxNotesLine)
		}
		statement, _ := m["statement"].(string)
		return statement
	})
	collect(vwp.ClaimsPath, "claims", "claim", func(m map[string]any) string {
		statement, _ := m["statement"].(string)
		return statement
	})
	collect(vwp.JudgmentsPath, "judgments", "judgment", func(m map[string]any) string {
		topic, _ := m["topic"].(string)
		return topic
	})
	collect(vwp.EvidenceIndex, "evidence", "evidence", func(m map[string]any) string {
		title, _ := m["title"].(string)
		return title
	})
	for _, calc := range b.calculationDocs() {
		if id, _ := calc.doc["id"].(string); id != "" {
			typ, _ := calc.doc["type"].(string)
			index[id] = citedObject{kind: "calculation", line: oneLine(typ)}
		}
	}
	return index
}

// calculationDoc is one JSON calculation part with its raw bytes kept so
// that row tables can follow the keys' first-seen order.
type calculationDoc struct {
	doc map[string]any
	raw []byte
}

// calculationDocs returns every JSON calculation object part, in manifest
// order. Calculation parts that are artifacts rather than calculation
// objects (no id and no outputs, such as the accounting schedule) are
// skipped; an object without a string id is named after its file.
func (b *Builder) calculationDocs() []calculationDoc {
	var docs []calculationDoc
	for _, part := range b.manifest.Parts {
		if part.Role != RoleCalculation || !strings.HasSuffix(strings.ToLower(part.Path), ".json") {
			continue
		}
		raw, err := os.ReadFile(b.path(part.Path))
		if err != nil {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
			continue
		}
		_, hasId := doc["id"].(string)
		_, hasOutputs := doc["outputs"]
		if !hasId && !hasOutputs {
			continue
		}
		if !hasId {
			doc["id"] = strings.TrimSuffix(path.Base(part.Path), path.Ext(part.Path))
		}
		docs = append(docs, calculationDoc{doc: doc, raw: raw})
	}
	return docs
}

// rowKeysFirstSeen returns the union of the keys of outputs.rows objects in
// the order they first appear in the part's JSON text.
func rowKeysFirstSeen(raw []byte) []string {
	var part struct {
		Outputs struct {
			Rows []json.RawMessage `json:"rows"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(raw, &part); err != nil {
		return nil
	}
	var keys []string
	seen := map[string]bool{}
	for _, row := range part.Outputs.Rows {
		dec := json.NewDecoder(bytes.NewReader(row))
		if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
			continue
		}
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			key, ok := tok.(string)
			if !ok {
				break
			}
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				break
			}
		}
	}
	return keys
}

func renderClaims(sb *strings.Builder, b *Builder, level int, heading string) {
	doc, err := b.readCollectionDoc(vwp.ClaimsPath)
	if err != nil {
		return
	}
	items, _ := doc["claims"].([]any)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n", headingLine(level, heading))
	for _, item := range items {
		claim, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(sb, "\n%s\n\n%v\n", headingLine(level+1, fmt.Sprint(claim["id"])), claim["statement"])
		if conclusion, ok := claim["conclusion"].(string); ok && conclusion != "" {
			fmt.Fprintf(sb, "\n- Conclusion: %s\n", conclusion)
		}
		if status, ok := claim["status"].(string); ok && status != "" {
			fmt.Fprintf(sb, "- Status: %s\n", status)
		}
	}
}

func renderJudgments(sb *strings.Builder, b *Builder, level int, heading string) {
	doc, err := b.readCollectionDoc(vwp.JudgmentsPath)
	if err != nil {
		return
	}
	items, _ := doc["judgments"].([]any)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n", headingLine(level, heading))
	for _, item := range items {
		j, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(sb, "\n%s\n\n%v\n", headingLine(level+1, fmt.Sprintf("%v — %v", j["id"], j["topic"])), j["question"])
		if pc, ok := j["proposedConclusion"].(string); ok && pc != "" {
			fmt.Fprintf(sb, "\n- Proposed conclusion: %s\n", pc)
		}
		if status, ok := j["status"].(string); ok && status != "" {
			fmt.Fprintf(sb, "- Status: %s\n", status)
		}
	}
}

// renderAlternatives lists, per judgment that recorded alternatives, the
// conclusions that were considered and not taken. Alternatives are strings
// or objects (conclusion or description, with notes or rationale).
func renderAlternatives(sb *strings.Builder, b *Builder, level int, heading string) {
	doc, err := b.readCollectionDoc(vwp.JudgmentsPath)
	if err != nil {
		return
	}
	var blocks []string
	for _, item := range getSliceAny(doc, "judgments") {
		j, ok := item.(map[string]any)
		if !ok {
			continue
		}
		alts := getSliceAny(j, "alternatives")
		if len(alts) == 0 {
			continue
		}
		var block strings.Builder
		fmt.Fprintf(&block, "\n%s\n\n", headingLine(level+1, fmt.Sprintf("%v - %v", j["id"], j["topic"])))
		for _, alt := range alts {
			switch a := alt.(type) {
			case string:
				fmt.Fprintf(&block, "- %s\n", oneLine(a))
			case map[string]any:
				text, _ := a["conclusion"].(string)
				if text == "" {
					text, _ = a["description"].(string)
				}
				notes, _ := a["notes"].(string)
				if notes == "" {
					notes, _ = a["rationale"].(string)
				}
				switch {
				case text != "" && notes != "":
					fmt.Fprintf(&block, "- %s: %s\n", oneLine(text), oneLine(notes))
				case text != "":
					fmt.Fprintf(&block, "- %s\n", oneLine(text))
				case notes != "":
					fmt.Fprintf(&block, "- %s\n", oneLine(notes))
				default:
					fmt.Fprintf(&block, "- %s\n", compactJSON(a, maxNotesLine))
				}
			default:
				fmt.Fprintf(&block, "- %s\n", compactJSON(alt, maxNotesLine))
			}
		}
		blocks = append(blocks, block.String())
	}
	if len(blocks) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n", headingLine(level, heading))
	for _, block := range blocks {
		sb.WriteString(block)
	}
}

// renderCalculations writes every JSON calculation part: a table when its
// outputs carry a rows array of objects, else the compact outputs JSON.
func renderCalculations(sb *strings.Builder, b *Builder, level int, heading string) {
	docs := b.calculationDocs()
	if len(docs) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n", headingLine(level, heading))
	for _, calc := range docs {
		id, _ := calc.doc["id"].(string)
		typ, _ := calc.doc["type"].(string)
		title := id
		if typ != "" {
			title = fmt.Sprintf("%s (%s)", id, typ)
		}
		fmt.Fprintf(sb, "\n%s\n\n", headingLine(level+1, title))
		outputs, _ := calc.doc["outputs"].(map[string]any)
		rows, ok := rowObjects(getSliceAny(outputs, "rows"))
		keys := rowKeysFirstSeen(calc.raw)
		if !ok || len(keys) == 0 {
			fmt.Fprintf(sb, "- outputs: %s\n", compactJSON(calc.doc["outputs"], maxOutputsJSON))
			continue
		}
		fmt.Fprintf(sb, "| %s |\n", strings.Join(keys, " | "))
		fmt.Fprintf(sb, "|%s\n", strings.Repeat("---|", len(keys)))
		for _, row := range rows {
			cells := make([]string, len(keys))
			for i, k := range keys {
				if v, ok := row[k]; ok {
					cells[i] = strings.ReplaceAll(cellText(v), "|", "\\|")
				}
			}
			fmt.Fprintf(sb, "| %s |\n", strings.Join(cells, " | "))
		}
	}
}

// rowObjects reports whether rows is a non-empty array of objects.
func rowObjects(rows []any) ([]map[string]any, bool) {
	if len(rows) == 0 {
		return nil, false
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			return nil, false
		}
		out = append(out, m)
	}
	return out, true
}

func cellText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return oneLine(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return fmt.Sprint(t)
	default:
		return compactJSON(t, maxNotesLine)
	}
}

// renderSources writes the numbered evidence list.
func renderSources(sb *strings.Builder, b *Builder, level int, heading string) {
	doc, err := b.readCollectionDoc(vwp.EvidenceIndex)
	if err != nil {
		return
	}
	items := getSliceAny(doc, "evidence")
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n\n", headingLine(level, heading))
	n := 0
	for _, item := range items {
		ev, ok := item.(map[string]any)
		if !ok {
			continue
		}
		n++
		title, _ := ev["title"].(string)
		if title == "" {
			title, _ = ev["id"].(string)
		}
		fmt.Fprintf(sb, "%d. %s", n, oneLine(title))
		if typ, _ := ev["type"].(string); typ != "" {
			fmt.Fprintf(sb, " (%s)", typ)
		}
		sha, _ := ev["sha256"].(string)
		if evPath, _ := ev["path"].(string); sha == "" && evPath != "" {
			if part, ok := b.manifest.FindPart(evPath); ok {
				sha = part.SHA256
			}
		}
		if sha != "" {
			fmt.Fprintf(sb, " - sha256 %s", sha)
		}
		sb.WriteString("\n")
	}
}

func renderValidations(sb *strings.Builder, b *Builder, level int, heading string) {
	doc, err := b.readCollectionDoc(vwp.ValidationsPath)
	if err != nil {
		return
	}
	items, _ := doc["validations"].([]any)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n\n", headingLine(level, heading))
	for _, item := range items {
		v, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(sb, "- `%v` — %v", v["validator"], v["status"])
		if msg, ok := v["message"].(string); ok && msg != "" {
			fmt.Fprintf(sb, " — %s", msg)
		}
		sb.WriteString("\n")
	}
}

// renderApprovals writes the review actions. With roles (the format's
// sign-off table) it writes one table row per role in order, filled from
// the latest approved action in that role (case-insensitive, trimmed;
// latest = last in approvals order); approvals that fill no row follow the
// table as a list under "Other review actions". Without roles every
// approval is listed.
func renderApprovals(sb *strings.Builder, b *Builder, level int, heading string, roles []string) {
	doc, err := b.readCollectionDoc(vwp.ApprovalsPath)
	if err != nil {
		return
	}
	var items []map[string]any
	for _, item := range getSliceAny(doc, "approvals") {
		if a, ok := item.(map[string]any); ok {
			items = append(items, a)
		}
	}
	if len(roles) == 0 {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(sb, "\n%s\n\n", headingLine(level, heading))
		renderApprovalList(sb, items)
		return
	}

	// One row per role: the index of the approval filling it, or -1.
	filled := make([]int, len(roles))
	for i := range filled {
		filled[i] = -1
	}
	used := make([]bool, len(items))
	for i, role := range roles {
		want := strings.ToLower(strings.TrimSpace(role))
		for j, a := range items {
			got, _ := a["role"].(string)
			if action, _ := a["action"].(string); action == vwp.ApprovalActionApproved && strings.ToLower(strings.TrimSpace(got)) == want {
				filled[i] = j
			}
		}
		if filled[i] >= 0 {
			used[filled[i]] = true
		}
	}
	fmt.Fprintf(sb, "\n%s\n\n", headingLine(level, heading))
	sb.WriteString("| Role | Name | Date |\n|---|---|---|\n")
	for i, role := range roles {
		name, date := "", ""
		if j := filled[i]; j >= 0 {
			name = approverName(items[j])
			ts, _ := items[j]["timestamp"].(string)
			date = calendarDate(ts)
		}
		fmt.Fprintf(sb, "| %s | %s | %s |\n", tableCell(role), tableCell(name), tableCell(date))
	}
	var rest []map[string]any
	for j, a := range items {
		if !used[j] {
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n\n", headingLine(level+1, "Other review actions"))
	renderApprovalList(sb, rest)
}

// tableCell makes untrusted text safe inside a markdown table cell.
func tableCell(s string) string {
	return strings.ReplaceAll(safeHeading(s), "|", "\\|")
}

func approverName(a map[string]any) string {
	approver, _ := a["approver"].(map[string]any)
	name, _ := approver["name"].(string)
	if name == "" {
		name, _ = approver["id"].(string)
	}
	return name
}

func renderApprovalList(sb *strings.Builder, items []map[string]any) {
	for _, a := range items {
		fmt.Fprintf(sb, "- %v by %s", a["action"], approverName(a))
		if role, ok := a["role"].(string); ok && role != "" {
			fmt.Fprintf(sb, " (%s)", role)
		}
		if ts, ok := a["timestamp"].(string); ok && ts != "" {
			fmt.Fprintf(sb, " at %s", ts)
		}
		if scope, ok := a["scope"].(string); ok && scope != "" {
			fmt.Fprintf(sb, " — scope: %s", scope)
		}
		if comments, ok := a["comments"].(string); ok && comments != "" {
			fmt.Fprintf(sb, " — %s", comments)
		}
		sb.WriteString("\n")
	}
}

// renderAttestations lists every analysis attestation (adversarial review)
// in the package with its findings, so the memo shows what was challenged
// and what a human must still disposition. Attestations never affect
// package validity; they are rendered for the reader, not enforced.
func renderAttestations(sb *strings.Builder, b *Builder, level int, heading string) {
	var atts []map[string]any
	for _, part := range b.manifest.Parts {
		if part.Role != RoleAttestation {
			continue
		}
		var att map[string]any
		if err := readJSONFile(b.path(part.Path), &att); err != nil {
			continue
		}
		atts = append(atts, att)
	}
	if len(atts) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s\n", headingLine(level, heading))
	for _, att := range atts {
		fmt.Fprintf(sb, "\n%s\n\n", headingLine(level+1, fmt.Sprint(att["id"])))
		if by, ok := att["performedBy"].(map[string]any); ok {
			fmt.Fprintf(sb, "- Performed by: %v (%v)", by["name"], by["type"])
			if model, ok := att["model"].(map[string]any); ok {
				fmt.Fprintf(sb, " — model %v/%v", model["provider"], model["name"])
				if v, ok := model["version"].(string); ok && v != "" {
					fmt.Fprintf(sb, " %s", v)
				}
			}
			sb.WriteString("\n")
		}
		if scope, ok := att["scope"].(map[string]any); ok {
			if ev, ok := scope["evidenceExamined"].([]any); ok {
				fmt.Fprintf(sb, "- Evidence examined: %d\n", len(ev))
			}
			if claims, ok := scope["claimsChallenged"].([]any); ok && len(claims) > 0 {
				fmt.Fprintf(sb, "- Claims challenged: %d\n", len(claims))
			}
		}
		findings, _ := att["findings"].([]any)
		if len(findings) == 0 {
			sb.WriteString("- Findings: none recorded\n")
		} else {
			fmt.Fprintf(sb, "\n**Findings (%d)**\n\n", len(findings))
			for _, f := range findings {
				fm, ok := f.(map[string]any)
				if !ok {
					continue
				}
				fmt.Fprintf(sb, "- [%v] %v — %v", fm["severity"], fm["id"], fm["summary"])
				if rel, ok := fm["relatedClaims"].([]any); ok && len(rel) > 0 {
					fmt.Fprintf(sb, " (claims: %v)", rel)
				}
				sb.WriteString("\n")
			}
		}
		if lims, ok := att["limitations"].([]any); ok && len(lims) > 0 {
			sb.WriteString("\n**Limitations**\n\n")
			for _, l := range lims {
				fmt.Fprintf(sb, "- %v\n", l)
			}
		}
		if disps, ok := att["dispositions"].([]any); ok && len(disps) > 0 {
			sb.WriteString("\n**Dispositions**\n\n")
			for _, d := range disps {
				dm, ok := d.(map[string]any)
				if !ok {
					continue
				}
				by, _ := dm["by"].(map[string]any)
				fmt.Fprintf(sb, "- %v: %v by %v", dm["findingId"], dm["decision"], by["name"])
				if note, ok := dm["note"].(string); ok && note != "" {
					fmt.Fprintf(sb, " — %s", note)
				}
				sb.WriteString("\n")
			}
		}
	}
}

// --- small rendering helpers ---

func getSliceAny(m map[string]any, key string) []any {
	s, _ := m[key].([]any)
	return s
}

// compactJSON renders v as single-line JSON, cut to max bytes with a marker.
func compactJSON(v any, max int) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	s := string(raw)
	if len(s) > max {
		cut := max
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut] + "..."
	}
	return s
}

// oneLine collapses whitespace runs (newlines included) into single spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
