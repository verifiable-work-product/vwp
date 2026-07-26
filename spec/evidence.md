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
| `externalRef` | string / object | conditional | Locator for evidence not included in the package (URL, URI, or structured locator). May carry `locatorType` and a `retrieval` block (see §5). |
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
can confirm they are looking at the same material. **Core (offline) verification MUST NOT
fetch external references** — this keeps `verify` deterministic and network-free, so the
same package bytes always produce the same result. A verifier MAY additionally offer an
**opt-in online mode** that fetches `externalRef.retrieval.url` and compares the retrieved
bytes against `retrieval.contentHash` (see §5); such checks are environment-dependent and
their results are reported separately from the deterministic core checks.

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
change over time. Set `externalRef.locatorType` (`url`, `path`, `vwp-ref`, `opaque`) to
declare how `locator` should be interpreted; `url` enables URI validation and the
`retrieval` block below.

### 5.1 Retrieval and authenticated, long-lived references

Long-lived references (e.g. a system-of-record URL that stays valid for years) MAY carry a
`retrieval` object describing **how** to fetch them, including authentication. The design
follows the IETF standards: **RFC 6750** for Bearer-token usage and **RFC 9728** for OAuth
2.0 protected-resource metadata discovery.

| `retrieval` field | Description |
|---|---|
| `url` | Absolute URL to fetch (`format: uri`). SHOULD be stable/long-lived. |
| `method` | `GET` (default), `POST`, or `HEAD`. |
| `auth.scheme` | `bearer` (RFC 6750) or `none` are defined; other strings are permitted and interpreted by the retriever. |
| `auth.header` | Request header carrying the credential (default `Authorization`). |
| `auth.tokenHint` | **Where** to obtain the credential — never the credential itself. A source reference: `env:VAR`, `vault:path`, `oauth:client-id`, `keychain:item`, `prompt:label`, or `none`. |
| `auth.discovery` | RFC 9728 metadata URL (typically `<origin>/.well-known/oauth-protected-resource`), from which a retriever learns the authorization server, token types, and scopes. |
| `auth.scopes` | OAuth scopes required to access the resource. |
| `contentHash` | SHA-256 the retrieved bytes are expected to match. This is what turns a live URL into *verifiable* evidence. |
| `retrievedSnapshotAt` | When the bytes behind `contentHash` were last retrieved and hashed. |

**Credentials are gathered over the top, never embedded.** The package declares only
*where* a credential comes from (`tokenHint`) and *how to discover* the auth server
(`discovery`) — it MUST NOT contain the secret. A retriever's flow is: enumerate the
distinct origins the package's references target, obtain one credential per origin
(from env, an interactive prompt, a vault, or an OAuth exchange guided by `discovery`),
then fetch. Embedding a literal secret in `tokenHint` is a validation **failure**
(`core.externalRefs.wellFormed`), because packages are shareable and untrusted.

`contentHash` is the bridge to determinism: a live, authenticated, long-lived URL becomes
verifiable evidence when an online retriever fetches it and confirms the bytes hash to the
declared value — so the reference is both *reachable* and *unchanged*, without embedding
the material or the secret.

For locating content *inside* evidence, facts carry anchors on their evidence references
(`page`, `lines`, `cells`, `charSpan`, `byteRange`, `section`, `jsonPath`, `region`,
`quote`; see [facts.md](facts.md) §4). Anchors are locators inside already-hashed
evidence: the integrity envelope covers the evidence bytes via `sha256`, anchors never
affect integrity verification, and only `quote` is machine-checked (against text
evidence). Anchors never carry file paths — the evidence object owns the file.

## 6. Privacy and security considerations

- Packages MAY contain sensitive data (financial records, personal data, internal system
  detail). Producers SHOULD minimize embedded evidence to what the conclusions require,
  and handle packages under the same controls as the underlying records.
- Secrets MUST NOT be placed in packages: no credentials, API keys, session tokens, or
  private keys — including inside `accessNotes`, raw API-response evidence, or
  `externalRef.retrieval.auth.tokenHint` (which carries only a *source reference* such as
  `env:VAR`, never the secret). Producers SHOULD redact secrets from captured responses
  before embedding. The `core.externalRefs.wellFormed` validator fails hard if a
  `tokenHint` appears to embed a literal secret.
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

An authenticated, long-lived external reference (secret gathered over the top, not stored):

```json
{
  "id": "ev-sor-001",
  "type": "referenceDocument",
  "title": "Signed engagement source held in the system of record",
  "sourceSystem": "bitwave",
  "externalRef": {
    "system": "bitwave",
    "locator": "https://api.bitwave.io/vwp/engagements/acme/sources/agreement",
    "locatorType": "url",
    "retrieval": {
      "url": "https://api.bitwave.io/vwp/engagements/acme/sources/agreement",
      "method": "GET",
      "auth": {
        "scheme": "bearer",
        "header": "Authorization",
        "tokenHint": "env:BITWAVE_VWP_TOKEN",
        "discovery": "https://api.bitwave.io/.well-known/oauth-protected-resource",
        "scopes": ["vwp:read"]
      },
      "contentHash": "c2d282ab43fb5957ea1ca200252eaab5450e9e2c008bdc8389e5e5a595b27be9",
      "retrievedSnapshotAt": "2026-07-24T00:00:00Z"
    }
  },
  "retrievedAt": "2026-07-24T00:00:00Z"
}
```
