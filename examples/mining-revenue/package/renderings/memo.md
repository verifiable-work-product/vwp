# Mining Revenue Recognition - DemoCo Solo Mining April 2026

- Package: `vwp_demo_mining_revenue`
- Profile: `vwp-finance`
- Extension: `vwp-digital-assets-mining-revenue`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DemoCo mines with its own hash power and receives the coinbase output directly; it is the principal and, with no customer identified, ASC 606 is applied by analogy with rewards presented as mining revenue. April 2026 rewards of 9.545 BTC (three blocks, subsidy plus fees) tie to the chain record and total 677,725 USD at recognition-date prices.

## Claims

### s1_c_arrangement

DemoCo mines solo with its own fleet, receives every coinbase output to an address it controls, and uses no pool and sells no hash power.

- Conclusion: Arrangement facts established.
- Status: supported

### s2_c_principal

DemoCo is the principal for the mining service; with no customer identified, ASC 606 is applied by analogy and block rewards are presented as mining revenue.

- Conclusion: Principal; ASC 606 by analogy; mining revenue caption.
- Status: supported

### s3_c_timing

Block rewards are recognized on the date the block is accepted and its coinbase output pays to the DemoCo-controlled address, measured at that date's price; the 100-block maturity is a spending restriction, not a recognition criterion.

- Conclusion: Recognize on block acceptance; recognition-date price.
- Status: supported

### s4_c_measurement

April 2026 block rewards of 9.545 BTC (subsidy 9.375 plus fees 0.17) are measured at recognition-date fair value totalling 677,725 USD and tie to the node export.

- Conclusion: Schedule recomputes and ties to chain.
- Status: supported

### s5_c_subsequent

Mined BTC is an in-scope crypto asset subsequently measured at fair value under ASC 350-60 with remeasurement in net income; mining revenue is presented in its own caption with the fleet's energy and depreciation as cost of revenue.

- Conclusion: Subsequent accounting under ASC 350-60; separate revenue caption.
- Status: supported

## Judgments

### s2_j_principal — Customer identification, principal versus agent and nature of the reward

Is there a customer for DemoCo's mining service, is DemoCo the principal, and is the block reward revenue under ASC 606 (directly or by analogy) or other income?

- Proposed conclusion: DemoCo is the principal: it controls the hash power, builds its own block templates and bears the cost and risk of finding no block. No pool or hosting counterparty contracts for the service and the protocol is not a party that can be invoiced, so ASC 606 is applied by analogy for recognition and noncash-consideration measurement, with the reward presented as mining revenue in a separate caption.
- Status: reviewed

### s3_j_timing — Recognition timing and pricing date of block rewards

Is a block reward recognized when the block is found and the coinbase output pays to DemoCo's address, when the output matures, or when it is spent — and which date's price measures it?

- Proposed conclusion: Recognized on the date the block is accepted and its coinbase output pays to the DemoCo-controlled address: the amount is fixed by the protocol and the asset is DemoCo's, with the 100-block maturity a spending restriction rather than a condition of entitlement; a reorganization that orphans the block before maturity reverses the reward (none occurred). Each reward is measured at the fair value on that recognition date (pricingDateBasis recognitionDate); subsequent price movements are ASC 350-60 remeasurement, not mining revenue. No block straddles the 2026-04-30 reporting boundary.
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
- `core.factAnchors.quotesMatch` — pass — 7 quote anchors verified
- `core.claimRefs.resolve` — pass
- `core.graph.refsResolve` — pass
- `core.graph.acyclic` — pass
- `core.graph.declaredInputs` — pass
- `core.digests.match` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `mining.rewards.schemaValid` — pass — calculations/s4_mining_schedule.json
- `mining.rewards.recompute` — pass — 1 schedule(s) recompute from units × price
- `mining.rewards.chainTieOut` — pass — recognized units reconcile to the chain-observed receipts stated by the cited facts
- `mining.rewards.pricingEvidence` — pass — every reward is priced as of the date its pricing basis fixes, by a fact stating that price on that date
- `mining.rewards.population` — pass — rewards lie in their periods, references are unique and schedules do not overlap
- `mining.accountingSchedule.tieOut` — pass — 3 entries tie to 1 calculation(s) as mining_revenue in USD
