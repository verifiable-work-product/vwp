//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// asc740-income-taxes: one jurisdiction, one fiscal year, a synthetic 25%
// statutory rate, two permanent and three temporary differences, a
// valuation allowance released as the loss carryforward is used, an
// uncertain tax position, the rate reconciliation, and the provision
// entries (current, unrecognized tax benefits, deferred). Synthetic data.
//
// Figures: pretax 1,000,000; permanent +30,000; deductible accrued
// compensation 100,000 → 150,000 (+50,000 to taxable income); taxable
// depreciation 200,000 → 260,000 (−60,000); deductible loss carryforward
// 80,000 → 0 (−80,000). Taxable income 940,000; current 235,000. Gross DTA
// 45,000 → 37,500; allowance 20,000 → 0; DTL 50,000 → 65,000; net deferred
// −25,000 → −27,500; deferred expense 2,500. UTB 15,000 → 25,000 (+10,000).
// Total 247,500; effective rate 24.75%.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "asc740-income-taxes", extension: vwp.ExtensionASC740, domain: "accounting.incomeTaxes",
		title:   "ASC 740 Income Tax Provision - DemoCo FY2026",
		summary: "At a 25% statutory rate on 1,000,000 USD of pretax income, DemoCo's FY2026 provision is 247,500 USD: current tax 235,000 on taxable income of 940,000 after permanent and temporary differences, a 10,000 increase in unrecognized tax benefits, and deferred tax expense of 2,500 as the net deferred position moves from a 25,000 liability to a 27,500 liability (the 20,000 valuation allowance released as the loss carryforward is used). Effective rate 24.75%.",
		refs:    []string{"ASC 740-10-25-2", "ASC 740-10-30-5", "ASC 740-10-25-6", "ASC 740-10-50-12"},
		period:  []string{"2026-01-01", "2026-12-31"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_trial_balance", "trialBalance", "FY2026 pretax trial balance summary", "evidence/trial-balance-fy2026.csv",
						"line,amount_usd\nrevenue,6400000.00\nexpenses,-5400000.00\npretax_income,1000000.00\nnondeductible_fines,20000.00\nmeals_disallowed_portion,10000.00\n")
				},
				func() error {
					return ev(b, "ev_tax_basis_schedule", "taxWorkpaper", "Book-tax basis differences at 2025-12-31 and 2026-12-31", "evidence/basis-differences.csv",
						"item,kind,opening_difference_usd,closing_difference_usd,rate\naccrued_compensation,deductible,100000.00,150000.00,0.25\naccelerated_depreciation,taxable,200000.00,260000.00,0.25\nloss_carryforward,deductible,80000.00,0.00,0.25\nvaluation_allowance,,20000.00,0.00,\nunrecognized_tax_benefits,,15000.00,25000.00,\n")
				},
				func() error {
					return ev(b, "ev_rate_memo", "taxMemo", "Enacted rate memo for the demo jurisdiction", "evidence/enacted-rate-memo.txt",
						"Demo jurisdiction corporate income tax\n\nThe enacted rate applying to fiscal year 2026 and to the reversal of all scheduled temporary differences is 25.0%. No rate change has been enacted.\n")
				},
				func() error {
					return fact(b, "s2_f_pretax_income", "FY2026 pretax book income is 1,000,000 USD.", "amount", m("amount", 1000000, "currency", "USD", "date", "2026-12-31"), "ev_trial_balance", "pretax_income,1000000.00")
				},
				func() error {
					return fact(b, "s1_f_statutory_rate", "The enacted rate for FY2026 and for the reversal of all scheduled differences is 25%.", "rate", m("rate", 0.25, "date", "2026-12-31"), "ev_rate_memo", "The enacted rate applying to fiscal year 2026 and to the reversal of all scheduled temporary differences is 25.0%.")
				},
				func() error {
					return fact(b, "s2_f_permanent", "Non-deductible fines of 20,000 USD and the disallowed 10,000 USD portion of meals are permanent differences.", "amount", m("fines", 20000, "meals", 10000, "currency", "USD"), "ev_trial_balance", "nondeductible_fines,20000.00")
				},
				func() error {
					return fact(b, "s3_f_differences", "Cumulative book-tax differences: accrued compensation 100,000 to 150,000 USD (deductible), accelerated depreciation 200,000 to 260,000 USD (taxable), loss carryforward 80,000 to 0 USD (deductible), all reversing at 25%.", "schedule", m("accruedCompensationClosing", 150000, "depreciationClosing", 260000, "lossCarryforwardClosing", 0), "ev_tax_basis_schedule", "accrued_compensation,deductible,100000.00,150000.00,0.25")
				},
				func() error {
					return fact(b, "s4_f_allowance_and_utb", "The valuation allowance of 20,000 USD at 2025-12-31 is released to zero as the loss carryforward is used; unrecognized tax benefits rise from 15,000 to 25,000 USD.", "amount", m("valuationAllowanceClosing", 0, "unrecognizedTaxBenefitsClosing", 25000, "currency", "USD"), "ev_tax_basis_schedule", "valuation_allowance,,20000.00,0.00,")
				},
				func() error {
					return judgment(b, "s4_j_valuation_allowance", "Valuation allowance on deferred tax assets",
						"Is it more likely than not that some portion of the deferred tax assets will not be realized at 2026-12-31?",
						"No. The 20,000 USD allowance carried at 2025-12-31 against the loss carryforward is released: the carryforward was fully used against FY2026 taxable income, the remaining deferred tax asset (accrued compensation, 37,500 USD) reverses as the compensation is paid, and taxable temporary differences of 260,000 USD provide a source of future taxable income exceeding it. No allowance is required at 2026-12-31.",
						"ASC 740-10-30-5(e) and 30-17 (more-likely-than-not realizability; sources of taxable income including reversing taxable differences); ASC 740-10-30-18; basis difference schedule.",
						vwp.JudgmentAlternative{Conclusion: "Retain a partial allowance", Notes: "Rejected: the taxable temporary differences alone exceed the remaining deferred tax asset and reverse in the same periods."}, "high")
				},
				func() error {
					return judgment(b, "s4_j_uncertain_positions", "Unrecognized tax benefits",
						"Which tax positions taken in the return fail the more-likely-than-not recognition threshold, and what liability is measured for them?",
						"One position (the deductibility of a cross-border service charge) does not meet the more-likely-than-not threshold on its technical merits; the full benefit of 25,000 USD at 2026-12-31 (15,000 USD at 2025-12-31) is unrecognized and carried as a liability, with the 10,000 USD increase in FY2026 tax expense. No interest or penalties accrue yet.",
						"ASC 740-10-25-6 (recognition threshold); ASC 740-10-30-7 (measurement at the largest amount more than 50% likely of being realized); ASC 740-10-45-11 (presented as a liability, not with taxes payable); basis difference schedule.",
						vwp.JudgmentAlternative{Conclusion: "Recognize the full benefit", Notes: "Rejected: the position's technical merits do not reach more likely than not."}, "high")
				},
				func() error {
					return calc(b, "s3_tax_provision", "incomeTaxProvision",
						"taxableIncome = pretaxIncome + permanent differences + change in deductible temporary differences - change in taxable temporary differences; currentTaxExpense = taxableIncome x statutoryRate; deferred tax assets/liabilities = cumulative differences x enacted rate; net deferred = DTA - valuation allowance - DTL; deferredTaxExpense = net deferred opening - closing; uncertainTaxPositionsExpense = change in unrecognized tax benefits; total = current + UTB + deferred; effectiveRate = total / pretax.",
						validators.ValidatorASC740ProvisionRecompute,
						m("currency", "USD", "periodStart", "2026-01-01", "periodEnd", "2026-12-31", "jurisdiction", "Demo jurisdiction",
							"statutoryRate", 0.25, "statutoryRateFact", "s1_f_statutory_rate", "pretaxIncome", 1000000, "pretaxIncomeFact", "s2_f_pretax_income",
							"permanentDifferences", []any{
								m("id", "pd_fines", "description", "Non-deductible fines", "amount", 20000),
								m("id", "pd_meals", "description", "Disallowed portion of meals", "amount", 10000),
							},
							"temporaryDifferences", []any{
								m("id", "td_accrued_compensation", "description", "Accrued compensation deductible when paid", "kind", "deductible", "openingDifference", 100000, "closingDifference", 150000, "rate", 0.25),
								m("id", "td_depreciation", "description", "Accelerated tax depreciation", "kind", "taxable", "openingDifference", 200000, "closingDifference", 260000, "rate", 0.25),
								m("id", "td_loss_carryforward", "description", "Loss carryforward used in FY2026", "kind", "deductible", "openingDifference", 80000, "closingDifference", 0, "rate", 0.25),
							},
							"valuationAllowance", m("opening", 20000, "closing", 0, "judgmentRef", "s4_j_valuation_allowance"),
							"uncertainTaxPositions", m("opening", 15000, "closing", 25000, "judgmentRef", "s4_j_uncertain_positions")),
						m("taxableIncome", 940000, "currentTaxExpense", 235000, "uncertainTaxPositionsExpense", 10000, "deferredTaxExpense", 2500, "totalTaxExpense", 247500, "effectiveRate", 0.2475,
							"deferredTaxAssetOpening", 45000, "deferredTaxAssetClosing", 37500, "deferredTaxLiabilityOpening", 50000, "deferredTaxLiabilityClosing", 65000,
							"netDeferredOpening", -25000, "netDeferredClosing", -27500,
							"rateReconciliation", []any{
								m("item", "statutory", "amount", 250000),
								m("item", "permanent differences", "amount", 7500),
								m("item", "valuation allowance release", "amount", -20000),
								m("item", "unrecognized tax benefits", "amount", 10000),
							}))
				},
				func() error {
					return schedule(b, "s5_accounting_schedule", validators.ValidatorASC740AccountingScheduleTieOut, []string{"s3_tax_provision"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "FY2026 income tax provision entries", Kind: accountingschedule.KindIncomeTaxProvision, Currency: "USD", AmountScale: 2,
						StartDate: "2026-12-31", EndDate: "2026-12-31", TotalAmount: "260000.00",
						Entries: []accountingschedule.Entry{
							entry("current-2026-12-31", "2026-12-31", "Current tax on taxable income of 940,000 at 25%", "current",
								dr(validators.RoleIncomeTaxExpense, "235000.00"), cr(validators.RoleIncomeTaxesPayable, "235000.00")),
							entry("utb-2026-12-31", "2026-12-31", "Increase in the liability for unrecognized tax benefits", "uncertainTaxPositions",
								dr(validators.RoleIncomeTaxExpense, "10000.00"), cr(validators.RoleUnrecognizedTaxBenefits, "10000.00")),
							entry("deferred-2026-12-31", "2026-12-31", "Deferred tax: net deferred tax asset up 12,500 (allowance released), deferred tax liability up 15,000, deferred expense 2,500", "deferred",
								dr(validators.RoleDeferredTaxAsset, "12500.00"), dr(validators.RoleIncomeTaxExpense, "2500.00"), cr(validators.RoleDeferredTaxLiability, "15000.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_scope", statement: "DemoCo files in one jurisdiction at an enacted 25% rate with no enacted change; the provision covers FY2026 under ASC 740.",
						conclusion: "Single jurisdiction, enacted rate established.", refs: []string{"ASC 740-10-15-2", "ASC 740-10-30-8"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s1_f_statutory_rate"}, evidence: []string{"ev_rate_memo"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_current", statement: "Taxable income is 940,000 USD after 30,000 USD of permanent differences and the net 90,000 USD reduction from temporary differences, giving current tax expense of 235,000 USD.",
						conclusion: "Current tax recomputes from pretax income and the differences.", refs: []string{"ASC 740-10-25-2", "ASC 740-10-20"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s2_f_pretax_income", "s2_f_permanent"}, calcs: []string{"s3_tax_provision"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_deferred", statement: "Deferred tax assets fall from 45,000 to 37,500 USD and deferred tax liabilities rise from 50,000 to 65,000 USD at 25%, moving the net deferred position from a 25,000 USD liability to a 27,500 USD liability and producing 2,500 USD of deferred tax expense.",
						conclusion: "Deferred balances recompute from the cumulative differences at the enacted rate.", refs: []string{"ASC 740-10-25-2", "ASC 740-10-30-8", "ASC 740-10-45-4"}, assertions: []string{"valuationAllocation", "completeness"},
						facts: []string{"s3_f_differences"}, calcs: []string{"s3_tax_provision"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_allowance_utb", statement: "The 20,000 USD valuation allowance is released because realization of the remaining deferred tax asset is more likely than not; unrecognized tax benefits of 25,000 USD are carried as a liability, 10,000 USD of which arose in FY2026.",
						conclusion: "Allowance released; uncertain position measured and presented as a liability.", refs: []string{"ASC 740-10-30-5", "ASC 740-10-25-6", "ASC 740-10-45-11"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s4_f_allowance_and_utb"}, judgments: []string{"s4_j_valuation_allowance", "s4_j_uncertain_positions"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_rate_reconciliation", statement: "The 247,500 USD provision reconciles from the 250,000 USD statutory expectation through permanent differences (7,500), the allowance release (-20,000) and unrecognized tax benefits (10,000), an effective rate of 24.75%; the provision entries book current tax, the unrecognized-benefit liability and the deferred movement.",
						conclusion: "Rate reconciliation ties; provision entries derived from the calculation.", refs: []string{"ASC 740-10-50-12", "ASC 740-10-50-9"}, assertions: []string{"presentationDisclosure", "valuationAllocation"},
						calcs: []string{"s3_tax_provision", "s5_accounting_schedule"}})
				},
			)
		},
	})
}
