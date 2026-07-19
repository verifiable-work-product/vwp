# `rev-606-saas` — coordinator notes

## Preparer packet (what a CPA receives)

- `brief.md`
- `numbered/` — the citable numbered copies (committed so line cites stay
  stable; regenerate only if evidence changes, and never mid-run)
- `extraction-worksheet.csv` — slot column prefilled
- `../../templates/cpa-instructions.md`

Preparers do NOT receive: `evidence/` raw files (the numbered copies are
their reading set), `required-facts.json` beyond the worksheet slots,
anything under `benchmark/answer-keys/` (gitignored; never in the shared
drive preparers use).

## Agent packet

- `brief.md`, the raw `evidence/` files with `evidence.json`, and
  `required-facts.json`. The agent produces a `.vwp` package; three
  independent runs; no touch-ups.

## Coordinator commands

```bash
# regenerate numbered copies (only if evidence changed)
node ../../tools/number-evidence.mjs evidence numbered

# transcribe a returned CPA worksheet into anchored facts
node ../../tools/transcribe-worksheet.mjs <returned-worksheet.csv> \
  required-facts.json --evidence evidence > facts.json

# score any work product's keyed items (agent package dir or transcribed facts)
node ../../tools/score-key.mjs <facts.json | package-dir> \
  ../../answer-keys/rev-606-saas/answer-key.json required-facts.json
```

## Status

- Evidence bundle: complete (9 documents, canary `VWPB-R606-7C41`)
- Answer key: DRAFT — awaiting independent reconciliation by the two
  assigned key authors, then freeze
- QA: perfect-path worksheet scores 8/8 correct; wrong-path worksheet
  discriminates (term alternatives flagged for grader review, missed
  anomalies score incorrect)
