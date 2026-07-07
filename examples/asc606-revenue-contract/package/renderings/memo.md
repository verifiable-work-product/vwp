# Revenue Recognition Memo - Acme Corp Contract (SYNTHETIC DEMO)

> **Synthetic demo data.** This memo is a rendering of a VWP demo package. It is
> not accounting advice, and the underlying contract, amounts, and parties are
> fictional. This rendering is a derived view; the structured package is the
> source of truth.

- **Entity:** DemoCo
- **Subject:** Customer contract `contract_acme_001` (Acme Corp)
- **Standard:** ASC 606
- **Status:** Draft — review required
- **Prepared:** 2026-07-07

## Background

DemoCo executed Order Form #OF-2026-0142 with Acme Corp dated 2026-01-01 for a
24-month arrangement totaling 300,000 USD, billed upfront, comprising a hosted
SaaS subscription (200,000 USD), implementation services (55,000 USD), and
premium support (45,000 USD). (Evidence: `ev_order_form`, `ev_msa_excerpt`.)

## Five-step analysis

1. **Identify the contract.** A contract exists as of 2026-01-01: approved
   order form, identified rights and payment terms, commercial substance, and
   probable collection. (Claim `c_step1_contract_identified`.)
2. **Identify performance obligations.** Three promised services were
   identified. The SaaS subscription and premium support are distinct. Whether
   implementation services are distinct is a professional judgment
   (`j_impl_distinct`) currently **pending revenue-manager review**; the draft
   treats them as distinct. (Claim `c_step2_performance_obligations`.)
3. **Determine the transaction price.** 300,000 USD, with no variable
   consideration or significant financing component identified in this demo
   arrangement. (Claim `c_step3_transaction_price`.)
4. **Allocate the transaction price.** Allocation on a relative standalone
   selling price basis using price book SSPs (calculation
   `calc_ssp_allocation`):

   | Performance obligation | SSP (USD) | Allocated (USD) |
   |---|---:|---:|
   | SaaS subscription | 240,000 | 192,000 |
   | Implementation services | 75,000 | 60,000 |
   | Premium support | 60,000 | 48,000 |
   | **Total** | **375,000** | **300,000** |

5. **Recognize revenue.** Subscription and support ratably over 24 months
   (8,000 and 2,000 USD per month respectively); implementation services at
   completion, expected 2026-03-31, per `calculations/revenue-schedule.csv`.
   (Claim `c_step5_recognition`.)

## Open items

- Judgment `j_impl_distinct` (are implementation services distinct?) requires
  review and approval by the revenue manager before this work product can be
  finalized. The alternative treatment would combine implementation with the
  subscription and recognize its allocation over the term.

## Validation summary

- `asc606.allocation.tieOut` — pass (allocations sum to the transaction price)
- `asc606.revenueSchedule.total` — pass (schedule ties to allocated consideration)
- `finance.requiredJudgments.complete` — review required (1 open judgment)

---

VWP validation means that a package conforms to specified schemas and
validation rules. It does not mean the professional conclusion is correct,
complete, audited, legally sufficient, or appropriate for a particular use.
Professional judgment, applicable standards, and independent review still
govern reliance on any VWP package.
