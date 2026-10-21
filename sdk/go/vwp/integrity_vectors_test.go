package vwp_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
)

// The conformance vectors (testdata/hashing/vectors.json) pin the integrity
// graph algorithm across implementations: every implementation must
// reproduce the self hashes, closure hashes and roots listed there. This
// test checks the Go SDK against the file; `go test ./vwp -run Vectors
// -update` regenerates it from the inputs defined below. Regenerate only
// after a deliberate algorithm change, and say so.

var updateVectors = flag.Bool("update", false, "rewrite testdata/hashing/vectors.json from the Go SDK")

const vectorsDescription = "VWP integrity graph conformance vectors (spec/integrity.md). " +
	"`self` lists objects with their RFC 8785 canonical bytes after review-field stripping and the expected self hash " +
	"sha256(\"vwp/1/self/\" + kind + \"\\n\" + canonical). " +
	"`strip` shows the review-layer stripping cases: input, stripped object, canonical bytes, self hash. " +
	"`evidenceClosure` lists evidence entries with their self hash and closure hash, whose single dependency line is " +
	"\"bytes:\" + sha256 (package-local), \"bytes:\" + externalRef.retrieval.contentHash (external), or \"bytes:-\". " +
	"`closure.package` is a complete small package keyed by part path; `closure.objects` lists every object of it " +
	"(narrative sections have kind narrative and id = section id) with its sorted dependency ids, self hash and closure hash " +
	"sha256(\"vwp/1/closure/\" + kind + \"\\n\" + selfHash + \"\\n\" + join(sorted \"id:closureHash\", \"\\n\")); " +
	"`closure.collections` are the collection roots sha256(\"vwp/1/root/\" + kind + \"\\n\" + join(sorted \"id:closureHash\", \"\\n\")), " +
	"`closure.formatHash` the self hash of narrative/format.json with kind memo-format, and " +
	"`closure.analysisRoot`, `closure.contentRoot`, `closure.reviewRoot` the roots of section 5. " +
	"See testdata/hashing/README.md."

type selfVector struct {
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	Object    map[string]any `json:"object"`
	Canonical string         `json:"canonical"`
	SelfHash  string         `json:"selfHash"`
}

type stripVector struct {
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	Input     map[string]any `json:"input"`
	Stripped  map[string]any `json:"stripped"`
	Canonical string         `json:"canonical"`
	SelfHash  string         `json:"selfHash"`
}

type evidenceClosureVector struct {
	Name        string         `json:"name"`
	Object      map[string]any `json:"object"`
	BytesLine   string         `json:"bytesLine"`
	SelfHash    string         `json:"selfHash"`
	ClosureHash string         `json:"closureHash"`
}

type objectVector struct {
	Id          string   `json:"id"`
	Kind        string   `json:"kind"`
	Deps        []string `json:"deps"`
	SelfHash    string   `json:"selfHash"`
	ClosureHash string   `json:"closureHash"`
}

type closureVector struct {
	Package      map[string]any    `json:"package"`
	Objects      []objectVector    `json:"objects"`
	Collections  map[string]string `json:"collections"`
	FormatHash   string            `json:"formatHash"`
	AnalysisRoot string            `json:"analysisRoot"`
	ContentRoot  string            `json:"contentRoot"`
	ReviewRoot   string            `json:"reviewRoot"`
}

type vectorsFile struct {
	Description     string                  `json:"description"`
	Algorithm       string                  `json:"algorithm"`
	Self            []selfVector            `json:"self"`
	Strip           []stripVector           `json:"strip"`
	EvidenceClosure []evidenceClosureVector `json:"evidenceClosure"`
	Closure         closureVector           `json:"closure"`
}

func vectorsPath() string { return fixturePath("testdata", "hashing", "vectors.json") }

func obj(pairs ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return m
}

func list(items ...any) []any {
	if items == nil {
		return []any{}
	}
	return items
}

