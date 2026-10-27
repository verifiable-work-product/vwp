package validators

import (
	"fmt"
	"math"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// ASC 830 extension validator ids.
const (
	ValidatorASC830RemeasurementSchemaValid        = "asc830.remeasurement.schemaValid"
	ValidatorASC830RemeasurementRecompute          = "asc830.remeasurement.recompute"
	ValidatorASC830RemeasurementRateEvidence       = "asc830.remeasurement.rateEvidence"
	ValidatorASC830RemeasurementMonetaryClassified = "asc830.remeasurement.monetaryClassified"
	ValidatorASC830AccountingScheduleTieOut        = vwp.ValidatorASC830AccountingScheduleTieOut
)

// Data contract locations consumed by the ASC 830 validators.
const (
	fxRemeasurementType      = "foreignCurrencyRemeasurement"
	fxRemeasurementSchemaRef = "extensions/accounting-asc830/foreign-currency-remeasurement.schema.json"
	fxDefaultTol             = 0.01
)

// Account roles the ASC 830 executable schedule (kind fx_remeasurement)
// uses. Each remeasured item carries its own ledger role (its accountRole,
// bound to that item's account); the transaction gain or loss goes to one
// fixed role (ASC 830-20-35-1).
const (
	RoleFXGainLoss = "fx_gain_loss"
)

// ASC830 implements the vwp-accounting-asc830 extension validators over
// every remeasurement in the package (one per functional currency and date):
//
//   - recompute: a monetary item's remeasured amount is foreignAmount ×
//     spotRate (ASC 830-20-35-1); its gain or loss follows its balance side
//     — remeasured − carrying for a debit-balance item, carrying −
//     remeasured for a credit-balance one, so a positive figure is always a
//     gain; a non-monetary item stays at its carrying amount with no gain
//     or loss (830-10-45-18); the net gain or loss is the sum;
//   - rateEvidence: each monetary item's spot rate equals the rate the
//     cited fact states AS OF the remeasurement date;
//   - monetaryClassified: every item declares whether it is monetary, no
//     non-monetary item carries a remeasurement difference, and the
//     functional currency determination (830-10-45-2 to 45-6) rests on a
//     judgment in the package;
//   - accountingSchedule.tieOut: the executable schedule books one
//     fx_remeasurement group per monetary item with a difference, on the
//     remeasurement date: a gain debits the item's role and credits
//     fx_gain_loss, a loss the reverse.
func ASC830(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	all := []string{ValidatorASC830RemeasurementRecompute, ValidatorASC830RemeasurementRateEvidence, ValidatorASC830RemeasurementMonetaryClassified}

	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "functionalCurrency") + "|" + getString(in, "asOf") + "|" + getString(in, "scope")
	}
	hits, found := locateCalculations(pkg, add, ValidatorASC830RemeasurementSchemaValid, fxRemeasurementType, fxRemeasurementSchemaRef, scope)
	exp := scheduleExpectation{kind: accountingschedule.KindFXRemeasurement}
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid foreign currency remeasurement", all...)
		add(tieOutAccountingSchedule(pkg, ValidatorASC830AccountingScheduleTieOut, exp))
		return results
	}

	facts := factIndex(ctx)
	functionalCurrencyJudged := false
	for _, j := range ctx.Judgments {
		text := strings.ToLower(getString(j, "topic") + " " + getString(j, "question"))
		if strings.Contains(text, "functional currency") {
			functionalCurrencyJudged = true
		}
	}
	var recompute, rateErrs, classErrs []string
	for _, hit := range hits {
		pfx := hit.Path + ": "
		tol := tolerance(hit.Doc, "roundingTolerance", fxDefaultTol)
		in := getMap(hit.Doc, "inputs")
		out := getMap(hit.Doc, "outputs")
		functional := getString(in, "functionalCurrency")
		exp.sources++
		currency := getString(in, "currency")
		if currency == "" {
			currency = functional
		} else if currency != functional {
			recompute = append(recompute, fmt.Sprintf("%scurrency %s but the functional currency is %s: remeasurement books in the functional currency", pfx, currency, functional))
		}
		exp.currency(currency)
		asOf, okDate := parseWhen(getString(in, "asOf"))
		asOfIso, _ := isoDate(getString(in, "asOf"))
		if !okDate {
			exp.problem(pfx + "asOf is not a date")
		}
		if !functionalCurrencyJudged {
			classErrs = append(classErrs, pfx+"no judgment in the package concludes on the functional currency (ASC 830-10-45-2 to 45-6)")
		}

		rows, dups := indexByID(getMaps(out, "rows"), "id")
		for _, d := range dups {
			recompute = append(recompute, pfx+"duplicate output row for item "+d)
		}
		seen := map[string]bool{}
		var net float64
		for _, item := range getMaps(in, "items") {
			id := getString(item, "id")
			label := pfx + "item " + id
			if seen[id] {
				recompute = append(recompute, label+" listed twice")
				continue
			}
			seen[id] = true
			monetary, isBool := item["monetary"].(bool)
			if !isBool {
				classErrs = append(classErrs, label+": monetary must be declared true or false")
			}
			if item["currency"] == functional {
				classErrs = append(classErrs, label+": denominated in the functional currency, nothing to remeasure")
			}
			side := getString(item, "balanceSide")
			foreign := jsNumber(item["foreignAmount"])
			carrying := jsNumber(item["carryingAmount"])
			rate := jsNumber(item["spotRate"])
			wantRemeasured, wantGainLoss := carrying, 0.0
			if monetary {
				if math.IsNaN(rate) {
					classErrs = append(classErrs, label+": a monetary item needs spotRate and rateFact")
					rate = 0
				}
				wantRemeasured = foreign * rate
				wantGainLoss = wantRemeasured - carrying
				if side == "credit" {
					wantGainLoss = carrying - wantRemeasured
				}
				ref := getString(item, "rateFact")
				if msg := checkFactValue(facts, ref, rate, 1e-9, label+" spotRate", "rate", "spotRate", "spot", "price"); msg != "" {
					rateErrs = append(rateErrs, msg)
				} else if fd, ok := factDate(facts[ref]); !ok {
					rateErrs = append(rateErrs, fmt.Sprintf("%s: fact %q states no date for its rate (value needs a date)", label, ref))
				} else if okDate && !fd.Equal(asOf) {
					rateErrs = append(rateErrs, fmt.Sprintf("%s: fact %q is a rate as of %s, not the remeasurement date %s", label, ref, fd.Format("2006-01-02"), asOfIso))
				}
			}
			row, ok := rows[id]
			if !ok {
				recompute = append(recompute, label+": no output row")
				continue
			}
			remeasured := jsNumber(row["remeasured"])
			gainLoss := jsNumber(row["gainLoss"])
			if !nearTol(remeasured, wantRemeasured, tol) {
				if monetary {
					recompute = append(recompute, fmt.Sprintf("%s: remeasured %s but %s × %s = %s", label, num(remeasured), num(foreign), num(rate), num(wantRemeasured)))
				} else {
					recompute = append(recompute, fmt.Sprintf("%s: non-monetary items stay at carrying amount %s, not %s", label, num(carrying), num(remeasured)))
				}
			}
			if !nearTol(gainLoss, wantGainLoss, tol) {
				if monetary {
					recompute = append(recompute, fmt.Sprintf("%s: gainLoss %s but the %s-balance item moved from %s to %s, a %s of %s", label, num(gainLoss), side, num(carrying), num(wantRemeasured), gainWord(wantGainLoss), num(math.Abs(wantGainLoss))))
				} else {
					classErrs = append(classErrs, fmt.Sprintf("%s: a non-monetary item carries a remeasurement difference of %s", label, num(gainLoss)))
				}
			}
			net += wantGainLoss
			if monetary && okDate && math.Abs(wantGainLoss) > 0 {
				g := scheduleGroup{obligationId: id, date: asOfIso, label: label}
				amount := math.Abs(wantGainLoss)
				if wantGainLoss > 0 {
					g.postings = []schedulePosting{debit(amount, getString(item, "accountRole")), credit(amount, RoleFXGainLoss)}
				} else {
					g.postings = []schedulePosting{debit(amount, RoleFXGainLoss), credit(amount, getString(item, "accountRole"))}
				}
				exp.group(g)
			}
		}
		for id := range rows {
			if !seen[id] {
				recompute = append(recompute, pfx+"output row for item "+id+" which is not an input item")
			}
		}
		if declared := jsNumber(out["netGainLoss"]); !nearTol(declared, net, tol) {
			recompute = append(recompute, fmt.Sprintf("%snetGainLoss %s does not equal the sum of the item differences %s", pfx, num(declared), num(net)))
		}
	}

	emit := func(id, passMsg, failPrefix string, errs []string) {
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusFail, Message: failPrefix + joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusPass, Message: passMsg})
		}
	}
	emit(ValidatorASC830RemeasurementRecompute, fmt.Sprintf("%d remeasurement(s): every monetary item remeasures at its spot rate and the net gain or loss sums", len(hits)), "remeasurement does not recompute: ", recompute)
	emit(ValidatorASC830RemeasurementRateEvidence, "every spot rate equals the rate its fact states as of the remeasurement date", "rates disagree with their cited facts: ", rateErrs)
	emit(ValidatorASC830RemeasurementMonetaryClassified, "every item is classified monetary or non-monetary, non-monetary items carry no difference, and the functional currency is judged", "classification problems: ", classErrs)
	if len(recompute) > 0 {
		exp.problem("the remeasurement does not recompute")
	}
	add(tieOutAccountingSchedule(pkg, ValidatorASC830AccountingScheduleTieOut, exp))
	return results
}

func gainWord(v float64) string {
	if v >= 0 {
		return "gain"
	}
	return "loss"
}
