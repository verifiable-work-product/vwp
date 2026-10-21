package validators

import (
	"fmt"
	"sort"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// Cost basis extension validator ids.
const (
	ValidatorCostBasisLotReliefSchemaValid     = "costBasis.lotRelief.schemaValid"
	ValidatorCostBasisLotReliefUnitsConserved  = "costBasis.lotRelief.unitsConserved"
	ValidatorCostBasisLotReliefRecompute       = "costBasis.lotRelief.recompute"
	ValidatorCostBasisLotReliefMethodOrder     = "costBasis.lotRelief.methodOrder"
	ValidatorCostBasisWalletAllocSchemaValid   = "costBasis.walletAllocation.schemaValid"
	ValidatorCostBasisWalletAllocTieOut        = "costBasis.walletAllocation.tieOut"
	ValidatorCostBasisWalletAllocEligibility   = "costBasis.walletAllocation.eligibility"
	ValidatorCostBasisAccountingScheduleTieOut = vwp.ValidatorCostBasisAccountingScheduleTieOut
)

const (
	lotReliefType             = "lotRelief"
	lotReliefSchemaRef        = "extensions/digital-assets-cost-basis/lot-relief.schema.json"
	walletAllocationType      = "walletBasisAllocation"
	walletAllocationSchemaRef = "extensions/digital-assets-cost-basis/wallet-basis-allocation.schema.json"
	costBasisDefaultUnitTol   = 1e-8
	// safeHarborDate is the only date the Rev. Proc. 2024-28 transition safe
	// harbor allocates unused basis as of.
	safeHarborDate = "2025-01-01"
)

type lotState struct {
	id         string
	acquiredAt time.Time
	units      float64
	costBasis  float64
	remaining  float64
}

func (l lotState) unitBasis() float64 {
	if l.units == 0 {
		return 0
	}
	return l.costBasis / l.units
}

// CostBasis implements the vwp-digital-assets-cost-basis extension
// validators over every lot relief schedule (one per asset and pool) and
// every wallet allocation in the package: units are conserved from the lot
// population through every relief into the remaining lots, basis relieved
// and realized gain/loss recompute from the lots, no disposal relieves a
// lot acquired after it, the relief order follows the declared method, and
// the Rev. Proc. 2024-28 allocation ties to the pre-2025 lot population.
func CostBasis(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	reliefScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "asset") + "|" + getString(in, "pool")
	}
	reliefs, found := locateCalculations(pkg, add, ValidatorCostBasisLotReliefSchemaValid, lotReliefType, lotReliefSchemaRef, reliefScope)
	// Lots per asset and the first post-safe-harbor disposal per asset, for
	// the allocation eligibility check below.
	lotsByAsset := map[string][]lotState{}
	disposalsBefore := map[string]bool{}
	firstDisposalAfter := map[string]time.Time{}
	if !found || len(reliefs) == 0 {
		notRun(add, "no schema-valid lot relief", ValidatorCostBasisLotReliefUnitsConserved, ValidatorCostBasisLotReliefRecompute, ValidatorCostBasisLotReliefMethodOrder)
	} else {
		var conserved, recompute, order []string
		for _, lr := range reliefs {
			c, r, o, lots, early, firstAfter := lotReliefChecks(lr)
			conserved = append(conserved, c...)
			recompute = append(recompute, r...)
			order = append(order, o...)
			asset := getString(getMap(lr.Doc, "inputs"), "asset")
			lotsByAsset[asset] = append(lotsByAsset[asset], lots...)
			if early {
				disposalsBefore[asset] = true
			}
			if !firstAfter.IsZero() {
				if cur, ok := firstDisposalAfter[asset]; !ok || firstAfter.Before(cur) {
					firstDisposalAfter[asset] = firstAfter
				}
			}
		}
		emit := func(id, passMsg, failPrefix string, errs []string) {
			if len(errs) > 0 {
				add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusFail, Message: failPrefix + joinCapped(errs, 5)})
			} else {
				add(vwp.ValidationResult{ValidatorId: id, Status: vwp.StatusPass, Message: passMsg})
			}
		}
		emit(ValidatorCostBasisLotReliefUnitsConserved, fmt.Sprintf("%d schedule(s): units conserved from lots through reliefs to remaining lots", len(reliefs)), "units are not conserved: ", conserved)
		emit(ValidatorCostBasisLotReliefRecompute, "basis relieved and realized gain/loss recompute from the lots", "disposals do not recompute: ", recompute)
		emit(ValidatorCostBasisLotReliefMethodOrder, "reliefs follow the declared method order", "method order not followed: ", order)
	}

	// Executable disposal entries: one per disposal, relieving cost basis
	// against proceeds with the difference as realized gain or loss.
	relief := scheduleExpectation{kind: accountingschedule.KindRealizedGainLoss}
	for _, lr := range reliefs {
		lotReliefEntries(lr, &relief)
	}
	add(tieOutAccountingSchedule(pkg, ValidatorCostBasisAccountingScheduleTieOut, relief))

	allocScope := func(doc map[string]any) string {
		in := getMap(doc, "inputs")
		return getString(in, "asset") + "|" + getString(in, "asOf")
	}
	allocs, found := locateCalculations(pkg, add, ValidatorCostBasisWalletAllocSchemaValid, walletAllocationType, walletAllocationSchemaRef, allocScope)
	if !found || len(allocs) == 0 {
		notRun(add, "no schema-valid wallet allocation", ValidatorCostBasisWalletAllocTieOut, ValidatorCostBasisWalletAllocEligibility)
		return results
	}
	facts := factIndex(ctx)
	var tie, elig []string
	eligibleChecked := 0
	for _, wa := range allocs {
		pfx := wa.Path + ": "
		tol := tolerance(wa.Doc, "roundingTolerance", 0.01)
		unitTol := tolerance(wa.Doc, "unitTolerance", costBasisDefaultUnitTol)
		in := getMap(wa.Doc, "inputs")
		out := getMap(wa.Doc, "outputs")
		walletUnits := map[string]float64{}
		for _, w := range getMaps(in, "wallets") {
			walletUnits[getString(w, "walletId")] = jsNumber(w["units"])
		}
		var basisSum, unitSum float64
		seenWallet := map[string]bool{}
		for _, a := range getMaps(out, "allocations") {
			id := getString(a, "walletId")
			basisSum += jsNumber(a["allocatedBasis"])
			unitSum += jsNumber(a["units"])
			if u, ok := walletUnits[id]; !ok || !nearTol(u, jsNumber(a["units"]), unitTol) {
				tie = append(tie, pfx+"allocation for "+id+" does not match the input wallet units")
			}
			if seenWallet[id] {
				tie = append(tie, pfx+"wallet "+id+" allocated twice")
			}
			seenWallet[id] = true
		}
		for id := range walletUnits {
			if !seenWallet[id] {
				tie = append(tie, pfx+"wallet "+id+" has no allocation")
			}
		}
		totalBasis := jsNumber(in["totalUnallocatedBasis"])
		totalUnits := jsNumber(in["totalUnits"])
		if !nearTol(basisSum, totalBasis, tol) {
			tie = append(tie, fmt.Sprintf("%sallocated basis %s does not equal totalUnallocatedBasis %s", pfx, num(basisSum), num(totalBasis)))
		}
		if !nearTol(unitSum, totalUnits, unitTol) {
			tie = append(tie, fmt.Sprintf("%sallocated units %s do not equal totalUnits %s", pfx, num(unitSum), num(totalUnits)))
		}

		// Eligibility: the safe harbor allocates unused basis held at
		// 2025-01-01 across the units held then; it never covers units
		// acquired on or after that date. When the package carries the lot
		// population for the asset, the allocation must equal the pre-2025
		// lots exactly.
		asOf := getString(in, "asOf")
		asset := getString(in, "asset")
		if asOf != safeHarborDate {
			elig = append(elig, fmt.Sprintf("%sasOf %q is not the Rev. Proc. 2024-28 safe harbor date %s", pfx, asOf, safeHarborDate))
			continue
		}
		cut, _ := parseWhen(asOf)

		// The method record: a fact, dated, and dated when the Rev. Proc.
		// requires — a global rule described before 2025-01-01; a specific
		// unit allocation completed by the earlier of the first disposition
		// on or after 2025-01-01 or the 2025 return due date (the latter is
		// not in the package, so only the disposition bound is checked).
		recordRef := getString(in, "methodRecordFact")
		method := getString(in, "allocationMethod")
		if f, ok := facts[recordRef]; !ok {
			elig = append(elig, fmt.Sprintf("%smethodRecordFact %q does not resolve to a fact", pfx, recordRef))
		} else if recorded, ok := factDate(f); !ok {
			elig = append(elig, fmt.Sprintf("%smethodRecordFact %q states no date (its value must carry the date the method was recorded)", pfx, recordRef))
		} else {
			switch method {
			case "globalAllocation":
				if !recorded.Before(cut) {
					elig = append(elig, fmt.Sprintf("%sglobal allocation rule recorded %s; Rev. Proc. 2024-28 §5.02(5) requires the rule in the books and records before %s", pfx, recorded.Format("2006-01-02"), safeHarborDate))
				}
			case "specificUnit":
				if first, ok := firstDisposalAfter[asset]; ok && recorded.After(first) {
					elig = append(elig, fmt.Sprintf("%sspecific unit allocation recorded %s, after the first disposition of %s on or after %s (%s); §5.02(4) requires it by the earlier of that disposition or the 2025 return due date", pfx, recorded.Format("2006-01-02"), asset, safeHarborDate, first.Format("2006-01-02")))
				}
			}
		}

		lots, ok := lotsByAsset[asset]
		switch {
		case !ok:
			continue // no lot population in the package for this asset
		case disposalsBefore[asset]:
			elig = append(elig, pfx+"the lot relief schedule for "+asset+" disposes units before "+safeHarborDate+", so the unused basis at that date cannot be read from the lot population; evidence it separately")
			continue
		}
		eligibleChecked++
		var preUnits, preBasis float64
		for _, l := range lots {
			if l.acquiredAt.Before(cut) {
				preUnits += l.units
				preBasis += l.costBasis
			}
		}
		if !nearTol(preUnits, totalUnits, unitTol) || !nearTol(preBasis, totalBasis, tol) {
			elig = append(elig, fmt.Sprintf("%slots of %s acquired before %s hold %s units / %s basis but the allocation covers %s units / %s basis", pfx, asset, safeHarborDate, num(preUnits), num(preBasis), num(totalUnits), num(totalBasis)))
		}
	}
	if len(tie) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorCostBasisWalletAllocTieOut, Status: vwp.StatusFail, Message: joinCapped(tie, 5)})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorCostBasisWalletAllocTieOut, Status: vwp.StatusPass, Message: fmt.Sprintf("%d allocation(s) tie to their declared basis and units", len(allocs))})
	}
	switch {
	case len(elig) > 0:
		add(vwp.ValidationResult{ValidatorId: ValidatorCostBasisWalletAllocEligibility, Status: vwp.StatusFail, Message: joinCapped(elig, 5)})
	case eligibleChecked == 0:
		add(vwp.ValidationResult{ValidatorId: ValidatorCostBasisWalletAllocEligibility, Status: vwp.StatusWarning, Message: "allocation dated " + safeHarborDate + " but the package carries no lot population to tie it to"})
	default:
		add(vwp.ValidationResult{ValidatorId: ValidatorCostBasisWalletAllocEligibility, Status: vwp.StatusPass, Message: "allocation equals the units and basis of lots acquired before " + safeHarborDate})
	}
	return results
}

