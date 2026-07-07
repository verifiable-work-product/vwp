# ASC 606 revenue schedule — CSV column contract

**Status:** v0.1 draft, experimental.

The `vwp-accounting-asc606` extension expects an optional revenue schedule at
`calculations/revenue-schedule.csv`. CSV has no JSON Schema, so its contract is
stated here and enforced by the `asc606.revenueSchedule.total` validator.

## Columns

- `month` (required) — `YYYY-MM`, one row per period.
- `total` (required) — numeric; the total revenue recognized in the period.
  **The validator sums this column and requires it to equal the sum of
  `allocatedAmount` values in `calculations/ssp-allocation.json`.**
- Any number of additional numeric columns MAY break the total down by
  component (e.g. `subscription`, `support`, `implementation`). Component
  columns are informative in v0.1; only `total` is validated.

## Format rules

- First row is the header. Column names are case-sensitive and
  comma-separated; no quoted fields containing commas.
- Numeric cells are plain decimals (no currency symbols or thousands
  separators).
- Blank lines are ignored.

## Example

```csv
month,subscription,support,implementation,total
2026-01,8000,2000,0,10000
2026-02,8000,2000,0,10000
2026-03,8000,2000,60000,70000
```
