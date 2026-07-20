# VWP Roadmap

VWP is experimental. This roadmap is directional, not a commitment; ordering and scope
will change based on feedback and real-world use.

## v0.1 — Foundations (current)

- Core package format: `.vwp` ZIP container and part layout
- JSON Schemas (draft 2020-12) for the core objects
- TypeScript CLI: pack, unpack, verify
- Example packages (synthetic data only)
- Basic verification: structure, hashes, referential integrity, core validators

### Delivered in v0.1.x (pulled forward)

- Analysis attestation schema (originally a v0.2 item; see the whitepaper,
  section 4.5): structured records of model-assisted review procedures over a
  defined evidence scope — distinct from deterministic validation, never
  affecting package validity, always recording the exact model used (provider,
  model identifier, version) so challengers can differ from preparers.
  Published as `schemas/v0.1/analysis-attestation.schema.json` with
  [`spec/attestations.md`](./spec/attestations.md); the CLI schema-checks
  `attestations/*.json` parts.
- Intra-file evidence anchors (originally a v0.3 item): optional `anchors[]`
  on fact evidence references — composable per-chunk locators (`quote`,
  `page`, `lines`, `cells`, `charSpan`, `byteRange`, `section`, `jsonPath`,
  `region`). `quote` is deterministically verified against text evidence
  (`core.factAnchors.quotesMatch`); the rest are advisory and never affect
  integrity verification. See [`spec/facts.md`](./spec/facts.md) §4.
- Agent-session provenance: optional `sessionDigest` on provenance `toolchain`
  entries, tying a package to an append-only agent-session transcript by a
  hash-chain head without leaking its content. See
  [`spec/provenance.md`](./spec/provenance.md) §5.

## v0.2 — Verification depth

- Better signature support (beyond the v0.1 experimental placeholder)
- Stronger package diffing (structural diffs between package versions)
- Package conformance tests
- Finance profile validators (assertion coverage, support coverage, review surfacing)
- ASC 606 and digital-asset validator improvements

## v0.3 — Ecosystem

- Python and Go SDKs (currently stubs under `sdk/`)
- More domain profiles beyond finance/audit/controls/digital assets
- Evidence locator standard (stable addressing of external evidence; the
  intra-file anchor portion — line/character spans, pages, cell ranges —
  shipped early as `anchors[]`, see "Delivered in v0.1.x" above)
- Review/approval event log (append-only record of review activity)
- Better rendering templates for human-readable views

## v1.0 — Stability

- Stable package format and core schemas
- Validator plugin interface
- Signature and provenance specification
- Conformance suite for independent implementations
- Public reference packages

Feedback and proposals are welcome — see [`CONTRIBUTING.md`](./CONTRIBUTING.md).
