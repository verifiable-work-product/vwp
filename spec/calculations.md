# VWP Calculations

**Status:** v0.1 draft, experimental.

## 1. Overview

Calculations capture computations performed in producing the work product: totals,
reconciliations, variance analyses, threshold tests. A calculation declares its inputs,
outputs, and method so that a verifier can — ideally — recompute it and confirm the
result. Calculation objects live under `calculations/` and validate against
`https://vwp.dev/schemas/v0.1/calculation.schema.json`.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. Referenced by claims. |
| `type` | string | yes | Kind of calculation (e.g. `sum`, `reconciliation`, `variance`, `thresholdTest`, `custom`). |
| `inputs` | array / object | yes | Input values, or references to facts and evidence supplying them. |
| `outputs` | array / object | yes | Computed results. |
| `method` | string | no | Description or identifier of the computation method (formula, algorithm, spec reference). |
| `validator` | string | no | Identifier of a validator capable of recomputing this calculation. |
| `recomputable` | boolean | no | True when a verifier can recompute outputs from inputs deterministically. |

## 3. Recomputability

- Calculations SHOULD be recomputable: given `inputs` and `method`, an independent
  validator can reproduce `outputs` exactly. Recomputable calculations with a named
  `validator` are the strongest form of computational evidence in a package.
- When `recomputable` is true, `inputs` MUST be fully specified within the package —
  either inline values or references to facts/evidence that carry the values. A
  recomputable calculation with unresolved inputs is a defect; validators SHOULD report it.
- Recomputation is performed by trusted validator implementations identified by
  `validator`. Verifiers MUST NOT execute code shipped inside the package to recompute
  a calculation.
- Non-recomputable calculations (e.g. results imported from an external system) are
  permitted; they SHOULD set `recomputable: false` and cite the source as evidence.
- Numeric values SHOULD be represented as strings when decimal precision matters
  (currency amounts), to avoid floating-point drift across implementations.

## 4. Example

```json
{
  "id": "calc-002",
  "type": "reconciliation",
  "inputs": {
    "ledgerBalance": { "factRef": "fact-005" },
    "statementBalance": { "factRef": "fact-003" },
    "reconcilingItems": [{ "factRef": "fact-006" }]
  },
  "outputs": {
    "difference": "0.00",
    "reconciled": true
  },
  "method": "statementBalance - ledgerBalance - sum(reconcilingItems)",
  "validator": "vwp-finance/reconciliation@0.1",
  "recomputable": true
}
```
