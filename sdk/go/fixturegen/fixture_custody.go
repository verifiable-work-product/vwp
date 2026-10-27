//go:build ignore

package main

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// custody-safeguarding: a custodian's custodied crypto asset populations at
// a period end under SAB 122 — control evidenced per population, assets
// valued for disclosure, the ASC 450-20 loss contingency assessed per
// population, and the one probable and estimable loss (customer assets lost
// in a hot-wallet incident) accrued. Synthetic data.
func init() {
	extraFixtures = append(extraFixtures, fixture{
		dir: "custody-safeguarding", extension: vwp.ExtensionCustodySafeguarding, domain: "accounting.custody",
		title:   "Crypto Custody and Safeguarding - DemoCo Custodied Assets at 2026-06-30",
		summary: "DemoCo safeguards 100 BTC and 2,000 ETH for customers (6,500,000 and 4,500,000 USD) under an obligation to return them; following SAB 122 no safeguarding liability or asset is recorded for assets held, the arrangements and the controls are disclosed, and the 10 ETH of customer assets lost in the 2026-06-12 hot-wallet incident is a probable, estimable loss accrued at 22,500 USD.",
		refs:    []string{"SEC SAB 122", "ASC 450-20-25-2", "ASC 450-20-50-1"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_custody_terms", "contractDocument", "Custody services agreement excerpt", "evidence/custody-agreement-excerpt.txt",
						"DemoCo Custody Services Agreement (excerpt)\n\n2.1 DemoCo holds Customer Digital Assets in segregated omnibus wallets whose private keys are generated and stored in DemoCo-controlled hardware security modules.\n2.2 ETH balances may be held with a qualified sub-custodian under DemoCo's instruction; DemoCo remains obligated to the Customer for their return.\n3.1 DemoCo shall return Customer Digital Assets, or their value, on the Customer's instruction.\n7.2 DemoCo is liable to make Customers whole for Customer Digital Assets lost through a failure of DemoCo's controls.\n")
				},
				func() error {
					return ev(b, "ev_signed_message", "chainRecord", "Signed-message proof of control for the BTC omnibus wallet, 2026-06-30", "evidence/btc-omnibus-proof.txt",
						"Address: bc1-demo-omnibus\nMessage: DemoCo custody proof of control 2026-06-30\nSignature: verified 2026-06-30 by DemoCo internal audit\nBalance at 2026-06-30: 100.00000000 BTC\n")
				},
				func() error {
					return ev(b, "ev_subcustodian_statement", "custodianStatement", "Sub-custodian holdings statement as of 2026-06-30", "evidence/subcustodian-statement.txt",
						"Demo Sub-Custody LLC holdings statement as of 2026-06-30\nAccount: DEMOCO-CLIENT-ETH\nETH balance: 2,000.00000000\nHeld for: DemoCo custody customers, segregated\n")
				},
				func() error {
					return ev(b, "ev_incident_report", "incidentReport", "Hot-wallet incident report, 2026-06-12", "evidence/incident-report.txt",
						"Incident 2026-06-12: unauthorized withdrawal from the ETH hot wallet\n\n10.00000000 ETH of customer assets were transferred out after a signing-service credential was compromised.\nRecovery attempts have failed; the funds are considered unrecoverable.\nUnder clause 7.2 DemoCo will make the affected customers whole in ETH or its value.\n")
				},
				func() error {
					return ev(b, "ev_period_end_prices", "pricingRecord", "Period-end prices from the principal market, 2026-06-30", "evidence/period-end-prices.csv",
						"asset,price_usd,source,asof\nBTC,65000.00,demo-exchange,2026-06-30\nETH,2250.00,demo-exchange,2026-06-30\n")
				},
				func() error {
					return fact(b, "s1_f_keys", "DemoCo generates and stores the private keys for customer omnibus wallets in its own HSMs.", "term", "entity-controlled keys", "ev_custody_terms", "private keys are generated and stored in DemoCo-controlled hardware security modules.")
				},
				func() error {
					return fact(b, "s1_f_subcustody", "ETH may be held with a qualified sub-custodian under DemoCo's instruction; DemoCo remains obligated for its return.", "term", "sub-custody with retained obligation", "ev_custody_terms", "DemoCo remains obligated to the Customer for their return.")
				},
				func() error {
					return fact(b, "s2_f_return_obligation", "DemoCo must return customer digital assets, or their value, on instruction.", "term", "obligation to return", "ev_custody_terms", "3.1 DemoCo shall return Customer Digital Assets, or their value, on the Customer's instruction.")
				},
				func() error {
					return fact(b, "s2_f_make_whole", "DemoCo is liable to make customers whole for assets lost through a failure of its controls.", "term", "make-whole liability", "ev_custody_terms", "7.2 DemoCo is liable to make Customers whole for Customer Digital Assets lost through a failure of DemoCo's controls.")
				},
				func() error {
					return fact(b, "s1_f_btc_balance", "The BTC omnibus wallet held 100 BTC at 2026-06-30 under a verified signed-message proof of control.", "quantity", m("units", 100, "asOf", "2026-06-30"), "ev_signed_message", "Balance at 2026-06-30: 100.00000000 BTC")
				},
				func() error {
					return fact(b, "s1_f_eth_balance", "The sub-custodian held 2,000 ETH for DemoCo customers at 2026-06-30.", "quantity", m("units", 2000, "asOf", "2026-06-30"), "ev_subcustodian_statement", "ETH balance: 2,000.00000000")
				},
				func() error {
					return fact(b, "s4_f_incident_units", "10 ETH of customer assets were withdrawn without authorization on 2026-06-12 and are unrecoverable.", "quantity", m("units", 10, "date", "2026-06-12"), "ev_incident_report", "10.00000000 ETH of customer assets were transferred out")
				},
				func() error {
					return fact(b, "s3_f_btc_price", "BTC closed at 65,000.00 USD on 2026-06-30.", "price", m("price", 65000, "date", "2026-06-30", "currency", "USD"), "ev_period_end_prices", "BTC,65000.00,demo-exchange,2026-06-30")
				},
				func() error {
					return fact(b, "s3_f_eth_price", "ETH closed at 2,250.00 USD on 2026-06-30.", "price", m("price", 2250, "date", "2026-06-30", "currency", "USD"), "ev_period_end_prices", "ETH,2250.00,demo-exchange,2026-06-30")
				},
				func() error {
					return judgment(b, "s2_j_safeguarding_obligation", "Safeguarding obligation and SAB 122 treatment of assets held",
						"Does DemoCo have an obligation to safeguard the customer BTC and ETH, and does holding them give rise to a liability or an asset on DemoCo's balance sheet?",
						"Yes, DemoCo is obligated to return the assets or their value (clause 3.1) and remains obligated for the ETH placed with the sub-custodian (clause 2.2), so it safeguards both populations. Under SAB 122, which rescinded SAB 121's requirement to recognize a safeguarding liability and corresponding asset, holding the assets is not itself a liability: DemoCo applies ASC 450-20 to the risk of loss from safeguarding and discloses the nature and amount of the assets safeguarded, the controls and the risks. The customer assets are not DemoCo's assets and are not recognized.",
						"SEC Staff Accounting Bulletin No. 122 (rescinding SAB 121, effective for periods beginning after 2024-12-15); ASC 450-20-25-2 (recognition only when a loss is probable and estimable); custody agreement clauses 2.1, 2.2, 3.1; proof of control and sub-custodian statement.",
						vwp.JudgmentAlternative{Conclusion: "Recognize a safeguarding liability and asset at fair value for all custodied assets", Notes: "Rejected: that was the SAB 121 model, rescinded by SAB 122; an entity that has not yet adopted SAB 122 would still apply it — adoption is a fact to confirm."}, "high")
				},
				func() error {
					return judgment(b, "s4_j_incident_loss", "Loss contingency for the hot-wallet incident",
						"Is a loss from the 2026-06-12 incident probable and reasonably estimable, and at what amount is it accrued?",
						"Yes. Recovery has failed and clause 7.2 obliges DemoCo to make the affected customers whole, so a loss is probable; it is estimable at the 10 ETH lost measured at the 2026-06-30 price of 2,250 USD, 22,500 USD, with no insurance recovery asserted. The accrual is recorded as a safeguarding loss against a liability to the customers, remeasured each period until settled in ETH or its value.",
						"ASC 450-20-25-2 (probable and estimable); ASC 450-20-30-1 (best estimate within the range — the ETH lost is known, only its price moves); ASC 450-20-50-1; incident report; clause 7.2.",
						vwp.JudgmentAlternative{Conclusion: "Disclose only, pending recovery efforts", Notes: "Rejected: recovery attempts have failed and the make-whole obligation is contractual; disclosure alone would understate liabilities."}, "high")
				},
				func() error {
					return judgment(b, "s4_j_no_loss_held", "Loss contingency for the assets held in custody",
						"Is a loss from safeguarding the 100 BTC and 2,000 ETH still held probable or reasonably possible at 2026-06-30?",
						"No loss is probable: the BTC keys are in DemoCo HSMs with a verified proof of control and the ETH sits segregated with the sub-custodian; no incident affects these populations. A loss is at most reasonably possible given the inherent risks of key custody, so no accrual is recorded and the risk is disclosed with the safeguarded amounts.",
						"ASC 450-20-25-2 and 450-20-50-3 to 50-5 (disclosure of reasonably possible losses); SAB 122 disclosure expectations; proof of control; sub-custodian statement.",
						vwp.JudgmentAlternative{Conclusion: "Accrue an expected-loss provision for custody risk", Notes: "Rejected: ASC 450-20 does not permit accruing a general reserve for possible future losses (450-20-25-8 by analogy to general reserves)."}, "medium")
				},
				func() error {
					return register(b, validators.RegisterCustodiedAssets, m(
						"description", "Custodied crypto asset populations at 2026-06-30 with the SAB 122 / ASC 450-20 assessment per population. Synthetic data.",
						"asOf", "2026-06-30", "currency", "USD", "roundingTolerance", 0.01,
						"holdings", []any{
							m("id", "h_btc_omnibus", "asset", "BTC", "wallet", "bc1-demo-omnibus", "units", 100, "unitPrice", 65000, "priceFact", "s3_f_btc_price", "fairValue", 6500000,
								"holderRole", "custodian", "customerCount", 240, "obligationToReturn", true, "controlEvidenceRef", "ev_signed_message",
								"lossProbable", false, "lossEstimable", false, "recognizedLiability", 0, "judgmentRef", "s4_j_no_loss_held"),
							m("id", "h_eth_subcustody", "asset", "ETH", "wallet", "DEMOCO-CLIENT-ETH", "units", 2000, "unitPrice", 2250, "priceFact", "s3_f_eth_price", "fairValue", 4500000,
								"holderRole", "subCustodian", "subCustodian", "Demo Sub-Custody LLC", "customerCount", 180, "obligationToReturn", true, "controlEvidenceRef", "ev_subcustodian_statement",
								"lossProbable", false, "lossEstimable", false, "recognizedLiability", 0, "judgmentRef", "s4_j_no_loss_held"),
							m("id", "h_eth_incident", "asset", "ETH", "wallet", "eth-hot-wallet", "units", 10, "unitPrice", 2250, "priceFact", "s3_f_eth_price", "fairValue", 22500,
								"holderRole", "custodian", "customerCount", 6, "obligationToReturn", true, "controlEvidenceRef", "s4_f_incident_units",
								"lossProbable", true, "lossEstimable", true, "recognizedLiability", 22500, "judgmentRef", "s4_j_incident_loss",
								"notes", "Customer ETH lost in the 2026-06-12 hot-wallet incident; make-whole obligation under clause 7.2."),
						}))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorCustodyAccountingScheduleTieOut, []string{"custodied-assets register"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "Safeguarding loss accrual at 2026-06-30", Kind: accountingschedule.KindSafeguardingLiability, Currency: "USD", AmountScale: 2,
						StartDate: "2026-06-30", EndDate: "2026-06-30", TotalAmount: "22500.00",
						Entries: []accountingschedule.Entry{
							entry("h_eth_incident-2026-06-30", "2026-06-30", "Accrual for 10 ETH of customer assets lost in the 2026-06-12 incident, at 2,250.00", "h_eth_incident", dr(validators.RoleSafeguardingLoss, "22500.00"), cr(validators.RoleSafeguardingLiability, "22500.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_population", statement: "DemoCo safeguards 100 BTC in an HSM-keyed omnibus wallet it controls and 2,000 ETH placed with a sub-custodian under its instruction, for 420 customers.",
						conclusion: "Custodied populations and key control established.", refs: []string{"SEC SAB 122"}, assertions: []string{"existence", "completeness"},
						facts: []string{"s1_f_keys", "s1_f_subcustody", "s1_f_btc_balance", "s1_f_eth_balance"}, evidence: []string{"ev_signed_message", "ev_subcustodian_statement"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_obligation", statement: "DemoCo has a safeguarding obligation for both populations; under SAB 122 holding them creates no liability or asset, and ASC 450-20 governs the risk of loss.",
						conclusion: "Safeguarding obligation; SAB 122 model; no gross-up.", refs: []string{"SEC SAB 122", "ASC 450-20-25-2"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s2_f_return_obligation", "s2_f_make_whole"}, judgments: []string{"s2_j_safeguarding_obligation"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_valuation", statement: "Custodied assets are valued for disclosure at 6,500,000 USD (BTC) and 4,500,000 USD (ETH) at 2026-06-30 prices, with the 10 ETH lost valued at 22,500 USD.",
						conclusion: "Valuation recomputes from dated price facts.", refs: []string{"SEC SAB 122", "ASC 820-10-35-1"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s3_f_btc_price", "s3_f_eth_price", "s1_f_btc_balance", "s1_f_eth_balance"}, evidence: []string{"ev_period_end_prices"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_contingency", statement: "A 22,500 USD loss for the 10 ETH lost on 2026-06-12 is probable and estimable and is accrued; no loss is probable for the assets still held, whose risk is reasonably possible and disclosed.",
						conclusion: "One accrual; disclosure for the rest.", refs: []string{"ASC 450-20-25-2", "ASC 450-20-30-1", "ASC 450-20-50-3"}, assertions: []string{"completeness", "valuationAllocation"},
						facts: []string{"s4_f_incident_units", "s3_f_eth_price"}, evidence: []string{"ev_incident_report"}, judgments: []string{"s4_j_incident_loss", "s4_j_no_loss_held"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_disclosure", statement: "The nature and amount of custodied assets, the custody and sub-custody arrangements, the controls over keys, the incident and the accrued loss are disclosed; customer assets are not presented on DemoCo's balance sheet.",
						conclusion: "SAB 122 and ASC 450-20-50 disclosures; off-balance-sheet customer assets.", refs: []string{"SEC SAB 122", "ASC 450-20-50-1", "ASC 450-20-50-3"}, assertions: []string{"presentationDisclosure"},
						judgments: []string{"s2_j_safeguarding_obligation"}})
				},
			)
		},
	})
}
