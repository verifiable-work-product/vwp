# VWP Manifest

**Status:** v0.1 draft, experimental.

## 1. Overview

`manifest.json` is the entry point of every VWP package. It identifies the package,
declares the profile in use, and indexes package parts with their integrity digests.
It MUST validate against `https://vwp.dev/schemas/v0.1/manifest.schema.json`.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `vwpVersion` | string | yes | Specification version. MUST be the constant `"0.1"`. |
| `packageId` | string | yes | Globally unique package identifier. UUID or URI RECOMMENDED. |
| `createdAt` | string | yes | Package creation time, RFC 3339 timestamp. |
| `profile` | string | yes | Profile identifier (e.g. `"vwp-core"`, `"vwp-finance"`). See [profiles.md](profiles.md). |
| `extension` | string | no | Extension identifier layered on the profile (e.g. `"vwp-accounting-asc606"`). |
| `subject` | object | no | Free-form description of what the work product is about (entity, period, system, etc.). |
| `workproduct` | part ref | yes | Reference to `workproduct.json`. |
| `parts` | array of part refs | no | All other indexed package parts. |
| `provenance` | part ref | yes | Reference to `provenance/provenance.json`. |
| `signatures` | array of part refs | no | References to signature records under `signatures/`. |
| `digests` | object | no | Integrity graph roots (`algorithm`, `analysisRoot`, `contentRoot`, `reviewRoot`, `collections`). Recomputed by verifiers; see [integrity.md](integrity.md). |

Required fields: `vwpVersion`, `packageId`, `createdAt`, `profile`, `workproduct`,
`provenance`.

## 3. Package part references

A package part reference is an object:

| Field | Type | Required | Description |
|---|---|---|---|
| `path` | string | yes | Package-relative path (forward slashes, no `..`, not absolute). |
| `mediaType` | string | yes | IANA media type of the part (e.g. `application/json`, `application/pdf`). |
| `sha256` | string | no | Lowercase hex SHA-256 digest of the part's bytes. |
| `role` | string | no | Functional role hint (e.g. `claims`, `validationReport`, `rendering`, `primary-artifact`). Unknown roles remain valid in v0.1. |

`sha256` SHOULD be present on every part reference. Parts without a declared digest are
outside the integrity envelope (see [package.md](package.md), section 5).

## 4. Example

```json
{
  "vwpVersion": "0.1",
  "packageId": "urn:uuid:6f9d2c1a-8b3e-4f2a-9c7d-1e5a0b4d8f21",
  "createdAt": "2026-07-01T18:22:05Z",
  "profile": "vwp-finance",
  "extension": "vwp-digital-assets-wallet-ownership-control",
  "subject": { "entity": "Example Corp", "period": "2026-Q2" },
  "workproduct": {
    "path": "workproduct.json",
    "mediaType": "application/json",
    "sha256": "9b2f…"
  },
  "parts": [
    { "path": "claims/claims.json", "mediaType": "application/json", "sha256": "4c1a…", "role": "claims" },
    { "path": "evidence/bank-statement.pdf", "mediaType": "application/pdf", "sha256": "77de…", "role": "evidence" },
    { "path": "artifacts/revenue-analysis.xlsx", "mediaType": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "sha256": "31b0…", "role": "primary-artifact" },
    { "path": "renderings/memo.md", "mediaType": "text/markdown", "sha256": "aa03…", "role": "rendering" }
  ],
  "provenance": {
    "path": "provenance/provenance.json",
    "mediaType": "application/json",
    "sha256": "e410…"
  },
  "signatures": []
}
```

## 5. Verification rules

Verifiers MUST:

1. Parse `manifest.json` and validate it against the v0.1 manifest schema.
2. Reject `vwpVersion` values other than `"0.1"` (for v0.1 verifiers).
3. Reject any part reference whose path violates the path rules in [package.md](package.md).
4. Resolve every referenced part to an existing ZIP entry; a missing referenced part is a failure.
5. Recompute and compare `sha256` for every part reference that declares one; mismatch is a failure.

Verifiers SHOULD warn about archive entries not referenced by the manifest.

## 6. Proposed v0.2 readable contract

The draft v0.2 manifest schema makes `parts` required and requires a Markdown
readable part:

- `path` MUST end in `.md` (case-insensitive); and
- `mediaType` MUST be `text/markdown`.

New producers SHOULD declare exactly one such part with role `readable`. For
new packages it SHOULD use a lowercase filename under `renderings/`. For
backwards compatibility, a Markdown part with the v0.1 role `rendering`, an
alternate safe placement, or uppercase extension also satisfies the schema. If
no explicit `readable` exists, viewers select a legacy Markdown rendering
deterministically.

The readable is the stable human review surface. A new producer SHOULD place
the original XLSX, PDF, DOCX, or other native work product under `artifacts/`
and identify one preferred part as `primary-artifact`. The schema deliberately
keeps artifact role cardinality and placement permissive for legacy packages.
A native artifact never satisfies the v0.2 Markdown requirement. See
[`schemas/v0.2/manifest.schema.json`](../schemas/v0.2/manifest.schema.json).
