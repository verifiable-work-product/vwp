# Wallet Ownership and Control Workpaper - DemoCo Treasury LLC, 2026-Q2 (SYNTHETIC DEMO)

> **Synthetic demo data.** All entities, wallets, addresses, balances, and
> signatures are fictional. This rendering is a derived view; the structured
> package is the source of truth.

- **Entity:** DemoCo Treasury LLC
- **Period:** 2026-04-01 through 2026-06-30
- **Status:** In review

## Population

| Wallet / account | Chain | Custody | Included | Period-end balance |
|---|---|---|---|---:|
| `wallet_eth_treasury` | Ethereum | Self-custody | Yes | 1,250.5 ETH |
| `acct_btc_custodian` | Bitcoin | Demo Custody Inc. | Yes | 85.25 BTC |
| `wallet_test_legacy` | Ethereum | Self-custody | No — test wallet | 0 ETH |

## Procedures and support

1. **Existence.** The Ethereum wallet's period-end balance is referenced
   against a block explorer snapshot (`ev_explorer_snapshot`); the custodian
   holding appears on the Demo Custody Inc. statement
   (`ev_custodian_statement`). (Claims `cl_eth_wallet_exists`,
   `cl_btc_custodian_holding`.)
2. **Rights and obligations (control).** Treasury operations obtained and
   verified a signed message over a dated challenge nonce for the self-custody
   wallet (`ev_signed_message`). The custodian account is held under a custody
   agreement for the benefit of the entity. Both are mapped to DemoCo Treasury
   LLC in the approved entity mapping register (`ev_entity_mapping`). (Claims
   `cl_eth_wallet_control`, `cl_btc_custodian_holding`.)
3. **Completeness.** Every wallet in the inventory is either included in the
   population or excluded with a documented rationale. `wallet_test_legacy` is
   excluded as an engineering test wallet, never funded by treasury
   operations, with a zero period-end balance. (Claims
   `cl_population_complete`, `cl_exclusion_documented`.)
4. **Valuation / allocation (tie-out).** Period-end balances tie to source
   evidence. Fair value measurement is a downstream work product and out of
   scope here. (Claim `cl_balances_tie`.)

## Validation summary

All four extension validators pass: entity mapping, proof of control,
exclusion rationale, and balance evidence.

## Important limitations

Proof-of-control evidence, such as a signed wallet message, can demonstrate
control of a private key or signing authority at a point in time. It does not,
by itself, establish legal title, beneficial ownership, or accounting
recognition. Those conclusions require additional evidence and professional
judgment. In this demo, legal ownership support rests on the entity mapping
register, treasury policy, and the custody agreement.

VWP validation means that a package conforms to specified schemas and
validation rules. It does not mean the professional conclusion is correct,
complete, audited, legally sufficient, or appropriate for a particular use.
Professional judgment, applicable standards, and independent review still
govern reliance on any VWP package.
