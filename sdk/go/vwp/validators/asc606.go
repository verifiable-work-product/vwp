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
	sspAllocationPath      = "calculations/ssp-allocation.json"
	revenueSchedulePath    = "calculations/revenue-schedule.csv"
	sspAllocationSchemaRef = "extensions/accounting-asc606/ssp-allocation.schema.json"
)

// ASC606 implements the vwp-accounting-asc606 extension validators:
// allocation schema, then allocation and revenue schedule tie-outs.
func ASC606(pkg vwp.Source, _ *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	allocationDoc, present := readJSONValue(pkg, sspAllocationPath)

	allocationValid := false
	var allocation map[string]any
	if !present {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606AllocationSchemaValid,
			Status:      vwp.StatusNotRun,
			Message:     "calculations/ssp-allocation.json not present",
		})
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606AllocationTieOut,
			Status:      vwp.StatusNotRun,
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
				Status:      vwp.StatusNotRun,
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

	if !pkg.Has(revenueSchedulePath) {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusNotRun,
			Message:     "calculations/revenue-schedule.csv not present",
		})
		return results
	}
	if !allocationValid {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusNotRun,
			Message:     "requires a schema-valid calculations/ssp-allocation.json to tie against",
		})
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
	allocated := allocatedTotal(allocation)
	if near(scheduleTotal, allocated) {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusPass,
			Message:     fmt.Sprintf("schedule total %s equals allocated consideration %s", num(scheduleTotal), num(allocated)),
		})
	} else {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorASC606RevenueScheduleTotal,
			Status:      vwp.StatusFail,
			Message:     fmt.Sprintf("schedule total %s does not equal allocated consideration %s", num(scheduleTotal), num(allocated)),
		})
	}
	return results
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
