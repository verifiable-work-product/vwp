# Security Policy

VWP is experimental (v0.1). This document covers the security posture of the package
format and the reference CLI, and how to report vulnerabilities.

## Sensitive contents

`.vwp` packages may contain sensitive material: contracts, financial records, wallet
addresses, customer data, legal memos, or audit evidence. Treat packages accordingly —
store, transmit, and share them with the same care as the most sensitive document they
contain. VWP itself does not encrypt package contents in v0.1.

## Treat packages as untrusted input

A `.vwp` file is a ZIP archive plus JSON produced by parties you may not control.
Tools that consume packages must treat them as **untrusted input**:

- **ZIP path traversal**: the CLI must protect against ZIP path traversal (entries
  like `../../etc/passwd` or absolute paths) when unpacking. Entries must resolve to
  paths inside the extraction directory.
- **No execution of package contents**: the CLI avoids executing package contents.
  Evidence files, renderings, and any scripts inside a package are data, never code to
  run.
- Consumers should also apply the usual archive hygiene: entry count and size limits,
  decompression-bomb protection, and careful handling of file names.

## Limits of integrity checks

- **Hash validation detects tampering only for declared package parts.** Content not
  listed (with a hash) in the manifest is not covered. A package that omits parts from
  its manifest offers correspondingly weaker integrity guarantees.
- **Signatures are experimental in v0.1.** Do not rely on v0.1 signatures for
  authenticity decisions with real-world consequences.
- Hash and signature checks verify integrity and, eventually, origin — not the truth,
  quality, or sufficiency of the contents.

## Never put real secrets in examples

Never put real secrets, private keys, seed phrases, customer records, or production
financial data in example packages — in this repository or in packages you share for
demonstration purposes. All examples must be synthetic. Proof-of-control evidence in
examples must use throwaway keys and synthetic challenge text.

## Reporting a vulnerability

Please report suspected vulnerabilities privately rather than in public issues:

- Email: **security@vwp.dev** (placeholder address for this experimental project)
- Or use **GitHub private vulnerability reporting** on this repository

Include reproduction steps and affected versions where possible. As an experimental
project we do not yet commit to a formal response SLA, but reports will be triaged as
promptly as we can.
