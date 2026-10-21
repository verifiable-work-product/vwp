//go:build ignore

// Command fixturegen regenerates the accounting extension example packages
// (ASC 350-60, ASC 350-30, staking rewards, lending, cost basis, stablecoin)
// with the builder, so every part hash in each manifest is consistent and
// every package verifies clean against the registry. Each example is written
// as <root>/<name>/package plus a packed <root>/<name>/<name>.vwp:
//
//	go run fixturegen/main.go [root]
//
// The hand-authored examples (asc606-revenue-contract, asc820-fair-value,
// digital-assets-wallet-ownership) and the testdata/ trees are NOT touched
// here. All content below is synthetic demo data.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

const createdAt = "2026-10-14T12:00:00Z"

// defaultRoot is the examples tree the fixtures are written into.
const defaultRoot = "../../examples"

type fixture struct {
	dir       string
	extension string
	domain    string
	title     string
	summary   string
	refs      []string
	period    []string
	build     func(b *builder.Builder) error
}

func main() {
	root := defaultRoot
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	for _, f := range fixtures {
		dir := filepath.Join(root, f.dir, "package")
		if err := os.RemoveAll(dir); err != nil {
			fail(f.dir, err)
		}
		b, err := builder.Scaffold(dir, builder.ScaffoldConfig{
			PackageId: "vwp_demo_" + strings.ReplaceAll(f.dir, "-", "_"),
			CreatedAt: createdAt,
			Profile:   vwp.ProfileFinance,
			Extension: f.extension,
			Title:     f.title,
			Subject:   map[string]any{"type": "demo", "entity": "DemoCo"},
			Workproduct: vwp.Workproduct{
				Id: "wp_demo_" + strings.ReplaceAll(f.dir, "-", "_"), Title: f.title, Domain: f.domain,
				StandardRefs: f.refs, Entity: map[string]any{"id": "entity_demo", "name": "DemoCo"},
				Period:            period(f),
				ConclusionSummary: f.summary, Status: "inReview", ReviewState: "inReview",
				CreatedBy: map[string]any{"type": "agent", "name": "vwp-demo-agent", "version": "0.1.0"},
			},
			Provenance: vwp.Provenance{
				CreatedAt: createdAt,
				CreatedBy: vwp.ProvenanceAgent{Type: "agent", Name: "vwp-demo-agent", Version: "0.1.0"},
				Toolchain: []vwp.ToolchainEntry{{Name: "vwp-go/fixturegen", Version: "0.1.0"}},
				Models:    []vwp.ModelRef{{Provider: "demo", Name: "manual-fixture", Version: "n/a"}},
				Notes:     "Synthetic demo package for the " + f.extension + " extension. All entities, assets, prices and parties are fictional.",
			},
		})
		if err != nil {
			fail(f.dir, err)
		}
		if err := f.build(b); err != nil {
			fail(f.dir, err)
		}
		report, err := b.RunValidators(validators.Registry())
		if err != nil {
			fail(f.dir, err)
		}
		if err := b.Render(); err != nil {
			fail(f.dir, err)
		}
		if !report.Valid {
			for _, r := range report.Results {
				if r.Status == vwp.StatusFail {
					fmt.Fprintf(os.Stderr, "  FAIL %s: %s\n", r.ValidatorId, r.Message)
				}
			}
			fail(f.dir, fmt.Errorf("fixture does not verify"))
		}
		if err := vwp.Pack(dir, filepath.Join(root, f.dir, f.dir+".vwp")); err != nil {
			fail(f.dir, err)
		}
		fmt.Printf("%s: %d validations, valid\n", f.dir, len(report.Results))
	}
}

func period(f fixture) map[string]any {
	if len(f.period) == 2 {
		return map[string]any{"start": f.period[0], "end": f.period[1]}
	}
	return map[string]any{"start": "2026-04-01", "end": "2026-06-30"}
}

func fail(dir string, err error) {
	fmt.Fprintf(os.Stderr, "fixturegen: %s: %v\n", dir, err)
	os.Exit(1)
}

// --- helpers ---

func ev(b *builder.Builder, id, typ, title, path, content string) error {
	return b.AddEvidence(vwp.Evidence{Id: id, Type: typ, Title: title + " (synthetic)", SourceSystem: "demo", Path: path, RetrievedAt: createdAt}, []byte(content))
}

func fact(b *builder.Builder, id, statement, factType string, value any, evidenceId, quote string) error {
	conf := 0.98
	review := false
	return b.AddFact(vwp.Fact{
		Id: id, Statement: statement, FactType: factType, Value: value,
		EvidenceRefs:     []vwp.EvidenceRef{{EvidenceId: evidenceId, Anchors: []vwp.Anchor{{Quote: quote}}}},
		ExtractionMethod: "manual", Confidence: &conf, RequiresReview: &review,
	})
}

func judgment(b *builder.Builder, id, topic, question, conclusion, basis string, alt vwp.JudgmentAlternative, risk string) error {
	return b.AddJudgment(vwp.Judgment{
		Id: id, Topic: topic, Question: question, ProposedConclusion: conclusion, Basis: basis,
		Alternatives: []vwp.JudgmentAlternative{alt}, RiskLevel: risk, RequiredApproverRole: "technical-accounting-reviewer", Status: "reviewed",
	})
}

func calc(b *builder.Builder, id, typ, method, validator string, inputs, outputs map[string]any) error {
	t := true
	return b.AddCalculation(vwp.Calculation{Id: id, Type: typ, Inputs: inputs, Outputs: outputs, Method: method, Validator: validator, Recomputable: &t})
}

func register(b *builder.Builder, name string, doc map[string]any) error {
	return b.AddRegister(name, doc)
}

// schedule writes the executable accounting schedule the way ai-svc's
// typed writer does: the authoring calculation (type accountingSchedule,
// outputs = the artifact) at its fixed path and the artifact itself at
// accountingschedule.ArtifactPath.
func schedule(b *builder.Builder, id, validator string, derivedFrom []string, a accountingschedule.Artifact) error {
	body, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	if _, err := accountingschedule.Parse(body); err != nil {
		return fmt.Errorf("fixture schedule %s: %w", id, err)
	}
	var outputs map[string]any
	if err := json.Unmarshal(body, &outputs); err != nil {
		return err
	}
	t := true
	c := vwp.Calculation{Id: id, Type: vwp.CalcTypeAccountingSchedule, Method: "Executable entries derived from " + strings.Join(derivedFrom, ", ") + "; amounts at amountScale, balanced semantic account-role postings.",
		Inputs: m("derivedFrom", derivedFrom, "currency", a.Currency, "amountScale", a.AmountScale), Outputs: outputs, Validator: validator, Recomputable: &t}
	if err := b.AddCalculation(c, builder.WithCalculationPath("calculations/accounting-schedule-calculation.json")); err != nil {
		return err
	}
	return b.AddFilePart(accountingschedule.ArtifactPath, "application/json", builder.RoleCalculation, body)
}

func entry(id, date, description, obligation string, postings ...accountingschedule.Posting) accountingschedule.Entry {
	return accountingschedule.Entry{ID: id, Date: date, Description: description, ObligationID: obligation, Postings: postings}
}

func dr(role, amount string) accountingschedule.Posting {
	return accountingschedule.Posting{AccountRole: role, Debit: amount}
}
func cr(role, amount string) accountingschedule.Posting {
	return accountingschedule.Posting{AccountRole: role, Credit: amount}
}

type claimSpec struct {
	id, statement, conclusion string
	refs, assertions          []string
	facts, evidence, calcs    []string
	judgments                 []string
}

func claim(b *builder.Builder, c claimSpec) error {
	review := false
	return b.AddClaim(vwp.Claim{
		Id: c.id, Statement: c.statement, Domain: "finance", StandardRefs: c.refs, Assertions: c.assertions,
		SupportingFacts: c.facts, SupportingEvidence: c.evidence, SupportingCalculations: c.calcs, Judgments: c.judgments,
		Conclusion: c.conclusion, Status: "supported", RequiresReview: &review,
	})
}

func all(steps ...func() error) error {
	for _, s := range steps {
		if err := s(); err != nil {
			return err
		}
	}
	return nil
}

