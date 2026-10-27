# ASC 450 Loss Contingencies and Going Concern - DemoCo at 2026-06-30

- Package: `vwp_demo_asc450_contingencies`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc450`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

Of three loss contingencies at 2026-06-30, the supplier litigation is probable and estimable in a range of 400,000 to 900,000 USD with no better estimate within it, so 400,000 USD is accrued; the regulatory inquiry is reasonably possible (disclosed, not accrued); the guarantee of an affiliate's facility is remote (disclosed). Management's evaluation finds no substantial doubt about DemoCo's ability to continue as a going concern within one year of issuance.

## Claims

### s1_c_population

Three loss contingencies exist at 2026-06-30: the supplier litigation, the regulatory inquiry and the guarantee of the affiliate's facility, each supported by counsel's letter or the guarantee agreement.

- Conclusion: Population complete from counsel's letter and the contracts register.
- Status: supported

### s2_c_likelihood

The litigation loss is probable and estimable; the regulatory loss is reasonably possible; the guarantee loss is remote.

- Conclusion: Likelihood concluded per item from counsel's assessment.
- Status: supported

### s3_c_measurement

400,000 USD is accrued for the supplier litigation, the minimum of counsel's range because no amount within it is a better estimate; nothing is accrued for the other two items.

- Conclusion: Accrual at the low end of the range; the register and the accrual schedule carry it.
- Status: supported

### s4_c_going_concern

Conditions and events, in the aggregate, do not raise substantial doubt about DemoCo's ability to continue as a going concern within one year after the issuance date: forecast cash stays above 2,820,000 USD and no debt matures.

- Conclusion: No substantial doubt; no ASC 205-40-50-12 disclosure.
- Status: supported

### s5_c_disclosure

The nature of each contingency, the 400,000 USD accrued, the exposure to loss above the accrual up to 900,000 USD, the 50,000 to 250,000 USD reasonably possible regulatory penalty and the 1,000,000 USD guarantee are disclosed.

- Conclusion: Disclosure per ASC 450-20-50-1 to 50-6.
- Status: supported

## Judgments

### s2_j_litigation — Likelihood and estimability of the supplier litigation

Is a loss on the supplier litigation probable and reasonably estimable at 2026-06-30, and what amount is accrued?

- Proposed conclusion: Probable and estimable. Counsel assesses an unfavorable outcome as probable and the loss as a range of 400,000 to 900,000 USD with no better estimate within it; ASC 450-20-30-1 accrues the minimum of the range, 400,000 USD, and the exposure above it is disclosed.
- Status: reviewed

### s2_j_regulatory — Likelihood of the regulatory inquiry

Is a loss on the regulatory inquiry probable at 2026-06-30?

- Proposed conclusion: Reasonably possible, not probable: counsel does not assess an adverse finding as likely. No accrual; the nature of the inquiry and the 50,000 to 250,000 USD range are disclosed.
- Status: reviewed

### s2_j_guarantee — Likelihood of a loss under the affiliate guarantee

Is a loss under the guarantee of the affiliate's facility probable or reasonably possible at 2026-06-30?

- Proposed conclusion: Remote: the facility is undrawn and the affiliate is in compliance. Remote guarantee-type contingencies are nonetheless disclosed (ASC 450-20-50-6), with the maximum exposure of 1,000,000 USD; the guarantee's own recognition under ASC 460 is addressed separately.
- Status: reviewed

### s4_j_going_concern — Going concern evaluation

Do conditions or events, considered in the aggregate, raise substantial doubt about DemoCo's ability to continue as a going concern within one year after the date the financial statements are issued?

- Proposed conclusion: No. The approved twelve-month cash forecast keeps closing cash above 2,820,000 USD throughout, the accrued litigation loss is covered several times over, and no debt matures in the period; no conditions raising substantial doubt were identified, so no mitigation plans need to be evaluated and no disclosure under ASC 205-40-50-12 is required.
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
- `core.factAnchors.quotesMatch` — pass — 4 quote anchors verified
- `core.claimRefs.resolve` — pass
- `core.graph.refsResolve` — pass
- `core.graph.acyclic` — pass
- `core.graph.declaredInputs` — pass
- `core.digests.match` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `asc450.register.present` — pass — 3 contingencies in register
- `asc450.register.schemaValid` — pass
- `asc450.register.judgmentLinked` — pass — every contingency links a resolvable judgment and its evidence resolves
- `asc450.register.accrualConsistent` — pass — accruals follow ASC 450-20-25-2 and 30-1 from each item's likelihood, estimability and range
- `asc450.register.disclosureConsistent` — pass — disclosure follows ASC 450-20-50 from each item's likelihood and type
- `asc450.goingConcern.judged` — pass — the going concern evaluation is a judgment and a claim citing ASC 205-40 concludes on it
- `asc450.accountingSchedule.tieOut` — pass — 1 entry tie to 1 calculation(s) as contingency_accrual in USD
