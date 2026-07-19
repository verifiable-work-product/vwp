# VWP-Bench — Preparer instructions

You are preparing an accounting work product from a synthetic evidence
bundle, exactly as you would for a client file. Your work will be graded
blind against a rubric by CPAs who don't know which work products came from
humans and which from software. Work alone.

## Your packet

- `brief.md` — the engagement: what happened, what deliverables are due.
- `numbered-evidence/` — the source documents, rendered with page and line
  numbers in the margin (like a deposition transcript). These are your
  citable copies.
- `extraction-worksheet.csv` — a short structured worksheet (see below).

## What to produce

1. **The deliverables in the brief** — typically a memo, supporting
   schedule(s), and journal entries. Use Word/Excel/Google Docs, your normal
   style, as if a manager will review it. Cite evidence in your memo the way
   you normally would.
2. **The extraction worksheet** — one row per listed slot. For each: the
   value you concluded, and where it comes from — document id, page, line(s),
   read straight off the numbered copies. Example:

   | slot | value | currency_or_unit | evidence_id | page | lines |
   |---|---|---|---|---|---|
   | transactionPrice | 300000 | USD | ev_order_form | 1 | 16 |

   Think of it as a tickmark legend in spreadsheet form. Derived values
   (period totals, ending balances) come from your own schedule — cite the
   evidence the computation rests on and say "per my schedule" in notes.
3. **A time log** — start/stop times, rounded to 15 minutes.

## Ground rules

- **Time budget: 4 hours** (hard stop at 6). Bill all of it; this is paid
  work at your normal rate. Quality within the budget is the exercise —
  don't gold-plate past it.
- **No generative AI.** No ChatGPT, Claude, Copilot, or AI features inside
  other tools. Everything else you'd normally use is fine: Excel, the FASB
  codification, firm guides, EY/PwC/KPMG/Deloitte publications, a calculator,
  your own brain.
- **Don't discuss the challenge with anyone** — especially not the other
  participants — until the whole run is closed.
- **The evidence may contain problems.** Treat inconsistencies the way you
  would in practice: investigate, resolve if you can, and document what you
  found and what you'd do about it (memo or worksheet notes). Silence about
  a real problem costs points; a documented open item does not.
- **Where GAAP admits more than one supportable position**, pick one and
  document your basis. Documented reasonable positions earn credit even when
  they differ from the primary key answer.

## What happens to your work

Your memo and schedules are re-typeset into a neutral format (so graders
can't tell who or what produced any work product) and graded blind against
the same rubric as everything else. Your worksheet is machine-transcribed:
every page/line cite you give is resolved to the cited text and checked
against the evidence automatically. You'll see your own results and the
anonymized distribution when the run closes.
