# VWP v0.2 schema drafts

This directory contains schemas proposed for the next VWP revision. It is not
yet a complete v0.2 schema set, and a package must not declare
`"vwpVersion": "0.2"` until the full revision and verifier support ship.

The current drafts are:

- `manifest.schema.json`: requires a `.md`/`text/markdown` part. New producers
  place it under `renderings/` with the preferred role `readable`; the legacy
  role `rendering`, alternate placement, and extension casing also remain
  accepted. At most one explicit `readable` is permitted. Native artifact role
  hints remain permissive; new producers should identify one preferred artifact.
- `presentation-map.schema.json`: the bidirectional index between
  authoritative structured objects and their appearances in readable or
  embedded artifacts (ADR 0004).
