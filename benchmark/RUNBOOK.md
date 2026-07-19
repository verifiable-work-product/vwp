# VWP-Bench pilot runbook — 4 in-house CPAs

Operational plan for running the pilot with four CPAs (call them A, B, C, D).
Nobody may grade or key a challenge they prepared, so everyone wears two
hats across different challenges.

## Assignment matrix

| Challenge | Preparers (blind) | Key authors → graders |
|---|---|---|
| `rev-606-saas` | A, B | C, D |
| `fv-820-digital` | C, D | A, B |
| `dep-360-fleet` | A, C | B, D |
| `re-805-acquisition` | B, D | A, C |
| `crypto-350-60` | A, D | B, C |

Each CPA prepares 2–3 challenges (~4 hrs each) and keys/grades 2–3
(~2 hrs each): roughly 15–18 hours per person across the pilot.
n = 2 human work products per challenge (vs. 3 in DESIGN.md) — acceptable
for a pilot; say so in the report.

Staffing rule: match specialization where it exists (digital-asset
challenges get at least one digital-asset-experienced preparer and grader).

## Sequence per challenge

1. **Coordinator** (not a CPA participant) finalizes the fact pattern and
   evidence bundle; runs `tools/number-evidence.mjs` to produce the numbered
   copies; assembles preparer packets (brief + numbered evidence + worksheet
   + `templates/cpa-instructions.md`).
2. **Key authors** independently draft expected values for every keyed slot,
   then reconcile in one sitting; disagreements become documented
   `alternatives` with `requiredDocumentation` in `answer-key.json`
   (validated against `schemas/answer-key.schema.json`). The key is frozen
   before any preparer packet goes out.
3. **Preparers** work alone under `cpa-instructions.md`: deliverables +
   extraction worksheet + time log. No genAI, no discussion.
4. **Agent arm** runs 3× from the identical packet (brief + raw evidence —
   agents get the un-numbered originals plus the required-facts manifest);
   packages verified with `vwp verify`; no human touch-ups.
5. **Coordinator** transcribes worksheets
   (`tools/transcribe-worksheet.mjs --evidence <dir>`), assembles human
   packages, runs `vwp verify` on all packages, runs `tools/score-key.mjs`
   for keyed items, and re-typesets all memos/schedules into the neutral
   format with opaque IDs.
6. **Graders** (the challenge's key authors) grade all normalized work
   products blind against the rubric, independently, no conferring;
   coordinator collects sheets and computes agreement.
7. **Debrief** after all challenges close: reveal, per-challenge results,
   seeded-anomaly detection, review-time study on a subset.

## Integrity rules

- The answer key and seeded-anomaly list never touch a preparer's inbox;
  keys live outside the shared drive the preparers use.
- Preparer identities are opaque IDs from step 5 onward; the coordinator
  holds the mapping alone.
- All agent runs are graded — no rerunning a bad run.
- Time logs are recorded as reported; don't round up quality time.

## Known limitation to disclose

In-house CPAs grading an in-house agent is a fair objection to *published*
claims even with blinding and format normalization. Use the pilot to debug
the harness, the rubric, and the time budgets. For the publishable run,
recruit external preparers and graders per DESIGN.md §4–5.
