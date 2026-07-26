package schema_test

import (
	"strings"
	"testing"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

const shaA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func mustValid(t *testing.T, name string, doc any) {
	t.Helper()
	errs, err := schema.Validate(name, doc)
	if err != nil {
		t.Fatalf("Validate(%s): %v", name, err)
	}
	if len(errs) > 0 {
		t.Fatalf("Validate(%s) = %v, want valid", name, errs)
	}
}

func mustInvalid(t *testing.T, name string, doc any) []string {
	t.Helper()
	errs, err := schema.Validate(name, doc)
	if err != nil {
		t.Fatalf("Validate(%s): %v", name, err)
	}
	if len(errs) == 0 {
		t.Fatalf("Validate(%s) unexpectedly valid", name)
	}
	return errs
}

func validManifest() map[string]any {
	return map[string]any{
		"vwpVersion": "0.1",
		"packageId":  "pkg_test",
		"createdAt":  "2026-07-19T12:00:00Z",
		"profile":    "vwp-core",
		"workproduct": map[string]any{
			"path": "workproduct.json", "mediaType": "application/json",
		},
		"provenance": map[string]any{
			"path": "provenance/provenance.json", "mediaType": "application/json",
		},
	}
}

func TestManifestSchema(t *testing.T) {
	mustValid(t, schema.Manifest, validManifest())

	wrongVersion := validManifest()
	wrongVersion["vwpVersion"] = "0.2"
	mustInvalid(t, schema.Manifest, wrongVersion)

	badTimestamp := validManifest()
	badTimestamp["createdAt"] = "yesterday"
	mustInvalid(t, schema.Manifest, badTimestamp) // format assertion is on

	badHash := validManifest()
	badHash["workproduct"] = map[string]any{
		"path": "workproduct.json", "mediaType": "application/json", "sha256": "NOT-A-HASH",
	}
	mustInvalid(t, schema.Manifest, badHash)
}

func TestUnknownSchemaName(t *testing.T) {
	if _, err := schema.Validate("nope", map[string]any{}); err == nil {
		t.Error("unknown schema name should error")
	}
	if _, err := schema.ValidateExtension("extensions/nope/nope.schema.json", map[string]any{}); err == nil {
		t.Error("unknown extension schema should error")
	}
}

func TestValidateBytes(t *testing.T) {
	errs, err := schema.ValidateBytes(schema.Claim, []byte(`{"id": "c1", "statement": "s"}`))
	if err != nil || len(errs) != 0 {
		t.Fatalf("ValidateBytes = %v, %v", errs, err)
	}
	if _, err := schema.ValidateBytes(schema.Claim, []byte(`{not json`)); err == nil {
		t.Error("invalid JSON should error")
	}
	errs, err = schema.ValidateBytes(schema.Claim, []byte(`{"id": "c1"}`))
	if err != nil || len(errs) == 0 {
		t.Errorf("claim without statement should be invalid; errs=%v err=%v", errs, err)
	}
}

// Provenance toolchain sessionDigest (2026-07 additive schema change).
func TestProvenanceSessionDigest(t *testing.T) {
	prov := func(digest map[string]any) map[string]any {
		return map[string]any{
			"createdAt": "2026-07-08T09:00:00Z",
			"createdBy": map[string]any{"type": "agent", "name": "demo-agent"},
			"toolchain": []any{map[string]any{
				"name": "demo-agent-harness", "version": "1.0.0", "sessionDigest": digest,
			}},
		}
	}
	mustValid(t, schema.Provenance, prov(map[string]any{
		"system": "demo-agent-harness", "sessionId": "sess-001",
		"transcriptSha256": shaA, "eventCount": 42,
	}))
	mustInvalid(t, schema.Provenance, prov(map[string]any{
		"system": "demo-agent-harness", "sessionId": "sess-001",
		"transcriptSha256": "XYZ", "eventCount": 42,
	}))
	mustInvalid(t, schema.Provenance, prov(map[string]any{
		"system": "demo-agent-harness", "transcriptSha256": shaA, "eventCount": 42,
	}))
}

// Fact evidence-ref anchors[] (2026-07 additive schema change).
func TestFactAnchors(t *testing.T) {
	fact := func(anchors []any) map[string]any {
		return map[string]any{
			"id": "f-001", "statement": "An observation.",
			"evidenceRefs": []any{map[string]any{"evidenceId": "ev-001", "anchors": anchors}},
		}
	}
	// The unified anchor vocabulary: composable optional fields, one anchor
	// per chunk.
	mustValid(t, schema.Fact, fact([]any{
		map[string]any{"quote": "verbatim excerpt"},
		map[string]any{"quote": "pin cite", "page": 7, "lines": map[string]any{"from": 12, "to": 14}},
		map[string]any{"lines": map[string]any{"from": 3}},
		map[string]any{"cells": map[string]any{"sheet": "Q1", "range": "B7:D9"}},
		map[string]any{"cells": map[string]any{"range": "B7"}},
		map[string]any{"charSpan": map[string]any{"start": 100, "end": 240}},
		map[string]any{"byteRange": map[string]any{"start": 0, "end": 1024}},
		map[string]any{"section": "2(a)", "jsonPath": "$.total"},
		map[string]any{"region": map[string]any{"x": 1, "y": 2, "width": 3, "height": 4}},
	}))

	// additionalProperties:false — unknown anchor fields fail.
	mustInvalid(t, schema.Fact, fact([]any{map[string]any{"paragraph": 3}}))
	mustInvalid(t, schema.Fact, fact([]any{map[string]any{"quote": "x", "type": "lineSpan"}}))
	// The retired typed-variant dialect is invalid.
	mustInvalid(t, schema.Fact, fact([]any{map[string]any{"type": "page", "page": 7}}))
	// Nested locators keep their required fields and closed shapes.
	mustInvalid(t, schema.Fact, fact([]any{map[string]any{"cells": map[string]any{"sheet": "Q1"}}}))
	mustInvalid(t, schema.Fact, fact([]any{map[string]any{"charSpan": map[string]any{"start": 100}}}))
	mustInvalid(t, schema.Fact, fact([]any{map[string]any{"lines": map[string]any{"from": 1, "at": 2}}}))
	// anchors must not be empty when present.
	mustInvalid(t, schema.Fact, fact([]any{}))

	// The retired singular anchor field is gone (additionalProperties:false
	// on the ref object).
	mustInvalid(t, schema.Fact, map[string]any{
		"id": "f-001", "statement": "An observation.",
		"evidenceRefs": []any{map[string]any{
			"evidenceId": "ev-002",
			"anchor":     map[string]any{"quote": "verbatim excerpt"},
		}},
	})

	// Plain string refs and anchor-less object refs stay valid.
	mustValid(t, schema.Fact, map[string]any{
		"id": "f-001", "statement": "An observation.",
		"evidenceRefs": []any{
			"ev-001",
			map[string]any{"evidenceId": "ev-002"},
			map[string]any{
				"evidenceId": "ev-003",
				"anchors":    []any{map[string]any{"quote": "verbatim excerpt", "page": 2}},
			},
		},
	})
}

// Analysis attestation (2026-07 additive schema change).
func TestAnalysisAttestationSchema(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{
			"id":          "att-001",
			"createdAt":   "2026-07-08T09:30:00Z",
			"performedBy": map[string]any{"type": "human", "name": "demo-reviewer"},
			"model": map[string]any{
				"provider": "demo-challenger", "name": "challenger-model-mini",
				"version": "2026-06-01", "role": "challenger",
			},
			"scope": map[string]any{
				"evidenceExamined": []any{map[string]any{"evidenceId": "ev-001", "sha256": shaA}},
				"claimsChallenged": []any{"c-001"},
			},
			"findings": []any{map[string]any{
				"id": "find-001", "severity": "medium", "summary": "A concern.",
				"requiresDisposition": true,
			}},
			"dispositions": []any{map[string]any{
				"findingId": "find-001", "decision": "acknowledged",
				"by": map[string]any{"name": "demo-manager"}, "at": "2026-07-08T16:00:00Z",
			}},
		}
	}
	mustValid(t, schema.AnalysisAttestation, valid())

	minimal := valid()
	delete(minimal, "findings")
	delete(minimal, "dispositions")
	mustValid(t, schema.AnalysisAttestation, minimal)

	noModelVersion := valid()
	delete(noModelVersion["model"].(map[string]any), "version")
	mustInvalid(t, schema.AnalysisAttestation, noModelVersion)

	badHash := valid()
	badHash["scope"].(map[string]any)["evidenceExamined"] = []any{
		map[string]any{"evidenceId": "ev-001", "sha256": "not-a-hash"},
	}
	mustInvalid(t, schema.AnalysisAttestation, badHash)

	noDisposition := valid()
	noDisposition["findings"].([]any)[0].(map[string]any)["requiresDisposition"] = false
	mustInvalid(t, schema.AnalysisAttestation, noDisposition)

	badSeverity := valid()
	badSeverity["findings"].([]any)[0].(map[string]any)["severity"] = "catastrophic"
	mustInvalid(t, schema.AnalysisAttestation, badSeverity)

	badDecision := valid()
	badDecision["dispositions"].([]any)[0].(map[string]any)["decision"] = "ignored"
	mustInvalid(t, schema.AnalysisAttestation, badDecision)
}

