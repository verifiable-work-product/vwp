package vwp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	jcs "github.com/gowebpki/jcs"
)

// The integrity graph (spec/integrity.md): every package object gets a self
// hash over its canonical bytes and a closure hash over its self hash and
// the closure hashes of everything it references; collections roll up into
// roots; the manifest's digests block records the roots. Everything here is
// derivable from the package alone, so a verifier recomputes it and a
// producer writes it.

// DigestAlgorithm is the identifier of the integrity graph algorithm written
// to the manifest digests block.
const DigestAlgorithm = "vwp-dag/1"

// Object kinds of the integrity graph (spec/integrity.md section 3). The
// memo format kind only ever self-hashes (narrative formatHash).
const (
	KindEvidence    = "evidence"
	KindFact        = "fact"
	KindCalculation = "calculation"
	KindJudgment    = "judgment"
	KindClaim       = "claim"
	KindNarrative   = "narrative"
	KindApproval    = "approval"
	KindAttestation = "attestation"
	KindWorkproduct = "workproduct"
	KindProvenance  = "provenance"
	KindMemoFormat  = "memo-format"
)

// collectionKinds are the kinds whose collection roots the manifest digests
// block carries, in analysis root order followed by narrative and approval.
var collectionKinds = []string{KindEvidence, KindFact, KindCalculation, KindJudgment, KindClaim, KindNarrative, KindApproval}

// EvidenceBytesSentinel is the bytes dependency of an evidence entry that
// declares neither a package-local sha256 nor an external contentHash.
const EvidenceBytesSentinel = "-"

// Digests is the manifest digests block: the three roots and the collection
// roots they are built from.
type Digests struct {
	Algorithm    string            `json:"algorithm"`
	AnalysisRoot string            `json:"analysisRoot"`
	ContentRoot  string            `json:"contentRoot"`
	ReviewRoot   string            `json:"reviewRoot"`
	Collections  map[string]string `json:"collections,omitempty"`
}

// Equal reports whether two digests blocks carry the same roots. A nil
// Collections map on either side matches any collections.
func (d *Digests) Equal(o *Digests) bool {
	if d == nil || o == nil {
		return d == o
	}
	if d.Algorithm != o.Algorithm || d.AnalysisRoot != o.AnalysisRoot || d.ContentRoot != o.ContentRoot || d.ReviewRoot != o.ReviewRoot {
		return false
	}
	if d.Collections == nil || o.Collections == nil {
		return true
	}
	if len(d.Collections) != len(o.Collections) {
		return false
	}
	for k, v := range d.Collections {
		if o.Collections[k] != v {
			return false
		}
	}
	return true
}

// Canonical returns the RFC 8785 (JSON Canonicalization Scheme) bytes of a
// JSON-encodable value: keys sorted by UTF-16 code units, no whitespace,
// ES6 number formatting, minimal string escaping. Go structs are encoded
// through their JSON tags first.
func Canonical(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to encode value: %w", err)
	}
	out, err := jcs.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize value: %w", err)
	}
	return out, nil
}

// strippedFields are the review-layer fields removed before an object of
// the given kind is hashed (spec/integrity.md section 2).
var strippedFields = map[string][]string{
	KindJudgment:    {"status"},
	KindWorkproduct: {"conclusionSummary"},
}

// Strip returns a copy of obj with the review-layer fields of its kind
// removed: a judgment's status and a work product's conclusionSummary.
// Every other kind is returned unchanged (as a copy). A nil obj yields an
// empty object.
func Strip(kind string, obj map[string]any) map[string]any {
	out := make(map[string]any, len(obj))
	for k, v := range obj {
		out[k] = v
	}
	for _, field := range strippedFields[kind] {
		delete(out, field)
	}
	return out
}

