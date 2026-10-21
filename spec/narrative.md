# VWP Narrative and Memo Format

**Status:** v0.1 draft, experimental.

## 1. Overview

A package's structured parts are its source of truth; the memo a human reads
is derived from them. Two optional parts make that memo verifiable and let an
organization keep its own house style:

- `narrative/format.json` (role `memo-format`) is the **memo format**: the
  organization's section list, in its own words and order, with the kind of
  package content that fills each section, static boilerplate kept verbatim,
  and the header block. It validates against
  `https://vwp.dev/schemas/v0.1/memo-format.schema.json`.
- `narrative/narrative.json` (role `narrative`) is the **narrative**: the
  prose written against that format, section by section, after the analysis
  validates. Every paragraph cites the objects it rests on. It validates
  against `https://vwp.dev/schemas/v0.1/narrative.schema.json`.

A package without either part renders in the default layout (section 5).

## 2. Memo format

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | integer | yes | `1`. |
| `name` | string | no | Display name of the format. |
| `source` | object | no | The customer document it was derived from: `filename`, `mediaType`, `sha256`, `size`. |
| `header` | array | no | Up to 12 header lines `{ label, source, value? }`. `source` is one of `title`, `entity`, `period`, `subject`, `status`, `preparer`, `approvers`, `literal`; `value` is used with `literal`. |
| `sections` | array | yes | 1 to 40 sections, in memo order. |
| `footer` | string | no | Static text rendered verbatim at the end. |

A section:

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Slug, unique within the format. |
| `heading` | string | yes | The heading exactly as the organization writes it. Single line. |
| `level` | integer | yes | 1 to 3. |
| `kind` | string | yes | What fills the section: `narrative`, `conclusion` (prose), `alternatives`, `claims`, `judgments`, `calculations`, `sources` (structural), `signoff`, `validations`, `attestations` (trailer), `boilerplate` (the `body`, verbatim). |
| `templateSection` | string | no | An analysis template section the prose covers (producer-defined, e.g. `s2`). |
| `guidance` | string | no | The organization's own description of what belongs here. For the author; never rendered. |
| `body` | string | no | Boilerplate text, `boilerplate` kind only. |
| `required` | boolean | no | A prose section the narrative must fill. |

At least one section of kind `narrative` or `conclusion` MUST exist.

## 3. Narrative

| Field | Type | Required | Description |
|---|---|---|---|
| `formatHash` | string | yes | Self hash of the memo format snapshot the narrative follows ([integrity.md](integrity.md)). |
| `analysisRoot` | string | yes | The package analysis root when the narrative was last written. |
| `sections` | array | yes | Top-level sections, each named by a format section id of a prose kind. |

A section has `id`, optional `heading` (subsections only), `paragraphs` and
optional `subsections` (one level). A paragraph has `text` (plain text, no
markdown structure), `role` (`analysis` or `conclusion`) and `cites`: at
least one `{ id, hash }` naming a fact, claim, judgment, calculation or
evidence entry and its closure hash at the time of writing.

## 4. Semantics

- The narrative is written last, against a package whose analysis
  validates. Producers SHOULD refuse to write prose while any
  non-narrative validator fails.
- Conclusion-role paragraphs MUST cite at least one claim or judgment.
  Every claim SHOULD be cited by some paragraph.
- A conclusion-kind section's conclusion-role paragraphs are the work
  product's `conclusionSummary`; producers rewrite it from the section.
- Prose, boilerplate, footers and headings are untrusted text in the
  rendering: renderers MUST escape markdown structure and HTML in them so no
  section of the memo can be forged or hidden. Headings MUST be single-line.
- The verifiability trailer (validations, review actions, attestations) is
  always rendered, after the organization's sections, whether or not the
  format lists it.

## 5. Default layout

A package without a memo format renders as: Conclusion (`conclusionSummary`),
Claims, Judgments, Validations, Review actions, Analysis attestations. A
producer that writes a narrative against the default layout SHOULD snapshot
that layout into `narrative/format.json` so the package stays self-contained.

## 6. Validators

| Id | Status on problem | Meaning |
|---|---|---|
| `narrative.schemaValid` | fail | the part validates against the schema; headings single-line; text plain |
| `narrative.sections` | fail | every top-level section is a prose-kind format section; every `required` prose section has paragraphs (also run when only the format snapshot exists); `formatHash` matches the snapshot |
| `narrative.citations` | fail / reviewRequired | every cite id resolves (fail); every claim is cited (reviewRequired) |
| `narrative.roles` | fail / reviewRequired | conclusion paragraphs cite a claim or judgment (fail); a section bound to a template section cites an object with that prefix (reviewRequired) |
| `narrative.cites.fresh`, `narrative.drift` | see [integrity.md](integrity.md) §7 |
