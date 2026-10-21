# Extension Sketch: `vwp-digital-assets-staking-rewards`

> **Status:** Experimental draft (v0.1). This is not accounting advice. All example
> data is synthetic. VWP validation is not an audit opinion, legal opinion, accounting
> opinion, or assurance engagement.

How a staking rewards memo maps onto VWP objects: rewards recognized as income at
fair value on their recognition date, reconciled to what the chain actually paid.

## Shape of a staking package

- **Facts** — the staking arrangement and its terms, the chain-observed receipts for
  the period, the unit price as of each measurement date (with pricing evidence).
- **Judgments** — the recognition basis (inception, recognition or receipt) and the
  unit of account for rewards not yet received.
- **Calculations** — one or more `stakingRewardsSchedule` calculations
  ([`rewards-schedule.schema.json`](../../schemas/v0.1/extensions/digital-assets-staking-rewards/rewards-schedule.schema.json)):
  every reward with units, measurement date, unit price, fair value, recognition and
  receipt dates, and the reconciliation of recognized units to chain receipts.
- **Claims** — occurrence, completeness and measurement of reward income.

## Validators (Go SDK)

| Id | Semantics |
|---|---|
| `staking.rewards.schemaValid` | Every schedule validates against the extension schema. |
| `staking.rewards.recompute` | Each row's fair value is units × unit price and the totals sum. |
| `staking.rewards.pricingEvidence` | Each row's measurement date is the date its `pricingDateBasis` fixes, and the cited price fact states that price as of that date. |
| `staking.rewards.chainTieOut` | Recognized units reconcile to the chain-observed receipts through the declared unreceived opening/closing balances and reconciling items. |
| `staking.rewards.population` | Recognition dates lie in the schedule period; reward references are unique per asset and arrangement; schedules of one arrangement never overlap. |
| `staking.accountingSchedule.tieOut` | The executable schedule books each reward as income on its recognition date (to the asset when received, otherwise to a receivable transferred on receipt). |

## Executable accounting schedule

Packages under this extension are required to carry the executable accounting
schedule at `calculations/accounting-schedule.json` whenever the calculations book
anything (see [`accounting-schedule.md`](../accounting/accounting-schedule.md) for
the artifact contract and `core.requiredOutputs.exist`). The schedule's `kind`,
posting roles and amounts are tied out, entry by entry, to the extension's own
calculations by the `staking.accountingSchedule.tieOut` validator.

See [`examples/staking-rewards`](../../examples/staking-rewards).
