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

## 4. Locators

`externalRef` MAY be a plain URL, a URN, or a structured locator object (system, record
id, query, as-of time). Locators SHOULD be specific enough that an authorized party can
re-retrieve the material, and SHOULD be paired with `retrievedAt` because external systems
change over time.

## 5. Privacy and security considerations

- Packages MAY contain sensitive data (financial records, personal data, internal system
  detail). Producers SHOULD minimize embedded evidence to what the conclusions require,
  and handle packages under the same controls as the underlying records.
- Secrets MUST NOT be placed in packages: no credentials, API keys, session tokens, or
  private keys — including inside `accessNotes` or raw API-response evidence. Producers
  SHOULD redact secrets from captured responses before embedding.
- Consumers MUST treat evidence files as untrusted input. Evidence MUST NOT be executed,
  and viewers SHOULD render evidence (PDFs, HTML captures) in sandboxed contexts.

## 6. Example

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
