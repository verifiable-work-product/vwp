# VWP Finance Profile (`vwp-finance`)

> **Status:** Experimental draft (v0.1). This profile, like the rest of VWP, is under active
> development and subject to change.

The finance profile adds domain vocabulary to VWP Core for financial work products:
account reconciliations, revenue memos, valuation support, close packages, and audit
evidence bundles. VWP Core is domain-neutral; this profile supplies the terms and
validators that make a package meaningful in a financial reporting context.

## Assertion vocabulary

Finance claims may carry one or more of the following assertions. These correspond to
the management assertions commonly used in financial reporting and auditing, and they
are useful for mapping packaged evidence to audit assertions.

- **`existence`** — The asset, liability, or equity interest referenced by the claim
  actually exists as of the stated date. Example: a custody statement supporting that a
  reported balance exists at period end.
- **`occurrence`** — The recorded transaction or event actually took place and pertains
  to the entity. Example: a signed contract and delivery confirmation supporting a
  recorded sale.
- **`completeness`** — Everything that should have been recorded or disclosed has been.
  Example: a population reconciliation showing that all accounts or wallets in scope are
  included.
- **`valuationAllocation`** — Amounts are recorded at appropriate values and any
  allocations or adjustments are appropriately recorded. Example: pricing-source
  evidence and a recomputable fair value calculation.
- **`rightsAndObligations`** — The entity holds the rights to assets and owes the
  obligations recorded. Example: title documents, custody agreements, or
  proof-of-control evidence (with appropriate caveats).
- **`presentationDisclosure`** — Items are appropriately classified, described, and
  disclosed. Example: a mapping of balances to financial statement line items and
  required disclosures.

## Finance profile validators

Packages declaring the `vwp-finance` profile are expected to satisfy the following
validators in addition to VWP Core validation:

1. **Assertion coverage** — every finance claim should carry at least one assertion
   from the vocabulary above.
2. **Support coverage** — every claim should have at least one supporting fact,
   evidence reference, calculation, or judgment.
3. **Review surfacing** — every judgment with a `reviewRequired` status must surface in
   the package's validation summary so a human reviewer cannot miss it.

Validator failures do not mean a conclusion is wrong; they mean the package does not
demonstrate its support in a machine-checkable way.

## Scope and limitations

VWP validation is a structural and consistency check on a package. It is **not** an
audit opinion, a legal opinion, an accounting opinion, or an assurance engagement, and
passing validation does not make a conclusion correct or sufficient for any regulatory
purpose. All examples in this profile are synthetic.

See also: [`assertions.md`](./assertions.md), [`standards-map.md`](./standards-map.md).
