# Integrity graph conformance vectors

`vectors.json` pins the hashing algorithm of [spec/integrity.md](../../spec/integrity.md)
so that every implementation (the Go SDK, the TypeScript CLI, anything else)
produces the same self hashes, closure hashes and roots for the same package.
CI runs the file against each implementation. The Go SDK generates it
(`cd sdk/go && go test ./vwp -run TestIntegrityVectors -update`); regenerate
only after a deliberate algorithm change, and say so in the change.

All hashes are lowercase hex SHA-256 over UTF-8 bytes. `algorithm` is the
manifest `digests.algorithm` value the vectors describe (`vwp-dag/1`).

## Canonical form

An object's canonical bytes are its RFC 8785 (JCS) serialization after
**stripping** the review-layer fields of its kind:

| kind | stripped |
|---|---|
| `judgment` | `status` |
| `workproduct` | `conclusionSummary` |
| every other kind | nothing |

JCS facts the vectors exercise: object keys sorted by UTF-16 code units
(`"B" < "Z" < "_c" < "a" < "aa" < "é" < "😀"`; note `"～"` U+FF5E sorts
before U+1F600 because the latter is a surrogate pair), no whitespace, ES6
number formatting (`1500000`, `0.1`, `1e+21`, `1e-7`, `150000` for `150000.0`,
`0.3333333333333333`), strings escaped minimally (`\"`, `\\`, `\n`, `\t`,
control characters as `\u00XX`; everything else, non-ASCII included, is
literal UTF-8).

## Hash inputs

```
selfHash(kind, obj)   = sha256("vwp/1/self/" + kind + "\n" + canonical)
closureHash(kind)     = sha256("vwp/1/closure/" + kind + "\n" + selfHash + "\n" + deps)
  deps                = join(sorted by id: id + ":" + closureHash(dep), "\n")
                        (ids deduplicated, sorted by Unicode code point /
                        UTF-8 byte order; empty when there are no deps, so
                        the input then ends with selfHash + "\n")
closureHash(evidence) = sha256("vwp/1/closure/evidence\n" + selfHash + "\nbytes:" + bytesSha256)
  bytesSha256         = the entry's sha256, else externalRef.retrieval.contentHash, else "-"
collectionRoot(kind)  = sha256("vwp/1/root/" + kind + "\n" + join(sorted by id: id + ":" + closureHash, "\n"))
                        (an empty collection hashes the header line alone,
                        e.g. sha256("vwp/1/root/attestation\n"))
analysisRoot          = sha256("vwp/1/analysis\n" + "workproduct:" + selfHash(workproduct) + "\n"
                        + "provenance:" + selfHash(provenance) + "\n" + "evidence:" + root + "\n"
                        + "fact:" + root + "\n" + "calculation:" + root + "\n" + "judgment:" + root + "\n"
                        + "claim:" + root)
contentRoot           = sha256("vwp/1/content\n" + analysisRoot + "\nnarrative:" + collectionRoot(narrative))
reviewRoot            = sha256("vwp/1/review\n" + join(sorted by id: id + ":" + status, "\n") + "\n"
                        + "approval:" + collectionRoot(approval))
                        (a judgment without a status contributes "id:"; with
                        no judgments the input is "vwp/1/review\n\napproval:...")
```

Dependency fields per kind: fact `evidenceRefs[].evidenceId` (a bare string
ref is the id itself); calculation `dependsOn[]`; judgment `basisFacts[]`
and `basisCalculations[]`; claim `supportingFacts[]`, `supportingEvidence[]`,
`supportingCalculations[]`, `judgments[]`; narrative section `cites[].id` of
every paragraph in the section, subsections included. Approvals,
attestations, the work product and provenance have no dependencies.

Objects and ids: evidence entries come from `evidence/evidence.json`, facts
from `facts/facts.json`, judgments from `judgments/judgments.json`, claims
from `claims/claims.json`, approvals from `approvals/approvals.json`;
calculations are every `calculations/*.json` document that is an object
with a string `id` (the accounting schedule artifact has none and is not an
object of the graph); attestations are `attestations/*.json` documents with
an `id`. The **narrative collection** has one object per top-level section
of `narrative/narrative.json`: kind `narrative`, id = the section id, self
hash over the section object as written (its cites carry the hashes
recorded at writing time), closure over the objects it cites. The memo
format snapshot (`narrative/format.json`) only ever self-hashes, with kind
`memo-format`; that value is the narrative's `formatHash`.

## File layout

- `description`, `algorithm`: what the file is.
- `self[]`: `{ name, kind, object, canonical, selfHash }`. `canonical` is
  the exact JCS string of the stripped object; `selfHash` its self hash.
- `strip[]`: `{ name, kind, input, stripped, canonical, selfHash }`. The
  judgment pair and the work product pair hash identically with and without
  their review field; the fact carrying a `status` is not stripped.
- `evidenceClosure[]`: `{ name, object, bytesLine, selfHash, closureHash }`
  for the three byte-hash cases (`bytesLine` is the single dependency line).
- `closure`: a complete small package and everything derived from it.
  - `package`: the parts keyed by package-relative path (JSON values, write
    them out to reconstruct the package). The narrative's cites carry the
    real closure hashes and its `analysisRoot` the real analysis root; the
    approval carries the real `contentRoot` and the `reviewRoot` as it was
    before the approval existed.
  - `objects[]`: `{ id, kind, deps, selfHash, closureHash }` for every
    object, narrative sections included; the work product and provenance
    appear with an empty id and no closure hash.
  - `collections`: the seven collection roots of the manifest digests block.
  - `formatHash`: self hash of `narrative/format.json`.
  - `analysisRoot`, `contentRoot`, `reviewRoot`: the three roots.

Two of the self hashes, verified by hand:

```
printf 'vwp/1/self/fact\n{"evidenceRefs":["ev_1"],"id":"f_1","statement":"Total contract value is 300,000 USD."}' | sha256sum
printf 'vwp/1/self/judgment\n{"basisFacts":["f_1"],"id":"j_1","proposedConclusion":"Yes","question":"Is it distinct?","topic":"Distinct"}' | sha256sum
```
