# Extension Sketch: Digital Asset Fair Value Support

> **Status:** Experimental stub (v0.1). All examples are synthetic. Nothing here is
> accounting advice. VWP validation is not an audit opinion, legal opinion, accounting
> opinion, or assurance engagement.

Structures fair value support for digital assets, complementing the general
[`vwp-accounting-asc820`](../accounting/asc820.md) sketch.

## Shape

- **Evidence** — pricing source extracts: exchange trade data, aggregator/vendor price
  files, order book snapshots. Each with source system, retrieval time, coverage
  window, and hash.
- **Judgments** — principal market determination; fair value hierarchy level; treatment
  of thinly traded or restricted tokens (typically `reviewRequired`).
- **Calculations** — quantity times price tie-outs per asset; portfolio fair value
  totals tied to reported balances; recomputable with declared rounding tolerance.
- **Claims** — "Asset A is measured at fair value X as of the measurement date,
  categorized Level N" (assertion: `valuationAllocation`), referencing `ASC 820` in
  `standardRefs` where applicable.

## Demo validators (illustrative)

- `dafv.price-source` — every fair value claim references pricing source evidence
  covering the measurement date.
- `dafv.tie-out` — fair value calculations recompute from declared quantities and
  prices and tie to claimed amounts.
- `dafv.level-judgment` — every fair value claim links to a hierarchy level judgment;
  Level 2/3 judgments surface in the validation summary.
