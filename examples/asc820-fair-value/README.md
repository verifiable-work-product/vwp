# Example: ASC 820 fair value support (synthetic)

A demo VWP package supporting fair value measurements for two fictional
digital asset positions as of 2026-06-30:

- **ETH, level 1** — 1,000 ETH at a 2,500 USD two-venue midpoint (2,500,000 USD), supported by exchange quote evidence.
- **LCK, level 3** — 100,000 transfer-restricted tokens at a thin-volume vendor quote of 2.00 USD less a 25% discount for lack of marketability (150,000 USD), with the hierarchy classification and DLOM recorded as an approved judgment (`j_lck_level3`).

**All data is synthetic. This example is not accounting or valuation advice.**

What it demonstrates:

- The `vwp-accounting-asc820` extension's structured **measurement register** at `facts/fair-value-measurements.json`, validating against `schemas/v0.1/extensions/accounting-asc820/fair-value-measurements.schema.json`.
- Extension validators: `asc820.measurements.schemaValid`, `asc820.pricingSources.resolve` (every measurement's pricing sources resolve to evidence), and `asc820.level3.judgmentLinked` (every level 3 measurement links a judgment — hierarchy leveling and unobservable inputs are judgment, not arithmetic).
- A judgment with alternatives and a considered range, approved by a valuation reviewer, with the approval scoped to the judgment ID.

## Try it

```bash
cd cli && npm install && npm run build
node dist/index.js pack ../examples/asc820-fair-value/package ../examples/asc820-fair-value/asc820-fair-value.vwp --write-hashes
node dist/index.js verify ../examples/asc820-fair-value/asc820-fair-value.vwp
node dist/index.js render ../examples/asc820-fair-value/asc820-fair-value.vwp
```
