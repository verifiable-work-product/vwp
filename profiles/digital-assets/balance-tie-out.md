# Extension Sketch: Digital Asset Balance Tie-Out

> **Status:** Experimental stub (v0.1). All examples are synthetic. VWP validation is
> not an audit opinion, legal opinion, accounting opinion, or assurance engagement.

Structures the tie-out of period-end digital asset balances across chain, custodian,
subledger, and general ledger.

## Shape

- **Evidence** — on-chain balance snapshots (address, network, block height,
  timestamp), custodian and exchange statements, subledger extracts, trial balance
  extracts. Each hash-pinned with source system and retrieval time.
- **Facts** — per-wallet, per-asset balances extracted from each source.
- **Calculations** — recomputable aggregations: wallet balances roll up to subledger
  totals; subledger totals tie to GL balances; differences itemized with declared
  tolerances.
- **Claims** — "Subledger balance for asset A ties to on-chain sources at period end"
  (assertions: `existence`, `completeness`, `valuationAllocation`), with reconciling
  items as their own claims or judgments when they require explanation.

## Demo validators (illustrative)

- `tieout.rollup-recomputes` — aggregation calculations recompute from per-wallet
  facts.
- `tieout.gl-ties` — subledger totals tie to GL balances within declared tolerance, or
  every difference is covered by a reconciling-item claim.
- `tieout.snapshot-coverage` — every wallet in the population has a period-end balance
  snapshot evidence item.
