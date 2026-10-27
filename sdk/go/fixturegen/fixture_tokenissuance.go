//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// Token issuance: a SAFT presale, a public sale of a utility token with a
// remaining service obligation, a treasury allocation and an airdrop. All
// synthetic.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "token-issuance", extension: vwp.ExtensionTokenIssuance, domain: "accounting.tokenIssuance",
		title:   "Token Issuance Accounting - DemoCo DEMO Token, Q2 2026",
		summary: "The SAFT proceeds of 1,000,000 USD are a liability to deliver DEMO tokens (not equity, not a derivative pre-launch); the public sale proceeds of 500,000 USD are a contract liability under ASC 606 because the token conveys platform services DemoCo must still provide; the treasury allocation and the airdrop received no consideration and recognize no asset or liability.",
		refs:    []string{"ASC 480-10-25", "ASC 815-10-15-83", "ASC 606-10-45-2", "ASC 350-60-15-1"},
		period:  []string{"2026-04-01", "2026-06-30"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_saft", "contractDocument", "Simple Agreement for Future Tokens excerpt", "evidence/saft-excerpt.txt",
						"Simple Agreement for Future Tokens (excerpt)\n\n1. Purchase amount: 1,000,000.00 USD, received 2026-04-15.\n2. Tokens: 2,000,000 DEMO, deliverable within 30 days of the network launch.\n3. Refund: none; the purchase amount is non-refundable except on dissolution before launch.\n4. Settlement: delivery of DEMO tokens only; no cash settlement alternative.\n5. The DEMO token confers no equity, dividend, voting or liquidation rights in DemoCo.\n")
				},
				func() error {
					return ev(b, "ev_public_sale_terms", "contractDocument", "Public sale terms excerpt", "evidence/public-sale-terms.txt",
						"DEMO Public Sale Terms (excerpt)\n\n1. Price: 0.50 USD per DEMO; 1,000,000 DEMO sold for 500,000.00 USD on 2026-06-01.\n2. Utility: each DEMO token entitles the holder to 1 compute unit on the DemoCo platform, to be provided by DemoCo on request through 2027-05-31.\n3. Delivery: tokens delivered at sale.\n4. The DEMO token confers no equity, dividend, voting or liquidation rights in DemoCo.\n")
				},
				func() error {
					return ev(b, "ev_bank_statement", "bankStatement", "Bank statement extract, April to June 2026", "evidence/bank-statement.csv",
						"date,description,amount_usd\n2026-04-15,SAFT purchase amount - investor class A,1000000.00\n2026-06-01,Public sale proceeds - DEMO,500000.00\n")
				},
				func() error {
					return ev(b, "ev_token_ledger", "subledgerExtract", "DEMO token allocation ledger", "evidence/token-allocation-ledger.csv",
						"instrument,tokens,date,consideration_usd\nSAFT-1,2000000,2026-04-15,1000000.00\nPUB-1,1000000,2026-06-01,500000.00\nTREAS-1,10000000,2026-05-01,0\nAIR-1,250000,2026-06-15,0\n")
				},
				func() error {
					return fact(b, "s1_f_saft_proceeds", "DemoCo received 1,000,000.00 USD under the SAFT on 2026-04-15.", "amount", m("amount", 1000000, "date", "2026-04-15"), "ev_bank_statement", "2026-04-15,SAFT purchase amount - investor class A,1000000.00")
				},
				func() error {
					return fact(b, "s1_f_saft_terms", "The SAFT is non-refundable, settles only in DEMO tokens and confers no equity rights.", "term", m("refundable", false, "settlementInEntityTokens", true, "equityRights", false), "ev_saft", "4. Settlement: delivery of DEMO tokens only; no cash settlement alternative.")
				},
				func() error {
					return fact(b, "s1_f_public_proceeds", "DemoCo received 500,000.00 USD in the public sale on 2026-06-01.", "amount", m("amount", 500000, "date", "2026-06-01"), "ev_bank_statement", "2026-06-01,Public sale proceeds - DEMO,500000.00")
				},
				func() error {
					return fact(b, "s1_f_public_utility", "Each DEMO token entitles the holder to one compute unit DemoCo must provide on request through 2027-05-31.", "term", "service obligation through 2027-05-31", "ev_public_sale_terms", "2. Utility: each DEMO token entitles the holder to 1 compute unit on the DemoCo platform, to be provided by DemoCo on request through 2027-05-31.")
				},
				func() error {
					return fact(b, "s1_f_allocations", "DemoCo allocated 2,000,000 DEMO to the SAFT, sold 1,000,000 DEMO publicly, allocated 10,000,000 DEMO to treasury and airdropped 250,000 DEMO.", "quantity", m("SAFT-1", 2000000, "PUB-1", 1000000, "TREAS-1", 10000000, "AIR-1", 250000), "ev_token_ledger", "SAFT-1,2000000,2026-04-15,1000000.00")
				},
				func() error {
					return judgment(b, "s2_j_saft_classification", "Classification of the SAFT obligation",
						"Is DemoCo's obligation to deliver 2,000,000 DEMO under the SAFT equity, a liability, a derivative, or a contract with a customer?",
						"A liability measured at the proceeds received (1,000,000 USD). Not equity: the DEMO token confers no ownership interest, so the obligation is not within ASC 480-10-25 as an obligation to issue shares, nor an equity-classified contract. Not a derivative at inception: before the network launches no market exists for DEMO, so the obligation lacks the net-settlement characteristic of ASC 815-10-15-83(c) (the token is not readily convertible to cash) — reassess at each reporting date. Not an ASC 606 contract at inception: the SAFT holder is not yet a customer for platform services; the tokens, once delivered, carry the utility analyzed for the public sale. The obligation is derecognized when the tokens are delivered.",
						"ASC 480-10-25-4 and 480-10-25-14 (obligations settled in shares do not apply — DEMO is not an equity share); ASC 815-10-15-83 (net settlement absent pre-launch); ASC 606-10-15-3 (no customer relationship for services at the SAFT date); s1_f_saft_terms; s1_f_saft_proceeds; AICPA Digital Assets Practice Aid, accounting by issuers.",
						vwp.JudgmentAlternative{Conclusion: "Derivative liability at fair value", Notes: "Rejected at inception for want of net settlement; becomes the answer if a liquid market for DEMO exists before delivery and the contract then meets ASC 815-10-15-83."}, "high")
				},
				func() error {
					return judgment(b, "s2_j_public_classification", "Classification of the public sale proceeds",
						"Are the public sale proceeds revenue, a contract liability, a liability to deliver tokens, or equity?",
						"A contract liability under ASC 606 (deferred revenue) of 500,000 USD. The buyers are customers: the DEMO token entitles them to compute units DemoCo must still provide, a performance obligation satisfied as the units are consumed or expire on 2027-05-31. The tokens themselves were delivered at sale, so no token-delivery liability remains; the token confers no equity rights.",
						"ASC 606-10-25-1 (contract with a customer), 606-10-25-14 (performance obligation), 606-10-45-2 (contract liability), 606-10-55-46 to 55-49 (breakage on unexercised rights); s1_f_public_utility; s1_f_public_proceeds.",
						vwp.JudgmentAlternative{Conclusion: "Revenue at sale", Notes: "Rejected: control of the compute units has not transferred; the obligation to provide them is unsatisfied at the sale date."}, "high")
				},
				func() error {
					return judgment(b, "s2_j_treasury", "Treasury allocation of self-issued tokens",
						"Does DemoCo recognize an asset for the 10,000,000 DEMO it allocated to its own treasury?",
						"No. Tokens created and issued by the reporting entity are outside ASC 350-60 (criterion f) and have no cost basis; no asset and no liability is recognized. The allocation is disclosed. An asset arises only on a sale of the treasury tokens for consideration.",
						"ASC 350-60-15-1(f); s1_f_allocations; AICPA Digital Assets Practice Aid, self-issued tokens held by the issuer.",
						vwp.JudgmentAlternative{Conclusion: "Recognize the tokens at fair value", Notes: "Rejected: the entity cannot recognize an asset for its own issued token with no cost."}, "medium")
				},
				func() error {
					return judgment(b, "s2_j_airdrop", "Airdropped tokens",
						"Does the airdrop of 250,000 DEMO to community wallets create an asset, a liability or an expense?",
						"Nothing is recognized: no consideration was received and the tokens carry no obligation beyond the platform utility already accounted for as a contract liability only where consideration was paid. The airdrop is disclosed as a distribution of self-issued tokens.",
						"ASC 350-60-15-1(f); s1_f_allocations; AICPA Digital Assets Practice Aid, self-issued tokens.",
						vwp.JudgmentAlternative{Conclusion: "Marketing expense at the token's fair value", Notes: "Considered: appropriate only where the entity previously recognized the tokens at cost; none was recognized here."}, "medium")
				},
				func() error {
					return register(b, validators.RegisterIssuanceInstruments, m(
						"description", "DEMO token issuance instruments register at 2026-06-30. Synthetic data.",
						"asOf", "2026-06-30", "currency", "USD",
						"instruments", []any{
							m("id", "SAFT-1", "type", "saft", "counterparty", "investor class A", "issuanceDate", "2026-04-15", "proceeds", 1000000, "proceedsFact", "s1_f_saft_proceeds",
								"tokensPromised", 2000000, "tokensDelivered", 0, "deliveryObligation", true, "refundable", false, "settlementInEntityTokens", true, "serviceObligation", false,
								"classification", "liability", "classificationJudgmentRef", "s2_j_saft_classification", "termsEvidence", []any{"ev_saft"}),
							m("id", "PUB-1", "type", "publicSale", "counterparty", "public sale purchasers", "issuanceDate", "2026-06-01", "proceeds", 500000, "proceedsFact", "s1_f_public_proceeds",
								"tokensPromised", 1000000, "tokensDelivered", 1000000, "deliveryObligation", false, "refundable", false, "settlementInEntityTokens", false, "serviceObligation", true,
								"classification", "deferredRevenue", "classificationJudgmentRef", "s2_j_public_classification", "termsEvidence", []any{"ev_public_sale_terms"}),
							m("id", "TREAS-1", "type", "treasuryAllocation", "counterparty", "DemoCo treasury", "issuanceDate", "2026-05-01", "proceeds", 0,
								"tokensPromised", 10000000, "tokensDelivered", 10000000, "deliveryObligation", false, "refundable", false, "settlementInEntityTokens", false, "serviceObligation", false,
								"classification", "none", "classificationJudgmentRef", "s2_j_treasury"),
							m("id", "AIR-1", "type", "airdrop", "counterparty", "community wallets", "issuanceDate", "2026-06-15", "proceeds", 0,
								"tokensPromised", 250000, "tokensDelivered", 250000, "deliveryObligation", false, "refundable", false, "settlementInEntityTokens", false, "serviceObligation", false,
								"classification", "none", "classificationJudgmentRef", "s2_j_airdrop"),
						}))
				},
				func() error {
					return calc(b, "s3_proceeds_allocation", "tokenIssuanceProceedsAllocation",
						"Per instrument the consideration received is allocated to the balance-sheet component its classification produces; byClassification and total are sums.",
						validators.ValidatorTokenIssuanceAllocationRecompute,
						m("asOf", "2026-06-30", "currency", "USD", "instruments", []any{
							m("id", "SAFT-1", "proceeds", 1000000, "components", []any{m("classification", "liability", "amount", 1000000, "basis", "proceeds received; obligation to deliver DEMO tokens")}),
							m("id", "PUB-1", "proceeds", 500000, "components", []any{m("classification", "deferredRevenue", "amount", 500000, "basis", "proceeds received; unsatisfied compute-unit performance obligation")}),
						}),
						m("byClassification", m("liability", 1000000, "equity", 0, "deferredRevenue", 500000, "derivativeLiability", 0), "total", 1500000))
				},
				func() error {
					return schedule(b, "s3_accounting_schedule", validators.ValidatorTokenIssuanceAccountingScheduleTieOut, []string{"s3_proceeds_allocation"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "DEMO token issuance, Q2 2026", Kind: accountingschedule.KindTokenIssuance, Currency: "USD", AmountScale: 2,
						StartDate: "2026-04-15", EndDate: "2026-06-01", TotalAmount: "1500000.00",
						Entries: []accountingschedule.Entry{
							entry("SAFT-1-2026-04-15", "2026-04-15", "SAFT proceeds received; liability to deliver 2,000,000 DEMO", "SAFT-1", dr(validators.RoleConsiderationReceived, "1000000.00"), cr(validators.RoleTokenLiability, "1000000.00")),
							entry("PUB-1-2026-06-01", "2026-06-01", "Public sale proceeds received; contract liability for compute units", "PUB-1", dr(validators.RoleConsiderationReceived, "500000.00"), cr(validators.RoleDeferredRevenue, "500000.00")),
						},
					})
				},
				func() error {
					return judgment(b, "s4_j_saft_remeasurement", "Subsequent measurement of the SAFT liability",
						"Is the SAFT liability remeasured before delivery?",
						"No while it is a liability to deliver tokens measured at proceeds: it is carried at 1,000,000 USD until the tokens are delivered, when it is derecognized against the tokens issued. If DEMO becomes readily convertible to cash before delivery the obligation is reassessed as a derivative under ASC 815 and measured at fair value from that date.",
						"ASC 405-20-40-1 (extinguishment on delivery); ASC 815-10-15-83; s1_f_saft_terms.",
						vwp.JudgmentAlternative{Conclusion: "Remeasure at the fair value of the tokens owed each period", Notes: "Rejected: a non-derivative, non-refundable obligation to deliver the entity's own non-equity token is not remeasured at fair value under current guidance; disclose the fair value of tokens owed instead."}, "medium")
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_instruments", statement: "DemoCo issued DEMO through a SAFT (1,000,000 USD received 2026-04-15), a public sale (500,000 USD received 2026-06-01), a treasury allocation and an airdrop, none of which conveys equity rights.",
						conclusion: "Instrument population and terms established from the agreements and bank statement.", refs: []string{"ASC 480-10-25", "ASC 606-10-25-1"}, assertions: []string{"completeness", "rightsAndObligations"},
						facts: []string{"s1_f_saft_proceeds", "s1_f_saft_terms", "s1_f_public_proceeds", "s1_f_public_utility", "s1_f_allocations"}, evidence: []string{"ev_saft", "ev_public_sale_terms", "ev_bank_statement", "ev_token_ledger"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_classification", statement: "The SAFT obligation is a liability, the public sale proceeds are a contract liability, and the treasury allocation and airdrop recognize nothing.",
						conclusion: "Each instrument classified by a judgment recorded in the register.", refs: []string{"ASC 480-10-25-4", "ASC 815-10-15-83", "ASC 606-10-45-2", "ASC 350-60-15-1"}, assertions: []string{"presentationDisclosure", "rightsAndObligations"},
						facts: []string{"s1_f_saft_terms", "s1_f_public_utility"}, judgments: []string{"s2_j_saft_classification", "s2_j_public_classification", "s2_j_treasury", "s2_j_airdrop"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_allocation", statement: "Of 1,500,000 USD received, 1,000,000 USD is a token liability and 500,000 USD deferred revenue; the executable schedule books each on its receipt date.",
						conclusion: "Allocation recomputes and ties to the register.", refs: []string{"ASC 606-10-45-2", "ASC 405-20-40-1"}, assertions: []string{"valuationAllocation", "completeness"},
						facts: []string{"s1_f_saft_proceeds", "s1_f_public_proceeds"}, calcs: []string{"s3_proceeds_allocation", "s3_accounting_schedule"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_subsequent", statement: "The SAFT liability is carried at proceeds until delivery and reassessed for derivative treatment at each reporting date; the contract liability is recognized as revenue as compute units are consumed or expire.",
						conclusion: "Subsequent measurement and derecognition concluded.", refs: []string{"ASC 405-20-40-1", "ASC 606-10-55-46", "ASC 815-10-15-83"}, assertions: []string{"valuationAllocation"},
						judgments: []string{"s4_j_saft_remeasurement", "s2_j_public_classification"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_disclosure", statement: "The token liability, the contract liability, the tokens outstanding by instrument and the treasury and airdrop allocations are disclosed.",
						conclusion: "Presentation and disclosure per ASC 606-10-50 and ASC 350-60-50.", refs: []string{"ASC 606-10-50-8", "ASC 350-60-50-1", "ASC 275-10-50"}, assertions: []string{"presentationDisclosure"},
						facts: []string{"s1_f_allocations"}, calcs: []string{"s3_proceeds_allocation"}})
				},
			)
		},
	})
}
