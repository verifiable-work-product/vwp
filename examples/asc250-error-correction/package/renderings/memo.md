# ASC 250 Error Correction - DemoCo Accrued Hosting Fees, FY2024-FY2025

- Package: `vwp_demo_asc250_error_correction`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc250`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

Hosting fees of 15,000.00 USD (FY2024) and 10,000.00 USD (FY2025) were never accrued. The cumulative 25,000.00 USD misstatement is 6.25% of FY2025 pretax income under the iron-curtain method (the largest single-year effect is 3.75% under rollover); with the qualitative factors considered the error is material and is corrected by restatement as of 2026-01-01.

## Claims

### s1_c_error

December hosting fees of 15,000.00 USD (FY2024) and 10,000.00 USD (FY2025) were not accrued at either year end: an error in previously issued financial statements, not a change in estimate.

- Conclusion: Error identified and characterized under ASC 250-10-20.
- Status: supported

### s2_c_quantified

Accrued liabilities are restated from 120,000.00 to 135,000.00 USD at 2024-12-31 and from 135,000.00 to 160,000.00 USD at 2025-12-31; the cumulative correction at 2026-01-01 is 25,000.00 USD.

- Conclusion: Quantified by period; cumulative correction recomputes.
- Status: supported

### s3_c_materiality

Under SAB 108 the misstatement is 3.75% of FY2025 pretax income by rollover and 6.25% by iron curtain; with the SAB 99 qualitative factors considered it is material.

- Conclusion: Material; both methods quantified.
- Status: supported

### s4_c_restatement

The error is corrected by restatement: a 25,000.00 USD prior-period adjustment to opening retained earnings against accrued liabilities as of 2026-01-01, with the FY2024 and FY2025 statements restated.

- Conclusion: Restatement through a prior-period adjustment.
- Status: supported

### s5_c_disclosure

The restatement discloses that previously issued statements were restated, the nature of the error, and the effect on each line item, income and retained earnings for each period presented.

- Conclusion: Disclosure per ASC 250-10-50-7 to 50-9.
- Status: supported

## Judgments

### s3_j_materiality — Materiality of the unaccrued hosting fees (SAB 99 / SAB 108)

Is the cumulative 25,000.00 USD understatement of accrued liabilities, 6.25% of FY2025 pretax income under the iron-curtain method and 3.75% at most under rollover, material?

- Proposed conclusion: Yes. The iron-curtain quantification exceeds the 5% threshold, the error affected two consecutive years, and it masks a change in the expense trend; no qualitative factor reduces its significance. The error is material and the prior periods are restated.
- Status: reviewed

### s4_j_method — Correction method

Is the error corrected by restating the prior periods (ASC 250-10-45-23) or as an out-of-period adjustment in FY2026?

- Proposed conclusion: Restatement: the FY2024 and FY2025 financial statements are restated, the opening balance of accrued liabilities and retained earnings at 2026-01-01 is corrected through a prior-period adjustment, and the restatement is disclosed.
- Status: reviewed

## Validations

- `core.package.safePaths` — pass
- `core.manifest.exists` — pass
- `core.manifest.schemaValid` — pass
- `core.requiredOutputs.exist` — pass
- `accountingSchedule.valid` — pass
- `core.requiredParts.exist` — pass
- `core.partHashes.match` — pass
- `core.workproduct.schemaValid` — pass
- `core.claims.schemaValid` — pass
- `core.provenance.schemaValid` — pass
- `core.evidence.schemaValid` — pass
- `core.facts.schemaValid` — pass
- `core.judgments.schemaValid` — pass
- `core.evidenceRefs.resolve` — pass
- `core.externalRefs.wellFormed` — notRun — no external retrieval metadata declared
- `core.factAnchors.quotesMatch` — pass — 5 quote anchors verified
- `core.claimRefs.resolve` — pass
- `core.graph.refsResolve` — pass
- `core.graph.acyclic` — pass
- `core.graph.declaredInputs` — pass
- `core.digests.match` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `asc250.correction.schemaValid` — pass — calculations/s2_error_correction.json
- `asc250.correction.recompute` — pass — 1 analysis(es): restated amounts, cumulative corrections and both SAB 108 quantifications recompute against a basis their fact states
- `asc250.correction.dualMethod` — pass — both the rollover and the iron-curtain quantifications are present against a declared threshold
- `asc250.correction.materialityJudged` — pass — the materiality judgment resolves and a material result is dispositioned
- `asc250.accountingSchedule.tieOut` — pass — 1 entry tie to 1 calculation(s) as error_correction in USD
