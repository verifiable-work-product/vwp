package validators

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// Account-role vocabularies the schedule-bearing extensions fix. A reviewer
// binds each role to a ledger account when the schedule is created; fixing
// the vocabulary per kind is what lets the tie-out prove the DIRECTION of
// every entry (a remeasurement gain booked as a loss balances just as well).
const (
	// fair_value_remeasurement (ASC 350-60): the crypto asset carrying
	// amount (or its fair value adjustment account) and the period's
	// remeasurement gain/loss in net income.
	RoleCryptoAsset         = "crypto_asset"
	RoleCryptoAssetGainLoss = "crypto_asset_gain_loss"
	// impairment (ASC 350-30).
	RoleIntangibleAsset = "intangible_asset"
	RoleImpairmentLoss  = "impairment_loss"
	// staking_rewards: the reward received (crypto_asset) or recognized
	// before receipt (staking_reward_receivable), against reward income.
	RoleStakingRewardReceivable = "staking_reward_receivable"
	RoleStakingRewardIncome     = "staking_reward_income"
	// interest_accrual (digital asset lending, in-kind fee at fair value).
	RoleInterestReceivable = "interest_receivable"
	RoleInterestIncome     = "interest_income"
	// realized_gain_loss (lot relief): proceeds in, cost basis out, and the
	// difference. The crypto asset leaves at COST BASIS: this schedule books
	// the subledger's relief, and any fair value adjustment carried under
	// ASC 350-60 is remeasured by that package's own schedule.
	RoleDisposalProceeds = "disposal_proceeds"
	RoleRealizedGainLoss = "realized_gain_loss"
)

// schedulePosting is one posting line an extension expects the executable
// schedule to carry: any of roles, on the debit or credit side, for amount.
type schedulePosting struct {
	roles  []string
	debit  bool
	amount float64
}

func debit(amount float64, roles ...string) schedulePosting {
	return schedulePosting{roles: roles, debit: true, amount: amount}
}

func credit(amount float64, roles ...string) schedulePosting {
	return schedulePosting{roles: roles, debit: false, amount: amount}
}

// scheduleGroup is the set of postings expected under one obligationId on
// one date. The artifact may split them over several entries; the group's
// postings are compared as a multiset so an entry pair (gain and loss on
// one asset at one period end) and one combined entry both tie.
type scheduleGroup struct {
	obligationId string
	date         string // YYYY-MM-DD
	label        string
	postings     []schedulePosting
	// optional groups may be absent (a receipt transfer after recognition)
	// but must tie when present.
	optional bool
}

// scheduleExpectation is what an extension derives from its own schema-valid
// calculations before looking at the artifact.
type scheduleExpectation struct {
	kind string
	// currencies declared by the contributing calculations; exactly one,
	// equal to the artifact's, is required.
	currencies map[string]bool
	groups     []scheduleGroup
	// problems are defects in the calculations themselves that make the
	// schedule unprovable (an unpriced in-kind accrual).
	problems []string
	// sources counts the calculations the expectation came from.
	sources int
}

func (e *scheduleExpectation) currency(c string) {
	if e.currencies == nil {
		e.currencies = map[string]bool{}
	}
	if strings.TrimSpace(c) != "" {
		e.currencies[c] = true
	}
}

func (e *scheduleExpectation) problem(msg string) {
	e.problems = append(e.problems, msg)
}

func (e *scheduleExpectation) group(g scheduleGroup) {
	e.groups = append(e.groups, g)
}

// bookable reports whether anything in the expectation needs an entry once
// amounts below half a unit at scale are dropped — the rule the matcher
// applies, so a package with nothing to book is told so consistently.
func (e scheduleExpectation) bookable(scale int) bool {
	tol := scheduleTolerance(scale)
	for _, g := range e.groups {
		if g.optional {
			continue
		}
		for _, p := range g.postings {
			if math.Abs(p.amount) >= tol {
				return true
			}
		}
	}
	return false
}

// scheduleTolerance is half a unit at the artifact's declared scale: the
// executable amount must be the calculation's amount correctly rounded.
func scheduleTolerance(scale int) float64 {
	return 0.5*math.Pow(10, -float64(scale)) + 1e-9
}

type actualPosting struct {
	entryId string
	role    string
	debit   bool
	amount  float64
	used    bool
}

