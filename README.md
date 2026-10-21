# VWP: Verifiable Work Product

VWP is an open package format for evidence-backed professional conclusions. A `.vwp` file bundles evidence references, extracted facts, claims, judgments, calculations, validations, approvals, provenance, and human-readable renderings into a portable, machine-verifiable artifact.

In accounting and audit, a VWP can render as a workpaper. In other domains, it may render as a memo, evidence package, control test, valuation report, claim file, or reproducibility bundle.

**Status: experimental v0.1 draft.** Everything here — format, schemas, CLI, and language — is open for discussion and subject to change.

> VWP validation is not an audit opinion, legal opinion, accounting opinion, or assurance engagement. It only means that a package conforms to specified schemas and validation rules. Professional judgment and applicable standards still govern reliance on any conclusion.

## Why VWP exists

Professional conclusions today live in PDFs, spreadsheets, and app databases. The memo a human reads is disconnected from the evidence, calculations, and judgments behind it, and no other system can check it without trusting the tool that produced it. This is getting worse, not better: AI systems now draft workpapers and memos at scale, and their output cannot be trusted directly.

VWP takes a different position:

- **The structured package is the source of truth.** The rendered memo or PDF is only a derived view.
- **AI output is not trusted directly.** AI may extract facts, draft claims, and render explanations — deterministic validators verify package integrity, schema validity, evidence links, math tie-outs, and completeness checks.
- **Model-agnostic.** Provenance records which tools and models produced a package, but verification never requires them.
- **Domain-extensible.** VWP Core is generic; profiles (finance) and extensions (ASC 606, ASC 820, ASC 350-60 crypto assets, ASC 350-30 impairment, wallet ownership/control, staking rewards, digital asset lending, cost basis, stablecoin classification, control tests, forward-looking forecasts) add domain vocabulary and validators.

## What a `.vwp` package contains

A `.vwp` file is a ZIP archive:

```text
example.vwp
  manifest.json                  required — identity, profile, parts, SHA-256 hashes
  workproduct.json               required — what work product this is
  claims/claims.json             required — the assertions and conclusions
  provenance/provenance.json     required — who/what produced it, with which tools and models
  evidence/                      source records (files or external references)
  facts/                         extracted observations, linked to evidence
  judgments/                     professional judgment points and their basis
  calculations/                  deterministic computations with inputs and outputs
  validations/                   recorded validator results
  approvals/                     review/approval events
  attestations/                  model-assisted review records (never affect validity)
  artifacts/                     optional embedded native work product (PDF, XLSX, DOCX, etc.)
  renderings/                    human-readable views; v0.2 proposes one required readable .md
  signatures/                    experimental in v0.1
```

The conceptual chain: `evidence -> facts -> claims -> judgments/calculations -> validations -> conclusion -> rendering`. See [spec/core.md](spec/core.md).

## Quick start

```bash
cd cli
npm install
npm run build

# Pack an example directory into a .vwp file (computing part hashes)
node dist/index.js pack ../examples/asc606-revenue-contract/package ../examples/asc606-revenue-contract/acme-asc606.vwp --write-hashes

# Verify it
node dist/index.js verify ../examples/asc606-revenue-contract/acme-asc606.vwp

# Inspect, render, unpack
node dist/index.js inspect ../examples/asc606-revenue-contract/acme-asc606.vwp
node dist/index.js render ../examples/asc606-revenue-contract/acme-asc606.vwp
node dist/index.js unpack ../examples/asc606-revenue-contract/acme-asc606.vwp /tmp/acme-asc606
```

The CLI can also construct a VWP without hand-authoring its manifest:

```bash
node dist/index.js init ./my-work-product --title "My Work Product"
node dist/index.js add evidence ./my-work-product --id ev_1 --title "Source data" --file ./source.csv
node dist/index.js add fact ./my-work-product --id f_1 --statement "Observed result." --evidence ev_1
node dist/index.js set-readable ./my-work-product ./memo.md
node dist/index.js embed ./my-work-product ./analysis.xlsx
node dist/index.js build ./my-work-product ./my-work-product.vwp
```

See the [CLI construction guide](cli/README.md) for claims, calculations, JSON
inputs, safety behavior, and the complete command reference.

The browser reader in [`frontend/`](frontend) displays the rendering and the
structured evidence chain together. After verification it can download a
self-contained `.readable.html` copy for an offline recipient, plus the
original `.vwp` for anyone who wants to re-run verification.

`verify` output for the ASC 606 example ends with:

```text
PASS finance.claims.haveAssertions
PASS finance.claims.haveSupport
REVIEW finance.requiredJudgments.complete - 1 judgment requires review: j_impl_distinct
PASS asc606.allocation.tieOut - allocated 300000 equals transaction price 300000
PASS asc606.revenueSchedule.total - schedule total 300000 equals allocated consideration 300000

Summary: valid with review items
```

