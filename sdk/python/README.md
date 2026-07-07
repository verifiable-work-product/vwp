# vwp (Python SDK)

**Status: planned stub.** This package is a placeholder. The working reference
implementation of the VWP verifier is the TypeScript CLI in [`../../cli/`](../../cli/)
— use that today.

## What this package will provide

- Dataclasses mirroring the VWP core objects (a minimal preview lives in
  [`vwp/types.py`](./vwp/types.py))
- A package builder
- A verifier matching the reference CLI's behavior
  ([`vwp/verifier.py`](./vwp/verifier.py) currently raises `NotImplementedError`)

## Not published

Version `0.0.0`; not published to PyPI. APIs will change without notice until the SDK
work begins (targeted around v0.3 on the [roadmap](../../ROADMAP.md)).
