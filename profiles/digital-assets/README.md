# VWP Digital Asset Extensions

> **Status:** Experimental draft (v0.1). All examples are synthetic. Nothing in these
> documents is accounting, legal, or tax advice. VWP validation is not an audit
> opinion, legal opinion, accounting opinion, or assurance engagement.

Digital asset extensions apply the finance profile to crypto and tokenized-asset
subject matter, where evidence is often programmatically retrievable (on-chain data,
exchange APIs, custodian reports) and therefore well suited to structured, hash-pinned
packaging.

## Extension areas

Digital asset extensions may cover, among other areas:

- **Wallet ownership and control** — which wallets exist, which the entity controls,
  and why wallets are included in or excluded from scope. See
  [`wallet-ownership-control.md`](./wallet-ownership-control.md).
- **Balance tie-outs** — period-end on-chain and exchange balances tied to subledger
  and general ledger balances. See [`balance-tie-out.md`](./balance-tie-out.md).
- **Fair value support** — pricing sources and valuation tie-outs for digital assets.
  See [`fair-value-support.md`](./fair-value-support.md).
- **Staking rewards** — reward accrual and receipt, with on-chain support.
- **Cost basis** — lot tracking, disposition methodology, and gain/loss calculations.
- **Subledger reconciliation** — completeness of transaction populations between chain
  data, subledger, and ledger.

## Caution: proof of control is not proof of ownership

Proof-of-control evidence, such as a signed wallet message, can demonstrate control of
a private key or signing authority at a point in time. It does not, by itself,
establish legal title, beneficial ownership, or accounting recognition. Those
conclusions require additional evidence and professional judgment.

Extensions in this family model that distinction explicitly: control demonstrations are
evidence items supporting `rightsAndObligations`-tagged claims, and ownership or
recognition conclusions are separate claims that require additional support and,
typically, explicit judgments.

## Safety note

Never include real private keys, seed phrases, or credentials in a VWP package. Signed
messages used as proof-of-control evidence should be generated with dedicated
challenge text and must never embed secret material. See the repository
[`SECURITY.md`](../../SECURITY.md).
