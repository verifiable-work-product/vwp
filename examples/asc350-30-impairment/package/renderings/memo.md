# ASC 350-30 Impairment Test - DemoCo NFT Holdings Q2 2026

- Package: `vwp_demo_asc350_30_impairment`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-asc350-30`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

Two NFTs held outside ASC 350-60 (non-fungible) are tested for impairment at 2026-06-30 against the lowest observable floor price; NFT-001 is impaired by 15,000 USD, NFT-002 is not.

## Claims

### s1_c_scope

The NFT holdings are indefinite-lived intangible assets under ASC 350-30, outside ASC 350-60 because they are not fungible.

- Conclusion: ASC 350-30 applies; indefinite useful life.
- Status: supported

### s4_c_impairment

NFT-001 is impaired by 15,000 USD to a new carrying amount of 25,000 USD; NFT-002's fair value exceeds its carrying amount and no impairment is recorded.

- Conclusion: Total impairment loss 15,000 USD; no reversal of prior impairments.
- Status: supported

### s5_c_presentation

The impairment loss is presented within income from continuing operations and the carrying amount after impairment becomes the new cost basis.

- Conclusion: Presentation per ASC 350-30-45-1.
- Status: supported

## Judgments

### s1_j_scope — Standard, rights and useful life of the NFT holdings

What rights does each NFT convey, which standard applies, and is the useful life indefinite?

- Proposed conclusion: Each NFT conveys only ownership of the token and a non-exclusive display licence; no right to goods, services, royalties or other assets. They fail ASC 350-60-15-1(e) (not fungible) and are intangible assets under ASC 350-30 with an indefinite useful life (ASC 350-30-35-4: no legal, contractual or economic limit), so they are carried at cost less impairment and tested under ASC 350-30-35-18.
- Status: reviewed

### s3_j_fair_value_basis — Fair value basis for the impairment test

Which observable price is the fair value for the impairment test of each NFT?

- Proposed conclusion: The lowest observable floor price for the collection during the period on the principal marketplace, consistent with the practice of testing at the lowest intraday observable price.
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
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `asc35030.impairment.schemaValid` — pass — calculations/s4_impairment_test.json
- `asc35030.impairment.recompute` — pass — 2 lot tests recompute; total impairment loss 15000
- `asc35030.impairment.noReversal` — pass — no lot is written up above its carrying amount
- `asc35030.impairment.fairValueEvidence` — pass — every lot's fair value equals the value of the fact it cites
- `asc35030.impairment.continuity` — pass — lots tested more than once carry forward the earlier test's carrying amount
- `asc35030.accountingSchedule.tieOut` — pass — 1 entry tie to 1 calculation(s) as impairment in USD
