# Extension Sketch: `vwp-digital-assets-cost-basis`

> **Status:** Experimental draft (v0.1). This is not accounting advice. All example
> data is synthetic. VWP validation is not an audit opinion, legal opinion, accounting
> opinion, or assurance engagement.

How a cost basis / lot relief memo maps onto VWP objects: lots acquired, lots relieved
on disposal under a declared method, the realized gain or loss, and the Rev. Proc.
2024-28 wallet-by-wallet allocation of unused basis as of 2025-01-01.

## Shape of a cost basis package

- **Facts** — the lot population per asset and pool (acquisition dates, units, basis),
  each disposal (date, units, proceeds), and wallet balances at the safe-harbor date.
- **Judgments** — the relief method (FIFO, specific identification, …) and the unit of
  account per pool.
- **Calculations** — `lotRelief`
  ([`lot-relief.schema.json`](../../schemas/v0.1/extensions/digital-assets-cost-basis/lot-relief.schema.json))
  per asset and pool, and `walletBasisAllocation`
  ([`wallet-basis-allocation.schema.json`](../../schemas/v0.1/extensions/digital-assets-cost-basis/wallet-basis-allocation.schema.json)).
- **Claims** — completeness of the lot population, measurement of relieved basis and
  realized gain/loss, eligibility of the allocation.

## Validators (Go SDK)

| Id | Semantics |
|---|---|
| `costBasis.lotRelief.schemaValid` | Every lot relief schedule validates against the extension schema. |
| `costBasis.lotRelief.unitsConserved` | Units are conserved from the lot population through every relief into the remaining lots. |
| `costBasis.lotRelief.recompute` | Basis relieved and realized gain/loss recompute from the lots; no disposal relieves a lot acquired after it. |
| `costBasis.lotRelief.methodOrder` | The relief order follows the declared method. |
| `costBasis.walletAllocation.schemaValid` | Every allocation validates against the extension schema. |
| `costBasis.walletAllocation.tieOut` | Each allocation ties to its declared basis and units. |
| `costBasis.walletAllocation.eligibility` | The allocation is dated 2025-01-01 and equals the units and basis of lots acquired before it (warns when no lot population is present to tie to). |
| `costBasis.accountingSchedule.tieOut` | The executable schedule books one entry per disposal: proceeds in, cost basis out (recomputed from the lots), the difference as realized gain or loss. |

## Executable accounting schedule

Packages under this extension are required to carry the executable accounting
schedule at `calculations/accounting-schedule.json` whenever the calculations book
anything (see [`accounting-schedule.md`](../accounting/accounting-schedule.md) for
the artifact contract and `core.requiredOutputs.exist`). The schedule's `kind`,
posting roles and amounts are tied out, entry by entry, to the extension's own
calculations by the `costBasis.accountingSchedule.tieOut` validator.

See [`examples/cost-basis-lot-relief`](../../examples/cost-basis-lot-relief).
