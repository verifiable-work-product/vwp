# VWP Roadmap

VWP is experimental. This roadmap is directional, not a commitment; ordering and scope
will change based on feedback and real-world use.

## v0.1 — Foundations (current)

- Core package format: `.vwp` ZIP container and part layout
- JSON Schemas (draft 2020-12) for the core objects
- TypeScript CLI: pack, unpack, verify
- Example packages (synthetic data only)
- Basic verification: structure, hashes, referential integrity, core validators

## v0.2 — Verification depth

- Better signature support (beyond the v0.1 experimental placeholder)
- Analysis attestation schema: structured, signed records of model-assisted
  review procedures over a defined evidence scope (see the whitepaper,
  section 4.5) — distinct from deterministic validation, never affecting
  package validity, always recording the exact model used (provider, model
  identifier, version) so challengers can differ from preparers
- Stronger package diffing (structural diffs between package versions)
- Package conformance tests
- Finance profile validators (assertion coverage, support coverage, review surfacing)
- ASC 606 and digital-asset validator improvements

## v0.3 — Ecosystem

- Python and Go SDKs (currently stubs under `sdk/`)
- More domain profiles beyond finance/audit/controls/digital assets
- Evidence locator standard (stable addressing of external evidence, plus
  optional intra-file anchors — line/character spans, pages, cell ranges — so
  facts can point inside the evidence they cite)
- Review/approval event log (append-only record of review activity)
- Better rendering templates for human-readable views

## v1.0 — Stability

- Stable package format and core schemas
- Validator plugin interface
- Signature and provenance specification
- Conformance suite for independent implementations
- Public reference packages

Feedback and proposals are welcome — see [`CONTRIBUTING.md`](./CONTRIBUTING.md).
