# Executable Accounting Schedule (required extension output)

> **Status:** Experimental draft (v0.1). This is not accounting advice. All example
> data is synthetic. VWP validation is not an audit opinion, legal opinion, accounting
> opinion, or assurance engagement.

Several accounting extensions conclude in something a ledger must book: revenue
recognized per period, a fair value remeasurement, an impairment loss, staking
income, an in-kind interest accrual, a realized gain or loss on disposal. VWP makes
that conclusion an **executable, versioned artifact** rather than prose, so an
approval service or ledger can post it without re-deriving it, and so a verifier can
prove the artifact books exactly what the package's calculations say.

## The artifact

`calculations/accounting-schedule.json`, declared in `manifest.parts` with role
`calculation`, media type `application/json` and a SHA-256, validating against
[`sdk/go/vwp/accountingschedule/schema.json`](../../sdk/go/vwp/accountingschedule/schema.json)
(`schemaVersion` 1):

| Field | Meaning |
|---|---|
| `name`, `kind` | A label and one of `revenue_recognition`, `fair_value_remeasurement`, `impairment`, `staking_rewards`, `interest_accrual`, `realized_gain_loss` (`depreciation` is reserved). |
| `currency`, `amountScale` | ISO 4217 code and the decimal precision every amount is written at (0–8). |
| `startDate`, `endDate`, `totalAmount` | The schedule's period and the sum of all entry debits. |
| `entries[]` | `id`, `date` (within the period), `description`, optional `obligationId`, and 2–100 `postings`. |
| `postings[]` | `accountRole` from the kind's fixed vocabulary, exactly one of `debit` or `credit` as a positive decimal at `amountScale`, optional `dimensions`. |

Amounts are decimal strings, never floats. Every entry must balance, duplicate keys and
unknown fields are rejected, and the artifact is bounded (1000 entries, 2 MiB).
Account **roles** (for example `deferred_revenue` / `revenue`, `crypto_asset` /
`crypto_asset_gain_loss`, `intangible_asset` / `impairment_loss`,
`staking_reward_receivable` / `staking_reward_income`, `interest_receivable` /
`interest_income`, `disposal_proceeds` / `realized_gain_loss`) are bound to real ledger
accounts by a reviewer when the schedule is created; fixing the vocabulary per kind is
what lets a verifier prove the *direction* of every entry.

## Required outputs

Each extension declares its mandatory outputs (`RequiredOutputs` in the Go SDK). Core
verification enforces the contract:

- **`core.requiredOutputs.exist`** — every unconditional required output is present and
  declared exactly once in `manifest.parts` with the right role, media type and hash;
  conditional outputs are checked for correct binding when present.
- **`accountingSchedule.valid`** — the artifact parses and satisfies every structural
  rule above. Reported whenever the artifact is present, and whenever the extension
  requires it unconditionally.
- **`<ext>.accountingSchedule.tieOut`** — the extension-specific proof that the
  schedule books exactly what the calculations say, and nothing else.

| Extension | Required outputs | Condition |
|---|---|---|
| `vwp-accounting-asc606` | `calculations/ssp-allocation.json`, `calculations/revenue-schedule.csv`, `calculations/accounting-schedule.json` (`revenue_recognition`) | always |
| `vwp-accounting-asc350-60` | accounting schedule (`fair_value_remeasurement`) | any roll-forward line carries a remeasurement gain or loss |
| `vwp-accounting-asc350-30` | accounting schedule (`impairment`) | any tested lot has an impairment loss |
| `vwp-digital-assets-staking-rewards` | accounting schedule (`staking_rewards`) | any reward has a fair value |
| `vwp-digital-assets-lending` | accounting schedule (`interest_accrual`) | any accrual window accrues a fee |
| `vwp-digital-assets-cost-basis` | accounting schedule (`realized_gain_loss`) | any lot relief schedule records a disposal |

A package with nothing to book carries no schedule; the tie-out validator says so.