// selfInputs are the self-hash cases: one object per kind plus the edge
// cases an encoder gets wrong (unicode and escaping, number formatting, key
// ordering, nested arrays, review fields present).
func selfInputs() []selfVector {
	return []selfVector{
		{Name: "fact", Kind: vwp.KindFact, Object: obj(
			"id", "f_1", "statement", "Total contract value is 300,000 USD.", "evidenceRefs", list("ev_1"))},
		{Name: "judgment with status (stripped)", Kind: vwp.KindJudgment, Object: obj(
			"id", "j_1", "topic", "Distinct", "question", "Is it distinct?", "proposedConclusion", "Yes",
			"basisFacts", list("f_1"), "status", "reviewed")},
		{Name: "workproduct with conclusionSummary (stripped)", Kind: vwp.KindWorkproduct, Object: obj(
			"id", "wp_1", "title", "Memo", "domain", "finance", "profile", "vwp-finance", "status", "draft",
			"conclusionSummary", "Written by the narrative.")},
		{Name: "claim with unicode and escaping", Kind: vwp.KindClaim, Object: obj(
			"id", "c_1", "statement", "R\u00e9sum\u00e9 na\u00efve \u2713 \u65e5\u672c\u8a9e \U0001F600 \"quoted\" <tag> & back\\slash\nnewline\ttab",
			"supportingFacts", list("f_1"))},
		{Name: "calculation with numbers", Kind: vwp.KindCalculation, Object: obj(
			"id", "calc_1", "type", "allocation",
			"inputs", obj("amount", 1500000, "rate", 0.1, "negative", -2.5, "large", 1e21, "zero", 0, "small", 1e-7, "whole", 150000.0, "third", 1.0/3.0),
			"outputs", obj("total", 150000), "dependsOn", list("f_1"))},
		{Name: "key ordering by UTF-16 code units", Kind: vwp.KindFact, Object: obj(
			"id", "f_order", "statement", "Keys sort by UTF-16 code units.", "evidenceRefs", list(),
			"B", 1, "_c", 2, "a", 3, "aa", 4, "\u00e9", 5, "Z", 6, "\U0001F600", 7, "\uff5e", 8)},
		{Name: "evidence with nested arrays", Kind: vwp.KindEvidence, Object: obj(
			"id", "ev_1", "type", "contractDocument", "title", "Order form", "path", "evidence/order-form.txt",
			"sha256", "6a2b9f5ab8bb6f22f7c5d6b1e2f4e1d9c0b8a7f6e5d4c3b2a1908f7e6d5c4b3a",
			"tags", list(list("a", 1), list(true, nil, obj("k", list()))), "retrievedAt", "2026-01-01T00:00:00Z")},
		{Name: "approval", Kind: vwp.KindApproval, Object: obj(
			"id", "a_1", "approver", obj("name", "Reviewer"), "action", "approved", "timestamp", "2026-10-20T01:00:00Z", "scope", "package")},
		{Name: "attestation", Kind: vwp.KindAttestation, Object: obj(
			"id", "att_1", "type", "adversarialReview", "performedBy", obj("type", "agent", "name", "reviewer"), "findings", list())},
		{Name: "provenance", Kind: vwp.KindProvenance, Object: obj(
			"createdAt", "2026-10-20T00:00:00Z", "createdBy", obj("type", "agent", "name", "vectorgen", "version", "1"))},
		{Name: "narrative section", Kind: vwp.KindNarrative, Object: obj(
			"id", "conclusion", "paragraphs", list(obj("text", "Revenue is recognized monthly.", "role", "conclusion",
				"cites", list(obj("id", "c_2", "hash", strings.Repeat("ab", 32))))))},
		{Name: "memo format", Kind: vwp.KindMemoFormat, Object: obj(
			"version", 1, "name", "House style", "sections", list(obj("id", "conclusion", "heading", "Conclusion", "level", 1, "kind", "conclusion")))},
	}
}

