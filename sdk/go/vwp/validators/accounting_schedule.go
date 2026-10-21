package validators

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// accountingScheduleTieOut uses exact amounts from the executable artifact.
// Every obligation and month must match the reviewed allocation and CSV; a
// balanced journal alone is insufficient evidence of correct recognition.
func accountingScheduleTieOut(pkg vwp.Source) []vwp.ValidationResult {
	result := vwp.ValidationResult{ValidatorId: vwp.ValidatorASC606AccountingScheduleTieOut, Status: vwp.StatusPass}
	if err := validateAccountingScheduleTieOut(pkg); err != nil {
		result.Status, result.Message = vwp.StatusFail, err.Error()
	}
	return []vwp.ValidationResult{result}
}

func validateAccountingScheduleTieOut(pkg vwp.Source) error {
	raw, err := pkg.Read(accountingschedule.ArtifactPath)
	if err != nil {
		return err
	}
	artifact, err := accountingschedule.Parse(raw)
	if err != nil {
		return err
	}
	if artifact.Kind != accountingschedule.KindRevenueRecognition {
		return fmt.Errorf("ASC 606 requires a revenue_recognition artifact")
	}
	allocationRaw, err := pkg.Read(sspAllocationPath)
	if err != nil {
		return fmt.Errorf("accounting schedule requires SSP allocation: %w", err)
	}
	var allocation struct {
		Inputs struct {
			Currency         string      `json:"currency"`
			TransactionPrice json.Number `json:"transactionPrice"`
		} `json:"inputs"`
		Outputs struct {
			Allocations []struct {
				ComponentID     string      `json:"componentId"`
				AllocatedAmount json.Number `json:"allocatedAmount"`
			} `json:"allocations"`
		} `json:"outputs"`
	}
	decoder := json.NewDecoder(bytes.NewReader(allocationRaw))
	decoder.UseNumber()
	if err := decoder.Decode(&allocation); err != nil {
		return err
	}
	if allocation.Inputs.Currency != artifact.Currency {
		return fmt.Errorf("accounting schedule currency must match the allocation currency")
	}
	expected := map[string]*big.Rat{}
	for _, item := range allocation.Outputs.Allocations {
		value, ok := new(big.Rat).SetString(string(item.AllocatedAmount))
		if !ok || value.Sign() < 0 || item.ComponentID == "" || expected[item.ComponentID] != nil {
			return fmt.Errorf("invalid or duplicate allocation component")
		}
		expected[item.ComponentID] = value
	}
	total, _ := accountingschedule.Decimal(artifact.TotalAmount, artifact.AmountScale)
	price, ok := new(big.Rat).SetString(string(allocation.Inputs.TransactionPrice))
	if !ok || total.Cmp(price) != 0 {
		return fmt.Errorf("accounting schedule total must exactly equal transaction price")
	}
	actual, months := map[string]*big.Rat{}, map[string]*big.Rat{}
	obligationMonths := map[string]*big.Rat{}
	for _, entry := range artifact.Entries {
		if expected[entry.ObligationID] == nil {
			return fmt.Errorf("entry %s must name an allocated obligationId", entry.ID)
		}
		amount := new(big.Rat)
		for _, posting := range entry.Postings {
			if posting.Debit != "" {
				value, _ := accountingschedule.Decimal(posting.Debit, artifact.AmountScale)
				amount.Add(amount, value)
			}
		}
		addAmount(actual, entry.ObligationID, amount)
		addAmount(months, entry.Date[:7], amount)
		addAmount(obligationMonths, entry.ObligationID+"\x00"+entry.Date[:7], amount)
	}
	for id, amount := range expected {
		got := actual[id]
		if got == nil {
			got = new(big.Rat)
		}
		if amount.Cmp(got) != 0 {
			return fmt.Errorf("accounting schedule does not exactly tie to allocation for obligation %s", id)
		}
	}
	csvRaw, err := pkg.Read(revenueSchedulePath)
	if err != nil {
		return fmt.Errorf("accounting schedule requires revenue schedule CSV: %w", err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(csvRaw))).ReadAll()
	if err != nil || len(rows) < 2 {
		return fmt.Errorf("revenue schedule CSV is invalid")
	}
	monthCol, totalCol := -1, -1
	obligationColumns := map[string]int{}
	headers := map[string]bool{}
	for i, name := range rows[0] {
		name = strings.TrimSpace(name)
		if headers[name] {
			return fmt.Errorf("duplicate revenue CSV column %s", name)
		}
		headers[name] = true
		switch name {
		case "month":
			monthCol = i
		case "total":
			totalCol = i
		default:
			if expected[name] == nil {
				return fmt.Errorf("CSV obligation columns must use allocated component IDs; unknown column %s", name)
			}
			obligationColumns[name] = i
		}
	}
	if monthCol < 0 || totalCol < 0 {
		return fmt.Errorf("revenue schedule CSV requires month and total columns")
	}
	if len(obligationColumns) != len(expected) {
		return fmt.Errorf("executable accounting schedules require a revenue CSV column for every allocated obligation ID")
	}
	expectedMonths := map[string]*big.Rat{}
	expectedObligationMonths := map[string]*big.Rat{}
	for _, row := range rows[1:] {
		month := strings.TrimSpace(row[monthCol])
		date, err := time.Parse("2006-01", month)
		if err != nil || date.Year() < 1 {
			return fmt.Errorf("CSV month must be YYYY-MM")
		}
		value, ok := new(big.Rat).SetString(strings.TrimSpace(row[totalCol]))
		if !ok || value.Sign() < 0 {
			return fmt.Errorf("invalid CSV recognition amount")
		}
		addAmount(expectedMonths, month, value)
		rowTotal := new(big.Rat)
		for obligation, column := range obligationColumns {
			amount, ok := new(big.Rat).SetString(strings.TrimSpace(row[column]))
			if !ok || amount.Sign() < 0 {
				return fmt.Errorf("invalid CSV amount for obligation %s in %s", obligation, month)
			}
			rowTotal.Add(rowTotal, amount)
			addAmount(expectedObligationMonths, obligation+"\x00"+month, amount)
		}
		if rowTotal.Cmp(value) != 0 {
			return fmt.Errorf("CSV obligation amounts do not equal total in %s", month)
		}
	}
	for month, amount := range expectedMonths {
		got := months[month]
		if got == nil {
			got = new(big.Rat)
		}
		if amount.Cmp(got) != 0 {
			return fmt.Errorf("accounting schedule and revenue CSV differ for month %s", month)
		}
		delete(months, month)
	}
	if len(months) > 0 {
		return fmt.Errorf("accounting schedule contains recognition months missing from revenue CSV")
	}
	for key, amount := range expectedObligationMonths {
		got := obligationMonths[key]
		if got == nil {
			got = new(big.Rat)
		}
		if amount.Cmp(got) != 0 {
			parts := strings.SplitN(key, "\x00", 2)
			return fmt.Errorf("accounting schedule and revenue CSV differ for obligation %s in %s", parts[0], parts[1])
		}
		delete(obligationMonths, key)
	}
	if len(obligationMonths) > 0 {
		return fmt.Errorf("accounting schedule contains obligation periods missing from revenue CSV")
	}
	return nil
}

func addAmount(amounts map[string]*big.Rat, key string, value *big.Rat) {
	if amounts[key] == nil {
		amounts[key] = new(big.Rat)
	}
	amounts[key].Add(amounts[key], value)
}
