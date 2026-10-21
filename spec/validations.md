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

Core, profile, and extension validators are deterministic. A validator MUST produce the
same status for the same package bytes, MUST NOT require network access or model
inference, and MUST NOT execute package contents.

**Online checks** are a separate, opt-in class that MAY access the network (e.g.
`core.externalRefs.reachable`, which fetches `externalRef.retrieval.url` with the declared
auth and compares `contentHash`). Online checks are environment-dependent — their result
depends on network reachability and available credentials — so they are **never** part of
default verification, MUST be explicitly enabled by the operator, and their outcomes MUST
be reported as distinct from the deterministic core result. Credentials are supplied by the
operator at run time (gathered per origin, over the top); a package never carries a secret.

Analysis attestations (see [attestations.md](attestations.md)) are not validators.
Verifiers schema-check attestation parts like any other part
(`core.attestations.schemaValid`); attestation content — findings, dispositions — never
affects any validation status.

## 5. Implemented validator ids

The validator ids below are implemented today by the Go SDK in this repository
(`sdk/go`); the reference TypeScript CLI (`cli/src/lib/validations.ts`) implements the
core, finance, ASC 606, ASC 820 and wallet-ownership ids, and does not yet implement
the required-output checks, the accounting-schedule tie-outs, or the ASC 350-60,
ASC 350-30, staking, lending, cost-basis and stablecoin extensions (it reports no result
for those). Independent implementations MUST agree on these ids and their semantics, so
that reports from different verifiers are comparable.

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
| `core.externalRefs.wellFormed` | Offline shape check of `externalRef.retrieval`: `url`/`discovery` are valid URIs when `locatorType` is `url`, `auth.scheme` is recognized (`bearer`/`none` warn otherwise), and — critically — `auth.tokenHint` is a source reference, not a literal secret. An embedded secret **fails**; a malformed URL or unknown scheme **warns**. Never accesses the network. |
| `core.factAnchors.quotesMatch` | Quote anchors appear in the cited text evidence after normalization; warns on quotes against binary or external evidence. |
| `core.claimRefs.resolve` | Claim `supportingFacts`, `judgments`, and `supportingCalculations` resolve to objects in the package. |
| `core.graph.refsResolve` | fail | every dependency edge (facts, calculations, judgments, claims, narrative cites) resolves to an object; see [integrity.md](integrity.md) |
| `core.graph.acyclic` | fail | the dependency graph has no cycle |
| `core.graph.declaredInputs` | warning | a calculation or judgment uses a recognizable input it did not declare in `dependsOn` or `basisFacts` |
| `core.digests.match` | fail / notRun | the manifest `digests` block equals the recomputed roots; notRun without a block |
| `approvals.current` | reviewRequired / notRun | an `approved` action whose `contentRoot` is no longer the package's content root is superseded |
| `narrative.schemaValid`, `narrative.sections`, `narrative.citations`, `narrative.roles`, `narrative.cites.fresh`, `narrative.drift` | see [narrative.md](narrative.md) and [integrity.md](integrity.md) | the memo narrative holds to the structured parts it cites |
| `core.signatures.present` | Warns when the package carries no signature (informational in v0.1). |
| `core.requiredOutputs.exist` | For extensions with a required-output contract (see [`profiles/accounting/accounting-schedule.md`](../profiles/accounting/accounting-schedule.md)): every unconditional required output is present, and every present required output is declared exactly once in `manifest.parts` with role `calculation`, its media type and a hash. |
| `accountingSchedule.valid` | `calculations/accounting-schedule.json` parses and satisfies the executable accounting schedule contract (balanced decimal entries at the declared scale, allowed roles for its kind, bounded size). Reported when the artifact is present, or when the extension requires it unconditionally. |

### Profile validators (`vwp-finance`)

| Id | Semantics |
|---|---|
| `finance.claims.haveAssertions` | Warns on claims with no `assertions`. |
| `finance.claims.haveSupport` | Warns on claims with no supporting facts, evidence, calculations, or judgments. |
| `finance.requiredJudgments.complete` | Reports `reviewRequired` while any judgment is `open` or `reviewRequired`. |

### Extension validators (`vwp-accounting-asc606`)

| Id | Semantics |
|---|---|
| `asc606.allocation.schemaValid` | `calculations/ssp-allocation.json` (or a calculation of type `relativeStandaloneSellingPriceAllocation`) validates against the SSP-allocation extension schema. **Fails** when absent: the allocation is a required output. |
| `asc606.allocation.tieOut` | Allocated amounts sum to the transaction price (0.01 tolerance). Fails when the allocation is absent or schema-invalid. |
| `asc606.revenueSchedule.total` | The revenue-schedule CSV `total` column (or the `outputs.rows[].total` of a `revenueSchedule` calculation) sums to the allocated consideration. Fails when the schedule or a valid allocation is absent. |
| `asc606.accountingSchedule.tieOut` | The executable `revenue_recognition` schedule's amounts match the SSP allocation and every obligation/month amount in the recognition CSV. |

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

### Extension validators (`vwp-accounting-asc350-60`)

