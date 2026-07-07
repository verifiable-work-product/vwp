# VWP Claims

**Status:** v0.1 draft, experimental.

## 1. Overview

Claims are the assertions a work product makes. Each claim states a proposition, links to
the facts, evidence, calculations, and judgments that support it, and records a
conclusion and status. Claims live in `claims/claims.json` (required for useful packages)
and validate against `https://vwp.dev/schemas/v0.1/claim.schema.json`.

Claims form a graph: claims reference supporting facts, evidence, calculations, and
judgments by `id`, and reference external standards by citation. Verifiers check the
integrity of this graph; profile validators check its domain content.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. |
| `statement` | string | yes | The proposition being asserted. |
| `domain` | string | no | Domain tag (e.g. `finance`, `security`, `operations`). |
| `standardRefs` | array | no | Citations to external standards, policies, or criteria the claim is evaluated against. |
| `assertions` | array of string | no | Assertion types addressed, from a profile vocabulary (see section 4). |
| `supportingFacts` | array of string | no | Fact `id`s supporting the claim. |
| `supportingEvidence` | array of string | no | Evidence `id`s cited directly. |
| `supportingCalculations` | array of string | no | Calculation `id`s supporting the claim. |
| `judgments` | array of string | no | Judgment `id`s the claim depends on. |
| `conclusion` | string | no | The conclusion reached for this claim. |
| `status` | string | no | e.g. `supported`, `notSupported`, `inconclusive`, `withdrawn`. |
| `requiresReview` | boolean | no | True when human review of the claim is outstanding. |

## 3. Reference integrity

All `supportingFacts`, `supportingEvidence`, `supportingCalculations`, and `judgments`
entries MUST resolve to objects present in the package. Verifiers MUST fail on dangling
references. A claim with no supporting references is schema-valid but weak; profile
validators SHOULD flag unsupported claims (`warning` or `reviewRequired`).

## 4. Assertion vocabularies

`assertions` values come from the profile declared in the manifest. The `vwp-finance`
profile defines:

`existence`, `occurrence`, `completeness`, `valuationAllocation`,
`rightsAndObligations`, `presentationDisclosure`

Other profiles define their own vocabularies (see [profiles.md](profiles.md)). Core
verification treats `assertions` as opaque strings; profile validators enforce membership
in the declared vocabulary.

## 5. Example

```json
{
  "id": "claim-001",
  "statement": "The entity controls wallet 0xabc… as of 2026-06-30.",
  "domain": "finance",
  "standardRefs": [{ "standard": "internal-policy", "section": "DA-4.2" }],
  "assertions": ["existence", "rightsAndObligations"],
  "supportingFacts": ["fact-003", "fact-007"],
  "supportingEvidence": ["ev-001", "ev-004"],
  "supportingCalculations": ["calc-002"],
  "judgments": ["jdg-001"],
  "conclusion": "Control demonstrated via signed message and observed outbound transfer.",
  "status": "supported",
  "requiresReview": false
}
```
