# VWP-TT — The VWP Trust Tests: Battery + Platform Specification

**Version:** 0.1 draft
**Status:** Build-ready specification. Hand to a build agent together with read access to the `vwp` repository (`/Users/patrickwhite/Source/bitwave/vwp`).
**Audience:** An engineering agent building the platform; CPAs and coordinators operating it.

---

## 0. One-paragraph summary

VWP-TT is (1) a battery of binary tests — modeled on the TTCW methodology (Chakrabarty et al., "Art or Artifice?", CHI 2024) but judging **accuracy, grounding, consistency, and judgment quality** for accounting and white-collar work products, instead of creativity — where every test is anchored to a VWP construct (`evidence → facts → claims → judgments → calculations → validations`), and (2) a web platform where a coordinator uploads evidence (PDFs), humans and LLMs independently produce structured work-product items in VWP form, deterministic validators run the machine-checkable tests, and blind human judges run the judgment-based tests TTCW-style (binary verdict + written rationale). Human involvement is structural, not optional: humans author novel fact patterns (contamination defense) and judge the gray-area tests (TTCW showed LLM judges correlate near zero with experts — automation of judging must be earned via a correlation gate, never assumed).

---

## 1. Naming

- **Battery:** **VWP-TT** — "VWP Trust Tests." (Echoes TTCW's "Tests of…" construction; "trust" is the construct being measured: can this work product be trusted?)
- **Platform:** **VWP-TT Studio** (the web app). Repo directory: `tt/`.
- Test IDs are prefixed by dimension: `G*` (Grounding), `C*` (Claims), `K*` (Calculations), `J*` (Judgments), `S*` (Consistency & Skepticism), `P*` (Completeness).
- Alternates considered, in case the owner prefers one: VWP-Trials, VWP-Proof, VTT. Everything below uses VWP-TT; renaming is a find/replace.

---

## 2. Relationship to existing repo assets

The platform **reuses, does not reinvent**. Build agent: read these before writing code.

| Existing asset | Role in VWP-TT |
|---|---|
| `spec/*.md`, `schemas/v0.1/` | Canonical object model. All submitted items serialize to these schemas. |
| `cli/` (TS reference impl) | Package assembly (`init`, `add evidence/fact/...`, `build`) and verification (`verify`) — invoked as a library/child process by the platform backend. |
| `sdk/go` + `backend/` (vwp-api on Cloud Run) | Independent second verifier; platform calls it for cross-implementation verification of every package. |
| `benchmark/DESIGN.md` | Methodology base: arms, blinding, rubric philosophy, validity threats. VWP-TT battery replaces/refines the 100-point rubric with binary tests; everything else carries over. |
| `benchmark/schemas/required-facts.schema.json` | The per-challenge fact-slot manifest — becomes a first-class DB entity and the backbone of test `P1`. |
| `benchmark/schemas/answer-key.schema.json` | Private keyed-item expectations incl. `alternatives`/`requiredDocumentation` — backbone of tests `K2`, `J2`. |
| `benchmark/tools/score-key.mjs` | Keyed-item scorer — port/wrap into the machine-test runner. |
| `benchmark/tools/number-evidence.mjs`, `transcribe-worksheet.mjs` | Evidence numbering for human packets; worksheet transcription. Wrap as backend jobs. |
| `benchmark/FORMAT-GAPS.md` Gap 1 (evidence anchors) | The platform **implements** the `{evidenceId, anchor:{quote,page,section,cells,lines,jsonPath}}` proposal. Test `G2` is the quote-anchor validator. |
| `benchmark/FORMAT-GAPS.md` Gap 2 (`factType`) | Required-facts slots bind to `factType`; the fact editor enforces it. |
| `benchmark/challenges/rev-606-saas/` | First seeded challenge; used to build the demo/E2E fixtures. |
| `frontend/public/index.html` (vwp.ac reader) | Rendering approach for the judge view; reuse its readable-rendering code where practical. |

---

## 3. The VWP-TT battery

### 3.1 Design rules (inherited from TTCW, adapted)

1. Every test is **binary** (PASS/FAIL) with a **required written rationale** for human-judged tests. No Likert scales.
2. Every test is anchored to a **named VWP construct** and, where possible, an **existing or new deterministic validator**.
3. Tests split into three execution modes:
   - **M (machine):** deterministic validator; no human; runs on every submission automatically.
   - **H (human):** blind expert judge; binary verdict + rationale ≥ 200 chars.
   - **X (hybrid):** machine computes candidate evidence (diffs, matches, flags); human confirms the verdict.
4. **LLM judging is disallowed by default.** An LLM may shadow-judge any H test, but its verdicts never count until that test passes the automation gate (§5.4).
5. A work product's result is the **test vector**, not a scalar. Reporting is per-test pass matrices per challenge. No headline aggregate in v0.1 (consistent with `benchmark/DESIGN.md` §5.5).

### 3.2 The 18 tests

#### Dimension G — Grounding (constructs: `evidence`, `facts`, anchors)

| ID | Mode | Question | Operationalization |
|----|------|----------|--------------------|
| **G1** | M | Does every fact resolve to evidence? | Reference-graph check: each `facts[].evidenceRefs[]` resolves to an evidence object in the package. Already in `vwp verify` core. |
| **G2** | M | Are fact anchors verbatim-verifiable? | For every fact whose evidenceRef carries an `anchor.quote`: the quoted bytes MUST appear in the cited evidence file (text extraction for PDFs; exact match after whitespace normalization). Implements FORMAT-GAPS Gap 1. FAIL if any quote is absent; FAIL if < 90% of facts carry anchors (threshold configurable per challenge). |
| **G3** | H | Is every material fact actually true to the evidence? | Judge samples all facts (or a stratified sample > 25 facts), reading each against its anchored location. FAIL if any material fact is unsupported by or contradicts the evidence (hallucination test). Machine pre-work: side-by-side fact/anchor viewer. |
| **G4** | H | Was anything material left un-extracted? | Judge, holding the private required-facts manifest and anomaly list, verdicts whether all conclusion-relevant information in the evidence made it into facts. Distinct from P1 (slot coverage): G4 catches material info *outside* the declared slots. |

#### Dimension C — Claims (constructs: `claims`, claim graph, standard refs)

| ID | Mode | Question | Operationalization |
|----|------|----------|--------------------|
| **C1** | M | Is every claim supported? | Graph check: each claim's support refs (facts/calculations/judgments) resolve; no orphan claims. Extends `finance.claims.haveSupport`. |
| **C2** | H | Are the cited standards the right ones, correctly applied? | Judge verdicts whether the authoritative references (e.g. ASC 606-10-25-19) actually govern the issues and are applied faithfully. FAIL on any materially wrong citation or misapplication. |
| **C3** | H | Does the conclusion follow? | Reading only the package's claims/judgments/calculations (not the rendering), would a competent reviewer accept that the stated conclusion follows without logical gaps? |

#### Dimension K — Calculations (constructs: `calculations`, extension validators)

| ID | Mode | Question | Operationalization |
|----|------|----------|--------------------|
| **K1** | M | Do all calculations recompute? | Re-execute every calculation from declared inputs; outputs must reproduce exactly (spec/calculations.md recomputability). |
| **K2** | M | Do keyed values match the answer key? | `score-key.mjs` semantics: every keyed slot exact or within declared tolerance, honoring `alternatives` (an alternative only passes if its `requiredDocumentation` is present — machine checks presence, J2 checks adequacy). FAIL if any keyed item misses. Also emit per-item detail for reporting. |
| **K3** | M | Do schedules foot, tie, and balance? | Run all applicable extension validators (`asc606.allocation.tieOut`, `asc606.revenueSchedule.total`, JE balance checks, rollforward continuity, etc.). FAIL on any validator FAIL (REVIEW items do not fail K3). |

#### Dimension J — Judgments (construct: `judgments`)

| ID | Mode | Question | Operationalization |
|----|------|----------|--------------------|
| **J1** | X | Were all genuine judgment points recognized as judgments? | Machine diffs the package's judgment set against the challenge's private judgment registry (part of the answer key); human confirms whether missing ones were genuinely required and whether silent assumptions occurred. FAIL if a registry judgment was silently assumed rather than surfaced. |
| **J2** | X | Is each position within the defensible range? | For enum-keyed judgments, machine compares against key + alternatives; human verdicts the not-keyed ones and confirms `requiredDocumentation` adequacy. FAIL if any position falls outside what the key authors documented as defensible. |
| **J3** | H | Would the judgment documentation survive reviewer sign-off? | Basis stated, alternatives considered, authority cited — per judgment, aggregated: FAIL if any material judgment is documented too thinly for a manager to sign. |

#### Dimension S — Consistency & Skepticism (constructs: whole package, variants, seeded anomalies)

| ID | Mode | Question | Operationalization |
|----|------|----------|--------------------|
| **S1** | M | Is treatment consistent across variants? | The same participant/model completes k perturbed variants (same economics, different surface: names, order, immaterial figures). Machine compares keyed conclusions and enum judgments across the participant's variant set. FAIL if any keyed conclusion flips. (Only applies to arms that run variants; typically LLM arm always, human arm optional.) |
| **S2** | M | Is the package internally consistent? | Cross-object checks: the same `factType` appearing twice with conflicting values; claim text amounts (parsed) contradicting calculation outputs; rendering figures (parsed from the required readable) contradicting structured values. FAIL on any material contradiction. |
| **S3** | X | Were the seeded anomalies caught? | Machine searches package text/facts/judgments for references to each seeded anomaly (registry carries match hints); human confirms detected / detected-and-resolved / missed. Binary verdict: PASS = all seeded anomalies at least detected-and-documented. Per-anomaly detail retained for reporting (this is the skepticism metric from DESIGN.md §5.3). |

#### Dimension P — Completeness (constructs: required-facts manifest, deliverable spec, renderings)

| ID | Mode | Question | Operationalization |
|----|------|----------|--------------------|
| **P1** | M | Are all required fact slots filled? | Every slot in the required-facts manifest has a fact with the bound `factType` and conforming `valueShape`. |
| **P2** | H | Is every required deliverable element present and responsive? | Judge checks the deliverable spec (memo sections, schedules, JEs, disclosures) against the rendering + structured content. FAIL if any required element is missing or non-responsive. |

### 3.3 Machine execution is conditional: the per-item structure contract

"Machine-run" does **not** mean "free." Because every review item is novel
(contamination defense, §4), most machine tests only execute against
structure that humans author per item. The battery therefore splits into
three execution classes, and the platform must treat the per-item structure
as a deliverable with its own pipeline (§6.1 Flow A′):

| Class | Tests | Per-item structure required | Authored by |
|---|---|---|---|
| **M0 — structure-free** | G1, G2, C1, K1, S2 | None. These check a package against itself and its own evidence (graph resolution, quote anchors, recomputation, internal consistency). They run on *any* conformant package, forever, with zero per-item authoring. | — |
| **M1 — structure-bound** | K2, K3*, P1, S1, J1†, J2†, S3† | K2: answer key (expected values, tolerances, alternatives). P1: required-facts manifest. S1: variant family + per-variant re-derived key. J1/J2: judgment registry. S3: anomaly registry + match hints. K3*: extension validators are per-*area* (reusable across items in the same area), but the challenge must declare which apply. | Key authors (×2, reconciled), per novel item |
| **H — human-judged** | G3, G4, C2, C3, J3, P2 (+ the human half of X tests) | Judge assignments; G4/J1/J2/S3 additionally consume vault slices. | Judges, per submission |

†J1/J2/S3 are X-mode: their machine half is structure-bound, their verdict is human.

So the honest accounting per novel item is: **5 tests free (M0), 7 tests
bought with key-author hours (M1), and 9 tests requiring judge hours (the 6
pure-H tests plus the human halves of J1, J2, S3).** The design goal is
to make the M1 column cheap and reliable, which is what the reference-solution
pipeline below does.

### 3.4 The reference-solution pipeline (how novel items get structured)

Rule: **the per-item structure is never hand-authored as loose JSON; it is
derived from a reference solution that is itself a VWP package.**

1. Each key author completes the novel challenge **in the same preparer
   workbench** participants use, producing a full reference `.vwp`: facts
   (anchored), claims, judgments, calculations, memo. This replaces
   hand-writing `answer-key.json` — the repo's `qa-facts.json` /
   `qa-perfect-worksheet.csv` idea, promoted to the primary authoring path.
2. The platform **derives structure scaffolds** from the reconciled reference
   package:
   - **Required-facts manifest** ← the reference package's typed facts
     (author marks which are required vs. incidental, which are keyed).
   - **Answer key** ← keyed facts' values + calculation outputs become
     `expected`; tolerances defaulted by valueShape; authors add
     `alternatives`/`requiredDocumentation` where GAAP admits them.
   - **Judgment registry** ← the reference package's judgment objects
     (question, position, basis) become registry entries; authors add the
     defensible-range annotations.
   - **Anomaly registry** ← seeded anomalies are declared at fact-pattern
     authoring time; the tool asks the author to anchor each anomaly to its
     evidence location, which becomes the S3 match hints.
3. **Reconciliation happens at the package level:** the authors'
   reference packages are diffed object-by-object (keyed values, judgment
   positions, calculation outputs). Divergences either get fixed or become
   documented `alternatives` — exactly the RUNBOOK step 2 semantics, now
   tool-assisted. The diff engine is n-way (2 authors in head-to-head mode,
   all enrolled humans in calibration mode — §6.0a).
4. **Key lint (freeze gate):** the reconciled reference package MUST itself
   pass every M0 test plus K3 and P1 against the derived manifest. A key
   whose own reference solution doesn't recompute, tie out, or cover the
   manifest is not freezable. This catches wrong keys before any participant
   sees the item — the strongest answer to DESIGN.md's "answer key is wrong"
   validity threat.
5. **LLM assistance is allowed for scaffolding only** (drafting the fact
   pattern's evidence documents, proposing perturbation maps, pre-filling
   manifest descriptions) and is always followed by human authoring of the
   actual values. LLM-drafted *expected values* are prohibited — the key must
   be independently human-derived, or K2 degenerates into model-vs-model
   agreement.
6. **Variant re-derivation:** because variants are produced by a
   deterministic perturbation map (§4.4), the platform re-derives each
   variant's answer key by pushing the same map through the reference
   package's facts and calculations, then **re-runs key lint per variant**
   (the map might break a tie-out; recomputation catches it). Only linted
   variants go live. This is what makes S1 affordable: one authored item
   yields k machine-testable variants.

**Per-item authoring budget** (to be validated in the pilot, and reported):
fact pattern + evidence ≈ 4–8 h (coordinator + LLM scaffold), 2 × reference
solution ≈ 2 × 4 h (the same half-day DESIGN.md budgets for preparers),
reconciliation + annotations ≈ 2 h, registries ≈ 1 h. ≈ **15–19 expert-hours
per novel item** before any participant runs. This is the real unit cost of
the benchmark and should appear in every results report alongside judge
hours.

### 3.5 Scoring & reporting semantics

- Per work product: an 18-bit vector (one bit per test: G1–G4, C1–C3, K1–K3, J1–J3, S1–S3, P1–P2) plus per-test detail records.
- **Machine tests** run instantly on submission; results visible to the coordinator immediately, to the participant per challenge policy (default: after challenge close, to prevent iterative gaming; configurable "practice mode" shows them live).
- **Human tests** need `n` judges (default 3). Test verdict = majority; report also unanimity rate. All rationales retained verbatim.
- **Aggregates reported:** per-challenge pass-rate matrix (arms × tests), inter-judge agreement per test (percent + Fleiss' κ for n≥3, Cohen's κ for n=2), TTCW-style aggregate ("tests passed out of applicable") with distribution, seeded-anomaly detail table, and variant-consistency detail. Explicitly **no single leaderboard number** in v0.1.

---

## 4. Contamination control model

This is a first-class subsystem, not a policy note.

1. **Challenge lifecycle:** `draft → frozen → live → retired → published`.
   - `draft`: coordinator authoring; nothing visible to participants.
   - `frozen`: fact pattern, evidence, required-facts manifest, answer key, judgment registry, anomaly registry, and the reference packages all locked (hash-sealed; any edit requires version bump and re-freeze). Freezing requires: two reference packages authored and reconciled (§3.4), derived key annotated, ≥1 seeded anomaly, key lint passing, canaries injected.
   - `live`: accepting submissions. Evidence downloadable only by enrolled participants; watermarked per participant (see 4.3).
   - `retired`: closed to new submissions; judging completes.
   - `published`: evidence + key + results exportable to the public repo. **A challenge may never return to `live` after `published`.**
2. **Answer-key isolation:** keys, judgment registries, and anomaly registries live in a separate Spanner database (`vwp-tt-vault`) accessible only to a dedicated vault service account (the app runtime SA has no IAM grant on it), encrypted at rest (application-level AES-GCM with a KMS-held key on top of Spanner's own encryption), readable only by roles `coordinator` and `key_author`, and **never** serialized into any API response reachable by `preparer`, `llm_runner`, or `judge_pre_assignment` roles. Judges receive key material only for their assigned challenge, only after the challenge is `retired` or per-test as needed (G4, J1, J2, S3).
3. **Canaries & watermarks:** on freeze, the platform injects (a) a per-challenge canary GUID string into every evidence file's metadata and an inconspicuous content location, recorded in the vault for future training-set contamination probes; (b) on each download, a per-participant zero-width/whitespace watermark in text evidence and PDF metadata tag, so leaked bundles are attributable.
4. **Variant generation:** the coordinator can spawn k surface variants of a frozen challenge (entity names, dates shifted by a constant, immaterial amounts jittered, document order shuffled) via a guided tool: an LLM proposes the perturbation map, the platform applies it **deterministically** (string/number substitution engine, never free rewriting of normative amounts), a key author reviews a diff, and the answer key is re-derived automatically through the same map. Variants share a `familyId`; S1 runs within families.
5. **Novelty attestation:** freezing requires each key author to check an attestation: "This fact pattern's entities, amounts, and dates are original; structure sources are recorded in SOURCES." SOURCES text is a required challenge field.
6. **Model-exposure ledger:** every LLM submission records provider, model ID/version, and date against the challenge. A challenge shows exactly which models have seen it; publishing exposes this ledger.

---

## 5. Human roles and the judging protocol

### 5.1 Roles

| Role | Can |
|---|---|
| `admin` | Everything incl. user management, settings. |
| `coordinator` | Create/freeze/retire/publish challenges, manage enrollment, assign judges, view all incl. vault, run exports. Cannot judge or prepare. |
| `key_author` | Author/reconcile answer keys, judgment registries, anomaly registries for assigned challenges. |
| `preparer` (human arm) | See enrolled challenge packets, use the workbench, submit. Never sees vault or other submissions. |
| `llm_runner` | API-key-scoped machine account; fetch packet, submit package, per challenge enrollment. |
| `judge` | See assigned judging tasks: blinded work products + the vault slices each test needs. Cannot see preparer identities or arm labels. |
| `viewer` | Read published results. |

One person may hold multiple roles but **never** conflicting ones on the same challenge (enforced: preparer ⊄ judge ⊄ key_author per challenge — the RUNBOOK's two-hats matrix, enforced in software).

### 5.2 Blinding

- Judges receive work products under opaque IDs (`wp_7f3a…`), in randomized order, arm labels stripped.
- All judged content is **format-normalized server-side**: the structured package is re-rendered through one shared template (memo + schedules + fact/claim/judgment tables in the platform's house style). Judges never see the participant's native rendering, tool fonts, or VWP headers. (DESIGN.md §5.1, automated.)
- The coordinator holds the ID mapping; it is revealed in exports only at `published`.

### 5.3 Judge workflow (TTCW-style)

For each assigned (work product × test) task:
1. Judge opens the task: left pane = normalized work product with construct navigation (facts with anchor jump-to-evidence, claims graph, judgments, calculations); right pane = the test card (question, operationalization, the vault slice if the test needs it, machine pre-work for X tests).
2. Judge selects **PASS / FAIL**, writes rationale (min 200 chars; the UI nudges toward citing specific object IDs — rationale box offers `#f_`, `#c_`, `#j_` autocompletion that inserts links).
3. Tasks are independent; judges cannot see other judges' verdicts until the challenge is retired and their own queue is complete.
4. Time-on-task recorded passively (for the review-time/verifiability arm of DESIGN.md §5.4).

### 5.4 LLM shadow judging and the automation gate

- Any H/X test can be configured with shadow judges: LLM configurations (provider, model, fixed judge prompt template versioned in the DB) that receive **exactly the same blinded, normalized material** and return verdict + rationale via the same schema.
- Shadow verdicts are stored but excluded from official results.
- Shadow judges run **only as a stage-F batch after human verdicts lock**
  (§6.0); the harness refuses to start while any human queue is open.
- **Automation gate:** a test may be flipped to machine-assisted-official only when, over ≥ 30 judged work products spanning ≥ 3 challenges, the shadow judge achieves κ ≥ 0.6 against the human majority AND has no systematic arm bias (pass-rate delta between arms vs humans < 10 pts). The gate check is a built-in report, and flipping requires coordinator action with the report attached. Even after flipping, 20% of tasks continue to be dual-judged for drift monitoring. (This operationalizes TTCW's core negative finding — their LLM judges correlated near zero with experts — as an ongoing guardrail instead of a one-time assumption.)

---

## 6. Platform — product spec

### 6.0 Pipeline sequencing: human-first, model-batch

The studio runs each challenge as a staged pipeline. The governing rule:
**human material is always gathered and reconciled before models run, and
human verdicts are always locked before model judges run.** Models execute
in batches against frozen inputs; they never interleave with human work.

| Stage | Who | What | Lock produced |
|---|---|---|---|
| **A — Structure** | Key authors | Fact pattern, evidence, reference packages, object-level reconciliation, derived key/registries, key lint (§3.4) | `frozen` seal |
| **B — Human arm** | Preparers | Human submissions against the live challenge | Human-arm lock (no more human submissions) |
| **C — Reconcile & dedupe** | Coordinator + key authors | Cross-submission review of the human arm's facts/judgments: merge duplicate alternative positions, dedupe anomaly mentions, and — if multiple independent humans take a defensible position the key missed — amend the key. **This is the last moment the key may change.** | Key-amendment lock |
| **D — Model batch** | Platform | All enrolled model configs run against the identical packet (n runs each, fresh context), recorded in the model-exposure ledger. Submission-time machine tests run on everything (both arms) now. | Submission pool lock |
| **E — Human judging** | Judges | Blind judging of the **mixed, randomized pool** (human + model work products together — the one stage where model outputs precede human work, by design: the head-to-head must be blind). Verdicts + rationales collected; rationales parsed into discrete findings, deduped into a canonical per-work-product defect list. | Verdict lock |
| **F — Shadow-judge batch** | Platform | All configured LLM judge configs run against the same tasks; correlation/gate stats computed against the locked human consensus (§5.4). | Results lock |

Enforcement (not convention — platform-enforced):

1. Stage D cannot start until stages B and C are locked. The "Run models"
   action is disabled with the unmet preconditions listed.
2. **Key immutability after first model exposure:** any key amendment after
   stage D begins is rejected. (Prevents tuning the key to model behavior in
   either direction; the stage-C amendment window exists precisely so
   legitimate key gaps surface from independent human work, not from model
   output.) All amendments carry a reason and appear in the audit log and
   the published methods export.
3. Stage F cannot start until every human judge queue for the challenge is
   complete and verdicts are locked. Shadow judges never run while human
   judging is open (no possibility of leakage, and the consensus target is
   fixed).
4. **Findings dedupe (stage E):** judge rationales are segmented into
   findings (one defect/observation each); the coordinator merges duplicates
   across judges into canonical findings with links back to each source
   rationale. Canonical findings power the per-defect drill-down: which
   defects exist in each work product, which judges caught them, and — after
   stage F — which model judges caught them.

### 6.0a Challenge modes and the Bitwave pilot

Every challenge carries a **mode**, chosen at creation:

- **`calibration`** — every enrolled human is a key author. All extractions
  (reference packages) are gathered, then reconciled/deduped in one sitting
  (§3.4 step 3, generalized from 2 to n authors: pairwise object diff →
  merged reference → divergences become documented `alternatives`). The
  reconciled result becomes the key; the LLM batch (stage D) is then scored
  against it. There is no blinded human arm and no head-to-head claim for
  the item — its human data is the key itself. Judging (stage E) still runs
  on model outputs only, when Phase 3 exists; until then, calibration items
  report machine tests alone.
- **`head_to_head`** — the RUNBOOK split: designated key authors (≥2)
  structure the item; the remaining humans are blinded preparers whose
  submissions are graded alongside the model batch (full §6.0 stages A–F).

Mode is immutable after freeze. A calibration item MAY later be cloned into
a head-to-head item **for new participants only** (its key already exists;
its exposure ledger carries over).

**The first Bitwave pilot (4 in-house CPAs, Cloud Run deployment):**

1. Coordinator stands up the studio (Cloud Run service `vwp-tt`, §7),
   provisions 4 CPA accounts + coordinator account, uploads the first
   challenge's evidence PDFs, authors brief/spec, declares anomalies.
2. Each CPA independently completes the extraction in the preparer
   workbench — anchored facts, judgments, calculations, memo. No
   discussion (enforced socially, logged time).
3. Coordinator convenes reconciliation: the platform's n-way object diff
   shows the 4 extractions side by side per slot/judgment/calculation;
   agreements auto-merge, divergences are resolved live or documented as
   alternatives; key lint must pass; key locks (stage C semantics).
4. LLM batch runs (stage D) — external-agent path (Flow C1) first;
   machine tests score every model run against the 4-CPA key.
5. Output: per-model test vectors + keyed-item detail on a genuinely novel,
   4-expert-reconciled item — publishable as calibration results, and the
   item becomes reusable structure for later head-to-head runs.

This means the **pilot needs only Phases 1–2** (workbench, reference
pipeline, machine tests) — judging (Phase 3) and shadow judging (Phase 4)
are not on the critical path for the first CPA send-out.

### 6.1 Core user flows

**Flow A — Coordinator creates a challenge**
1. New challenge → title, area (ASC 606 / 820 / 350-60 / 360 / 805 / custom), profile + extensions to enable, deliverable spec (markdown), SOURCES (markdown), time budget.
2. Upload evidence: PDFs (primary), plus txt/csv/xlsx/json/png. Per file: title, evidence ID, type. Platform computes SHA-256, extracts PDF text per page (store page-indexed text for anchor validation and the workbench viewer), renders page thumbnails.
3. Declare seeded anomalies at authoring time (each anchored to its evidence location — becomes the S3 match hints).
4. **Reference-solution authoring (§3.4):** each key author (≥2) completes the challenge in the preparer workbench, producing a reference `.vwp`. The platform diffs the two reference packages object-by-object; divergences are fixed or documented as `alternatives`.
5. **Structure derivation:** from the reconciled reference package the platform scaffolds the required-facts manifest, answer key, and judgment registry (validated against `benchmark/schemas/*.schema.json`); authors annotate (required/keyed flags, tolerances, alternatives + requiredDocumentation, defensible ranges) in the vault editors. Hand-editing derived values requires a note; hand-authoring the key from scratch is not a supported path.
6. **Key lint:** freeze is blocked until the reference package passes all M0 tests plus K3 and P1 against the derived manifest.
7. Freeze → platform seals hashes, injects canaries, flips to `frozen`. Enroll participants → `live`.

**Flow B — Human preparer works a challenge**
1. Enrolled preparer opens the challenge: brief, deliverable spec, evidence viewer (PDF viewer with text layer), required-facts manifest as a to-do list, running time log (start/stop, self-reported adjustments allowed with note).
2. **Workbench** — four item editors, all producing spec-conformant objects:
   - **Facts:** pick slot (or free fact), enter value in the slot's valueShape widget (money/date/enum/etc.), then **select text in the PDF viewer to attach the anchor** (quote + page auto-captured; section optional; cells picker for spreadsheets). Facts without anchors show a warning badge.
   - **Claims:** statement, assertion tags, standard refs (typeahead over a bundled ASC citation list, free text allowed), support links (pick from existing facts/calcs/judgments).
   - **Judgments:** question, position, basis, alternatives considered, authority refs.
   - **Calculations:** name, inputs (link facts or literals), method (expression DSL: arithmetic + sum/round over named inputs — enough for allocations, schedules, JEs; the CLI recompute semantics), outputs. A JE editor and a schedule (table) editor are sugar over calculations.
   - **Memo:** markdown editor for the required readable rendering; live preview.
3. Submit → platform assembles the `.vwp` (Flow D), runs verify + machine tests, stores the submission. Preparer sees a receipt (verify status only, not test results, unless practice mode).

**Flow C — LLM arm** (runs only in stage D, after the human-arm and key-amendment locks — §6.0)
- **C1 (external agent, build first):** `llm_runner` API key fetches the packet (`GET /api/challenges/:id/packet` → brief, deliverable spec, evidence files, required-facts manifest — never vault material), the operator's agent produces a `.vwp` by any means (typically the existing CLI), and `POST /api/challenges/:id/submissions` uploads it. Platform records model/provider metadata from the package's provenance (reject packages whose provenance omits model identity — DESIGN.md §4 provenance discipline).
- **C2 (built-in runner, later phase):** coordinator configures provider+model+key; the platform runs a fixed, versioned harness prompt chain that walks the model through fact extraction → claims → judgments → calculations → memo, calling the same assembly path as the human workbench. Every prompt/response logged. 3 runs default, fresh context each.

**Flow D — Package assembly & validation pipeline (server-side, both arms)**
1. Assemble working directory in CLI layout; write objects; `vwp` CLI `build` (with hashes) → `.vwp` stored in object storage, content-addressed.
2. Run TS `vwp verify` AND the Go `vwp-api` verifier; store both results (cross-implementation agreement is itself telemetry).
3. Run machine tests G1, G2, C1, K1, K2, K3, S2, P1 (+ S1 when the variant family completes); persist per-test result rows with machine detail JSON.
4. Generate the normalized judge rendering and cache it.

**Flow E — Judging** (stage E: runs on the mixed human+model pool, never before the model batch completes)
1. Coordinator retires challenge, clicks "Generate assignments": platform creates (work product × H/X test × judge) tasks honoring the conflict matrix and coverage (each WP×test judged by n=3), randomizes order per judge across the full mixed pool.
2. Judges work their queues (§5.3). Progress dashboard for the coordinator.
3. When all queues complete: verdicts lock, agreement stats computed.
4. **Findings reconciliation:** the platform segments rationales into findings (rule-based on sentence/ID-link boundaries; an LLM may propose segment boundaries and candidate duplicate pairs, but merging is a coordinator action); coordinator dedupes into canonical findings.
5. Only then may the shadow-judge batch (stage F, §5.4) be started.

**Flow F — Results & publication**
- Per-challenge dashboard: arms × tests pass matrix, per-test drill-down (every machine detail / judge rationale), keyed-item table, anomaly table, variant consistency table, agreement stats, time/cost per arm, shadow-judge correlation report.
- Export: results JSON + CSVs + the reference `.vwp` packages + (on `published`) evidence and keys, laid out to drop into `benchmark/` repo conventions.

### 6.2 Non-goals (v0.1)

- No public self-serve signup (users are provisioned by admin).
- No payments/recruiting workflow.
- No real-client data ever; synthetic-only warning at evidence upload.
- No mobile layout for the workbench (judge view should be tablet-usable).
- Not a general VWP authoring product — the workbench is benchmark-scoped.

---

## 7. Architecture & stack

Chosen to match the repo's existing tooling and deployment surface (GCP project `verifiable-work-product`, Cloud Run, Firebase Hosting, Cloud Build):

- **Monorepo location:** new top-level `tt/` directory: `tt/server`, `tt/web`, `tt/shared` (types + zod schemas), `tt/e2e`.
- **Backend:** Node 20 + TypeScript + Fastify. Imports the existing `cli/` build (`cli/dist`) as a library where its exports allow; otherwise child-process invocation of `node cli/dist/index.js` (pack/build/verify/render). Calls the Go `vwp-api` over HTTP for the second verification.
- **DB:** **Cloud Spanner** (prod), **Spanner emulator** (local dev/CI — Docker image `gcr.io/cloud-spanner-emulator/emulator`). Use the **GoogleSQL dialect** with the official Node client (`@google-cloud/spanner`). One instance, two **databases**: `vwp-tt-app` and `vwp-tt-vault` — vault isolation is achieved by separate databases with separate IAM service accounts (the app runtime SA has no permission on the vault DB; a dedicated vault data-access layer runs with the vault SA and exposes only the §4.2 role-gated operations), replacing the Postgres SECURITY DEFINER pattern. Fine-grained access control (Spanner database roles) additionally scopes the vault DAL to specific tables. Migrations via `wrench` (schema-as-DDL files in `tt/server/migrations/spanner/`, applied in CI). Design notes: random UUIDv4 primary keys (safe key distribution — never timestamp-prefixed keys); use interleaved tables where the child is always fetched with the parent (`evidence_pages` in `evidence_files`, `judge_findings` in `judge_tasks`); commit timestamps via `allow_commit_timestamp` for audit columns; read-only transactions for all dashboards; no foreign-key cascades relied on for integrity-critical deletes (results rows are append-only anyway, §12).
- **Object storage:** GCS bucket (local: filesystem adapter). Content-addressed: `evidence/{sha256}`, `packages/{sha256}.vwp`, `renders/{sha256}.html`.
- **Auth:** OAuth 2.0 / OIDC against **`auth.bitwave.io`** (client ID **`vwp-tt`**) — see §7.1. Human users sign in exclusively via Bitwave SSO; the platform keeps its own `users` row keyed by the IdP `sub` claim and manages roles locally (§5.1 — role assignment stays in-app, not in the IdP, so the conflict matrix remains platform-enforced). API keys (hashed, argon2id) remain for `llm_runner` machine accounts, which do not use OAuth. Session: httpOnly secure cookie holding a platform session ID; IdP tokens are kept server-side only.
- **PDF:** `pdfjs-dist` for text extraction (page-indexed) server-side and the viewer/text-selection UI client-side; `pdf-lib` for metadata canary/watermark injection.
- **Frontend:** React 18 + Vite + TypeScript, TanStack Query/Router, minimal component lib (shadcn or plain). PDF viewer with selectable text layer and highlight overlay for anchors.
- **Jobs:** **Cloud Tasks** (prod) targeting internal handler routes, with an in-process dispatcher fallback for local dev/emulator (same handler code; `JOBS_MODE=inline` env). Job state/idempotency rows live in the `jobs` table in Spanner (task IDs are deterministic per (entity, step) so retries are idempotent). Pipeline steps: extraction, verify, machine tests, render generation, variant application, model batches.
- **LLM access (shadow judges, built-in runner, perturbation proposer):** thin provider adapter (Anthropic + OpenAI + generic OpenAI-compatible), keys stored encrypted in `vault`, every call logged (prompt hash, model, tokens).
- **Deploy:** Dockerfile per service; Cloud Run service `vwp-tt`; static web via the existing Firebase Hosting project or served by the backend. CI: extend the repo's GitHub Actions with `tt/` build+test.

---

### 7.1 OAuth / OIDC integration (auth.bitwave.io)

- **Protocol:** OIDC Authorization Code flow **with PKCE**, confidential
  client. Client ID `vwp-tt`; client secret held in GCP Secret Manager,
  injected into Cloud Run as an env var (never in the repo or image).
- **Endpoints:** resolve via OIDC discovery
  (`https://auth.bitwave.io/.well-known/openid-configuration`) at boot;
  fail closed if discovery is unreachable. Do not hardcode authorize/token
  URLs. *(Build agent: confirm the discovery document exists; if
  auth.bitwave.io is OAuth2-only without OIDC discovery, fall back to
  explicitly configured `authorize`/`token`/`userinfo`/`jwks` URLs in env
  config — the adapter takes either.)*
- **Redirect URIs** (register with the IdP): 
  `https://<cloud-run-domain>/api/auth/callback` (prod),
  `http://localhost:8080/api/auth/callback` (dev). No wildcard URIs.
- **Scopes:** `openid profile email`.
- **Login flow:** `GET /api/auth/login` → 302 to authorize endpoint
  (state + nonce + PKCE) → callback exchanges code server-side → verify ID
  token (issuer, audience `vwp-tt`, nonce, signature via JWKS, `exp`) →
  upsert `users` by `sub` → create platform session (httpOnly, Secure,
  SameSite=Lax cookie). Access/refresh tokens, if retained for future
  Bitwave API calls, are encrypted at rest in `vault.provider_keys`-style
  storage; v0.1 does not call Bitwave APIs and SHOULD discard them.
- **Provisioning:** first login creates a `users` row with **no roles**
  (deny-by-default). An admin grants roles in-app. Optional allowlist:
  restrict login to configured email domains (`bitwave.io` initially) —
  external judges/preparers are added to the allowlist per §5.1
  provisioning when the pool goes external.
- **Logout:** clear platform session; call the IdP end-session endpoint if
  discovery advertises one.
- **Session policy:** 12 h absolute lifetime, 1 h idle refresh against the
  IdP (silent re-auth via refresh token when retained, else re-login);
  judge and vault routes re-check roles per request, not per session.
- **`llm_runner` exception:** machine accounts never touch OAuth — API keys
  only, scoped per enrollment, hashed at rest (§12). This keeps external
  agent operators out of the Bitwave IdP entirely.
- **Dev mode:** `AUTH_MODE=local` enables a seeded local-users bypass for
  tests/CI and offline dev (Playwright E2E must not depend on the IdP);
  refuses to start with `AUTH_MODE=local` in prod (checked against the
  Cloud Run service env).
- **TOTP:** dropped — MFA is the IdP's responsibility.

## 8. Data model

DDL sketch (build agent: generate Spanner GoogleSQL migrations from this; add `created_at/updated_at/created_by` everywhere — timestamps as `TIMESTAMP` with `allow_commit_timestamp` where they record writes; UUIDv4 string PKs; `jsonb` below maps to Spanner `JSON`; `unique` maps to a unique index):

```sql
-- vwp-tt-app database
users(id, oidc_sub unique, email, name, status)  -- provisioned on first SSO login, §7.1
roles(user_id, role)                         -- global roles
challenge_roles(challenge_id, user_id, role) -- per-challenge role incl. conflicts

challenges(id, slug, title, area, status,    -- draft|frozen|live|retired|published
  mode,                                      -- calibration|head_to_head (§6.0a)
  stage,                                     -- structure|human_arm|reconcile|model_batch|judging|shadow|done (§6.0)
  profile, extensions jsonb, brief_md, deliverable_spec_md, sources_md,
  time_budget_min, family_id?, variant_seed jsonb?, frozen_at, seal_hash)

evidence_files(id, challenge_id, evidence_id, title, type, storage_key,
  sha256, mime, page_count?, text_extracted bool)
evidence_pages(evidence_file_id, page_no, text)   -- for anchor validation + viewer

required_fact_slots(id, challenge_id, slot, fact_type, value_shape,
  unit?, enum_values jsonb?, description, keyed bool)

enrollments(id, challenge_id, user_id?, api_key_id?, arm) -- human|llm
submissions(id, challenge_id, enrollment_id, run_no, package_sha256,
  storage_key, provenance jsonb,            -- model/provider extracted from package
  verify_ts jsonb, verify_go jsonb, status, submitted_at, time_log jsonb)
work_products(id, submission_id, opaque_id, normalized_render_key)

test_definitions(id, code, dimension, mode,  -- M|H|X
  question, operationalization_md, active, version)
machine_test_results(id, work_product_id, test_code, verdict, detail jsonb, ran_at)
judge_tasks(id, work_product_id, test_code, judge_user_id, order_index,
  status, verdict?, rationale?, time_spent_s, vault_slice jsonb?)
shadow_judgments(id, work_product_id, test_code, llm_config_id,
  verdict, rationale, prompt_hash, tokens jsonb)
llm_configs(id, provider, model, prompt_template_id, purpose) -- judge|runner|perturber
prompt_templates(id, purpose, version, body)

model_batches(id, challenge_id, stage,        -- submission|shadow_judge
  status, started_at, completed_at, configs jsonb)
key_amendments(id, challenge_id, stage_at_amendment, diff jsonb,
  reason, author_user_id)                    -- post-stage-D writes rejected in the
                                             -- vault DAL transaction (Spanner has no
                                             -- triggers; the gate is application-
                                             -- enforced inside the same read-write
                                             -- txn that checks challenges.stage)
judge_findings(id, judge_task_id, seq, text, object_refs jsonb,
  canonical_finding_id?)
canonical_findings(id, work_product_id, label, description,
  merged_by_user_id)

agreement_reports(id, challenge_id, test_code, kappa, pct_agreement,
  n_items, computed_at)
automation_gate_reports(id, test_code, llm_config_id, kappa, arm_bias,
  n_items, eligible bool)

jobs(id, kind, entity_id, dedupe_key unique, status, attempts,
  detail jsonb, scheduled_at, completed_at)   -- Cloud Tasks idempotency ledger

audit_log(id, user_id?, api_key_id?, action, entity, entity_id, detail jsonb, at)

-- vwp-tt-vault database (separate IAM SA + Spanner database roles,
-- app-level AES-GCM encryption on value columns; §7 DB bullet)
reference_packages(id, challenge_id, author_user_id, package_sha256,
  storage_key_enc, status,                  -- draft|reconciled|frozen
  lint_result jsonb)                        -- §3.4: reference .vwp per key author
answer_keys(id, challenge_id, derived_from_reference_id, status,
  payload_enc, annotation_log jsonb)        -- answer-key.schema.json document,
                                            -- derived per §3.4; hand-edits logged
judgment_registry(id, challenge_id, payload_enc)  -- judgment points + defensible ranges
anomaly_registry(id, challenge_id, payload_enc)   -- seeded anomalies + match hints
canaries(id, challenge_id, canary_string_enc, locations jsonb)
watermarks(id, challenge_id, enrollment_id, mark_enc, issued_at)
provider_keys(id, provider, key_enc, label)
```

Package objects (facts, claims, …) are **not** row-modeled: the `.vwp` is the source of truth (consistent with the format's philosophy). The workbench keeps a draft JSON document per enrollment (`submission_drafts(enrollment_id, doc jsonb, autosaved_at)`) that serializes to package objects on submit. Machine tests parse the built package, not the draft.

---

## 9. API surface (REST, JSON; all under `/api`)

Auth: session cookie or `Authorization: Bearer <api-key>`. Errors RFC 7807.

```
GET    /auth/login            302 to auth.bitwave.io (OIDC code+PKCE, §7.1)
GET    /auth/callback         code exchange, session issue
POST   /auth/logout
GET    /me

# challenges (coordinator)
POST   /challenges                      create draft
PATCH  /challenges/:id                  edit draft fields
POST   /challenges/:id/evidence         multipart upload
DELETE /challenges/:id/evidence/:eid
PUT    /challenges/:id/required-facts   full manifest replace (draft only)
POST   /challenges/:id/freeze           validates freeze preconditions
POST   /challenges/:id/enrollments      {user_id|api_key, arm}
POST   /challenges/:id/lock-human-arm | /lock-key | /run-model-batch | /run-shadow-batch   # §6.0 stage gates
POST   /challenges/:id/retire | /publish
POST   /challenges/:id/variants         {k, perturbation_map?} -> family

# vault (coordinator, key_author; challenge-scoped)
GET/PUT /challenges/:id/vault/answer-key        per-author drafts
POST    /challenges/:id/vault/answer-key/reconcile
GET/PUT /challenges/:id/vault/judgment-registry
GET/PUT /challenges/:id/vault/anomaly-registry

# participants
GET    /challenges/:id/packet           brief+spec+manifest+evidence urls (watermarked)
GET/PUT /challenges/:id/draft           workbench autosave (human arm)
POST   /challenges/:id/submissions      workbench submit OR multipart .vwp upload (llm arm)
GET    /submissions/:id/receipt         verify status (+tests in practice mode)

# judging
POST   /challenges/:id/assignments      generate judge tasks
GET    /judge/queue
GET    /judge/tasks/:id                 blinded WP render + test card + vault slice
POST   /judge/tasks/:id/verdict         {verdict, rationale}

# results
GET    /challenges/:id/results          role-gated depth
GET    /challenges/:id/results/export   zip: json+csv+packages(+vault if published)
GET    /reports/automation-gate?test=J3&config=...
```

---

## 10. UI screens

1. **Challenge list & detail** (role-aware): status pipeline widget, freeze-precondition checklist, enrollment table, model-exposure ledger.
2. **Evidence manager:** upload, per-file hash/pages, canary status, text-extraction status.
3. **Manifest editor:** required-facts table with valueShape-aware validation.
4. **Vault editors:** answer key (two-author columns + reconciliation diff), judgment registry, anomaly registry. Visually distinct (dark border, "VAULT" banner) so screenshots are recognizable as sensitive.
5. **Preparer workbench:** three-panel — evidence viewer (PDF with text selection → anchor capture) | item editors with tabs (Facts/Claims/Judgments/Calculations/Memo) | to-do rail (slot coverage, unanchored-fact warnings, unbalanced-JE warnings). Autosave, time log.
6. **Judge queue & task view:** as §5.3. Keyboard-first (P/F, next).
7. **Coordinator pipeline dashboard:** per-submission verify + machine-test lights; judging progress; agreement stats.
8. **Results dashboard:** pass matrix (arms × 18 tests), drill-downs, export.
9. **Admin:** users, roles, API keys, LLM configs, prompt templates.

---

## 11. Machine-test runner details (build notes)

- **G2 quote matching:** normalize NFC, collapse whitespace, strip soft hyphens; exact substring against the cited page's text (±1 page tolerance for PDF extraction drift); for csv/xlsx anchors, `cells` resolution via SheetJS. Emit per-fact match records.
- **K2:** port `tools/score-key.mjs` comparison semantics into `tt/server`; keep the CLI tool working (shared lib in `tt/shared` or `benchmark/tools/lib`).
- **K3:** run `vwp verify` with the challenge's profile/extensions; map validator IDs → K3; REVIEW ≠ FAIL.
- **S1:** join keyed conclusions across a family for the same enrollment identity (same model+config for LLM arm); flag flips.
- **S2:** duplicate factType conflict check; numeric-literal extraction from claim text and memo (regex `[$€]?[\d,]+(\.\d+)?` with context window) compared against calculation outputs for the same labeled concept — conservative: only flag exact-label matches; everything else surfaces as INFO for judges.
- **S3 machine pre-work:** for each anomaly's match hints (strings, evidence IDs, amounts), search facts/judgments/claims/memo; produce candidate-mention list for the judge.
- All runners write `machine_test_results.detail` rich enough to render drill-downs without recomputation.

## 12. Security & integrity requirements

- `.vwp` uploads are untrusted: size caps (100 MB), zip-slip rejection (CLI already does), no execution of contents, MIME sniffing on evidence.
- Vault: separate Spanner database + separate IAM service account + database-role scoping + app-layer encryption; vault material never in logs; audit_log row for every vault read.
- Judge task payloads must be provably free of arm/participant identifiers (unit test: serialize every judge-facing response and assert no enrollment/user/provenance fields).
- Watermark issuance on every evidence download (per §4.3); downloads logged.
- Rate limits on auth and submission endpoints; API keys hashed (argon2id).
- OAuth hardening (§7.1): state + nonce + PKCE enforced; ID-token audience must equal `vwp-tt`; JWKS cached with kid-rollover handling; client secret only in Secret Manager; deny-by-default role provisioning on first login.
- Determinism: machine tests versioned (`test_definitions.version`); rerunning a test on the same package must reproduce verdicts; results rows are append-only (supersede, never update).

## 13. Build phases

**Phase 1 — Skeleton + machine spine (the demo that proves the loop).**
OIDC auth against auth.bitwave.io (client `vwp-tt`, §7.1) + local dev mode + RBAC, challenge CRUD, evidence upload with PDF text extraction, manifest editor, freeze lifecycle, LLM-arm submission endpoint (upload `.vwp`), pipeline (CLI build/verify + Go verify + G1, C1, K1, K3, P1), coordinator dashboard. Seed with `rev-606-saas` (evidence + manifest + key exist in repo). *Exit: upload the repo's example package against the seeded challenge and see a test-vector row.*

**Phase 2 — Anchors + key tests + workbench + reference-solution pipeline.**
G2 validator, anchor capture in a PDF viewer, the full preparer workbench (facts/claims/judgments/calculations/memo, autosave, submit-through-assembly), reference-package authoring + object-level diff/reconciliation + structure derivation + key lint (§3.4), vault annotation editors + K2, S2. *Exit: a key author can structure a brand-new challenge entirely through the reference-solution pipeline (no hand-written answer-key JSON), the n-way reconciliation works with 4 authors (calibration mode), a human can complete rev-606-saas end-to-end in the browser, and machine tests run on the result. This is the pilot-readiness gate (§6.0a).*

**Phase 3 — Judging + stage gates.**
Normalized render, assignment generation with conflict matrix, judge queue/task UI, verdicts, agreement stats, findings segmentation + canonical-finding dedupe UI, §6.0 stage-gate enforcement (human-arm lock → key-amendment lock → model batch → verdict lock → shadow batch), results dashboard with pass matrix, retire/publish + export.

**Phase 4 — Consistency + shadow judging + built-in runner.**
Variant generator + S1, S3 pre-work, shadow-judge harness + automation-gate report, built-in LLM runner (Flow C2), watermarking, canary probes tooling.

Each phase lands with wrench migrations (run against the Spanner emulator in CI), seeds, unit tests on runners (fixtures from `testdata/` + the ASC 606 example), one Playwright E2E per flow, and updated `tt/README.md`.

## 14. Acceptance criteria (v0.1 done means)

1. A coordinator can take a PDF-based fact pattern from upload to `live` entirely in the UI, with freeze preconditions enforced.
2. A human preparer can produce a submission that builds into a `.vwp` passing `vwp verify` on both implementations, with ≥90% of facts anchored via PDF text selection.
3. An external agent can fetch a packet and submit a `.vwp` via API; provenance-missing packages are rejected.
4. All 9 machine tests (G1, G2, C1, K1, K2, K3, S1, S2, P1) reproduce deterministically; the 8 submission-time ones run automatically on submit, and S1 runs automatically when its variant family completes.
5. Three judges can blind-judge the 9 H/X tests (G3, G4, C2, C3, J1, J2, J3, S3, P2) with enforced rationales; κ and agreement compute correctly (validated against a hand-computed fixture).
6. No API response reachable by preparer/llm_runner/judge-pre-assignment roles ever contains vault material (covered by tests).
7. Results dashboard renders the arms × tests matrix and exports the publication bundle.
8. The rev-606-saas seed works end-to-end as the reference E2E.
9. A completely novel challenge can be structured through the reference-solution pipeline (§3.4): two reference packages authored in the workbench, reconciled, manifest/key/registries derived, key lint enforced at freeze — with zero hand-authored answer-key JSON.
10. Variant re-derivation reproduces a linted answer key for every generated variant, and a deliberately key-breaking perturbation map is rejected by per-variant lint (negative test).
11. Stage gates enforce §6.0: the model batch cannot start before the human-arm and key-amendment locks; a key amendment attempted after stage D is rejected (negative test); the shadow-judge batch cannot start while any human judge queue is open (negative test).
12. Judge rationales can be segmented into findings and deduped into canonical findings, with every canonical finding linked back to its source rationales.
13. A calibration-mode challenge runs end-to-end with 4 human extractions reconciled via the n-way diff into a linted key, followed by a scored model batch — the Bitwave pilot flow (§6.0a) — using only Phase 1–2 functionality.
14. The full test suite (unit + Playwright E2E) runs against the Spanner emulator with `JOBS_MODE=inline` — no cloud dependency in CI; the stage-D key-amendment gate holds under concurrent transactions (write attempted in the same commit window as the stage flip — negative test).
15. Human sign-in works only via auth.bitwave.io SSO (client `vwp-tt`): first login provisions a role-less user; role grant is required before any challenge access (negative test); `AUTH_MODE=local` is rejected in the prod environment (negative test); `llm_runner` API keys authenticate without OAuth.

## 15. Open questions (decide before or during Phase 1)

1. **Battery name final?** VWP-TT used throughout; swap early if desired.
2. **Judge n:** default 3 — for the in-house pilot (4 CPAs, RUNBOOK matrix) n=2 with documented limitation?
3. **Anchor threshold for G2** (90% default) — per-challenge or global?
4. **Should S1 variants be mandatory for the human arm?** (Cost: each human does k× work. Recommend: LLM arm always, human arm 1 variant only, disclosed asymmetry.)
5. **Expression DSL for calculations:** reuse exactly the CLI's recompute semantics (confirm what `spec/calculations.md` fixes) vs. extending — any extension must land in the spec first.
6. **Where do published challenges live** — `benchmark/challenges/` in-repo (current convention) or a separate public data repo?

---

*This spec deliberately inherits `benchmark/DESIGN.md`'s validity-threat table (§6) and RUNBOOK's integrity rules; where this document and those conflict, this document governs for the platform, and DESIGN.md governs for methodology claims in publications.*