func m(kv ...any) map[string]any {
	out := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// --- fixtures ---

var fixtures = []fixture{
	{
		dir: "asc350-60-crypto-assets", extension: vwp.ExtensionASC35060, domain: "accounting.cryptoAssets",
		title:   "ASC 350-60 Crypto Asset Accounting - DemoCo Q2 2026",
		summary: "BTC and ETH are in scope of ASC 350-60 and measured at fair value; the self-issued WDEMO token is out of scope. The Q2 roll-forward ties from 1,850,000 USD to 1,550,000 USD and both holdings are significant for disclosure.",
		refs:    []string{"ASC 350-60", "ASC 350-30-30-1"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_holdings_extract", "subledgerExtract", "Digital asset subledger holdings extract as of 2026-06-30", "evidence/holdings-extract.txt",
						"DemoCo digital asset holdings as of 2026-06-30\n\nBTC units held: 10.00000000\nETH units held: 400.00000000\nWDEMO units held: 5000000.00000000 (DemoCo-issued utility token)\n\nQ2 2026 activity: 100 ETH sold on 2026-05-15 for 250,000.00 USD; no BTC activity.\n")
				},
				func() error {
					return ev(b, "ev_period_end_prices", "pricingRecord", "Period-end prices from the principal market, 2026-06-30", "evidence/period-end-prices.csv",
						"asset,price_usd,source,asof\nBTC,65000.00,demo-exchange,2026-06-30\nETH,2250.00,demo-exchange,2026-06-30\n")
				},
				func() error {
					return ev(b, "ev_wdemo_terms", "contractDocument", "WDEMO token terms excerpt", "evidence/wdemo-token-terms.txt",
						"WDEMO Token Terms (excerpt)\n\n1. Issuer. WDEMO is created and issued by DemoCo Inc. on the Demo Chain.\n2. Utility. WDEMO grants holders discounted access to DemoCo services.\n")
				},
				func() error {
					return fact(b, "s1_f_btc_units", "DemoCo held 10 BTC at 2026-06-30.", "quantity", 10, "ev_holdings_extract", "BTC units held: 10.00000000")
				},
				func() error {
					return fact(b, "s1_f_eth_units", "DemoCo held 400 ETH at 2026-06-30.", "quantity", 400, "ev_holdings_extract", "ETH units held: 400.00000000")
				},
				func() error {
					return fact(b, "s1_f_wdemo_issuer", "WDEMO is created and issued by DemoCo itself.", "term", "DemoCo-issued", "ev_wdemo_terms", "WDEMO is created and issued by DemoCo Inc.")
				},
				func() error {
					return fact(b, "s2_f_btc_price", "The principal-market BTC price at 2026-06-30 was 65,000.00 USD.", "amount", 65000, "ev_period_end_prices", "BTC,65000.00,demo-exchange,2026-06-30")
				},
				func() error {
					return fact(b, "s2_f_eth_price", "The principal-market ETH price at 2026-06-30 was 2,250.00 USD.", "amount", 2250, "ev_period_end_prices", "ETH,2250.00,demo-exchange,2026-06-30")
				},
				func() error {
					return fact(b, "s3_f_eth_sale", "100 ETH were sold on 2026-05-15 for 250,000.00 USD.", "amount", 250000, "ev_holdings_extract", "100 ETH sold on 2026-05-15 for 250,000.00 USD")
				},
				func() error {
					return judgment(b, "s1_j_wdemo_scope", "ASC 350-60 scope of the self-issued WDEMO token",
						"Is WDEMO, a token DemoCo created and issued, a crypto asset within ASC 350-60-15-1?",
						"No. WDEMO fails criterion (f): it was created by the reporting entity. It is not recognized as an asset of the issuer; the issuance is analysed separately.",
						"ASC 350-60-15-1(f) excludes assets created or issued by the reporting entity or its related parties. Token terms section 1 establishes DemoCo as issuer.",
						vwp.JudgmentAlternative{Conclusion: "In scope, measured at fair value", Notes: "Rejected: criterion (f) is explicit; self-issued tokens held in treasury are not assets of the issuer."}, "medium")
				},
				func() error {
					return register(b, validators.RegisterCryptoAssets, m(
						"description", "ASC 350-60 scope register for DemoCo holdings at 2026-06-30. Synthetic data.",
						"assets", []any{
							m("id", "ca_btc", "asset", "BTC", "chain", "bitcoin", "inScope", true, "criteriaEvidence", []any{"ev_holdings_extract"},
								"criteria", m("intangibleAsset", true, "noEnforceableRightsToGoodsOrServices", true, "distributedLedger", true, "securedThroughCryptography", true, "fungible", true, "notCreatedOrIssuedByReportingEntity", true)),
							m("id", "ca_eth", "asset", "ETH", "chain", "ethereum", "inScope", true, "criteriaEvidence", []any{"ev_holdings_extract"},
								"criteria", m("intangibleAsset", true, "noEnforceableRightsToGoodsOrServices", true, "distributedLedger", true, "securedThroughCryptography", true, "fungible", true, "notCreatedOrIssuedByReportingEntity", true)),
							m("id", "ca_wdemo", "asset", "WDEMO", "chain", "demo-chain", "inScope", false, "criteriaEvidence", []any{"ev_wdemo_terms"},
								"criteria", m("intangibleAsset", true, "noEnforceableRightsToGoodsOrServices", false, "distributedLedger", true, "securedThroughCryptography", true, "fungible", true, "notCreatedOrIssuedByReportingEntity", false),
								"scopeJudgmentRef", "s1_j_wdemo_scope", "outOfScopeTreatment", "Not an asset of the issuer; token issuance analysed under the token issuance work product"),
						}))
				},
				func() error {
					return calc(b, "s3_roll_forward", "cryptoAssetRollForward",
						"Per asset: openingBalance + additions - dispositions + gains - losses = closingBalance (ASC 350-60-50-3). Opening at 2026-03-31 fair values (BTC 60,000; ETH 2,500), closing at 2026-06-30 fair values.",
						validators.ValidatorASC35060RollForwardTieOut,
						m("currency", "USD", "periodStart", "2026-04-01", "periodEnd", "2026-06-30", "lines", []any{
							m("asset", "BTC", "openingBalance", 600000, "additions", 0, "dispositions", 0, "gains", 50000, "losses", 0, "closingBalance", 650000),
							m("asset", "ETH", "openingBalance", 1250000, "additions", 0, "dispositions", 250000, "gains", 0, "losses", 100000, "closingBalance", 900000),
						}),
						m("totals", m("openingBalance", 1850000, "additions", 0, "dispositions", 250000, "gains", 50000, "losses", 100000, "closingBalance", 1550000)))
				},
				func() error {
					return schedule(b, "s3_accounting_schedule", validators.ValidatorASC35060AccountingScheduleTieOut, []string{"s3_roll_forward"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "Q2 2026 crypto asset remeasurement", Kind: accountingschedule.KindFairValueRemeasurement, Currency: "USD", AmountScale: 2,
						StartDate: "2026-04-01", EndDate: "2026-06-30", TotalAmount: "150000.00",
						Entries: []accountingschedule.Entry{
							entry("BTC-2026-06-30-gain", "2026-06-30", "BTC remeasurement gain for Q2 2026", "BTC", dr(validators.RoleCryptoAsset, "50000.00"), cr(validators.RoleCryptoAssetGainLoss, "50000.00")),
							entry("ETH-2026-06-30-loss", "2026-06-30", "ETH remeasurement loss for Q2 2026", "ETH", dr(validators.RoleCryptoAssetGainLoss, "100000.00"), cr(validators.RoleCryptoAsset, "100000.00")),
						},
					})
				},
				func() error {
					return calc(b, "s5_significant_holdings", "significantCryptoHoldings",
						"Each holding at units x period-end price; significant = more than 5% of total crypto asset fair value (ASC 350-60-50-1).",
						validators.ValidatorASC35060HoldingsTieOut,
						m("currency", "USD", "asOf", "2026-06-30", "totalFairValue", 1550000, "holdings", []any{
							m("asset", "BTC", "units", 10, "costBasis", 450000, "fairValue", 650000, "significant", true, "restricted", false),
							m("asset", "ETH", "units", 400, "costBasis", 800000, "fairValue", 900000, "significant", true, "restricted", false),
						}),
						m("significantFairValue", 1550000, "otherFairValue", 0, "significantCostBasis", 1250000, "otherCostBasis", 0))
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_scope", statement: "BTC and ETH meet all six ASC 350-60-15-1 criteria and are accounted for under ASC 350-60; WDEMO is excluded as a self-issued token.",
						conclusion: "Scope register complete; one asset out of scope with a judgment.", refs: []string{"ASC 350-60-15-1"}, assertions: []string{"existence", "rightsAndObligations"},
						facts: []string{"s1_f_btc_units", "s1_f_eth_units", "s1_f_wdemo_issuer"}, evidence: []string{"ev_holdings_extract", "ev_wdemo_terms"}, judgments: []string{"s1_j_wdemo_scope"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_measurement", statement: "In-scope crypto assets were initially measured at cost including transaction costs under ASC 350-30-30-1 and are subsequently measured at fair value under ASC 350-60-35-1 using principal-market quoted prices at 2026-06-30: BTC 650,000 USD and ETH 900,000 USD.",
						conclusion: "Initial measurement per other GAAP; subsequent Level 1 fair value from the principal market.", refs: []string{"ASC 350-30-30-1", "ASC 350-60-35-1", "ASC 820-10-35-5"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s1_f_btc_units", "s1_f_eth_units", "s2_f_btc_price", "s2_f_eth_price"}, evidence: []string{"ev_period_end_prices"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_roll_forward", statement: "The Q2 2026 crypto asset roll-forward reconciles opening 1,850,000 USD to closing 1,550,000 USD through 250,000 USD of dispositions, 50,000 USD of gains and 100,000 USD of losses.",
						conclusion: "Roll-forward ties per asset and in total.", refs: []string{"ASC 350-60-50-3"}, assertions: []string{"completeness", "valuationAllocation"},
						facts: []string{"s3_f_eth_sale"}, calcs: []string{"s3_roll_forward"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_presentation", statement: "Crypto assets are presented separately from other intangible assets on the balance sheet, and remeasurement gains and losses are presented separately in net income.",
						conclusion: "Presentation follows ASC 350-60-45-1 and 45-2.", refs: []string{"ASC 350-60-45-1", "ASC 350-60-45-2"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s3_roll_forward"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_disclosure", statement: "Both holdings are significant and are disclosed with units, cost basis and fair value; no holding is subject to a contractual sale restriction; the cost basis method for realized gains and losses is FIFO.",
						conclusion: "Significant holdings table ties to total fair value and to the roll-forward.", refs: []string{"ASC 350-60-50-1", "ASC 350-60-50-2", "ASC 350-60-50-6"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s5_significant_holdings", "s3_roll_forward"}})
				},
			)
		},
	},
	{
		dir: "asc350-30-impairment", extension: vwp.ExtensionASC35030, domain: "accounting.intangibles",
		title:   "ASC 350-30 Impairment Test - DemoCo NFT Holdings Q2 2026",
		summary: "Two NFTs held outside ASC 350-60 (non-fungible) are tested for impairment at 2026-06-30 against the lowest observable floor price; NFT-001 is impaired by 15,000 USD, NFT-002 is not.",
		refs:    []string{"ASC 350-30"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_nft_register", "subledgerExtract", "NFT lot register as of 2026-06-30", "evidence/nft-lot-register.txt",
						"DemoCo NFT lots as of 2026-06-30\n\nLot NFT-001: DemoPunks #4821, carrying amount 40,000.00 USD, acquired 2025-11-02\nLot NFT-002: DemoApes #77, carrying amount 15,000.00 USD, acquired 2026-02-14\n\nEach NFT is unique and non-fungible.\n")
				},
				func() error {
					return ev(b, "ev_floor_prices", "pricingRecord", "Lowest observable floor price per collection during Q2 2026", "evidence/floor-price-observations.csv",
						"lot,collection,lowest_observed_usd,observed_at,venue\nNFT-001,DemoPunks,25000.00,2026-05-21T14:05:00Z,demo-marketplace\nNFT-002,DemoApes,18000.00,2026-06-02T09:30:00Z,demo-marketplace\n")
				},
				func() error {
					return fact(b, "s1_f_non_fungible", "Each NFT held is unique and non-fungible.", "term", "non-fungible", "ev_nft_register", "Each NFT is unique and non-fungible.")
				},
				func() error {
					return fact(b, "s2_f_lot1_carrying", "Lot NFT-001 has a carrying amount of 40,000.00 USD.", "amount", 40000, "ev_nft_register", "Lot NFT-001: DemoPunks #4821, carrying amount 40,000.00 USD")
				},
				func() error {
					return fact(b, "s2_f_lot2_carrying", "Lot NFT-002 has a carrying amount of 15,000.00 USD.", "amount", 15000, "ev_nft_register", "Lot NFT-002: DemoApes #77, carrying amount 15,000.00 USD")
				},
				func() error {
					return fact(b, "s3_f_lot1_fv", "The lowest observed floor price for NFT-001's collection in Q2 2026 was 25,000.00 USD.", "amount", 25000, "ev_floor_prices", "NFT-001,DemoPunks,25000.00,2026-05-21T14:05:00Z,demo-marketplace")
				},
				func() error {
					return fact(b, "s3_f_lot2_fv", "The lowest observed floor price for NFT-002's collection in Q2 2026 was 18,000.00 USD.", "amount", 18000, "ev_floor_prices", "NFT-002,DemoApes,18000.00,2026-06-02T09:30:00Z,demo-marketplace")
				},
				func() error {
					return judgment(b, "s1_j_scope", "Standard, rights and useful life of the NFT holdings",
						"What rights does each NFT convey, which standard applies, and is the useful life indefinite?",
						"Each NFT conveys only ownership of the token and a non-exclusive display licence; no right to goods, services, royalties or other assets. They fail ASC 350-60-15-1(e) (not fungible) and are intangible assets under ASC 350-30 with an indefinite useful life (ASC 350-30-35-4: no legal, contractual or economic limit), so they are carried at cost less impairment and tested under ASC 350-30-35-18.",
						"Token terms and licence (collection documentation); ASC 350-60-15-1(e); ASC 350-30-35-1 to 35-4 useful-life factors; ASC 350-30-35-18 impairment of indefinite-lived intangibles.",
						vwp.JudgmentAlternative{Conclusion: "Finite-lived intangible amortized over a licence term", Notes: "Rejected: the licence has no term and no economic factor limits the useful life."}, "medium")
				},
				func() error {
					return judgment(b, "s3_j_fair_value_basis", "Fair value basis for the impairment test",
						"Which observable price is the fair value for the impairment test of each NFT?",
						"The lowest observable floor price for the collection during the period on the principal marketplace, consistent with the practice of testing at the lowest intraday observable price.",
						"ASC 350-30-35-19 compares carrying amount to fair value; ASC 820 principal-market pricing; floor price is the most representative observable input for a unique asset.",
						vwp.JudgmentAlternative{Conclusion: "Period-end floor price only", Notes: "Rejected: an impairment indicator during the period cannot be reversed by a later recovery (ASC 350-30-35-20)."}, "high")
				},
				func() error {
					return calc(b, "s4_impairment_test", "indefiniteLivedIntangibleImpairmentTest",
						"Per lot: impairmentLoss = max(0, carryingAmount - fairValue); carryingAmountAfter = carryingAmount - impairmentLoss (ASC 350-30-35-19).",
						validators.ValidatorASC35030ImpairmentRecompute,
						m("currency", "USD", "testDate", "2026-06-30", "fairValueBasis", "lowest observable floor price in the principal market during Q2 2026", "lots", []any{
							m("lotId", "NFT-001", "asset", "DemoPunks #4821", "units", 1, "carryingAmount", 40000, "fairValue", 25000, "fairValueFact", "s3_f_lot1_fv"),
							m("lotId", "NFT-002", "asset", "DemoApes #77", "units", 1, "carryingAmount", 15000, "fairValue", 18000, "fairValueFact", "s3_f_lot2_fv"),
						}),
						m("lots", []any{
							m("lotId", "NFT-001", "impairmentLoss", 15000, "carryingAmountAfter", 25000),
							m("lotId", "NFT-002", "impairmentLoss", 0, "carryingAmountAfter", 15000),
						}, "totalImpairmentLoss", 15000))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorASC35030AccountingScheduleTieOut, []string{"s4_impairment_test"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "2026-06-30 NFT impairment", Kind: accountingschedule.KindImpairment, Currency: "USD", AmountScale: 2,
						StartDate: "2026-06-30", EndDate: "2026-06-30", TotalAmount: "15000.00",
						Entries: []accountingschedule.Entry{
							entry("NFT-001-2026-06-30", "2026-06-30", "Impairment of DemoPunks #4821 to fair value", "NFT-001", dr(validators.RoleImpairmentLoss, "15000.00"), cr(validators.RoleIntangibleAsset, "15000.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_scope", statement: "The NFT holdings are indefinite-lived intangible assets under ASC 350-30, outside ASC 350-60 because they are not fungible.",
						conclusion: "ASC 350-30 applies; indefinite useful life.", refs: []string{"ASC 350-60-15-1", "ASC 350-30-35-4", "ASC 350-30-35-18"}, assertions: []string{"existence", "rightsAndObligations"},
						facts: []string{"s1_f_non_fungible"}, judgments: []string{"s1_j_scope"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_impairment", statement: "NFT-001 is impaired by 15,000 USD to a new carrying amount of 25,000 USD; NFT-002's fair value exceeds its carrying amount and no impairment is recorded.",
						conclusion: "Total impairment loss 15,000 USD; no reversal of prior impairments.", refs: []string{"ASC 350-30-35-19", "ASC 350-30-35-20"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s2_f_lot1_carrying", "s2_f_lot2_carrying", "s3_f_lot1_fv", "s3_f_lot2_fv"}, calcs: []string{"s4_impairment_test"}, judgments: []string{"s3_j_fair_value_basis"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_presentation", statement: "The impairment loss is presented within income from continuing operations and the carrying amount after impairment becomes the new cost basis.",
						conclusion: "Presentation per ASC 350-30-45-1.", refs: []string{"ASC 350-30-45-1"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s4_impairment_test"}})
				},
			)
		},
	},
	{
		dir: "staking-rewards", extension: vwp.ExtensionStakingReward, domain: "accounting.stakingRewards",
		title:   "Staking Rewards Recognition - DemoCo ETH Validator April 2026",
		summary: "DemoCo operates its own validator and is the principal; rewards are recognized as other income when received, measured at the receipt-date fair value. April 2026 rewards of 1.5 ETH tie to the beacon-chain record and total 3,750 USD.",
		refs:    []string{"ASC 606-10-15-3", "ASC 606-10-55-36", "ASC 350-30-30-1"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_validator_rewards", "chainRecord", "Validator reward receipts, April 2026 (beacon-chain export)", "evidence/validator-rewards.csv",
						"date,units_eth,reference\n2026-04-10,0.50000000,epoch-281000\n2026-04-20,0.50000000,epoch-283250\n2026-04-30,0.50000000,epoch-285500\nTOTAL,1.50000000,april-2026\n")
				},
				func() error {
					return ev(b, "ev_eth_prices", "pricingRecord", "ETH daily closing prices, April 2026", "evidence/eth-daily-prices.csv",
						"date,close_usd,source\n2026-04-10,2400.00,demo-exchange\n2026-04-20,2500.00,demo-exchange\n2026-04-30,2600.00,demo-exchange\n")
				},
				func() error {
					return ev(b, "ev_staking_terms", "contractDocument", "Validator operations memo", "evidence/staking-terms.txt",
						"DemoCo Validator Operations (excerpt)\n\nDemoCo operates validator 0xDEMO on the Ethereum consensus layer using its own 32 ETH stake and its own signing keys.\nNo third-party staking provider is engaged and no rewards are shared.\nRewards accrue to the withdrawal address controlled by DemoCo treasury.\n")
				},
				func() error {
					return fact(b, "s1_f_own_validator", "DemoCo runs the validator with its own stake and keys; no staking provider is involved.", "term", "self-operated", "ev_staking_terms", "DemoCo operates validator 0xDEMO on the Ethereum consensus layer using its own 32 ETH stake and its own signing keys.")
				},
				func() error {
					return fact(b, "s1_f_no_sharing", "No rewards are shared with a third party.", "term", "no sharing", "ev_staking_terms", "No third-party staking provider is engaged and no rewards are shared.")
				},
				func() error {
					return fact(b, "s3_f_withdrawal_control", "Rewards accrue to a withdrawal address controlled by DemoCo treasury.", "term", "control", "ev_staking_terms", "Rewards accrue to the withdrawal address controlled by DemoCo treasury.")
				},
				func() error {
					return fact(b, "s4_f_chain_total", "The beacon-chain record shows 1.5 ETH of rewards received in April 2026.", "quantity", m("units", 1.5, "periodStart", "2026-04-01", "periodEnd", "2026-04-30"), "ev_validator_rewards", "TOTAL,1.50000000,april-2026")
				},
				func() error {
					return fact(b, "s4_f_price_0410", "ETH closed at 2,400.00 USD on 2026-04-10.", "price", m("price", 2400, "date", "2026-04-10", "currency", "USD"), "ev_eth_prices", "2026-04-10,2400.00,demo-exchange")
				},
				func() error {
					return fact(b, "s4_f_price_0420", "ETH closed at 2,500.00 USD on 2026-04-20.", "price", m("price", 2500, "date", "2026-04-20", "currency", "USD"), "ev_eth_prices", "2026-04-20,2500.00,demo-exchange")
				},
				func() error {
					return fact(b, "s4_f_price_0430", "ETH closed at 2,600.00 USD on 2026-04-30.", "price", m("price", 2600, "date", "2026-04-30", "currency", "USD"), "ev_eth_prices", "2026-04-30,2600.00,demo-exchange")
				},
				func() error {
					return judgment(b, "s2_j_principal", "Customer identification, principal versus agent and nature of the reward",
						"Is there a customer for DemoCo's validation service, is DemoCo the principal, and is the reward revenue under ASC 606 (directly or by analogy) or other income?",
						"DemoCo is the principal (it controls the validator and bears slashing risk). No delegators or protocol counterparty contract for the service, so ASC 606 is applied by analogy for recognition and noncash-consideration measurement, with the reward presented as other income rather than revenue from contracts with customers.",
						"ASC 606-10-15-3 customer definition; ASC 606-10-55-36 to 55-40 control indicators; ASC 606-10-32-21 to 32-23 noncash consideration (applied by analogy); AICPA Digital Assets Practice Aid staking Q&A; interpretive views differ on whether a protocol can be a customer, so the conclusion is a judgment.",
						vwp.JudgmentAlternative{Conclusion: "ASC 606 revenue with the protocol as customer", Notes: "Considered: supportable where the entity's ordinary activity is validation and it treats the protocol as the party receiving the service; would change the income statement caption but not the measurement basis."}, "high")
				},
				func() error {
					return judgment(b, "s3_j_timing", "Recognition timing and pricing date of validator rewards",
						"Are rewards recognized as validation duties are performed (over time, with the variable-consideration constraint), when credited to the withdrawal address, or when claimed — and which date's price measures them?",
						"Recognized when the reward is credited and withdrawable to the DemoCo-controlled address, which on this chain coincides with the end of each duty period: the amount is then fixed and the asset is controlled. Each reward is measured at the fair value on that recognition date (pricingDateBasis recognitionDate); subsequent price movements are ASC 350-60 remeasurement, not staking income. No reward straddles the 2026-04-30 reporting boundary.",
						"ASC 606-10-25-27 to 25-30 (satisfaction over time / control) and 606-10-32-11 constraint, applied by analogy; ASC 606-10-32-21 noncash consideration measured at inception fair value — the arrangement has no inception price for a protocol reward, so the recognition-date price is the inception-equivalent; ASC 350-30-30-1 initial measurement of an intangible received.",
						vwp.JudgmentAlternative{Conclusion: "Accrue daily as duties are performed with the constraint applied", Notes: "Considered: yields the same amounts here because credit occurs each period; would differ where credit lags performance across a reporting date — then accrue the constrained estimate and true up at credit."}, "medium")
				},
				func() error {
					return calc(b, "s4_rewards_schedule", "stakingRewardsSchedule",
						"Each reward at units x closing price on its recognition date (credit coincides with the end of each duty period, so inception, recognition and receipt fall on the same day); totals tie to the beacon-chain export; no unreceived balance at either period end.",
						validators.ValidatorStakingRewardsRecompute,
						m("asset", "ETH", "currency", "USD", "arrangement", "validator-0xDEMO", "pricingDateBasis", "recognitionDate", "periodStart", "2026-04-01", "periodEnd", "2026-04-30", "chainObservedUnits", 1.5, "chainObservedFact", "s4_f_chain_total", "rewards", []any{
							m("date", "2026-04-10", "receivedOn", "2026-04-10", "measurementDate", "2026-04-10", "units", 0.5, "unitPrice", 2400, "priceFact", "s4_f_price_0410", "reference", "epoch-281000"),
							m("date", "2026-04-20", "receivedOn", "2026-04-20", "measurementDate", "2026-04-20", "units", 0.5, "unitPrice", 2500, "priceFact", "s4_f_price_0420", "reference", "epoch-283250"),
							m("date", "2026-04-30", "receivedOn", "2026-04-30", "measurementDate", "2026-04-30", "units", 0.5, "unitPrice", 2600, "priceFact", "s4_f_price_0430", "reference", "epoch-285500"),
						}),
						m("rows", []any{
							m("date", "2026-04-10", "units", 0.5, "fairValue", 1200),
							m("date", "2026-04-20", "units", 0.5, "fairValue", 1250),
							m("date", "2026-04-30", "units", 0.5, "fairValue", 1300),
						}, "totalUnits", 1.5, "totalFairValue", 3750))
				},
				func() error {
					return schedule(b, "s4_accounting_schedule", validators.ValidatorStakingAccountingScheduleTieOut, []string{"s4_rewards_schedule"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "April 2026 staking reward income", Kind: accountingschedule.KindStakingRewards, Currency: "USD", AmountScale: 2,
						StartDate: "2026-04-01", EndDate: "2026-04-30", TotalAmount: "3750.00",
						Entries: []accountingschedule.Entry{
							entry("epoch-281000", "2026-04-10", "0.5 ETH staking reward, epoch 281000, at 2,400.00", "epoch-281000", dr(validators.RoleCryptoAsset, "1200.00"), cr(validators.RoleStakingRewardIncome, "1200.00")),
							entry("epoch-283250", "2026-04-20", "0.5 ETH staking reward, epoch 283250, at 2,500.00", "epoch-283250", dr(validators.RoleCryptoAsset, "1250.00"), cr(validators.RoleStakingRewardIncome, "1250.00")),
							entry("epoch-285500", "2026-04-30", "0.5 ETH staking reward, epoch 285500, at 2,600.00", "epoch-285500", dr(validators.RoleCryptoAsset, "1300.00"), cr(validators.RoleStakingRewardIncome, "1300.00")),
						},
					})
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_arrangement", statement: "DemoCo self-operates an Ethereum validator with its own stake and keys and shares no rewards with any provider.",
						conclusion: "Arrangement facts established.", refs: []string{"ASC 606-10-55-36"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s1_f_own_validator", "s1_f_no_sharing"}, evidence: []string{"ev_staking_terms"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_principal", statement: "DemoCo is the principal; with no customer identified, ASC 606 is applied by analogy and the rewards are presented as other income.",
						conclusion: "Principal; ASC 606 by analogy; other income caption.", refs: []string{"ASC 606-10-55-36", "ASC 606-10-15-3"}, assertions: []string{"occurrence", "presentationDisclosure"},
						facts: []string{"s1_f_own_validator"}, judgments: []string{"s2_j_principal"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_timing", statement: "Rewards are recognized when credited to the DemoCo-controlled withdrawal address, which coincides with the end of each duty period, and measured at that date's price.",
						conclusion: "Recognize on credit; recognition-date price.", refs: []string{"ASC 606-10-25-30", "ASC 606-10-32-21"}, assertions: []string{"occurrence"},
						facts: []string{"s3_f_withdrawal_control"}, judgments: []string{"s3_j_timing"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_measurement", statement: "April 2026 rewards of 1.5 ETH are measured at recognition-date fair value totalling 3,750 USD and tie to the beacon-chain record.",
						conclusion: "Schedule recomputes and ties to chain.", refs: []string{"ASC 606-10-32-21", "ASC 350-30-30-1"}, assertions: []string{"completeness", "valuationAllocation"},
						facts: []string{"s4_f_chain_total", "s4_f_price_0410", "s4_f_price_0420", "s4_f_price_0430"}, calcs: []string{"s4_rewards_schedule"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_subsequent", statement: "Received ETH rewards are in-scope crypto assets subsequently measured at fair value under ASC 350-60, with remeasurement in net income.",
						conclusion: "Subsequent accounting under ASC 350-60.", refs: []string{"ASC 350-60-35-1"}, assertions: []string{"valuationAllocation"},
						calcs: []string{"s4_rewards_schedule"}})
				},
			)
		},
	},
	{
		dir: "digital-asset-lending", extension: vwp.ExtensionLending, domain: "accounting.lending",
		title:   "Digital Asset Loan Accounting - DemoCo 100 ETH Loan to Borrower X",
		summary: "DemoCo lent 100 ETH for a 91-day term at 5% in kind; the borrower may rehypothecate, so control was surrendered (ASC 860-10-40-5 by analogy) and the ETH is derecognized for a crypto asset loan receivable measured at the fair value of the ETH to be returned, with a separate ASC 326 allowance. The fee accrues to 1.2466 ETH over the term; collateral of 400,000 USDC, marked daily, covers the 250,000 USD receivable at 1.6x against a 1.5x maintenance requirement, so the ASC 326-20-35-6 collateral maintenance expedient yields no allowance.",
		refs:    []string{"ASC 860-10-40-5", "ASC 326-20-35-6"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_loan_agreement", "contractDocument", "Master loan agreement excerpt", "evidence/loan-agreement-excerpt.txt",
						"Digital Asset Loan Agreement (excerpt)\n\n1. Principal: 100 ETH delivered to Borrower on 2026-04-01.\n2. Term: 2026-04-01 to 2026-07-01 (91 days).\n3. Fee: 5.00% per annum, accrued in ETH on an actual/365 basis, payable at maturity.\n4. Collateral: Borrower posts USDC equal to at least 150% of the loan value, marked daily.\n5. Use: Borrower may sell, lend or re-pledge the ETH without restriction during the term.\n")
				},
				func() error {
					return ev(b, "ev_collateral_statement", "custodianStatement", "Collateral account statement as of 2026-06-30", "evidence/collateral-statement.txt",
						"Collateral account DEMO-COLL-01 as of 2026-06-30\nUSDC balance: 400,000.00\nHeld for: DemoCo loan L-2026-04\n")
				},
				func() error {
					return ev(b, "ev_eth_price", "pricingRecord", "ETH price at 2026-06-30", "evidence/eth-price-2026-06-30.txt",
						"ETH/USD close 2026-06-30: 2,500.00 (demo-exchange)\n")
				},
				func() error {
					return fact(b, "s1_f_terms", "The loan is 100 ETH at 5.00% per annum, actual/365, from 2026-04-01 to 2026-07-01.", "term", m("principalUnits", 100, "annualRate", 0.05, "dayCountBasis", 365), "ev_loan_agreement", "3. Fee: 5.00% per annum, accrued in ETH on an actual/365 basis, payable at maturity.")
				},
				func() error {
					return fact(b, "s1_f_term_dates", "The loan term runs 2026-04-01 to 2026-07-01.", "date", "2026-04-01/2026-07-01", "ev_loan_agreement", "2. Term: 2026-04-01 to 2026-07-01 (91 days).")
				},
				func() error {
					return fact(b, "s2_f_rehypothecation", "The borrower may sell, lend or re-pledge the ETH without restriction.", "term", "unrestricted use", "ev_loan_agreement", "5. Use: Borrower may sell, lend or re-pledge the ETH without restriction during the term.")
				},
				func() error {
					return fact(b, "s4_f_collateral", "Collateral of 400,000.00 USDC was held at 2026-06-30.", "amount", 400000, "ev_collateral_statement", "USDC balance: 400,000.00")
				},
				func() error {
					return fact(b, "s4_f_required_ratio", "Collateral must be at least 150% of the loan value.", "ratio", 1.5, "ev_loan_agreement", "4. Collateral: Borrower posts USDC equal to at least 150% of the loan value, marked daily.")
				},
				func() error {
					return fact(b, "s4_f_eth_price", "ETH closed at 2,500.00 USD on 2026-06-30.", "amount", 2500, "ev_eth_price", "ETH/USD close 2026-06-30: 2,500.00")
				},
				func() error {
					return fact(b, "s3_f_fee_price", "The accrued fee is measured at the ETH close of 2,500.00 USD on 2026-06-30, the last accrued day of the quarter.", "price", m("price", 2500, "date", "2026-06-30"), "ev_eth_price", "ETH/USD close 2026-06-30: 2,500.00")
				},
				func() error {
					return judgment(b, "s2_j_derecognition", "Derecognition of the lent ETH",
						"Did DemoCo surrender control of the 100 ETH such that the ETH (an ASC 350-60 intangible, not a financial asset) is derecognized and a crypto asset loan receivable recognized?",
						"Yes. Applying ASC 860-10-40-5 by analogy, as the AICPA Digital Assets Practice Aid and the SEC staff view do for crypto intangibles: the borrower may sell, lend or re-pledge the ETH without restriction, DemoCo has no right to specific units back, and the ETH is beyond DemoCo's reach. Derecognize the ETH; recognize a crypto asset loan receivable for 100 ETH plus the accrued fee, measured at the fair value of the ETH to be returned at each reporting date with changes in income, and assess it for credit losses under ASC 326.",
						"ASC 860-10-40-5(a)-(c) by analogy (isolation, transferee's right to pledge or exchange, no effective control); ASC 610-20 derecognition of nonfinancial assets considered; AICPA Digital Assets Practice Aid crypto lending Q&A; clause 5 grants unrestricted use.",
						vwp.JudgmentAlternative{Conclusion: "Keep the ETH on balance sheet as a secured borrowing", Notes: "Rejected: effective control is not retained — no repurchase of the same units, no call on specific assets."}, "high")
				},
				func() error {
					return judgment(b, "s4_j_credit_loss", "Expected credit loss on the ETH receivable",
						"What allowance for credit losses applies to the crypto asset loan receivable under ASC 326-20, and which expedient (if any) is available?",
						"The agreement requires the borrower to continually adjust USDC collateral to at least 150% of the loan value, marked daily, and the borrower has met every call; the ASC 326-20-35-6 collateral maintenance practical expedient applies and expected credit losses are measured on the unsecured portion — the carrying amount of 250,000 USD less collateral fair value of 400,000 USD, floored at zero — which is zero at 2026-06-30. The 150% margin shortfall is a separate margin-call measure and is not the allowance. The collateral-dependent measurement in 35-5 is not available: the borrower is not in financial difficulty.",
						"ASC 326-20-35-6 (collateral maintenance provisions) versus 35-5 (collateral-dependent assets of borrowers in financial difficulty); clause 4 of the agreement; collateral statement; margin-call history.",
						vwp.JudgmentAlternative{Conclusion: "Estimate an allowance from borrower PD/LGD without the expedient", Notes: "Considered: required if collateral maintenance lapses or coverage falls below the requirement; revisit on any missed call."}, "medium")
				},
				func() error {
					return calc(b, "s3_interest_accrual", "digitalAssetLoanInterestAccrual",
						"accruedUnits = principalUnits x annualRate x days / dayCountBasis; days = accrualEnd - accrualStart; accruedFairValue = accruedUnits x unitPrice at the last accrued day.",
						validators.ValidatorLendingAccrualRecompute,
						m("asset", "ETH", "loanId", "L-2026-04", "currency", "USD", "principalUnits", 100, "annualRate", 0.05, "dayCountBasis", 365, "accrualStart", "2026-04-01", "accrualEnd", "2026-07-01", "termsFact", "s1_f_terms",
							"unitPrice", 2500, "priceFact", "s3_f_fee_price"),
						m("days", 91, "accruedUnits", 1.2465753425, "accruedFairValue", 3116.44))
				},
				func() error {
					return schedule(b, "s3_accounting_schedule", validators.ValidatorLendingAccountingScheduleTieOut, []string{"s3_interest_accrual"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "Loan L-2026-04 fee accrual, Q2 2026", Kind: accountingschedule.KindInterestAccrual, Currency: "USD", AmountScale: 2,
						StartDate: "2026-04-01", EndDate: "2026-06-30", TotalAmount: "3116.44",
						Entries: []accountingschedule.Entry{
							entry("L-2026-04-2026-06-30", "2026-06-30", "1.2466 ETH fee accrued 2026-04-01 to 2026-06-30 at 2,500.00", "L-2026-04", dr(validators.RoleInterestReceivable, "3116.44"), cr(validators.RoleInterestIncome, "3116.44")),
						},
					})
				},
				func() error {
					return calc(b, "s4_collateral_coverage", "collateralCoverage",
						"coverageRatio = collateralFairValue / loanFairValue; shortfall = max(0, loanFairValue x requiredCoverageRatio - collateralFairValue) is the margin-call measure; unsecuredPortion = max(0, loanFairValue - collateralFairValue) is the ASC 326-20-35-6 expected credit loss basis.",
						validators.ValidatorLendingCollateralRecompute,
						m("currency", "USD", "asOf", "2026-06-30", "loanId", "L-2026-04", "loanFairValue", 250000, "collateralFairValue", 400000, "requiredCoverageRatio", 1.5,
							"collateralFairValueFact", "s4_f_collateral", "requiredCoverageRatioFact", "s4_f_required_ratio"),
						m("coverageRatio", 1.6, "shortfall", 0, "unsecuredPortion", 0, "adequatelyCollateralized", true))
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_terms", statement: "DemoCo lent 100 ETH for 91 days at a 5% in-kind fee, collateralized at 150% in USDC.",
						conclusion: "Arrangement terms established from the agreement.", refs: []string{"ASC 860-10-05"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s1_f_terms", "s1_f_term_dates"}, evidence: []string{"ev_loan_agreement"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_derecognition", statement: "The lent ETH is derecognized and a crypto asset loan receivable for 100 ETH is recognized: the borrower obtained the right to use, sell and re-pledge the ETH and DemoCo's recall right is for equivalent units, not the units transferred (SEC staff / AICPA Practice Aid analysis for crypto intangibles; ASC 860-10-40-5 considered by analogy).",
						conclusion: "Derecognize; recognize a crypto asset loan receivable.", refs: []string{"AICPA Digital Assets Practice Aid — derecognition of lent crypto assets (SEC staff view)", "ASC 860-10-40-5 (by analogy)"}, assertions: []string{"existence", "rightsAndObligations"},
						facts: []string{"s2_f_rehypothecation"}, judgments: []string{"s2_j_derecognition"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_accrual", statement: "The receivable is measured at the fair value of the ETH to be returned (250,000 USD at 2026-06-30) and the in-kind fee accrues to 1.2466 ETH over the 91-day term, measured at the same ETH price.",
						conclusion: "Receivable at fair value of the lent asset; accrual recomputes from the agreement terms.", refs: []string{"ASC 820-10-35-1", "AICPA Digital Assets Practice Aid (crypto lending)"}, assertions: []string{"valuationAllocation", "completeness"},
						facts: []string{"s1_f_terms", "s4_f_eth_price", "s3_f_fee_price"}, calcs: []string{"s3_interest_accrual", "s3_accounting_schedule"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_credit_loss", statement: "The receivable of 250,000 USD is covered 1.6x by 400,000 USDC of daily-marked collateral against a 1.5x maintenance requirement; under the ASC 326-20-35-6 collateral maintenance expedient the unsecured portion (carrying amount less collateral fair value) is zero and no allowance is recorded at 2026-06-30.",
						conclusion: "Collateral maintenance expedient; no allowance.", refs: []string{"ASC 326-20-35-6"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s4_f_collateral", "s4_f_required_ratio", "s4_f_eth_price"}, calcs: []string{"s4_collateral_coverage"}, judgments: []string{"s4_j_credit_loss"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_disclosure", statement: "The receivable, its collateral and the borrower concentration are disclosed.",
						conclusion: "Disclosure per ASC 860-30-50 and ASC 326-20-50.", refs: []string{"ASC 860-30-50-1", "ASC 326-20-50"}, assertions: []string{"presentationDisclosure"},
						calcs: []string{"s4_collateral_coverage"}})
				},
			)
		},
	},
	{
		dir: "cost-basis-lot-relief", extension: vwp.ExtensionCostBasis, domain: "accounting.costBasis",
		title:   "Cost Basis and Lot Relief - DemoCo BTC Q1 2025",
		summary: "DemoCo elects FIFO lot relief applied wallet by wallet. The 190,000 USD of unused basis on 4 BTC held at 2025-01-01 (all acquired in 2024) is allocated across the two wallets under the Rev. Proc. 2024-28 specific unit allocation recorded on 2024-12-20; the 2.5 BTC disposal on 2025-03-20 then relieves lots L1 and half of L2 for 105,000 USD of basis and a 57,500 USD realized gain.",
		refs:    []string{"ASC 350-60-50-2", "Rev. Proc. 2024-28"},
		period:  []string{"2025-01-01", "2025-03-31"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_lot_ledger", "subledgerExtract", "BTC lot ledger at 2025-01-01", "evidence/lot-ledger.csv",
						"lot,acquired,units_btc,cost_basis_usd,wallet\nL1,2024-06-15,2.00000000,80000.00,W-A\nL2,2024-09-10,1.00000000,50000.00,W-A\nL3,2024-11-05,1.00000000,60000.00,W-B\n")
				},
				func() error {
					return ev(b, "ev_disposals", "subledgerExtract", "BTC disposals, Q1 2025", "evidence/disposals.csv",
						"disposal,date,units_btc,proceeds_usd,wallet\nD1,2025-03-20,2.50000000,162500.00,W-A\n")
				},
				func() error {
					return ev(b, "ev_policy", "policyDocument", "Digital asset cost basis policy", "evidence/cost-basis-policy.txt",
						"DemoCo Digital Asset Cost Basis Policy (excerpt)\n\nLot relief method: FIFO, applied wallet by wallet.\nSpecific unit allocation of unused basis to wallets under Rev. Proc. 2024-28 recorded on 2024-12-20, effective 2025-01-01: 3 BTC (130,000.00 USD) to W-A, 1 BTC (60,000.00 USD) to W-B.\n")
				},
				func() error {
					return fact(b, "s1_f_method", "The elected lot relief method is FIFO applied wallet by wallet.", "term", "FIFO", "ev_policy", "Lot relief method: FIFO, applied wallet by wallet.")
				},
				func() error {
					return fact(b, "s2_f_lot1", "Lot L1: 2 BTC acquired 2024-06-15 for 80,000.00 USD in wallet W-A.", "lot", m("units", 2, "costBasis", 80000), "ev_lot_ledger", "L1,2024-06-15,2.00000000,80000.00,W-A")
				},
				func() error {
					return fact(b, "s2_f_lot2", "Lot L2: 1 BTC acquired 2024-09-10 for 50,000.00 USD in wallet W-A.", "lot", m("units", 1, "costBasis", 50000), "ev_lot_ledger", "L2,2024-09-10,1.00000000,50000.00,W-A")
				},
				func() error {
					return fact(b, "s2_f_lot3", "Lot L3: 1 BTC acquired 2024-11-05 for 60,000.00 USD in wallet W-B.", "lot", m("units", 1, "costBasis", 60000), "ev_lot_ledger", "L3,2024-11-05,1.00000000,60000.00,W-B")
				},
				func() error {
					return fact(b, "s3_f_disposal", "Disposal D1 sold 2.5 BTC from wallet W-A on 2025-03-20 for 162,500.00 USD.", "disposal", m("units", 2.5, "proceeds", 162500), "ev_disposals", "D1,2025-03-20,2.50000000,162500.00,W-A")
				},
				func() error {
					return fact(b, "s5_f_allocation_record", "The specific unit allocation was recorded in the policy on 2024-12-20, effective 2025-01-01.", "record", m("recordedOn", "2024-12-20", "method", "specificUnit"), "ev_policy", "Specific unit allocation of unused basis to wallets under Rev. Proc. 2024-28 recorded on 2024-12-20, effective 2025-01-01")
				},
				func() error {
					return judgment(b, "s1_j_method", "Lot relief method election",
						"Which lot relief method applies, and is it applied per wallet or across all wallets?",
						"FIFO applied wallet by wallet, consistent with the policy and with the Treas. Reg. 1.1012-1(j) wallet-based identification requirement for tax; the same method is used for book to avoid a book-tax lot divergence.",
						"ASC 350-60-50-2 requires disclosure of the cost basis method; Treas. Reg. 1.1012-1(j) requires wallet-by-wallet identification for units held on or after 2025-01-01.",
						vwp.JudgmentAlternative{Conclusion: "Specific identification", Notes: "Considered: permitted, but the policy elects FIFO and the subledger does not tag disposals to lots."}, "medium")
				},
				func() error {
					return calc(b, "s3_lot_relief", "lotRelief",
						"FIFO within wallet W-A: D1 relieves L1 (2 BTC, 80,000) then 0.5 BTC of L2 (25,000). basisRelieved = sum(units x lot unit basis); gain = proceeds - basisRelieved.",
						validators.ValidatorCostBasisLotReliefRecompute,
						m("asset", "BTC", "currency", "USD", "method", "FIFO", "pool", "W-A", "lots", []any{
							m("lotId", "L1", "acquiredAt", "2024-06-15", "units", 2, "costBasis", 80000, "wallet", "W-A"),
							m("lotId", "L2", "acquiredAt", "2024-09-10", "units", 1, "costBasis", 50000, "wallet", "W-A"),
						}, "disposals", []any{
							m("disposalId", "D1", "date", "2025-03-20", "units", 2.5, "proceeds", 162500, "reliefs", []any{m("lotId", "L1", "units", 2), m("lotId", "L2", "units", 0.5)}),
						}),
						m("disposals", []any{m("disposalId", "D1", "basisRelieved", 105000, "realizedGainLoss", 57500)},
							"totalRealizedGainLoss", 57500,
							"remainingLots", []any{m("lotId", "L2", "units", 0.5, "costBasis", 25000)}))
				},
				func() error {
					return schedule(b, "s3_accounting_schedule", validators.ValidatorCostBasisAccountingScheduleTieOut, []string{"s3_lot_relief", "s3_lot_relief_wb"}, accountingschedule.Artifact{
						SchemaVersion: 1, Name: "BTC disposals 2025, realized gain or loss", Kind: accountingschedule.KindRealizedGainLoss, Currency: "USD", AmountScale: 2,
						StartDate: "2025-03-20", EndDate: "2025-03-20", TotalAmount: "162500.00",
						Entries: []accountingschedule.Entry{
							entry("D1-2025-03-20", "2025-03-20", "Disposal D1: 2.5 BTC for 162,500.00, FIFO basis relieved 105,000.00", "D1", dr(validators.RoleDisposalProceeds, "162500.00"), cr(validators.RoleCryptoAsset, "105000.00"), cr(validators.RoleRealizedGainLoss, "57500.00")),
						},
					})
				},
				func() error {
					return calc(b, "s3_lot_relief_wb", "lotRelief",
						"FIFO within wallet W-B: no disposals in the period; the lot carries forward.",
						validators.ValidatorCostBasisLotReliefRecompute,
						m("asset", "BTC", "currency", "USD", "method", "FIFO", "pool", "W-B", "lots", []any{
							m("lotId", "L3", "acquiredAt", "2024-11-05", "units", 1, "costBasis", 60000, "wallet", "W-B"),
						}, "disposals", []any{}),
						m("disposals", []any{}, "totalRealizedGainLoss", 0, "remainingLots", []any{m("lotId", "L3", "units", 1, "costBasis", 60000)}))
				},
				func() error {
					return calc(b, "s5_wallet_allocation", "walletBasisAllocation",
						"Rev. Proc. 2024-28 specific unit allocation, recorded 2024-12-20: the 190,000 USD of unused basis on the 4 BTC held at 2025-01-01 (lots L1-L3, all acquired in 2024) assigned to the wallets holding the units.",
						validators.ValidatorCostBasisWalletAllocTieOut,
						m("asset", "BTC", "currency", "USD", "asOf", "2025-01-01", "allocationMethod", "specificUnit", "methodRecordFact", "s5_f_allocation_record", "totalUnallocatedBasis", 190000, "totalUnits", 4, "wallets", []any{
							m("walletId", "W-A", "units", 3), m("walletId", "W-B", "units", 1),
						}),
						m("allocations", []any{m("walletId", "W-A", "units", 3, "allocatedBasis", 130000), m("walletId", "W-B", "units", 1, "allocatedBasis", 60000)}))
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_policy", statement: "DemoCo's cost basis method is FIFO applied wallet by wallet, disclosed under ASC 350-60-50-2.",
						conclusion: "Method elected and documented.", refs: []string{"ASC 350-60-50-2", "Treas. Reg. 1.1012-1(j)"}, assertions: []string{"presentationDisclosure"},
						facts: []string{"s1_f_method"}, judgments: []string{"s1_j_method"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_lots", statement: "The lot population at 2025-01-01 is three lots acquired in 2024 totalling 4 BTC with 190,000 USD of basis: L1 and L2 in W-A, L3 in W-B.",
						conclusion: "Lot population complete from the subledger.", refs: []string{"ASC 350-60-50-2"}, assertions: []string{"completeness"},
						facts: []string{"s2_f_lot1", "s2_f_lot2", "s2_f_lot3"}, evidence: []string{"ev_lot_ledger"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_relief", statement: "Disposal D1 relieves L1 in full and 0.5 BTC of L2 under FIFO within wallet W-A, for 105,000 USD of basis; wallet W-B had no disposals.",
						conclusion: "Relief follows FIFO within the wallet and conserves units.", refs: []string{"ASC 610-20-40-1", "Treas. Reg. 1.1012-1(j)"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s3_f_disposal"}, calcs: []string{"s3_lot_relief", "s3_lot_relief_wb"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_gain", statement: "The realized gain on D1 is 57,500 USD (proceeds 162,500 less basis 105,000).",
						conclusion: "Gain recomputes.", refs: []string{"ASC 350-60-45-2", "ASC 350-60-50-3"}, assertions: []string{"valuationAllocation", "occurrence"},
						calcs: []string{"s3_lot_relief"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_wallet_allocation", statement: "Under the Rev. Proc. 2024-28 specific unit allocation recorded on 2024-12-20, the 190,000 USD of unused basis on 4 BTC held at 2025-01-01 is allocated 130,000 USD to W-A (3 BTC) and 60,000 USD to W-B (1 BTC); no unit acquired on or after 2025-01-01 is in the allocation.",
						conclusion: "Allocation ties to the pre-2025 lot population.", refs: []string{"Rev. Proc. 2024-28", "Treas. Reg. 1.1012-1(j)"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s5_f_allocation_record"}, calcs: []string{"s5_wallet_allocation"}, judgments: []string{"s1_j_method"}})
				},
			)
		},
	},
	{
		dir: "stablecoin-classification", extension: vwp.ExtensionStablecoin, domain: "accounting.stablecoins",
		title:   "Stablecoin Classification - DemoCo Holdings at 2026-06-30",
		summary: "DUSD held directly with issuer redemption rights at par is a financial asset — a receivable-like claim on the issuer — in both wallets, not cash or a cash equivalent (it has no maturity; AICPA Practice Aid view); XEUR, an algorithmic token with no redemption right, is an ASC 350-60 crypto asset at fair value.",
		refs:    []string{"ASC 305-10-20", "ASC 230-10-20", "ASC 350-60-15-1"},
		build: func(b *builder.Builder) error {
			return all(
				func() error {
					return ev(b, "ev_dusd_terms", "contractDocument", "DUSD issuer terms of service excerpt", "evidence/issuer-terms-excerpt.txt",
						"DUSD Terms of Service (excerpt)\n\n4.1 Verified customers may redeem DUSD 1:1 for US dollars directly with the issuer on any business day.\n4.2 Redemptions settle within one business day by wire to the customer's bank account.\n5.1 Reserves consist solely of US dollar deposits and US Treasury bills with maturities of 90 days or less.\n")
				},
				func() error {
					return ev(b, "ev_reserve_attestation", "attestationReport", "DUSD monthly reserve attestation, June 2026", "evidence/reserve-attestation.txt",
						"Independent accountant's attestation, DUSD reserves as of 2026-06-30\n\nTokens outstanding: 4,000,000,000 DUSD\nReserve assets at fair value: 4,012,500,000 USD\nReserves exceed tokens outstanding.\n")
				},
				func() error {
					return ev(b, "ev_balances", "subledgerExtract", "Stablecoin balances by wallet at 2026-06-30", "evidence/stablecoin-balances.csv",
						"wallet,token,issuer,units,asof\ntreasury-main,DUSD,Demo Dollar Inc,1000000.00,2026-06-30\nops-hot,DUSD,Demo Dollar Inc,250000.00,2026-06-30\ndefi-vault,XEUR,XEUR Protocol,100000.00,2026-06-30\n")
				},
				func() error {
					return ev(b, "ev_xeur_docs", "protocolDocumentation", "XEUR protocol documentation excerpt", "evidence/xeur-protocol-docs.txt",
						"XEUR Protocol (excerpt)\n\nXEUR targets a 1 EUR peg through an algorithmic mint-and-burn mechanism against the protocol's governance token.\nThere is no issuer and no right to redeem XEUR for euros or any other asset.\n")
				},
				func() error {
					return fact(b, "s1_f_dusd_redemption", "Verified customers may redeem DUSD 1:1 for US dollars directly with the issuer on any business day.", "term", "direct redemption at par", "ev_dusd_terms", "4.1 Verified customers may redeem DUSD 1:1 for US dollars directly with the issuer on any business day.")
				},
				func() error {
					return fact(b, "s1_f_dusd_settlement", "DUSD redemptions settle within one business day.", "term", "T+1 settlement", "ev_dusd_terms", "4.2 Redemptions settle within one business day by wire to the customer's bank account.")
				},
				func() error {
					return fact(b, "s1_f_dusd_reserves", "DUSD reserves at 2026-06-30 exceeded tokens outstanding.", "term", "fully reserved", "ev_reserve_attestation", "Reserves exceed tokens outstanding.")
				},
				func() error {
					return fact(b, "s1_f_xeur_no_redemption", "XEUR has no issuer and no right of redemption.", "term", "no redemption right", "ev_xeur_docs", "There is no issuer and no right to redeem XEUR for euros or any other asset.")
				},
				func() error {
					return fact(b, "s1_f_balances", "DemoCo held 1,000,000 DUSD in treasury-main, 250,000 DUSD in ops-hot and 100,000 XEUR in defi-vault at 2026-06-30.", "quantity", m("DUSD", 1250000, "XEUR", 100000), "ev_balances", "treasury-main,DUSD,Demo Dollar Inc,1000000.00,2026-06-30")
				},
				func() error {
					return judgment(b, "s2_j_dusd_scope", "ASC 350-60 scope of DUSD",
						"Does DUSD meet the ASC 350-60-15-1 criteria?",
						"No. DUSD provides the holder an enforceable right to redeem for US dollars with the issuer, failing criterion (b). It is accounted for as a financial asset, not under ASC 350-60.",
						"ASC 350-60-15-1(b); issuer terms 4.1 and 4.2.",
						vwp.JudgmentAlternative{Conclusion: "Crypto asset at fair value under ASC 350-60", Notes: "Rejected: the enforceable redemption right is an asset-backed claim, not an intangible without rights."}, "medium")
				},
				func() error {
					return judgment(b, "s3_j_dusd_class", "Balance sheet classification of DUSD",
						"Is DUSD cash, a cash equivalent, or another financial asset?",
						"A financial asset: a receivable-like contractual right to receive 1 USD per token from the issuer on demand, carried at par with an ASC 326 credit loss assessment. Not cash (the issuer is not a depository institution and the token is not a demand deposit) and not a cash equivalent: DUSD has no maturity, so it cannot meet the ASC 230-10-20 short-maturity test however liquid it is — the AICPA Digital Assets Practice Aid view is that stablecoins are generally not cash equivalents. DemoCo's direct par redemption right settling T+1 and the attested reserves support par measurement and a nil allowance, not a cash-equivalent caption.",
						"ASC 230-10-20 (cash equivalents: short-term, highly liquid, readily convertible, so near maturity that value changes are insignificant — a stablecoin has no maturity); ASC 305-10-20 cash; AICPA Digital Assets Practice Aid stablecoin Q&A; terms 4.1, 4.2, 5.1; reserve attestation.",
						vwp.JudgmentAlternative{Conclusion: "Cash equivalent", Notes: "Rejected: the maturity leg of the definition is not met; a conclusion of cash equivalent would need to overcome the Practice Aid view and be owned by the reviewer."}, "high")
				},
				func() error {
					return judgment(b, "s3_j_xeur_class", "Balance sheet classification of XEUR",
						"How is XEUR classified given it has no issuer and no redemption right?",
						"ASC 350-60 crypto asset measured at fair value: it is an intangible on a distributed ledger, secured cryptographically, fungible, not created by DemoCo, and conveys no enforceable rights.",
						"ASC 350-60-15-1 criteria all met; protocol documentation confirms no redemption right.",
						vwp.JudgmentAlternative{Conclusion: "Financial asset at amortized cost", Notes: "Rejected: no contractual right to receive cash exists."}, "medium")
				},
				func() error {
					return register(b, validators.RegisterStablecoinHoldings, m(
						"description", "Stablecoin holdings classification register at 2026-06-30. Synthetic data.",
						"holdings", []any{
							m("id", "h_dusd_treasury", "token", "DUSD", "issuer", "Demo Dollar Inc", "chain", "ethereum", "wallet", "treasury-main", "units", 1000000, "asOf", "2026-06-30",
								"redemptionRight", "holderDirect", "redemptionTermsEvidence", []any{"ev_dusd_terms"}, "reserveEvidence", []any{"ev_reserve_attestation"},
								"classification", "financialAssetReceivable", "measurementBasis", "par (1 USD per DUSD) less ASC 326 allowance", "classificationJudgmentRef", "s3_j_dusd_class"),
							m("id", "h_dusd_ops", "token", "DUSD", "issuer", "Demo Dollar Inc", "chain", "ethereum", "wallet", "ops-hot", "units", 250000, "asOf", "2026-06-30",
								"redemptionRight", "holderDirect", "redemptionTermsEvidence", []any{"ev_dusd_terms"}, "reserveEvidence", []any{"ev_reserve_attestation"},
								"classification", "financialAssetReceivable", "measurementBasis", "par (1 USD per DUSD) less ASC 326 allowance", "classificationJudgmentRef", "s3_j_dusd_class"),
							m("id", "h_xeur_vault", "token", "XEUR", "issuer", "XEUR Protocol", "chain", "ethereum", "wallet", "defi-vault", "units", 100000, "asOf", "2026-06-30",
								"redemptionRight", "none", "classification", "cryptoAsset", "measurementBasis", "fair value (ASC 820 level 1)", "classificationJudgmentRef", "s3_j_xeur_class"),
						}))
				},
				func() error {
					return claim(b, claimSpec{id: "s1_c_terms", statement: "DUSD carries a direct, at-par, T+1 redemption right for verified customers backed by short-dated reserves; XEUR carries no redemption right.",
						conclusion: "Instrument terms established from issuer and protocol documentation.", refs: []string{"ASC 305-10-20"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s1_f_dusd_redemption", "s1_f_dusd_settlement", "s1_f_dusd_reserves", "s1_f_xeur_no_redemption"}, evidence: []string{"ev_dusd_terms", "ev_xeur_docs", "ev_reserve_attestation"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s2_c_scope", statement: "DUSD is outside ASC 350-60 because of its enforceable redemption right; XEUR is within ASC 350-60.",
						conclusion: "Scope split by redemption right.", refs: []string{"ASC 350-60-15-1"}, assertions: []string{"rightsAndObligations"},
						facts: []string{"s1_f_dusd_redemption", "s1_f_xeur_no_redemption"}, judgments: []string{"s2_j_dusd_scope", "s3_j_xeur_class"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s3_c_classification", statement: "All 1,250,000 DUSD is a financial asset (a receivable-like claim on the issuer at par), not cash or a cash equivalent; 100,000 XEUR is a crypto asset at fair value.",
						conclusion: "Classification consistent across wallets.", refs: []string{"ASC 230-10-20", "ASC 310-10-20", "ASC 350-60-35-1"}, assertions: []string{"presentationDisclosure", "valuationAllocation"},
						facts: []string{"s1_f_balances"}, judgments: []string{"s3_j_dusd_class", "s3_j_xeur_class"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s4_c_measurement", statement: "DUSD is carried at par as a receivable with a nil ASC 326 allowance given attested over-reserving and T+1 settlement; XEUR is remeasured at fair value through net income.",
						conclusion: "Measurement follows classification.", refs: []string{"ASC 326-20-30", "ASC 350-60-35-1"}, assertions: []string{"valuationAllocation"},
						facts: []string{"s1_f_dusd_reserves"}, judgments: []string{"s3_j_dusd_class"}})
				},
				func() error {
					return claim(b, claimSpec{id: "s5_c_presentation", statement: "DUSD is presented as a separate financial asset line (stablecoin receivable), outside cash and cash equivalents, and XEUR within crypto assets; the stablecoin concentration with one issuer is disclosed.",
						conclusion: "Presentation per ASC 210, ASC 230 and ASC 275.", refs: []string{"ASC 210-10-45", "ASC 230-10-45", "ASC 275-10-50"}, assertions: []string{"presentationDisclosure"},
						judgments: []string{"s3_j_dusd_class"}})
				},
			)
		},
	},
}