| Id | Semantics |
|---|---|
| `asc35060.scope.registerPresent` | Fails when `facts/crypto-assets.json` is missing or contains no assets. |
| `asc35060.scope.schemaValid` | The scope register validates against the crypto-assets extension schema. |
| `asc35060.scope.consistent` | `inScope` agrees with the six ASC 350-60-15-1 criteria for every asset; out-of-scope assets name their treatment. |
| `asc35060.scope.judgmentLinked` | Every out-of-scope asset links a resolvable scope judgment. |
| `asc35060.rollForward.schemaValid` | Every `cryptoAssetRollForward` calculation validates against the extension schema. |
| `asc35060.rollForward.tieOut` | Each roll-forward line and the totals tie; roll-forwards chain by scope and period. |
| `asc35060.rollForward.assetsInScope` | Only in-scope assets appear in a roll-forward. |
| `asc35060.significantHoldings.schemaValid` | Every `significantCryptoHoldings` calculation validates against the extension schema. |
| `asc35060.significantHoldings.tieOut` | Significant and "other" fair value and cost basis recompute from the holdings. |
| `asc35060.significantHoldings.assetsInScope` | Only in-scope assets appear in a holdings table. |
| `asc35060.significantHoldings.tiesToRollForward` | Per asset, holdings fair value equals the roll-forward closing balance on that date. |
| `asc35060.accountingSchedule.tieOut` | The `fair_value_remeasurement` schedule books each period's gain or loss per asset in the right direction, and nothing else. |

### Extension validators (`vwp-accounting-asc350-30`)

| Id | Semantics |
|---|---|
| `asc35030.impairment.schemaValid` | Every `indefiniteLivedIntangibleImpairmentTest` calculation validates against the extension schema. |
| `asc35030.impairment.recompute` | Per lot, loss = max(0, carrying − fair value); no lot tested twice in one test; no duplicate output rows. |
| `asc35030.impairment.noReversal` | The new carrying amount never exceeds the old one (ASC 350-30-35-20). |
| `asc35030.impairment.fairValueEvidence` | Each lot's fair value equals the value of the fact it cites. |
| `asc35030.impairment.continuity` | A lot tested again later starts from the carrying amount the earlier test left it at. |
| `asc35030.accountingSchedule.tieOut` | The `impairment` schedule books one loss per impaired lot on its test date. |

### Extension validators (`vwp-digital-assets-staking-rewards`)

| Id | Semantics |
|---|---|
| `staking.rewards.schemaValid` | Every `stakingRewardsSchedule` calculation validates against the extension schema. |
| `staking.rewards.recompute` | Each row's fair value is units × unit price; totals sum. |
| `staking.rewards.pricingEvidence` | Each row's measurement date is the date its `pricingDateBasis` fixes, and the cited price fact states that price as of that date. |
| `staking.rewards.chainTieOut` | Recognized units reconcile to chain-observed receipts through the declared unreceived balances and reconciling items. |
| `staking.rewards.population` | Recognition dates lie in the period; reward references are unique per asset and arrangement; schedules never overlap. |
| `staking.accountingSchedule.tieOut` | The `staking_rewards` schedule books each reward as income on its recognition date. |

### Extension validators (`vwp-digital-assets-lending`)

| Id | Semantics |
|---|---|
| `lending.accrual.schemaValid` | Every `digitalAssetLoanInterestAccrual` calculation validates against the extension schema. |
| `lending.accrual.recompute` | Each accrual is principal × rate × days / basis, and those terms match the cited terms fact. |
| `lending.collateral.schemaValid` | Every `collateralCoverage` calculation validates against the extension schema. |
| `lending.collateral.recompute` | Coverage ratio, margin shortfall and unsecured portion recompute. |
| `lending.derecognition.claimJudged` | A claim concludes on derecognition, citing a derecognition paragraph family, and links a judgment. |
| `lending.accountingSchedule.tieOut` | The `interest_accrual` schedule books one accrual per window on its last accrued day at the stated fair value. |

### Extension validators (`vwp-digital-assets-cost-basis`)

| Id | Semantics |
|---|---|
| `costBasis.lotRelief.schemaValid` | Every `lotRelief` calculation validates against the extension schema. |
| `costBasis.lotRelief.unitsConserved` | Units are conserved from the lot population through every relief into the remaining lots. |
| `costBasis.lotRelief.recompute` | Basis relieved and realized gain/loss recompute from the lots; no disposal relieves a lot acquired after it. |
| `costBasis.lotRelief.methodOrder` | The relief order follows the declared method. |
| `costBasis.walletAllocation.schemaValid` | Every `walletBasisAllocation` calculation validates against the extension schema. |
| `costBasis.walletAllocation.tieOut` | Each allocation ties to its declared basis and units. |
| `costBasis.walletAllocation.eligibility` | The allocation is dated 2025-01-01 and equals the units and basis of lots acquired before it; warns when no lot population is present. |
| `costBasis.accountingSchedule.tieOut` | The `realized_gain_loss` schedule books one entry per disposal: proceeds, cost basis relieved, realized gain or loss. |

### Extension validators (`vwp-digital-assets-stablecoin`)

| Id | Semantics |
|---|---|
| `stablecoin.holdings.registerPresent` | Fails when `facts/stablecoin-holdings.json` is missing or contains no holdings. |
| `stablecoin.holdings.schemaValid` | The register validates against the extension schema. |
| `stablecoin.classification.judgmentLinked` | Every holding links a resolvable classification judgment. |
| `stablecoin.redemption.evidence` | Cash-like classifications rest on evidenced redemption rights. |
| `stablecoin.classification.consistent` | One token from one issuer is classified the same way everywhere it is held. |
| `stablecoin.classification.cashEquivalentChallenged` | `reviewRequired` for holdings classified as cash or cash equivalent despite having no maturity. |

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
