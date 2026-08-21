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

## 5. Conventional calculation types

Four `type` values carry format-level recomputation semantics: any conforming
validator (the benchmark harness and the platform both implement these) can
recompute `outputs` from `inputs` with no other context. A calculation using
one of these types MUST use the input/output shapes below and MUST reproduce
within an absolute tolerance of **0.01** per compared number (decimal-string
inputs are parsed exactly; the tolerance absorbs cent-rounding, not errors).
Numbers may be JSON numbers or decimal strings (`"1234.56"`; thousands
separators permitted). A conventional-typed calculation is recomputable by
construction; validators recompute it regardless of the `recomputable` flag,
and a mismatch is a validation failure. Anything that does not fit these
shapes belongs under another type (e.g. `custom`), not force-fitted.

### 5.1 `sum`

- **Inputs:** `values` — non-empty array of numbers.
- **Outputs:** `total` — number.
- **Recompute:** `total = Σ values[i]`.

### 5.2 `schedule`

A rows × columns grid (e.g. a revenue schedule: rows = performance
obligations, columns = months).

- **Inputs:** `rows` — non-empty array of `{ label?, values: number[] }`.
- **Outputs:** `grandTotal` — number (required); `rowTotals` — number[]
  (optional, one per row); `columnTotals` — number[] (optional, one per
  column; ragged rows treat missing cells as 0).
- **Recompute:** `rowTotals[i] = Σ rows[i].values`, `columnTotals[j] = Σ_i
  rows[i].values[j]`, `grandTotal = Σ rowTotals`. Declared vectors are
  compared element-wise; `grandTotal` is always compared.

### 5.3 `journalEntries`

- **Inputs:** `entries` — non-empty array of
  `{ date?, description?, lines: [{ account, debit?, credit? }] }`, each with
  a non-empty `lines` array; omitted `debit`/`credit` count as 0.
- **Outputs:** `totalDebits`, `totalCredits` — numbers (both required).
- **Recompute:** `totalDebits = Σ lines[].debit`, `totalCredits =
  Σ lines[].credit` across all entries.

### 5.4 `relativeStandaloneSellingPriceAllocation`

Transaction price allocated pro rata over standalone selling prices
(e.g. ASC 606-10-32-31).

- **Inputs:** `transactionPrice` — number; `items` — non-empty array of
  `{ name?, ssp }`.
- **Outputs:** `allocations` — array of `{ name?, allocatedAmount }`, exactly
  one per input item, in input order.
- **Recompute:** `allocations[i].allocatedAmount = transactionPrice *
  items[i].ssp / Σ items[].ssp`, plus the tie-out
  `Σ allocations[].allocatedAmount = transactionPrice` (both within
  tolerance).
