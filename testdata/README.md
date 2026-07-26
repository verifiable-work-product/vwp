# testdata

Fixture packages shared by the CLI test suite (`cli/test`) and the Go SDK
test suite (`sdk/go`). Both implementations run against these same fixtures
(and the packages under `examples/`), so a fixture change exercises every
verifier.

- `valid/minimal-package` — the minimum valid v0.1 package: `manifest.json`, `workproduct.json`, `claims/claims.json`, `provenance/provenance.json`. Verifies with warnings (no hashes declared, no signature).
- `invalid/bad-hash` — the manifest declares a `sha256` for `workproduct.json` that does not match; `core.partHashes.match` must fail.
- `invalid/missing-provenance` — the declared provenance part is absent; `core.requiredParts.exist` must fail.
- `invalid/bad-anchor-quote` — a fact's anchor quote does not appear in the cited text evidence; `core.factAnchors.quotesMatch` must fail while `core.evidenceRefs.resolve` passes.
- `invalid/bad-workproduct-schema` — the workproduct is missing required fields and uses an invalid status; `core.workproduct.schemaValid` must fail.

ZIP-safety cases (path traversal, decompression limits) are crafted
in-memory by the test suites rather than checked in as binary fixtures.
