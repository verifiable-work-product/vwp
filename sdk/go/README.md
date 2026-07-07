# vwp (Go SDK)

**Status: planned stub.** This module is a placeholder. The working reference
implementation of the VWP verifier is the TypeScript CLI in [`../../cli/`](../../cli/)
— use that today.

## What this module will provide

- Structs mirroring the VWP core objects with JSON tags (a preview lives in
  [`vwp/types.go`](./vwp/types.go))
- A package builder
- A verifier matching the reference CLI's behavior
  ([`vwp/verifier.go`](./vwp/verifier.go) currently returns a "not implemented" error)

## Not released

Module path `github.com/verifiable-work-product/vwp/sdk/go`; no tagged releases. APIs
will change without notice until the SDK work begins (targeted around v0.3 on the
[roadmap](../../ROADMAP.md)).
