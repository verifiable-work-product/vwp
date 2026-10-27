package validators

import (
	"fmt"
	"sort"
	"strings"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// Mining revenue extension validator ids.
const (
	ValidatorMiningRewardsSchemaValid       = "mining.rewards.schemaValid"
	ValidatorMiningRewardsRecompute         = "mining.rewards.recompute"
	ValidatorMiningRewardsChainTieOut       = "mining.rewards.chainTieOut"
	ValidatorMiningRewardsPricingEvidence   = "mining.rewards.pricingEvidence"
	ValidatorMiningRewardsPopulation        = "mining.rewards.population"
	ValidatorMiningAccountingScheduleTieOut = vwp.ValidatorMiningAccountingScheduleTieOut
)

// Data contract locations consumed by the mining revenue validators.
const (
	miningScheduleType      = "miningRevenueSchedule"
	miningScheduleSchemaRef = "extensions/digital-assets-mining-revenue/mining-revenue-schedule.schema.json"
	miningDefaultUnitTol    = 1e-8
)

// Account roles the mining revenue executable schedule uses (with
// RoleCryptoAsset for a reward received on or before recognition).
const (
	RoleMiningRevenue          = "mining_revenue"
	RoleMiningRewardReceivable = "mining_reward_receivable"
)

// MiningRevenue implements the vwp-digital-assets-mining-revenue extension
// validators over every mining revenue schedule in the package (one per
// asset, arrangement and period), with the same discipline as staking
// rewards — a block reward or pool payout is noncash consideration for the
// mining service, measured at the price of the date the timing judgment
// fixes:
//
//   - recompute: each row's fair value is units × unit price, the fee part
//     never exceeds the units, and the totals sum;
//   - pricingEvidence: each row's measurement date is the date the declared
//     pricingDateBasis fixes (inception, recognition or receipt), and the
//     cited price fact states that price AS OF that date;
//   - chainTieOut: recognized units reconcile to the chain-observed receipts
//     (coinbase outputs or pool payouts) the cited fact states, through the
//     declared unreceived opening/closing balances and other reconciling
//     items (pool fees netted at source, orphaned blocks, immature coinbase);
//   - population: every reward's recognition date lies in the schedule's
//     period, references (block or payout ids) are unique across all
//     schedules of an asset and arrangement, and no two schedules of the
//     same asset and arrangement overlap in time;
//   - accountingSchedule.tieOut: the executable schedule books one
//     mining_revenue group per reward on its recognition date — the crypto
//     asset (received) or a receivable (received later or not yet) against
//     mining_revenue — and a receipt transfer where receipt follows
//     recognition.
func MiningRevenue(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }
	all := []string{ValidatorMiningRewardsRecompute, ValidatorMiningRewardsChainTieOut, ValidatorMiningRewardsPricingEvidence, ValidatorMiningRewardsPopulation}

	scope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "asset") + "|" + getString(in, "arrangement") + "|" + getString(in, "periodStart") + "|" + getString(in, "periodEnd")
	}
	hits, found := locateCalculations(pkg, add, ValidatorMiningRewardsSchemaValid, miningScheduleType, miningScheduleSchemaRef, scope)
	if !found || len(hits) == 0 {
		notRun(add, "no schema-valid mining revenue schedule", all...)
		add(tieOutAccountingSchedule(pkg, ValidatorMiningAccountingScheduleTieOut, scheduleExpectation{kind: accountingschedule.KindMiningRevenue}))
		return results
	}

	facts := factIndex(ctx)
	var recomputeErrs, chainErrs, priceErrs, popErrs []string
	refsSeen := map[string]string{} // asset|arrangement|reference → path
	var periods []stakingPeriod
	for _, hit := range hits {
		tol := tolerance(hit.Doc, "roundingTolerance", 0.01)
		unitTol := tolerance(hit.Doc, "unitTolerance", miningDefaultUnitTol)
		inputs := getMap(hit.Doc, "inputs")
		outputs := getMap(hit.Doc, "outputs")
		rewards := getMaps(inputs, "rewards")
		rows := getMaps(outputs, "rows")
		pfx := hit.Path + ": "
		basis := getString(inputs, "pricingDateBasis")
		key := getString(inputs, "asset") + "|" + getString(inputs, "arrangement")

		start, okS := parseWhen(getString(inputs, "periodStart"))
		end, okE := parseWhen(getString(inputs, "periodEnd"))
		if !okS || !okE || end.Before(start) {
			popErrs = append(popErrs, pfx+"periodStart/periodEnd are not an ordered date pair")
		} else {
			periods = append(periods, stakingPeriod{path: hit.Path, key: key, start: start, end: end})
		}

		var units, fairValue float64
		if len(rows) != len(rewards) {
			recomputeErrs = append(recomputeErrs, fmt.Sprintf("%s%d reward inputs but %d output rows", pfx, len(rewards), len(rows)))
		}
		for i, r := range rewards {
			label := fmt.Sprintf("%srow %d (%s)", pfx, i+1, getString(r, "date"))
			u := jsNumber(r["units"])
			price := jsNumber(r["unitPrice"])
			fv := u * price
			units += u
			fairValue += fv
			if i < len(rows) {
				row := rows[i]
				if getString(row, "date") != getString(r, "date") || !nearTol(jsNumber(row["units"]), u, unitTol) || !nearTol(jsNumber(row["fairValue"]), fv, tol) {
					recomputeErrs = append(recomputeErrs, fmt.Sprintf("%s: expected %s units / %s, declared %s units / %s", label, num(u), num(fv), num(jsNumber(row["units"])), num(jsNumber(row["fairValue"]))))
				}
			}
			// The fee part of a block reward is part of the units, never more.
			if raw, ok := r["feeUnits"]; ok {
				if fee := jsNumber(raw); fee > u+unitTol {
					recomputeErrs = append(recomputeErrs, fmt.Sprintf("%s: feeUnits %s exceed the reward's units %s", label, num(fee), num(u)))
				}
			}

			// Which date's price measures this reward.
			recognized, okR := parseWhen(getString(r, "date"))
			measured, okM := parseWhen(getString(r, "measurementDate"))
			if !okR || !okM {
				priceErrs = append(priceErrs, label+": date/measurementDate are not parseable dates")
			} else {
				var want time.Time
				var wantName string
				switch basis {
				case "contractInception":
					t, ok := parseWhen(getString(r, "inceptionOn"))
					if !ok {
						priceErrs = append(priceErrs, label+": pricingDateBasis contractInception requires inceptionOn on every row")
					} else if t.After(recognized) {
						priceErrs = append(priceErrs, label+": inceptionOn "+getString(r, "inceptionOn")+" is after the recognition date")
					}
					want, wantName = t, "inceptionOn"
				case "receiptDate":
					t, ok := parseWhen(getString(r, "receivedOn"))
					if !ok {
						priceErrs = append(priceErrs, label+": pricingDateBasis receiptDate requires receivedOn on every row")
					}
					want, wantName = t, "receivedOn"
				default:
					want, wantName = recognized, "date"
				}
				if !want.IsZero() && !measured.Equal(want) {
					priceErrs = append(priceErrs, fmt.Sprintf("%s: measurementDate %s is not the %s %s the %s basis requires", label, getString(r, "measurementDate"), wantName, want.Format("2006-01-02"), basis))
				}
				if okS && okE && (recognized.Before(start) || recognized.After(end)) {
					popErrs = append(popErrs, fmt.Sprintf("%s: recognition date outside the schedule period %s to %s", label, start.Format("2006-01-02"), end.Format("2006-01-02")))
				}
			}
			// The price fact must state this price as of the measurement date.
			ref := getString(r, "priceFact")
			if msg := checkFactValue(facts, ref, price, tol, label+" unitPrice", "unitPrice", "price", "amount", "close"); msg != "" {
				priceErrs = append(priceErrs, msg)
			} else if fd, ok := factDate(facts[ref]); !ok {
				priceErrs = append(priceErrs, fmt.Sprintf("%s: fact %q states no date for its price (value needs a date)", label, ref))
			} else if okM && !fd.Equal(measured) {
				priceErrs = append(priceErrs, fmt.Sprintf("%s: fact %q is a price as of %s, not the measurement date %s", label, ref, fd.Format("2006-01-02"), getString(r, "measurementDate")))
			}
			// Reward identity across schedules.
			reference := strings.TrimSpace(getString(r, "reference"))
			if reference == "" {
				popErrs = append(popErrs, label+": reference (block or payout id) is required to detect double counting")
			} else if prev, dup := refsSeen[key+"|"+reference]; dup {
				popErrs = append(popErrs, fmt.Sprintf("%s: reward %q already appears in %s", label, reference, prev))
			} else {
				refsSeen[key+"|"+reference] = hit.Path
			}
		}
		if !nearTol(jsNumber(outputs["totalUnits"]), units, unitTol) || !nearTol(jsNumber(outputs["totalFairValue"]), fairValue, tol) {
			recomputeErrs = append(recomputeErrs, fmt.Sprintf("%stotals do not equal the sum of rows: units %s vs %s, fair value %s vs %s",
				pfx, num(jsNumber(outputs["totalUnits"])), num(units), num(jsNumber(outputs["totalFairValue"])), num(fairValue)))
		}

		// Recognized units reconcile to chain receipts through unreceived
		// balances: received = recognized − closingUnreceived + openingUnreceived − other.
		observed := jsNumber(inputs["chainObservedUnits"])
		opening := jsNumber(inputs["unreceivedOpeningUnits"])
		closing := jsNumber(inputs["unreceivedClosingUnits"])
		other := jsNumber(inputs["otherReconcilingUnits"])
		if msg := checkFactValue(facts, getString(inputs, "chainObservedFact"), observed, unitTol, pfx+"chainObservedUnits", "units", "chainObservedUnits", "quantity", "amount"); msg != "" {
			chainErrs = append(chainErrs, msg)
		} else if expectReceived := units + opening - closing - other; !nearTol(expectReceived, observed, unitTol) {
			chainErrs = append(chainErrs, fmt.Sprintf("%srecognized %s + unreceived opening %s − unreceived closing %s − other %s = %s does not equal chain-observed receipts %s", pfx, num(units), num(opening), num(closing), num(other), num(expectReceived), num(observed)))
		}
		if (closing != 0 || opening != 0 || other != 0) && strings.TrimSpace(getString(inputs, "reconciliationNote")) == "" {
			chainErrs = append(chainErrs, pfx+"reconciling items declared without a reconciliationNote explaining them")
		}
		if ref := getString(inputs, "unreceivedClosingFact"); ref != "" {
			if msg := checkFactValue(facts, ref, closing, unitTol, pfx+"unreceivedClosingUnits", "units", "unreceivedUnits", "quantity", "amount"); msg != "" {
				chainErrs = append(chainErrs, msg)
			}
		}
	}

	// Overlapping schedules for one asset and arrangement.
	sort.Slice(periods, func(i, j int) bool {
		if periods[i].key != periods[j].key {
			return periods[i].key < periods[j].key
		}
		return periods[i].start.Before(periods[j].start)
	})
	for i := 1; i < len(periods); i++ {
		a, b := periods[i-1], periods[i]
		if a.key == b.key && periodsOverlap(a.start, a.end, b.start, b.end) {
			popErrs = append(popErrs, fmt.Sprintf("%s and %s cover overlapping periods for %s", a.path, b.path, a.key))
		}
	}

	emit := func(id, passMsg, failPrefix string, errs []string) {
		if len(errs) > 0 {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusFail, Message: failPrefix + joinCapped(errs, 5)})
		} else {
			add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusPass, Message: passMsg})
		}
	}
	emit(ValidatorMiningRewardsRecompute, fmt.Sprintf("%d schedule(s) recompute from units × price", len(hits)), "reward rows do not recompute: ", recomputeErrs)
	emit(ValidatorMiningRewardsChainTieOut, "recognized units reconcile to the chain-observed receipts stated by the cited facts", "", chainErrs)
	emit(ValidatorMiningRewardsPricingEvidence, "every reward is priced as of the date its pricing basis fixes, by a fact stating that price on that date", "pricing does not follow the declared basis or its facts: ", priceErrs)
	emit(ValidatorMiningRewardsPopulation, "rewards lie in their periods, references are unique and schedules do not overlap", "reward population problems: ", popErrs)
	add(tieOutAccountingSchedule(pkg, ValidatorMiningAccountingScheduleTieOut, miningScheduleExpectation(hits)))
	return results
}

