package validators

import (
	"fmt"
	"strconv"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// Custody and safeguarding extension validator ids.
const (
	ValidatorCustodyRegisterPresent            = "custody.register.present"
	ValidatorCustodyRegisterSchemaValid        = "custody.register.schemaValid"
	ValidatorCustodyRegisterJudgmentLinked     = "custody.register.judgmentLinked"
	ValidatorCustodyRegisterValuationRecompute = "custody.register.valuationRecompute"
	ValidatorCustodyRegisterControlEvidence    = "custody.register.controlEvidence"
	ValidatorCustodyAccountingScheduleTieOut   = vwp.ValidatorCustodyAccountingScheduleTieOut
)

// Data contract locations consumed by the custody validators.
const (
	custodiedAssetsPath      = "facts/custodied-assets.json"
	custodiedAssetsSchemaRef = "extensions/digital-assets-custody-safeguarding/custodied-assets.schema.json"
	custodyDefaultTol        = 0.01
)

// Account roles the safeguarding_liability executable schedule uses: the
// ASC 450-20 loss accrued for a custodied population against the
// liability to make customers whole.
const (
	RoleSafeguardingLoss      = "safeguarding_loss"
	RoleSafeguardingLiability = "safeguarding_liability"
)

// CustodySafeguarding implements the vwp-digital-assets-custody-safeguarding
// extension validators over the custodied-assets register. SAB 122 (2025)
// rescinded SAB 121's safeguarding liability and asset, so an entity that
// safeguards crypto assets for others recognizes a liability only when a
// loss from that risk is probable and reasonably estimable (ASC 450-20-25-2)
// and otherwise discloses. The validators prove the register, never the
// conclusion:
//
//   - judgmentLinked: every holding links a resolvable judgment, and a
//     recognized liability exists exactly when the loss is declared
//     probable and estimable;
//   - valuationRecompute: each holding's fair value is units × unit price
//     and the cited price fact states that price AS OF the register date;
//   - controlEvidence: every holding the entity is obligated to return
//     cites control evidence (signed message, test transaction, sub-
//     custodian statement) that resolves in the package;
//   - accountingSchedule.tieOut: the executable schedule books one
//     safeguarding_liability group per holding with a recognized liability
//     on the register date, and nothing else.
func CustodySafeguarding(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	all := []string{ValidatorCustodyRegisterJudgmentLinked, ValidatorCustodyRegisterValuationRecompute, ValidatorCustodyRegisterControlEvidence}

	registerDoc, present := readJSONValue(pkg, custodiedAssetsPath)
	doc := asMap(registerDoc)
	holdings := getMaps(doc, "holdings")
	if !present || len(holdings) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorCustodyRegisterPresent, Status: vwp.StatusFail, Message: custodiedAssetsPath + " missing or contains no holdings"})
		notRun(add, "requires "+custodiedAssetsPath, all...)
		add(tieOutAccountingSchedule(pkg, ValidatorCustodyAccountingScheduleTieOut, scheduleExpectation{kind: accountingschedule.KindSafeguardingLiability}))
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorCustodyRegisterPresent, Status: vwp.StatusPass, Message: strconv.Itoa(len(holdings)) + " custodied populations in register"})
	errs, err := schema.ValidateExtension(custodiedAssetsSchemaRef, registerDoc)
	if err != nil {
		errs = []string{err.Error()}
	}
	if len(errs) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorCustodyRegisterSchemaValid, Status: vwp.StatusFail, Message: joinCapped(errs, 5) + " (remaining custody validators not run)"})
		notRun(add, "requires a schema-valid "+custodiedAssetsPath, all...)
		add(tieOutAccountingSchedule(pkg, ValidatorCustodyAccountingScheduleTieOut, scheduleExpectation{kind: accountingschedule.KindSafeguardingLiability}))
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorCustodyRegisterSchemaValid, Status: vwp.StatusPass})

	facts := factIndex(ctx)
	judgmentIds := idSet(ctx.Judgments)
	evidenceIds := idSet(ctx.Evidence)
	tol := tolerance(doc, "roundingTolerance", custodyDefaultTol)
	asOf, okAsOf := parseWhen(getString(doc, "asOf"))
	asOfISO, _ := isoDate(getString(doc, "asOf"))
	exp := scheduleExpectation{kind: accountingschedule.KindSafeguardingLiability, sources: 1}
	exp.currency(getString(doc, "currency"))
	if !okAsOf {
		exp.problem(custodiedAssetsPath + ": asOf is not a date")
	}

	var unjudged, inconsistent, valuation, control []string
	seen := map[string]bool{}
	var totalFV float64
	for _, h := range holdings {
		id := getString(h, "id")
		if seen[id] {
			valuation = append(valuation, "holding id "+id+" appears twice")
			continue
		}
		seen[id] = true

		// Judgment and the ASC 450-20-25-2 consistency of the accrual.
		if ref := getString(h, "judgmentRef"); ref == "" || !judgmentIds[ref] {
			unjudged = append(unjudged, id)
		}
		probable, _ := h["lossProbable"].(bool)
		estimable, _ := h["lossEstimable"].(bool)
		liability := jsNumber(h["recognizedLiability"])
		switch {
		case liability > tol && (!probable || !estimable):
			inconsistent = append(inconsistent, fmt.Sprintf("%s recognizes %s but the loss is not declared both probable and estimable (ASC 450-20-25-2)", id, num(liability)))
		case probable && estimable && liability <= tol:
			inconsistent = append(inconsistent, fmt.Sprintf("%s declares the loss probable and estimable but recognizes no liability (ASC 450-20-25-2 requires the accrual)", id))
		}

		// Fair value for disclosure: units × price, price stated as of asOf.
		units := jsNumber(h["units"])
		price := jsNumber(h["unitPrice"])
		fv := jsNumber(h["fairValue"])
		if !nearTol(fv, units*price, tol) {
			valuation = append(valuation, fmt.Sprintf("%s fairValue %s does not equal units %s × unitPrice %s = %s", id, num(fv), num(units), num(price), num(units*price)))
		}
		totalFV += units * price
		ref := getString(h, "priceFact")
		if msg := checkFactValue(facts, ref, price, tol, id+" unitPrice", "unitPrice", "price", "amount", "close"); msg != "" {
			valuation = append(valuation, msg)
		} else if fd, ok := factDate(facts[ref]); !ok {
			valuation = append(valuation, fmt.Sprintf("%s: fact %q states no date for its price (value needs a date)", id, ref))
		} else if okAsOf && !fd.Equal(asOf) {
			valuation = append(valuation, fmt.Sprintf("%s: fact %q is a price as of %s, not the register date %s", id, ref, fd.Format("2006-01-02"), getString(doc, "asOf")))
		}

		// Control over the keys, for every population the entity must return.
		obligated, _ := h["obligationToReturn"].(bool)
		if cref := strings.TrimSpace(getString(h, "controlEvidenceRef")); cref != "" {
			if _, isFact := facts[cref]; !isFact && !evidenceIds[cref] {
				control = append(control, fmt.Sprintf("%s cites controlEvidenceRef %q which is neither evidence nor a fact in the package", id, cref))
			}
		} else if obligated {
			control = append(control, id+" is obligated to return the assets but cites no controlEvidenceRef")
		}
		if getString(h, "holderRole") == "agent" && obligated {
			control = append(control, id+" is held as agent yet declares an obligation to return: an agent with a return obligation is a custodian")
		}

		if liability > 0 {
			exp.group(scheduleGroup{obligationId: id, date: asOfISO, label: custodiedAssetsPath + " " + id,
				postings: []schedulePosting{debit(liability, RoleSafeguardingLoss), credit(liability, RoleSafeguardingLiability)}})
		}
	}
	sortStrings(unjudged)
	switch {
	case len(unjudged) > 0:
		add(vwp.ValidationResult{ValidatorId: ValidatorCustodyRegisterJudgmentLinked, Status: vwp.StatusFail,
			Message: "holdings without a resolvable judgmentRef: " + joinCapped(unjudged, 8), Refs: unjudged})
	case len(inconsistent) > 0:
		add(vwp.ValidationResult{ValidatorId: ValidatorCustodyRegisterJudgmentLinked, Status: vwp.StatusFail,
			Message: "loss-contingency assessment and accrual disagree: " + joinCapped(inconsistent, 5)})
	default:
		add(vwp.ValidationResult{ValidatorId: ValidatorCustodyRegisterJudgmentLinked, Status: vwp.StatusPass,
			Message: "every holding links a judgment and accrues a liability exactly when its loss is probable and estimable"})
	}
	failList(add, ValidatorCustodyRegisterValuationRecompute,
		fmt.Sprintf("every holding's fair value is units × a price its fact states as of %s; custodied assets total %s", getString(doc, "asOf"), num(totalFV)),
		"custodied asset valuation does not recompute: ", valuation)
	failList(add, ValidatorCustodyRegisterControlEvidence,
		"every population the entity must return cites resolvable control evidence",
		"control evidence problems: ", control)
	add(tieOutAccountingSchedule(pkg, ValidatorCustodyAccountingScheduleTieOut, exp))
	return results
}
