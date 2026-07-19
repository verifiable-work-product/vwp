# VWP-Bench v0.1 — Design (Draft)

A benchmark comparing AI-agent-produced accounting work products against work
products produced by practicing CPAs, from identical evidence, graded blind.

**Status: draft for discussion. Nothing here has been run.**

---

## 1. What the benchmark tests

Two claims, measured separately so neither contaminates the other:

1. **Quality parity.** Given the same evidence bundle and deliverable spec,
   does an AI agent produce accounting work products whose conclusions,
   calculations, completeness, and judgment documentation match or exceed
   those of practicing CPAs?
2. **Verifiability advantage.** Is the agent's output — a `.vwp` package —
   cheaper and faster for an independent reviewer to verify than a
   traditionally prepared memo-and-spreadsheet work product, and does
   deterministic validation (`vwp verify`) catch real defects?

Claim 1 is about the *content* of the work. Claim 2 is about the *form*.
Grading for claim 1 therefore happens on format-normalized renderings
(everyone's output becomes a memo + schedules), so the VWP structure cannot
help or hurt the quality score. Claim 2 is measured in a separate review-time
task.

### What this benchmark is not

- Not a CPA-exam-style multiple-choice eval. Those exist and saturate.
  VWP-Bench tests *work product production*: open-ended deliverables from
  messy evidence, the actual unit of professional accounting work.
- Not an assurance engagement or an audit of anyone's work.
- Not a leaderboard (yet). v0.1 is a pilot: 5 challenges, small n, reported
  per-challenge with full methodology, no aggregate headline number.

---

## 2. The five challenges

Each challenge is a self-contained evidence bundle + deliverable spec +
private answer key + 2–3 seeded anomalies. Difficulty targets a competent
senior associate / manager: roughly a half-day of focused work.

| # | ID | Area | Core judgment(s) | VWP extension exercised |
|---|----|------|------------------|-------------------------|
| 1 | `rev-606-saas` | ASC 606 revenue | Distinctness of implementation services; contract term under termination clause; SSP allocation; modification treatment | `accounting-asc606` (exists) |
| 2 | `fv-820-digital` | ASC 820 fair value | Principal market determination; hierarchy leveling; discount for lack of marketability on a locked token | `accounting-asc820` (exists) |
| 3 | `dep-360-fleet` | Depreciation & disposal (ASC 360 / ASC 250) | Useful-life revision as a change in estimate (prospective); component disposal gain/loss; impairment-indicator assessment | new: `accounting-depreciation` |
| 4 | `re-805-acquisition` | Real estate acquisition | Asset acquisition vs. business combination (ASC 805 screen test); purchase price allocation; closing-cost capitalization; settlement statement JEs | new: `accounting-ppa` |
| 5 | `crypto-350-60` | Crypto assets (ASU 2023-08, ASC 350-60) | Wallet population completeness; cost-basis relief method; fair value remeasurement through NI; rollforward disclosure | `digital-assets-wallet-ownership-control` (exists) + `accounting-asc820` |

Rationale for the mix:

- **1, 2, 5 exercise extensions that already exist** in `schemas/v0.1/extensions/`,
  so the agent arm produces packages whose tie-outs are deterministically
  checkable today.
- **3 and 4 force the format to grow** — a useful secondary output of the
  benchmark is two new extension schemas hardened against real work.
- **2 and 5 are Bitwave's home turf** (digital assets) and differentiate the
  benchmark from generic accounting evals; **1, 3, 4 are deliberately
  mainstream** so results generalize beyond crypto.
- Every challenge mixes **deterministic content** (schedules that must tie
  out, JEs that must balance) with **genuine judgment** (distinctness,
  leveling, screen test) so neither humans nor agents can win on arithmetic
  alone.

### 2.1 `rev-606-saas` — SaaS multi-element arrangement

- **Fact pattern.** A 3-year SaaS subscription with one-time implementation
  services, a usage-based overage component, and a mid-term upsell
  modification. A termination-for-convenience clause with a penalty that is
  not substantive puts the enforceable contract term in play.
- **Evidence bundle.** MSA excerpt, order form, upsell order form, first two
  invoices, a one-page SSP study summary, payment confirmations.
- **Deliverables.** Five-step memo; SSP allocation schedule; revenue
  recognition schedule by month for year 1; JEs for the first quarter
  (billing, deferral, recognition); modification analysis.
- **Keyed items (examples).** Transaction price; allocation amounts;
  month-1 through month-3 revenue; deferred revenue balance at Q1;
  prospective vs. cumulative-catch-up treatment of the modification.
- **Seeded anomalies.** Invoice amount that disagrees with the order form;
  an SSP study whose stated range excludes the contract price for one
  element.

### 2.2 `fv-820-digital` — digital asset fair value

- **Fact pattern.** A treasury portfolio: two liquid tokens traded on
  multiple exchanges (with materially different volumes — principal market
  matters), one thinly traded token (level 2), and one token subject to a
  24-month lockup (level 3, DLOM required).
- **Evidence bundle.** Custody statements, exchange price/volume CSVs for
  the measurement date, the token lockup agreement, a third-party valuation
  specialist's DLOM range memo.
- **Deliverables.** Fair value memo with hierarchy classification and basis;
  measurement schedule; draft ASC 820 disclosure table.
- **Keyed items.** Per-position marks; hierarchy level per position; total
  portfolio fair value; DLOM within supportable range with documented basis.
- **Seeded anomalies.** A stale quote (timestamp outside the measurement
  window) on one exchange; a price pulled from a market that volume data
  shows is not the principal market.

### 2.3 `dep-360-fleet` — fixed asset depreciation, revision, disposal

- **Fact pattern.** A fleet of equipment (mining rigs or manufacturing
  machines) acquired across multiple dates. Mid-year, management revises the
  remaining useful life downward (ASC 250 change in estimate — prospective,
  a classic trap for retrospective restatement). A subset of units is sold;
  one asset shows an impairment indicator requiring a recoverability
  assessment.
- **Evidence bundle.** Fixed asset register, purchase invoices, board memo
  on the life revision, disposal agreement, cash receipt, market pricing for
  the impairment indicator.
- **Deliverables.** Depreciation schedule (before and after revision);
  disposal JE with gain/loss; impairment-indicator memo (ASC 360-10 step 1
  recoverability, undiscounted cash flows); annual depreciation expense.
- **Keyed items.** Depreciation expense pre/post revision; NBV at disposal;
  gain/loss amount; recoverability conclusion.
- **Seeded anomalies.** A register row whose in-service date postdates its
  first depreciation charge; a disposal unit that appears twice in the
  register.

### 2.4 `re-805-acquisition` — real estate acquisition

- **Fact pattern.** Purchase of an industrial property with in-place leases.
  First gate: ASC 805-10 screen test — is substantially all the fair value
  concentrated in a single asset/group (→ asset acquisition)? Consequences
  flow: closing costs capitalized (asset acquisition) vs. expensed (business
  combination). Then relative-fair-value allocation across land, building,
  land improvements, and in-place lease intangibles; settlement statement
  JEs including prorations and credits.
- **Evidence bundle.** Purchase and sale agreement (structure adapted from a
  real EDGAR EX-10 exhibit), ALTA settlement statement, appraisal summary
  with component fair values, lease abstracts.
- **Deliverables.** Screen-test memo; purchase price allocation schedule;
  acquisition JEs off the settlement statement; capitalized-cost summary.
- **Keyed items.** Screen conclusion; total capitalized basis; allocation
  per component; the acquisition JE (must balance to cash + credits).
- **Seeded anomalies.** A settlement statement proration computed on the
  wrong day-count; an appraisal component subtotal that doesn't foot.

### 2.5 `crypto-350-60` — crypto assets under ASU 2023-08

- **Fact pattern.** An entity holds BTC and ETH across self-custody wallets
  and an exchange account. Period activity: purchases, a partial sale
  (cost-basis relief under the entity's documented method), a wallet
  transfer (not a disposal — trap), and period-end fair value remeasurement
  through net income. One wallet in the on-chain data is not on the entity's
  wallet register — completeness must be addressed.
- **Evidence bundle.** Wallet register with proof-of-control evidence,
  on-chain transaction export, exchange account statement, price data,
  accounting policy memo (basis-relief method).
- **Deliverables.** Crypto asset rollforward (units and USD); remeasurement
  and disposal JEs; completeness memo addressing the orphan wallet; draft
  ASC 350-60 disclosure.
- **Keyed items.** Realized gain/loss on sale; period-end carrying value;
  remeasurement amount; transfer correctly excluded from disposals;
  disposition of the orphan wallet.
- **Seeded anomalies.** The orphan wallet; a transfer that would double-count
  units if read as buy+sell; an exchange statement fee not in the on-chain
  data.

---

## 3. Source content strategy

**Synthetic but grounded, always.** Fact patterns take their *structure* from
real public documents and their *numbers* from us:

- Contract language and document shapes adapted from SEC EDGAR exhibits
  (EX-10 material contracts, real PSAs, real SaaS agreements) — public
  filings, freely reusable as source structure.
- Schedule and disclosure shapes from real 10-K disclosures (e.g., public
  crypto holders' ASC 350-60 disclosures, real estate REIT PPA disclosures).
- Standard-setter illustrative examples (ASU basis-for-conclusions examples,
  freely downloadable from FASB) used to calibrate difficulty and confirm
  our answer keys — never copied as the fact pattern.

Two hard rules, both validity-critical:

1. **Novel numbers, novel names.** If a fact pattern is verbatim from a Big 4
   guide or FASB example, a model may have memorized the worked answer. Every
   challenge uses original entities, amounts, and dates. Each evidence bundle
   embeds a canary string so we can detect training-set contamination later.
2. **Licensing-clean.** Anything published in the benchmark repo is either
   authored by us, public domain (government works, SEC filings used as
   structural reference only), or Apache-2.0-compatible. Big 4 guide text is
   never reproduced.

A per-challenge `SOURCES.md` records which public documents informed the fact
pattern (see Appendix A for the candidate source list).

---

## 4. Arms and participants

### CPA arm

- **n = 3 CPAs per challenge.** Recruit 5 CPAs, each completing 3 challenges
  (balanced assignment), for 15 human work products total.
- **Qualification.** Active US CPA license; 3+ years relevant technical
  accounting experience; per-challenge screening (e.g., digital-asset
  experience required for challenges 2 and 5, at least for one of the three
  assigned CPAs).
- **Conditions.** Paid at market hourly rates for the full time budget
  (4 hours per challenge, hard stop at 6). Normal professional tools allowed:
  Excel, authoritative guidance (FASB codification, firm guides). **No
  generative AI** in v0.1 — the comparison is agent vs. unassisted
  professional. A CPA+AI arm is the obvious v0.2 extension.
- **Instructions.** The deliverable spec, the evidence bundle, and a
  statement that the work will be graded by peers against a rubric. CPAs are
  told anomalies may exist in the evidence and should be handled as they
  would in practice (investigate, document, or note for follow-up).

### Agent arm

- **The agent** is Bitwave's hosted VWP agent (Wavie), producing a `.vwp`
  package per challenge from the identical evidence bundle and deliverable
  spec.
- **3 independent runs per challenge** (fresh context each) to expose
  variance; all three are graded. 15 agent work products total.
- **Provenance discipline.** Every run records exact provider, model
  identifier, and version in package provenance, consistent with the
  format's requirements — results are meaningless without knowing precisely
  which model produced them.
- **No human touch-ups.** The package is graded as produced. If a run fails
  `vwp verify`, that is itself a result (recorded under claim 2), and the
  rendered output is still graded for claim 1.

---

## 5. Grading

### 5.1 Format normalization and blinding

Every work product — human or agent — is normalized to the same rendered
form: a PDF memo plus schedules in a standard layout, re-typeset by a
non-grading coordinator to strip authorship signals (tool fonts, VWP
rendering headers, Excel styling, handwriting of any kind). Graders receive
work products in randomized order, labeled only with opaque IDs, and are not
told the human/agent mix.

### 5.2 Graders

- 3 independent CPA graders per challenge, distinct from the preparer pool,
  senior to it (manager/senior-manager+), paid for grading time.
- Each grader scores all 6 work products for their challenge (3 human, 3
  agent) against the rubric and the answer key.
- Inter-rater reliability reported (ICC on rubric totals; percent agreement
  on keyed items). Keyed-item disputes resolved against the answer key by
  the technical reviewer, not by grader vote.

### 5.3 Rubric (100 points per work product)

| Dimension | Pts | How scored |
|---|---|---|
| Technical conclusions | 30 | Keyed items: each conclusion right/wrong/partially-supported against the answer key (including documented acceptable alternatives) |
| Calculation accuracy | 20 | Keyed numeric items: exact or within stated tolerance; schedules foot and tie |
| Completeness | 15 | Checklist: every required deliverable element present and responsive |
| Evidence & support | 15 | Are conclusions traceable to specific evidence? Are amounts sourced? Anchored 0–5 scale × 3 |
| Judgment documentation | 10 | Basis stated, alternatives considered, authoritative support cited. Anchored 0–5 scale × 2 |
| Clarity & professional presentation | 10 | Would this survive manager review as written? Anchored 0–5 scale × 2 |

**Seeded-anomaly detection is scored separately** (not inside the 100): for
each seeded anomaly — detected / detected-and-resolved / missed. This is
reported as its own metric because it is the sharpest available proxy for
professional skepticism.

Answer keys are authored independently by two senior CPAs, reconciled, and
signed off by an external technical reviewer before any participant sees a
challenge. Where US GAAP genuinely admits alternatives, the key lists
acceptable positions and the documentation each requires.

### 5.4 Claim-2 measurement: the review task

Separate from quality grading, a **review-time study** on a subset (one human
and one agent work product per challenge, 5 reviewers):

- Reviewer receives a work product in its *native* form (CPA's actual
  memo + Excel vs. the `.vwp` package with the CLI verifier and rendered
  view) and answers: *Would you sign off? What would you send back?* —
  time-boxed, with time-to-decision and defects-found recorded.
- For agent packages, also record: `vwp verify` status, validator findings,
  and whether findings correspond to real defects (validator precision).
- This is where VWP's structure is allowed to matter. It is kept fully out
  of the claim-1 quality scores.

### 5.5 Metrics summary

Per challenge, reported separately for each arm:

- Rubric total (median, range) and per-dimension scores
- Keyed-item accuracy (%)
- Seeded-anomaly detection rate
- Preparation time and cost (CPA hours × rate vs. agent tokens/compute × 3 runs)
- Agent-only: run-to-run variance; `vwp verify` pass rate; validator findings
- Review task: time-to-decision, defects found, sign-off decision

No single aggregate number in v0.1. The honest output is a 5-row table and a
methods section.

---

## 6. Validity threats and mitigations

| Threat | Mitigation |
|---|---|
| Training-data contamination (model has seen the worked example) | Original fact patterns; novel numbers/names; canary strings; publish challenges only after the run |
| Grader bias toward/against AI | Blinding + format normalization + randomized order; report inter-rater reliability |
| Answer key is wrong or too narrow | Dual independent authorship, reconciliation, external technical reviewer; documented acceptable alternatives |
| CPAs under-invest (it's not their client) | Market-rate pay, realistic time budget, peer-graded framing, license-holding professionals |
| Cherry-picked agent runs | All runs graded, n fixed in advance, variance reported |
| VWP format flatters the agent in quality scores | Claim 1 graded on normalized renderings only |
| Small n over-generalized | Per-challenge reporting; pilot framing; no headline aggregate; raw scores published |
| Anomaly seeding makes evidence unrealistic | Anomaly rates calibrated to what practitioners see in the field; anomalies reviewed by answer-key authors for plausibility |

Known limitation, stated up front: 5 challenges × 3 preparers per arm cannot
support broad claims like "agents match CPAs." It can support: "on these five
representative work products, under blind grading, here is exactly what
happened" — which is more than any current accounting-agent claim offers.

---

## 7. Deliverables and repo layout

```
benchmark/
  DESIGN.md                    this document
  rubric/
    rubric.md                  anchored scales, keyed-item scoring rules
    grader-instructions.md
  challenges/
    rev-606-saas/
      brief.md                 deliverable spec given to both arms
      evidence/                the evidence bundle
      SOURCES.md               public documents that informed the pattern
    fv-820-digital/ ...
    dep-360-fleet/ ...
    re-805-acquisition/ ...
    crypto-350-60/ ...
  answer-keys/                 PRIVATE until results publish (separate repo)
  results/                     scores, transcripts of method, report
```

Publication sequence: methodology first (this doc), challenges after the run
completes, answer keys and raw scores with the results report. Agent `.vwp`
packages published as reference packages (also feeding the ROADMAP's "public
reference packages" goal).

---

## 8. Phasing

1. **Pilot one challenge end-to-end** (`rev-606-saas`, since the extension
   and example already exist): author fact pattern + answer key, run 1 CPA
   and 1 agent run, grade with 1 grader. Goal: shake out the rubric, the
   time budget, and the normalization pipeline. ~2–3 weeks.
2. **Author the remaining four** fact patterns + answer keys; build the two
   new extensions (`accounting-depreciation`, `accounting-ppa`) as needed
   for the agent arm. ~3–4 weeks, parallelizable.
3. **Recruit** 5 preparer CPAs + 3–5 grader CPAs + 1 external technical
   reviewer.
4. **Run** both arms; **grade**; run the review-time study.
5. **Report**: per-challenge results, methods, raw data, reference packages.

Estimated external cost, order of magnitude: ~15 CPA-prep engagements
(4–6 hrs) + ~90 grading passes (~1 hr each) + review study + technical
reviewer ≈ 150–200 paid professional hours.

---

## Appendix A — Candidate public source material

Per the strategy in §3: these inform *structure* and calibrate *answer keys*;
fact patterns, entities, and numbers are always original.

### 1. `rev-606-saas`

- **Regent Education Master SaaS Agreement** (EDGAR EX-10.3, Bridgepoint
  Education 10-Q, 2016) — full-text SaaS MSA with cover page, terms, and
  order-form structure; some fees redacted (irrelevant — we author our own).
  Verified accessible.
  <https://www.sec.gov/Archives/edgar/data/1305323/000130532316000120/bpi2016q210-qxexx103regent.htm>
- **ASU 2014-09 Section A** (free FASB PDF) — the 60+ authoritative
  illustrative examples; answer-key calibration.
  <https://storage.fasb.org/ASU%202014-09_Section%20A.pdf>
- Additional MSA shapes: Demandware Master Subscription Agreement amendment
  (<https://www.sec.gov/Archives/edgar/data/1301031/000119312511189260/dex1020.htm>),
  Coin Metrics MSA
  (<https://www.sec.gov/Archives/edgar/data/1767057/000093041321001609/c101984_ex10-2.htm>).

### 2. `fv-820-digital`

- **ASU 2023-08** (free FASB PDF) — includes the contractual-sale-restriction
  disclosure requirements that motivate the locked-token position.
  <https://storage.fasb.org/ASU%202023-08.pdf>
- **RSM, "Determining token fair market value"** and **Cohen & Co on
  partially illiquid cryptoassets** — practitioner framing for principal
  market and DLOM; answer-key calibration only (not reproducible).
  <https://rsmus.com/insights/industries/financial-services/determining-token-fair-market-value-complex.html>,
  <https://www.cohenco.com/knowledge-center/insights/march-2021/3-key-considerations-in-the-valuation-of-partially-illiquid-cryptoassets>
- Exchange price/volume data: generate synthetic CSVs shaped like real
  exchange exports; real historical data is licensable but unnecessary.

### 3. `dep-360-fleet`

- **IRS Publication 946** (public domain) — MACRS classes/conventions if a
  tax-basis sub-task is added; also a useful register-format reference.
  <https://www.irs.gov/pub/irs-pdf/p946.pdf>
- Fixed asset register, invoices, board memo, and disposal agreement are
  straightforward to author; no external anchor needed. ASC 250/360
  answer-key calibration from the codification (free basic view).

### 4. `re-805-acquisition`

- **Rockwell Automation / First Industrial Purchase and Sale Agreement**
  (EDGAR EX-10.P, 2005) — complete ~400KB industrial-property PSA with land/
  improvements identification, price mechanics, prorations, closing costs.
  Verified accessible; ideal structural skeleton.
  <https://www.sec.gov/Archives/edgar/data/1024478/000095012305013478/e14450exv10wp.htm>
- **ALTA settlement statement forms** (alta.org policy forms; fillable
  templates widely available) — the settlement-statement evidence shape.
  <https://www.alta.org/policies-and-standards/policy-forms/settlement-statements>
- **Grant Thornton "Identifying a business combination" viewpoint PDF**
  (freely downloadable) — screen-test answer-key calibration.
  <https://www.grantthornton.com/content/dam/grantthornton/website/assets/content-page-files/private-equity/pdfs/2021/viewpoint-identifying-business-combination.pdf>

### 5. `crypto-350-60`

- **ASU 2023-08** (above) — authoritative measurement/disclosure text
  including the required rollforward.
- **Strategy (MicroStrategy) 10-K/10-Q ASC 350-60 disclosures** — the
  real-world shape of adoption, remeasurement through NI, and holdings
  disclosure (e.g., FY2023 10-K:
  <https://www.sec.gov/Archives/edgar/data/1050446/000095017024015847/mstr-20231231.htm>);
  **Bitcoin Depot 10-K** as a smaller-entity example
  (<https://www.sec.gov/Archives/edgar/data/1901799/000095017024044405/btm-20231231.htm>).
- On-chain transaction data: synthesize wallet exports in standard explorer
  CSV shape (Bitwave has this in-house); proof-of-control evidence follows
  the existing `digital-assets-wallet-ownership` example.

### Existing benchmarks to differentiate from

- **AccountingBench (Penrose, 2025)** — LLMs close the books monthly on real
  SaaS data; long-horizon bookkeeping, found reward-hacking against
  validation checks. Closest in spirit; tests *bookkeeping*, not judgment
  work products, and has no human-CPA comparison arm.
  <https://accounting.penrose.com/>
- **Ramp accounting-agent benchmark** — 237 tasks across 8 synthetic
  business worlds; automation-oriented task completion.
  <https://www.zenml.io/llmops-database/benchmarking-and-optimizing-ai-agents-for-accounting-automation>
- **"Can LLMs Pass the CPA Exam?" (2024)** and **CA-Ben (2025,
  arXiv:2506.21031)** — exam-style Q&A; saturating and far from work-product
  production.
- **FinAuditing (2025)** — XBRL multi-document structure/math tasks; no
  open-ended deliverables.

**VWP-Bench's open lane:** open-ended professional work products (memo +
schedules + JEs) from messy evidence, blind head-to-head grading against
licensed CPAs, seeded-anomaly skepticism measurement, and a verifiability
arm — none of the above does any of these. AccountingBench's reward-hacking
finding is also a direct argument for VWP's posture: validation checks an
agent can game are exactly why packages need deterministic, inspectable
verification plus human-owned judgment.
