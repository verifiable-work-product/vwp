# ADR 0002: Human Renderings Are Not the Source of Truth

**Status:** accepted

## Context

Professional work products are traditionally consumed as prose: memos, PDFs,
workpapers. A VWP package can include such renderings under `renderings/` so humans
can read the work product the way they always have. That raises the question: when the
rendering and the structured data disagree, which one is authoritative? And what
should verifiers verify?

Prose is not mechanically checkable. A verifier cannot determine whether a paragraph
faithfully reflects the evidence, whether a narrative total was recomputed, or whether
a hedge in the wording changes the conclusion. Structured claims, facts, calculations,
judgments, and approvals are checkable: references resolve, hashes match, arithmetic
recomputes, review-required items surface.

## Decision

- **The structured package is the source of truth.** Claims, facts, evidence
  references, calculations, judgments, validations, and approvals in their JSON forms
  are the authoritative content of a work product.
- **The rendered memo/PDF is a derived view.** Renderings exist for human consumption
  and should be generated from (or reconciled to) the structured content. If a
  rendering disagrees with the structured data, the structured data governs and the
  rendering is defective.
- **Verifiers verify structure, not prose.** VWP validation operates on the structured
  objects. Renderings are covered only by part-level integrity hashes (the bytes are
  what the manifest declared), never by semantic validation.

## Consequences

- Verification is deterministic and implementable: no NLP, no judgment calls inside
  the verifier.
- Preparers must express conclusions in structured form to get any verification value;
  a package that is "just a PDF" verifies almost nothing, and tooling should say so.
- Rendering generators become important ecosystem components (better templates are on
  the roadmap), since the human-readable view must be regenerable from structure.
- Reviewers should be aware that a polished rendering carries no more verified weight
  than its underlying structure; tooling and documentation must reinforce this to
  avoid misplaced trust in prose.
- Disputes about what a work product "says" resolve to the structured content, which
  is diffable and hashable across package versions.
