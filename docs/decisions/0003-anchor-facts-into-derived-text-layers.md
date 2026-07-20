# ADR 0003: Anchor Facts into Derived Text Layers, Not Binary Evidence

**Status:** accepted

## Context

Facts originally referenced evidence only by object id — a whole file. That is too
coarse: a fact citing a 40-page purchase agreement or a 5,000-row transaction export
gives a reviewer no way to check support without hunting, and gives validators nothing
to check at all. Facts need anchors: locations inside evidence, ideally verifiable by
machine.

The obstacle is binary evidence. Most professional source material arrives as PDFs,
Word documents, and spreadsheets. Anchoring into binaries directly (PDF quad points,
OOXML ranges) would require format-specific parsers inside the verifier, and text
extraction from PDFs is notoriously non-deterministic across libraries — reading order,
whitespace, ligatures, and OCR all vary. Core verification must stay deterministic
(ADR 0002 posture: no NLP, no judgment calls, and per spec no parsing of binary
evidence).

## Decision

- **Anchors are part of the fact's evidence reference.** An `evidenceRefs` entry may be
  an object `{ evidenceId, anchors }`, where each anchor in the array locates one chunk
  and carries `quote`, `page`, `lines`, `cells`, `charSpan`, `byteRange`, `section`,
  `jsonPath`, and/or `region`. Anchors never carry file paths; the evidence object the
  id resolves to owns the file and its hash.
- **The verbatim `quote` is the machine-checkable anchor.** For package-local text
  evidence (`.txt`, `.md`, `.csv`, `.json`), the verifier checks that the quote appears
  in the evidence bytes after a fixed normalization (Unicode NFC, curly quotes/dashes
  to ASCII, whitespace collapsed). This is validator `core.factAnchors.quotesMatch`;
  a non-matching quote fails verification. Other anchor fields are locators for humans.
- **Binary evidence gets a derived text layer, and facts anchor to that.** A PDF or
  Word document is extracted to page-preserving text/Markdown; a spreadsheet to CSV per
  sheet. The extraction is added as its own evidence object with `derivedFrom` pointing
  at the original and `transform` recording the extractor tool, version, and method.
  Both original and extraction are hashed inside the package integrity envelope.
- **The verifier never parses binaries.** Quote anchors pointing directly at binary or
  external evidence are surfaced as warnings, steering producers toward derived text
  layers.

## Consequences

- "Is this fact actually in the evidence" becomes a deterministic check for text-backed
  facts — previously an entirely human review step. This extends "AI builds,
  deterministic software verifies" one level deeper, to extraction itself.
- The derived text layer is a trusted-generator surface: a dishonest or buggy extractor
  could misrepresent the binary. Mitigations are structural rather than in-verifier:
  the transform is disclosed and reproducible (re-run the tool, compare hashes), the
  original bytes ship in the same package, and extraction fidelity is exactly the kind
  of procedure analysis attestations (whitepaper §4.5) exist to challenge.
- Packages grow: binaries ship alongside their extractions. Acceptable — evidence
  minimization guidance already applies, and text layers compress well.
- Producer toolchains need an extraction step with pinned tool versions. Extractors
  become ecosystem components the way rendering generators are (ADR 0002).
- Normalization rules are frozen in the spec; loosening or tightening them later is a
  breaking change to verification semantics and must be versioned.
