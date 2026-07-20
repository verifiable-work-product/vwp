# VWP Facts

**Status:** v0.1 draft, experimental.

## 1. Overview

Facts are extracted observations, not conclusions. A fact states something directly
observable in evidence — a balance on a statement, a signer address on a transaction, a
clause in a contract — together with where it came from and how it was extracted. Claims
then build on facts. Fact objects live under `facts/` and validate against
`https://vwp.dev/schemas/v0.1/fact.schema.json`.

Keeping facts separate from claims lets a reviewer or validator check the ground truth
layer independently: is each fact actually supported by the cited evidence, before asking
whether the claims follow from the facts.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. Referenced by claims. |
| `statement` | string | yes | Human-readable statement of the observation. |
| `factType` | string | no | Namespaced type for the fact (e.g. `asc606.transactionPrice`). Extensions may declare fact types, validate `value` shapes, and require fact types for a work product type. |
| `value` | any | no | Structured value of the observation (number, string, object), when one exists. See §5 for conventional shapes. |
| `evidenceRefs` | array | yes | Non-empty. Each entry is an evidence `id` string, or an object `{ evidenceId, anchors }` locating the observation inside the evidence (§4). |
| `extractionMethod` | string | no | How the fact was extracted (e.g. `manual`, `ocr`, `parser`, `llm`, `apiField`). |
| `extractionModel` | object | no | When a model extracted this fact: the exact `provider`, `name`, and `version` (see §3). |
| `confidence` | number | no | Extraction confidence in `[0, 1]`. Confidence in the extraction, not in any conclusion. |
| `requiresReview` | boolean | no | True when a human should confirm the extraction before reliance. Defaults to false. |

## 3. Semantics

- A fact MUST be an observation attributable to evidence. Interpretations, inferences, and
  conclusions belong in claims or judgments, not facts.
- Every `evidenceRefs` entry MUST resolve to an evidence `id` present in the package.
  Verifiers MUST fail on dangling references.
- `value` SHOULD be provided whenever the observation has a natural structured form, so
  calculations and validators can consume it without re-parsing `statement`.
- Facts extracted by automated means (including models) SHOULD set `extractionMethod` and
  `confidence`, and SHOULD set `requiresReview: true` when confidence is low or the fact
  is material to the conclusion. Whether `requiresReview` facts block acceptance is a
  matter for profile validators and organizational policy, not core verification.
- Facts extracted by a model SHOULD set `extractionModel` with the exact `provider`,
  `name` (the provider's published model identifier, not a family or marketing name), and
  `version`. The model MUST also appear in `provenance/provenance.json` `models`. Exact
  identity lets a reviewer challenge model-extracted facts with a *different* model than
  the one that produced them (see [provenance.md](provenance.md)).

## 4. Anchors

An anchored evidence reference is `{ evidenceId, anchors }`, where `anchors` is a
non-empty array of anchor objects. Each anchor locates **one chunk** of the cited
evidence, so a reviewer — human or machine — does not have to hunt through a 40-page
contract or a 5,000-row export; a fact supported by several places in the same document
carries several anchors. Fields *within* an anchor compose to describe its single chunk
(e.g. `page` + `lines` + `quote`); all fields are optional:

| Field | Type | Description |
|---|---|---|
| `quote` | string | Verbatim excerpt from the evidence supporting the fact. |
| `page` | integer | 1-based page number. |
| `lines` | object | `{ from, to? }`, 1-based line range. Page-relative when combined with `page` (a pin cite: page 7, lines 12–14); otherwise global to the text evidence. |
| `cells` | object | `{ sheet?, range }`, A1-style range in tabular evidence. |
| `charSpan` | object | `{ start, end }`, 0-based character offsets into text evidence, end-exclusive. |
| `byteRange` | object | `{ start, end }`, 0-based byte offsets into the evidence bytes, end-exclusive. |
| `section` | string | Human-readable section/clause label (e.g. `2(a)`). |
| `jsonPath` | string | Path into JSON evidence. |
| `region` | object | `{ x, y, width, height }` pixel region in image evidence. |

**Anchors never address files.** The file (or external locator) is identified by the
evidence object that `evidenceId` resolves to — its `path`, `externalRef`, and `sha256`
— so an anchor cannot disagree with the evidence record about where the bytes live, and
the integrity envelope stays with the evidence object. Content spanning multiple files
is multiple evidence objects, each referenced (and anchored) separately.

`quote` is the load-bearing field: for package-local **text evidence** (`.txt`, `.md`,
`.csv`, `.json`), verifiers check deterministically that the quote appears in the
evidence bytes (validator `core.factAnchors.quotesMatch`); every quote in every anchor
is checked independently. `page` + `lines` is the conventional pin cite into paginated
evidence (including derived text layers with page markers), and adding `quote` to the
same anchor pairs the human locator with the machine check. Matching applies the same
normalization to quote and evidence text: Unicode NFC, curly quotes/dashes mapped to
ASCII, whitespace runs collapsed to single spaces. A quote that does not match fails
verification. All other anchor fields are advisory locators for human reviewers and
tooling; they never affect integrity verification and are not machine-verified in v0.1.

Quotes against binary evidence (PDF, DOCX, XLSX, images) or external evidence cannot be
machine-checked and surface as a warning. Producers SHOULD instead include a **derived
text layer** — a text extraction of the binary, itself an evidence object with
`derivedFrom` and `transform` set (see [evidence.md](evidence.md) §4) — and anchor facts
to the extraction. The binary remains the authoritative source; the extraction is
inside the package hash envelope and independently re-derivable.

- Anchors SHOULD be provided for any fact whose evidence exceeds roughly a page, and for
  every fact material to a conclusion.
- An anchored reference's `evidenceId` MUST resolve like any other evidence reference.

Multiple chunks in one reference:

```json
{
  "evidenceId": "ev-001",
  "anchors": [
    { "quote": "Closing balance as of June 30, 2026: $1,250,344.18", "page": 2, "lines": { "from": 41 } },
    { "quote": "Account 4471-220 (operating)", "page": 1, "lines": { "from": 6 } }
  ]
}
```

## 5. Conventional value shapes

`value` is schema-open, but producers SHOULD use these shapes so validators and
downstream tools can consume values without guessing:

| Shape | Form | Example |
|---|---|---|
| money | `{ "amount": number \| string, "currency": string }` | `{ "amount": 300000, "currency": "USD" }` |
| quantity | `{ "value": number, "unit": string }` | `{ "value": 24, "unit": "months" }` |
| date | RFC 3339 date string | `"2026-06-30"` |
| period | `{ "from": date, "to": date }` | `{ "from": "2026-01-01", "to": "2027-12-31" }` |

Extensions MAY constrain the `value` of a declared `factType` to one of these shapes (or
a stricter one) via an extension schema.

## 6. Example

```json
{
  "id": "fact-003",
  "statement": "The closing balance of the operating account on 2026-06-30 was USD 1,250,344.18.",
  "factType": "finance.accountBalance",
  "value": { "amount": "1250344.18", "currency": "USD" },
  "evidenceRefs": [
    {
      "evidenceId": "ev-001",
      "anchors": [
        {
          "quote": "Closing balance as of June 30, 2026: $1,250,344.18",
          "page": 2
        }
      ]
    }
  ],
  "extractionMethod": "llm",
  "extractionModel": {
    "provider": "example-ai",
    "name": "example-model-large",
    "version": "2026-05-14"
  },
  "confidence": 0.97,
  "requiresReview": false
}
```
