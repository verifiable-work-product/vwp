package validators_test

import (
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

const asc250Fixture = "asc250-error-correction"

func asc250Dir(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "examples", asc250Fixture, "package")
}

func editASC250(t *testing.T, dir string, edit func(in, out map[string]any)) {
	t.Helper()
	editCalc(t, dir, "s2_error_correction", func(doc map[string]any) {
		edit(doc["inputs"].(map[string]any), doc["outputs"].(map[string]any))
	})
}

func TestASC250FixturePassesEveryValidator(t *testing.T) {
	report := verifyDir(t, filepath.Join(repoRoot, "examples", asc250Fixture, "package"))
	if !report.Valid {
		for _, r := range report.Results {
			if r.Status == vwp.StatusFail {
				t.Errorf("FAIL %s: %s", r.ValidatorId, r.Message)
			}
		}
	}
	ids := []string{vwp.ValidatorRequiredOutputsExist, vwp.ValidatorAccountingScheduleValid}
	for _, o := range vwp.RequiredOutputs(vwp.ExtensionASC250) {
		ids = append(ids, o.Validators...)
	}
	for _, id := range ids {
		if res := result(t, report, id); res.Status != vwp.StatusPass {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	}
}

func TestASC250CorrectionRecompute(t *testing.T) {
	id := validators.ValidatorASC250CorrectionRecompute
	t.Run("restated balances carry the cumulative correction forward", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) {
			out["lines"].([]any)[0].(map[string]any)["periods"].([]any)[1].(map[string]any)["asRestated"] = 145000.0
		})
		expectFail(t, dir, id, "FY2025: asRestated declared 145000 but asReported 135000 + cumulative correction 25000 = 160000")
	})
	t.Run("both SAB 108 figures recompute from the periods", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) {
			out["rollover"].(map[string]any)["maxPeriodCorrection"] = 10000.0
		})
		expectFail(t, dir, id, "rollover.maxPeriodCorrection declared 10000 but recomputes to 15000")
		editASC250(t, dir, func(in, out map[string]any) {
			out["rollover"].(map[string]any)["maxPeriodCorrection"] = 15000.0
			out["ironCurtain"].(map[string]any)["pctOfBasis"] = 0.05
		})
		expectFail(t, dir, id, "ironCurtain.pctOfBasis declared 0.05 but recomputes to 0.0625")
	})
	t.Run("the basis must equal its fact", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) {
			in["materiality"].(map[string]any)["basisAmount"] = 500000.0
			out["rollover"].(map[string]any)["pctOfBasis"] = 0.03
			out["ironCurtain"].(map[string]any)["pctOfBasis"] = 0.05
		})
		expectFail(t, dir, id, `materiality.basisAmount is 500000 but fact "s3_f_pretax_income_fy2025" says 400000`)
	})
	t.Run("duplicate periods and lines fail", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) {
			in["lines"].([]any)[0].(map[string]any)["periods"].([]any)[1].(map[string]any)["period"] = "FY2024"
		})
		expectFail(t, dir, id, "period FY2024 listed twice")
	})
}

func TestASC250DualMethodAndJudgment(t *testing.T) {
	t.Run("the iron-curtain quantification cannot be dropped", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) { delete(out, "ironCurtain") })
		expectFail(t, dir, validators.ValidatorASC250CorrectionSchemaValid, "ironCurtain")
	})
	t.Run("the materiality judgment must resolve", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) { in["materialityJudgmentRef"] = "s3_j_nowhere" })
		expectFail(t, dir, validators.ValidatorASC250CorrectionMaterialityJudged, `"s3_j_nowhere" is not a judgment in the package`)
	})
	t.Run("a material result still open is not dispositioned", func(t *testing.T) {
		dir := asc250Dir(t)
		editJSON(t, filepath.Join(dir, "judgments", "judgments.json"), func(doc map[string]any) {
			for _, j := range doc["judgments"].([]any) {
				if jm := j.(map[string]any); jm["id"] == "s3_j_materiality" {
					jm["status"] = "open"
				}
			}
		})
		rehash(t, dir, "judgments/judgments.json")
		expectFail(t, dir, validators.ValidatorASC250CorrectionMaterialityJudged, "reaches the 0.05 threshold")
	})
	t.Run("an immaterial result may stay open", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) {
			in["materiality"].(map[string]any)["quantitativeThreshold"] = 0.10
		})
		editJSON(t, filepath.Join(dir, "judgments", "judgments.json"), func(doc map[string]any) {
			for _, j := range doc["judgments"].([]any) {
				if jm := j.(map[string]any); jm["id"] == "s3_j_materiality" {
					jm["status"] = "open"
				}
			}
		})
		rehash(t, dir, "judgments/judgments.json")
		expectPass(t, dir, validators.ValidatorASC250CorrectionMaterialityJudged)
	})
}