// tieOutAccountingSchedule proves the executable artifact books exactly
// what the extension's calculations say, and nothing else: right kind and
// currency; one group of postings per obligation and date; each posting on
// the right side, under a role the kind allows, for the calculation's
// amount rounded to the artifact's scale; no entries the calculations do
// not explain. It reports ONE result under validatorId.
func tieOutAccountingSchedule(pkg vwp.Source, validatorId string, exp scheduleExpectation) vwp.ValidationResult {
	result := vwp.ValidationResult{ValidatorId: validatorId, Refs: []string{accountingschedule.ArtifactPath}}
	fail := func(msg string) vwp.ValidationResult {
		result.Status, result.Message = vwp.StatusFail, msg
		return result
	}
	if len(exp.problems) > 0 {
		return fail("the calculations cannot be booked: " + joinCapped(exp.problems, 5))
	}
	if !pkg.Has(accountingschedule.ArtifactPath) {
		// Nothing to book is the only reason the schedule may be absent;
		// decide at the finest scale the artifact allows so no nonzero
		// amount is an excuse (the preparer picks the scale that rounds it).
		if exp.sources == 0 {
			return fail("no calculation to derive " + accountingschedule.ArtifactPath + " from")
		}
		if !exp.bookable(8) {
			result.Status = vwp.StatusPass
			result.Message = fmt.Sprintf("%d calculation(s) book nothing; no executable schedule is required", exp.sources)
			return result
		}
		return fail(accountingschedule.ArtifactPath + " is required: the calculations book entries (write it through vwp_add_calculation with type accountingSchedule)")
	}
	raw, err := pkg.Read(accountingschedule.ArtifactPath)
	if err != nil {
		return fail(err.Error())
	}
	artifact, err := accountingschedule.Parse(raw)
	if err != nil {
		return fail(err.Error())
	}
	if artifact.Kind != exp.kind {
		return fail(fmt.Sprintf("accounting schedule kind %q; this package books %s entries", artifact.Kind, exp.kind))
	}
	switch len(exp.currencies) {
	case 0:
		return fail("the calculations declare no currency; declare inputs.currency on each so the schedule's currency can be checked")
	case 1:
		if !exp.currencies[artifact.Currency] {
			for c := range exp.currencies {
				return fail(fmt.Sprintf("accounting schedule currency %s but the calculations are in %s", artifact.Currency, c))
			}
		}
	default:
		return fail("the calculations declare more than one currency; one schedule carries one currency")
	}
	tol := scheduleTolerance(artifact.AmountScale)

	// Actual postings by obligation and date.
	actual := map[string][]*actualPosting{}
	entryIds := map[string][]string{}
	for _, entry := range artifact.Entries {
		key := entry.ObligationID + "\x00" + entry.Date
		entryIds[key] = append(entryIds[key], entry.ID)
		for _, p := range entry.Postings {
			value, isDebit := p.Debit, true
			if p.Credit != "" {
				value, isDebit = p.Credit, false
			}
			amount, _ := accountingschedule.Decimal(value, artifact.AmountScale)
			f, _ := new(big.Rat).Set(amount).Float64()
			actual[key] = append(actual[key], &actualPosting{entryId: entry.ID, role: p.AccountRole, debit: isDebit, amount: f})
		}
	}

	var errs []string
	expectedKeys := map[string]bool{}
	for _, g := range exp.groups {
		key := g.obligationId + "\x00" + g.date
		expectedKeys[key] = true
		var wanted []schedulePosting
		for _, p := range g.postings {
			if math.Abs(p.amount) >= tol {
				wanted = append(wanted, p)
			}
		}
		got := actual[key]
		if len(wanted) == 0 {
			if len(got) > 0 {
				errs = append(errs, fmt.Sprintf("%s: nothing to book on %s but entries %s exist", g.label, g.date, strings.Join(entryIds[key], ", ")))
			}
			continue
		}
		if len(got) == 0 {
			if g.optional {
				continue
			}
			errs = append(errs, fmt.Sprintf("%s: no entry with obligationId %q dated %s", g.label, g.obligationId, g.date))
			continue
		}
		unmatched := 0
		for _, w := range wanted {
			matched := false
			for _, a := range got {
				if a.used || a.debit != w.debit || !nearTol(a.amount, w.amount, tol) {
					continue
				}
				if !roleAllowed(a.role, w.roles) {
					continue
				}
				a.used, matched = true, true
				break
			}
			if !matched {
				unmatched++
				errs = append(errs, fmt.Sprintf("%s on %s: expected %s %s %s; the entries carry %s", g.label, g.date, sideName(w.debit), strings.Join(w.roles, "|"), num(w.amount), describePostings(got)))
			}
		}
		for _, a := range got {
			// A leftover posting is only news when every expected one matched;
			// otherwise it is the other half of the mismatch already reported.
			if !a.used && unmatched == 0 {
				errs = append(errs, fmt.Sprintf("%s on %s: entry %s posts %s %s %s which no calculation explains", g.label, g.date, a.entryId, sideName(a.debit), a.role, num(a.amount)))
			}
		}
	}
	var stray []string
	for key, ids := range entryIds {
		if !expectedKeys[key] {
			parts := strings.SplitN(key, "\x00", 2)
			stray = append(stray, fmt.Sprintf("%s (obligationId %q, %s)", strings.Join(ids, ", "), parts[0], parts[1]))
		}
	}
	sort.Strings(stray)
	if len(stray) > 0 {
		errs = append(errs, "entries not derived from any calculation: "+joinCapped(stray, 5))
	}
	if len(errs) > 0 {
		return fail("accounting schedule does not tie to the calculations: " + joinCapped(errs, 6))
	}
	result.Status = vwp.StatusPass
	result.Message = fmt.Sprintf("%d entr%s tie to %d calculation(s) as %s in %s", len(artifact.Entries), plural(len(artifact.Entries), "y", "ies"), exp.sources, artifact.Kind, artifact.Currency)
	return result
}

func roleAllowed(role string, roles []string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func sideName(isDebit bool) string {
	if isDebit {
		return "debit"
	}
	return "credit"
}

func describePostings(ps []*actualPosting) string {
	var out []string
	for _, p := range ps {
		out = append(out, fmt.Sprintf("%s %s %s", sideName(p.debit), p.role, num(p.amount)))
	}
	return strings.Join(out, ", ")
}

// isoDate renders a parsed date the way the artifact states its dates.
func isoDate(s string) (string, bool) {
	t, ok := parseWhen(s)
	if !ok {
		return "", false
	}
	return t.Format("2006-01-02"), true
}
