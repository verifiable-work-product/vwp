# VWP-Bench rubric

The published scoring rubric for VWP-Bench (research proposal, August 2026).
Every work product in a study — model runs and the CPA control group alike —
is scored against it by two independent CPA graders who do not know who
prepared it. This directory is the open half of the grading protocol: the
rubric, the sheet format, and a reference scorer, so anyone holding the
published sheets can recompute every number in a results table.

| File | Purpose |
|---|---|
| [`vwp-bench-rubric.v1.json`](vwp-bench-rubric.v1.json) | The rubric: seven areas, 100 points, the signability scale and its cap conditions. Schema: [`../schemas/rubric.schema.json`](../schemas/rubric.schema.json). |
| [`../schemas/grade-sheet.schema.json`](../schemas/grade-sheet.schema.json) | One grader's sheet for one blinded work product. |
| [`../tools/score-rubric.mjs`](../tools/score-rubric.mjs) | Reference scorer: one sheet → score; two sheets (+ tiebreak) → disagreements and consensus; `--lint` a rubric. Logic in [`../tools/lib/rubric.mjs`](../tools/lib/rubric.mjs). |

The rubric, weights, answer key, and tie-breaking rules are **fixed before
any model sees the case**. A study cites the rubric name and version it
froze with; a later revision is a new version, never an edit.

## The rubric (v1.0)

| Area | Points | Scored as |
|---|---|---|
| Material facts and evidence | 20 | Proportional credit for required facts correctly stated and supported by the source material. The private case key lists the required facts; graders accept any expression that adequately captures a keyed fact. 6 of 8 → 15/20. |
| ASC 606 analysis | 25 | Five 5-point yes/no items — contract, performance obligations, transaction price, allocation, timing/pattern of recognition. Both the answer and its material reasoning must be correct. Other accounting areas swap in their own five items. |
| Calculations and schedules | 10 | Proportional credit for correct required calculations and schedules, within tolerances fixed before grading. Calculations must also be reproducible. |
| Traceability and documentation | 10 | Four 2.5-point yes/no items: source links resolve; sources support the stated facts; calculations link to their inputs; conclusions link to facts and guidance. |
| Judgments and anomaly response | 10 | Five 2-point yes/no items: issue identified; investigated; considered against contrary evidence; documented; reasonably resolved. |
| Memo | 5 | 0–5 rating: does the memo clearly and accurately reflect the work recorded in the rest of the VWP? |
| Overall signability | 20 | 0–5 rating × 4 (scale below). |
| **Total** | **100** | Component scores and total are both published. |

**Signability scale.** 5 signable as submitted with no comments · 4 signable
after minor or editorial comments · 3 limited substantive changes required ·
2 material correction or partial re-performance required · 1 extensive
re-performance required · 0 unusable, nonresponsive, or materially
incomplete.

**Cap conditions.** A materially wrong conclusion, an unsupported material
conclusion, a missing required deliverable, or an unreproducible material
calculation **cannot receive a 4 or 5**, regardless of the total score. A
grader flags the condition on the sheet; the scorer holds the signability
rating at 3.

## Scoring rules

- **Yes/no items** earn full or zero credit. Nothing in between.
- **Count items** earn proportional credit: `points × captured / total`,
  where `total` is the count in the private key.
- **Ratings** convert to points by the area's fixed multiplier
  (memo × 1, signability × 4).
- **Cap conditions** apply to the signability rating only, after grading.
- **Two blind graders** score every work product independently. If they
  disagree on an item, a **third CPA decides that item** — only that item —
  without seeing either primary's answer. Consensus is the agreed items plus
  the third grader's decisions; a work product has no score until every
  disagreement is resolved.
- **Machine tests are pre-work, not scores.** Deterministic checks (do the
  source links resolve, do the calculations recompute, do keyed values match)
  run on every package and are shown to graders beside the area they inform.
  A grader decides every item.
- **Safeguard against double-counting.** Grader instructions name the single
  area in which an underlying mistake is deducted; the same mistake is not
  deducted again elsewhere. (The rubric's area guidance carries these rules
  per study.)

## Publication

Every scheduled run is published with its component scores, total, and
signability rating. Runs are never selected or repaired after scoring. A
model's ranking score is the average of all its scheduled runs; models are
listed highest to lowest. The CPA control group's scores appear beside the
ranking as a reference range — the CPAs are a control, not a competitor. The
highest-ranked model is the best performer *on this case, under this harness
and these rules*; the study makes no claim about accounting ability in
general.

## Recomputing a published score

```bash
# one grader's sheet
node benchmark/tools/score-rubric.mjs benchmark/rubric/vwp-bench-rubric.v1.json sheet-a.json

# two blind primaries plus the third grader's decisions → consensus
node benchmark/tools/score-rubric.mjs benchmark/rubric/vwp-bench-rubric.v1.json \
  sheet-a.json sheet-b.json --tiebreak sheet-tiebreak.json

# check a rubric's structure (points vs items, multipliers, cap areas)
node benchmark/tools/score-rubric.mjs --lint benchmark/rubric/vwp-bench-rubric.v1.json

# run the reference tests
node --test benchmark/tools/
```

## Relationship to the rest of the benchmark

- [`../DESIGN.md`](../DESIGN.md) §5 — arms, blinding, graders, and the
  publication table this rubric serves.
- [`../schemas/answer-key.schema.json`](../schemas/answer-key.schema.json) —
  the private case key graders hold (required facts with leeway, keyed
  calculations with tolerances, judgment points with their defensible range).
- [`../fact-catalog/`](../fact-catalog) — the public catalog both arms
  receive instead of the required-facts list.
- The VWP-TT battery of machine tests (G1 … P2) supplies each area's
  `preWorkTests`. The battery's human-judged tests (G3, G4, C2, C3, J3, P2)
  are retained as future work and are not part of the published score.
