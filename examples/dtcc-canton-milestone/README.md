# DTCC / Canton Coin Milestone Arrangement (illustrative)

Two linked VWP packages analyzing a milestone-based Canton Coin arrangement in which
DTCC may earn up to **10,000,000 Canton Coin** across three milestones (press release
2M, go-live 6M, volume threshold 2M). **All data is illustrative** — the milestone
mechanics are modeled on the public Canton Improvement Proposal framework (Accountability
Committee / Tokenomics Working Group / two-thirds Super Validator mint, e.g. the Nasdaq
CIP-0097 precedent); the OTC prices are synthetic stand-ins for Telegram-sourced messages.

## Why two packages

The consideration amount depends entirely on the per-coin fair value, and that
valuation is contested (a published network fee price vs. informal OTC quotes). VWP
keeps the two concerns separate and links them:

- **`asc820-coin-fv/`** — establishes the fair value of one Canton Coin at signing.
  Concludes **0.0033 USD/CC (Level 3)** from informal OTC/Telegram evidence, and
  **rejects the 0.0005 USD/CC network fee-reference ("gas") price** as a fee-conversion
  parameter that is not an exit price under ASC 820-10-20.
- **`asc606-milestone-consideration/`** — analyzes the arrangement and *consumes* the
  820 package's per-coin conclusion (as external evidence `ev_coin_fv_package`, pinned by
  SHA-256). Concludes the coin is unminted and committee-gated (no asset/control at
  inception), characterizes it as noncash consideration for services, and measures fair
  value at **each milestone's approval/mint date** rather than at inception.

## Key conclusions and the rejected alternatives

| Judgment | Adopted | Rejected alternative |
|---|---|---|
| Coin fair value input | OTC 0.0033 USD/CC, Level 3 | 0.0005 USD/CC gas price (not an exit price) |
| Measurement date | Each milestone approval/mint | Fixed at inception |
| Arrangement type | Noncash consideration for services | Nonmonetary grant |

At the adopted fair value the total consideration is **33,000 USD**; at the rejected gas
price it would be only **5,000 USD** — a ~6.6x swing that is the crux of the analysis.

Every judgment is `reviewRequired`; both packages verify as **valid with review items**.

## Reproduce

```bash
cd ../../cli
node dist/index.js verify ../examples/dtcc-canton-milestone/asc820-coin-fv/canton-coin-fv.vwp
node dist/index.js verify ../examples/dtcc-canton-milestone/asc606-milestone-consideration/dtcc-milestone.vwp
node dist/index.js render ../examples/dtcc-canton-milestone/asc606-milestone-consideration/dtcc-milestone.vwp
```

If you re-pack the 820 package its hash changes; update `sha256` on `ev_coin_fv_package`
in the 606 package's `evidence/evidence.json`, then re-pack the 606 package.

> Not an audit, legal, or accounting opinion. VWP validation checks structure, integrity,
> and declared rules only. Replace all illustrative evidence with real captured artifacts
> before any reliance.
