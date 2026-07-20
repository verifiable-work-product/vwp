# Format assessment: what VWP-Bench exposes about VWP v0.1

Reviewing the v0.1 format against the benchmark design (see
[DESIGN.md](DESIGN.md)) — where the format holds up, and four gaps the
benchmark forces us to close. Each gap is also a real-world gap; the
benchmark just makes it concrete first.

## What holds up

The core chain — `evidence -> facts -> claims -> judgments/calculations` —
survives the broader lens. The ASC 606 example shows facts doing real work:
typed-ish values feeding a calculation that a validator ties out, claims
carrying assertions and standard references, an unresolved judgment
surfacing in verification. Provenance already distinguishes
`human | agent | system` creators, so CPA-prepared packages are
representable today.

## Gap 1 — Facts cite files, not places in files

`evidenceRefs` points at whole evidence objects. `f_contract_value` cites
`ev_order_form` — acceptable for a 3-page order form, useless for the
40-page PSA in `re-805-acquisition` or a 5,000-row transaction export in
`crypto-350-60`. A grader (or auditor) verifying traceability has to hunt.
This is the "formalized markers for extracted facts" problem.

**Proposal.** Allow `evidenceRefs` items to be either a string (today's
form, unchanged) or an object:

```json
{
  "evidenceId": "ev_psa",
  "anchor": {
    "quote": "Buyer shall pay to Seller the sum of $14,250,000",
    "page": 3,
    "section": "2(a)"
  }
}
```

Anchor vocabulary: `quote` (verbatim excerpt), `page`, `lines`,
`cells` (sheet + range), `jsonPath`, `section`. All optional; `quote` is the
important one, because it enables a **new deterministic validator**: for
text-typed package-local evidence, the quoted bytes must appear in the cited
file. "Is this fact actually in the evidence" stops being a purely human
check — which is exactly the whitepaper's "AI builds; deterministic software
verifies" posture extended one level deeper. This pulls the ROADMAP's v0.3
"evidence locator standard" forward; the benchmark needs it in the pilot.

## Gap 2 — No canonical fact slots, so work products aren't comparable

Two preparers given the same evidence will extract different fact sets with
different ids and free-text statements. Fine for a single work product;
fatal for grading keyed items across six work products per challenge, and a
real-world problem too (an auditor comparing this year's package to last
year's wants `transactionPrice`, not whatever id the preparer invented).

**Proposal, two levels:**

- **Format level:** add an optional `factType` field to facts, and let
  extensions declare fact-type value schemas the way `accounting-asc606`
  already declares a calculation schema (`ssp-allocation`). An extension
  validator can then check that a package includes the fact types the work
  product type requires, with values of the right shape. Alongside this,
  document conventional value shapes (`money {amount, currency}`,
  `quantity {value, unit}`, `period {from, to}`) — today `value` is
  untyped `any` and the example already improvises inconsistently
  (`{amount, currency}` vs `{months}`).
- **Benchmark level:** each challenge brief includes a **required-facts
  manifest** — named slots with types and units (e.g. `transactionPrice:
  money`, `contractTermMonths: quantity`, `screenTestConclusion: enum`).
  The agent fills them as typed facts; CPAs fill them via the worksheet
  (Gap 3). Keyed-item scoring then becomes deterministic: compare slot
  values against the answer key with declared tolerances.

## Gap 3 — No authoring path for humans

CPAs will not write JSON, and nothing in v0.1 pretends they will. For the
benchmark (and for any human-in-the-loop adoption story) we need a capture
mechanism whose mental model CPAs already have: the tickmark.

**Proposal.** A per-challenge **extraction worksheet** (spreadsheet
template) mirroring the required-facts manifest, one row per fact slot:

| slot | value | source document | page/cell | verbatim quote |
|---|---|---|---|---|

CPAs complete it as part of their deliverables — it is a formalized tickmark
legend, not a new skill. A coordinator (or script) transcribes worksheets
into `facts.json` with `extractionMethod: "manual"`, provenance
`createdBy: {type: "human"}`, and the transcription tool recorded in
`toolchain`. Both arms' keyed items then run through the same deterministic
scorer, and the review-time study can compare like against like.

## Gap 4 — Answer keys should be machine-checkable artifacts

If keyed items are typed fact slots (Gap 2), the answer key itself should be
a schema'd object: per slot, the expected value, tolerance, and documented
acceptable alternatives (each with the documentation it requires). Propose a
`benchmark-key` extension schema so objective scoring is deterministic,
auditable, and publishable with the results — and so the benchmark
infrastructure is itself a consumer of the format.

## Drift found while reviewing (fix regardless)

- `spec/facts.md` field table marks `evidenceRefs` **required**, but
  `fact.schema.json` requires only `id` and `statement` — the validator
  cannot catch an unsupported fact today. Align (schema-require
  `evidenceRefs`, keep "SHOULD be non-empty" as the profile check, or
  soften the spec table).
- `extractionModel` appears in `fact.schema.json` and in the spec's
  semantics section, but is missing from the spec's field table.

## Status (2026-07-19): all four gaps closed in v0.1 draft

1. **Drift** — fixed. `fact.schema.json` now requires `evidenceRefs`
   (non-empty); `extractionModel` added to the spec field table.
2. **Anchors (Gap 1)** — implemented. `evidenceRefs` entries may be
   `{ evidenceId, anchors: [...] }`, one anchor per cited chunk, each with
   `quote`/`page`/`lines`/`cells`/`charSpan`/`byteRange`/`section`/
   `jsonPath`/`region`; new deterministic validator
   `core.factAnchors.quotesMatch` checks quotes against text evidence after
   spec-frozen normalization; evidence gained `derivedFrom` + `transform`
   for text layers extracted from binaries. See spec/facts.md §4,
   spec/evidence.md §4, ADR 0003, and the anchored ASC 606 example.
3. **Fact types (Gap 2)** — implemented at the format level (`factType`
   field, conventional value shapes in spec/facts.md §5) and at the
   benchmark level (`schemas/required-facts.schema.json`, first manifest at
   `challenges/rev-606-saas/required-facts.json`). Remaining: manifests for
   the other four challenges, authored with their fact patterns.
4. **Human authoring (Gap 3)** — implemented: `templates/` extraction
   worksheet + `tools/number-evidence.mjs` (deposition-style numbered copies
   of the evidence for CPAs to cite) + `tools/transcribe-worksheet.mjs`
   (worksheet CSV → schema-valid anchored facts; page/line pin cites are
   resolved against the numbering convention and become machine-verifiable
   quote anchors automatically — humans never author anchors).
5. **Machine-checkable keys (Gap 4)** — implemented:
   `schemas/answer-key.schema.json` + `tools/score-key.mjs` (deterministic
   keyed-item scoring with tolerances and documented acceptable
   alternatives). Note: key schemas live under `benchmark/schemas/`, not as
   a package extension — the answer key is benchmark infrastructure, not
   package content.
