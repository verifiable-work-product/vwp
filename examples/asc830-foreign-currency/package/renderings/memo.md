# ASC 830 Foreign Currency Remeasurement - DemoCo Q2 2026

- Package: `vwp_demo_asc830_foreign_currency`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc830`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DemoCo's functional currency is the US dollar. At 2026-06-30 its EUR cash of 100,000 remeasures from 108,000.00 to 110,000.00 USD (a 2,000.00 gain) and its GBP trade payable of 50,000 from 63,000.00 to 64,000.00 USD (a 1,000.00 loss); the EUR prepayment is non-monetary and stays at 21,600.00 USD. The net transaction gain of 1,000.00 USD is in income.

## Claims

### s1_c_functional_currency

DemoCo's functional currency is the US dollar; its euro and sterling balances are foreign-currency transactions to be remeasured.

- Conclusion: USD functional currency.
- Status: supported

### s2_c_population

The foreign-currency balances at 2026-06-30 are EUR cash and a GBP trade payable (monetary) and a EUR prepayment (non-monetary).

- Conclusion: Population classified.
- Status: supported

### s3_c_rates

The 2026-06-30 spot rates are 1.1000 USD per EUR and 1.2800 USD per GBP.

- Conclusion: Current rates evidenced.
- Status: supported

### s4_c_remeasurement

Remeasuring the monetary items at the 2026-06-30 spot rates produces a 2,000.00 USD gain on EUR cash and a 1,000.00 USD loss on the GBP payable, a net transaction gain of 1,000.00 USD, booked by the executable schedule.

- Conclusion: Remeasurement recomputes and books.
- Status: supported

### s5_c_presentation

The net transaction gain of 1,000.00 USD is presented in income and the aggregate transaction gain or loss is disclosed; no foreign entity exists, so no translation adjustment arises.

- Conclusion: Presentation and disclosure per ASC 830-20-45 and 830-20-50.
- Status: supported

## Judgments

### s1_j_functional_currency — Functional currency

What is DemoCo's functional currency under ASC 830-10-45-2 to 45-6?

- Proposed conclusion: The US dollar. The cash flow, sales price, sales market, expense, financing and intra-entity indicators of ASC 830-10-55-5 all point to the US dollar (s1_f_functional_indicators); the euro and sterling balances arise from isolated transactions that are remeasured, not translated.
- Status: reviewed

### s2_j_monetary_classification — Monetary versus non-monetary items

Which foreign-currency balances are monetary and remeasured at the current rate?

- Proposed conclusion: Cash and the trade payable are monetary (fixed amounts of currency receivable or payable) and remeasure at the 2026-06-30 spot rates; the prepayment is non-monetary and stays at the historical rate (ASC 830-10-45-18 and the 830-10-20 definition of monetary assets and liabilities).
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
- `core.factAnchors.quotesMatch` — pass — 6 quote anchors verified
- `core.claimRefs.resolve` — pass
- `core.graph.refsResolve` — pass
- `core.graph.acyclic` — pass
- `core.graph.declaredInputs` — pass
- `core.digests.match` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `asc830.remeasurement.schemaValid` — pass — calculations/s4_remeasurement.json
- `asc830.remeasurement.recompute` — pass — 1 remeasurement(s): every monetary item remeasures at its spot rate and the net gain or loss sums
- `asc830.remeasurement.rateEvidence` — pass — every spot rate equals the rate its fact states as of the remeasurement date
- `asc830.remeasurement.monetaryClassified` — pass — every item is classified monetary or non-monetary, non-monetary items carry no difference, and the functional currency is judged
- `asc830.accountingSchedule.tieOut` — pass — 2 entries tie to 1 calculation(s) as fx_remeasurement in USD
