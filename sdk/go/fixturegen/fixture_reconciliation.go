//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// Period-end reconciliation and flux: one exchange account reconciled to
// the ledger at quarter end with a timing difference and an unrecorded
// fee, and a flux analysis of the digital-asset balance sheet lines. All
// content is synthetic demo data.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "period-end-reconciliation", extension: vwp.ExtensionReconciliation, domain: "accounting.reconciliation",
		title:   "Period-End Reconciliation and Flux - DemoCo Exchange Account, Q2 2026",
		summary: "The exchange account reconciles at 2026-06-30: the 2,650.00 USD difference between the statement (1,250,000.00) and the ledger (1,247,350.00) is a 3,000.00 deposit in transit and a 350.00 trading fee the ledger had not recorded; the fee is booked, the deposit clears itself. The quarter's flux explains the one variance above threshold (digital assets, +42%) with the ETH purchase.",
		refs:    []string{"AU-C 500", "ASC 230-10-45"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_exchange_statement", "custodianStatement", "Exchange account statement as of 2026-06-30", "evidence/exchange-statement.txt",
						"Exchange account DEMO-EX-01, statement as of 2026-06-30 23:59 UTC\n\nUSD balance: 1,250,000.00\n\nActivity 2026-06-30:\n- 2026-06-30 18:40 UTC wire deposit received: 3,000.00 (ref WIRE-0630)\n- 2026-06-30 trading fee charged: 350.00 (ref FEE-Q2-DEMO-EX-01)\n")
				},
				func() error {
					return ev(b, "ev_ledger_extract", "subledgerExtract", "General ledger balances, exchange cash and digital asset lines, 2026-03-31 and 2026-06-30", "evidence/ledger-extract.csv",
						"account,2026-03-31,2026-06-30\nexchange_cash,1230000.00,1247350.00\ndigital_assets,1850000.00,2627000.00\naccrued_exchange_fees,1200.00,1150.00\n")
				},
				func() error {
					return ev(b, "ev_wire_confirmation", "bankRecord", "Outbound wire confirmation, 2026-06-30", "evidence/wire-confirmation.txt",
						"Wire WIRE-0630 sent 2026-06-30 18:02 UTC to DEMO-EX-01: 3,000.00 USD. Recorded in the ledger on 2026-07-01 (cutoff).\n")
				},
				func() error {
					return ev(b, "ev_eth_purchase", "tradeConfirmation", "ETH purchase confirmation, 2026-05-20", "evidence/eth-purchase.txt",
						"2026-05-20: bought 300 ETH at 2,590.00 USD = 777,000.00 USD on DEMO-EX-01.\n")
				},
				func() error {
					return fact(b, "s1_f_source_balance", "The exchange statement shows a USD balance of 1,250,000.00 at 2026-06-30.", "amount", m("amount", 1250000, "date", "2026-06-30"), "ev_exchange_statement", "USD balance: 1,250,000.00")
				},
				func() error {
					return fact(b, "s1_f_ledger_balance", "The ledger exchange_cash balance was 1,247,350.00 at 2026-06-30.", "amount", m("amount", 1247350, "date", "2026-06-30"), "ev_ledger_extract", "exchange_cash,1230000.00,1247350.00")
				},
				func() error {
					return fact(b, "s2_f_deposit_in_transit", "A 3,000.00 USD wire reached the exchange on 2026-06-30 and was recorded in the ledger on 2026-07-01.", "amount", m("amount", 3000, "date", "2026-06-30"), "ev_wire_confirmation", "Recorded in the ledger on 2026-07-01 (cutoff).")
				},
				func() error {
					return fact(b, "s2_f_trading_fee", "The exchange charged a 350.00 USD trading fee on 2026-06-30 that the ledger had not recorded.", "amount", m("amount", 350, "date", "2026-06-30"), "ev_exchange_statement", "trading fee charged: 350.00")
				},
				func() error {
					return fact(b, "s3_f_eth_purchase", "300 ETH were bought on 2026-05-20 for 777,000.00 USD.", "amount", 777000, "ev_eth_purchase", "bought 300 ETH at 2,590.00 USD = 777,000.00 USD")
				},
				func() error {
					return judgment(b, "s4_j_fee_omission", "Booking the unrecorded exchange fee for scope DEMO-EX-01",
						"Is the 350.00 USD trading fee an omission the ledger must record at 2026-06-30, and in which accounts?",
						"Yes. The fee was charged within the period and reduces the exchange cash balance; it is recorded as exchange fee expense against exchange cash as of 2026-06-30. The 3,000.00 deposit in transit is a timing difference that clears on 2026-07-01 and is not booked.",
						"s2_f_trading_fee: the statement shows the fee charged on 2026-06-30; s2_f_deposit_in_transit: the wire was recorded one day late at cutoff. Omissions are booked; timing differences are carried as reconciling items only.",
						vwp.JudgmentAlternative{Conclusion: "Accrue the fee through accrued_exchange_fees", Notes: "Rejected: the fee was already charged to the account balance, not accrued."}, "low")
				},
				func() error {
					return calc(b, "s2_balance_reconciliation", "balanceReconciliation",
						"difference = sourceBalance - ledgerBalance; explained = sum of explained items (signed source minus ledger); unexplained = difference - explained. Deposit in transit +3,000.00 (ledger lags), trading fee -350.00 (ledger overstated).",
						validators.ValidatorReconciliationBalanceRecompute,
						m("scope", "DEMO-EX-01", "asOf", "2026-06-30", "currency", "USD",
							"sourceBalance", 1250000, "sourceFact", "s1_f_source_balance",
							"ledgerBalance", 1247350, "ledgerFact", "s1_f_ledger_balance",
							"items", []any{
								m("id", "RI-1", "description", "Wire deposit in transit at cutoff, recorded in the ledger 2026-07-01", "amount", 3000, "category", "inTransit", "status", "explained", "bookable", false, "supportRef", "s2_f_deposit_in_transit"),
								m("id", "RI-2", "description", "Trading fee charged by the exchange on 2026-06-30, not recorded in the ledger", "amount", -350, "category", "omission", "status", "explained", "bookable", true, "debitRole", "exchange_fee_expense", "creditRole", "exchange_cash", "supportRef", "s2_f_trading_fee"),
							}),
						m("difference", 2650, "explained", 2650, "unexplained", 0))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorReconciliationAccountingScheduleTieOut, []string{"s2_balance_reconciliation"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "DEMO-EX-01 reconciling adjustments, 2026-06-30", Kind: accountingschedule.KindReconcilingAdjustment, Currency: "USD", AmountScale: 2,
						StartDate: "2026-06-30", EndDate: "2026-06-30", TotalAmount: "350.00",
						Entries: []accountingschedule.Entry{
							entry("RI-2-2026-06-30", "2026-06-30", "Exchange trading fee charged 2026-06-30, omitted from the ledger", "RI-2", dr("exchange_fee_expense", "350.00"), cr("exchange_cash", "350.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_digital_assets_flux", statement: "Digital assets rose from 1,850,000.00 to 2,627,000.00 USD (+42%) because 300 ETH were bought on 2026-05-20 for 777,000.00 USD.",
						conclusion: "Variance explained by the ETH purchase.", refs: []string{"ASC 350-60-50-3"}, assertions: []string{"completeness", "valuationAllocation"},
						facts: []string{"s3_f_eth_purchase"}, evidence: []string{"ev_eth_purchase", "ev_ledger_extract"}})
				},
				func() error {
					return calc(b, "s3_flux_analysis", "fluxAnalysis",
						"Per account: variance = current - prior; variancePct = variance / |prior|; a variance above 100,000.00 USD or 10% must be explained by a claim.",
						validators.ValidatorReconciliationFluxRecompute,
						m("currency", "USD", "priorPeriodEnd", "2026-03-31", "currentPeriodEnd", "2026-06-30", "thresholdAmount", 100000, "thresholdPct", 0.10, "lines", []any{
							m("account", "exchange_cash", "prior", 1230000, "current", 1247350),
							m("account", "digital_assets", "prior", 1850000, "current", 2627000, "explanationClaim", "s3_c_digital_assets_flux"),
							m("account", "accrued_exchange_fees", "prior", 1200, "current", 1150),
						}),
						m("rows", []any{
							m("account", "exchange_cash", "variance", 17350, "variancePct", 0.014105691056910569),
							m("account", "digital_assets", "variance", 777000, "variancePct", 0.42),
							m("account", "accrued_exchange_fees", "variance", -50, "variancePct", -0.041666666666666664),
						}, "totalPrior", 3081200, "totalCurrent", 3875500))
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_scope", statement: "The reconciliation covers exchange account DEMO-EX-01 at 2026-06-30: the statement balance of 1,250,000.00 USD and the ledger exchange_cash balance of 1,247,350.00 USD, both as of the cutoff.",
						conclusion: "Scope and sources established with balances as of the reconciliation date.", refs: []string{"AU-C 500"}, assertions: []string{"existence", "completeness"},
						facts: []string{"s1_f_source_balance", "s1_f_ledger_balance"}, evidence: []string{"ev_exchange_statement", "ev_ledger_extract"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_reconciled", statement: "The 2,650.00 USD difference is fully explained by a 3,000.00 USD deposit in transit and a 350.00 USD trading fee not yet recorded; nothing is unexplained.",
						conclusion: "Reconciled with no unexplained residual.", refs: []string{"AU-C 500"}, assertions: []string{"completeness", "accuracy"},
						facts: []string{"s2_f_deposit_in_transit", "s2_f_trading_fee"}, calcs: []string{"s2_balance_reconciliation"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_flux", statement: "Of the three balance sheet lines, only digital assets moved beyond the 100,000.00 USD / 10% thresholds in Q2 2026, and that variance is explained.",
						conclusion: "Flux analysis complete; every variance above threshold is explained.", refs: []string{"AU-C 520"}, assertions: []string{"completeness"},
						calcs: []string{"s3_flux_analysis"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_adjustment", statement: "One adjusting entry is proposed: 350.00 USD of exchange fee expense against exchange cash as of 2026-06-30; the deposit in transit is not booked.",
						conclusion: "Omission booked through the executable schedule; timing difference carried.", refs: []string{"ASC 230-10-45"}, assertions: []string{"accuracy", "cutoff"},
						facts: []string{"s2_f_trading_fee"}, calcs: []string{"s2_balance_reconciliation", "s4_accounting_schedule"}, judgments: []string{"s4_j_fee_omission"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_conclusion", statement: "The exchange account is reconciled at 2026-06-30 with one adjustment proposed and no unexplained items; the package is ready for reviewer sign-off.",
						conclusion: "Ready for sign-off.", refs: []string{"AU-C 500"}, assertions: []string{"completeness"},
						calcs: []string{"s2_balance_reconciliation", "s3_flux_analysis"}, judgments: []string{"s4_j_fee_omission"}})
				},
			)
		},
	})
}
