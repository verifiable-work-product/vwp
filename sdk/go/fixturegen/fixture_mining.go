//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// mining-revenue: a solo miner's April 2026 block rewards (subsidy plus
// fees), recognized when the coinbase output is credited to its own
// address, measured at that date's price, reconciled to the chain record
// and booked as mining revenue. Synthetic data.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "mining-revenue", extension: vwp.ExtensionMiningRevenue, domain: "accounting.miningRevenue",
		title:   "Mining Revenue Recognition - DemoCo Solo Mining April 2026",
		summary: "DemoCo mines with its own hash power and receives the coinbase output directly; it is the principal and, with no customer identified, ASC 606 is applied by analogy with rewards presented as mining revenue. April 2026 rewards of 9.545 BTC (three blocks, subsidy plus fees) tie to the chain record and total 677,725 USD at recognition-date prices.",
		refs:    []string{"ASC 606-10-15-3", "ASC 606-10-55-36", "ASC 606-10-32-21", "ASC 350-30-30-1"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_block_rewards", "chainRecord", "Coinbase outputs to DemoCo addresses, April 2026 (node export)", "evidence/block-rewards.csv",
						"date,block,subsidy_btc,fees_btc,total_btc\n2026-04-05,890100,3.12500000,0.05000000,3.17500000\n2026-04-18,890900,3.12500000,0.08000000,3.20500000\n2026-04-29,891700,3.12500000,0.04000000,3.16500000\nTOTAL,april-2026,9.37500000,0.17000000,9.54500000\n")
				},
				func() error {
					return ev(b, "ev_btc_prices", "pricingRecord", "BTC daily closing prices, April 2026", "evidence/btc-daily-prices.csv",
						"date,close_usd,source\n2026-04-05,70000.00,demo-exchange\n2026-04-18,72000.00,demo-exchange\n2026-04-29,71000.00,demo-exchange\n")
				},
				func() error {
					return ev(b, "ev_mining_ops", "contractDocument", "Mining operations memo", "evidence/mining-operations.txt",
						"DemoCo Mining Operations (excerpt)\n\nDemoCo operates its own ASIC fleet at the Demo Valley facility and mines solo: it submits its own block templates and the coinbase output of every block it finds pays to an address controlled by DemoCo treasury.\nNo pool is used and no hash power is sold or hosted for third parties.\nCoinbase outputs are spendable after the protocol's 100-block maturity.\n")
				},
				func() error {
					return fact(b, "s1_f_solo", "DemoCo mines solo with its own fleet; the coinbase output pays to an address it controls.", "term", "solo", "ev_mining_ops", "the coinbase output of every block it finds pays to an address controlled by DemoCo treasury.")
				},
				func() error {
					return fact(b, "s1_f_no_pool", "No pool is used and no hash power is sold or hosted for third parties.", "term", "no pool", "ev_mining_ops", "No pool is used and no hash power is sold or hosted for third parties.")
				},
				func() error {
					return fact(b, "s3_f_maturity", "Coinbase outputs are spendable after the protocol's 100-block maturity.", "term", "100-block maturity", "ev_mining_ops", "Coinbase outputs are spendable after the protocol's 100-block maturity.")
				},
				func() error {
					return fact(b, "s4_f_chain_total", "The node export shows 9.545 BTC of coinbase outputs (9.375 subsidy, 0.17 fees) credited to DemoCo addresses in April 2026.", "quantity", m("units", 9.545, "periodStart", "2026-04-01", "periodEnd", "2026-04-30"), "ev_block_rewards", "TOTAL,april-2026,9.37500000,0.17000000,9.54500000")
				},
				func() error {
					return fact(b, "s4_f_price_0405", "BTC closed at 70,000.00 USD on 2026-04-05.", "price", m("price", 70000, "date", "2026-04-05", "currency", "USD"), "ev_btc_prices", "2026-04-05,70000.00,demo-exchange")
				},
				func() error {
					return fact(b, "s4_f_price_0418", "BTC closed at 72,000.00 USD on 2026-04-18.", "price", m("price", 72000, "date", "2026-04-18", "currency", "USD"), "ev_btc_prices", "2026-04-18,72000.00,demo-exchange")
				},
				func() error {
					return fact(b, "s4_f_price_0429", "BTC closed at 71,000.00 USD on 2026-04-29.", "price", m("price", 71000, "date", "2026-04-29", "currency", "USD"), "ev_btc_prices", "2026-04-29,71000.00,demo-exchange")
				},
				func() error {
					return judgment(b, "s2_j_principal", "Customer identification, principal versus agent and nature of the reward",
						"Is there a customer for DemoCo's mining service, is DemoCo the principal, and is the block reward revenue under ASC 606 (directly or by analogy) or other income?",
						"DemoCo is the principal: it controls the hash power, builds its own block templates and bears the cost and risk of finding no block. No pool or hosting counterparty contracts for the service and the protocol is not a party that can be invoiced, so ASC 606 is applied by analogy for recognition and noncash-consideration measurement, with the reward presented as mining revenue in a separate caption.",
						"ASC 606-10-15-3 customer definition; ASC 606-10-55-36 to 55-40 control indicators; ASC 606-10-32-21 to 32-23 noncash consideration (by analogy); AICPA Digital Assets Practice Aid mining Q&A (block subsidy and transaction fees as consideration for the mining service); interpretive views differ on whether the protocol or the transaction originators are customers, so the conclusion is a judgment.",
						vwp.JudgmentAlternative{Conclusion: "ASC 606 revenue with transaction originators and the protocol as customers", Notes: "Considered: supportable under the view that fee payers receive a validation service; changes the caption, not the measurement basis."}, "high")
				},
				func() error {
					return judgment(b, "s3_j_timing", "Recognition timing and pricing date of block rewards",
						"Is a block reward recognized when the block is found and the coinbase output pays to DemoCo's address, when the output matures, or when it is spent — and which date's price measures it?",
						"Recognized on the date the block is accepted and its coinbase output pays to the DemoCo-controlled address: the amount is fixed by the protocol and the asset is DemoCo's, with the 100-block maturity a spending restriction rather than a condition of entitlement; a reorganization that orphans the block before maturity reverses the reward (none occurred). Each reward is measured at the fair value on that recognition date (pricingDateBasis recognitionDate); subsequent price movements are ASC 350-60 remeasurement, not mining revenue. No block straddles the 2026-04-30 reporting boundary.",
						"ASC 606-10-25-30 (control) and 606-10-32-11 constraint, applied by analogy — the orphan risk lapses within hours, so no amount is constrained at period end; ASC 606-10-32-21 noncash consideration measured at inception fair value — each block is its own activity, so the recognition-date price is the inception-equivalent; ASC 350-30-30-1 initial measurement of an intangible received.",
						vwp.JudgmentAlternative{Conclusion: "Recognize at coinbase maturity", Notes: "Considered: defers recognition about 17 hours; would differ only for a block found within 100 blocks of a reporting date — then recognize the constrained amount and true up at maturity."}, "medium")
				},
				func() error {
					return calc(b, "s4_mining_schedule", "miningRevenueSchedule",
						"Each block reward (subsidy + fees) at units x closing price on its recognition date (block accepted and coinbase output paid to the DemoCo address, so inception, recognition and receipt fall on the same day); totals tie to the node export; no unreceived balance at either period end.",
						validators.ValidatorMiningRewardsRecompute,
						m("asset", "BTC", "currency", "USD", "operatorType", "solo", "arrangement", "solo-demo-valley", "pricingDateBasis", "recognitionDate", "periodStart", "2026-04-01", "periodEnd", "2026-04-30", "chainObservedUnits", 9.545, "chainObservedFact", "s4_f_chain_total", "rewards", []any{
							m("date", "2026-04-05", "receivedOn", "2026-04-05", "measurementDate", "2026-04-05", "units", 3.175, "feeUnits", 0.05, "unitPrice", 70000, "priceFact", "s4_f_price_0405", "reference", "block-890100"),
							m("date", "2026-04-18", "receivedOn", "2026-04-18", "measurementDate", "2026-04-18", "units", 3.205, "feeUnits", 0.08, "unitPrice", 72000, "priceFact", "s4_f_price_0418", "reference", "block-890900"),
							m("date", "2026-04-29", "receivedOn", "2026-04-29", "measurementDate", "2026-04-29", "units", 3.165, "feeUnits", 0.04, "unitPrice", 71000, "priceFact", "s4_f_price_0429", "reference", "block-891700"),
						}),
						m("rows", []any{
							m("date", "2026-04-05", "units", 3.175, "fairValue", 222250),
							m("date", "2026-04-18", "units", 3.205, "fairValue", 230760),
							m("date", "2026-04-29", "units", 3.165, "fairValue", 224715),
						}, "totalUnits", 9.545, "totalFairValue", 677725))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorMiningAccountingScheduleTieOut, []string{"s4_mining_schedule"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "April 2026 mining revenue", Kind: accountingschedule.KindMiningRevenue, Currency: "USD", AmountScale: 2,
						StartDate: "2026-04-01", EndDate: "2026-04-30", TotalAmount: "677725.00",
						Entries: []accountingschedule.Entry{
							entry("block-890100", "2026-04-05", "3.175 BTC block reward (3.125 subsidy + 0.05 fees), block 890100, at 70,000.00", "block-890100", dr(validators.RoleCryptoAsset, "222250.00"), cr(validators.RoleMiningRevenue, "222250.00")),
							entry("block-890900", "2026-04-18", "3.205 BTC block reward (3.125 subsidy + 0.08 fees), block 890900, at 72,000.00", "block-890900", dr(validators.RoleCryptoAsset, "230760.00"), cr(validators.RoleMiningRevenue, "230760.00")),
							entry("block-891700", "2026-04-29", "3.165 BTC block reward (3.125 subsidy + 0.04 fees), block 891700, at 71,000.00", "block-891700", dr(validators.RoleCryptoAsset, "224715.00"), cr(validators.RoleMiningRevenue, "224715.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_arrangement", statement: "DemoCo mines solo with its own fleet, receives every coinbase output to an address it controls, and uses no pool and sells no hash power.",
						conclusion: "Arrangement facts established.", refs: []string{"ASC 606-10-55-36"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s1_f_solo", "s1_f_no_pool"}, evidence: []string{"ev_mining_ops"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_principal", statement: "DemoCo is the principal for the mining service; with no customer identified, ASC 606 is applied by analogy and block rewards are presented as mining revenue.",
						conclusion: "Principal; ASC 606 by analogy; mining revenue caption.", refs: []string{"ASC 606-10-55-36", "ASC 606-10-15-3"}, assertions: []string{"occurrence", "presentationDisclosure"},
						facts: []string{"s1_f_solo"}, judgments: []string{"s2_j_principal"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_timing", statement: "Block rewards are recognized on the date the block is accepted and its coinbase output pays to the DemoCo-controlled address, measured at that date's price; the 100-block maturity is a spending restriction, not a recognition criterion.",
						conclusion: "Recognize on block acceptance; recognition-date price.", refs: []string{"ASC 606-10-25-30", "ASC 606-10-32-21"}, assertions: []string{"occurrence"},
						facts: []string{"s3_f_maturity"}, judgments: []string{"s3_j_timing"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_measurement", statement: "April 2026 block rewards of 9.545 BTC (subsidy 9.375 plus fees 0.17) are measured at recognition-date fair value totalling 677,725 USD and tie to the node export.",
						conclusion: "Schedule recomputes and ties to chain.", refs: []string{"ASC 606-10-32-21", "ASC 350-30-30-1"}, assertions: []string{"completeness", "valuationAllocation"},
						facts: []string{"s4_f_chain_total", "s4_f_price_0405", "s4_f_price_0418", "s4_f_price_0429"}, calcs: []string{"s4_mining_schedule"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_subsequent", statement: "Mined BTC is an in-scope crypto asset subsequently measured at fair value under ASC 350-60 with remeasurement in net income; mining revenue is presented in its own caption with the fleet's energy and depreciation as cost of revenue.",
						conclusion: "Subsequent accounting under ASC 350-60; separate revenue caption.", refs: []string{"ASC 350-60-35-1", "ASC 350-60-45-1"}, assertions: []string{"valuationAllocation", "presentationDisclosure"},
						calcs: []string{"s4_mining_schedule"}})
				},
			)
		},
	})
}