// lotReliefChecks validates one lot relief schedule and returns the unit
// conservation, recompute and method-order problems, the lot population
// (for cross-checks) and whether any disposal predates the safe harbor
// date.
func lotReliefChecks(hit calcHit) (conserved, recompute, order []string, population []lotState, earlyDisposal bool, firstDisposalAfter time.Time) {
	doc := hit.Doc
	pfx := hit.Path + ": "
	tol := tolerance(doc, "roundingTolerance", 0.01)
	unitTol := tolerance(doc, "unitTolerance", costBasisDefaultUnitTol)
	in := getMap(doc, "inputs")
	out := getMap(doc, "outputs")
	method := getString(in, "method")
	cut, _ := parseWhen(safeHarborDate)

	lots := map[string]*lotState{}
	var lotOrder []string
	var inputUnits float64
	for _, l := range getMaps(in, "lots") {
		id := getString(l, "lotId")
		if _, dup := lots[id]; dup {
			conserved = append(conserved, pfx+"duplicate lot id "+id)
			continue
		}
		when, ok := parseWhen(getString(l, "acquiredAt"))
		if !ok {
			conserved = append(conserved, pfx+"lot "+id+" has an unparseable acquiredAt "+getString(l, "acquiredAt"))
		}
		st := &lotState{id: id, acquiredAt: when, units: jsNumber(l["units"]), costBasis: jsNumber(l["costBasis"]), remaining: jsNumber(l["units"])}
		lots[id] = st
		lotOrder = append(lotOrder, id)
		inputUnits += st.units
		population = append(population, *st)
	}

	type disposal struct {
		doc  map[string]any
		when time.Time
	}
	var disposals []disposal
	seenDisposal := map[string]bool{}
	for _, d := range getMaps(in, "disposals") {
		id := getString(d, "disposalId")
		if seenDisposal[id] {
			conserved = append(conserved, pfx+"duplicate disposal id "+id)
			continue
		}
		seenDisposal[id] = true
		when, ok := parseWhen(getString(d, "date"))
		if !ok {
			conserved = append(conserved, pfx+"disposal "+id+" has an unparseable date "+getString(d, "date"))
		}
		if when.Before(cut) {
			earlyDisposal = true
		} else if ok && (firstDisposalAfter.IsZero() || when.Before(firstDisposalAfter)) {
			firstDisposalAfter = when
		}
		disposals = append(disposals, disposal{doc: d, when: when})
	}
	sort.SliceStable(disposals, func(i, j int) bool { return disposals[i].when.Before(disposals[j].when) })
	outByDisposal, dupOut := indexByID(getMaps(out, "disposals"), "disposalId")
	for _, d := range dupOut {
		recompute = append(recompute, pfx+"duplicate output row for disposal "+d)
	}

	var totalGain, relievedUnits float64
	for _, dd := range disposals {
		d := dd.doc
		id := getString(d, "disposalId")
		reliefs := getMaps(d, "reliefs")
		var reliefUnits, basisRelieved float64

		// Method order: the lots the method would relieve first, among those
		// with units remaining and acquired on or before the disposal.
		var available []*lotState
		for _, lid := range lotOrder {
			if l := lots[lid]; l.remaining > unitTol && !l.acquiredAt.After(dd.when) {
				available = append(available, l)
			}
		}
		sortByMethod(available, method)
		cursor := 0

		for _, r := range reliefs {
			l, ok := lots[getString(r, "lotId")]
			u := jsNumber(r["units"])
			if !ok {
				conserved = append(conserved, pfx+id+" relieves unknown lot "+getString(r, "lotId"))
				continue
			}
			// Eligibility holds for every method: a sale cannot relieve
			// units bought after it.
			if l.acquiredAt.After(dd.when) {
				conserved = append(conserved, fmt.Sprintf("%s%s (%s) relieves lot %s acquired later (%s)", pfx, id, getString(d, "date"), l.id, l.acquiredAt.Format("2006-01-02")))
			}
			if u > l.remaining+unitTol {
				conserved = append(conserved, fmt.Sprintf("%s%s (lot %s has %s units left, %s relieved)", pfx, id, l.id, num(l.remaining), num(u)))
			}
			if method != "specificIdentification" {
				if cursor >= len(available) || available[cursor].id != l.id {
					want := "none"
					if cursor < len(available) {
						want = available[cursor].id
					}
					order = append(order, fmt.Sprintf("%s%s relieved %s but %s picks %s", pfx, id, l.id, method, want))
				} else if u+unitTol < available[cursor].remaining && reliefUnits+u+unitTol < jsNumber(d["units"]) {
					order = append(order, fmt.Sprintf("%s%s left %s units in %s before moving on", pfx, id, num(available[cursor].remaining-u), l.id))
				}
				if cursor < len(available) && available[cursor].id == l.id && u+unitTol >= available[cursor].remaining {
					cursor++
				}
			}
			basisRelieved += u * l.unitBasis()
			reliefUnits += u
			l.remaining -= u
		}
		relievedUnits += reliefUnits
		if !nearTol(reliefUnits, jsNumber(d["units"]), unitTol) {
			conserved = append(conserved, fmt.Sprintf("%s%s (reliefs total %s units, disposal is %s)", pfx, id, num(reliefUnits), num(jsNumber(d["units"]))))
		}
		gain := jsNumber(d["proceeds"]) - basisRelieved
		totalGain += gain
		o, ok := outByDisposal[id]
		if !ok {
			recompute = append(recompute, pfx+id+" (no output row)")
			continue
		}
		if !nearTol(jsNumber(o["basisRelieved"]), basisRelieved, tol) || !nearTol(jsNumber(o["realizedGainLoss"]), gain, tol) {
			recompute = append(recompute, fmt.Sprintf("%s%s (expected basis %s / gain %s, declared %s / %s)", pfx, id, num(basisRelieved), num(gain), num(jsNumber(o["basisRelieved"])), num(jsNumber(o["realizedGainLoss"]))))
		}
	}
	for id := range outByDisposal {
		if !seenDisposal[id] {
			recompute = append(recompute, pfx+"output row for unknown disposal "+id)
		}
	}

	// Remaining lots: exactly the input lots with units left, nothing else.
	var remainingUnits float64
	remainingOut := map[string]map[string]any{}
	for _, r := range getMaps(out, "remainingLots") {
		lid := getString(r, "lotId")
		if _, dup := remainingOut[lid]; dup {
			conserved = append(conserved, pfx+"remainingLots lists "+lid+" twice")
		}
		remainingOut[lid] = r
		if _, known := lots[lid]; !known {
			conserved = append(conserved, pfx+"remainingLots lists "+lid+" which is not an input lot")
			continue
		}
		remainingUnits += jsNumber(r["units"])
	}
	for _, lid := range lotOrder {
		l := lots[lid]
		r, ok := remainingOut[lid]
		if !ok {
			if l.remaining > unitTol {
				conserved = append(conserved, fmt.Sprintf("%slot %s has %s units left but no remainingLots row", pfx, lid, num(l.remaining)))
			}
			continue
		}
		if !nearTol(jsNumber(r["units"]), l.remaining, unitTol) || !nearTol(jsNumber(r["costBasis"]), l.remaining*l.unitBasis(), tol) {
			conserved = append(conserved, fmt.Sprintf("%slot %s remaining declared %s units / %s basis, expected %s / %s", pfx, lid, num(jsNumber(r["units"])), num(jsNumber(r["costBasis"])), num(l.remaining), num(l.remaining*l.unitBasis())))
		}
	}
	if !nearTol(inputUnits, relievedUnits+remainingUnits, unitTol) {
		conserved = append(conserved, fmt.Sprintf("%slots hold %s units but %s were relieved and %s remain", pfx, num(inputUnits), num(relievedUnits), num(remainingUnits)))
	}

	if declared := jsNumber(out["totalRealizedGainLoss"]); !nearTol(declared, totalGain, tol) {
		recompute = append(recompute, fmt.Sprintf("%stotalRealizedGainLoss %s does not equal the sum of disposals %s", pfx, num(declared), num(totalGain)))
	}
	return conserved, recompute, order, population, earlyDisposal, firstDisposalAfter
}

