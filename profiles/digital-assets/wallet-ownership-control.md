# Extension Sketch: `vwp-digital-assets-wallet-ownership-control`

> **Status:** Experimental draft (v0.1). All examples are synthetic. Nothing here is
> accounting or legal advice. VWP validation is not an audit opinion, legal opinion,
> accounting opinion, or assurance engagement.

This extension structures the question "which wallets are the entity's, and how do we
know?" into claims, evidence, and deterministic validators.

## Claims

For a declared wallet population and period, the extension expects claims of these
kinds:

1. **Wallet exists** — the address exists on the stated network (assertion:
   `existence`). Support: on-chain lookup evidence with block height/timestamp.
2. **Entity controls wallet** — the entity holds signing authority over the wallet as
   of a stated time (assertion: `rightsAndObligations`). Support: proof-of-control
   evidence (signed message against a dated challenge), custodian attestations, or
   documented key-management records.
3. **Wallet included in population** — the wallet is in scope for the stated purpose
   (assertion: `completeness`). Support: population reconciliation against source
   systems.
4. **Balance ties to source** — the period-end balance for the wallet ties to on-chain
   or custodian source data (assertions: `existence`, `valuationAllocation`). Support:
   balance snapshot evidence and a tie-out calculation.
5. **Excluded wallets have rationale** — every wallet identified but excluded from
   scope carries a documented exclusion rationale (assertion: `completeness`).

## Assertions used

`rightsAndObligations`, `existence`, `completeness`, `valuationAllocation` — from the
finance profile vocabulary ([`../finance/assertions.md`](../finance/assertions.md)).

## Demo validators (illustrative)

- **`wallet.entity-mapping`** — every included wallet has an entity mapping.
- **`wallet.proof-of-control`** — every included self-custody wallet has
  proof-of-control evidence.
- **`wallet.exclusion-rationale`** — every excluded wallet has an exclusion rationale.
- **`wallet.balance-source`** — every period-end balance has source evidence
  (on-chain snapshot or custodian report) referenced by the balance claim.

These are structural checks: they verify the package demonstrates its support, not
that the underlying conclusions are correct.

### Data contract (implemented in the reference CLI)

The reference CLI implements these as `digitalAssets.wallets.entityMapping`,
`digitalAssets.wallets.proofOfControl`, `digitalAssets.wallets.exclusionRationale`,
and `digitalAssets.wallets.balanceEvidence`, preceded by
`digitalAssets.wallets.schemaValid`. They consume a structured wallet register at
`facts/wallets.json`, validating against
[`schemas/v0.1/extensions/digital-assets-wallet-ownership-control/wallets.schema.json`](../../schemas/v0.1/extensions/digital-assets-wallet-ownership-control/wallets.schema.json).
See [`examples/digital-assets-wallet-ownership`](../../examples/digital-assets-wallet-ownership).

## Caution: proof of control is not proof of ownership

Proof-of-control evidence, such as a signed wallet message, can demonstrate control of
a private key or signing authority at a point in time. It does not, by itself,
establish legal title, beneficial ownership, or accounting recognition. Those
conclusions require additional evidence and professional judgment.

In this extension, an ownership or recognition conclusion should be modeled as its own
claim, supported by proof-of-control evidence *plus* whatever additional evidence and
explicit `Judgment` objects the preparer relies on (agreements, legal analysis,
custody arrangements). Validators can check that the pieces are present; they cannot
weigh them.
