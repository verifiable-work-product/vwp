//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// DeFi positions: a liquidity-pool position, a lending deposit and a
// wrapped token, each with its own unit-of-account and derecognition
// conclusion. All protocols, tokens and prices are fictional.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "defi-positions", extension: vwp.ExtensionDeFiPositions, domain: "accounting.defiPositions",
		title:   "DeFi Positions - DemoCo at 2026-06-30",
		summary: "The DemoSwap LP position is a new asset (the LP token) recognized on contribution: control of the ETH and DUSD passed to the pool, so they were derecognized; it is valued through its pro-rata underlyings at 1,125,000 USD. The DemoLend deposit of 100 ETH was derecognized for a receipt token conveying a right to equivalent ETH, a financial-asset-like claim measured at the ETH fair value of 250,000 USD. The wrapped BTC conveys a redemption right to BTC, failing ASC 350-60-15-1(b); it is an indefinite-lived intangible under ASC 350-30 valued for disclosure at 650,000 USD.",
		refs:    []string{"ASC 860-10-40-5", "ASC 350-60-15-1", "ASC 350-30-35-18", "ASC 820-10-35-1"},
		period:  []string{"2026-04-01", "2026-06-30"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_positions_query", "protocolQuery", "Protocol position query at 2026-06-30", "evidence/positions-query.csv",
						"protocol,position,receipt_token,receipt_units,underlying,underlying_units,wallet,asof\nDemoSwap,ETH/DUSD pool,DSLP-ETH-DUSD,1000,ETH,250,treasury-defi,2026-06-30\nDemoSwap,ETH/DUSD pool,DSLP-ETH-DUSD,1000,DUSD,500000,treasury-defi,2026-06-30\nDemoLend,ETH deposit,dETH,100,ETH,100,treasury-defi,2026-06-30\nWrapBridge,wrapped BTC,WBTC-D,10,BTC,10,treasury-defi,2026-06-30\n")
				},
				func() error {
					return ev(b, "ev_wallet_control", "custodyAttestation", "Signed message from the treasury-defi wallet", "evidence/wallet-control.txt",
						"Message: DemoCo controls treasury-defi as of 2026-06-30\nSigned: 2026-06-30 by the treasury-defi key\nVerified against the on-chain address.\n")
				},
				func() error {
					return ev(b, "ev_pool_docs", "protocolDocumentation", "DemoSwap pool documentation excerpt", "evidence/pool-docs.txt",
						"DemoSwap (excerpt)\n\nLiquidity providers deposit both assets into the pool contract and receive DSLP tokens. Deposited assets are commingled and used by the pool for swaps with any user. An LP may redeem DSLP tokens at any time for a pro-rata share of the pool's current reserves, not for the specific units deposited.\n")
				},
				func() error {
					return ev(b, "ev_lend_docs", "protocolDocumentation", "DemoLend documentation excerpt", "evidence/lend-docs.txt",
						"DemoLend (excerpt)\n\nDepositors receive dETH 1:1 for ETH deposited. Deposited ETH is lent to borrowers; the depositor may withdraw an equivalent amount of ETH plus accrued interest at any time, subject to pool liquidity.\n")
				},
				func() error {
					return ev(b, "ev_wrap_docs", "protocolDocumentation", "WrapBridge documentation excerpt", "evidence/wrap-docs.txt",
						"WrapBridge (excerpt)\n\nWBTC-D is minted 1:1 against BTC held by the bridge custodian. Any holder may redeem WBTC-D for BTC with the custodian.\n")
				},
				func() error {
					return ev(b, "ev_prices", "pricingRecord", "Period-end prices from the principal market, 2026-06-30", "evidence/period-end-prices.csv",
						"asset,price_usd,source,asof\nETH,2500.00,demo-exchange,2026-06-30\nDUSD,1.00,demo-exchange,2026-06-30\nBTC,65000.00,demo-exchange,2026-06-30\n")
				},
				func() error {
					return fact(b, "s1_f_lp_position", "DemoCo holds 1,000 DSLP-ETH-DUSD in treasury-defi, a pro-rata claim on 250 ETH and 500,000 DUSD of pool reserves at 2026-06-30.", "quantity", m("receiptUnits", 1000, "ETH", 250, "DUSD", 500000), "ev_positions_query", "DemoSwap,ETH/DUSD pool,DSLP-ETH-DUSD,1000,ETH,250,treasury-defi,2026-06-30")
				},
				func() error {
					return fact(b, "s1_f_lend_position", "DemoCo holds 100 dETH in treasury-defi against 100 ETH deposited with DemoLend at 2026-06-30.", "quantity", m("receiptUnits", 100, "ETH", 100), "ev_positions_query", "DemoLend,ETH deposit,dETH,100,ETH,100,treasury-defi,2026-06-30")
				},
				func() error {
					return fact(b, "s1_f_wrap_position", "DemoCo holds 10 WBTC-D in treasury-defi, redeemable for 10 BTC, at 2026-06-30.", "quantity", m("receiptUnits", 10, "BTC", 10), "ev_positions_query", "WrapBridge,wrapped BTC,WBTC-D,10,BTC,10,treasury-defi,2026-06-30")
				},
				func() error {
					return fact(b, "s1_f_wallet_control", "DemoCo controls the treasury-defi wallet, evidenced by a signed message verified on 2026-06-30.", "control", m("wallet", "treasury-defi", "date", "2026-06-30"), "ev_wallet_control", "Message: DemoCo controls treasury-defi as of 2026-06-30")
				},
				func() error {
					return fact(b, "s2_f_pool_terms", "DemoSwap commingles deposited assets, uses them for swaps with any user, and redeems LP tokens for a pro-rata share of current reserves, not the units deposited.", "term", "pro-rata redemption of commingled reserves", "ev_pool_docs", "An LP may redeem DSLP tokens at any time for a pro-rata share of the pool's current reserves, not for the specific units deposited.")
				},
				func() error {
					return fact(b, "s2_f_lend_terms", "DemoLend lends deposited ETH to borrowers; the depositor may withdraw an equivalent amount of ETH plus interest subject to pool liquidity.", "term", "right to equivalent ETH", "ev_lend_docs", "the depositor may withdraw an equivalent amount of ETH plus accrued interest at any time, subject to pool liquidity.")
				},
				func() error {
					return fact(b, "s2_f_wrap_terms", "WBTC-D is minted 1:1 against custodied BTC and any holder may redeem it for BTC.", "term", "redemption right to BTC", "ev_wrap_docs", "Any holder may redeem WBTC-D for BTC with the custodian.")
				},
				func() error {
					return fact(b, "s3_f_price_eth", "ETH closed at 2,500.00 USD on 2026-06-30.", "price", m("price", 2500, "date", "2026-06-30"), "ev_prices", "ETH,2500.00,demo-exchange,2026-06-30")
				},
				func() error {
					return fact(b, "s3_f_price_dusd", "DUSD closed at 1.00 USD on 2026-06-30.", "price", m("price", 1, "date", "2026-06-30"), "ev_prices", "DUSD,1.00,demo-exchange,2026-06-30")
				},
				func() error {
					return fact(b, "s3_f_price_btc", "BTC closed at 65,000.00 USD on 2026-06-30.", "price", m("price", 65000, "date", "2026-06-30"), "ev_prices", "BTC,65000.00,demo-exchange,2026-06-30")
				},
				func() error {
					return judgment(b, "s2_j_lp_treatment", "Unit of account and derecognition of the liquidity pool contribution",
						"On contributing ETH and DUSD to the DemoSwap pool, did DemoCo surrender control of them, and what asset does it hold?",
						"Control passed: the pool commingles the assets, swaps them with any user, and returns only a pro-rata share of whatever reserves exist on redemption, never the units deposited — the transferee may pledge or exchange them freely and DemoCo has no right to specific units (ASC 860-10-40-5(b) and (c) by analogy, as the AICPA Digital Assets Practice Aid applies to crypto intangibles). The ETH and DUSD are derecognized and the DSLP token is recognized as a new asset at the fair value of the assets given up. The unit of account is the underlyings for valuation (the token has no market of its own); subsequent measurement follows ASC 350-30 as an indefinite-lived intangible because the LP token conveys an enforceable right to pool reserves and so fails ASC 350-60-15-1(b).",
						"ASC 860-10-40-5 by analogy; ASC 610-20-40-1; ASC 350-60-15-1(b); ASC 350-30-35-18; AICPA Digital Assets Practice Aid, liquidity pool contributions and wrapped tokens; s2_f_pool_terms; s1_f_lp_position.",
						vwp.JudgmentAlternative{Conclusion: "Retain the ETH and DUSD on balance sheet as if lent", Notes: "Rejected: the right to a pro-rata share of changing reserves is not a right to the units transferred; the exposure is to the pool, not the deposited assets."}, "high")
				},
				func() error {
					return judgment(b, "s2_j_lend_treatment", "Unit of account and derecognition of the lending deposit",
						"On depositing 100 ETH with DemoLend for dETH, did DemoCo surrender control of the ETH, and what does it hold?",
						"Control passed: the protocol lends the ETH to third parties and DemoCo's claim is to an equivalent amount of ETH plus interest, not the units deposited (ASC 860-10-40-5 by analogy, consistent with the SEC staff and AICPA Practice Aid view on lent crypto). The ETH is derecognized; DemoCo recognizes the receipt token as a claim on the protocol for 100 ETH, measured at the fair value of the ETH owed with changes in income and assessed for credit and protocol risk under ASC 326 by analogy. The unit of account is the underlying ETH owed.",
						"ASC 860-10-40-5 by analogy; AICPA Digital Assets Practice Aid, crypto lending; ASC 326-20-30-2; s2_f_lend_terms; s1_f_lend_position.",
						vwp.JudgmentAlternative{Conclusion: "Keep the ETH on balance sheet as a secured deposit", Notes: "Rejected: no right to the specific units and unrestricted use by the protocol."}, "high")
				},
				func() error {
					return judgment(b, "s2_j_wrap_treatment", "Classification of the wrapped token",
						"Is WBTC-D a crypto asset under ASC 350-60, or something else?",
						"Not an ASC 350-60 crypto asset: the holder has an enforceable right to redeem it for BTC with the custodian, failing criterion (b) of 350-60-15-1. It is an indefinite-lived intangible under ASC 350-30 measured at cost less impairment, with the BTC it is redeemable for as the reference for its fair value. Control of the underlying BTC rests with the bridge custodian; DemoCo did not hold BTC before wrapping, so nothing was derecognized.",
						"ASC 350-60-15-1(b); ASC 350-30-35-18; AICPA Digital Assets Practice Aid, wrapped tokens; s2_f_wrap_terms; s1_f_wrap_position.",
						vwp.JudgmentAlternative{Conclusion: "Look through to the BTC and apply ASC 350-60", Notes: "Rejected: the entity holds a claim on the custodian, not BTC; the Practice Aid does not permit look-through."}, "high")
				},
				func() error {
					return register(b, validators.RegisterDeFiPositions, m(
						"description", "DeFi positions register at 2026-06-30. Synthetic data.",
						"asOf", "2026-06-30", "currency", "USD",
						"positions", []any{
							m("id", "pos_lp_eth_dusd", "protocol", "DemoSwap", "positionType", "liquidityPool", "receiptToken", "DSLP-ETH-DUSD", "receiptUnits", 1000, "wallet", "treasury-defi",
								"underlyings", []any{m("asset", "ETH", "units", 250), m("asset", "DUSD", "units", 500000)},
								"controlRetained", false, "derecognized", true, "unitOfAccount", "underlyings", "subsequentMeasurement", "ASC 350-30 indefinite-lived intangible (LP token)",
								"treatmentJudgmentRef", "s2_j_lp_treatment", "custodyEvidenceRef", "ev_wallet_control"),
							m("id", "pos_lend_eth", "protocol", "DemoLend", "positionType", "lending", "receiptToken", "dETH", "receiptUnits", 100, "wallet", "treasury-defi",
								"underlyings", []any{m("asset", "ETH", "units", 100)},
								"controlRetained", false, "derecognized", true, "unitOfAccount", "underlyings", "subsequentMeasurement", "claim on the protocol at the fair value of ETH owed, ASC 326 assessment",
								"treatmentJudgmentRef", "s2_j_lend_treatment", "custodyEvidenceRef", "ev_wallet_control"),
							m("id", "pos_wrapped_btc", "protocol", "WrapBridge", "positionType", "wrapped", "receiptToken", "WBTC-D", "receiptUnits", 10, "wallet", "treasury-defi",
								"underlyings", []any{m("asset", "BTC", "units", 10)},
								"controlRetained", false, "derecognized", false, "unitOfAccount", "underlyings", "subsequentMeasurement", "ASC 350-30 indefinite-lived intangible",
								"treatmentJudgmentRef", "s2_j_wrap_treatment", "custodyEvidenceRef", "s1_f_wallet_control"),
						}))
				},
				func() error {
					return calc(b, "s3_position_valuation", "defiPositionValuation",
						"Each position at the sum of its components x the period-end price its fact states; total is the sum of positions.",
						validators.ValidatorDeFiValuationRecompute,
						m("asOf", "2026-06-30", "currency", "USD", "positions", []any{
							m("id", "pos_lp_eth_dusd", "components", []any{m("asset", "ETH", "units", 250, "unitPrice", 2500, "priceFact", "s3_f_price_eth"), m("asset", "DUSD", "units", 500000, "unitPrice", 1, "priceFact", "s3_f_price_dusd")}),
							m("id", "pos_lend_eth", "components", []any{m("asset", "ETH", "units", 100, "unitPrice", 2500, "priceFact", "s3_f_price_eth")}),
							m("id", "pos_wrapped_btc", "components", []any{m("asset", "BTC", "units", 10, "unitPrice", 65000, "priceFact", "s3_f_price_btc")}),
						}),
						m("positions", []any{
							m("id", "pos_lp_eth_dusd", "fairValue", 1125000),
							m("id", "pos_lend_eth", "fairValue", 250000),
							m("id", "pos_wrapped_btc", "fairValue", 650000),
						}, "totalFairValue", 2025000))
				},
				func() error {
					return judgment(b, "s4_j_measurement_routes", "Subsequent measurement routes",
						"How is each recognized position measured after initial recognition?",
						"The LP token and the wrapped token are indefinite-lived intangibles at cost less impairment under ASC 350-30, tested when indicators exist with the pro-rata reserves and the redeemable BTC as the fair value references; the lending claim is measured at the fair value of the ETH owed with changes in income and an ASC 326 assessment of protocol and borrower risk. None is an ASC 350-60 crypto asset because each conveys an enforceable right to other assets.",
						"ASC 350-30-35-18 to 35-20; ASC 350-60-15-1(b); ASC 326-20-30-2; s2_f_pool_terms; s2_f_lend_terms; s2_f_wrap_terms.",
						vwp.JudgmentAlternative{Conclusion: "Fair value through net income for all three", Notes: "Rejected: no election is available for the intangibles; permitted for the lending claim only under an ASC 825-10 fair value option analysis, documented as the alternative for the reviewer."}, "high")
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_population", statement: "DemoCo holds three DeFi positions in the treasury-defi wallet it controls: a DemoSwap LP position, a DemoLend ETH deposit and 10 WBTC-D.",
						conclusion: "Position population established from the protocol query and wallet control evidence.", refs: []string{"AU-C 501", "ASC 860-10-40-5"}, assertions: []string{"existence", "completeness", "rightsAndObligations"},
						facts: []string{"s1_f_lp_position", "s1_f_lend_position", "s1_f_wrap_position", "s1_f_wallet_control"}, evidence: []string{"ev_positions_query", "ev_wallet_control"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_treatment", statement: "The LP contribution and the lending deposit surrendered control and were derecognized for new assets; the wrapped token is a claim on the custodian, not a crypto asset under ASC 350-60.",
						conclusion: "Unit of account and derecognition concluded per position.", refs: []string{"ASC 860-10-40-5", "ASC 610-20-40-1", "ASC 350-60-15-1"}, assertions: []string{"rightsAndObligations", "existence"},
						facts: []string{"s2_f_pool_terms", "s2_f_lend_terms", "s2_f_wrap_terms"}, judgments: []string{"s2_j_lp_treatment", "s2_j_lend_treatment", "s2_j_wrap_treatment"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_valuation", statement: "At 2026-06-30 the positions are valued at 1,125,000 USD (LP), 250,000 USD (lending claim) and 650,000 USD (wrapped BTC), 2,025,000 USD in total, at prices evidenced on that date.",
						conclusion: "Valuation recomputes and ties to the register.", refs: []string{"ASC 820-10-35-1", "ASC 820-10-35-5"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s3_f_price_eth", "s3_f_price_dusd", "s3_f_price_btc"}, calcs: []string{"s3_position_valuation"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_measurement", statement: "The LP and wrapped tokens are indefinite-lived intangibles under ASC 350-30; the lending claim is measured at the fair value of the ETH owed with an ASC 326 assessment.",
						conclusion: "Subsequent measurement route per position.", refs: []string{"ASC 350-30-35-18", "ASC 326-20-30-2", "ASC 350-60-15-1"}, assertions: []string{"valuationAllocation"},
						judgments: []string{"s4_j_measurement_routes"}, calcs: []string{"s3_position_valuation"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_disclosure", statement: "The positions, the protocols' custody of the underlyings, the measurement bases and the concentration in one wallet are disclosed.",
						conclusion: "Presentation and disclosure per ASC 350-30-50, ASC 275-10-50 and ASC 820-10-50.", refs: []string{"ASC 350-30-50-1", "ASC 275-10-50-16", "ASC 820-10-50-2"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s3_position_valuation"}})
				},
			)
		},
	})
}
