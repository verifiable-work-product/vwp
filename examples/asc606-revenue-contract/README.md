# Example: ASC 606 revenue contract (synthetic)

A demo VWP package for a revenue recognition workpaper covering a fictional
24-month, 300,000 USD contract between DemoCo and Acme Corp with three
components: a hosted SaaS subscription, implementation services, and premium
support, billed upfront.

**All data is synthetic. This example is not accounting advice.**

What it demonstrates:

- **Evidence** — an order form, an MSA excerpt, and a price book extract, indexed by `evidence/evidence.json`.
- **Facts** — contract terms and standalone selling prices extracted from the evidence.
- **Claims** — one claim per ASC 606 step (identify contract, identify performance obligations, determine transaction price, allocate, recognize), each linked to supporting facts, evidence, calculations, and judgments.
- **Judgment** — whether implementation services are distinct (`j_impl_distinct`), intentionally left in `reviewRequired` so `vwp verify` reports a review item.
- **Calculations** — a relative standalone selling price allocation (`calc_ssp_allocation`) and a monthly revenue schedule CSV; both are checked by the demo validators `asc606.allocation.tieOut` and `asc606.revenueSchedule.total`.
- **Analysis attestation** — `attestations/att-001.json`, a synthetic model-assisted challenge of the distinctness claim by a different model than the preparer's, with findings and human dispositions. Schema-checked by `core.attestations.schemaValid`; never affects package validity.
- **Rendering** — `renderings/memo.md`, a derived view of the structured package.

## Try it

```bash
cd cli && npm install && npm run build
node dist/index.js pack ../examples/asc606-revenue-contract/package ../examples/asc606-revenue-contract/acme-asc606.vwp --write-hashes
node dist/index.js verify ../examples/asc606-revenue-contract/acme-asc606.vwp
node dist/index.js inspect ../examples/asc606-revenue-contract/acme-asc606.vwp
node dist/index.js render ../examples/asc606-revenue-contract/acme-asc606.vwp
```

Expected verification summary: `valid with review items` (the distinctness
judgment is pending review by design).
