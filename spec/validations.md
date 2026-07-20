# VWP Validations

**Status:** v0.1 draft, experimental.

## 1. Overview

Validation objects record the results of deterministic checks run against a package:
schema validation, hash verification, reference-graph integrity, calculation
recomputation, and profile/extension rules. Producers SHOULD ship validation results
under `validations/` so consumers can see what was checked at production time; verifiers
re-run checks independently and MUST NOT rely on shipped results as proof. Validation
objects validate against `https://vwp.dev/schemas/v0.1/validation.schema.json`.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package (or within a verification report). |
| `validator` | string | yes | Identifier and version of the validator that ran (e.g. `vwp-core/refgraph@0.1`). |
| `scope` | string / object | no | What was checked: whole package, a part path, or specific object `id`s. |
| `status` | string | yes | One of `pass`, `fail`, `warning`, `reviewRequired`, `notRun`. |
| `severity` | string | no | e.g. `info`, `minor`, `major`, `critical`. Qualifies non-pass results. |
| `message` | string | no | Human-readable summary of the result. |
| `details` | object | no | Structured detail (offending paths, ids, expected vs actual values). |

## 3. Status semantics

| Status | Meaning | Effect on verification |
|---|---|---|
| `pass` | Check succeeded. | None. |
| `fail` | Check failed. | **Blocks verification.** The package MUST NOT be reported as verified. |
| `warning` | Check found a non-blocking concern. | Does not fail verification, unless the verifier runs in strict mode. |
| `reviewRequired` | Check identified something a human must review. | Does not fail verification, unless the verifier runs in strict mode. |
| `notRun` | Check was skipped or not applicable. | Informational only. |

Verifiers SHOULD offer a strict mode in which `warning` and `reviewRequired` results are
treated as failures.

## 4. Validator classes

- **Core validators** — always applicable: manifest schema, part hashes, path safety,
  object schemas, reference-graph integrity (no dangling `evidenceRefs`, fact/claim/
  judgment/calculation references).
- **Profile validators** — run when the manifest declares a profile: vocabulary
  membership (e.g. finance assertion types), domain consistency rules.
- **Extension validators** — run when the manifest declares an extension: narrow,
  domain-specific rules (see [profiles.md](profiles.md)).

Validators are deterministic. A validator MUST produce the same status for the same
package bytes, MUST NOT require network access or model inference, and MUST NOT execute
package contents.

Analysis attestations (see [attestations.md](attestations.md)) are not validators.
Verifiers schema-check attestation parts like any other part
(`core.attestations.schemaValid`); attestation content — findings, dispositions — never
affects any validation status.

## 5. Implemented validator ids

The validator ids below are implemented today by the reference TypeScript CLI
(`cli/src/lib/validations.ts`). Independent implementations — including the Go SDK in
development at Bitwave, to be upstreamed under `sdk/go` — MUST agree on these ids and
their semantics, so that reports from different verifiers are comparable.

### Core validators