// sortByMethod orders the lots a method relieves first, earliest first.
// Ties keep lot declaration order.
func sortByMethod(lots []*lotState, method string) {
	switch method {
	case "FIFO":
		sort.SliceStable(lots, func(i, j int) bool { return lots[i].acquiredAt.Before(lots[j].acquiredAt) })
	case "LIFO":
		sort.SliceStable(lots, func(i, j int) bool { return lots[i].acquiredAt.After(lots[j].acquiredAt) })
	case "HIFO":
		sort.SliceStable(lots, func(i, j int) bool { return lots[i].unitBasis() > lots[j].unitBasis() })
	}
}

// lotReliefEntries adds one expected entry group per disposal in a lot
// relief schedule: proceeds in, the cost basis the reliefs take from their
// lots out, and the difference as realized gain (credit) or loss (debit).
// Basis is recomputed from the lots, never read from the declared outputs.
func lotReliefEntries(hit calcHit, exp *scheduleExpectation) {
	in := getMap(hit.Doc, "inputs")
	exp.sources++
	exp.currency(getString(in, "currency"))
	unitBasis := map[string]float64{}
	for _, l := range getMaps(in, "lots") {
		st := lotState{units: jsNumber(l["units"]), costBasis: jsNumber(l["costBasis"])}
		unitBasis[getString(l, "lotId")] = st.unitBasis()
	}
	for _, d := range getMaps(in, "disposals") {
		id := getString(d, "disposalId")
		label := hit.Path + " disposal " + id
		date, ok := isoDate(getString(d, "date"))
		if !ok {
			exp.problem(label + ": date is not a date")
			continue
		}
		var basis float64
		for _, r := range getMaps(d, "reliefs") {
			ub, known := unitBasis[getString(r, "lotId")]
			if !known {
				exp.problem(label + ": relieves unknown lot " + getString(r, "lotId"))
			}
			basis += jsNumber(r["units"]) * ub
		}
		proceeds := jsNumber(d["proceeds"])
		g := scheduleGroup{obligationId: id, date: date, label: label,
			postings: []schedulePosting{debit(proceeds, RoleDisposalProceeds), credit(basis, RoleCryptoAsset)}}
		switch gain := proceeds - basis; {
		case gain > 0:
			g.postings = append(g.postings, credit(gain, RoleRealizedGainLoss))
		case gain < 0:
			g.postings = append(g.postings, debit(-gain, RoleRealizedGainLoss))
		}
		exp.group(g)
	}
}
