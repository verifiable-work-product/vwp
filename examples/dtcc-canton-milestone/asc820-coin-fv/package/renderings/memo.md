# Fair Value of Canton Coin at Signing — ASC 820 Support (Illustrative)

**Subject:** One Canton Coin (CC), measurement date **2026-01-15** (DTCC milestone-contract signing)
**Purpose:** Establish the per-coin fair value used as noncash consideration in the referencing ASC 606 milestone package.
**Status:** In review — the input-selection judgment requires valuation-reviewer approval.

## Conclusion

**Fair value = USD 0.0033 per Canton Coin, Level 3.**

## Two candidate inputs at signing

| Input | Value (USD/CC) | Nature | Use |
|---|---|---|---|
| Network fee-reference price | 0.0005 | Protocol-set fee-denomination parameter | **Rejected as FV input**; corroborating floor only |
| OTC secondary-market (Telegram) | 0.0030–0.0038 (settled 0.0032, 0.0033) | Informal arm's-length asks/bids/trades | **Adopted**, median settled = 0.0033 |

## Reasoning

At signing, Canton Coin was **not listed on any exchange** — no active market, so **no Level 1 input** (`c_no_active_market`).

The published **0.0005 USD/CC** figure is the only on-network number, but it is the price the network uses to convert its USD fee schedule into coin for paying gas. Under **ASC 820-10-20**, fair value is the **exit price** in an orderly transaction between market participants. A fee-conversion parameter has no order book, bid/ask, or settled trade behind it and is not such a price. It is therefore **rejected as a fair value input** and retained only as a corroborating floor (`c_gas_price_not_fv`).

The **OTC channel evidence** — informal, thin, and self-reported, but reflecting willing-buyer/willing-seller activity — is the best available exit-price evidence. Two self-reported settled trades (0.0032 and 0.0033) inside a 0.0030–0.0038 band support **0.0033 USD/CC**, classified **Level 3** and confidence-adjusted (`c_cc_fair_value`).

The adopted value is **~6.6× the fee-reference price**, which itself evidences that the fee price is not representative of market value.

## Limitations

The OTC evidence is Telegram-sourced, unverified as to counterparty and settlement, and thin. The input-selection judgment (`j_cc_fv_hierarchy`) is **reviewRequired** and pending valuation-reviewer approval. Replace the illustrative price evidence with real captured messages before any reliance. This is not an audit or valuation opinion.
