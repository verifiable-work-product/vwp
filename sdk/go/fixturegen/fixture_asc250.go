//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// ASC 250 error correction: an accrued liability understated in two prior
// fiscal years, quantified under both SAB 108 methods, material under the
// iron-curtain method, corrected by restating the opening balance of the
// earliest period presented. All content is synthetic demo data.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "asc250-error-correction", extension: vwp.ExtensionASC250, domain: "accounting.errorCorrection",
		title:   "ASC 250 Error Correction - DemoCo Accrued Hosting Fees, FY2024-FY2025",
		summary: "Hosting fees of 15,000.00 USD (FY2024) and 10,000.00 USD (FY2025) were never accrued. The cumulative 25,000.00 USD misstatement is 6.25% of FY2025 pretax income under the iron-curtain method (the largest single-year effect is 3.75% under rollover); with the qualitative factors considered the error is material and is corrected by restatement as of 2026-01-01.",
		refs:    []string{"ASC 250-10-45-23", "SAB Topic 1.M (SAB 99)", "SAB Topic 1.N (SAB 108)"},
		period:  []string{"2024-01-01", "2025-12-31"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_hosting_invoices", "vendorInvoices", "Hosting vendor invoices for December 2024 and December 2025, received after year end", "evidence/hosting-invoices.txt",
						"Invoice H-2024-12 dated 2025-01-14 for December 2024 hosting: 15,000.00 USD (service period 2024-12-01 to 2024-12-31)\nInvoice H-2025-12 dated 2026-01-12 for December 2025 hosting: 10,000.00 USD (service period 2025-12-01 to 2025-12-31)\nNeither invoice was accrued at the respective year end.\n")
				},
				func() error {
					return ev(b, "ev_trial_balances", "subledgerExtract", "Accrued liabilities and pretax income as reported, FY2024 and FY2025", "evidence/trial-balances.csv",
						"line,FY2024,FY2025\naccrued_liabilities,120000.00,135000.00\npretax_income,350000.00,400000.00\n")
				},
				func() error {
					return fact(b, "s1_f_fy2024_omission", "December 2024 hosting of 15,000.00 USD was not accrued at 2024-12-31.", "amount", 15000, "ev_hosting_invoices", "Invoice H-2024-12 dated 2025-01-14 for December 2024 hosting: 15,000.00 USD")
				},
				func() error {
					return fact(b, "s1_f_fy2025_omission", "December 2025 hosting of 10,000.00 USD was not accrued at 2025-12-31.", "amount", 10000, "ev_hosting_invoices", "Invoice H-2025-12 dated 2026-01-12 for December 2025 hosting: 10,000.00 USD")
				},
				func() error {
					return fact(b, "s2_f_accrued_fy2024", "Accrued liabilities as reported at 2024-12-31 were 120,000.00 USD.", "amount", 120000, "ev_trial_balances", "accrued_liabilities,120000.00,135000.00")
				},
				func() error {
					return fact(b, "s2_f_accrued_fy2025", "Accrued liabilities as reported at 2025-12-31 were 135,000.00 USD.", "amount", 135000, "ev_trial_balances", "accrued_liabilities,120000.00,135000.00")
				},
				func() error {
					return fact(b, "s3_f_pretax_income_fy2025", "Pretax income as reported for FY2025 was 400,000.00 USD.", "amount", 400000, "ev_trial_balances", "pretax_income,350000.00,400000.00")
				},
				func() error {
					return judgment(b, "s3_j_materiality", "Materiality of the unaccrued hosting fees (SAB 99 / SAB 108)",
						"Is the cumulative 25,000.00 USD understatement of accrued liabilities, 6.25% of FY2025 pretax income under the iron-curtain method and 3.75% at most under rollover, material?",
						"Yes. The iron-curtain quantification exceeds the 5% threshold, the error affected two consecutive years, and it masks a change in the expense trend; no qualitative factor reduces its significance. The error is material and the prior periods are restated.",
						"s1_f_fy2024_omission and s1_f_fy2025_omission quantify the error; s3_f_pretax_income_fy2025 is the basis. SAB 99 qualitative factors considered: trend masking, no effect on covenants, no management compensation effect, not intentional.",
						vwp.JudgmentAlternative{Conclusion: "Immaterial under rollover; correct out of period in FY2026", Notes: "Rejected: SAB 108 requires both methods; the iron-curtain result is material."}, "high")
				},
				func() error {
					return judgment(b, "s4_j_method", "Correction method",
						"Is the error corrected by restating the prior periods (ASC 250-10-45-23) or as an out-of-period adjustment in FY2026?",
						"Restatement: the FY2024 and FY2025 financial statements are restated, the opening balance of accrued liabilities and retained earnings at 2026-01-01 is corrected through a prior-period adjustment, and the restatement is disclosed.",
						"ASC 250-10-45-23 to 45-24 require a material error in previously issued statements to be corrected by restatement; s3_j_materiality concludes the error is material.",
						vwp.JudgmentAlternative{Conclusion: "Out-of-period correction in FY2026", Notes: "Rejected: only available when the error is immaterial to every period affected and to the current period."}, "high")
				},
				func() error {
					return calc(b, "s2_error_correction", "errorCorrectionAnalysis",
						"Per period (chronological): asRestated = asReported + the cumulative correction through that period; cumulativeCorrection = sum of corrections. Rollover = largest single-period misstatement across lines; iron curtain = cumulative uncorrected misstatement at the correction date; each as a share of the basis (FY2025 pretax income).",
						validators.ValidatorASC250CorrectionRecompute,
						m("currency", "USD", "correctionDate", "2026-01-01", "materialityJudgmentRef", "s3_j_materiality",
							"lines", []any{
								m("id", "L1", "lineItem", "Accrued liabilities (hosting fees)", "accountRole", "accrued_liabilities", "offsetRole", validators.RolePriorPeriodAdjustment, "balanceSide", "credit",
									"periods", []any{
										m("period", "FY2024", "asReported", 120000, "correction", 15000),
										m("period", "FY2025", "asReported", 135000, "correction", 10000),
									}),
							},
							"materiality", m("quantitativeThreshold", 0.05, "basis", "FY2025 pretax income", "basisAmount", 400000, "basisFact", "s3_f_pretax_income_fy2025")),
						m("lines", []any{
							m("id", "L1", "periods", []any{
								m("period", "FY2024", "asRestated", 135000),
								m("period", "FY2025", "asRestated", 160000),
							}, "cumulativeCorrection", 25000),
						},
							"rollover", m("maxPeriodCorrection", 15000, "pctOfBasis", 0.0375),
							"ironCurtain", m("cumulativeCorrection", 25000, "pctOfBasis", 0.0625)))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorASC250AccountingScheduleTieOut, []string{"s2_error_correction"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "Restatement of accrued hosting fees as of 2026-01-01", Kind: accountingschedule.KindErrorCorrection, Currency: "USD", AmountScale: 2,
						StartDate: "2026-01-01", EndDate: "2026-01-01", TotalAmount: "25000.00",
						Entries: []accountingschedule.Entry{
							entry("L1-2026-01-01", "2026-01-01", "Prior-period adjustment: accrue FY2024 and FY2025 hosting fees", "L1", dr(validators.RolePriorPeriodAdjustment, "25000.00"), cr("accrued_liabilities", "25000.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_error", statement: "December hosting fees of 15,000.00 USD (FY2024) and 10,000.00 USD (FY2025) were not accrued at either year end: an error in previously issued financial statements, not a change in estimate.",
						conclusion: "Error identified and characterized under ASC 250-10-20.", refs: []string{"ASC 250-10-20", "ASC 250-10-45-22"}, assertions: []string{"completeness", "cutoff"},
						facts: []string{"s1_f_fy2024_omission", "s1_f_fy2025_omission"}, evidence: []string{"ev_hosting_invoices"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_quantified", statement: "Accrued liabilities are restated from 120,000.00 to 135,000.00 USD at 2024-12-31 and from 135,000.00 to 160,000.00 USD at 2025-12-31; the cumulative correction at 2026-01-01 is 25,000.00 USD.",
						conclusion: "Quantified by period; cumulative correction recomputes.", refs: []string{"ASC 250-10-45-23"}, assertions: []string{"accuracy", "valuationAllocation"},
						facts: []string{"s2_f_accrued_fy2024", "s2_f_accrued_fy2025"}, calcs: []string{"s2_error_correction"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_materiality", statement: "Under SAB 108 the misstatement is 3.75% of FY2025 pretax income by rollover and 6.25% by iron curtain; with the SAB 99 qualitative factors considered it is material.",
						conclusion: "Material; both methods quantified.", refs: []string{"SAB Topic 1.M (SAB 99)", "SAB Topic 1.N (SAB 108)"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s3_f_pretax_income_fy2025"}, calcs: []string{"s2_error_correction"}, judgments: []string{"s3_j_materiality"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_restatement", statement: "The error is corrected by restatement: a 25,000.00 USD prior-period adjustment to opening retained earnings against accrued liabilities as of 2026-01-01, with the FY2024 and FY2025 statements restated.",
						conclusion: "Restatement through a prior-period adjustment.", refs: []string{"ASC 250-10-45-23", "ASC 250-10-45-24"}, assertions: []string{"accuracy", "presentationDisclosure"},
						calcs: []string{"s2_error_correction", "s4_accounting_schedule"}, judgments: []string{"s4_j_method"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_disclosure", statement: "The restatement discloses that previously issued statements were restated, the nature of the error, and the effect on each line item, income and retained earnings for each period presented.",
						conclusion: "Disclosure per ASC 250-10-50-7 to 50-9.", refs: []string{"ASC 250-10-50-7", "ASC 250-10-50-8", "ASC 250-10-50-9"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s2_error_correction"}, judgments: []string{"s4_j_method"}})
				},
			)
		},
	})
}
