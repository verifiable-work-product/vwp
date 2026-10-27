# DeFi Positions - DemoCo at 2026-06-30

- Package: `vwp_demo_defi_positions`
- Profile: `vwp-finance`
- Extension: `vwp-digital-assets-defi-positions`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

The DemoSwap LP position is a new asset (the LP token) recognized on contribution: control of the ETH and DUSD passed to the pool, so they were derecognized; it is valued through its pro-rata underlyings at 1,125,000 USD. The DemoLend deposit of 100 ETH was derecognized for a receipt token conveying a right to equivalent ETH, a financial-asset-like claim measured at the ETH fair value of 250,000 USD. The wrapped BTC conveys a redemption right to BTC, failing ASC 350-60-15-1(b); it is an indefinite-lived intangible under ASC 350-30 valued for disclosure at 650,000 USD.

## Claims

### s1_c_population

DemoCo holds three DeFi positions in the treasury-defi wallet it controls: a DemoSwap LP position, a DemoLend ETH deposit and 10 WBTC-D.

- Conclusion: Position population established from the protocol query and wallet control evidence.
- Status: supported

### s2_c_treatment

The LP contribution and the lending deposit surrendered control and were derecognized for new assets; the wrapped token is a claim on the custodian, not a crypto asset under ASC 350-60.

- Conclusion: Unit of account and derecognition concluded per position.
- Status: supported

### s3_c_valuation

At 2026-06-30 the positions are valued at 1,125,000 USD (LP), 250,000 USD (lending claim) and 650,000 USD (wrapped BTC), 2,025,000 USD in total, at prices evidenced on that date.

- Conclusion: Valuation recomputes and ties to the register.
- Status: supported

### s4_c_measurement

The LP and wrapped tokens are indefinite-lived intangibles under ASC 350-30; the lending claim is measured at the fair value of the ETH owed with an ASC 326 assessment.

- Conclusion: Subsequent measurement route per position.
- Status: supported

### s5_c_disclosure

The positions, the protocols' custody of the underlyings, the measurement bases and the concentration in one wallet are disclosed.

- Conclusion: Presentation and disclosure per ASC 350-30-50, ASC 275-10-50 and ASC 820-10-50.
- Status: supported

## Judgments

### s2_j_lp_treatment — Unit of account and derecognition of the liquidity pool contribution

On contributing ETH and DUSD to the DemoSwap pool, did DemoCo surrender control of them, and what asset does it hold?

- Proposed conclusion: Control passed: the pool commingles the assets, swaps them with any user, and returns only a pro-rata share of whatever reserves exist on redemption, never the units deposited — the transferee may pledge or exchange them freely and DemoCo has no right to specific units (ASC 860-10-40-5(b) and (c) by analogy, as the AICPA Digital Assets Practice Aid applies to crypto intangibles). The ETH and DUSD are derecognized and the DSLP token is recognized as a new asset at the fair value of the assets given up. The unit of account is the underlyings for valuation (the token has no market of its own); subsequent measurement follows ASC 350-30 as an indefinite-lived intangible because the LP token conveys an enforceable right to pool reserves and so fails ASC 350-60-15-1(b).
- Status: reviewed

### s2_j_lend_treatment — Unit of account and derecognition of the lending deposit

On depositing 100 ETH with DemoLend for dETH, did DemoCo surrender control of the ETH, and what does it hold?

- Proposed conclusion: Control passed: the protocol lends the ETH to third parties and DemoCo's claim is to an equivalent amount of ETH plus interest, not the units deposited (ASC 860-10-40-5 by analogy, consistent with the SEC staff and AICPA Practice Aid view on lent crypto). The ETH is derecognized; DemoCo recognizes the receipt token as a claim on the protocol for 100 ETH, measured at the fair value of the ETH owed with changes in income and assessed for credit and protocol risk under ASC 326 by analogy. The unit of account is the underlying ETH owed.
- Status: reviewed

### s2_j_wrap_treatment — Classification of the wrapped token

Is WBTC-D a crypto asset under ASC 350-60, or something else?

- Proposed conclusion: Not an ASC 350-60 crypto asset: the holder has an enforceable right to redeem it for BTC with the custodian, failing criterion (b) of 350-60-15-1. It is an indefinite-lived intangible under ASC 350-30 measured at cost less impairment, with the BTC it is redeemable for as the reference for its fair value. Control of the underlying BTC rests with the bridge custodian; DemoCo did not hold BTC before wrapping, so nothing was derecognized.
- Status: reviewed

### s4_j_measurement_routes — Subsequent measurement routes

How is each recognized position measured after initial recognition?

- Proposed conclusion: The LP token and the wrapped token are indefinite-lived intangibles at cost less impairment under ASC 350-30, tested when indicators exist with the pro-rata reserves and the redeemable BTC as the fair value references; the lending claim is measured at the fair value of the ETH owed with changes in income and an ASC 326 assessment of protocol and borrower risk. None is an ASC 350-60 crypto asset because each conveys an enforceable right to other assets.
- Status: reviewed

## Validations

- `core.package.safePaths` — pass
- `core.manifest.exists` — pass
- `core.manifest.schemaValid` — pass
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
- `core.factAnchors.quotesMatch` — pass — 10 quote anchors verified
- `core.claimRefs.resolve` — pass
- `core.graph.refsResolve` — pass
- `core.graph.acyclic` — pass
- `core.graph.declaredInputs` — pass
- `core.digests.match` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `defi.register.present` — pass — 3 positions in register
- `defi.register.schemaValid` — pass
- `defi.register.judgmentLinked` — pass — every position links a resolvable treatment judgment and custody evidence, and derecognition agrees with control
- `defi.valuation.schemaValid` — pass — calculations/s3_position_valuation.json
- `defi.valuation.recompute` — pass — 1 valuation(s): every position is Σ units × price and the total is their sum
- `defi.valuation.pricingEvidence` — pass — every component is priced by a fact stating that price as of the valuation date
- `defi.valuation.tiesToRegister` — pass — every valued position is in the register with its units, and every register position is valued
