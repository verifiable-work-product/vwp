package validators_test

import (
	"math"
	"testing"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func leaseCalc(doc map[string]any) (map[string]any, map[string]any) {
	return doc["inputs"].(map[string]any), doc["outputs"].(map[string]any)
}

// The example's initial liability is the present value of 36 payments of
// 10,000 in arrears at 0.5% a month, computed here independently of the
// generator: 10,000 × (1 − 1.005^−36) / 0.005 = 328,710.16.
func TestASC842ExampleLiabilityIsTheIndependentPresentValue(t *testing.T) {
	want := 10000 * (1 - math.Pow(1.005, -36)) / 0.005
	if math.Abs(want-328710.16) > 0.005 {
		t.Fatalf("independent present value = %.4f", want)
	}
	dir := copyFixture(t, "examples", "asc842-lease", "package")
	editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
		_, out := leaseCalc(doc)
		if got := out["initialLiability"].(float64); math.Abs(got-want) > 0.005 {
			t.Fatalf("example initialLiability %v, independent %v", got, want)
		}
	})
	expectPass(t, dir, validators.ValidatorASC842LeasePvRecompute)
}

func TestASC842FailureModes(t *testing.T) {
	t.Run("present value off", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			_, out := leaseCalc(doc)
			out["initialLiability"] = 330000.0
		})
		expectFail(t, dir, validators.ValidatorASC842LeasePvRecompute, "initialLiability 330000 but the 36 payments discounted at 0.005 per period (arrears) have a present value of 328710.16")
	})
	t.Run("advance timing changes the present value", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			in, _ := leaseCalc(doc)
			in["paymentTiming"] = "advance"
		})
		// An annuity due is worth (1 + r) times the annuity in arrears.
		expectFail(t, dir, validators.ValidatorASC842LeasePvRecompute, "present value of 330353.71")
	})
	t.Run("right-of-use asset ignores initial direct costs", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			in, _ := leaseCalc(doc)
			in["initialDirectCosts"] = 5000.0
		})
		expectFail(t, dir, validators.ValidatorASC842LeasePvRecompute, "initialRouAsset 328710.16 but liability 328710.16 + initial direct costs 5000")
		// The commencement entry must then credit lease_payable for the costs.
		expectFail(t, dir, validators.ValidatorASC842AccountingScheduleTieOut, "cannot be booked")
	})
	t.Run("interest not at the rate", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			_, out := leaseCalc(doc)
			p := out["periods"].([]any)[1].(map[string]any)
			p["interest"] = 1700.0
		})
		expectFail(t, dir, validators.ValidatorASC842LeaseRollForward, "period 2 (2026-02-28): interest 1700 but 320353.71 × 0.005 = 1601.76")
	})
	t.Run("liability does not retire", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			_, out := leaseCalc(doc)
			periods := out["periods"].([]any)
			last := periods[len(periods)-1].(map[string]any)
			last["principal"] = 9900.0
			last["interest"] = 100.0
			last["closingLiability"] = 50.22
		})
		expectFail(t, dir, validators.ValidatorASC842LeaseRollForward, "the final period must close the liability at zero, not 50.22")
	})
	t.Run("operating lease cost must be straight-line", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			_, out := leaseCalc(doc)
			p := out["periods"].([]any)[0].(map[string]any)
			p["leaseCost"] = 1643.55 + 8356.45 + 100
		})
		expectFail(t, dir, validators.ValidatorASC842LeaseRollForward, "leaseCost 10100 but the straight-line single lease cost is 10000")
	})
	t.Run("finance classification changes the amortization and the roles", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			in, _ := leaseCalc(doc)
			in["classification"] = "finance"
		})
		// Finance: ROU amortizes straight-line (328,710.16 / 36 = 9,130.84), not as the operating plug.
		expectFail(t, dir, validators.ValidatorASC842LeaseRollForward, "rouAmortization 8356.45 but the right-of-use asset 328710.16 over 36 periods is 9130.83")
	})
	t.Run("classification judgment and rate fact", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editCalc(t, dir, "s4_lease_schedule", func(doc map[string]any) {
			in, _ := leaseCalc(doc)
			in["classificationJudgmentRef"] = "s2_j_missing"
			in["discountRate"] = 0.065
		})
		expectFail(t, dir, validators.ValidatorASC842LeaseClassificationJudged, `classificationJudgmentRef "s2_j_missing" is not a judgment in the package`)
		expectFail(t, dir, validators.ValidatorASC842LeaseRateEvidence, `discountRate is 0.065 but fact "s3_f_rate" says 0.06`)
	})
	t.Run("schedule: wrong role, missing commencement, extra period", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[1].Postings[0].AccountRole = validators.RoleLeaseInterestExpense
		})
		expectFail(t, dir, validators.ValidatorASC842AccountingScheduleTieOut, "expected debit lease_expense 10000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[1].Postings[0].AccountRole = validators.RoleLeaseExpense
			a.Entries = a.Entries[1:]
			a.TotalAmount = "688710.16"
		})
		expectFail(t, dir, validators.ValidatorASC842AccountingScheduleTieOut, `commencement: no entry with obligationId "OFF-001" dated 2026-01-01`)
	})
	t.Run("missing schedule is required", func(t *testing.T) {
		dir := copyFixture(t, "examples", "asc842-lease", "package")
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, validators.ValidatorASC842AccountingScheduleTieOut, "is required")
	})
}