// jsonObject converts v (a struct, map or raw JSON) into its decoded JSON
// object shape.
func jsonObject(v any) (map[string]any, error) {
	var raw []byte
	switch t := v.(type) {
	case json.RawMessage:
		raw = t
	case []byte:
		raw = t
	default:
		var err error
		if raw, err = json.Marshal(v); err != nil {
			return nil, fmt.Errorf("failed to encode object: %w", err)
		}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("failed to decode object: %w", err)
	}
	if out == nil {
		return nil, fmt.Errorf("failed to decode object: not a JSON object")
	}
	return out, nil
}

func hashHex(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SelfHash is sha256("vwp/1/self/" + kind + "\n" + JCS(strip(obj))). obj may
// be a struct, a map or raw JSON bytes of an object.
func SelfHash(kind string, obj any) (string, error) {
	m, err := jsonObject(obj)
	if err != nil {
		return "", err
	}
	canon, err := Canonical(Strip(kind, m))
	if err != nil {
		return "", err
	}
	return hashHex("vwp/1/self/", kind, "\n", string(canon)), nil
}

// Dep is one dependency line of a closure hash or one entry of a collection
// root: an id and the closure hash recorded for it.
type Dep struct {
	Id   string
	Hash string
}

// depLines renders "id:hash" lines sorted by id (byte order).
func depLines(deps []Dep) string {
	sorted := make([]Dep, len(deps))
	copy(sorted, deps)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Id < sorted[j].Id })
	lines := make([]string, len(sorted))
	for i, d := range sorted {
		lines[i] = d.Id + ":" + d.Hash
	}
	return strings.Join(lines, "\n")
}

// ClosureHash is sha256("vwp/1/closure/" + kind + "\n" + selfHash + "\n" +
// deps) where deps joins "id:closureHash" lines sorted by id with "\n". An
// object without dependencies hashes the header, its self hash and a
// trailing newline.
func ClosureHash(kind, selfHash string, deps []Dep) string {
	return hashHex("vwp/1/closure/", kind, "\n", selfHash, "\n", depLines(deps))
}

// EvidenceClosureHash is the closure hash of an evidence entry: its single
// dependency is the SHA-256 of the evidence bytes keyed "bytes", or
// EvidenceBytesSentinel when no byte hash is declared.
func EvidenceClosureHash(selfHash, bytesSha256 string) string {
	if bytesSha256 == "" {
		bytesSha256 = EvidenceBytesSentinel
	}
	return ClosureHash(KindEvidence, selfHash, []Dep{{Id: "bytes", Hash: bytesSha256}})
}

// EvidenceBytesHash returns the byte hash an evidence entry commits to: its
// sha256 for package-local bytes, else externalRef.retrieval.contentHash,
// else EvidenceBytesSentinel.
func EvidenceBytesHash(ev map[string]any) string {
	if sha := getString(ev, "sha256"); sha != "" {
		return sha
	}
	if ref := asMap(ev["externalRef"]); ref != nil {
		if sha := getString(asMap(ref["retrieval"]), "contentHash"); sha != "" {
			return sha
		}
	}
	return EvidenceBytesSentinel
}

// CollectionRoot is sha256("vwp/1/root/" + kind + "\n" + entries) where
// entries joins "id:closureHash" lines sorted by id with "\n". An empty
// collection hashes the header line alone.
func CollectionRoot(kind string, entries []Dep) string {
	return hashHex("vwp/1/root/", kind, "\n", depLines(entries))
}

// AnalysisRootOf combines the work product and provenance self hashes with
// the evidence, fact, calculation, judgment and claim collection roots
// (spec/integrity.md section 5).
func AnalysisRootOf(workproductSelf, provenanceSelf string, collections map[string]string) string {
	return hashHex("vwp/1/analysis\n",
		"workproduct:", workproductSelf, "\n",
		"provenance:", provenanceSelf, "\n",
		"evidence:", collections[KindEvidence], "\n",
		"fact:", collections[KindFact], "\n",
		"calculation:", collections[KindCalculation], "\n",
		"judgment:", collections[KindJudgment], "\n",
		"claim:", collections[KindClaim])
}

