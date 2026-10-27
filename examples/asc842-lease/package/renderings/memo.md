# ASC 842 Lease Accounting - DemoCo Office Lease OFF-001

- Package: `vwp_demo_asc842_lease`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc842`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DemoCo's 36-month office lease is an operating lease: none of the ASC 842-10-25-2 criteria is met. The 36 payments of 10,000 USD in arrears discount at the 6% incremental borrowing rate to a lease liability of 328,710.16 USD at commencement, with an equal right-of-use asset; the single straight-line lease cost is 10,000 USD a month and the liability retires with the final payment.

## Claims

### s1_c_lease

The agreement is a lease: it conveys the right to control the use of an identified office suite for 36 months in exchange for consideration.

- Conclusion: A lease under ASC 842-10-15-3 exists.
- Status: supported

### s2_c_classification

Lease OFF-001 is an operating lease: none of the ASC 842-10-25-2 criteria is met.

- Conclusion: Operating lease.
- Status: supported

### s3_c_measurement_inputs

The lease payments are 36 monthly payments of 10,000.00 USD in arrears and the discount rate is the 6.00% incremental borrowing rate.

- Conclusion: Measurement inputs established.
- Status: supported

### s4_c_schedules

The lease liability at commencement is 328710.16 USD with an equal right-of-use asset; the liability roll-forward and the straight-line lease cost of 10,000.00 USD a month recompute period by period and the executable schedule books them.

- Conclusion: Initial measurement and the period schedule recompute.
- Status: supported

### s5_c_presentation

The operating lease right-of-use asset and lease liability are presented separately from finance leases, lease cost is a single operating line, and the maturity analysis and weighted-average term and rate are disclosed.

- Conclusion: Presentation and disclosure per ASC 842-20-45 and 50.
- Status: supported

## Judgments

### s2_j_classification — Lease classification

Is lease OFF-001 a finance lease or an operating lease under ASC 842-10-25-2?

- Proposed conclusion: Operating lease. None of the five criteria in ASC 842-10-25-2 is met: title does not transfer, there is no purchase option, the 3-year term (s1_f_term) is 15% of the 20-year remaining economic life (s2_f_economic_life) and not a major part, the present value of the payments (328,710.16 USD) is 16% of the 2,000,000.00 USD fair value (s2_f_fair_value) and not substantially all, and general-purpose office space is not specialized.
- Status: reviewed

### s3_j_discount_rate — Discount rate

Which discount rate measures the lease liability?

- Proposed conclusion: The 6.00% incremental borrowing rate (s3_f_rate): the rate implicit in the lease is not readily determinable, and the collateralized 3-year borrowing rate for a similar amount and term is the ASC 842-20-30-3 rate.
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
- `asc842.lease.schemaValid` — pass — calculations/s4_lease_schedule.json
- `asc842.lease.pvRecompute` — pass — 1 lease(s): initial liability is the present value of the payments and the right-of-use asset follows from it
- `asc842.lease.rollForward` — pass — every period's interest, principal, closing liability, lease cost and amortization recompute and the liability retires at the end of the term
- `asc842.lease.classificationJudged` — pass — every lease's classification rests on a judgment in the package
- `asc842.lease.rateEvidence` — pass — every discount rate equals the value of the fact it cites
- `asc842.accountingSchedule.tieOut` — pass — 37 entries tie to 1 calculation(s) as lease_cost in USD
