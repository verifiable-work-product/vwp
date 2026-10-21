# Cost Basis and Lot Relief - DemoCo BTC Q1 2025

- Package: `vwp_demo_cost_basis_lot_relief`
- Profile: `vwp-finance`
- Extension: `vwp-digital-assets-cost-basis`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DemoCo elects FIFO lot relief applied wallet by wallet. The 190,000 USD of unused basis on 4 BTC held at 2025-01-01 (all acquired in 2024) is allocated across the two wallets under the Rev. Proc. 2024-28 specific unit allocation recorded on 2024-12-20; the 2.5 BTC disposal on 2025-03-20 then relieves lots L1 and half of L2 for 105,000 USD of basis and a 57,500 USD realized gain.

## Claims

### s1_c_policy

DemoCo's cost basis method is FIFO applied wallet by wallet, disclosed under ASC 350-60-50-2.

- Conclusion: Method elected and documented.
- Status: supported

### s2_c_lots

The lot population at 2025-01-01 is three lots acquired in 2024 totalling 4 BTC with 190,000 USD of basis: L1 and L2 in W-A, L3 in W-B.

- Conclusion: Lot population complete from the subledger.
- Status: supported

### s3_c_relief

Disposal D1 relieves L1 in full and 0.5 BTC of L2 under FIFO within wallet W-A, for 105,000 USD of basis; wallet W-B had no disposals.

- Conclusion: Relief follows FIFO within the wallet and conserves units.
- Status: supported

### s4_c_gain

The realized gain on D1 is 57,500 USD (proceeds 162,500 less basis 105,000).

- Conclusion: Gain recomputes.
- Status: supported

### s5_c_wallet_allocation

Under the Rev. Proc. 2024-28 specific unit allocation recorded on 2024-12-20, the 190,000 USD of unused basis on 4 BTC held at 2025-01-01 is allocated 130,000 USD to W-A (3 BTC) and 60,000 USD to W-B (1 BTC); no unit acquired on or after 2025-01-01 is in the allocation.

- Conclusion: Allocation ties to the pre-2025 lot population.
- Status: supported

## Judgments

### s1_j_method — Lot relief method election

Which lot relief method applies, and is it applied per wallet or across all wallets?

- Proposed conclusion: FIFO applied wallet by wallet, consistent with the policy and with the Treas. Reg. 1.1012-1(j) wallet-based identification requirement for tax; the same method is used for book to avoid a book-tax lot divergence.
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
- `costBasis.lotRelief.schemaValid` — pass — calculations/s3_lot_relief.json, calculations/s3_lot_relief_wb.json
- `costBasis.lotRelief.unitsConserved` — pass — 2 schedule(s): units conserved from lots through reliefs to remaining lots
- `costBasis.lotRelief.recompute` — pass — basis relieved and realized gain/loss recompute from the lots
- `costBasis.lotRelief.methodOrder` — pass — reliefs follow the declared method order
- `costBasis.accountingSchedule.tieOut` — pass — 1 entry tie to 2 calculation(s) as realized_gain_loss in USD
- `costBasis.walletAllocation.schemaValid` — pass — calculations/s5_wallet_allocation.json
- `costBasis.walletAllocation.tieOut` — pass — 1 allocation(s) tie to their declared basis and units
- `costBasis.walletAllocation.eligibility` — pass — allocation equals the units and basis of lots acquired before 2025-01-01
