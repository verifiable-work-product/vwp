# VWP Facts

**Status:** v0.1 draft, experimental.

## 1. Overview

Facts are extracted observations, not conclusions. A fact states something directly
observable in evidence — a balance on a statement, a signer address on a transaction, a
clause in a contract — together with where it came from and how it was extracted. Claims
then build on facts. Fact objects live under `facts/` and validate against
`https://vwp.dev/schemas/v0.1/fact.schema.json`.

Keeping facts separate from claims lets a reviewer or validator check the ground truth
layer independently: is each fact actually supported by the cited evidence, before asking
whether the claims follow from the facts.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. Referenced by claims. |
| `statement` | string | yes | Human-readable statement of the observation. |
| `value` | any | no | Structured value of the observation (number, string, object), when one exists. |
| `evidenceRefs` | array of string | yes | `id`s of evidence objects the fact was extracted from. SHOULD be non-empty. |
| `extractionMethod` | string | no | How the fact was extracted (e.g. `manual`, `ocr`, `parser`, `llm`, `apiField`). |
| `confidence` | number | no | Extraction confidence in `[0, 1]`. Confidence in the extraction, not in any conclusion. |
| `requiresReview` | boolean | no | True when a human should confirm the extraction before reliance. Defaults to false. |

## 3. Semantics

- A fact MUST be an observation attributable to evidence. Interpretations, inferences, and
  conclusions belong in claims or judgments, not facts.
- `evidenceRefs` MUST resolve to evidence `id`s present in the package. Verifiers MUST
  fail on dangling references.
- `value` SHOULD be provided whenever the observation has a natural structured form, so
  calculations and validators can consume it without re-parsing `statement`.
- Facts extracted by automated means (including models) SHOULD set `extractionMethod` and
  `confidence`, and SHOULD set `requiresReview: true` when confidence is low or the fact
  is material to the conclusion. Whether `requiresReview` facts block acceptance is a
  matter for profile validators and organizational policy, not core verification.

## 4. Example

```json
{
  "id": "fact-003",
  "statement": "The closing balance of the operating account on 2026-06-30 was USD 1,250,344.18.",
  "value": { "currency": "USD", "amount": "1250344.18", "asOf": "2026-06-30" },
  "evidenceRefs": ["ev-001"],
  "extractionMethod": "llm",
  "confidence": 0.97,
  "requiresReview": false
}
```
