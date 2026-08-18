# VWP Package Container

**Status:** v0.1 draft, experimental.

## 1. Container format

A VWP package is a single file with the `.vwp` extension. The container is a standard ZIP
archive (PKZIP format). Producers SHOULD use no compression or DEFLATE; consumers MUST
support both. Encrypted ZIP features MUST NOT be used.

## 2. Internal layout

```
manifest.json                  required
workproduct.json               required
evidence/                      optional, recommended
facts/                         optional, recommended
claims/claims.json             required for useful packages
judgments/                     optional
calculations/                  optional
artifacts/                     optional — embedded native work-product files
validations/                   optional, recommended
approvals/                     optional
attestations/                  optional
renderings/                    optional
provenance/provenance.json     required
signatures/                    optional in v0.1
```

- `manifest.json` — package index and integrity declarations. See [manifest.md](manifest.md).
- `workproduct.json` — the top-level work product: subject, scope, and conclusion.
- `evidence/` — package-local evidence files plus evidence metadata. See [evidence.md](evidence.md).
- `facts/` — extracted facts. See [facts.md](facts.md).
- `claims/claims.json` — the claim graph. See [claims.md](claims.md). A package without
  claims is structurally valid but conveys no conclusions; `claims/claims.json` is
  required for useful packages.
- `judgments/`, `calculations/`, `validations/`, `approvals/` — corresponding object
  collections. See their respective specs.
- `artifacts/` — native files that are themselves the work product, such as a PDF,
  spreadsheet, document, or HTML report. A package MAY identify a preferred manifest part
  with role `primary-artifact`; viewers MAY use it as a fallback when no Markdown rendering
  exists. Additional files MAY use role `artifact`. Artifact bytes are opaque, hash-covered
  package parts. The v0.1 schema does not impose cardinality on these additive role hints.
- `attestations/` — analysis attestations (one JSON object per file), records of
  model-assisted review procedures. Never affect package validity. See
  [attestations.md](attestations.md).
- `renderings/` — derived human-readable views (Markdown, HTML, PDF). Never authoritative.
  A package intended for human review or external sharing SHOULD include at least one
  rendering so a recipient can read the work product without interpreting its JSON parts.
- `provenance/provenance.json` — production provenance. See [provenance.md](provenance.md).
- `signatures/` — detached signatures, optional in v0.1. See [signatures.md](signatures.md).

## 3. Minimum valid package

A minimum valid v0.1 package contains exactly these parts:

1. `manifest.json`
2. `workproduct.json`
3. `claims/claims.json`
4. `provenance/provenance.json`

Verifiers MUST fail packages missing `manifest.json`, `workproduct.json`, or
`provenance/provenance.json`.

## 4. Path rules

All internal paths, both as ZIP entry names and as path references within JSON documents:

- MUST use forward slashes (`/`) as separators;
- MUST be relative to the package root;
- MUST NOT contain `..` segments;
- MUST NOT be absolute (no leading `/`, no drive letters, no scheme prefixes).

Verifiers MUST reject packages containing path traversal (any entry or reference that
would resolve outside the package root). Extractors SHOULD normalize and re-validate paths
before writing to a filesystem.

## 5. Content integrity

The manifest declares SHA-256 hex digests for package parts. Verifiers MUST recompute the
digest of each declared part and fail verification on mismatch. Hash validation detects
tampering only for declared parts; parts present in the archive but not declared in the
manifest are outside the integrity envelope, and verifiers SHOULD warn when they are found.

## 6. Encoding

All JSON parts MUST be UTF-8 encoded. JSON parts MUST validate against the corresponding
schema at `schemas/v0.1/` (`$id`s under `https://vwp.dev/schemas/v0.1/`, JSON Schema
draft 2020-12). Non-JSON parts (evidence files, renderings) are opaque bytes identified by
`mediaType` and digest.

## 7. Safety

Packages are untrusted input. Verifiers and viewers MUST NOT execute package contents
(scripts, macros, embedded active content) and SHOULD apply resource limits when
decompressing (entry counts, sizes, compression ratios) to mitigate decompression attacks.
Viewers opening a `primary-artifact` MUST use a sandboxed or non-executing preview and MUST
NOT run macros, scripts, or other active content embedded in that file.

## 8. Proposed v0.2 readable part

VWP v0.2 separates the two human-facing concerns:

- Every package MUST contain a `.md`/`text/markdown` file. New producers SHOULD
  put a lowercase filename under `renderings/` with role `readable`; role
  `rendering`, legacy safe placement, and extension casing remain accepted.
- A package MAY contain at most one explicit `readable`. If it has none,
  viewers choose a legacy Markdown `rendering` deterministically.
- A new producer SHOULD embed native files under `artifacts/` and MAY identify
  one preferred file with role `primary-artifact`. Consumers remain permissive
  about legacy artifact placement and duplicate role hints. A native file is
  not a substitute for the readable Markdown.

This guarantees that every v0.2 work product has a safe, portable review
surface while still allowing an XLSX, PDF, DOCX, or other native deliverable to
travel inside the same `.vwp` package.
