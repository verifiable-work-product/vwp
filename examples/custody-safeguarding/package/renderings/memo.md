# Crypto Custody and Safeguarding - DemoCo Custodied Assets at 2026-06-30

- Package: `vwp_demo_custody_safeguarding`
- Profile: `vwp-finance`
- Extension: `vwp-digital-assets-custody-safeguarding`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DemoCo safeguards 100 BTC and 2,000 ETH for customers (6,500,000 and 4,500,000 USD) under an obligation to return them; following SAB 122 no safeguarding liability or asset is recorded for assets held, the arrangements and the controls are disclosed, and the 10 ETH of customer assets lost in the 2026-06-12 hot-wallet incident is a probable, estimable loss accrued at 22,500 USD.

## Claims

### s1_c_population

DemoCo safeguards 100 BTC in an HSM-keyed omnibus wallet it controls and 2,000 ETH placed with a sub-custodian under its instruction, for 420 customers.

- Conclusion: Custodied populations and key control established.
- Status: supported

### s2_c_obligation

DemoCo has a safeguarding obligation for both populations; under SAB 122 holding them creates no liability or asset, and ASC 450-20 governs the risk of loss.

- Conclusion: Safeguarding obligation; SAB 122 model; no gross-up.
- Status: supported

### s3_c_valuation

Custodied assets are valued for disclosure at 6,500,000 USD (BTC) and 4,500,000 USD (ETH) at 2026-06-30 prices, with the 10 ETH lost valued at 22,500 USD.

- Conclusion: Valuation recomputes from dated price facts.
- Status: supported

### s4_c_contingency

A 22,500 USD loss for the 10 ETH lost on 2026-06-12 is probable and estimable and is accrued; no loss is probable for the assets still held, whose risk is reasonably possible and disclosed.

- Conclusion: One accrual; disclosure for the rest.
- Status: supported

### s5_c_disclosure

The nature and amount of custodied assets, the custody and sub-custody arrangements, the controls over keys, the incident and the accrued loss are disclosed; customer assets are not presented on DemoCo's balance sheet.

- Conclusion: SAB 122 and ASC 450-20-50 disclosures; off-balance-sheet customer assets.
- Status: supported

## Judgments

### s2_j_safeguarding_obligation — Safeguarding obligation and SAB 122 treatment of assets held

Does DemoCo have an obligation to safeguard the customer BTC and ETH, and does holding them give rise to a liability or an asset on DemoCo's balance sheet?

- Proposed conclusion: Yes, DemoCo is obligated to return the assets or their value (clause 3.1) and remains obligated for the ETH placed with the sub-custodian (clause 2.2), so it safeguards both populations. Under SAB 122, which rescinded SAB 121's requirement to recognize a safeguarding liability and corresponding asset, holding the assets is not itself a liability: DemoCo applies ASC 450-20 to the risk of loss from safeguarding and discloses the nature and amount of the assets safeguarded, the controls and the risks. The customer assets are not DemoCo's assets and are not recognized.
- Status: reviewed

### s4_j_incident_loss — Loss contingency for the hot-wallet incident

Is a loss from the 2026-06-12 incident probable and reasonably estimable, and at what amount is it accrued?

- Proposed conclusion: Yes. Recovery has failed and clause 7.2 obliges DemoCo to make the affected customers whole, so a loss is probable; it is estimable at the 10 ETH lost measured at the 2026-06-30 price of 2,250 USD, 22,500 USD, with no insurance recovery asserted. The accrual is recorded as a safeguarding loss against a liability to the customers, remeasured each period until settled in ETH or its value.
- Status: reviewed

### s4_j_no_loss_held — Loss contingency for the assets held in custody

Is a loss from safeguarding the 100 BTC and 2,000 ETH still held probable or reasonably possible at 2026-06-30?

- Proposed conclusion: No loss is probable: the BTC keys are in DemoCo HSMs with a verified proof of control and the ETH sits segregated with the sub-custodian; no incident affects these populations. A loss is at most reasonably possible given the inherent risks of key custody, so no accrual is recorded and the risk is disclosed with the safeguarded amounts.
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
- `core.factAnchors.quotesMatch` — pass — 9 quote anchors verified
- `core.claimRefs.resolve` — pass
- `core.graph.refsResolve` — pass
- `core.graph.acyclic` — pass
- `core.graph.declaredInputs` — pass
- `core.digests.match` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `custody.register.present` — pass — 3 custodied populations in register
- `custody.register.schemaValid` — pass
- `custody.register.judgmentLinked` — pass — every holding links a judgment and accrues a liability exactly when its loss is probable and estimable
- `custody.register.valuationRecompute` — pass — every holding's fair value is units × a price its fact states as of 2026-06-30; custodied assets total 11022500
- `custody.register.controlEvidence` — pass — every population the entity must return cites resolvable control evidence
- `custody.accountingSchedule.tieOut` — pass — 1 entry tie to 1 calculation(s) as safeguarding_liability in USD
