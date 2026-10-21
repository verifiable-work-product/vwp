# ASC 350-60 Crypto Asset Accounting - DemoCo Q2 2026

- Package: `vwp_demo_asc350_60_crypto_assets`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc350-60`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

BTC and ETH are in scope of ASC 350-60 and measured at fair value; the self-issued WDEMO token is out of scope. The Q2 roll-forward ties from 1,850,000 USD to 1,550,000 USD and both holdings are significant for disclosure.

## Claims

### s1_c_scope

BTC and ETH meet all six ASC 350-60-15-1 criteria and are accounted for under ASC 350-60; WDEMO is excluded as a self-issued token.

- Conclusion: Scope register complete; one asset out of scope with a judgment.
- Status: supported

### s2_c_measurement

In-scope crypto assets were initially measured at cost including transaction costs under ASC 350-30-30-1 and are subsequently measured at fair value under ASC 350-60-35-1 using principal-market quoted prices at 2026-06-30: BTC 650,000 USD and ETH 900,000 USD.

- Conclusion: Initial measurement per other GAAP; subsequent Level 1 fair value from the principal market.
- Status: supported

### s3_c_roll_forward

The Q2 2026 crypto asset roll-forward reconciles opening 1,850,000 USD to closing 1,550,000 USD through 250,000 USD of dispositions, 50,000 USD of gains and 100,000 USD of losses.

- Conclusion: Roll-forward ties per asset and in total.
- Status: supported

### s4_c_presentation

Crypto assets are presented separately from other intangible assets on the balance sheet, and remeasurement gains and losses are presented separately in net income.

- Conclusion: Presentation follows ASC 350-60-45-1 and 45-2.
- Status: supported

### s5_c_disclosure

Both holdings are significant and are disclosed with units, cost basis and fair value; no holding is subject to a contractual sale restriction; the cost basis method for realized gains and losses is FIFO.

- Conclusion: Significant holdings table ties to total fair value and to the roll-forward.
- Status: supported

## Judgments

### s1_j_wdemo_scope — ASC 350-60 scope of the self-issued WDEMO token

Is WDEMO, a token DemoCo created and issued, a crypto asset within ASC 350-60-15-1?

- Proposed conclusion: No. WDEMO fails criterion (f): it was created by the reporting entity. It is not recognized as an asset of the issuer; the issuance is analysed separately.
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
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `asc35060.scope.registerPresent` — pass — 3 assets in scope register
- `asc35060.scope.schemaValid` — pass
- `asc35060.scope.consistent` — pass — 3 assets: inScope agrees with the criteria; out-of-scope assets name their treatment
- `asc35060.scope.judgmentLinked` — pass — every out-of-scope asset links a resolvable scoping judgment
- `asc35060.rollForward.schemaValid` — pass — calculations/s3_roll_forward.json
- `asc35060.rollForward.tieOut` — pass — 1 roll-forward(s) tie and chain by scope and period
- `asc35060.rollForward.assetsInScope` — pass — every roll-forward line is an in-scope asset
- `asc35060.accountingSchedule.tieOut` — pass — 2 entries tie to 1 calculation(s) as fair_value_remeasurement in USD
- `asc35060.significantHoldings.schemaValid` — pass — calculations/s5_significant_holdings.json
- `asc35060.significantHoldings.tieOut` — pass — significant and other fair value and cost basis recompute from the holdings; total fair value 1550000
- `asc35060.significantHoldings.assetsInScope` — pass — every holding is an in-scope asset
- `asc35060.significantHoldings.tiesToRollForward` — pass — every holdings table reconciles per asset to the roll-forward closing on its date; total 1550000
