package validators

import (
	"fmt"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// ASC 606 extension validator ids.
const (
	ValidatorASC606AllocationSchemaValid = "asc606.allocation.schemaValid"
	ValidatorASC606AllocationTieOut      = "asc606.allocation.tieOut"
	ValidatorASC606RevenueScheduleTotal  = "asc606.revenueSchedule.total"
)

// Data contract paths consumed by the ASC 606 validators.
const (
	sspAllocationPath      = vwp.ASC606SSPAllocationPath
	revenueSchedulePath    = vwp.ASC606RevenueSchedulePath
	sspAllocationSchemaRef = "extensions/accounting-asc606/ssp-allocation.schema.json"
	sspAllocationType      = "relativeStandaloneSellingPriceAllocation"
	revenueScheduleType    = "revenueSchedule"
)

// Exported so package writers put the data contract where the validators
// read it.
const (
	ASC606SSPAllocationPath   = sspAllocationPath
	ASC606RevenueSchedulePath = revenueSchedulePath
)

// ASC606 implements the vwp-accounting-asc606 extension validators:
// allocation schema, then allocation and revenue schedule tie-outs.
func ASC606(pkg vwp.Source, _ *vwp.ExtensionContext) []vwp.ValidationResult {
	results := accountingScheduleTieOut(pkg)
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	// The allocation lives at the reference path or, when a template's
	// section prefix named the calculation (s4_ssp_allocation), wherever a
	// calculation of the allocation type sits under calculations/.
	allocationDoc, present := readJSONValue(pkg, sspAllocationPath)
	if !present {
		if hits := calculationsByType(pkg, sspAllocationType); len(hits) > 0 {
			allocationDoc, present = hits[0].Doc, true
		}
	}

	allocationValid := false
	var allocation map[string]any
	if !present {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606AllocationSchemaValid,
			Status:      vwp.StatusFail,
			Message:     "calculations/ssp-allocation.json not present",
		})
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606AllocationTieOut,
			Status:      vwp.StatusFail,
			Message:     "calculations/ssp-allocation.json not present",
		})
	} else {
		errs, err := schema.ValidateExtension(sspAllocationSchemaRef, allocationDoc)
		if err != nil {
			errs = []string{err.Error()}
		}
		if len(errs) > 0 {
			add(vwp.ValidationResult{
				ValidatorId: ValidatorASC606AllocationSchemaValid,
				Status:      vwp.StatusFail,
				Message:     joinCapped(errs, 5),
			})
			add(vwp.ValidationResult{
				ValidatorId: ValidatorASC606AllocationTieOut,
				Status:      vwp.StatusFail,
				Message:     "allocation failed extension schema validation",
			})
		} else {
			add(vwp.ValidationResult{ValidatorId: ValidatorASC606AllocationSchemaValid, Status: vwp.StatusPass})
			allocationValid = true
			allocation = asMap(allocationDoc)
			transactionPrice := jsNumber(getMap(allocation, "inputs")["transactionPrice"])
			total := allocatedTotal(allocation)
			if near(total, transactionPrice) {
				add(vwp.ValidationResult{
					ValidatorId: ValidatorASC606AllocationTieOut,
					Status:      vwp.StatusPass,
					Message:     fmt.Sprintf("allocated %s equals transaction price %s", num(total), num(transactionPrice)),
				})
			} else {
				add(vwp.ValidationResult{
					ValidatorId: ValidatorASC606AllocationTieOut,
					Status:      vwp.StatusFail,
					Message:     fmt.Sprintf("allocated total %s does not equal transaction price %s", num(total), num(transactionPrice)),
				})
			}
		}
	}

	// The schedule is the reference CSV or, for builders that can only
	// write JSON calculations, a calculation of type revenueSchedule whose
	// outputs.rows carry a `total` per period.
	var scheduleCalc calcHit
	hasScheduleCalc := false
	if hits := calculationsByType(pkg, revenueScheduleType); len(hits) > 0 {
		scheduleCalc, hasScheduleCalc = hits[0], true
	}
	if !pkg.Has(revenueSchedulePath) && !hasScheduleCalc {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusFail,
			Message:     "calculations/revenue-schedule.csv not present",
		})
		return results
	}
	if !allocationValid {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusFail,
			Message:     "requires a schema-valid calculations/ssp-allocation.json to tie against",
		})
		return results
	}
	if !pkg.Has(revenueSchedulePath) {
		rows := getMaps(getMap(scheduleCalc.Doc, "outputs"), "rows")
		if len(rows) == 0 {
			add(vwp.ValidationResult{
				ValidatorId: ValidatorASC606RevenueScheduleTotal,
				Status:      vwp.StatusFail,
				Message:     scheduleCalc.Path + " has no outputs.rows",
			})
			return results
		}
		scheduleTotal := 0.0
		for _, row := range rows {
			scheduleTotal += jsNumber(row["total"])
		}
		add(scheduleTieOut(scheduleTotal, allocatedTotal(allocation)))
		return results
	}

	csvBytes, err := pkg.Read(revenueSchedulePath)
	if err != nil {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusFail,
			Message:     "revenue-schedule.csv unreadable: " + err.Error(),
		})
		return results
	}
	rows := parseCSV(string(csvBytes))
	totalCol := -1
	if len(rows) > 0 {
		for i, cell := range rows[0] {
			if cell == "total" {
				totalCol = i
				break
			}
		}
	}
	if totalCol == -1 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusFail,
			Message:     "revenue-schedule.csv has no 'total' column",
		})
		return results
	}
	scheduleTotal := 0.0
	for _, row := range rows[1:] {
		var cell any
		if totalCol < len(row) {
			cell = row[totalCol]
		}
		scheduleTotal += jsNumber(cell)
	}
	add(scheduleTieOut(scheduleTotal, allocatedTotal(allocation)))
	return results
}

func scheduleTieOut(scheduleTotal, allocated float64) vwp.ValidationResult {
	if near(scheduleTotal, allocated) {
		return vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusPass,
			Message:     fmt.Sprintf("schedule total %s equals allocated consideration %s", num(scheduleTotal), num(allocated)),
		}
	}
	return vwp.ValidationResult{
		ValidatorId: ValidatorASC606RevenueScheduleTotal,
		Status:      vwp.StatusFail,
		Message:     fmt.Sprintf("schedule total %s does not equal allocated consideration %s", num(scheduleTotal), num(allocated)),
	}
}

func allocatedTotal(allocation map[string]any) float64 {
	total := 0.0
	for _, a := range getMaps(getMap(allocation, "outputs"), "allocations") {
		total += jsNumber(a["allocatedAmount"])
	}
	return total
}

// parseCSV mirrors the reference CLI's deliberately simple CSV handling:
// newline-split rows, comma-split cells, trimmed, blank lines dropped. The
// revenue-schedule data contract has no quoted or escaped cells.
func parseCSV(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cells := strings.Split(line, ",")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}
