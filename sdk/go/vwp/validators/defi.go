package validators

import (
	"fmt"
	"strconv"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// DeFi positions extension validator ids.
const (
	ValidatorDeFiRegisterPresent          = "defi.register.present"
	ValidatorDeFiRegisterSchemaValid      = "defi.register.schemaValid"
	ValidatorDeFiRegisterJudgmentLinked   = "defi.register.judgmentLinked"
	ValidatorDeFiValuationSchemaValid     = "defi.valuation.schemaValid"
	ValidatorDeFiValuationRecompute       = "defi.valuation.recompute"
	ValidatorDeFiValuationPricingEvidence = "defi.valuation.pricingEvidence"
	ValidatorDeFiValuationTiesToRegister  = "defi.valuation.tiesToRegister"
)

// Data contract locations consumed by the DeFi positions validators.
const (
	defiPositionsPath          = "facts/defi-positions.json"
	defiPositionsSchemaRef     = "extensions/digital-assets-defi-positions/defi-positions.schema.json"
	defiValuationType          = "defiPositionValuation"
	defiValuationSchemaRef     = "extensions/digital-assets-defi-positions/defi-position-valuation.schema.json"
	defiDefaultRoundingTol     = 0.01
	defiDefaultUnitTol         = 1e-8
	defiUnitOfAccountReceipt   = "receiptToken"
	defiUnitOfAccountUnderlier = "underlyings"
)

// DeFiPositions implements the vwp-digital-assets-defi-positions extension
// validators over the defi-positions register and every position valuation
// in the package. The extension has no executable schedule: the package
// concludes on unit of account, derecognition and the subsequent
// measurement route per position; the remeasurement itself belongs to the
// ASC 350-60, ASC 350-30 or financial-asset package that route names.
//
//   - register.judgmentLinked: every position links a resolvable treatment
//     judgment and a resolvable custody evidence reference, and a position
//     whose contributed assets are derecognized does not also claim to
//     retain control of them;
//   - valuation.recompute: each position's fair value is the sum of its
//     components at units × unit price, and the total is their sum;
//   - valuation.pricingEvidence: every component's price fact states that
//     price as of the valuation date;
//   - valuation.tiesToRegister: every valued position is in the register and,
//     where the unit of account is the underlyings, its components carry the
//     register's underlying units; every register position is valued.
func DeFiPositions(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	registerDoc, present := readJSONValue(pkg, defiPositionsPath)
	registerMap := asMap(registerDoc)
	positions := getMaps(registerMap, "positions")
	if !present || len(positions) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorDeFiRegisterPresent, Status: vwp.StatusFail, Message: defiPositionsPath + " missing or contains no positions"})
		notRun(add, "requires the defi-positions register", ValidatorDeFiRegisterSchemaValid, ValidatorDeFiRegisterJudgmentLinked)
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorDeFiRegisterPresent, Status: vwp.StatusPass, Message: strconv.Itoa(len(positions)) + " positions in register"})
	}
	registerValid := false
	if present && len(positions) > 0 {
		errs, err := schema.ValidateExtension(defiPositionsSchemaRef, registerDoc)
		if err != nil {
			errs = []string{err.Error()}
		}
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorDeFiRegisterSchemaValid, Status: vwp.StatusFail, Message: joinCapped(errs, 5)})
			notRun(add, "requires a schema-valid register", ValidatorDeFiRegisterJudgmentLinked)
		} else {
			registerValid = true
			add(vwp.ValidationResult{ValidatorId: ValidatorDeFiRegisterSchemaValid, Status: vwp.StatusPass})
		}
	}

	facts := factIndex(ctx)
	judgmentIds := idSet(ctx.Judgments)
	evidenceIds := idSet(ctx.Evidence)
	byId := map[string]map[string]any{}
	if registerValid {
		var problems []string
		for _, p := range positions {
			id := getString(p, "id")
			if _, dup := byId[id]; dup {
				problems = append(problems, id+" listed twice")
				continue
			}
			byId[id] = p
			if ref := getString(p, "treatmentJudgmentRef"); ref == "" || !judgmentIds[ref] {
				problems = append(problems, id+" (treatmentJudgmentRef "+strconv.Quote(ref)+" does not resolve)")
			}
			if ref := getString(p, "custodyEvidenceRef"); ref == "" || (!evidenceIds[ref] && facts[ref] == nil) {
				problems = append(problems, id+" (custodyEvidenceRef "+strconv.Quote(ref)+" is neither evidence nor a fact in the package)")
			}
			derecognized, _ := p["derecognized"].(bool)
			retained, _ := p["controlRetained"].(bool)
			if derecognized && retained {
				problems = append(problems, id+" derecognizes assets it says it still controls")
			}
		}
		failList(add, ValidatorDeFiRegisterJudgmentLinked,
			"every position links a resolvable treatment judgment and custody evidence, and derecognition agrees with control",
			"positions with unresolved support or inconsistent control conclusions: ", problems)
	}

	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "asOf") + "|" + getString(in, "currency")
	}
	hits, found := locateCalculations(pkg, add, ValidatorDeFiValuationSchemaValid, defiValuationType, defiValuationSchemaRef, scope)
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid position valuation", ValidatorDeFiValuationRecompute, ValidatorDeFiValuationPricingEvidence, ValidatorDeFiValuationTiesToRegister)
		return results
	}

	var wrong, price, tie []string
	valued := map[string]bool{}
	for _, hit := range hits {
		pfx := hit.Path + ": "
		tol := tolerance(hit.Doc, "roundingTolerance", defiDefaultRoundingTol)
		unitTol := tolerance(hit.Doc, "unitTolerance", defiDefaultUnitTol)
		in := getMap(hit.Doc, "inputs")
		out := getMap(hit.Doc, "outputs")
		asOf, okAsOf := parseWhen(getString(in, "asOf"))
		if !okAsOf {
			price = append(price, pfx+"asOf is not a parseable date")
		}
		if registerValid && getString(registerMap, "currency") != getString(in, "currency") {
			tie = append(tie, fmt.Sprintf("%scurrency %s but the register is in %s", pfx, getString(in, "currency"), getString(registerMap, "currency")))
		}
		if registerValid && getString(registerMap, "asOf") != getString(in, "asOf") {
			tie = append(tie, fmt.Sprintf("%svalued as of %s but the register is as of %s", pfx, getString(in, "asOf"), getString(registerMap, "asOf")))
		}
		outBy, dups := indexByID(getMaps(out, "positions"), "id")
		for _, d := range dups {
			wrong = append(wrong, pfx+"duplicate output row for position "+d)
		}
		seen := map[string]bool{}
		var total float64
		for _, pos := range getMaps(in, "positions") {
			id := getString(pos, "id")
			label := pfx + id
			if seen[id] {
				wrong = append(wrong, label+" valued twice")
				continue
			}
			seen[id] = true
			var fv float64
			unitsByAsset := map[string]float64{}
			for i, c := range getMaps(pos, "components") {
				units, unitPrice := jsNumber(c["units"]), jsNumber(c["unitPrice"])
				fv += units * unitPrice
				unitsByAsset[getString(c, "asset")] += units
				clabel := fmt.Sprintf("%s component %d (%s)", label, i+1, getString(c, "asset"))
				ref := getString(c, "priceFact")
				if msg := checkFactValue(facts, ref, unitPrice, tol, clabel+" unitPrice", "unitPrice", "price", "amount", "close"); msg != "" {
					price = append(price, msg)
				} else if fd, ok := factDate(facts[ref]); !ok {
					price = append(price, fmt.Sprintf("%s: fact %q states no date for its price (value needs a date)", clabel, ref))
				} else if okAsOf && !fd.Equal(asOf) {
					price = append(price, fmt.Sprintf("%s: fact %q is a price as of %s, not the valuation date %s", clabel, ref, fd.Format("2006-01-02"), getString(in, "asOf")))
				}
			}
			total += fv
			if row, ok := outBy[id]; !ok {
				wrong = append(wrong, label+" has no output row")
			} else if got := jsNumber(row["fairValue"]); !nearTol(got, fv, tol) {
				wrong = append(wrong, fmt.Sprintf("%s fairValue %s does not equal Σ units × price = %s", label, num(got), num(fv)))
			}
			// Register tie: the position exists; when the underlyings are the
			// unit of account, the components carry the register's units.
			reg, ok := byId[id]
			switch {
			case !registerValid:
			case !ok:
				tie = append(tie, label+" is not in the register")
			default:
				valued[id] = true
				if getString(reg, "unitOfAccount") == defiUnitOfAccountUnderlier {
					regUnits := map[string]float64{}
					for _, u := range getMaps(reg, "underlyings") {
						regUnits[getString(u, "asset")] += jsNumber(u["units"])
					}
					for asset, units := range regUnits {
						if !nearTol(unitsByAsset[asset], units, unitTol) {
							tie = append(tie, fmt.Sprintf("%s values %s %s but the register holds %s", label, num(unitsByAsset[asset]), asset, num(units)))
						}
					}
					for asset := range unitsByAsset {
						if _, known := regUnits[asset]; !known {
							tie = append(tie, fmt.Sprintf("%s values %s which the register does not list as an underlying", label, asset))
						}
					}
				} else if receipt := getString(reg, "receiptToken"); receipt != "" {
					if !nearTol(unitsByAsset[receipt], jsNumber(reg["receiptUnits"]), unitTol) {
						tie = append(tie, fmt.Sprintf("%s values %s %s but the register holds %s receipt units", label, num(unitsByAsset[receipt]), receipt, num(jsNumber(reg["receiptUnits"]))))
					}
				}
			}
		}
		for id := range outBy {
			if !seen[id] {
				wrong = append(wrong, pfx+"output row for position "+id+" which is not valued in this calculation")
			}
		}
		if got := jsNumber(out["totalFairValue"]); !nearTol(got, total, tol) {
			wrong = append(wrong, fmt.Sprintf("%stotalFairValue %s does not equal the sum of positions %s", pfx, num(got), num(total)))
		}
	}
	if registerValid {
		for _, p := range positions {
			if id := getString(p, "id"); !valued[id] {
				tie = append(tie, "register position "+id+" is not valued")
			}
		}
	}
	failList(add, ValidatorDeFiValuationRecompute,
		fmt.Sprintf("%d valuation(s): every position is Σ units × price and the total is their sum", len(hits)),
		"valuation does not recompute: ", wrong)
	failList(add, ValidatorDeFiValuationPricingEvidence,
		"every component is priced by a fact stating that price as of the valuation date",
		"pricing is not evidenced as of the valuation date: ", price)
	if !registerValid {
		notRun(add, "requires a schema-valid defi-positions register", ValidatorDeFiValuationTiesToRegister)
	} else {
		failList(add, ValidatorDeFiValuationTiesToRegister,
			"every valued position is in the register with its units, and every register position is valued",
			"valuation does not tie to the register: ", tie)
	}
	return results
}
