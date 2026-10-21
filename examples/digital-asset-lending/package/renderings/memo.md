# Digital Asset Loan Accounting - DemoCo 100 ETH Loan to Borrower X

- Package: `vwp_demo_digital_asset_lending`
- Profile: `vwp-finance`
- Extension: `vwp-digital-assets-lending`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

DemoCo lent 100 ETH for a 91-day term at 5% in kind; the borrower may rehypothecate, so control was surrendered (ASC 860-10-40-5 by analogy) and the ETH is derecognized for a crypto asset loan receivable measured at the fair value of the ETH to be returned, with a separate ASC 326 allowance. The fee accrues to 1.2466 ETH over the term; collateral of 400,000 USDC, marked daily, covers the 250,000 USD receivable at 1.6x against a 1.5x maintenance requirement, so the ASC 326-20-35-6 collateral maintenance expedient yields no allowance.

## Claims

### s1_c_terms

DemoCo lent 100 ETH for 91 days at a 5% in-kind fee, collateralized at 150% in USDC.

- Conclusion: Arrangement terms established from the agreement.
- Status: supported

### s2_c_derecognition

The lent ETH is derecognized and a crypto asset loan receivable for 100 ETH is recognized: the borrower obtained the right to use, sell and re-pledge the ETH and DemoCo's recall right is for equivalent units, not the units transferred (SEC staff / AICPA Practice Aid analysis for crypto intangibles; ASC 860-10-40-5 considered by analogy).

- Conclusion: Derecognize; recognize a crypto asset loan receivable.
- Status: supported

### s3_c_accrual

The receivable is measured at the fair value of the ETH to be returned (250,000 USD at 2026-06-30) and the in-kind fee accrues to 1.2466 ETH over the 91-day term, measured at the same ETH price.

- Conclusion: Receivable at fair value of the lent asset; accrual recomputes from the agreement terms.
- Status: supported

### s4_c_credit_loss

The receivable of 250,000 USD is covered 1.6x by 400,000 USDC of daily-marked collateral against a 1.5x maintenance requirement; under the ASC 326-20-35-6 collateral maintenance expedient the unsecured portion (carrying amount less collateral fair value) is zero and no allowance is recorded at 2026-06-30.

- Conclusion: Collateral maintenance expedient; no allowance.
- Status: supported

### s5_c_disclosure

The receivable, its collateral and the borrower concentration are disclosed.

- Conclusion: Disclosure per ASC 860-30-50 and ASC 326-20-50.
- Status: supported

## Judgments

### s2_j_derecognition — Derecognition of the lent ETH

Did DemoCo surrender control of the 100 ETH such that the ETH (an ASC 350-60 intangible, not a financial asset) is derecognized and a crypto asset loan receivable recognized?

- Proposed conclusion: Yes. Applying ASC 860-10-40-5 by analogy, as the AICPA Digital Assets Practice Aid and the SEC staff view do for crypto intangibles: the borrower may sell, lend or re-pledge the ETH without restriction, DemoCo has no right to specific units back, and the ETH is beyond DemoCo's reach. Derecognize the ETH; recognize a crypto asset loan receivable for 100 ETH plus the accrued fee, measured at the fair value of the ETH to be returned at each reporting date with changes in income, and assess it for credit losses under ASC 326.
- Status: reviewed

### s4_j_credit_loss — Expected credit loss on the ETH receivable

What allowance for credit losses applies to the crypto asset loan receivable under ASC 326-20, and which expedient (if any) is available?

- Proposed conclusion: The agreement requires the borrower to continually adjust USDC collateral to at least 150% of the loan value, marked daily, and the borrower has met every call; the ASC 326-20-35-6 collateral maintenance practical expedient applies and expected credit losses are measured on the unsecured portion — the carrying amount of 250,000 USD less collateral fair value of 400,000 USD, floored at zero — which is zero at 2026-06-30. The 150% margin shortfall is a separate margin-call measure and is not the allowance. The collateral-dependent measurement in 35-5 is not available: the borrower is not in financial difficulty.
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
- `lending.accrual.schemaValid` — pass — calculations/s3_interest_accrual.json
- `lending.accrual.recompute` — pass — 1 accrual(s) recompute from the terms their facts state
- `lending.accountingSchedule.tieOut` — pass — 1 entry tie to 1 calculation(s) as interest_accrual in USD
- `lending.collateral.schemaValid` — pass — calculations/s4_collateral_coverage.json
- `lending.collateral.recompute` — pass — 1 coverage calculation(s) recompute
- `lending.derecognition.claimJudged` — pass — 1 derecognition claim(s) link a judgment
