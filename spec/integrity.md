# VWP Integrity Graph

**Status:** draft for v0.1 (additive), proposed 2026-10-20.

## 1. Overview

A VWP package is a directed acyclic graph: evidence anchors facts, facts and
calculations support claims, judgments qualify claims and weigh facts, the
narrative cites all of them, and approvals cover the whole. Part digests in
the manifest (see [manifest.md](manifest.md)) make the package tamper-evident
at the file level. They cannot say what a conclusion rests on, or whether any
of it changed after prose was written or a reviewer signed.

The integrity graph makes those questions mechanical. Every object gets a
**self hash** over its canonical bytes and a **closure hash** over its self
hash and the closure hashes of everything it references. A claim's closure
hash is a commitment to its entire support chain; a narrative paragraph
records the closure hashes of what it cites; an approval records the
package's content root. A verifier recomputes all of it from the package
alone.

Everything in this document is derivable: no hash in this section is
authoritative over the objects it summarizes. A producer writes the roots
into the manifest so a reader can confirm the package's own commitment; a
verifier that finds a mismatch reports it and continues.

## 2. Canonicalization

The canonical form of a JSON value is its RFC 8785 (JSON Canonicalization
Scheme) serialization, UTF-8 encoded. Producers and verifiers MUST use JCS;
the ordering and number formatting of any particular language's encoder are
never part of the contract.

Before canonicalization an object is **stripped** of its review-layer fields,
so a human review action never changes the hash of the content reviewed:

| Object | Fields stripped |
|---|---|
| judgment | `status` |
| workproduct | `conclusionSummary` (written by the narrative) |
| every object | nothing else |

## 3. Hashes

All hashes are SHA-256, lowercase hex. Inputs are domain-separated with a
version prefix so no two hash kinds can collide and the algorithm can be
revised.

```
selfHash(kind, obj)      = sha256("vwp/1/self/" + kind + "\n" + JCS(strip(obj)))
closureHash(kind, obj)   = sha256("vwp/1/closure/" + kind + "\n" + selfHash + "\n" + deps)
  deps                   = join(sorted by id: depId + ":" + closureHash(dep), "\n")
collectionRoot(kind)     = sha256("vwp/1/root/" + kind + "\n" + join(sorted by id: id + ":" + closureHash, "\n"))
```

`kind` is one of `evidence`, `fact`, `calculation`, `judgment`, `claim`,
`narrative`, `approval`, `attestation`, `workproduct`, `provenance`. An empty
collection's root is the hash of its header line alone.

Evidence binds to bytes: an evidence entry's closure includes the SHA-256 of
the evidence bytes as its single dependency, keyed `bytes`:

```
closureHash(evidence) = sha256("vwp/1/closure/evidence\n" + selfHash + "\nbytes:" + bytesSha256)
```

where `bytesSha256` is the entry's `sha256` (package-local bytes, verified by
`core.partHashes.match`) or `retrieval.contentHash` for external evidence,
or the literal `-` when neither is declared.

## 4. Edges

An object's dependencies are the ids named in these fields. Every id MUST
resolve to an object in the package; the graph MUST be acyclic.

| Object | Dependency fields |
|---|---|
| fact | `evidenceRefs[].evidenceId` |
| calculation | `dependsOn[]` (new, optional: ids of facts, calculations or evidence the computation consumed) |
| judgment | `basisFacts[]`, `basisCalculations[]` (new, optional: what the judgment weighed) |
| claim | `supportingFacts[]`, `supportingEvidence[]`, `supportingCalculations[]`, `judgments[]` |
| narrative paragraph | `cites[].id` |
| approval | none (approvals commit to a root, section 6) |