// ContentRootOf adds the narrative collection root to the analysis root.
func ContentRootOf(analysisRoot, narrativeRoot string) string {
	return hashHex("vwp/1/content\n", analysisRoot, "\nnarrative:", narrativeRoot)
}

// JudgmentStatus is one judgment's review status line of the review root.
type JudgmentStatus struct {
	Id     string
	Status string
}

// ReviewRootOf covers what humans did: every judgment's status (sorted by
// id, "id:status" lines) and the approval collection root.
func ReviewRootOf(judgments []JudgmentStatus, approvalRoot string) string {
	sorted := make([]JudgmentStatus, len(judgments))
	copy(sorted, judgments)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Id < sorted[j].Id })
	lines := make([]string, len(sorted))
	for i, j := range sorted {
		lines[i] = j.Id + ":" + j.Status
	}
	return hashHex("vwp/1/review\n", strings.Join(lines, "\n"), "\n", "approval:", approvalRoot)
}

// Edge is one declared dependency of an object: the field it was declared
// in and the id it names.
type Edge struct {
	Field string
	Ref   string
}

// Object is one node of the integrity graph.
type Object struct {
	Kind  string
	Id    string
	Value map[string]any
	// Edges are the declared dependencies in document order; Deps are their
	// ids, deduplicated and sorted, as hashed.
	Edges []Edge
	Deps  []string
	// SelfHash is always set. ClosureHash is empty when Tainted: the object
	// or something it transitively depends on is unresolved or cyclic.
	SelfHash    string
	ClosureHash string
	Tainted     bool
}

// DuplicateIdError is returned by BuildGraph when an id is used by objects
// of two kinds, which would make every reference to it ambiguous.
type DuplicateIdError struct {
	Kind         string
	Id           string
	ExistingKind string
}

func (e *DuplicateIdError) Error() string {
	return fmt.Sprintf("%s %s: id already used by a %s", e.Kind, e.Id, e.ExistingKind)
}

// Problem is an unresolved dependency: an object names an id that is not in
// the package.
type Problem struct {
	Kind  string
	Id    string
	Field string
	Ref   string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s %s: unknown %s ref %s", p.Kind, p.Id, p.Field, p.Ref)
}

// Graph is the integrity graph of a package: every referenceable object by
// id, the narrative sections, the two singletons, and what did not resolve.
type Graph struct {
	// Objects holds evidence, facts, calculations, judgments, claims,
	// approvals and attestations by id.
	Objects map[string]*Object
	// Narrative holds one object per top-level narrative section (kind
	// narrative, id = section id), in document order. Sections are not
	// referenceable, so they live outside the id index.
	Narrative []*Object
	// Workproduct and Provenance are nil when the part is absent.
	Workproduct *Object
	Provenance  *Object
	// NarrativePart is the parsed narrative, nil when absent.
	NarrativePart *Narrative
	// Unresolved lists every dependency that names an unknown id.
	Unresolved []Problem
	// Cycles lists every dependency cycle found, each as the ids on the
	// cycle in traversal order.
	Cycles [][]string
}

// Complete reports whether every dependency resolved and no cycle exists,
// which is when every closure hash and the roots are defined.
func (g *Graph) Complete() bool {
	return len(g.Unresolved) == 0 && len(g.Cycles) == 0
}

// Lookup returns the referenceable object with the given id.
func (g *Graph) Lookup(id string) (*Object, bool) {
	o, ok := g.Objects[id]
	return o, ok
}

// Kind returns the kind of the referenceable object with the given id, or
// "" when unknown.
func (g *Graph) Kind(id string) string {
	if o, ok := g.Objects[id]; ok {
		return o.Kind
	}
	return ""
}