Exit codes: `0` valid (warnings/review items allowed unless `--strict`), `1` invalid, `2` usage/runtime error.

## Examples (all synthetic data)

- [`examples/asc606-revenue-contract`](examples/asc606-revenue-contract) — a revenue recognition workpaper: five-step claims, an SSP allocation calculation that must tie out, a distinctness judgment left in review, and a memo rendering.
- [`examples/digital-assets-wallet-ownership`](examples/digital-assets-wallet-ownership) — wallet ownership/control for a treasury entity: proof-of-control evidence, entity mapping, completeness with exclusion rationale, and balance tie-outs.
- [`examples/asc820-fair-value`](examples/asc820-fair-value) — fair value support: a level 1 position priced from exchange quotes and a level 3 restricted token whose hierarchy classification and marketability discount are judgment-linked.
- [`examples/asc350-60-crypto-assets`](examples/asc350-60-crypto-assets) — ASC 350-60 crypto assets: a scope register judged against the six criteria, a per-asset roll-forward and significant-holdings table that tie to each other, and the remeasurement accounting schedule.
- [`examples/asc350-30-impairment`](examples/asc350-30-impairment) — indefinite-lived intangible impairment of NFT lots outside ASC 350-60, with the impairment accounting schedule.
- [`examples/staking-rewards`](examples/staking-rewards) — staking income at fair value on recognition, reconciled to chain-observed receipts, with the income accounting schedule.
- [`examples/digital-asset-lending`](examples/digital-asset-lending) — in-kind interest accrual and collateral coverage on a crypto loan, a judged derecognition conclusion, and the accrual accounting schedule.
- [`examples/cost-basis-lot-relief`](examples/cost-basis-lot-relief) — FIFO lot relief with realized gain/loss and the Rev. Proc. 2024-28 wallet-by-wallet allocation, with the gain/loss accounting schedule.
- [`examples/stablecoin-classification`](examples/stablecoin-classification) — a stablecoin holdings register whose classifications are judgment-linked and consistent per token and issuer.
- [`examples/asc842-lease`](examples/asc842-lease) — minimal lease workpaper stub.
- [`examples/sox-control-test`](examples/sox-control-test) — minimal internal-control evidence stub.

A note on the wallet example: proof-of-control evidence, such as a signed wallet message, can demonstrate control of a private key or signing authority at a point in time. It does not, by itself, establish legal title, beneficial ownership, or accounting recognition. Those conclusions require additional evidence and professional judgment.

## Repository layout

- [`docs/whitepaper.md`](docs/whitepaper.md) — the VWP whitepaper: motivation, object model, verification model, and role in audit workflows.
- [`spec/`](spec) — the v0.1 draft specification (package format, manifest, core objects, profiles).
- [`schemas/v0.1/`](schemas/v0.1) — JSON Schemas (draft 2020-12) for every core object.
- [`schemas/v0.2/`](schemas/v0.2) — proposed manifest and presentation-map schemas, including a required Markdown readable with legacy `rendering` role compatibility.
- [`profiles/`](profiles) — finance, audit, accounting, digital-assets, and controls profile documentation.
- [`cli/`](cli) — the reference TypeScript CLI (`pack`, `unpack`, `verify`, `inspect`, `diff`, `render`).
- [`viewer/`](viewer) — native macOS `.vwp` viewer: double-click file association, embedded-artifact preview, and structured inspection tabs.
- [`sdk/`](sdk) — SDKs: [`sdk/go`](sdk/go) is a full Go implementation (verifier, builder, schema validation); TypeScript and Python are stubs (the CLI is the reference implementation).
- [`testdata/`](testdata) — valid and invalid fixture packages used by the tests.
- [`docs/decisions/`](docs/decisions) — architecture decision records.

## What VWP is not

- Not an audit, assurance, legal, or accounting opinion — validation checks structure, integrity, and declared rules, not professional correctness.
- Not an accounting engine, a hosted service, or an AI agent — it is a format plus a verifier.
- Not crypto-specific — digital assets are one profile among several.
- Not blockchain-anchored, RDF-based, or cryptographically novel — v0.1 is deliberately boring: ZIP, JSON Schema, SHA-256, and experimental signatures.

## Security

`.vwp` packages may contain sensitive data and must be treated as untrusted input. The CLI refuses ZIP path traversal on unpack and never executes package contents. See [SECURITY.md](SECURITY.md). Never put real secrets, keys, customer records, or production financial data in example packages.

## Roadmap and contributing

See [ROADMAP.md](ROADMAP.md) (signatures, conformance tests, more profiles, Python SDK, validator plugin interface) and [CONTRIBUTING.md](CONTRIBUTING.md). Feedback from accounting, audit, and engineering stakeholders is the point of this draft — issues and PRs welcome.

## License

Apache-2.0. See [LICENSE](LICENSE).
