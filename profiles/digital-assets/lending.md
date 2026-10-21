# Extension Sketch: `vwp-digital-assets-lending`

> **Status:** Experimental draft (v0.1). This is not accounting advice. All example
> data is synthetic. VWP validation is not an audit opinion, legal opinion, accounting
> opinion, or assurance engagement.

How a digital asset lending memo maps onto VWP objects: in-kind interest accrued on a
loan of crypto assets, collateral coverage, and the derecognition conclusion that
decides whether the lent asset leaves the balance sheet.

## Shape of a lending package

- **Facts** — the loan terms (principal, rate, day-count basis, dates), collateral
  positions and prices, the fair value of the fee asset.
- **Judgments** — whether control of the lent asset passed (ASC 860-10-40-5 and the
  AICPA Digital Assets Practice Aid analysis), and the loss basis for the unsecured
  portion.
- **Calculations** — `digitalAssetLoanInterestAccrual`
  ([`interest-accrual.schema.json`](../../schemas/v0.1/extensions/digital-assets-lending/interest-accrual.schema.json))
  per accrual window, and `collateralCoverage`
  ([`collateral-coverage.schema.json`](../../schemas/v0.1/extensions/digital-assets-lending/collateral-coverage.schema.json))
  with coverage ratio, margin shortfall and the unsecured portion (the ASC 326-20-35-6
  loss basis).
- **Claims** — derecognition (or not), accrual of the fee, collateral and credit loss.

## Validators (Go SDK)

| Id | Semantics |
|---|---|
| `lending.accrual.schemaValid` | Every accrual validates against the extension schema. |
| `lending.accrual.recompute` | Each accrual is principal × rate × days / basis, and those terms are what the cited terms fact states. |
| `lending.collateral.schemaValid` | Every coverage calculation validates against the extension schema. |
| `lending.collateral.recompute` | Coverage ratio, margin shortfall and unsecured portion recompute. |
| `lending.derecognition.claimJudged` | A claim concludes on derecognition, citing a derecognition paragraph family, and links a judgment — whether control passed is never arithmetic. |
| `lending.accountingSchedule.tieOut` | The executable schedule books one accrual per window, on its last accrued day, at the fair value the priced accrual states. |

## Executable accounting schedule

Packages under this extension are required to carry the executable accounting
schedule at `calculations/accounting-schedule.json` whenever the calculations book
anything (see [`accounting-schedule.md`](../accounting/accounting-schedule.md) for
the artifact contract and `core.requiredOutputs.exist`). The schedule's `kind`,
posting roles and amounts are tied out, entry by entry, to the extension's own
calculations by the `lending.accountingSchedule.tieOut` validator.

See [`examples/digital-asset-lending`](../../examples/digital-asset-lending).
