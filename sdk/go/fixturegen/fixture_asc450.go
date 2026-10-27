//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// asc450-contingencies: three loss contingencies assessed at a period end
// (one probable and estimable with a range and no best estimate, one
// reasonably possible, one remote guarantee), the going concern
// evaluation, and the accrual schedule booking the one accrued item at
// the low end of its range (ASC 450-20-30-1). Synthetic data.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "asc450-contingencies", extension: vwp.ExtensionASC450, domain: "accounting.contingencies",
		title:   "ASC 450 Loss Contingencies and Going Concern - DemoCo at 2026-06-30",
		summary: "Of three loss contingencies at 2026-06-30, the supplier litigation is probable and estimable in a range of 400,000 to 900,000 USD with no better estimate within it, so 400,000 USD is accrued; the regulatory inquiry is reasonably possible (disclosed, not accrued); the guarantee of an affiliate's facility is remote (disclosed). Management's evaluation finds no substantial doubt about DemoCo's ability to continue as a going concern within one year of issuance.",
		refs:    []string{"ASC 450-20-25-2", "ASC 450-20-30-1", "ASC 450-20-50-3", "ASC 205-40-50-1"},
		period:  []string{"2026-04-01", "2026-06-30"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_legal_letter", "legalLetter", "Outside counsel's litigation letter as of 2026-06-30", "evidence/legal-letter-2026-06-30.txt",
						"Counsel letter to DemoCo, matters as of 2026-06-30\n\nMatter 1 - Supplier v. DemoCo (breach of supply agreement). In our opinion an unfavorable outcome is probable. We estimate the loss, inclusive of costs, at between 400,000.00 and 900,000.00 USD; no amount within that range is a better estimate than any other at this stage.\n\nMatter 2 - Regulatory inquiry (data retention). An adverse finding is reasonably possible but not probable. A penalty, if any, would fall between 50,000.00 and 250,000.00 USD.\n")
				},
				func() error {
					return ev(b, "ev_guarantee", "contractDocument", "Guarantee of affiliate credit facility (excerpt)", "evidence/affiliate-guarantee-excerpt.txt",
						"Guarantee Agreement (excerpt)\n\n1. DemoCo guarantees the obligations of DemoCo Services Ltd under its 1,000,000.00 USD revolving facility.\n2. The facility is undrawn as of 2026-06-30 and the affiliate is current on all covenants.\n")
				},
				func() error {
					return ev(b, "ev_cash_forecast", "managementReport", "Twelve-month cash forecast approved 2026-07-10", "evidence/cash-forecast-2026-07.csv",
						"month,opening_cash_usd,net_cash_flow_usd,closing_cash_usd\n2026-07,4200000.00,-150000.00,4050000.00\n2026-12,3400000.00,-120000.00,3280000.00\n2027-06,2900000.00,-80000.00,2820000.00\nminimum closing cash over the twelve months: 2820000.00\n")
				},
				func() error {
					return fact(b, "s1_f_litigation_range", "Counsel assesses an unfavorable outcome in the supplier litigation as probable, with a loss of 400,000 to 900,000 USD and no better estimate within the range.", "estimate", m("low", 400000, "high", 900000, "currency", "USD"), "ev_legal_letter", "We estimate the loss, inclusive of costs, at between 400,000.00 and 900,000.00 USD; no amount within that range is a better estimate than any other at this stage.")
				},
				func() error {
					return fact(b, "s1_f_regulatory_range", "Counsel assesses an adverse regulatory finding as reasonably possible, with a penalty of 50,000 to 250,000 USD.", "estimate", m("low", 50000, "high", 250000, "currency", "USD"), "ev_legal_letter", "A penalty, if any, would fall between 50,000.00 and 250,000.00 USD.")
				},
				func() error {
					return fact(b, "s1_f_guarantee_terms", "DemoCo guarantees its affiliate's 1,000,000 USD revolving facility, undrawn at 2026-06-30 with covenants met.", "term", m("maximumExposure", 1000000, "drawn", 0, "currency", "USD"), "ev_guarantee", "The facility is undrawn as of 2026-06-30 and the affiliate is current on all covenants.")
				},
				func() error {
					return fact(b, "s4_f_minimum_cash", "The approved twelve-month cash forecast shows a minimum closing cash balance of 2,820,000 USD.", "amount", m("amount", 2820000, "currency", "USD", "date", "2027-06-30"), "ev_cash_forecast", "minimum closing cash over the twelve months: 2820000.00")
				},
				func() error {
					return judgment(b, "s2_j_litigation", "Likelihood and estimability of the supplier litigation",
						"Is a loss on the supplier litigation probable and reasonably estimable at 2026-06-30, and what amount is accrued?",
						"Probable and estimable. Counsel assesses an unfavorable outcome as probable and the loss as a range of 400,000 to 900,000 USD with no better estimate within it; ASC 450-20-30-1 accrues the minimum of the range, 400,000 USD, and the exposure above it is disclosed.",
						"ASC 450-20-25-2 (probable and reasonably estimable); ASC 450-20-30-1 (range with no best estimate: accrue the minimum); ASC 450-20-50-3 and 50-4 (disclose the exposure above the accrual); counsel letter matter 1.",
						vwp.JudgmentAlternative{Conclusion: "Accrue the midpoint of the range", Notes: "Rejected: ASC 450-20-30-1 accrues the minimum when no amount in the range is a better estimate; the midpoint is not supported by counsel."}, "high")
				},
				func() error {
					return judgment(b, "s2_j_regulatory", "Likelihood of the regulatory inquiry",
						"Is a loss on the regulatory inquiry probable at 2026-06-30?",
						"Reasonably possible, not probable: counsel does not assess an adverse finding as likely. No accrual; the nature of the inquiry and the 50,000 to 250,000 USD range are disclosed.",
						"ASC 450-20-25-2(a) not met; ASC 450-20-50-3 and 50-4 require disclosure of a reasonably possible loss and the range; counsel letter matter 2.",
						vwp.JudgmentAlternative{Conclusion: "Probable; accrue 50,000 USD", Notes: "Rejected: counsel's assessment is reasonably possible, and an accrual of a reasonably possible loss is not permitted."}, "medium")
				},
				func() error {
					return judgment(b, "s2_j_guarantee", "Likelihood of a loss under the affiliate guarantee",
						"Is a loss under the guarantee of the affiliate's facility probable or reasonably possible at 2026-06-30?",
						"Remote: the facility is undrawn and the affiliate is in compliance. Remote guarantee-type contingencies are nonetheless disclosed (ASC 450-20-50-6), with the maximum exposure of 1,000,000 USD; the guarantee's own recognition under ASC 460 is addressed separately.",
						"ASC 450-20-50-6 (guarantees of indebtedness of others disclosed even when remote); ASC 460-10 for the guarantee itself; guarantee agreement clauses 1 and 2.",
						vwp.JudgmentAlternative{Conclusion: "No disclosure because the loss is remote", Notes: "Rejected: ASC 450-20-50-6 carves guarantees out of the remote exception."}, "low")
				},
				func() error {
					return judgment(b, "s4_j_going_concern", "Going concern evaluation",
						"Do conditions or events, considered in the aggregate, raise substantial doubt about DemoCo's ability to continue as a going concern within one year after the date the financial statements are issued?",
						"No. The approved twelve-month cash forecast keeps closing cash above 2,820,000 USD throughout, the accrued litigation loss is covered several times over, and no debt matures in the period; no conditions raising substantial doubt were identified, so no mitigation plans need to be evaluated and no disclosure under ASC 205-40-50-12 is required.",
						"ASC 205-40-50-1 to 50-4 (evaluation period and conditions); ASC 205-40-50-12 and 50-13 (disclosure when substantial doubt is raised); twelve-month cash forecast; counsel letter.",
						vwp.JudgmentAlternative{Conclusion: "Substantial doubt raised, alleviated by management's plans", Notes: "Rejected: no condition raising substantial doubt was identified, so the plans analysis is not reached."}, "medium")
				},
				func() error {
					return register(b, validators.RegisterContingencies, m(
						"description", "Loss contingency register at 2026-06-30. Synthetic data.",
						"asOf", "2026-06-30", "currency", "USD", "roundingTolerance", 0.01,
						"items", []any{
							m("id", "c_supplier_litigation", "description", "Supplier v. DemoCo, breach of supply agreement", "type", "litigation",
								"likelihood", "probable", "estimable", true, "estimateLow", 400000, "estimateHigh", 900000, "estimateFact", "s1_f_litigation_range",
								"accrued", 400000, "disclosureRequired", true, "judgmentRef", "s2_j_litigation", "evidenceRef", "ev_legal_letter"),
							m("id", "c_regulatory_inquiry", "description", "Regulatory inquiry into data retention", "type", "regulatory",
								"likelihood", "reasonablyPossible", "estimable", true, "estimateLow", 50000, "estimateHigh", 250000, "estimateFact", "s1_f_regulatory_range",
								"accrued", 0, "disclosureRequired", true, "judgmentRef", "s2_j_regulatory", "evidenceRef", "ev_legal_letter"),
							m("id", "c_affiliate_guarantee", "description", "Guarantee of DemoCo Services Ltd revolving facility", "type", "guarantee",
								"likelihood", "remote", "estimable", true, "estimateLow", 0, "estimateHigh", 1000000,
								"accrued", 0, "disclosureRequired", true, "judgmentRef", "s2_j_guarantee", "evidenceRef", "ev_guarantee"),
						}))
				},
				func() error {
					return schedule(b, "s3_accounting_schedule", validators.ValidatorASC450AccountingScheduleTieOut, []string{"facts/contingencies.json"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "Loss contingency accrual at 2026-06-30", Kind: accountingschedule.KindContingencyAccrual, Currency: "USD", AmountScale: 2,
						StartDate: "2026-06-30", EndDate: "2026-06-30", TotalAmount: "400000.00",
						Entries: []accountingschedule.Entry{
							entry("c_supplier_litigation-2026-06-30", "2026-06-30", "Accrual of the probable supplier litigation loss at the low end of the 400,000 to 900,000 USD range", "c_supplier_litigation",
								dr(validators.RoleContingencyLoss, "400000.00"), cr(validators.RoleAccruedContingencyLiability, "400000.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_population", statement: "Three loss contingencies exist at 2026-06-30: the supplier litigation, the regulatory inquiry and the guarantee of the affiliate's facility, each supported by counsel's letter or the guarantee agreement.",
						conclusion: "Population complete from counsel's letter and the contracts register.", refs: []string{"ASC 450-20-25-1", "ASC 450-20-20"}, assertions: []string{"completeness", "existence"},
						facts: []string{"s1_f_litigation_range", "s1_f_regulatory_range", "s1_f_guarantee_terms"}, evidence: []string{"ev_legal_letter", "ev_guarantee"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_likelihood", statement: "The litigation loss is probable and estimable; the regulatory loss is reasonably possible; the guarantee loss is remote.",
						conclusion: "Likelihood concluded per item from counsel's assessment.", refs: []string{"ASC 450-20-25-2", "ASC 450-20-20"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s1_f_litigation_range", "s1_f_regulatory_range", "s1_f_guarantee_terms"}, judgments: []string{"s2_j_litigation", "s2_j_regulatory", "s2_j_guarantee"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_measurement", statement: "400,000 USD is accrued for the supplier litigation, the minimum of counsel's range because no amount within it is a better estimate; nothing is accrued for the other two items.",
						conclusion: "Accrual at the low end of the range; the register and the accrual schedule carry it.", refs: []string{"ASC 450-20-30-1", "ASC 450-20-25-2"}, assertions: []string{"valuationAllocation", "completeness"},
						facts: []string{"s1_f_litigation_range"}, calcs: []string{"s3_accounting_schedule"}, judgments: []string{"s2_j_litigation"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_going_concern", statement: "Conditions and events, in the aggregate, do not raise substantial doubt about DemoCo's ability to continue as a going concern within one year after the issuance date: forecast cash stays above 2,820,000 USD and no debt matures.",
						conclusion: "No substantial doubt; no ASC 205-40-50-12 disclosure.", refs: []string{"ASC 205-40-50-1", "ASC 205-40-50-4"}, assertions: []string{"presentationDisclosure"},
						facts: []string{"s4_f_minimum_cash"}, evidence: []string{"ev_cash_forecast"}, judgments: []string{"s4_j_going_concern"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_disclosure", statement: "The nature of each contingency, the 400,000 USD accrued, the exposure to loss above the accrual up to 900,000 USD, the 50,000 to 250,000 USD reasonably possible regulatory penalty and the 1,000,000 USD guarantee are disclosed.",
						conclusion: "Disclosure per ASC 450-20-50-1 to 50-6.", refs: []string{"ASC 450-20-50-1", "ASC 450-20-50-3", "ASC 450-20-50-4", "ASC 450-20-50-6"}, assertions: []string{"presentationDisclosure"},
						judgments: []string{"s2_j_litigation", "s2_j_regulatory", "s2_j_guarantee"}})
				},
			)
		},
	})
}
