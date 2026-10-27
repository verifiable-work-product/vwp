# Period-End Reconciliation and Flux - DemoCo Exchange Account, Q2 2026

- Package: `vwp_demo_period_end_reconciliation`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-reconciliation`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

The exchange account reconciles at 2026-06-30: the 2,650.00 USD difference between the statement (1,250,000.00) and the ledger (1,247,350.00) is a 3,000.00 deposit in transit and a 350.00 trading fee the ledger had not recorded; the fee is booked, the deposit clears itself. The quarter's flux explains the one variance above threshold (digital assets, +42%) with the ETH purchase.

## Claims

### s3_c_digital_assets_flux

Digital assets rose from 1,850,000.00 to 2,627,000.00 USD (+42%) because 300 ETH were bought on 2026-05-20 for 777,000.00 USD.

- Conclusion: Variance explained by the ETH purchase.
- Status: supported

### s1_c_scope

The reconciliation covers exchange account DEMO-EX-01 at 2026-06-30: the statement balance of 1,250,000.00 USD and the ledger exchange_cash balance of 1,247,350.00 USD, both as of the cutoff.

- Conclusion: Scope and sources established with balances as of the reconciliation date.
- Status: supported

### s2_c_reconciled

The 2,650.00 USD difference is fully explained by a 3,000.00 USD deposit in transit and a 350.00 USD trading fee not yet recorded; nothing is unexplained.

- Conclusion: Reconciled with no unexplained residual.
- Status: supported

### s3_c_flux

Of the three balance sheet lines, only digital assets moved beyond the 100,000.00 USD / 10% thresholds in Q2 2026, and that variance is explained.

- Conclusion: Flux analysis complete; every variance above threshold is explained.
- Status: supported

### s4_c_adjustment

One adjusting entry is proposed: 350.00 USD of exchange fee expense against exchange cash as of 2026-06-30; the deposit in transit is not booked.

- Conclusion: Omission booked through the executable schedule; timing difference carried.
- Status: supported

### s5_c_conclusion

The exchange account is reconciled at 2026-06-30 with one adjustment proposed and no unexplained items; the package is ready for reviewer sign-off.

- Conclusion: Ready for sign-off.
- Status: supported

## Judgments

### s4_j_fee_omission — Booking the unrecorded exchange fee for scope DEMO-EX-01

Is the 350.00 USD trading fee an omission the ledger must record at 2026-06-30, and in which accounts?

- Proposed conclusion: Yes. The fee was charged within the period and reduces the exchange cash balance; it is recorded as exchange fee expense against exchange cash as of 2026-06-30. The 3,000.00 deposit in transit is a timing difference that clears on 2026-07-01 and is not booked.
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
- `reconciliation.balance.schemaValid` — pass — calculations/s2_balance_reconciliation.json
- `reconciliation.balance.recompute` — pass — 1 reconciliation(s): difference, explained and unexplained recompute from balances their facts state as of the reconciliation date
- `reconciliation.balance.itemsSupported` — pass — every reconciling item is supported, errors and omissions are bookable with roles, and any unexplained residual is judged
- `reconciliation.accountingSchedule.tieOut` — pass — 1 entry tie to 1 calculation(s) as reconciling_adjustment in USD
- `reconciliation.flux.schemaValid` — pass — calculations/s3_flux_analysis.json
- `reconciliation.flux.recompute` — pass — 1 flux analysis(es): variances, percentages and totals recompute from the lines
- `reconciliation.flux.variancesExplained` — pass — every variance above either threshold is explained by a claim in the package
