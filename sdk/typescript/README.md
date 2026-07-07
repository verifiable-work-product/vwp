# @vwp/sdk (TypeScript)

**Status: planned.** This package is a stub. The working reference implementation of
the VWP verifier is the TypeScript CLI in [`../../cli/`](../../cli/) — use that today.

## What this package will provide

- **Types** — TypeScript interfaces for the core VWP objects (manifest, work product,
  evidence, facts, claims, judgments, calculations, validation results, approvals,
  provenance). A preview lives in [`src/types.ts`](./src/types.ts).
- **A builder** — a programmatic API for assembling `.vwp` packages
  ([`src/builder.ts`](./src/builder.ts), currently throws).
- **A verifier** — a library-form verifier matching the CLI's behavior
  ([`src/verifier.ts`](./src/verifier.ts), currently throws).

## Not published

`@vwp/sdk` is `private: true` at version `0.0.0` and is not published to npm. APIs
here will change without notice until the SDK work begins in earnest (targeted around
v0.3 on the [roadmap](../../ROADMAP.md)).
