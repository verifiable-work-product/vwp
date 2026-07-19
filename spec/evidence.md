# VWP Evidence

**Status:** v0.1 draft, experimental.

## 1. Overview

Evidence is the source material a work product rests on: exported reports, bank
statements, screenshots, API responses, contracts, configuration dumps. Evidence objects
describe that material; facts and claims reference evidence by `id`. Evidence objects live
under `evidence/` alongside any package-local evidence files, and validate against
`https://vwp.dev/schemas/v0.1/evidence.schema.json`.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. Referenced by facts and claims. |
| `type` | string | yes | Kind of evidence (e.g. `document`, `screenshot`, `apiResponse`, `export`, `attestation`). |
| `title` | string | yes | Human-readable label. |
| `sourceSystem` | string | no | System of origin (e.g. `"NetSuite"`, `"Etherscan"`, `"BankOfExample portal"`). |
| `path` | string | conditional | Package-relative path to the evidence file, for package-local evidence. |
| `externalRef` | string / object | conditional | Locator for evidence not included in the package (URL, URI, or structured locator). |
| `sha256` | string | no | Lowercase hex SHA-256 of the evidence bytes. |
| `derivedFrom` | string | no | Evidence `id` this object was mechanically derived from (e.g. a text extraction of a PDF). See §4. |
| `transform` | object | no | How derived evidence was produced: `{ tool, version?, method? }`. SHOULD be set whenever `derivedFrom` is set. |
| `retrievedAt` | string | no | RFC 3339 timestamp when the evidence was obtained. |
| `periodCovered` | string / object | no | Period the evidence pertains to (e.g. `"2026-Q2"` or `{ "from": …, "to": … }`). |
| `accessNotes` | string | no | How the evidence was accessed and any access constraints. |

Each evidence object MUST have `path` or `externalRef` (or both, when a local copy of an
external source is included).

## 3. Package-local vs external evidence

**Package-local evidence** is embedded under `evidence/` and referenced by `path`. It
SHOULD carry a `sha256`, and SHOULD be indexed in the manifest `parts` array so it falls
inside the package integrity envelope. Package-local evidence is the strongest form: the
verifier can confirm the exact bytes the conclusion was based on.

**External evidence** is referenced by `externalRef` without being embedded — because it
is too large, access-restricted, or lives in a system of record. External evidence SHOULD
still carry a `sha256` of the bytes as observed at `retrievedAt`, so a party with access
can confirm they are looking at the same material. Verifiers MUST NOT fetch external
references during core verification; external evidence is verifiable only by out-of-band
retrieval.

## 4. Derived text layers

Facts anchor into evidence by quoting it (see [facts.md](facts.md) §4), and quote anchors
are machine-verifiable only against text. For binary evidence — PDFs, Word documents,
spreadsheets, images — producers SHOULD include a **derived text layer**: a plain-text or
Markdown extraction of the binary, added as its own evidence object with:

- `derivedFrom` set to the original evidence `id`,
- `transform` recording the extractor (`tool`, `version`, `method` — e.g.
  `pdf-text-extraction`, `ocr`, `xlsx-to-csv`),
- `path` and `sha256` like any package-local evidence, inside the integrity envelope.

Facts then anchor to the extraction, not the binary. The original binary remains the
authoritative source; the text layer is a checkable bridge to it. Because the transform
and both hashes are recorded, any party can re-run the extraction and compare — and a
challenger (human or model) can audit the extraction against the original, the same way
any other derivation in the package is challengeable. Verifiers MUST NOT attempt to parse
binary evidence during core verification; that is exactly the non-determinism the derived
layer exists to avoid.

Recommended shapes: page-preserving Markdown for PDFs and Word documents (page breaks
marked, e.g. `<!-- page 7 -->`, so `page` anchors remain meaningful), one CSV per sheet
for spreadsheets, and OCR text for images when text content matters.

`derivedFrom` MUST resolve to an evidence `id` present in the package.

## 5. Locators

`externalRef` MAY be a plain URL, a URN, or a structured locator object (system, record
id, query, as-of time). Locators SHOULD be specific enough that an authorized party can
re-retrieve the material, and SHOULD be paired with `retrievedAt` because external systems
change over time.

## 6. Privacy and security considerations

- Packages MAY contain sensitive data (financial records, personal data, internal system
  detail). Producers SHOULD minimize embedded evidence to what the conclusions require,
  and handle packages under the same controls as the underlying records.
- Secrets MUST NOT be placed in packages: no credentials, API keys, session tokens, or
  private keys — including inside `accessNotes` or raw API-response evidence. Producers
  SHOULD redact secrets from captured responses before embedding.
- Consumers MUST treat evidence files as untrusted input. Evidence MUST NOT be executed,
  and viewers SHOULD render evidence (PDFs, HTML captures) in sandboxed contexts.

## 7. Example

```json
{
  "id": "ev-001",
  "type": "document",
  "title": "Operating account bank statement, June 2026",
  "sourceSystem": "BankOfExample portal",
  "path": "evidence/bank-statement-2026-06.pdf",
  "sha256": "77de…",
  "retrievedAt": "2026-07-01T15:04:11Z",
  "periodCovered": { "from": "2026-06-01", "to": "2026-06-30" },
  "accessNotes": "Downloaded by preparer with read-only treasury role."
}
```
