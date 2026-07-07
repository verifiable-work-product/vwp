# Example: Digital asset wallet ownership and control (synthetic)

A demo VWP package supporting wallet ownership/control conclusions for a
fictional entity, DemoCo Treasury LLC, for 2026-Q2. The population contains a
self-custody Ethereum wallet (included), a Bitcoin custodian account
(included), and an engineering test wallet (excluded with rationale).

**All data is synthetic: fictional wallets, placeholder addresses and
signatures, fictional custodian and balances.**

What it demonstrates:

- **Rights and obligations (control)** — a signed-message proof-of-control record for the self-custody wallet, plus an approved entity mapping register and a custodian statement.
- **Existence** — balance references to a (synthetic) block explorer snapshot and custodian statement.
- **Completeness** — every wallet in the inventory is included or excluded with a documented rationale.
- **Valuation tie-out** — period-end balances tie to source evidence; fair value is explicitly out of scope.
- **Extension validators** — `facts/wallets.json` is a structured register checked by the `vwp-digital-assets-wallet-ownership-control` demo validators: entity mapping, proof of control, exclusion rationale, and balance evidence.

## Important limitation

Proof-of-control evidence, such as a signed wallet message, can demonstrate
control of a private key or signing authority at a point in time. It does not,
by itself, establish legal title, beneficial ownership, or accounting
recognition. Those conclusions require additional evidence and professional
judgment.

## Try it

```bash
cd cli && npm install && npm run build
node dist/index.js pack ../examples/digital-assets-wallet-ownership/package ../examples/digital-assets-wallet-ownership/wallet-ownership.vwp --write-hashes
node dist/index.js verify ../examples/digital-assets-wallet-ownership/wallet-ownership.vwp
node dist/index.js render ../examples/digital-assets-wallet-ownership/wallet-ownership.vwp
```