// ObjectsOfKind returns the referenceable objects of one kind sorted by id.
func (g *Graph) ObjectsOfKind(kind string) []*Object {
	var out []*Object
	for _, o := range g.Objects {
		if o.Kind == kind {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}

// CollectionRoot computes the root of one collection kind over the objects
// the graph holds. Tainted objects contribute an empty hash; callers that
// need a defined root check Complete first.
func (g *Graph) CollectionRoot(kind string) string {
	var entries []Dep
	if kind == KindNarrative {
		for _, o := range g.Narrative {
			entries = append(entries, Dep{Id: o.Id, Hash: o.ClosureHash})
		}
	} else {
		for _, o := range g.ObjectsOfKind(kind) {
			entries = append(entries, Dep{Id: o.Id, Hash: o.ClosureHash})
		}
	}
	return CollectionRoot(kind, entries)
}

// JudgmentStatuses returns every judgment's id and status.
func (g *Graph) JudgmentStatuses() []JudgmentStatus {
	var out []JudgmentStatus
	for _, o := range g.ObjectsOfKind(KindJudgment) {
		out = append(out, JudgmentStatus{Id: o.Id, Status: getString(o.Value, "status")})
	}
	return out
}

// Digests computes the manifest digests block. It fails when the graph is
// incomplete (unresolved ids or cycles) or the work product or provenance
// part is missing, since the roots are undefined then.
func (g *Graph) Digests() (*Digests, error) {
	if len(g.Unresolved) > 0 {
		return nil, fmt.Errorf("failed to compute digests: unresolved reference: %s", g.Unresolved[0])
	}
	if len(g.Cycles) > 0 {
		return nil, fmt.Errorf("failed to compute digests: dependency cycle: %s", strings.Join(g.Cycles[0], " -> "))
	}
	if g.Workproduct == nil {
		return nil, fmt.Errorf("failed to compute digests: %s is missing", WorkproductPath)
	}
	if g.Provenance == nil {
		return nil, fmt.Errorf("failed to compute digests: %s is missing", ProvenancePath)
	}
	collections := make(map[string]string, len(collectionKinds))
	for _, kind := range collectionKinds {
		collections[kind] = g.CollectionRoot(kind)
	}
	analysis := AnalysisRootOf(g.Workproduct.SelfHash, g.Provenance.SelfHash, collections)
	return &Digests{
		Algorithm:    DigestAlgorithm,
		AnalysisRoot: analysis,
		ContentRoot:  ContentRootOf(analysis, collections[KindNarrative]),
		ReviewRoot:   ReviewRootOf(g.JudgmentStatuses(), collections[KindApproval]),
		Collections:  collections,
	}, nil
}

// ComputeDigests builds the integrity graph of a package and returns its
// digests block.
func ComputeDigests(pkg Source) (*Digests, error) {
	g, err := BuildGraph(pkg)
	if err != nil {
		return nil, err
	}
	return g.Digests()
}

// dependencyFields lists, per kind, the fields whose string entries are
// dependency ids (spec/integrity.md section 4). Fact evidenceRefs and
// narrative cites are handled separately because their entries are
// objects.
var dependencyFields = map[string][]string{
	KindCalculation: {"dependsOn"},
	KindJudgment:    {"basisFacts", "basisCalculations"},
	KindClaim:       {"supportingFacts", "supportingEvidence", "supportingCalculations", "judgments"},
}

// edgesOf extracts an object's declared dependencies.
func edgesOf(kind string, obj map[string]any) []Edge {
	var edges []Edge
	add := func(field, ref string) {
		edges = append(edges, Edge{Field: field, Ref: ref})
	}
	switch kind {
	case KindFact:
		for _, ref := range getSlice(obj, "evidenceRefs") {
			add("evidenceRefs", refEvidenceId(ref))
		}
	case KindNarrative:
		eachParagraph(Narrative{Sections: []NarrativeSection{sectionFromValue(obj)}}, func(_ string, p NarrativeParagraph) {
			for _, c := range p.Cites {
				add("cites", c.Id)
			}
		})
	default:
		for _, field := range dependencyFields[kind] {
			for _, ref := range getSlice(obj, field) {
				s, _ := ref.(string)
				add(field, s)
			}
		}
	}
	return edges
}

// sectionFromValue decodes a narrative section object; a shape that does
// not decode yields a section with no paragraphs.
func sectionFromValue(obj map[string]any) NarrativeSection {
	raw, err := json.Marshal(obj)
	if err != nil {
		return NarrativeSection{}
	}
	var s NarrativeSection
	if err := json.Unmarshal(raw, &s); err != nil {
		return NarrativeSection{}
	}
	return s
}

func uniqueSorted(edges []Edge) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range edges {
		if !seen[e.Ref] {
			seen[e.Ref] = true
			out = append(out, e.Ref)
		}
	}
	sort.Strings(out)
	return out
}

// newObject hashes an object and records its edges.
func newObject(kind, id string, value map[string]any) (*Object, error) {
	self, err := SelfHash(kind, value)
	if err != nil {
		return nil, fmt.Errorf("failed to hash %s %s: %w", kind, id, err)
	}
	edges := edgesOf(kind, value)
	return &Object{Kind: kind, Id: id, Value: value, Edges: edges, Deps: uniqueSorted(edges), SelfHash: self}, nil
}

// BuildGraph reads every object of a package, hashes it, resolves its
// dependencies and computes closure hashes. Unresolved ids and cycles are
// reported on the graph, not as errors; an error means a part could not be
// read or parsed, or an id is used by two objects (its kind would be
// ambiguous).
func BuildGraph(pkg Source) (*Graph, error) {
	g := &Graph{Objects: map[string]*Object{}}

	readDoc := func(relPath string) (map[string]any, bool, error) {
		if !pkg.Has(relPath) {
			return nil, false, nil
		}
		raw, err := pkg.Read(relPath)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read %s: %w", relPath, err)
		}
		doc, err := jsonObject(json.RawMessage(raw))
		if err != nil {
			return nil, false, fmt.Errorf("failed to parse %s: %w", relPath, err)
		}
		return doc, true, nil
	}
	index := func(kind string, value map[string]any, where string) error {
		id := getString(value, "id")
		if id == "" {
			return fmt.Errorf("failed to index %s in %s: object has no id", kind, where)
		}
		if existing, ok := g.Objects[id]; ok {
			return &DuplicateIdError{Kind: kind, Id: id, ExistingKind: existing.Kind}
		}
		obj, err := newObject(kind, id, value)
		if err != nil {
			return err
		}
		g.Objects[id] = obj
		return nil
	}
	collection := func(relPath, key, kind string) error {
		doc, ok, err := readDoc(relPath)
		if err != nil || !ok {
			return err
		}
		for _, item := range getSlice(doc, key) {
			m, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("failed to parse %s: %s entry is not an object", relPath, key)
			}
			if err := index(kind, m, relPath); err != nil {
				return err
			}
		}
		return nil
	}

	wp, ok, err := readDoc(WorkproductPath)
	if err != nil {
		return nil, err
	}
	if ok {
		if g.Workproduct, err = newObject(KindWorkproduct, getString(wp, "id"), wp); err != nil {
			return nil, err
		}
	}
	prov, ok, err := readDoc(ProvenancePath)
	if err != nil {
		return nil, err
	}
	if ok {
		if g.Provenance, err = newObject(KindProvenance, "", prov); err != nil {
			return nil, err
		}
	}
	if err := collection(EvidenceIndex, "evidence", KindEvidence); err != nil {
		return nil, err
	}
	if err := collection(FactsPath, "facts", KindFact); err != nil {
		return nil, err
	}
	for _, p := range pkg.List() {
		var kind string
		switch {
		case strings.HasPrefix(p, "calculations/") && strings.HasSuffix(p, ".json"):
			kind = KindCalculation
		case attestationPathRe.MatchString(p):
			kind = KindAttestation
		default:
			continue
		}
		raw, err := pkg.Read(p)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", p, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil || getString(doc, "id") == "" {
			// Not an object with an id: an artifact (the accounting
			// schedule), a malformed part another validator reports.
			continue
		}
		if err := index(kind, doc, p); err != nil {
			return nil, err
		}
	}
	if err := collection(JudgmentsPath, "judgments", KindJudgment); err != nil {
		return nil, err
	}
	if err := collection(ClaimsPath, "claims", KindClaim); err != nil {
		return nil, err
	}
	if err := collection(ApprovalsPath, "approvals", KindApproval); err != nil {
		return nil, err
	}

	narrativeDoc, ok, err := readDoc(NarrativePath)
	if err != nil {
		return nil, err
	}
	if ok {
		var n Narrative
		raw, _ := json.Marshal(narrativeDoc)
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", NarrativePath, err)
		}
		g.NarrativePart = &n
		for _, item := range getSlice(narrativeDoc, "sections") {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("failed to parse %s: section is not an object", NarrativePath)
			}
			obj, err := newObject(KindNarrative, getString(m, "id"), m)
			if err != nil {
				return nil, err
			}
			g.Narrative = append(g.Narrative, obj)
		}
	}

	g.resolveAll()
	return g, nil
}

