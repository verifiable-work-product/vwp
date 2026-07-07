# Finance Profile: Assertions

> **Status:** Experimental draft (v0.1). All examples are synthetic.

This document describes each assertion in the `vwp-finance` vocabulary and the kinds of
evidence that typically support it. The lists are illustrative, not exhaustive, and
inclusion of a given evidence type in a package does not make that evidence sufficient
or appropriate for any particular purpose. VWP validation is not an audit opinion,
legal opinion, accounting opinion, or assurance engagement.

## `existence`

**Definition:** The asset, liability, or equity interest referenced by a claim exists
as of a stated date.

**Evidence that typically supports it:**

- Third-party statements (bank, broker, custodian) as of period end
- Confirmations from counterparties
- On-chain balance snapshots with block height and timestamp
- Physical inspection records or system-of-record extracts

## `occurrence`

**Definition:** A recorded transaction or event took place during the period and
pertains to the entity.

**Evidence that typically supports it:**

- Executed contracts or purchase orders
- Invoices, delivery confirmations, or service acceptance records
- Transaction hashes and settlement records
- System logs showing the event with timestamps

## `completeness`

**Definition:** All transactions, balances, and disclosures that should have been
recorded or included are recorded or included.

**Evidence that typically supports it:**

- Population reconciliations (source system vs. ledger vs. package scope)
- Sequential-numbering or gap analyses
- Cutoff testing evidence around period end
- Exclusion lists with documented rationale for anything out of scope

## `valuationAllocation`

**Definition:** Amounts are recorded at appropriate values, and allocations,
adjustments, and impairments are appropriately recorded.

**Evidence that typically supports it:**

- Pricing-source extracts (exchange prices, broker quotes, index data)
- Recomputable calculations with declared inputs, outputs, and methods
- Standalone selling price analyses or allocation schedules
- Impairment or reserve analyses with stated methodology

## `rightsAndObligations`

**Definition:** The entity holds or controls the rights to assets, and liabilities are
the obligations of the entity.

**Evidence that typically supports it:**

- Title documents, custody agreements, and account agreements
- Legal opinions or counsel memos (referenced, not asserted by VWP)
- Proof-of-control evidence such as signed wallet messages — note that control of a key
  is not the same as legal title or beneficial ownership; see the digital-assets
  profile for the required caution
- Lease agreements, loan agreements, and other executed instruments

## `presentationDisclosure`

**Definition:** Items are appropriately aggregated or disaggregated, clearly described,
classified, and disclosed.

**Evidence that typically supports it:**

- Mappings of balances to financial statement line items
- Disclosure checklists and drafted footnotes
- Classification memos (e.g., current vs. non-current, operating vs. finance)
- Prior-period comparatives and tie-outs to reported figures
