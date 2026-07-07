# VWP Accounting Extensions

> **Status:** Experimental draft (v0.1). All examples are synthetic. Nothing in these
> documents is accounting advice.

Accounting extensions narrow the finance profile to specific accounting standards.
Where the finance profile provides a general assertion vocabulary and baseline
validators, an accounting extension maps claims to the structure of a particular
standard (ASC 606, ASC 842, ASC 820, and so on) and adds validators specific to that
standard's mechanics.

## Extension identifiers

Extensions use identifiers of the form `vwp-accounting-<standard>`:

| Extension | Standard | Sketch |
| --- | --- | --- |
| `vwp-accounting-asc606` | ASC 606 — Revenue from contracts with customers | [`asc606.md`](./asc606.md) |
| `vwp-accounting-asc842` | ASC 842 — Leases | [`asc842.md`](./asc842.md) |
| `vwp-accounting-asc820` | ASC 820 — Fair value measurement | [`asc820.md`](./asc820.md) |

A package declares its extension in the manifest (`extension` field) alongside its
profile (typically `vwp-finance`). Claims reference the standard informatively via
`standardRefs` (see [`../finance/standards-map.md`](../finance/standards-map.md)).

## What an accounting extension provides

- A claim taxonomy aligned to the standard's decision structure (e.g., the five-step
  revenue model for ASC 606).
- Extension-specific validators, typically arithmetic and coverage checks (e.g.,
  "allocated revenue equals transaction price").
- Conventions for which judgments must be explicit (e.g., discount rate selection under
  ASC 842, fair value hierarchy level under ASC 820).

## What an accounting extension does not provide

Extensions do not interpret standards, and passing an extension's validators does not
establish compliance with the standard. VWP validation is not an audit opinion, legal
opinion, accounting opinion, or assurance engagement. Whether a conclusion is
appropriate under a standard remains a matter of professional judgment.