// miningScheduleExpectation derives the executable revenue entries from the
// reward rows: each reward is mining revenue on its recognition date at
// units × price. A reward received on that date (or earlier) debits the
// crypto asset; one received later is a receivable on recognition,
// transferred to the crypto asset on its receipt date; one not yet received
// (a pending pool payout) stays a receivable.
func miningScheduleExpectation(hits []calcHit) scheduleExpectation {
	exp := scheduleExpectation{kind: accountingschedule.KindMiningRevenue}
	for _, hit := range hits {
		inputs := getMap(hit.Doc, "inputs")
		exp.sources++
		exp.currency(getString(inputs, "currency"))
		for i, r := range getMaps(inputs, "rewards") {
			label := fmt.Sprintf("%s row %d (%s)", hit.Path, i+1, getString(r, "reference"))
			fv := jsNumber(r["units"]) * jsNumber(r["unitPrice"])
			date, ok := isoDate(getString(r, "date"))
			if !ok {
				exp.problem(label + ": date is not a date")
				continue
			}
			recognized, _ := parseWhen(date)
			assetRole := RoleCryptoAsset
			received, hasReceipt := isoDate(getString(r, "receivedOn"))
			if !hasReceipt {
				assetRole = RoleMiningRewardReceivable
			} else if t, _ := parseWhen(received); t.After(recognized) {
				assetRole = RoleMiningRewardReceivable
				exp.group(scheduleGroup{obligationId: getString(r, "reference"), date: received, label: label + " receipt",
					postings: []schedulePosting{debit(fv, RoleCryptoAsset), credit(fv, RoleMiningRewardReceivable)}})
			}
			exp.group(scheduleGroup{obligationId: getString(r, "reference"), date: date, label: label,
				postings: []schedulePosting{debit(fv, assetRole), credit(fv, RoleMiningRevenue)}})
		}
	}
	return exp
}
