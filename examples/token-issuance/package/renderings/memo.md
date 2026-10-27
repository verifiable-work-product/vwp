# Token Issuance Accounting - DemoCo DEMO Token, Q2 2026

- Package: `vwp_demo_token_issuance`
- Profile: `vwp-finance`
- Extension: `vwp-accounting-token-issuance`
- Status: inReview

> Derived rendering. The structured package is the source of truth.

## Conclusion

The SAFT proceeds of 1,000,000 USD are a liability to deliver DEMO tokens (not equity, not a derivative pre-launch); the public sale proceeds of 500,000 USD are a contract liability under ASC 606 because the token conveys platform services DemoCo must still provide; the treasury allocation and the airdrop received no consideration and recognize no asset or liability.

## Claims

### s1_c_instruments

DemoCo issued DEMO through a SAFT (1,000,000 USD received 2026-04-15), a public sale (500,000 USD received 2026-06-01), a treasury allocation and an airdrop, none of which conveys equity rights.

- Conclusion: Instrument population and terms established from the agreements and bank statement.
- Status: supported

### s2_c_classification

The SAFT obligation is a liability, the public sale proceeds are a contract liability, and the treasury allocation and airdrop recognize nothing.

- Conclusion: Each instrument classified by a judgment recorded in the register.
- Status: supported

### s3_c_allocation

Of 1,500,000 USD received, 1,000,000 USD is a token liability and 500,000 USD deferred revenue; the executable schedule books each on its receipt date.

- Conclusion: Allocation recomputes and ties to the register.
- Status: supported

### s4_c_subsequent

The SAFT liability is carried at proceeds until delivery and reassessed for derivative treatment at each reporting date; the contract liability is recognized as revenue as compute units are consumed or expire.

- Conclusion: Subsequent measurement and derecognition concluded.
- Status: supported

### s5_c_disclosure

The token liability, the contract liability, the tokens outstanding by instrument and the treasury and airdrop allocations are disclosed.

- Conclusion: Presentation and disclosure per ASC 606-10-50 and ASC 350-60-50.
- Status: supported

## Judgments

### s2_j_saft_classification — Classification of the SAFT obligation

Is DemoCo's obligation to deliver 2,000,000 DEMO under the SAFT equity, a liability, a derivative, or a contract with a customer?

- Proposed conclusion: A liability measured at the proceeds received (1,000,000 USD). Not equity: the DEMO token confers no ownership interest, so the obligation is not within ASC 480-10-25 as an obligation to issue shares, nor an equity-classified contract. Not a derivative at inception: before the network launches no market exists for DEMO, so the obligation lacks the net-settlement characteristic of ASC 815-10-15-83(c) (the token is not readily convertible to cash) — reassess at each reporting date. Not an ASC 606 contract at inception: the SAFT holder is not yet a customer for platform services; the tokens, once delivered, carry the utility analyzed for the public sale. The obligation is derecognized when the tokens are delivered.
- Status: reviewed

### s2_j_public_classification — Classification of the public sale proceeds

Are the public sale proceeds revenue, a contract liability, a liability to deliver tokens, or equity?

- Proposed conclusion: A contract liability under ASC 606 (deferred revenue) of 500,000 USD. The buyers are customers: the DEMO token entitles them to compute units DemoCo must still provide, a performance obligation satisfied as the units are consumed or expire on 2027-05-31. The tokens themselves were delivered at sale, so no token-delivery liability remains; the token confers no equity rights.
- Status: reviewed

### s2_j_treasury — Treasury allocation of self-issued tokens

Does DemoCo recognize an asset for the 10,000,000 DEMO it allocated to its own treasury?

- Proposed conclusion: No. Tokens created and issued by the reporting entity are outside ASC 350-60 (criterion f) and have no cost basis; no asset and no liability is recognized. The allocation is disclosed. An asset arises only on a sale of the treasury tokens for consideration.
- Status: reviewed

### s2_j_airdrop — Airdropped tokens

Does the airdrop of 250,000 DEMO to community wallets create an asset, a liability or an expense?

- Proposed conclusion: Nothing is recognized: no consideration was received and the tokens carry no obligation beyond the platform utility already accounted for as a contract liability only where consideration was paid. The airdrop is disclosed as a distribution of self-issued tokens.
- Status: reviewed

### s4_j_saft_remeasurement — Subsequent measurement of the SAFT liability

Is the SAFT liability remeasured before delivery?

- Proposed conclusion: No while it is a liability to deliver tokens measured at proceeds: it is carried at 1,000,000 USD until the tokens are delivered, when it is derecognized against the tokens issued. If DEMO becomes readily convertible to cash before delivery the obligation is reassessed as a derivative under ASC 815 and measured at fair value from that date.
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
- `core.graph.refsResolve` — pass
- `core.graph.acyclic` — pass
- `core.graph.declaredInputs` — pass
- `core.digests.match` — pass
- `core.signatures.present` — warning — no package signature found
- `finance.claims.haveAssertions` — pass
- `finance.claims.haveSupport` — pass
- `finance.requiredJudgments.complete` — pass
- `tokenIssuance.register.present` — pass — 4 instruments in register
- `tokenIssuance.register.schemaValid` — pass
- `tokenIssuance.register.judgmentLinked` — pass — every instrument links a resolvable classification judgment and every instrument with proceeds is classified
- `tokenIssuance.allocation.schemaValid` — pass — calculations/s3_proceeds_allocation.json
- `tokenIssuance.allocation.recompute` — pass — 1 allocation(s): components sum to proceeds, classification totals and the total recompute
- `tokenIssuance.allocation.tiesToRegister` — pass — every allocated instrument is in the register with the same proceeds, every register instrument with proceeds is allocated, and proceeds equal their cited facts
- `tokenIssuance.accountingSchedule.tieOut` — pass — 2 entries tie to 1 calculation(s) as token_issuance in USD
