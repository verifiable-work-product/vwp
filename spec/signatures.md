# VWP Signatures

**Status:** v0.1 draft, experimental. Signatures are the least mature part of v0.1 and
WILL change in later versions.

## 1. Overview

Signatures make a package tamper-evident and attributable: a signature over the manifest —
which in turn declares SHA-256 digests for package parts — covers the integrity of every
declared part. Signature records live under `signatures/` and are referenced from the
manifest `signatures` array. Signatures are OPTIONAL in v0.1.

## 2. v0.1 mechanism

v0.1 allows two forms of signature record:

1. **Detached JWS over the manifest.** A JWS (RFC 7515) whose signed payload is the exact
   bytes of `manifest.json`. Detached-payload JWS (RFC 7797, `b64: false`) is RECOMMENDED
   so the manifest is not duplicated. The record stores the JWS, the signing algorithm,
   and signer metadata (identity, role, timestamp). Because the manifest carries the
   `sha256` of each declared part, a valid manifest signature extends integrity to all
   declared parts. Parts without declared digests are NOT covered.
2. **Placeholder signature records.** A structured record naming an intended or asserted
   signer without cryptographic material, for workflows where signing infrastructure does
   not yet exist. Placeholder records carry no integrity or authenticity guarantee, and
   verifiers MUST report them as unverified.

Recommended algorithms for v0.1: `ES256` or `EdDSA` (Ed25519). `none` MUST be rejected.

## 3. Verification rules

Verifiers processing signatures MUST:

1. Verify the JWS over the exact manifest bytes as stored in the archive.
2. Report which parts are covered (declared `sha256`) and which are not.
3. Report placeholder records as present but unverified.
4. Treat signature verification as separate from core structural verification: a package
   MAY verify structurally while carrying no valid signature, and verifiers MUST report
   the two outcomes distinctly.

Verifiers MUST NOT execute package contents for any purpose, including signature
processing. Signature records and keys embedded in packages are untrusted input.

## 4. Key distribution in v0.1

v0.1 does not define key discovery. Signature records MAY embed a public key or `x5c`
chain, but embedded keys prove only internal consistency (the signature matches the
embedded key), not signer identity. Establishing that a key belongs to a claimed signer
is out of scope for v0.1 and MUST be handled out of band (organizational key registries,
existing PKI).

## 5. Roadmap (non-normative)

Later versions intend to specify:

- **Key discovery** — resolving signer identifiers to keys (DID-style identifiers,
  JWKS endpoints, or certificate chains) so verifiers can establish identity, not just
  integrity.
- **Multiple signers** — well-defined semantics for independent signatures by preparer,
  reviewer, and approver roles over the same manifest.
- **Countersignatures** — signatures over prior signatures, to express "I reviewed and
  accept the package as signed by X" (e.g. reviewer countersigning a preparer signature).
- **Timestamping** — third-party timestamp tokens to anchor signing time.
- Alignment with existing detached-signature ecosystems where practical.

## 6. Practical guidance

Producers SHOULD declare `sha256` for every part (bringing it under signature coverage)
and SHOULD sign the manifest as the final production step — after approvals and shipped
validation results are written — so the signature covers the complete package state.
