# ASC 606 Revenue Recognition Workpaper - Acme Contract

- Package: `vwp_demo_acme_asc606_2026q1`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc606`
- Status: draft

> Derived rendering. The structured package is the source of truth.

## Conclusion

Draft revenue recognition treatment prepared for review. Transaction price of 300,000 USD allocated across three performance obligations on a relative standalone selling price basis. Whether implementation services are distinct remains subject to review.

## Claims

### c_step1_contract_identified

A contract with a customer exists under ASC 606-10-25-1: the arrangement is approved, identifies each party's rights and payment terms, has commercial substance, and collection is probable.

- Conclusion: A contract exists as of 2026-01-01.
- Status: supported

### c_step2_performance_obligations

The contract contains three promised goods and services: hosted SaaS subscription, implementation services, and premium support. The SaaS subscription and premium support are distinct. Whether implementation services are distinct from the SaaS subscription requires professional judgment.

- Conclusion: Three performance obligations, subject to review of the implementation-services distinctness judgment.
- Status: proposed

### c_step3_transaction_price

The transaction price is 300,000 USD. There is no variable consideration, significant financing component, noncash consideration, or consideration payable to the customer identified in the demo arrangement.

- Conclusion: Transaction price is 300,000 USD.
- Status: supported

### c_step4_allocation

The transaction price is allocated to the performance obligations on a relative standalone selling price basis: 192,000 USD to the SaaS subscription, 60,000 USD to implementation services, and 48,000 USD to premium support.

- Conclusion: Allocation ties to the relative SSP calculation and sums to the transaction price.
- Status: supported

### c_step5_recognition

Revenue allocated to the SaaS subscription and premium support is recognized ratably over the 24-month term. Revenue allocated to implementation services is recognized at completion of implementation, expected 2026-03-31, subject to the distinctness judgment.

- Conclusion: Draft recognition pattern per the revenue schedule; pending judgment review.
- Status: proposed

## Judgments

### j_impl_distinct — Performance obligation identification

Are the implementation services distinct from the hosted SaaS subscription under ASC 606-10-25-19 through 25-21?

- Proposed conclusion: Distinct. The implementation services are routine configuration, data migration, and training; the hosted service is a standard offering that is not materially modified, and other vendors could perform comparable implementation work.
- Status: reviewRequired

## Validations

- `core.manifest.schemaValid` — pass — Manifest conforms to schemas/v0.1/manifest.schema.json.
- `asc606.allocation.tieOut` — pass — Allocated amounts (192,000 + 60,000 + 48,000) equal the transaction price of 300,000 USD.
- `asc606.revenueSchedule.total` — pass — Revenue schedule total of 300,000 USD equals allocated consideration.
- `core.requiredOutputs.exist` — pass — The SSP allocation, recognition CSV, and version-1 accounting schedule are present at their canonical paths and declared as hashed calculation parts.
- `accountingSchedule.valid` — pass — The version-1 accounting schedule has explicit dates, exact decimal amounts, and balanced posting lines.
- `asc606.accountingSchedule.tieOut` — pass — Accounting schedule amounts match the SSP allocation and every obligation/month amount in the recognition CSV.
- `finance.requiredJudgments.complete` — reviewRequired — Judgment j_impl_distinct (implementation services distinctness) requires review by revenue-manager.

## Review actions

- acknowledged by Demo Preparer (preparer) at 2026-07-07T15:05:00Z — scope: package — Draft prepared and submitted for review. Judgment j_impl_distinct pending revenue-manager review.

## Analysis attestations

### att-001

- Performed by: demo-reviewer (human) — model demo-challenger/challenger-model-mini 2026-06-01
- Evidence examined: 2
- Claims challenged: 1

**Findings (2)**

- [medium] att-001-f1 — MSA Section 3.2 includes data migration in the implementation services, but the excerpt does not state whether Customer can use the hosted service before migration completes. If go-live depends on migration, the customer may not benefit from the subscription on its own, which weighs against distinctness. The basis recorded in j_impl_distinct does not address this dependency. (claims: [c_step2_performance_obligations])
- [low] att-001-f2 — The examined MSA is an excerpt covering only Sections 3 and 9. Clauses elsewhere in the full agreement (e.g. customization or integration commitments) could contradict Section 3.1's statement that the hosted service is not materially modified; they were not available to this procedure. (claims: [c_step2_performance_obligations])

**Limitations**

- Only the two scoped evidence files were examined; the price book and the full Master Subscription Agreement were out of scope.
- Whether implementation services are distinct is a professional judgment; this procedure surfaces contrary indicators, it does not settle the question.

**Dispositions**

- att-001-f1: acknowledged by demo-revenue-manager — Routed into the open j_impl_distinct judgment review; the go-live dependency point will be addressed in the judgment basis.
- att-001-f2: remediated by demo-revenue-manager — Full agreement reviewed in the demo contract system; no customization or integration commitments beyond Section 3.
