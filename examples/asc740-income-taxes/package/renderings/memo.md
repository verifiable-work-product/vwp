# ASC 740 Income Tax Provision - DemoCo FY2026

- Package: `vwp_demo_asc740_income_taxes`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc740`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

At a 25% statutory rate on 1,000,000 USD of pretax income, DemoCo's FY2026 provision is 247,500 USD: current tax 235,000 on taxable income of 940,000 after permanent and temporary differences, a 10,000 increase in unrecognized tax benefits, and deferred tax expense of 2,500 as the net deferred position moves from a 25,000 liability to a 27,500 liability (the 20,000 valuation allowance released as the loss carryforward is used). Effective rate 24.75%.

## Claims

### s1_c_scope

DemoCo files in one jurisdiction at an enacted 25% rate with no enacted change; the provision covers FY2026 under ASC 740.

- Conclusion: Single jurisdiction, enacted rate established.
- Status: supported

### s2_c_current

Taxable income is 940,000 USD after 30,000 USD of permanent differences and the net 90,000 USD reduction from temporary differences, giving current tax expense of 235,000 USD.

- Conclusion: Current tax recomputes from pretax income and the differences.
- Status: supported

### s3_c_deferred

Deferred tax assets fall from 45,000 to 37,500 USD and deferred tax liabilities rise from 50,000 to 65,000 USD at 25%, moving the net deferred position from a 25,000 USD liability to a 27,500 USD liability and producing 2,500 USD of deferred tax expense.

- Conclusion: Deferred balances recompute from the cumulative differences at the enacted rate.
- Status: supported

### s4_c_allowance_utb

The 20,000 USD valuation allowance is released because realization of the remaining deferred tax asset is more likely than not; unrecognized tax benefits of 25,000 USD are carried as a liability, 10,000 USD of which arose in FY2026.

- Conclusion: Allowance released; uncertain position measured and presented as a liability.
- Status: supported

### s5_c_rate_reconciliation

The 247,500 USD provision reconciles from the 250,000 USD statutory expectation through permanent differences (7,500), the allowance release (-20,000) and unrecognized tax benefits (10,000), an effective rate of 24.75%; the provision entries book current tax, the unrecognized-benefit liability and the deferred movement.

- Conclusion: Rate reconciliation ties; provision entries derived from the calculation.
- Status: supported

## Judgments

### s4_j_valuation_allowance — Valuation allowance on deferred tax assets

Is it more likely than not that some portion of the deferred tax assets will not be realized at 2026-12-31?

- Proposed conclusion: No. The 20,000 USD allowance carried at 2025-12-31 against the loss carryforward is released: the carryforward was fully used against FY2026 taxable income, the remaining deferred tax asset (accrued compensation, 37,500 USD) reverses as the compensation is paid, and taxable temporary differences of 260,000 USD provide a source of future taxable income exceeding it. No allowance is required at 2026-12-31.
- Status: reviewed

### s4_j_uncertain_positions — Unrecognized tax benefits

Which tax positions taken in the return fail the more-likely-than-not recognition threshold, and what liability is measured for them?

- Proposed conclusion: One position (the deductibility of a cross-border service charge) does not meet the more-likely-than-not threshold on its technical merits; the full benefit of 25,000 USD at 2026-12-31 (15,000 USD at 2025-12-31) is unrecognized and carried as a liability, with the 10,000 USD increase in FY2026 tax expense. No interest or penalties accrue yet.
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
- `asc740.provision.schemaValid` — pass — calculations/s3_tax_provision.json
- `asc740.provision.recompute` — pass — 1 provision(s): taxable income, current, deferred and total tax recompute from the differences and the pretax income fact
- `asc740.provision.deferredRollForward` — pass — deferred tax assets and liabilities recompute from the cumulative differences at the enacted rates; the allowance never exceeds the gross asset
- `asc740.provision.rateReconciliation` — pass — the rate reconciliation starts from the statutory expectation and sums to the total provision
- `asc740.provision.valuationAllowanceJudged` — pass — valuation allowance and uncertain tax position conclusions rest on resolvable judgments
- `asc740.accountingSchedule.tieOut` — pass — 3 entries tie to 1 calculation(s) as income_tax_provision in USD
