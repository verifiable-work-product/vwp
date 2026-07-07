# VWP Validations

**Status:** v0.1 draft, experimental.

## 1. Overview

Validation objects record the results of deterministic checks run against a package:
schema validation, hash verification, reference-graph integrity, calculation
recomputation, and profile/extension rules. Producers SHOULD ship validation results
under `validations/` so consumers can see what was checked at production time; verifiers
re-run checks independently and MUST NOT rely on shipped results as proof. Validation
objects validate against `https://vwp.dev/schemas/v0.1/validation.schema.json`.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package (or within a verification report). |
| `validator` | string | yes | Identifier and version of the validator that ran (e.g. `vwp-core/refgraph@0.1`). |
| `scope` | string / object | no | What was checked: whole package, a part path, or specific object `id`s. |
| `status` | string | yes | One of `pass`, `fail`, `warning`, `reviewRequired`, `notRun`. |
| `severity` | string | no | e.g. `info`, `minor`, `major`, `critical`. Qualifies non-pass results. |
| `message` | string | no | Human-readable summary of the result. |
| `details` | object | no | Structured detail (offending paths, ids, expected vs actual values). |

## 3. Status semantics

| Status | Meaning | Effect on verification |
|---|---|---|
| `pass` | Check succeeded. | None. |
| `fail` | Check failed. | **Blocks verification.** The package MUST NOT be reported as verified. |
| `warning` | Check found a non-blocking concern. | Does not fail verification, unless the verifier runs in strict mode. |
| `reviewRequired` | Check identified something a human must review. | Does not fail verification, unless the verifier runs in strict mode. |
| `notRun` | Check was skipped or not applicable. | Informational only. |

Verifiers SHOULD offer a strict mode in which `warning` and `reviewRequired` results are
treated as failures.

## 4. Validator classes

- **Core validators** — always applicable: manifest schema, part hashes, path safety,
  object schemas, reference-graph integrity (no dangling `evidenceRefs`, fact/claim/
  judgment/calculation references).
- **Profile validators** — run when the manifest declares a profile: vocabulary
  membership (e.g. finance assertion types), domain consistency rules.
- **Extension validators** — run when the manifest declares an extension: narrow,
  domain-specific rules (see [profiles.md](profiles.md)).

Validators are deterministic. A validator MUST produce the same status for the same
package bytes, MUST NOT require network access or model inference, and MUST NOT execute
package contents.

## 5. Limitations

> VWP validation means that a package conforms to specified schemas and validation rules.
> It does not mean the professional conclusion is correct, complete, audited, legally
> sufficient, or appropriate for a particular use. Professional judgment, applicable
> standards, and independent review still govern reliance on any VWP package.

## 6. Example

```json
{
  "id": "val-004",
  "validator": "vwp-finance/reconciliation@0.1",
  "scope": { "calculationIds": ["calc-002"] },
  "status": "pass",
  "severity": "info",
  "message": "Reconciliation calc-002 recomputed; outputs match.",
  "details": { "recomputedDifference": "0.00" }
}
```
