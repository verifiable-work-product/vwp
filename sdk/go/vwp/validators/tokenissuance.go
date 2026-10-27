package validators

import (
	"fmt"
	"strconv"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// Token issuance extension validator ids.
const (
	ValidatorTokenIssuanceRegisterPresent          = "tokenIssuance.register.present"
	ValidatorTokenIssuanceRegisterSchemaValid      = "tokenIssuance.register.schemaValid"
	ValidatorTokenIssuanceRegisterJudgmentLinked   = "tokenIssuance.register.judgmentLinked"
	ValidatorTokenIssuanceAllocationSchemaValid    = "tokenIssuance.allocation.schemaValid"
	ValidatorTokenIssuanceAllocationRecompute      = "tokenIssuance.allocation.recompute"
	ValidatorTokenIssuanceAllocationTiesToRegister = "tokenIssuance.allocation.tiesToRegister"
	ValidatorTokenIssuanceAccountingScheduleTieOut = vwp.ValidatorTokenIssuanceAccountingScheduleTieOut
)

// Data contract locations consumed by the token issuance validators.
const (
	issuanceInstrumentsPath         = "facts/issuance-instruments.json"
	issuanceInstrumentsSchemaRef    = "extensions/accounting-token-issuance/issuance-instruments.schema.json"
	proceedsAllocationType          = "tokenIssuanceProceedsAllocation"
	proceedsAllocationSchemaRef     = "extensions/accounting-token-issuance/token-issuance-proceeds-allocation.schema.json"
	tokenIssuanceDefaultRoundingTol = 0.01
	tokenIssuanceClassificationNone = "none"
)

// Account roles the token_issuance executable schedule uses: the
// consideration received (cash or the asset taken in) against the
// balance-sheet component each classification produces.
const (
	RoleConsiderationReceived = "consideration_received"
	RoleTokenLiability        = "token_liability"
	RoleEquityTokenProceeds   = "equity_token_proceeds"
	RoleDeferredRevenue       = "deferred_revenue"
	RoleDerivativeLiability   = "derivative_liability"
)

// classificationRoles maps an allocation component's classification to
// the role its credit is booked under.
var classificationRoles = map[string]string{
	"liability":           RoleTokenLiability,
	"equity":              RoleEquityTokenProceeds,
	"deferredRevenue":     RoleDeferredRevenue,
	"derivativeLiability": RoleDerivativeLiability,
}

// TokenIssuance implements the vwp-accounting-token-issuance extension
// validators over the issuance-instruments register and every proceeds
// allocation in the package:
//
//   - register.judgmentLinked: every instrument links a resolvable
//     classification judgment (whether the instrument is a liability, equity,
//     a contract liability under ASC 606 or a derivative is never decided
//     here — only that a human-owned judgment exists for it), and no
//     instrument that received consideration is left classified "none";
//   - allocation.recompute: per instrument the components sum to its
//     proceeds, the by-classification totals and the grand total are the
//     sums of the components, no instrument appears twice;
//   - allocation.tiesToRegister: every allocated instrument exists in the
//     register with the same proceeds, every register instrument with
//     proceeds is allocated, and each register instrument's proceeds equal
//     the value of the proceeds fact it cites;
//   - accountingSchedule.tieOut: the executable schedule books one
//     token_issuance group per instrument with proceeds, on its issuance
//     date: consideration_received debited for the proceeds and, per
//     component, the classification's role credited for the component
//     amount — and nothing else. A register with no proceeds needs no
//     schedule.
func TokenIssuance(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	registerDoc, present := readJSONValue(pkg, issuanceInstrumentsPath)
	registerMap := asMap(registerDoc)
	instruments := getMaps(registerMap, "instruments")
	if !present || len(instruments) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorTokenIssuanceRegisterPresent, Status: vwp.StatusFail, Message: issuanceInstrumentsPath + " missing or contains no instruments"})
		notRun(add, "requires the issuance-instruments register", ValidatorTokenIssuanceRegisterSchemaValid, ValidatorTokenIssuanceRegisterJudgmentLinked)
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorTokenIssuanceRegisterPresent, Status: vwp.StatusPass, Message: strconv.Itoa(len(instruments)) + " instruments in register"})
	}
	registerValid := false
	if present && len(instruments) > 0 {
		errs, err := schema.ValidateExtension(issuanceInstrumentsSchemaRef, registerDoc)
		if err != nil {
			errs = []string{err.Error()}
		}
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: ValidatorTokenIssuanceRegisterSchemaValid, Status: vwp.StatusFail, Message: joinCapped(errs, 5)})
			notRun(add, "requires a schema-valid register", ValidatorTokenIssuanceRegisterJudgmentLinked)
		} else {
			registerValid = true
			add(vwp.ValidationResult{ValidatorId: ValidatorTokenIssuanceRegisterSchemaValid, Status: vwp.StatusPass})
		}
	}

	facts := factIndex(ctx)
	judgmentIds := idSet(ctx.Judgments)
	byId := map[string]map[string]any{}
	if registerValid {
		var problems []string
		for _, inst := range instruments {
			id := getString(inst, "id")
			if _, dup := byId[id]; dup {
				problems = append(problems, id+" listed twice")
				continue
			}
			byId[id] = inst
			if ref := getString(inst, "classificationJudgmentRef"); ref == "" || !judgmentIds[ref] {
				problems = append(problems, id+" (classificationJudgmentRef "+strconv.Quote(ref)+" does not resolve)")
			}
			if getString(inst, "classification") == tokenIssuanceClassificationNone && jsNumber(inst["proceeds"]) > 0 {
				problems = append(problems, id+" received "+num(jsNumber(inst["proceeds"]))+" but is classified none")
			}
		}
		failList(add, ValidatorTokenIssuanceRegisterJudgmentLinked,
			"every instrument links a resolvable classification judgment and every instrument with proceeds is classified",
			"instruments without a resolvable judgment or classification: ", problems)
	}

	// Proceeds allocation: one per valuation date and currency.
	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "asOf") + "|" + getString(in, "currency")
	}
	hits, found := locateCalculations(pkg, add, ValidatorTokenIssuanceAllocationSchemaValid, proceedsAllocationType, proceedsAllocationSchemaRef, scope)
	exp := scheduleExpectation{kind: accountingschedule.KindTokenIssuance}
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid proceeds allocation", ValidatorTokenIssuanceAllocationRecompute, ValidatorTokenIssuanceAllocationTiesToRegister)
		add(tieOutAccountingSchedule(pkg, ValidatorTokenIssuanceAccountingScheduleTieOut, exp))
		return results
	}

	var wrong, tie []string
	allocated := map[string]bool{}
	for _, hit := range hits {
		pfx := hit.Path + ": "
		tol := tolerance(hit.Doc, "roundingTolerance", tokenIssuanceDefaultRoundingTol)
		in := getMap(hit.Doc, "inputs")
		out := getMap(hit.Doc, "outputs")
		currency := getString(in, "currency")
		exp.sources++
		exp.currency(currency)
		if registerValid && getString(registerMap, "currency") != currency {
			tie = append(tie, fmt.Sprintf("%scurrency %s but the register is in %s", pfx, currency, getString(registerMap, "currency")))
		}
		sums := map[string]float64{}
		var total float64
		seen := map[string]bool{}
		for _, inst := range getMaps(in, "instruments") {
			id := getString(inst, "id")
			label := pfx + id
			if seen[id] {
				wrong = append(wrong, label+" allocated twice")
				continue
			}
			seen[id] = true
			proceeds := jsNumber(inst["proceeds"])
			total += proceeds
			var componentSum float64
			group := scheduleGroup{obligationId: id, label: label}
			for _, c := range getMaps(inst, "components") {
				class := getString(c, "classification")
				amount := jsNumber(c["amount"])
				componentSum += amount
				sums[class] += amount
				if role, ok := classificationRoles[class]; ok {
					group.postings = append(group.postings, credit(amount, role))
				} else {
					wrong = append(wrong, fmt.Sprintf("%s component classification %q is not one the schedule can book", label, class))
				}
			}
			if !nearTol(componentSum, proceeds, tol) {
				wrong = append(wrong, fmt.Sprintf("%s components sum to %s, proceeds %s", label, num(componentSum), num(proceeds)))
			}
			// Register tie: existence, proceeds, and the proceeds fact.
			reg, ok := byId[id]
			switch {
			case !registerValid:
			case !ok:
				tie = append(tie, label+" is not in the register")
			default:
				allocated[id] = true
				if regProceeds := jsNumber(reg["proceeds"]); !nearTol(regProceeds, proceeds, tol) {
					tie = append(tie, fmt.Sprintf("%s allocates %s but the register records proceeds of %s", label, num(proceeds), num(regProceeds)))
				}
				if proceeds > 0 {
					ref := getString(reg, "proceedsFact")
					if ref == "" {
						tie = append(tie, label+" received proceeds but the register cites no proceedsFact")
					} else if msg := checkFactValue(facts, ref, proceeds, tol, label+" proceeds", "proceeds", "amount", "consideration"); msg != "" {
						tie = append(tie, msg)
					}
				}
				issued, okDate := isoDate(getString(reg, "issuanceDate"))
				if !okDate {
					exp.problem(label + ": register issuanceDate is not a date")
				} else {
					group.date = issued
				}
			}
			if proceeds > 0 {
				group.postings = append([]schedulePosting{debit(proceeds, RoleConsiderationReceived)}, group.postings...)
			}
			if group.date != "" {
				exp.group(group)
			}
		}
		by := getMap(out, "byClassification")
		for _, class := range []string{"liability", "equity", "deferredRevenue", "derivativeLiability"} {
			if got := jsNumber(by[class]); !nearTol(got, sums[class], tol) {
				wrong = append(wrong, fmt.Sprintf("%sbyClassification.%s %s does not equal the components' sum %s", pfx, class, num(got), num(sums[class])))
			}
		}
		if got := jsNumber(out["total"]); !nearTol(got, total, tol) {
			wrong = append(wrong, fmt.Sprintf("%stotal %s does not equal the instruments' proceeds %s", pfx, num(got), num(total)))
		}
	}
	if registerValid {
		for _, inst := range instruments {
			id := getString(inst, "id")
			if jsNumber(inst["proceeds"]) > 0 && !allocated[id] {
				tie = append(tie, "register instrument "+id+" received proceeds but is not allocated")
			}
		}
	}
	failList(add, ValidatorTokenIssuanceAllocationRecompute,
		fmt.Sprintf("%d allocation(s): components sum to proceeds, classification totals and the total recompute", len(hits)),
		"allocation does not recompute: ", wrong)
	if !registerValid {
		notRun(add, "requires a schema-valid issuance-instruments register", ValidatorTokenIssuanceAllocationTiesToRegister)
	} else {
		failList(add, ValidatorTokenIssuanceAllocationTiesToRegister,
			"every allocated instrument is in the register with the same proceeds, every register instrument with proceeds is allocated, and proceeds equal their cited facts",
			"allocation does not tie to the register: ", tie)
	}
	add(tieOutAccountingSchedule(pkg, ValidatorTokenIssuanceAccountingScheduleTieOut, exp))
	return results
}
