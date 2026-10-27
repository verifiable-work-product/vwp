//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// ASC 830 example: a USD-functional entity remeasures a EUR cash balance
// and a GBP payable at the quarter-end spot rates; a EUR prepayment is
// non-monetary and stays at its historical amount.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "asc830-foreign-currency", extension: vwp.ExtensionASC830, domain: "accounting.foreignCurrency",
		title:   "ASC 830 Foreign Currency Remeasurement - DemoCo Q2 2026",
		summary: "DemoCo's functional currency is the US dollar. At 2026-06-30 its EUR cash of 100,000 remeasures from 108,000.00 to 110,000.00 USD (a 2,000.00 gain) and its GBP trade payable of 50,000 from 63,000.00 to 64,000.00 USD (a 1,000.00 loss); the EUR prepayment is non-monetary and stays at 21,600.00 USD. The net transaction gain of 1,000.00 USD is in income.",
		refs:    []string{"ASC 830-20-35-1", "ASC 830-10-45-18"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_fx_rates", "pricingRecord", "Quarter-end spot rates, 2026-06-30", "evidence/spot-rates-2026-06-30.csv",
						"pair,rate,source,asof\nEUR/USD,1.1000,demo-rate-service,2026-06-30\nGBP/USD,1.2800,demo-rate-service,2026-06-30\n")
				},
				func() error {
					return ev(b, "ev_fx_balances", "ledgerExtract", "Foreign-currency balances at 2026-06-30", "evidence/fx-balances-2026-06-30.txt",
						"Foreign-currency denominated balances as of 2026-06-30\n\nCash - EUR account: EUR 100,000.00, carried at USD 108,000.00 (booked at 1.0800)\nTrade payables - GBP: GBP 50,000.00, carried at USD 63,000.00 (booked at 1.2600)\nPrepaid conference fees - EUR: EUR 20,000.00, carried at USD 21,600.00 (historical rate 1.0800)\n")
				},
				func() error {
					return ev(b, "ev_functional_currency_memo", "analysisMemo", "Functional currency assessment", "evidence/functional-currency-memo.txt",
						"Functional currency assessment\n\nDemoCo's sales are priced and settled in US dollars, its financing is raised in US dollars, its payroll and most operating costs are paid in US dollars, and its cash flows do not affect a parent's foreign cash flows. The euro and sterling balances are isolated transactions. The functional currency is the US dollar.\n")
				},
				func() error {
					return fact(b, "s1_f_functional_indicators", "DemoCo prices and settles sales, raises financing and pays most costs in US dollars.", "term", "USD cash flow, sales price, financing and expense indicators", "ev_functional_currency_memo", "sales are priced and settled in US dollars, its financing is raised in US dollars, its payroll and most operating costs are paid in US dollars")
				},
				func() error {
					return fact(b, "s2_f_cash_eur", "The EUR cash balance is EUR 100,000.00, carried at USD 108,000.00.", "amount", m("foreignAmount", 100000, "carryingAmount", 108000, "currency", "EUR"), "ev_fx_balances", "Cash - EUR account: EUR 100,000.00, carried at USD 108,000.00")
				},
				func() error {
					return fact(b, "s2_f_ap_gbp", "The GBP trade payable is GBP 50,000.00, carried at USD 63,000.00.", "amount", m("foreignAmount", 50000, "carryingAmount", 63000, "currency", "GBP"), "ev_fx_balances", "Trade payables - GBP: GBP 50,000.00, carried at USD 63,000.00")
				},
				func() error {
					return fact(b, "s2_f_prepaid_eur", "The EUR prepayment is EUR 20,000.00, carried at USD 21,600.00 at its historical rate.", "amount", m("foreignAmount", 20000, "carryingAmount", 21600, "currency", "EUR"), "ev_fx_balances", "Prepaid conference fees - EUR: EUR 20,000.00, carried at USD 21,600.00 (historical rate 1.0800)")
				},
				func() error {
					return fact(b, "s3_f_eur_rate", "EUR/USD closed at 1.1000 on 2026-06-30.", "rate", m("rate", 1.1, "date", "2026-06-30", "pair", "EUR/USD"), "ev_fx_rates", "EUR/USD,1.1000,demo-rate-service,2026-06-30")
				},
				func() error {
					return fact(b, "s3_f_gbp_rate", "GBP/USD closed at 1.2800 on 2026-06-30.", "rate", m("rate", 1.28, "date", "2026-06-30", "pair", "GBP/USD"), "ev_fx_rates", "GBP/USD,1.2800,demo-rate-service,2026-06-30")
				},
				func() error {
					return judgment(b, "s1_j_functional_currency", "Functional currency",
						"What is DemoCo's functional currency under ASC 830-10-45-2 to 45-6?",
						"The US dollar. The cash flow, sales price, sales market, expense, financing and intra-entity indicators of ASC 830-10-55-5 all point to the US dollar (s1_f_functional_indicators); the euro and sterling balances arise from isolated transactions that are remeasured, not translated.",
						"ASC 830-10-45-2 to 45-6 and 830-10-55-3 to 55-5; s1_f_functional_indicators.",
						vwp.JudgmentAlternative{Conclusion: "Euro functional currency", Notes: "Rejected: no primary economic environment indicator points to the euro."}, "medium")
				},
				func() error {
					return judgment(b, "s2_j_monetary_classification", "Monetary versus non-monetary items",
						"Which foreign-currency balances are monetary and remeasured at the current rate?",
						"Cash and the trade payable are monetary (fixed amounts of currency receivable or payable) and remeasure at the 2026-06-30 spot rates; the prepayment is non-monetary and stays at the historical rate (ASC 830-10-45-18 and the 830-10-20 definition of monetary assets and liabilities).",
						"ASC 830-10-20 (monetary assets and liabilities); ASC 830-10-45-17 and 45-18; s2_f_cash_eur; s2_f_ap_gbp; s2_f_prepaid_eur.",
						vwp.JudgmentAlternative{Conclusion: "Remeasure the prepayment too", Notes: "Rejected: a prepayment for services is settled in services, not in a fixed amount of currency."}, "low")
				},
				func() error {
					return calc(b, "s4_remeasurement", "foreignCurrencyRemeasurement",
						"Monetary items: remeasured = foreignAmount × spot rate at asOf (ASC 830-20-35-1); gainLoss = remeasured − carrying for a debit-balance item, carrying − remeasured for a credit-balance item, so a positive figure is a gain. Non-monetary items stay at carrying amount (830-10-45-18). netGainLoss is the sum.",
						validators.ValidatorASC830RemeasurementRecompute,
						m("functionalCurrency", "USD", "currency", "USD", "asOf", "2026-06-30", "items", []any{
							m("id", "cash_eur", "accountRole", "cash_eur", "balanceSide", "debit", "currency", "EUR", "foreignAmount", 100000, "monetary", true, "carryingAmount", 108000, "spotRate", 1.1, "rateFact", "s3_f_eur_rate"),
							m("id", "ap_gbp", "accountRole", "trade_payables_gbp", "balanceSide", "credit", "currency", "GBP", "foreignAmount", 50000, "monetary", true, "carryingAmount", 63000, "spotRate", 1.28, "rateFact", "s3_f_gbp_rate"),
							m("id", "prepaid_eur", "accountRole", "prepaid_expenses_eur", "balanceSide", "debit", "currency", "EUR", "foreignAmount", 20000, "monetary", false, "carryingAmount", 21600),
						}),
						m("rows", []any{
							m("id", "cash_eur", "remeasured", 110000, "gainLoss", 2000),
							m("id", "ap_gbp", "remeasured", 64000, "gainLoss", -1000),
							m("id", "prepaid_eur", "remeasured", 21600, "gainLoss", 0),
						}, "netGainLoss", 1000))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorASC830AccountingScheduleTieOut, []string{"s4_remeasurement"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "Q2 2026 foreign currency remeasurement", Kind: accountingschedule.KindFXRemeasurement, Currency: "USD", AmountScale: 2,
						StartDate: "2026-06-30", EndDate: "2026-06-30", TotalAmount: "3000.00",
						Entries: []accountingschedule.Entry{
							entry("cash_eur-2026-06-30", "2026-06-30", "EUR cash remeasured at 1.1000: transaction gain", "cash_eur", dr("cash_eur", "2000.00"), cr(validators.RoleFXGainLoss, "2000.00")),
							entry("ap_gbp-2026-06-30", "2026-06-30", "GBP trade payable remeasured at 1.2800: transaction loss", "ap_gbp", dr(validators.RoleFXGainLoss, "1000.00"), cr("trade_payables_gbp", "1000.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_functional_currency", statement: "DemoCo's functional currency is the US dollar; its euro and sterling balances are foreign-currency transactions to be remeasured.",
						conclusion: "USD functional currency.", refs: []string{"ASC 830-10-45-2", "ASC 830-10-45-5", "ASC 830-10-55-5"}, assertions: []string{"presentationDisclosure"},
						facts: []string{"s1_f_functional_indicators"}, judgments: []string{"s1_j_functional_currency"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_population", statement: "The foreign-currency balances at 2026-06-30 are EUR cash and a GBP trade payable (monetary) and a EUR prepayment (non-monetary).",
						conclusion: "Population classified.", refs: []string{"ASC 830-10-45-17", "ASC 830-10-45-18"}, assertions: []string{"completeness", "existence"},
						facts: []string{"s2_f_cash_eur", "s2_f_ap_gbp", "s2_f_prepaid_eur"}, judgments: []string{"s2_j_monetary_classification"}, evidence: []string{"ev_fx_balances"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_rates", statement: "The 2026-06-30 spot rates are 1.1000 USD per EUR and 1.2800 USD per GBP.",
						conclusion: "Current rates evidenced.", refs: []string{"ASC 830-20-35-1", "ASC 830-10-20"}, assertions: []string{"accuracy"},
						facts: []string{"s3_f_eur_rate", "s3_f_gbp_rate"}, evidence: []string{"ev_fx_rates"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_remeasurement", statement: "Remeasuring the monetary items at the 2026-06-30 spot rates produces a 2,000.00 USD gain on EUR cash and a 1,000.00 USD loss on the GBP payable, a net transaction gain of 1,000.00 USD, booked by the executable schedule.",
						conclusion: "Remeasurement recomputes and books.", refs: []string{"ASC 830-20-35-1", "ASC 830-20-35-2"}, assertions: []string{"valuationAllocation", "accuracy"},
						facts: []string{"s2_f_cash_eur", "s2_f_ap_gbp", "s3_f_eur_rate", "s3_f_gbp_rate"}, calcs: []string{"s4_remeasurement", "s4_accounting_schedule"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_presentation", statement: "The net transaction gain of 1,000.00 USD is presented in income and the aggregate transaction gain or loss is disclosed; no foreign entity exists, so no translation adjustment arises.",
						conclusion: "Presentation and disclosure per ASC 830-20-45 and 830-20-50.", refs: []string{"ASC 830-20-45-1", "ASC 830-20-50-1"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s4_remeasurement"}})
				},
			)
		},
	})
}
