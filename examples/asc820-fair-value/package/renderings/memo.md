# Fair Value Support Memo - DemoCo Digital Asset Positions, 2026-06-30 (SYNTHETIC DEMO)

> **Synthetic demo data.** All positions, prices, vendors, and agreements are
> fictional. This memo is not accounting advice. It is a derived rendering;
> the structured package is the source of truth.

- **Entity:** DemoCo
- **Measurement date:** 2026-06-30
- **Standard:** ASC 820
- **Status:** In review

## Measurements

| Position | Quantity | Level | Technique | Fair value (USD) |
|---|---:|---|---|---:|
| ETH | 1,000 | Level 1 | Unadjusted quoted prices, active markets (two-venue midpoint) | 2,500,000 |
| LCK (restricted) | 100,000 | Level 3 | Market approach, 25% discount for lack of marketability | 150,000 |

## ETH — level 1

Last-trade prices at the measurement date were 2,501.00 USD (demo-exchange-a)
and 2,499.00 USD (demo-exchange-b); the workpaper uses the 2,500.00 USD
midpoint (evidence `ev_exchange_quotes`). 1,000 ETH × 2,500.00 = 2,500,000
USD. (Claim `cl_eth_fair_value`.)

## LCK — level 3

The pricing vendor's indicative quote for *unrestricted* LCK tokens was 2.00
USD in thin volume (`ev_vendor_quote`). DemoCo's tokens are transfer-restricted
until 2027-06-30 with monthly release thereafter (`ev_token_agreement`), so
the measurement applies a 25% discount for lack of marketability:
100,000 × 2.00 × (1 − 0.25) = 150,000 USD.

The hierarchy classification and discount are professional judgments recorded
as `j_lck_level3`: level 3 because the marketability adjustment is a
significant unobservable input; DLOM of 25% selected within a considered range
of 15%–35%; approved by the valuation reviewer on 2026-07-05. (Claim
`cl_lck_fair_value`.)

## Validation summary

All three `vwp-accounting-asc820` extension validators pass: register schema,
pricing source resolution, and level 3 judgment linkage.

---

VWP validation means that a package conforms to specified schemas and
validation rules. It does not mean the professional conclusion is correct,
complete, audited, legally sufficient, or appropriate for a particular use.
Professional judgment, applicable standards, and independent review still
govern reliance on any VWP package.
