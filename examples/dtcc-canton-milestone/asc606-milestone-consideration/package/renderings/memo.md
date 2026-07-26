# DTCC / Canton Coin Milestone Arrangement — Noncash Consideration Analysis (Illustrative)

**Entity:** DTCC (illustrative) · **Counterparty:** Canton Foundation · **Signing:** 2026-01-15
**Standard:** ASC 606 (noncash consideration) with fair value per the referenced ASC 820 package
**Status:** In review — two judgments and the referenced fair value are reviewRequired.

## The arrangement

DTCC may earn up to **10,000,000 Canton Coin** across three milestones:

| Milestone | Trigger | Coin | Consideration @ 0.0033 USD/CC |
|---|---|---:|---:|
| M1 | Joint press release | 2,000,000 | 6,600 |
| M2 | Production go-live on Canton | 6,000,000 | 19,800 |
| M3 | Cumulative settlement volume threshold | 2,000,000 | 6,600 |
| **Total** | | **10,000,000** | **33,000** |

## The mechanism that drives the accounting

This is an **on-chain, committee-gated** award, not an ordinary customer contract with goods transferred at signing:

- Coin is **not transferred at signing** — it is **unminted**, sitting in the network's Unclaimed Rewards pool.
- For each milestone, DTCC must **present proof of completion and a coin calculation** to the Tokenomics Working Group.
- The **Accountability Committee confirms** the milestone was met.
- Only then do **two-thirds of Super Validator operators mint** the approved coin to DTCC.

So at inception DTCC holds neither the asset nor control of it — only a **right contingent on a governance approval** outside its unilateral control (`c_not_transferred_at_signing`).

## Conclusions

**1. Characterization — noncash consideration for services (`j_arrangement_type`).** DTCC delivers value to the network (announcement, go-live, volume) in exchange for coin. Modeled as noncash consideration at fair value (ASC 606-10-32-21), not a pure grant. *Grant treatment recorded as the rejected alternative.*

**2. Measurement date — each milestone's approval/mint, not inception (`j_measurement_date`).** Because the coin is unminted and uncontrolled until committee confirmation and the two-thirds SV mint, the variable-consideration constraint (ASC 606-10-32-11) is not overcome, and no asset is obtained, until approval. Fair value is therefore struck **milestone-by-milestone on each approval/mint date**. *Fixing value at inception is recorded as the rejected alternative — it would recognize consideration DTCC cannot yet compel.*

**3. Amount (`c_consideration_amount`).** At the referenced Level 3 fair value of **0.0033 USD/CC**, total earnable consideration is **33,000 USD**, recognized as each milestone is approved (`calc_milestone_consideration`).

## The valuation hinge

The consideration amount is entirely driven by the per-coin fair value, concluded in the sibling **[ASC 820 package](../asc820-coin-fv/canton-coin-fv.vwp)** (`ev_coin_fv_package`):

- **Adopted:** 0.0033 USD/CC (Level 3, from thin OTC/Telegram evidence) → **33,000 USD**.
- **Rejected:** 0.0005 USD/CC network fee-reference price → would be only **5,000 USD**. That price is a fee-conversion parameter, not an exit price, and is not fair value.

The ~6.6× gap is the whole ballgame, and the OTC evidence supporting the higher figure is thin and self-reported — hence reviewRequired.

## What a reviewer must sign off

1. Arrangement characterization (`j_arrangement_type`) — technical-accounting reviewer.
2. Measurement date (`j_measurement_date`) — technical-accounting reviewer.
3. The referenced Level 3 per-coin fair value (`j_cc_fv_hierarchy` in the 820 package) — valuation reviewer.

Replace illustrative price/contract evidence with real captured artifacts before any reliance. This is not an audit, legal, or accounting opinion.
