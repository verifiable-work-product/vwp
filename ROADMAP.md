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
- Go SDK (originally a v0.3 item): a full implementation under
  [`sdk/go`](./sdk/go) — package container with untrusted-input
  protections, integrity verification, every validator id from the
  conformance table in [`spec/validations.md`](./spec/validations.md),
  and an idempotent package builder. Its embedded schema copy is
  drift-tested against `schemas/v0.1` in CI.
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

- Required Markdown readable: new producers use one `text/markdown` `.md` part
  with role `readable`; legacy packages using role `rendering` remain accepted.
  Optional native XLSX/PDF/DOCX stays separate as the `primary-artifact`
- Presentation-map schema and referential-integrity validator: bidirectional links from
  facts, claims, calculations, and judgments to statements, pages, cells, and regions in
  embedded native artifacts or derived renderings (ADR 0004)
- Better signature support (beyond the v0.1 experimental placeholder)
- Stronger package diffing (structural diffs between package versions)
- Package conformance tests
- Finance profile validators (assertion coverage, support coverage, review surfacing)
- ASC 606 and digital-asset validator improvements
- Forecast extension (`vwp-finance-forecast`) validator implementations — register
  schema/presence, scenario-calculation and assumption-judgment linkage, input
  freshness relative to the as-of date, horizon coverage, back-test resolution
  (specified in [`profiles/finance/forecast.md`](./profiles/finance/forecast.md))
  — plus a synthetic example package (e.g. a validator-rewards forecast)

## v0.3 — Ecosystem

- Python SDK (currently a stub under `sdk/python`)
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