// stripInputs are the review-layer stripping cases. A fact carrying a
// status field is included to show that only judgments and work products
// are stripped.
func stripInputs() []stripVector {
	judgment := obj("id", "j_1", "topic", "Distinct", "question", "Is it distinct?", "proposedConclusion", "Yes")
	judgmentReviewed := obj("id", "j_1", "topic", "Distinct", "question", "Is it distinct?", "proposedConclusion", "Yes", "status", "reviewed")
	wp := obj("id", "wp_1", "title", "Memo", "domain", "finance", "profile", "vwp-finance", "status", "draft")
	wpSummary := obj("id", "wp_1", "title", "Memo", "domain", "finance", "profile", "vwp-finance", "status", "draft", "conclusionSummary", "Written by the narrative.")
	fact := obj("id", "f_1", "statement", "A fact is never stripped.", "evidenceRefs", list(), "status", "reviewed")
	return []stripVector{
		{Name: "judgment without status", Kind: vwp.KindJudgment, Input: judgment},
		{Name: "judgment with status", Kind: vwp.KindJudgment, Input: judgmentReviewed},
		{Name: "workproduct without conclusionSummary", Kind: vwp.KindWorkproduct, Input: wp},
		{Name: "workproduct with conclusionSummary", Kind: vwp.KindWorkproduct, Input: wpSummary},
		{Name: "fact with a status field (not stripped)", Kind: vwp.KindFact, Input: fact},
	}
}

func evidenceClosureInputs() []evidenceClosureVector {
	return []evidenceClosureVector{
		{Name: "package-local bytes", Object: obj("id", "ev_local", "type", "contractDocument", "title", "Order form",
			"path", "evidence/order-form.txt", "sha256", "f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2")},
		{Name: "external contentHash", Object: obj("id", "ev_external", "type", "systemExport", "title", "Ledger export",
			"externalRef", obj("locator", "https://example.com/ledger.csv", "locatorType", "url",
				"retrieval", obj("url", "https://example.com/ledger.csv", "contentHash", "0a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f9")))},
		{Name: "no byte hash (sentinel)", Object: obj("id", "ev_opaque", "type", "other", "title", "Ticket",
			"externalRef", obj("locator", "TICKET-42", "locatorType", "opaque"))},
	}
}

// closurePackageParts is the small package of the closure section, before
// the narrative (whose cite hashes depend on everything else) is added.
func closurePackageParts() map[string]any {
	return map[string]any{
		vwp.WorkproductPath: obj("id", "wp_vectors", "title", "Vectors", "domain", "finance", "profile", "vwp-finance",
			"status", "draft", "conclusionSummary", "Ignored by the hash."),
		vwp.ProvenancePath: obj("createdAt", "2026-10-20T00:00:00Z", "createdBy", obj("type", "agent", "name", "vectorgen")),
		vwp.EvidenceIndex: obj("evidence", list(
			obj("id", "ev_1", "type", "contractDocument", "title", "Order form", "path", "evidence/order-form.txt",
				"sha256", "f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2"))),
		vwp.FactsPath: obj("facts", list(
			obj("id", "f_1", "statement", "Total contract value is 300,000 USD.",
				"evidenceRefs", list(obj("evidenceId", "ev_1", "anchors", list(obj("quote", "300,000 USD"))))),
			obj("id", "f_2", "statement", "The term is 24 months.", "evidenceRefs", list("ev_1")))),
		"calculations/calc_1.json": obj("id", "calc_1", "type", "allocation",
			"inputs", obj("total", 300000, "months", 24), "outputs", obj("monthly", 12500), "dependsOn", list("f_1", "f_2")),
		vwp.JudgmentsPath: obj("judgments", list(
			obj("id", "j_1", "topic", "Term", "question", "Is the term fixed?", "proposedConclusion", "Yes",
				"basisFacts", list("f_2"), "status", "reviewRequired"))),
		vwp.ClaimsPath: obj("claims", list(
			obj("id", "c_1", "statement", "A contract exists.", "supportingFacts", list("f_1"), "supportingEvidence", list("ev_1")),
			obj("id", "c_2", "statement", "Revenue is recognized monthly.", "supportingFacts", list("f_2"),
				"supportingCalculations", list("calc_1"), "judgments", list("j_1")))),
		vwp.MemoFormatPath: obj("version", 1, "name", "Vectors style", "sections", list(
			obj("id", "background", "heading", "Background", "level", 1, "kind", "narrative"),
			obj("id", "conclusion", "heading", "Conclusion", "level", 1, "kind", "conclusion"))),
	}
}

