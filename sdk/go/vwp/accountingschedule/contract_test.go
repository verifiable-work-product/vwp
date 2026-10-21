package accountingschedule

import (
	"encoding/json"
	"strings"
	"testing"
)

const validArtifact = `{
 "schemaVersion":1,"name":"Subscription","kind":"revenue_recognition",
 "currency":"USD","amountScale":2,"startDate":"2026-01-01","endDate":"2026-03-31",
 "totalAmount":"100.00","entries":[
  {"id":"jan","date":"2026-01-31","description":"January revenue","obligationId":"subscription",
   "postings":[{"accountRole":"deferred_revenue","debit":"33.33"},{"accountRole":"revenue","credit":"33.33"}]},
  {"id":"feb","date":"2026-02-28","description":"February revenue","obligationId":"subscription",
   "postings":[{"accountRole":"deferred_revenue","debit":"33.33"},{"accountRole":"revenue","credit":"33.33"}]},
  {"id":"mar","date":"2026-03-31","description":"March revenue and rounding residue","obligationId":"subscription",
   "postings":[{"accountRole":"deferred_revenue","debit":"33.34"},{"accountRole":"revenue","credit":"33.34"}]}
 ]}`

func TestExactRoundingAndStableCreation(t *testing.T) {
	a, err := Parse([]byte(validArtifact))
	if err != nil {
		t.Fatal(err)
	}
	if err := (CreationConfig{JournalID: "general", AccountBindings: map[string]string{"revenue": "a1", "deferred_revenue": "a2"}}).Validate(*a); err != nil {
		t.Fatal(err)
	}
	source := SourceReference{OrgID: "org", WorkspaceID: "ws", WorkproductID: "vwp", VersionHash: "approved", ArtifactPath: ArtifactPath, ArtifactSHA256: Hash([]byte(validArtifact))}
	first := CreationID(source)
	source.ArtifactSHA256 = "changed"
	if CreationID(source) != first {
		t.Fatal("a conflicting payload must collide rather than create another schedule")
	}
	source.WorkspaceID = "other"
	if CreationID(source) == first {
		t.Fatal("creation identity must be tenant scoped")
	}
}

func TestRejectsUnsafeFinancialArtifacts(t *testing.T) {
	cases := map[string]string{
		"unbalanced":        strings.Replace(validArtifact, `"credit":"33.34"`, `"credit":"33.33"`, 1),
		"wrong total":       strings.Replace(validArtifact, `"totalAmount":"100.00"`, `"totalAmount":"99.99"`, 1),
		"numeric amount":    strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":33.33`, 1),
		"exponent":          strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"3.333e1"`, 1),
		"precision":         strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"33.333"`, 1),
		"negative":          strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"-33.33"`, 1),
		"zero":              strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"0"`, 1),
		"double sided":      strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"33.33","credit":"33.33"`, 1),
		"duplicate entry":   strings.Replace(validArtifact, `"id":"feb"`, `"id":"jan"`, 1),
		"duplicate field":   strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"99.00","debit":"33.33"`, 1),
		"missing precision": strings.Replace(validArtifact, `"amountScale":2,`, ``, 1),
		"null precision":    strings.Replace(validArtifact, `"amountScale":2`, `"amountScale":null`, 1),
		"invalid date":      strings.Replace(validArtifact, `"2026-02-28"`, `"2026-02-30"`, 1),
		"date out of range": strings.Replace(validArtifact, `"2026-02-28"`, `"2026-04-30"`, 1),
		"unknown field":     strings.Replace(validArtifact, `"amountScale":2`, `"amountScale":2,"unchecked":true`, 1),
		"wrong key case":    strings.Replace(validArtifact, `"entries"`, `"Entries"`, 1),
		"wrong nested case": strings.Replace(validArtifact, `"accountRole"`, `"AccountRole"`, 1),
		"null other side":   strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"33.33","credit":null`, 1),
		"null dimensions":   strings.Replace(validArtifact, `"debit":"33.33"`, `"debit":"33.33","dimensions":null`, 1),
		"year zero":         strings.Replace(validArtifact, `"2026-01-01"`, `"0000-01-01"`, 1),
		"trailing document": validArtifact + `{}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(raw)); err == nil {
				t.Fatal("invalid artifact accepted")
			}
		})
	}
}

func TestExactBytesSurviveSourceWireRoundTrip(t *testing.T) {
	a, err := Parse([]byte(validArtifact))
	if err != nil {
		t.Fatal(err)
	}
	source := ApprovedSource{Artifact: *a, ArtifactBytes: []byte(validArtifact), Source: SourceReference{ArtifactSHA256: Hash([]byte(validArtifact))}}
	wire, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ApprovedSource
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if Hash(decoded.ArtifactBytes) != decoded.Source.ArtifactSHA256 {
		t.Fatal("wire encoding changed approved artifact bytes")
	}
}
