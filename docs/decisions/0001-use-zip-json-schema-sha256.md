# ADR 0001: Use ZIP, JSON Schema, and SHA-256 for v0

**Status:** accepted

## Context

VWP needs a container format, a schema language, an integrity mechanism, and a
signature story for v0.1. The primary goal at this stage is implementability: the
format should be buildable and verifiable with commodity tooling in any mainstream
language, by small teams, without specialist infrastructure. Verifiers must be
deterministic, and the format must be inspectable by a human with a ZIP tool and a
text editor. Credibility for a verification format comes from being boring and
checkable, not from novel machinery.

## Decision

- **Container: ZIP.** A `.vwp` file is a ZIP archive with a conventional layout
  (`manifest.json`, `workproduct.json`, `evidence/`, `facts/`, `claims/`,
  `judgments/`, `calculations/`, `validations/`, `approvals/`, `renderings/`,
  `provenance/`, `signatures/`). ZIP is universally supported, streamable enough for
  our sizes, and transparently inspectable.
- **Schemas: JSON Schema draft 2020-12** for all v0 schemas. JSON is the package's
  native data format; draft 2020-12 has solid validator coverage across languages.
- **Integrity: SHA-256** for content integrity of declared package parts. Ubiquitous,
  fast, and adequate for tamper detection.
- **Signatures: simple JWS-style/placeholder signatures in v0.** Enough to prototype
  signing workflows; explicitly experimental and not to be relied on (see
  `SECURITY.md`). A real signature and provenance specification is a v1.0 item.

## Explicitly rejected for v0

- **RDF / semantic web machinery** (ontologies, triple stores, SPARQL). Powerful but a
  large adoption tax; JSON with schemas covers the structural needs of v0.
- **Blockchain anchoring.** Anchoring hashes on-chain adds operational and conceptual
  cost without improving the core problem (structural verifiability of a package). It
  can be layered on later by anyone who wants it, since parts are already hashed.
- **Complex cryptography** (zero-knowledge proofs, threshold signatures, novel
  commitment schemes). Not needed for v0's guarantees, and each would shrink the pool
  of people who can implement and audit the format.

These are deliberately boring choices to maximize implementability.

## Consequences

- Any mainstream language can read, write, and verify packages with standard
  libraries; a reference verifier fits in a small TypeScript CLI.
- Humans can inspect a package by unzipping it.
- ZIP requires defensive unpacking (path traversal, decompression bombs) — see
  `SECURITY.md`; this burden falls on every consumer implementation.
- v0 signatures provide only weak authenticity; downstream users must not treat them
  as production-grade. This is acceptable for an experimental release and is called
  out prominently.
- We forgo semantic-web interoperability and on-chain anchoring in v0; the hashed,
  schema-validated structure leaves the door open to layering either on later without
  format changes.