// writeParts materializes JSON parts into dir and opens it as a package.
func writeParts(t *testing.T, parts map[string]any) vwp.Source {
	t.Helper()
	dir := t.TempDir()
	for rel, value := range parts {
		raw, err := vwp.EncodeJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func mustSelfHash(t *testing.T, kind string, o map[string]any) string {
	t.Helper()
	h, err := vwp.SelfHash(kind, o)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func mustCanonical(t *testing.T, o map[string]any) string {
	t.Helper()
	c, err := vwp.Canonical(o)
	if err != nil {
		t.Fatal(err)
	}
	return string(c)
}

func closureHashOf(t *testing.T, g *vwp.Graph, id string) string {
	t.Helper()
	o, ok := g.Lookup(id)
	if !ok || o.Tainted {
		t.Fatalf("object %s missing or tainted", id)
	}
	return o.ClosureHash
}

// buildClosureVectors assembles the closure package (narrative included)
// and records every hash the Go SDK computes for it.
func buildClosureVectors(t *testing.T) closureVector {
	t.Helper()
	parts := closurePackageParts()
	g, err := vwp.BuildGraph(writeParts(t, parts))
	if err != nil {
		t.Fatal(err)
	}
	if !g.Complete() {
		t.Fatalf("closure package incomplete: %v %v", g.Unresolved, g.Cycles)
	}
	preDigests, err := g.Digests()
	if err != nil {
		t.Fatal(err)
	}
	formatHash := mustSelfHash(t, vwp.KindMemoFormat, parts[vwp.MemoFormatPath].(map[string]any))
	cite := func(id string) map[string]any { return obj("id", id, "hash", closureHashOf(t, g, id)) }
	parts[vwp.NarrativePath] = obj("formatHash", formatHash, "analysisRoot", preDigests.AnalysisRoot, "sections", list(
		obj("id", "background", "paragraphs", list(
			obj("text", "The order form fixes the price and the term.", "role", "analysis", "cites", list(cite("f_1"), cite("f_2"), cite("ev_1"))))),
		obj("id", "conclusion", "paragraphs", list(
			obj("text", "Revenue is recognized monthly.", "role", "conclusion", "cites", list(cite("c_2"), cite("f_1")))),
			"subsections", list(obj("id", "basis", "heading", "Basis", "paragraphs", list(
				obj("text", "A contract exists.", "role", "analysis", "cites", list(cite("c_1")))))))))
	// The approval commits to the content root, which the narrative is
	// part of; its reviewRoot is the review state before it was recorded.
	contentRoot := vwp.ContentRootOf(preDigests.AnalysisRoot, func() string {
		g2, err := vwp.BuildGraph(writeParts(t, parts))
		if err != nil {
			t.Fatal(err)
		}
		return g2.CollectionRoot(vwp.KindNarrative)
	}())
	parts[vwp.ApprovalsPath] = obj("approvals", list(
		obj("id", "a_1", "approver", obj("name", "Reviewer"), "action", "approved", "timestamp", "2026-10-20T01:00:00Z",
			"scope", "package", "contentRoot", contentRoot, "reviewRoot", preDigests.ReviewRoot)))

	g, err = vwp.BuildGraph(writeParts(t, parts))
	if err != nil {
		t.Fatal(err)
	}
	digests, err := g.Digests()
	if err != nil {
		t.Fatal(err)
	}
	if digests.ContentRoot != contentRoot {
		t.Fatal("content root moved when the approval was added")
	}
	var objects []objectVector
	for _, o := range g.Objects {
		objects = append(objects, objectVector{Id: o.Id, Kind: o.Kind, Deps: depsOrEmpty(o.Deps), SelfHash: o.SelfHash, ClosureHash: o.ClosureHash})
	}
	for _, o := range g.Narrative {
		objects = append(objects, objectVector{Id: o.Id, Kind: o.Kind, Deps: depsOrEmpty(o.Deps), SelfHash: o.SelfHash, ClosureHash: o.ClosureHash})
	}
	objects = append(objects,
		objectVector{Id: "", Kind: vwp.KindWorkproduct, Deps: []string{}, SelfHash: g.Workproduct.SelfHash},
		objectVector{Id: "", Kind: vwp.KindProvenance, Deps: []string{}, SelfHash: g.Provenance.SelfHash})
	sort.Slice(objects, func(i, j int) bool {
		if objects[i].Kind != objects[j].Kind {
			return objects[i].Kind < objects[j].Kind
		}
		return objects[i].Id < objects[j].Id
	})
	return closureVector{
		Package:      parts,
		Objects:      objects,
		Collections:  digests.Collections,
		FormatHash:   formatHash,
		AnalysisRoot: digests.AnalysisRoot,
		ContentRoot:  digests.ContentRoot,
		ReviewRoot:   digests.ReviewRoot,
	}
}

func depsOrEmpty(deps []string) []string {
	if deps == nil {
		return []string{}
	}
	return deps
}

func generateVectors(t *testing.T) vectorsFile {
	t.Helper()
	f := vectorsFile{Description: vectorsDescription, Algorithm: vwp.DigestAlgorithm}
	for _, v := range selfInputs() {
		v.Canonical = mustCanonical(t, vwp.Strip(v.Kind, v.Object))
		v.SelfHash = mustSelfHash(t, v.Kind, v.Object)
		f.Self = append(f.Self, v)
	}
	for _, v := range stripInputs() {
		v.Stripped = vwp.Strip(v.Kind, v.Input)
		v.Canonical = mustCanonical(t, v.Stripped)
		v.SelfHash = mustSelfHash(t, v.Kind, v.Input)
		f.Strip = append(f.Strip, v)
	}
	for _, v := range evidenceClosureInputs() {
		v.SelfHash = mustSelfHash(t, vwp.KindEvidence, v.Object)
		v.BytesLine = "bytes:" + vwp.EvidenceBytesHash(v.Object)
		v.ClosureHash = vwp.EvidenceClosureHash(v.SelfHash, vwp.EvidenceBytesHash(v.Object))
		f.EvidenceClosure = append(f.EvidenceClosure, v)
	}
	f.Closure = buildClosureVectors(t)
	return f
}

func TestIntegrityVectors(t *testing.T) {
	if *updateVectors {
		f := generateVectors(t)
		raw, err := vwp.EncodeJSON(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(vectorsPath()), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(vectorsPath(), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", vectorsPath())
	}

	raw, err := os.ReadFile(vectorsPath())
	if err != nil {
		t.Fatalf("read vectors (run with -update to generate): %v", err)
	}
	var f vectorsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if f.Algorithm != vwp.DigestAlgorithm {
		t.Fatalf("vectors algorithm %q, SDK implements %q", f.Algorithm, vwp.DigestAlgorithm)
	}

	for _, v := range f.Self {
		if got := mustCanonical(t, vwp.Strip(v.Kind, v.Object)); got != v.Canonical {
			t.Errorf("self %q: canonical\n got %s\nwant %s", v.Name, got, v.Canonical)
		}
		if got := mustSelfHash(t, v.Kind, v.Object); got != v.SelfHash {
			t.Errorf("self %q: selfHash got %s want %s", v.Name, got, v.SelfHash)
		}
	}
	for _, v := range f.Strip {
		stripped := vwp.Strip(v.Kind, v.Input)
		wantStripped, _ := json.Marshal(v.Stripped)
		gotStripped, _ := json.Marshal(stripped)
		if !bytes.Equal(wantStripped, gotStripped) {
			t.Errorf("strip %q: stripped got %s want %s", v.Name, gotStripped, wantStripped)
		}
		if got := mustCanonical(t, stripped); got != v.Canonical {
			t.Errorf("strip %q: canonical got %s want %s", v.Name, got, v.Canonical)
		}
		if got := mustSelfHash(t, v.Kind, v.Input); got != v.SelfHash {
			t.Errorf("strip %q: selfHash got %s want %s", v.Name, got, v.SelfHash)
		}
	}
	// The stripped cases hash identically with and without the review field.
	byName := map[string]string{}
	for _, v := range f.Strip {
		byName[v.Name] = v.SelfHash
	}
	if byName["judgment without status"] != byName["judgment with status"] {
		t.Error("judgment status must not change the self hash")
	}
	if byName["workproduct without conclusionSummary"] != byName["workproduct with conclusionSummary"] {
		t.Error("workproduct conclusionSummary must not change the self hash")
	}

	for _, v := range f.EvidenceClosure {
		self := mustSelfHash(t, vwp.KindEvidence, v.Object)
		bytesHash := vwp.EvidenceBytesHash(v.Object)
		if self != v.SelfHash || "bytes:"+bytesHash != v.BytesLine || vwp.EvidenceClosureHash(self, bytesHash) != v.ClosureHash {
			t.Errorf("evidenceClosure %q: got self %s bytes %s closure %s", v.Name, self, bytesHash, vwp.EvidenceClosureHash(self, bytesHash))
		}
	}

	g, err := vwp.BuildGraph(writeParts(t, f.Closure.Package))
	if err != nil {
		t.Fatal(err)
	}
	if !g.Complete() {
		t.Fatalf("closure package incomplete: %v %v", g.Unresolved, g.Cycles)
	}
	for _, want := range f.Closure.Objects {
		var got *vwp.Object
		switch want.Kind {
		case vwp.KindWorkproduct:
			got = g.Workproduct
		case vwp.KindProvenance:
			got = g.Provenance
		case vwp.KindNarrative:
			for _, o := range g.Narrative {
				if o.Id == want.Id {
					got = o
				}
			}
		default:
			got, _ = g.Lookup(want.Id)
		}
		if got == nil {
			t.Errorf("closure object %s %q missing", want.Kind, want.Id)
			continue
		}
		if got.SelfHash != want.SelfHash || got.ClosureHash != want.ClosureHash || strings.Join(got.Deps, ",") != strings.Join(want.Deps, ",") {
			t.Errorf("closure object %s %q: got self %s closure %s deps %v; want self %s closure %s deps %v",
				want.Kind, want.Id, got.SelfHash, got.ClosureHash, got.Deps, want.SelfHash, want.ClosureHash, want.Deps)
		}
	}
	digests, err := g.Digests()
	if err != nil {
		t.Fatal(err)
	}
	for kind, want := range f.Closure.Collections {
		if digests.Collections[kind] != want {
			t.Errorf("collection root %s: got %s want %s", kind, digests.Collections[kind], want)
		}
	}
	format := f.Closure.Package[vwp.MemoFormatPath].(map[string]any)
	if got := mustSelfHash(t, vwp.KindMemoFormat, format); got != f.Closure.FormatHash {
		t.Errorf("formatHash got %s want %s", got, f.Closure.FormatHash)
	}
	if digests.AnalysisRoot != f.Closure.AnalysisRoot || digests.ContentRoot != f.Closure.ContentRoot || digests.ReviewRoot != f.Closure.ReviewRoot {
		t.Errorf("roots got %+v want analysis %s content %s review %s", digests, f.Closure.AnalysisRoot, f.Closure.ContentRoot, f.Closure.ReviewRoot)
	}
}
