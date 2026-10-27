//go:build ignore

package main

import (
	"fmt"
	"math"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// ASC 842 example: a 36-month office lease, 10,000 USD a month in arrears,
// discounted at a 6% incremental borrowing rate, classified as operating.
// Every period's figures are rounded to cents as a preparer would book
// them; the final period retires the liability exactly.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "asc842-lease", extension: vwp.ExtensionASC842, domain: "accounting.leases",
		title:   "ASC 842 Lease Accounting - DemoCo Office Lease OFF-001",
		summary: "DemoCo's 36-month office lease is an operating lease: none of the ASC 842-10-25-2 criteria is met. The 36 payments of 10,000 USD in arrears discount at the 6% incremental borrowing rate to a lease liability of 328,710.16 USD at commencement, with an equal right-of-use asset; the single straight-line lease cost is 10,000 USD a month and the liability retires with the final payment.",
		refs:    []string{"ASC 842-10-25-2", "ASC 842-20-30-1", "ASC 842-20-25-6"},
		period:  []string{"2026-01-01", "2028-12-31"},
		build: func(b *builder.Builder) error {
			const (
				n        = 36
				payment  = 10000.0
				rate     = 0.06
				r        = rate / 12
				leaseId  = "OFF-001"
				currency = "USD"
			)
			// Present value of the annuity in arrears.
			pv := 0.0
			for i := 1; i <= n; i++ {
				pv += payment / math.Pow(1+r, float64(i))
			}
			liability := cents(pv)
			rou := liability

			type row struct {
				end                                                     string
				opening, pay, interest, principal, closing, cost, amort float64
			}
			var rows []row
			opening := liability
			for i := 0; i < n; i++ {
				first := time.Date(2026, time.Month(1+i), 1, 0, 0, 0, 0, time.UTC)
				end := first.AddDate(0, 1, -1).Format("2006-01-02")
				interest := cents(opening * r)
				principal := cents(payment - interest)
				if i == n-1 {
					principal = opening
					interest = cents(payment - principal)
				}
				closing := cents(opening - principal)
				amort := cents(payment - interest)
				rows = append(rows, row{end, opening, payment, interest, principal, closing, payment, amort})
				opening = closing
			}

			var payments, periods []any
			for _, rw := range rows {
				payments = append(payments, m("date", rw.end, "amount", rw.pay))
				periods = append(periods, m("periodEnd", rw.end, "openingLiability", rw.opening, "payment", rw.pay, "interest", rw.interest,
					"principal", rw.principal, "closingLiability", rw.closing, "leaseCost", rw.cost, "rouAmortization", rw.amort))
			}

			entries := []accountingschedule.Entry{
				entry(leaseId+"-commencement", "2026-01-01", "Lease OFF-001 commencement: right-of-use asset and lease liability", leaseId,
					dr(validators.RoleRouAsset, amt(rou)), cr(validators.RoleLeaseLiability, amt(liability))),
			}
			total := rou
			for _, rw := range rows {
				entries = append(entries, entry(leaseId+"-"+rw.end, rw.end, "Lease OFF-001 period ended "+rw.end+": straight-line lease cost, payment and liability reduction", leaseId,
					dr(validators.RoleLeaseExpense, amt(rw.cost)), dr(validators.RoleLeaseLiability, amt(rw.principal)),
					cr(validators.RoleLeasePayable, amt(rw.pay)), cr(validators.RoleRouAsset, amt(rw.amort))))
				total += rw.cost + rw.principal
			}

			return all(
				func() error {
					return ev(b, "ev_lease_agreement", "contractDocument", "Office lease agreement excerpt", "evidence/lease-agreement-excerpt.txt",
						"Office Lease Agreement (excerpt)\n\n1. Premises: Suite 400, 1 Demo Plaza (approximately 8,000 square feet of office space).\n2. Term: 36 months commencing 2026-01-01 and ending 2028-12-31. No renewal, termination or purchase option.\n3. Rent: 10,000.00 USD per month, payable in arrears on the last day of each month.\n4. Title: the premises remain the lessor's property at all times.\n5. The premises are general-purpose office space with an estimated remaining economic life of 20 years; the lessor's appraisal values the building interest leased at 2,000,000.00 USD.\n")
				},
				func() error {
					return ev(b, "ev_ibr_memo", "analysisMemo", "Incremental borrowing rate memo", "evidence/ibr-memo.txt",
						"Incremental borrowing rate as of 2026-01-01\n\nThe rate implicit in the lease is not readily determinable. DemoCo's collateralized 3-year borrowing rate per its bank term sheet dated 2025-12-15 is 6.00% per annum, applied as the incremental borrowing rate for leases of similar term (ASC 842-20-30-3).\n")
				},
				func() error {
					return fact(b, "s1_f_term", "The lease runs 36 months from 2026-01-01 to 2028-12-31 with no renewal, termination or purchase option.", "term", m("months", 36, "start", "2026-01-01", "end", "2028-12-31"), "ev_lease_agreement", "2. Term: 36 months commencing 2026-01-01 and ending 2028-12-31. No renewal, termination or purchase option.")
				},
				func() error {
					return fact(b, "s1_f_identified_asset", "The lease conveys the right to use an identified office suite for the term.", "term", "Suite 400, 1 Demo Plaza", "ev_lease_agreement", "1. Premises: Suite 400, 1 Demo Plaza")
				},
				func() error {
					return fact(b, "s2_f_economic_life", "The premises have an estimated remaining economic life of 20 years.", "quantity", m("years", 20), "ev_lease_agreement", "estimated remaining economic life of 20 years")
				},
				func() error {
					return fact(b, "s2_f_fair_value", "The leased building interest is appraised at 2,000,000.00 USD.", "amount", 2000000, "ev_lease_agreement", "values the building interest leased at 2,000,000.00 USD")
				},
				func() error {
					return fact(b, "s3_f_payments", "Rent is 10,000.00 USD per month for 36 months, payable in arrears.", "amount", m("amount", 10000, "count", 36, "timing", "arrears"), "ev_lease_agreement", "3. Rent: 10,000.00 USD per month, payable in arrears on the last day of each month.")
				},
				func() error {
					return fact(b, "s3_f_rate", "The incremental borrowing rate applied is 6.00% per annum.", "rate", m("rate", 0.06, "date", "2026-01-01"), "ev_ibr_memo", "6.00% per annum, applied as the incremental borrowing rate")
				},
				func() error {
					return judgment(b, "s2_j_classification", "Lease classification",
						"Is lease OFF-001 a finance lease or an operating lease under ASC 842-10-25-2?",
						"Operating lease. None of the five criteria in ASC 842-10-25-2 is met: title does not transfer, there is no purchase option, the 3-year term (s1_f_term) is 15% of the 20-year remaining economic life (s2_f_economic_life) and not a major part, the present value of the payments (328,710.16 USD) is 16% of the 2,000,000.00 USD fair value (s2_f_fair_value) and not substantially all, and general-purpose office space is not specialized.",
						"ASC 842-10-25-2(a) to (e) and 842-10-25-3; s1_f_term; s2_f_economic_life; s2_f_fair_value; the 75% and 90% thresholds of 842-10-55-2 used as reasonable approaches to 'major part' and 'substantially all'.",
						vwp.JudgmentAlternative{Conclusion: "Finance lease", Notes: "Rejected: no criterion is met; the term and present value are far below the thresholds."}, "medium")
				},
				func() error {
					return judgment(b, "s3_j_discount_rate", "Discount rate",
						"Which discount rate measures the lease liability?",
						"The 6.00% incremental borrowing rate (s3_f_rate): the rate implicit in the lease is not readily determinable, and the collateralized 3-year borrowing rate for a similar amount and term is the ASC 842-20-30-3 rate.",
						"ASC 842-20-30-2 and 30-3; s3_f_rate; the bank term sheet behind the memo.",
						vwp.JudgmentAlternative{Conclusion: "Risk-free rate practical expedient", Notes: "Not elected: DemoCo is not a private company electing the 842-20-30-3 expedient by class of asset."}, "medium")
				},
				func() error {
					return calc(b, "s4_lease_schedule", "leaseSchedule",
						"initialLiability = present value of the 36 payments in arrears at 0.06/12 per month (ASC 842-20-30-1); initialRouAsset = liability (no initial direct costs, prepayments or incentives, 842-20-30-5). Per period: interest = openingLiability × 0.005, principal = payment − interest, closingLiability = openingLiability − principal; the final period's principal is the remaining balance and its interest the payment less that balance. Operating lease: leaseCost = total payments / 36 (842-20-25-6); rouAmortization = leaseCost − interest (842-20-35-3). Figures rounded to cents each period.",
						validators.ValidatorASC842LeaseRollForward,
						m("leaseId", leaseId, "currency", currency, "classification", "operating", "classificationJudgmentRef", "s2_j_classification",
							"commencementDate", "2026-01-01", "discountRate", rate, "rateFact", "s3_f_rate", "paymentTiming", "arrears", "periodsPerYear", 12,
							"payments", payments, "initialDirectCosts", 0, "leaseIncentives", 0, "prepaidLeasePayments", 0),
						m("initialLiability", liability, "initialRouAsset", rou, "straightLineCost", payment, "periods", periods))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorASC842AccountingScheduleTieOut, []string{"s4_lease_schedule"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "Lease OFF-001 commencement and monthly lease cost, 2026-2028", Kind: accountingschedule.KindLeaseCost, Currency: currency, AmountScale: 2,
						StartDate: "2026-01-01", EndDate: "2028-12-31", TotalAmount: amt(total), Entries: entries,
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_lease", statement: "The agreement is a lease: it conveys the right to control the use of an identified office suite for 36 months in exchange for consideration.",
						conclusion: "A lease under ASC 842-10-15-3 exists.", refs: []string{"ASC 842-10-15-3", "ASC 842-10-15-9"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s1_f_term", "s1_f_identified_asset"}, evidence: []string{"ev_lease_agreement"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_classification", statement: "Lease OFF-001 is an operating lease: none of the ASC 842-10-25-2 criteria is met.",
						conclusion: "Operating lease.", refs: []string{"ASC 842-10-25-2", "ASC 842-10-25-3"}, assertions: []string{"presentationDisclosure"},
						facts: []string{"s1_f_term", "s2_f_economic_life", "s2_f_fair_value"}, judgments: []string{"s2_j_classification"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_measurement_inputs", statement: "The lease payments are 36 monthly payments of 10,000.00 USD in arrears and the discount rate is the 6.00% incremental borrowing rate.",
						conclusion: "Measurement inputs established.", refs: []string{"ASC 842-10-30-5", "ASC 842-20-30-3"}, assertions: []string{"accuracy", "valuationAllocation"},
						facts: []string{"s3_f_payments", "s3_f_rate"}, judgments: []string{"s3_j_discount_rate"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_schedules", statement: fmt.Sprintf("The lease liability at commencement is %s USD with an equal right-of-use asset; the liability roll-forward and the straight-line lease cost of 10,000.00 USD a month recompute period by period and the executable schedule books them.", amt(liability)),
						conclusion: "Initial measurement and the period schedule recompute.", refs: []string{"ASC 842-20-30-1", "ASC 842-20-30-5", "ASC 842-20-25-6", "ASC 842-20-35-3"}, assertions: []string{"valuationAllocation", "completeness"},
						facts: []string{"s3_f_payments", "s3_f_rate"}, calcs: []string{"s4_lease_schedule", "s4_accounting_schedule"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_presentation", statement: "The operating lease right-of-use asset and lease liability are presented separately from finance leases, lease cost is a single operating line, and the maturity analysis and weighted-average term and rate are disclosed.",
						conclusion: "Presentation and disclosure per ASC 842-20-45 and 50.", refs: []string{"ASC 842-20-45-1", "ASC 842-20-45-4", "ASC 842-20-50-4", "ASC 842-20-50-6"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s4_lease_schedule"}})
				},
			)
		},
	})
}

func cents(f float64) float64 { return math.Round(f*100) / 100 }

func amt(f float64) string { return fmt.Sprintf("%.2f", f) }
