# testdata

Fixture packages used by the CLI test suite.

- `valid/minimal-package` — the minimum valid v0.1 package: `manifest.json`, `workproduct.json`, `claims/claims.json`, `provenance/provenance.json`. Verifies with warnings (no hashes declared, no signature).
- `invalid/bad-hash` — the manifest declares a `sha256` for `workproduct.json` that does not match; `core.partHashes.match` must fail.
- `invalid/missing-provenance` — the declared provenance part is absent; `core.requiredParts.exist` must fail.
- `invalid/bad-workproduct-schema` — the workproduct is missing required fields and uses an invalid status; `core.workproduct.schemaValid` must fail.