| Id | Semantics |
|---|---|
| `core.package.safePaths` | Fails when any archive entry is absolute, contains `..` segments, backslashes, or drive letters. |
| `core.manifest.exists` | Fails when `manifest.json` is missing or not valid JSON. |
| `core.manifest.schemaValid` | Manifest validates against the manifest schema. |
| `core.requiredParts.exist` | `manifest.json`, `workproduct.json`, `claims/claims.json`, and `provenance/provenance.json` are present. |
| `core.partHashes.match` | Every manifest-declared part exists and its recomputed SHA-256 matches; warns when no hashes are declared. |
| `core.workproduct.schemaValid` | `workproduct.json` validates against the workproduct schema. |
| `core.claims.schemaValid` | Every claim validates against the claim schema. |
| `core.provenance.schemaValid` | `provenance/provenance.json` validates against the provenance schema. |
| `core.evidence.schemaValid` | Every evidence object validates against the evidence schema (when `evidence/evidence.json` is present). |
| `core.facts.schemaValid` | Every fact validates against the fact schema (when `facts/facts.json` is present). |
| `core.judgments.schemaValid` | Every judgment validates against the judgment schema (when present). |
| `core.approvals.schemaValid` | Every approval validates against the approval schema (when present). |
| `core.validations.schemaValid` | Every shipped validation result validates against the validation schema (when present). |
| `core.attestations.schemaValid` | Every `attestations/*.json` part validates against the analysis-attestation schema (when present). Attestation content never otherwise affects verification. |
| `core.evidenceRefs.resolve` | Evidence `path`s exist with matching hashes; `derivedFrom`, fact `evidenceRefs`, and claim `supportingEvidence` resolve to evidence ids. |
| `core.factAnchors.quotesMatch` | Quote anchors appear in the cited text evidence after normalization; warns on quotes against binary or external evidence. |
| `core.claimRefs.resolve` | Claim `supportingFacts`, `judgments`, and `supportingCalculations` resolve to objects in the package. |
| `core.signatures.present` | Warns when the package carries no signature (informational in v0.1). |

### Profile validators (`vwp-finance`)

| Id | Semantics |
|---|---|
| `finance.claims.haveAssertions` | Warns on claims with no `assertions`. |
| `finance.claims.haveSupport` | Warns on claims with no supporting facts, evidence, calculations, or judgments. |
| `finance.requiredJudgments.complete` | Reports `reviewRequired` while any judgment is `open` or `reviewRequired`. |

### Extension validators (`vwp-accounting-asc606`)

| Id | Semantics |
|---|---|
| `asc606.allocation.schemaValid` | `calculations/ssp-allocation.json` validates against the SSP-allocation extension schema. |
| `asc606.allocation.tieOut` | Allocated amounts sum to the transaction price (0.01 tolerance). |
| `asc606.revenueSchedule.total` | The revenue-schedule CSV `total` column sums to the allocated consideration. |

### Extension validators (`vwp-accounting-asc820`)

| Id | Semantics |
|---|---|
| `asc820.measurements.present` | Fails when `facts/fair-value-measurements.json` is missing or unreadable. |
| `asc820.measurements.schemaValid` | The measurement register validates against the fair-value-measurements extension schema. |
| `asc820.pricingSources.resolve` | Every measurement's `pricingSourceEvidence` resolves to evidence ids. |
| `asc820.level3.judgmentLinked` | Every `level3` measurement links a resolvable hierarchy/unobservable-inputs `judgmentRef`. |

### Extension validators (`vwp-digital-assets-wallet-ownership-control`)

| Id | Semantics |
|---|---|
| `digitalAssets.wallets.registerPresent` | Fails when `facts/wallets.json` is missing or contains no wallets. |
| `digitalAssets.wallets.schemaValid` | The wallet register validates against the wallets extension schema. |
| `digitalAssets.wallets.entityMapping` | Every included wallet has resolvable entity-mapping evidence. |
| `digitalAssets.wallets.proofOfControl` | Every included self-custody wallet has resolvable proof-of-control evidence. |
| `digitalAssets.wallets.exclusionRationale` | Every excluded wallet records an exclusion rationale. |
| `digitalAssets.wallets.balanceEvidence` | Every included wallet with a period-end balance has resolvable balance evidence. |

## 6. Limitations

> VWP validation means that a package conforms to specified schemas and validation rules.
> It does not mean the professional conclusion is correct, complete, audited, legally
> sufficient, or appropriate for a particular use. Professional judgment, applicable
> standards, and independent review still govern reliance on any VWP package.

## 7. Example

```json
{
  "id": "val-004",
  "validator": "vwp-finance/reconciliation@0.1",
  "scope": { "calculationIds": ["calc-002"] },
  "status": "pass",
  "severity": "info",
  "message": "Reconciliation calc-002 recomputed; outputs match.",
  "details": { "recomputedDifference": "0.00" }
}
```
