# Extraction worksheet

The worksheet is the CPA arm's structured capture form — a machine-readable
tickmark legend. One row per slot in the challenge's `required-facts.json`.
CPAs complete it alongside their memo and schedules; a coordinator transcribes
it into `facts/facts.json` with `tools/transcribe-worksheet.mjs`.

## How citations work

CPAs receive **numbered copies** of the evidence (produced by
`tools/number-evidence.mjs`): every text document rendered with page breaks
and margin line numbers, like a deposition transcript. Citing is therefore
just `evidence_id` + `page` + `lines` — no anchor syntax, no copying quotes.
At transcription time (`--evidence <dir>`), the pin cite is resolved against
the same numbering convention and the cited text is filled in as a
machine-verifiable quote anchor automatically. A pin cite that falls outside
the document fails transcription loudly.

`verbatim_quote` remains available for the rare case where a cite needs to be
tighter than whole lines (e.g. one clause on a dense line); when present it
overrides the resolved text.

## Columns

| Column | Required | Meaning |
|---|---|---|
| `slot` | yes | Slot name from the challenge's required-facts manifest. |
| `statement` | yes | The observation in your own words, as you would state it in a workpaper. |
| `value` | yes | The value: a number for money/quantity slots, one of the allowed values for enum slots, a date for date slots. |
| `currency_or_unit` | for money/quantity | `USD`, `months`, `BTC`, … |
| `evidence_id` | yes | The document id printed on each evidence file in the challenge bundle. |
| `page` | if applicable | Page in the cited document. |
| `lines` | if applicable | Line or line range, e.g. `12` or `12-14`. Relative to `page` when a page is given, otherwise counted from the top of the document. |
| `cells` | if applicable | Cell or range for spreadsheet evidence, e.g. `B7` or `Rates!B2:B13`. |
| `section` | if applicable | Section/clause label, e.g. `2(a)`. |
| `verbatim_quote` | optional | Usually left blank — the quote is derived from your page/line cite. Fill it only to cite something narrower than whole lines. |
| `notes` | no | Anything a reviewer should know (ambiguity found, anomaly investigated, alternative considered). |

Derived slots (period revenue, ending balances) are computed on your schedule
deliverable, not extracted from evidence; for those rows cite the evidence the
computation rests on and leave `verbatim_quote` blank, or reference your
schedule in `notes`.

The template ships with one example row — delete it before submitting.
