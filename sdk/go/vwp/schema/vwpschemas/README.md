# Embedded VWP schemas

This directory embeds the JSON Schemas from this repository's canonical
`schemas/v0.1/` tree (`*.json` files only; `$id`s under
`https://vwp.dev/schemas/v0.1/`, JSON Schema draft 2020-12) into the `schema`
package via `go:embed`, so validation needs no filesystem or network access.

`go:embed` cannot reach outside the Go module, so this is a copy — but it is
mechanically enforced, not hand-synced: `drift_test.go` asserts byte-equality
with `schemas/v0.1/**` on every `go test` run, so any schema change that is
not mirrored here fails CI.

To refresh after changing the canonical schemas:

    cd sdk/go && make sync-schemas
