# VWP Core Concepts

**Status:** v0.1 draft, experimental. Subject to breaking changes.

## 1. Purpose

VWP (Verifiable Work Product) is an open package format and verification framework for
evidence-backed professional conclusions: workpapers, memos, control tests, reconciliations,
and similar artifacts. A VWP package is a portable, signed, evidence-linked,
machine-verifiable ZIP archive. The rendered memo or PDF is a *derived view*; the structured
package is the source of truth.

VWP is model-agnostic. AI systems MAY help create packages, but deterministic validators
verify them. Verification never depends on the tool or model that produced a package.

## 2. Conceptual model

The core information flow in every VWP package is:

```
evidence -> facts -> claims -> judgments / calculations -> validations -> conclusion -> rendering
```

- **Evidence** is source material (documents, exports, screenshots, API responses),
  package-local or externally referenced, identified by content hash where possible.
- **Facts** are extracted observations grounded in evidence. Facts are not conclusions.
- **Claims** are assertions supported by facts, evidence, calculations, and judgments.
  Claims form a graph linking to standards, assertions, and supporting objects.
- **Judgments** capture points where professional discretion was required, including the
  question, proposed conclusion, basis, and alternatives considered.
- **Calculations** are computations over inputs, ideally recomputable by a verifier.
- **Validations** are the results of deterministic checks run against the package.
- **Approvals** record human review actions over the package or parts of it.
- **Provenance** records who and what (tools, models, systems) produced the package.
- **Renderings** are human-readable views (Markdown, HTML, PDF) derived from the
  structured content. Renderings are never the source of truth.

## 3. Questions a package must answer

A well-formed VWP package SHOULD allow a reader or verifier to answer:

1. What conclusion was reached?
2. What evidence supports it?
3. What facts were extracted from that evidence?
4. What claims were made, and what supports each claim?
5. What judgments were required, and on what basis?
6. What calculations were performed, and are they recomputable?
7. What validations passed, failed, or require review?
8. Who approved it, and in what capacity?
9. What tools and models produced it?
10. Can another system verify all of the above without trusting the original UI?

## 4. Design principles

- **Structured over rendered.** Machine-readable objects are authoritative; renderings are derived.
- **Deterministic verification.** Core verification is schema validation, hash checking, and
  reference-graph checking. It MUST NOT require network access, model inference, or
  execution of package contents.
- **Model-agnostic.** Provenance MAY record which model produced a package; verification
  MUST NOT require that model.
- **Domain-extensible.** VWP Core is generic. Profiles (see [profiles.md](profiles.md)) add
  domain vocabulary; extensions add narrower validators. Core verification never requires a profile.
- **Untrusted input.** Verifiers MUST treat packages as untrusted input: reject path
  traversal, never execute package contents, and bound resource use.

## 5. Specification structure

| Document | Scope |
|---|---|
| [package.md](package.md) | ZIP container and internal layout |
| [manifest.md](manifest.md) | `manifest.json` fields and part references |
| [evidence.md](evidence.md) | Evidence objects and locators |
| [facts.md](facts.md) | Extracted facts |
| [claims.md](claims.md) | Claims and the claim graph |
| [judgments.md](judgments.md) | Professional judgments |
| [calculations.md](calculations.md) | Calculations and recomputability |
| [validations.md](validations.md) | Validation results and semantics |
| [approvals.md](approvals.md) | Review and approval records |
| [provenance.md](provenance.md) | Toolchain and model provenance |
| [signatures.md](signatures.md) | Signing (experimental in v0.1) |
| [profiles.md](profiles.md) | Profiles and extensions |

JSON Schemas for all objects live at `schemas/v0.1/` with `$id`s under
`https://vwp.dev/schemas/v0.1/`. Schemas use JSON Schema draft 2020-12.

## 6. Conformance language

The key words MUST, MUST NOT, SHOULD, SHOULD NOT, and MAY are to be interpreted as
described in RFC 2119 when, and only when, they appear in all capitals.

## 7. Limitations

> VWP validation means that a package conforms to specified schemas and validation rules.
> It does not mean the professional conclusion is correct, complete, audited, legally
> sufficient, or appropriate for a particular use. Professional judgment, applicable
> standards, and independent review still govern reliance on any VWP package.

Nothing in this specification constitutes legal, accounting, or audit advice.
