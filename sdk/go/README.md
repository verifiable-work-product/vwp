# vwp (Go SDK)

The official Go implementation of VWP (Verifiable Work Product): package
reading/writing, integrity verification, JSON Schema validation, core /
profile / extension validators, and an incremental package builder.

```
go get github.com/verifiable-work-product/vwp/sdk/go
```

Packages:

| Import path | Contents |
|---|---|
| `github.com/verifiable-work-product/vwp/sdk/go/vwp` | Core types, package container (`OpenPackage`, `Pack`, `Unpack`), manifest + SHA-256 integrity, `Verify` |
| `.../sdk/go/vwp/validators` | Built-in extension validators (ASC 606, ASC 820, wallet ownership, ASC 350-60, ASC 350-30, staking rewards, lending, cost basis, stablecoin) and the batteries-included `validators.Verify` |
| `.../sdk/go/vwp/accountingschedule` | The executable accounting schedule artifact (`calculations/accounting-schedule.json`): parse, validate, and the reviewer's account-binding config |
| `.../sdk/go/vwp/builder` | Incremental package builder over an unpacked directory |
| `.../sdk/go/vwp/schema` | JSON Schema validation against the embedded v0.1 schemas (core parts, memo format, narrative) |

## Verify a package

```go
import (
    vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
    "github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

pkg, err := vwp.OpenPackage("acme-asc606.vwp") // .vwp file or unpacked dir
if err != nil { ... }

report := validators.Verify(pkg) // core + profile + extension validators
for _, r := range report.Results {
    fmt.Printf("%-14s %s %s\n", r.Status, r.ValidatorId, r.Message)
}
if !report.Valid { ... } // any result with status "fail"
```

Packages are treated as untrusted input: ZIP entries with unsafe paths
(absolute, drive-letter, `..`) are rejected, and decompression is bounded
(`vwp.WithLimits`). `vwp.OpenPackageBytes` verifies in-memory uploads without
touching disk.

## Build a package

```go
import (
    vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
    "github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
)

b, err := builder.Scaffold(dir, builder.ScaffoldConfig{
    PackageId: "urn:uuid:...",
    CreatedAt: "2026-07-01T12:00:00Z",
    Profile:   "vwp-finance",
    Extension: "vwp-accounting-asc606",
    Workproduct: vwp.Workproduct{Id: "wp_1", Title: "...", Domain: "finance", Status: "draft"},
    Provenance:  vwp.Provenance{CreatedAt: "...", CreatedBy: vwp.ProvenanceAgent{Type: "agent", Name: "my-agent"}},
})

err = b.AddEvidence(vwp.Evidence{Id: "ev_1", Type: "contractDocument", Title: "...", Path: "evidence/contract.txt"}, contractBytes)
err = b.AddFact(vwp.Fact{Id: "f_1", Statement: "...", EvidenceRefs: []vwp.EvidenceRef{{
    EvidenceId: "ev_1",
    Anchors:    []vwp.Anchor{{Quote: "verbatim excerpt"}},
}}})
err = b.AddClaim(vwp.Claim{Id: "c_1", Statement: "..."})
err = b.LinkSupport("c_1", builder.SupportFacts, "f_1")
_, err = b.RunValidators(nil) // ships validations/validation-report.json
err = b.Render()              // derived markdown rendering
err = vwp.Pack(dir, "out.vwp")
```

Every builder operation validates the object against its schema before
writing, re-hashes touched parts, and rewrites the manifest, so the package
on disk is always integrity-consistent. Operations are idempotent by object
id: re-adding an identical object is a no-op, re-using an id with different
content returns `builder.ErrConflict` — safe under at-least-once retries
(e.g. workflow-engine activities).

## Integrity graph

Every package object has a self hash (SHA-256 over its RFC 8785 canonical
bytes, review-layer fields stripped) and a closure hash over everything it
references; collections roll up into the analysis, content and review roots
([`spec/integrity.md`](../../spec/integrity.md)). The builder keeps the
manifest `digests` block current on every mutation, stamps narrative cites
with the cited objects' closure hashes and approvals with the roots they
were taken against; the verifier recomputes all of it
(`core.graph.refsResolve`, `core.graph.acyclic`, `core.graph.declaredInputs`,
`core.digests.match`, `narrative.cites.fresh`, `narrative.drift`,
`approvals.current`).

```go
g, err := vwp.BuildGraph(pkg)          // objects by id, closure hashes, unresolved refs, cycles
d, err := g.Digests()                  // the manifest digests block (analysis, content, review roots)
h, err := vwp.SelfHash(vwp.KindFact, fact)
err = vwp.VerifyDigests(pkg)           // nil when the manifest's digests match or are absent

// builder: cites by id, hashes stamped for you
err = b.SetNarrativeSection(vwp.NarrativeSection{Id: "conclusion", Paragraphs: []vwp.NarrativeParagraph{{
    Text: "...", Role: vwp.ParagraphRoleConclusion, Cites: vwp.CiteIds("c_1", "calc_1"),
}}})
err = b.RecordApproval(vwp.Approval{...}) // contentRoot and reviewRoot stamped when absent
```

Declare `DependsOn` on calculations and `BasisFacts` on judgments so their
closure hashes cover what they consumed; `core.graph.declaredInputs` warns
when an input is recognizable but undeclared. The conformance vectors in
[`testdata/hashing/`](../../testdata/hashing/) pin the algorithm; `go test
./vwp -run TestIntegrityVectors -update` regenerates them.

## Required outputs

`vwp.RequiredOutputs(extension)` is the per-extension contract of mandatory,
manifest-bound outputs (the ASC 606 allocation, recognition CSV and accounting
schedule; the accounting schedule for every other extension that books
entries). Core verification enforces it (`core.requiredOutputs.exist`,
`accountingSchedule.valid`) and each extension's `*.accountingSchedule.tieOut`
proves the schedule books exactly what its calculations say. See
[`profiles/accounting/accounting-schedule.md`](../../profiles/accounting/accounting-schedule.md).

## Conformance

Validator ids and their semantics are the conformance table in
[`spec/validations.md`](../../spec/validations.md). For the ids the reference
TypeScript CLI implements, reports are comparable id-for-id, including message
conventions (capped at five offending items) and the shared 0.01 rounding
tolerance in the tie-out validators. This SDK is the only implementation of
the required-output checks, the accounting-schedule tie-outs and the ASC
350-60, ASC 350-30, staking, lending, cost-basis and stablecoin extensions.

## Embedded schemas

The SDK embeds `schemas/v0.1/**/*.json` (see
[`vwp/schema/vwpschemas/`](./vwp/schema/vwpschemas/)) because `go:embed`
cannot reach outside the module. The copy is enforced, not hand-maintained: a
drift test asserts byte-equality with the repository's schema tree on every
`go test` run. If it fails after a schema change:

```
cd sdk/go && make sync-schemas
```

## Development

```
cd sdk/go
make test   # go vet + go test ./...
```

Tests run against the repository's own [`examples/`](../../examples) and
[`testdata/`](../../testdata) fixtures — the same fixtures the reference CLI
tests use. The six accounting-extension examples are generated with the
builder by `go run fixturegen/main.go` (from `sdk/go`), which refuses to write
a package that does not verify clean; the same run refreshes the
hand-authored examples in place (manifest part hashes, integrity digests,
packed `.vwp`). Re-run it and commit the result after changing a validator's
data contract or the hashing algorithm.

Module path `github.com/verifiable-work-product/vwp/sdk/go`, `go 1.23`. No
tagged releases yet; APIs may change until the first tag.