Producers SHOULD declare `dependsOn` on every calculation and `basisFacts`
on every judgment. An undeclared dependency does not make a package invalid;
it makes the object's closure hash blind to that input, which the
`core.graph.declaredInputs` validator reports as a warning when an input id
is recognizable (an extension's known link fields, e.g. ASC 606 `sspFact`).

## 5. Roots and the manifest `digests` block

```
analysisRoot = sha256("vwp/1/analysis\n"
                 + "workproduct:" + selfHash(workproduct) + "\n"
                 + "provenance:"  + selfHash(provenance)  + "\n"
                 + "evidence:"    + collectionRoot(evidence) + "\n"
                 + "fact:"        + collectionRoot(fact) + "\n"
                 + "calculation:" + collectionRoot(calculation) + "\n"
                 + "judgment:"    + collectionRoot(judgment) + "\n"
                 + "claim:"       + collectionRoot(claim))
contentRoot  = sha256("vwp/1/content\n" + analysisRoot + "\nnarrative:" + collectionRoot(narrative))
reviewRoot   = sha256("vwp/1/review\n"
                 + join(sorted judgments: id + ":" + status, "\n") + "\n"
                 + "approval:" + collectionRoot(approval))
```

The analysis root covers what the model produced; the content root adds the
prose a human reads; the review root covers what humans did about it.
Recording a review changes the review root only.

The manifest MAY carry the roots:

```json
"digests": {
  "algorithm": "vwp-dag/1",
  "analysisRoot": "…",
  "contentRoot": "…",
  "reviewRoot": "…",
  "collections": { "evidence": "…", "fact": "…", "calculation": "…", "judgment": "…", "claim": "…", "narrative": "…", "approval": "…" }
}
```

Because the manifest is what a signature covers (see
[signatures.md](signatures.md)), a signed manifest with a `digests` block
signs the whole graph, not only the file list.

## 6. Commitments

- **Narrative.** Each paragraph's `cites` entries are `{ "id", "hash" }`
  where `hash` is the cited object's closure hash at the time of writing.
  The narrative part records `analysisRoot` and `formatHash` (the self hash
  of the memo format snapshot it was written against).
- **Approvals.** An approval records `contentRoot` (and SHOULD record
  `reviewRoot`) at the time of the action. Its `scope` may still name ids.
- **Attestations.** `scope` MAY carry `contentRoot`; `evidenceExamined`
  keeps its per-evidence `sha256`.

## 7. Validators

Core validators, run by every verifier:

| Id | Status on problem | Meaning |
|---|---|---|
| `core.graph.refsResolve` | fail | every dependency id in section 4 resolves (subsumes `core.claimRefs.resolve`) |
| `core.graph.acyclic` | fail | no dependency cycle |
| `core.graph.declaredInputs` | warning | a calculation or judgment uses a recognizable input it did not declare |
| `core.digests.match` | fail | the manifest's `digests` block, when present, equals the recomputed roots |
| `narrative.cites.fresh` | fail | every cited `hash` equals the object's current closure hash; names the stale objects and the sections citing them |
| `narrative.drift` | reviewRequired | all cites are fresh but the analysis root moved since the narrative was written (something new and uncited exists) |
| `approvals.current` | reviewRequired | the package has `approved` actions with a `contentRoot` but none equals the current content root: every approval is superseded and a new one is required. Superseded approvals followed by a current one are history (pass, listed in the message); approvals stay in the record |

Existing narrative validators (`narrative.schemaValid`, `.sections`,
`.citations`, `.roles`) are unchanged except that `narrative.stale` is
replaced by `narrative.cites.fresh` and `narrative.drift`.

## 7a. Details both implementations follow

- **Narrative objects.** The `narrative` collection has one object per
  top-level narrative section: kind `narrative`, id = the section id, self
  hash over the section as written (subsections included), dependencies =
  every cite id in the section and its subsections. Sections are not in
  the id index: nothing can cite a section.
- **Dependency ids are deduplicated** before sorting; sorting is by byte
  (code point) order of the id.
- **Empty inputs.** An object with no dependencies hashes
  `selfHash + "\n"` (an empty `deps`). A package with no judgments hashes
  the review root over `"vwp/1/review\n" + "\n" + "approval:" + root`. A
  judgment without a `status` contributes `id + ":"`.
- **Collections in `digests.collections`** are exactly `evidence`, `fact`,
  `calculation`, `judgment`, `claim`, `narrative`, `approval`. Attestations
  are hashed as objects (kind `attestation`) but roll into no root.
- **Discovery.** Calculations are the `calculations/*.json` parts that are
  objects with a string `id` (artifacts such as the executable accounting
  schedule are not calculation objects). Attestations are the
  `attestations/*.json` parts with an `id`.
- **Duplicate ids** across collections make the graph unbuildable:
  `core.graph.refsResolve` fails naming the id and both kinds.
- **Skipped checks are reported as `notRun`**, never omitted:
  `core.digests.match` without a `digests` block, `approvals.current` when
  no approval carries a `contentRoot`, `narrative.cites.fresh` when no cite
  carries a hash, `narrative.drift` while `narrative.cites.fresh` fails.
- **Basis mentions** (`core.graph.declaredInputs`) match fact ids as whole
  tokens: an id is mentioned when it appears bounded by characters outside
  `[A-Za-z0-9_-]` or the text's ends.

## 8. Conformance vectors

`testdata/hashing/vectors.json` lists canonical objects and their expected
self hashes, a small package and its closure hashes and roots, and the
stripping cases (a judgment with and without `status`, a work product with and
without `conclusionSummary`). Every implementation MUST pass the vectors; CI
runs them for the Go SDK and the TypeScript CLI.

## 9. Compatibility

All fields are optional additions to the v0.1 schemas. A package without a
`digests` block, `cites[].hash`, or approval `contentRoot` verifies as before;
the new validators skip what is absent. The bitwave-svcs mirror's earlier
narrative stamps (`analysisDigest`, `formatSha256`) are replaced, not
migrated: no production package carries them.
