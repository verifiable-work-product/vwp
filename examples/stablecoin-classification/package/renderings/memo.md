# Stablecoin Classification - DemoCo Holdings at 2026-06-30

- Package: `vwp_demo_stablecoin_classification`
- Profile: `vwp-finance`
- Extension: `vwp-digital-assets-stablecoin`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DUSD held directly with issuer redemption rights at par is a financial asset — a receivable-like claim on the issuer — in both wallets, not cash or a cash equivalent (it has no maturity; AICPA Practice Aid view); XEUR, an algorithmic token with no redemption right, is an ASC 350-60 crypto asset at fair value.

## Claims

### s1_c_terms

DUSD carries a direct, at-par, T+1 redemption right for verified customers backed by short-dated reserves; XEUR carries no redemption right.

- Conclusion: Instrument terms established from issuer and protocol documentation.
- Status: supported

### s2_c_scope

DUSD is outside ASC 350-60 because of its enforceable redemption right; XEUR is within ASC 350-60.

- Conclusion: Scope split by redemption right.
- Status: supported

### s3_c_classification

All 1,250,000 DUSD is a financial asset (a receivable-like claim on the issuer at par), not cash or a cash equivalent; 100,000 XEUR is a crypto asset at fair value.

- Conclusion: Classification consistent across wallets.
- Status: supported

### s4_c_measurement

DUSD is carried at par as a receivable with a nil ASC 326 allowance given attested over-reserving and T+1 settlement; XEUR is remeasured at fair value through net income.

- Conclusion: Measurement follows classification.
- Status: supported

### s5_c_presentation

DUSD is presented as a separate financial asset line (stablecoin receivable), outside cash and cash equivalents, and XEUR within crypto assets; the stablecoin concentration with one issuer is disclosed.

- Conclusion: Presentation per ASC 210, ASC 230 and ASC 275.
- Status: supported

## Judgments

### s2_j_dusd_scope — ASC 350-60 scope of DUSD

Does DUSD meet the ASC 350-60-15-1 criteria?

- Proposed conclusion: No. DUSD provides the holder an enforceable right to redeem for US dollars with the issuer, failing criterion (b). It is accounted for as a financial asset, not under ASC 350-60.
- Status: reviewed

### s3_j_dusd_class — Balance sheet classification of DUSD

Is DUSD cash, a cash equivalent, or another financial asset?

- Proposed conclusion: A financial asset: a receivable-like contractual right to receive 1 USD per token from the issuer on demand, carried at par with an ASC 326 credit loss assessment. Not cash (the issuer is not a depository institution and the token is not a demand deposit) and not a cash equivalent: DUSD has no maturity, so it cannot meet the ASC 230-10-20 short-maturity test however liquid it is — the AICPA Digital Assets Practice Aid view is that stablecoins are generally not cash equivalents. DemoCo's direct par redemption right settling T+1 and the attested reserves support par measurement and a nil allowance, not a cash-equivalent caption.
- Status: reviewed

### s3_j_xeur_class — Balance sheet classification of XEUR

How is XEUR classified given it has no issuer and no redemption right?

- Proposed conclusion: ASC 350-60 crypto asset measured at fair value: it is an intangible on a distributed ledger, secured cryptographically, fungible, not created by DemoCo, and conveys no enforceable rights.
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
- `core.factAnchors.quotesMatch` — pass — 5 quote anchors verified
- `core.claimRefs.resolve` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `stablecoin.holdings.registerPresent` — pass — 3 holdings in register
- `stablecoin.holdings.schemaValid` — pass
- `stablecoin.classification.judgmentLinked` — pass — every holding links a resolvable classification judgment
- `stablecoin.redemption.evidence` — pass — cash-like classifications rest on an evidenced redemption right
- `stablecoin.classification.cashEquivalentChallenged` — pass — no holding is classified as cash or a cash equivalent
- `stablecoin.classification.consistent` — pass — 2 token/issuer pairs classified consistently
