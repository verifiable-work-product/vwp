# Staking Rewards Recognition - DemoCo ETH Validator April 2026

- Package: `vwp_demo_staking_rewards`
- Profile: `vwp-finance`
- Extension: `vwp-digital-assets-staking-rewards`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DemoCo operates its own validator and is the principal; rewards are recognized as other income when received, measured at the receipt-date fair value. April 2026 rewards of 1.5 ETH tie to the beacon-chain record and total 3,750 USD.

## Claims

### s1_c_arrangement

DemoCo self-operates an Ethereum validator with its own stake and keys and shares no rewards with any provider.

- Conclusion: Arrangement facts established.
- Status: supported

### s2_c_principal

DemoCo is the principal; with no customer identified, ASC 606 is applied by analogy and the rewards are presented as other income.

- Conclusion: Principal; ASC 606 by analogy; other income caption.
- Status: supported

### s3_c_timing

Rewards are recognized when credited to the DemoCo-controlled withdrawal address, which coincides with the end of each duty period, and measured at that date's price.

- Conclusion: Recognize on credit; recognition-date price.
- Status: supported

### s4_c_measurement

April 2026 rewards of 1.5 ETH are measured at recognition-date fair value totalling 3,750 USD and tie to the beacon-chain record.

- Conclusion: Schedule recomputes and ties to chain.
- Status: supported

### s5_c_subsequent

Received ETH rewards are in-scope crypto assets subsequently measured at fair value under ASC 350-60, with remeasurement in net income.

- Conclusion: Subsequent accounting under ASC 350-60.
- Status: supported

## Judgments

### s2_j_principal — Customer identification, principal versus agent and nature of the reward

Is there a customer for DemoCo's validation service, is DemoCo the principal, and is the reward revenue under ASC 606 (directly or by analogy) or other income?

- Proposed conclusion: DemoCo is the principal (it controls the validator and bears slashing risk). No delegators or protocol counterparty contract for the service, so ASC 606 is applied by analogy for recognition and noncash-consideration measurement, with the reward presented as other income rather than revenue from contracts with customers.
- Status: reviewed

### s3_j_timing — Recognition timing and pricing date of validator rewards

Are rewards recognized as validation duties are performed (over time, with the variable-consideration constraint), when credited to the withdrawal address, or when claimed — and which date's price measures them?

- Proposed conclusion: Recognized when the reward is credited and withdrawable to the DemoCo-controlled address, which on this chain coincides with the end of each duty period: the amount is then fixed and the asset is controlled. Each reward is measured at the fair value on that recognition date (pricingDateBasis recognitionDate); subsequent price movements are ASC 350-60 remeasurement, not staking income. No reward straddles the 2026-04-30 reporting boundary.
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
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `staking.rewards.schemaValid` — pass — calculations/s4_rewards_schedule.json
- `staking.rewards.recompute` — pass — 1 schedule(s) recompute from units × price
- `staking.rewards.chainTieOut` — pass — recognized units reconcile to the chain-observed receipts stated by the cited facts
- `staking.rewards.pricingEvidence` — pass — every reward is priced as of the date its pricing basis fixes, by a fact stating that price on that date
- `staking.rewards.population` — pass — rewards lie in their periods, references are unique and schedules do not overlap
- `staking.accountingSchedule.tieOut` — pass — 3 entries tie to 1 calculation(s) as staking_rewards in USD