func TestExtensionSchemaValidation(t *testing.T) {
	errs, err := schema.ValidateExtension(
		"extensions/accounting-asc606/ssp-allocation.schema.json",
		map[string]any{
			"id":   "calc_x",
			"type": "relativeStandaloneSellingPriceAllocation",
			"inputs": map[string]any{
				"transactionPrice": 100,
				"components": []any{
					map[string]any{"id": "a", "standaloneSellingPrice": 125},
				},
			},
			"outputs": map[string]any{
				"allocations": []any{map[string]any{"componentId": "a", "allocatedAmount": 100}},
			},
		})
	if err != nil || len(errs) != 0 {
		t.Fatalf("ssp allocation should be valid: %v, %v", errs, err)
	}

	errs, err = schema.ValidateExtension(
		"extensions/accounting-asc606/ssp-allocation.schema.json",
		map[string]any{"id": "calc_x", "type": "somethingElse"})
	if err != nil || len(errs) == 0 {
		t.Fatalf("wrong-type allocation should be invalid: %v, %v", errs, err)
	}
	joined := strings.Join(errs, "; ")
	if !strings.Contains(joined, "inputs") && !strings.Contains(joined, "(root)") {
		t.Errorf("errors should mention missing fields: %v", errs)
	}
}

func TestForecastExtensionSchemaValidation(t *testing.T) {
	const path = "extensions/finance-forecast/forecast.schema.json"
	valid := func() map[string]any {
		return map[string]any{
			"subject":  "Canton Coin super validator rewards, node sv-main-1",
			"asOfDate": "2026-07-24",
			"unit":     "CC",
			"horizon": map[string]any{
				"start": "2026-08-01", "end": "2026-10-31", "periodization": "monthly",
			},
			"inputs": []any{
				map[string]any{
					"name": "current super validator weight", "factRef": "fact_sv_weight",
					"asOf": "2026-07-22", "maxAgeDays": 7,
					"refreshTrigger": "super validator set change",
				},
			},
			"scenarios": []any{
				map[string]any{
					"id": "scn_base", "label": "base", "calculationRef": "calc_base_projection",
					"assumptionJudgmentRefs": []any{"jdg_constant_sv_set"},
					"outputs": []any{
						map[string]any{"period": "2026-08", "value": "1250000", "low": "1100000", "high": "1400000"},
					},
				},
			},
			"backTest": map[string]any{
				"predecessorEvidenceRef": "ev_prior_forecast",
				"actualsFactRefs":        []any{"fact_actuals_q2"},
				"varianceCalculationRef": "calc_variance_q2",
			},
		}
	}

	errs, err := schema.ValidateExtension(path, valid())
	if err != nil || len(errs) != 0 {
		t.Fatalf("forecast register should be valid: %v, %v", errs, err)
	}

	noScenarios := valid()
	noScenarios["scenarios"] = []any{}
	if errs, err = schema.ValidateExtension(path, noScenarios); err != nil || len(errs) == 0 {
		t.Fatalf("empty scenarios should be invalid: %v, %v", errs, err)
	}

	noCalc := valid()
	delete(noCalc["scenarios"].([]any)[0].(map[string]any), "calculationRef")
	if errs, err = schema.ValidateExtension(path, noCalc); err != nil || len(errs) == 0 {
		t.Fatalf("scenario without calculationRef should be invalid: %v, %v", errs, err)
	}

	staleInput := valid()
	delete(staleInput["inputs"].([]any)[0].(map[string]any), "asOf")
	if errs, err = schema.ValidateExtension(path, staleInput); err != nil || len(errs) == 0 {
		t.Fatalf("input without asOf should be invalid: %v, %v", errs, err)
	}

	danglingBackTest := valid()
	delete(danglingBackTest["backTest"].(map[string]any), "predecessorEvidenceRef")
	if errs, err = schema.ValidateExtension(path, danglingBackTest); err != nil || len(errs) == 0 {
		t.Fatalf("backTest without predecessorEvidenceRef should be invalid: %v, %v", errs, err)
	}
}

func TestNamesCoverAllVendoredCoreSchemas(t *testing.T) {
	for _, name := range schema.Names() {
		mustInvalid(t, name, map[string]any{"unexpected": true})
	}
}