// resolveAll computes closure hashes bottom-up, recording unresolved ids
// and cycles. Tainted objects (unresolved or cyclic, directly or
// transitively) get no closure hash.
func (g *Graph) resolveAll() {
	const (
		unvisited = iota
		inProgress
		done
	)
	state := map[string]int{}
	var visit func(o *Object, stack []string)
	visit = func(o *Object, stack []string) {
		state[o.Id] = inProgress
		var deps []Dep
		for _, e := range o.Edges {
			if _, ok := g.Objects[e.Ref]; !ok || e.Ref == "" {
				g.Unresolved = append(g.Unresolved, Problem{Kind: o.Kind, Id: o.Id, Field: e.Field, Ref: e.Ref})
				o.Tainted = true
			}
		}
		for _, id := range o.Deps {
			dep, ok := g.Objects[id]
			if !ok {
				continue
			}
			switch state[id] {
			case inProgress:
				start := 0
				for i, s := range stack {
					if s == id {
						start = i
						break
					}
				}
				cycle := append(append([]string{}, stack[start:]...), id)
				g.Cycles = append(g.Cycles, cycle)
				o.Tainted = true
				continue
			case unvisited:
				visit(dep, append(stack, id))
			}
			if dep.Tainted {
				o.Tainted = true
			}
			deps = append(deps, Dep{Id: id, Hash: dep.ClosureHash})
		}
		if !o.Tainted {
			if o.Kind == KindEvidence {
				o.ClosureHash = EvidenceClosureHash(o.SelfHash, EvidenceBytesHash(o.Value))
			} else {
				o.ClosureHash = ClosureHash(o.Kind, o.SelfHash, deps)
			}
		}
		state[o.Id] = done
	}
	ids := make([]string, 0, len(g.Objects))
	for id := range g.Objects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if state[id] == unvisited {
			visit(g.Objects[id], []string{id})
		}
	}
	// Narrative sections depend on objects only, so they resolve last and
	// can neither be depended on nor cycle.
	for _, sec := range g.Narrative {
		var deps []Dep
		for _, e := range sec.Edges {
			if _, ok := g.Objects[e.Ref]; !ok {
				g.Unresolved = append(g.Unresolved, Problem{Kind: sec.Kind, Id: sec.Id, Field: e.Field, Ref: e.Ref})
				sec.Tainted = true
			}
		}
		for _, id := range sec.Deps {
			dep, ok := g.Objects[id]
			if !ok {
				continue
			}
			if dep.Tainted {
				sec.Tainted = true
			}
			deps = append(deps, Dep{Id: id, Hash: dep.ClosureHash})
		}
		if !sec.Tainted {
			sec.ClosureHash = ClosureHash(KindNarrative, sec.SelfHash, deps)
		}
	}
}
