# VWP primer (v2)

VWP — Verifiable Work Product — is a package format for professional work in
which a memo's every load-bearing element is a machine-checkable object. A
`.vwp` package is a claim graph over evidence, and an independent verifier can
deterministically re-check its structure: do the citations resolve, do the
quotes actually appear in the cited documents, do the schedules tie out, do
the conclusions link to support? Your job is to produce work whose quality
survives that inspection.

## The construct chain

Work flows through five object kinds, each anchored to the one before it:

**Evidence → Facts → Calculations / Judgments → Claims (assessments) → Memo**

1. **Evidence** is the source material: the documents in the packet. Evidence
   is read-only; you never edit it, you cite into it. Each document has an id
   (e.g. `ev_msa`) and a text layer — the extracted, line-numbered text you
   read with `read_evidence`. The text layer is the byte surface verification
   checks quotes against.

2. **Facts** are observations extracted from evidence — not conclusions. "The
   order form states an annual subscription fee of $180,000" is a fact; "the
   implementation services are distinct" is not (that is a judgment or
   claim). Every fact carries at least one **anchor**: a verbatim quote from
   the cited evidence, optionally with page and line range. Verification
   re-finds the quote in the evidence bytes; whitespace, line breaks,
   curly-vs-straight quote marks, and dash styles are normalized before
   matching, but the words must be exactly as written. Facts also carry a
   `factType` — a namespaced label (e.g. `asc606.transactionPrice`) that lets
   independent preparers land the same economic fact under the same name —
   and a structured `value`. Conventional value shapes:
   - money: `{ "amount": 180000, "currency": "USD" }`
   - quantity: `{ "value": 36, "unit": "months" }`
   - date: `"2026-03-31"`; period: `{ "from": "...", "to": "..." }`
   - enum: one of the listed strings; boolean; string.
   The **required-facts manifest** in the packet lists the slots your work
   product must fill: for each slot, produce one fact with the manifest's
   exact `factType` and a value conforming to its `valueShape`. Do not create
   two facts of the same factType with conflicting values — if the evidence
   itself conflicts (e.g. an invoice disagrees with the order form), pick
   distinct factTypes or document the discrepancy explicitly rather than
   recording the same type twice with different numbers.

3. **Calculations** carry the arithmetic: declared `inputs`, declared
   `outputs`, and a `method`. Conventional types are recomputed by the
   verifier from your inputs — `sum`, `schedule` (rows × columns with row,
   column, and grand totals), `journalEntries` (entries with debit/credit
   lines, totalled), and `relativeStandaloneSellingPriceAllocation`
   (transaction price allocated pro rata over standalone selling prices, and
   the allocations must tie back to the transaction price). Outputs must
   reproduce from inputs within a 0.01 tolerance. Anything that does not fit
   a conventional type is `custom` — still declare inputs and outputs
   honestly.

4. **Judgments** are the points where reasonable professionals could
   conclude differently: distinctness of a performance obligation, the
   enforceable contract term under a termination clause, modification
   treatment. A judgment records the `question`, your `proposedConclusion`,
   the `basis` for the conclusion (with authority citations), and the
   **`alternatives`** — each alternative accounting position, itemized, with
   the reason it does not apply. The alternatives list is part of the
   deliverable contract: an empty alternatives list on a genuine judgment
   point reads as incomplete documentation.

5. **Claims** (created with `add_assessment`) are the assertions and
   conclusions of the work product — typically one per method step. A claim
   links to everything that supports it: `supportingFacts`, `judgments`,
   `supportingCalculations`, `supportingEvidence`, plus `standardRefs`
   (authoritative guidance, e.g. "ASC 606-10-25-19") and, in the finance
   profile, `assertions` (existence, occurrence, completeness,
   valuationAllocation, rightsAndObligations, presentationDisclosure). A
   claim with no support links is an **orphan** and fails validation. The
   support links are the point of the format: a reviewer clicks from your
   conclusion down to the exact quoted line of evidence.

6. The **memo** (`write_memo`) is the human rendering: the professional
   memo a reviewer reads, structured per the deliverable spec. Everything
   material in the memo should exist in the graph — a number in the memo
   that appears in no fact or calculation is unverifiable.

## What validation checks

`validate()` runs the full offline battery — exactly what a diligent
preparer could self-check, and nothing derived from any answer key:

- **Package verification** — schemas, part hashes, link resolution, and
  verbatim-quote anchors over the assembled package (the reference
  verifier).
- **G1** — every fact's evidence refs resolve; no fact without evidence.
- **G2** — every anchor quote is found in its evidence text; at least 90% of
  facts carry a quote anchor.
- **C1** — every claim's support links resolve; no orphan claims.
- **K1** — every conventional calculation recomputes from its inputs.
- **S2** — no duplicate factType carries conflicting values.
- **P1** — every required-facts slot is filled by a fact of the right
  factType whose value conforms to the declared shape.

Findings come back structured (check id, message). Fix all failures and
re-run `validate()` until `ok: true`, then `submit()`.

## Documentation expectations

Write like a professional whose work will be reviewed by a skeptical peer:
statements in plain declarative prose; amounts and dates exactly as
evidenced; authority cited at the paragraph that relies on it; discrepancies
in the evidence surfaced, not smoothed over. Anchor generously — the best
work products quote the operative contract language, not just the numbers.