func TestASC250ScheduleTieOut(t *testing.T) {
	id := validators.ValidatorASC250AccountingScheduleTieOut
	t.Run("a credit-natural account increases on the credit side", func(t *testing.T) {
		dir := asc250Dir(t)
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{{AccountRole: "accrued_liabilities", Debit: "25000.00"}, {AccountRole: validators.RolePriorPeriodAdjustment, Credit: "25000.00"}}
		})
		expectFail(t, dir, id, "expected credit accrued_liabilities 25000")
	})
	t.Run("the offset is the line's declared role", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) {
			in["lines"].([]any)[0].(map[string]any)["offsetRole"] = "hosting_expense"
		})
		expectFail(t, dir, id, "expected debit hosting_expense 25000")
	})
	t.Run("a negative cumulative correction flips the sides", func(t *testing.T) {
		dir := asc250Dir(t)
		editASC250(t, dir, func(in, out map[string]any) {
			periods := in["lines"].([]any)[0].(map[string]any)["periods"].([]any)
			periods[0].(map[string]any)["correction"] = -15000.0
			periods[1].(map[string]any)["correction"] = -10000.0
			op := out["lines"].([]any)[0].(map[string]any)
			op["periods"].([]any)[0].(map[string]any)["asRestated"] = 105000.0
			op["periods"].([]any)[1].(map[string]any)["asRestated"] = 110000.0
			op["cumulativeCorrection"] = -25000.0
		})
		expectFail(t, dir, id, "expected debit accrued_liabilities 25000")
		editSchedule(t, dir, func(a *accountingschedule.Artifact) {
			a.Entries[0].Postings = []accountingschedule.Posting{{AccountRole: "accrued_liabilities", Debit: "25000.00"}, {AccountRole: validators.RolePriorPeriodAdjustment, Credit: "25000.00"}}
		})
		expectPass(t, dir, id)
	})
	t.Run("corrections that net to zero book nothing", func(t *testing.T) {
		dir := asc250Dir(t)
		removePart(t, dir, accountingschedule.ArtifactPath)
		expectFail(t, dir, id, "is required")
		editASC250(t, dir, func(in, out map[string]any) {
			periods := in["lines"].([]any)[0].(map[string]any)["periods"].([]any)
			periods[1].(map[string]any)["correction"] = -15000.0
			op := out["lines"].([]any)[0].(map[string]any)
			op["periods"].([]any)[1].(map[string]any)["asRestated"] = 135000.0
			op["cumulativeCorrection"] = 0.0
			out["ironCurtain"].(map[string]any)["cumulativeCorrection"] = 0.0
			out["ironCurtain"].(map[string]any)["pctOfBasis"] = 0.0
		})
		res := result(t, verifyDir(t, dir), id)
		if res.Status != vwp.StatusPass || !strings.Contains(res.Message, "book nothing") {
			t.Errorf("%s = %s %q", id, res.Status, res.Message)
		}
	})
}

// rehash rewrites the manifest hash of a part edited in place.
func rehash(t *testing.T, dir, relPath string) {
	t.Helper()
	var doc map[string]any
	editJSON(t, filepath.Join(dir, filepath.FromSlash(relPath)), func(d map[string]any) { doc = d })
	addPart(t, dir, relPath, "judgments", doc)
}
