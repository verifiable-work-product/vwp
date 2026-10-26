package vwp

import (
	"strings"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

// Canonical ASC 606 output paths and calculation types used by package writers.
const (
	ASC606SSPAllocationPath    = "calculations/ssp-allocation.json"
	ASC606RevenueSchedulePath  = "calculations/revenue-schedule.csv"
	CalcTypeSSPAllocation      = "relativeStandaloneSellingPriceAllocation"
	CalcTypeRevenueSchedule    = "revenueRecognitionSchedule"
	CalcTypeAccountingSchedule = "accountingSchedule"

	ValidatorRequiredOutputsExist    = "core.requiredOutputs.exist"
	ValidatorAccountingScheduleValid = "accountingSchedule.valid"
)

// Validator ids that prove an executable accounting schedule ties to the
// calculation it books. Defined here so the required-output contract can
// name them; the validators package implements them.
const (
	ValidatorASC606AccountingScheduleTieOut      = "asc606.accountingSchedule.tieOut"
	ValidatorASC35060AccountingScheduleTieOut    = "asc35060.accountingSchedule.tieOut"
	ValidatorASC35030AccountingScheduleTieOut    = "asc35030.accountingSchedule.tieOut"
	ValidatorStakingAccountingScheduleTieOut     = "staking.accountingSchedule.tieOut"
	ValidatorLendingAccountingScheduleTieOut     = "lending.accountingSchedule.tieOut"
	ValidatorCostBasisAccountingScheduleTieOut   = "costBasis.accountingSchedule.tieOut"
	ValidatorCommissionsAccountingScheduleTieOut = "commissions.accountingSchedule.tieOut"
)

// RequiredOutput describes a mandatory, manifest-bound extension output.
// CalculationType is the authoring calculation that produces the output; the
// output itself may be a CSV or an executable artifact rather than a Calculation.
//
// Condition, when set, states when the output is required: the core check
// then only insists on correct manifest binding once the part exists, and
// the extension validator named in Validators decides — from the package's
// own calculations — whether the output had to be written. An output with
// no Condition is required unconditionally.
type RequiredOutput struct {
	Kind            string `json:"kind"`
	Path            string `json:"path"`
	MediaType       string `json:"mediaType"`
	CalculationType string `json:"calculationType,omitempty"`
	SchemaVersion   int    `json:"schemaVersion,omitempty"`
	// Condition is the human-readable rule for when a conditional output
	// must be present ("any lot has an impairment loss").
	Condition string `json:"condition,omitempty"`
	// Validators are the ids that must pass for this output to count as
	// delivered, beyond core.requiredOutputs.exist.
	Validators []string `json:"validators,omitempty"`
}

// accountingScheduleOutput is the executable schedule output every
// schedule-bearing extension requires. kind is the schedule kind the
// extension's tie-out accepts; condition states when there is something
// to book (a period with nothing to book has no entries, and the artifact
// contract has no empty schedule).
func accountingScheduleOutput(tieOut, condition string) RequiredOutput {
	return RequiredOutput{
		Kind:            "accountingSchedule",
		Path:            accountingschedule.ArtifactPath,
		MediaType:       "application/json",
		CalculationType: CalcTypeAccountingSchedule,
		SchemaVersion:   accountingschedule.SchemaVersion,
		Condition:       condition,
		Validators:      []string{ValidatorAccountingScheduleValid, tieOut},
	}
}

// RequiredOutputs returns the mandatory output contract for an extension.
// Each call returns an independent slice so callers cannot change verification.
func RequiredOutputs(extension string) []RequiredOutput {
	switch extension {
	case ExtensionASC606:
		return []RequiredOutput{
			{Kind: "sspAllocation", Path: ASC606SSPAllocationPath, MediaType: "application/json", CalculationType: CalcTypeSSPAllocation,
				Validators: []string{"asc606.allocation.schemaValid", "asc606.allocation.tieOut"}},
			{Kind: "revenueSchedule", Path: ASC606RevenueSchedulePath, MediaType: "text/csv", CalculationType: CalcTypeRevenueSchedule,
				Validators: []string{"asc606.revenueSchedule.total"}},
			{Kind: "accountingSchedule", Path: accountingschedule.ArtifactPath, MediaType: "application/json", CalculationType: CalcTypeAccountingSchedule, SchemaVersion: accountingschedule.SchemaVersion,
				Validators: []string{ValidatorAccountingScheduleValid, ValidatorASC606AccountingScheduleTieOut}},
		}
	case ExtensionASC35060:
		return []RequiredOutput{accountingScheduleOutput(ValidatorASC35060AccountingScheduleTieOut,
			"any roll-forward line carries a remeasurement gain or loss for the period (kind fair_value_remeasurement; one entry per asset, period end and direction)")}
	case ExtensionASC35030:
		return []RequiredOutput{accountingScheduleOutput(ValidatorASC35030AccountingScheduleTieOut,
			"any tested lot has an impairment loss (kind impairment; one entry per impaired lot on its test date)")}
	case ExtensionStakingReward:
		return []RequiredOutput{accountingScheduleOutput(ValidatorStakingAccountingScheduleTieOut,
			"any reward has a fair value (kind staking_rewards; one entry per reward on its recognition date)")}
	case ExtensionLending:
		return []RequiredOutput{accountingScheduleOutput(ValidatorLendingAccountingScheduleTieOut,
			"any accrual window accrues a fee (kind interest_accrual; one entry per accrual window on its last accrued day, at the fair value the priced accrual states)")}
	case ExtensionCostBasis:
		return []RequiredOutput{accountingScheduleOutput(ValidatorCostBasisAccountingScheduleTieOut,
			"any lot relief schedule records a disposal (kind realized_gain_loss; one entry per disposal on its date)")}
	case ExtensionCommissions:
		return []RequiredOutput{accountingScheduleOutput(ValidatorCommissionsAccountingScheduleTieOut,
			"any period of the commission statement earns a commission (kind commission_accrual; one entry per period on its last day, at the schedule-currency value the statement states)")}
	}
	return nil
}

// RequiresAccountingSchedule reports whether the extension's contract names
// the executable accounting schedule, conditionally or not.
func RequiresAccountingSchedule(extension string) bool {
	for _, o := range RequiredOutputs(extension) {
		if o.Path == accountingschedule.ArtifactPath {
			return true
		}
	}
	return false
}

func requiresAccountingScheduleUnconditionally(extension string) bool {
	for _, o := range RequiredOutputs(extension) {
		if o.Path == accountingschedule.ArtifactPath && o.Condition == "" {
			return true
		}
	}
	return false
}

func verifyRequiredOutputs(pkg Source, manifest *Manifest, add func(ValidationResult)) {
	outputs := RequiredOutputs(manifest.Extension)
	if len(outputs) == 0 {
		return
	}
	var problems, refs []string
	for _, output := range outputs {
		refs = append(refs, output.Path)
		if !pkg.Has(output.Path) {
			if output.Condition != "" {
				// The extension validator decides whether the package had
				// anything to book; core only polices the binding of what
				// was written.
				continue
			}
			problems = append(problems, "required output missing: "+output.Path)
		}
		var matches []PartRef
		for _, part := range manifest.AllPartRefs() {
			if part.Path == output.Path {
				matches = append(matches, part)
			}
		}
		if len(matches) != 1 {
			problems = append(problems, output.Path+" must be declared exactly once in manifest.parts")
			continue
		}
		inParts := false
		for _, part := range manifest.Parts {
			inParts = inParts || part.Path == output.Path
		}
		part := matches[0]
		if !inParts || part.Role != "calculation" || part.MediaType != output.MediaType || part.SHA256 == "" {
			problems = append(problems, output.Path+" must be declared in manifest.parts with role calculation, mediaType "+output.MediaType+", and sha256")
		}
	}
	result := ValidationResult{ValidatorId: ValidatorRequiredOutputsExist, Status: StatusPass, Refs: refs}
	if len(problems) > 0 {
		result.Status, result.Message = StatusFail, strings.Join(problems, "; ")
	}
	add(result)
}
