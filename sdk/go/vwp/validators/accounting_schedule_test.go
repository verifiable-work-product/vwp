package validators_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
)

func fixtureSchedule() accountingschedule.Artifact {
	a := accountingschedule.Artifact{SchemaVersion: 1, Name: "Revenue", Kind: "revenue_recognition", Currency: "USD", AmountScale: 2, StartDate: "2026-01-01", EndDate: "2027-12-31", TotalAmount: "300000"}
	for i := 0; i < 24; i++ {
		date := time.Date(2026, time.January+time.Month(i)+1, 0, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
		for obligation, amount := range map[string]string{"saas_subscription": "8000", "premium_support": "2000"} {
			a.Entries = append(a.Entries, accountingschedule.Entry{ID: fmt.Sprintf("%s-%d", obligation, i), Date: date, Description: obligation, ObligationID: obligation, Postings: []accountingschedule.Posting{{AccountRole: "deferred_revenue", Debit: amount}, {AccountRole: "revenue", Credit: amount}}})
		}
	}
	a.Entries = append(a.Entries, accountingschedule.Entry{ID: "implementation", Date: "2026-03-31", Description: "Implementation", ObligationID: "implementation_services", Postings: []accountingschedule.Posting{{AccountRole: "deferred_revenue", Debit: "60000"}, {AccountRole: "revenue", Credit: "60000"}}})
	return a
}

func TestExecutableScheduleMustMatchApprovedAllocationAndRecognitionMonths(t *testing.T) {
	for name, edit := range map[string]func(*accountingschedule.Artifact){
		"valid":                     func(a *accountingschedule.Artifact) {},
		"wrong obligation":          func(a *accountingschedule.Artifact) { a.Entries[0].ObligationID = "unknown" },
		"wrong month":               func(a *accountingschedule.Artifact) { a.Entries[len(a.Entries)-1].Date = "2026-04-30" },
		"wrong currency":            func(a *accountingschedule.Artifact) { a.Currency = "EUR" },
		"redistributed obligations": func(a *accountingschedule.Artifact) { a.Entries[len(a.Entries)-1].ObligationID = "saas_subscription" },
	} {
		t.Run(name, func(t *testing.T) {
			dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
			a := fixtureSchedule()
			edit(&a)
			raw, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, accountingschedule.ArtifactPath), raw, 0600); err != nil {
				t.Fatal(err)
			}
			report := verifyDir(t, dir)
			res := result(t, report, "asc606.accountingSchedule.tieOut")
			want := vwp.StatusFail
			if name == "valid" {
				want = vwp.StatusPass
			}
			if res.Status != want {
				t.Fatalf("got %s: %s; want %s", res.Status, res.Message, want)
			}
		})
	}
}

func TestEqualTotalsCannotHideAnObligationTimingSwap(t *testing.T) {
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	allocation := `{"id":"allocation","type":"relativeStandaloneSellingPriceAllocation","inputs":{"transactionPrice":200,"currency":"USD","components":[{"id":"a","standaloneSellingPrice":100},{"id":"b","standaloneSellingPrice":100}]},"outputs":{"allocations":[{"componentId":"a","allocatedAmount":100},{"componentId":"b","allocatedAmount":100}]}}`
	csv := "month,a,b,total\n2026-01,100,0,100\n2026-02,0,100,100\n"
	artifact := accountingschedule.Artifact{SchemaVersion: 1, Name: "Revenue", Kind: "revenue_recognition", Currency: "USD", AmountScale: 2, StartDate: "2026-01-01", EndDate: "2026-02-28", TotalAmount: "200", Entries: []accountingschedule.Entry{
		{ID: "a", Date: "2026-02-28", Description: "Wrong timing for A", ObligationID: "a", Postings: []accountingschedule.Posting{{AccountRole: "deferred_revenue", Debit: "100"}, {AccountRole: "revenue", Credit: "100"}}},
		{ID: "b", Date: "2026-01-31", Description: "Wrong timing for B", ObligationID: "b", Postings: []accountingschedule.Posting{{AccountRole: "deferred_revenue", Debit: "100"}, {AccountRole: "revenue", Credit: "100"}}},
	}}
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string][]byte{"calculations/ssp-allocation.json": []byte(allocation), "calculations/revenue-schedule.csv": []byte(csv), accountingschedule.ArtifactPath: raw} {
		if err := os.WriteFile(filepath.Join(dir, path), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	res := result(t, verifyDir(t, dir), "asc606.accountingSchedule.tieOut")
	if res.Status != vwp.StatusFail || !strings.Contains(res.Message, "differ for obligation") {
		t.Fatalf("timing swap accepted: %+v", res)
	}
}
