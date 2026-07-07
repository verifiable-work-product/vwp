# vwp CLI

Reference command-line tool for VWP (Verifiable Work Product) packages. Status: experimental v0.1 draft.

## Setup

```bash
cd cli
npm install
npm run build
```

The CLI resolves JSON Schemas from `../schemas/v0.1` relative to the repo checkout. Set `VWP_SCHEMAS_DIR` to override.

## Commands

```bash
node dist/index.js pack <directory> <output.vwp> [--write-hashes]
node dist/index.js unpack <package.vwp> <directory>
node dist/index.js verify <package.vwp | directory> [--strict]
node dist/index.js inspect <package.vwp | directory>
node dist/index.js diff <old.vwp | dir> <new.vwp | dir>
node dist/index.js render <package.vwp | directory>
```

- `pack` zips a package directory into a `.vwp` file. With `--write-hashes` it first computes SHA-256 digests for every part declared in `manifest.json` and updates the manifest in place.
- `unpack` extracts a `.vwp` file. It refuses entries with absolute paths, drive letters, backslashes, or `..` segments (ZIP path traversal protection) and never executes package contents.
- `verify` runs core validators (manifest presence and schema validity, required parts, declared part hashes, workproduct/claims/provenance schema validity, evidence and claim reference resolution), plus finance profile validators when `profile` is `vwp-finance`, plus demo extension validators for `vwp-accounting-asc606` and `vwp-digital-assets-wallet-ownership-control`.
- `inspect` prints a package summary: IDs, profile, counts, validation summary, renderings.
- `diff` prints a simple structural diff: changed workproduct fields, added/removed evidence IDs, added/removed/changed claims, changed judgment conclusions, changed calculation outputs, changed validation statuses.
- `render` prints the package's markdown rendering (`renderings/memo.md` or `renderings/workpaper.md`). Renderings are derived views, never the source of truth.

## Exit codes

- `0` — valid; no failures (warnings and review-required items allowed unless `--strict`)
- `1` — invalid; one or more validators failed
- `2` — usage or runtime error

## Example session

```bash
node dist/index.js pack ../examples/asc606-revenue-contract/package ../examples/asc606-revenue-contract/acme-asc606.vwp
node dist/index.js verify ../examples/asc606-revenue-contract/acme-asc606.vwp
node dist/index.js inspect ../examples/asc606-revenue-contract/acme-asc606.vwp
node dist/index.js unpack ../examples/asc606-revenue-contract/acme-asc606.vwp /tmp/acme-asc606
```

## Tests

```bash
npm test
```

## Limitations

VWP validation means that a package conforms to specified schemas and validation rules. It does not mean the professional conclusion is correct, complete, audited, legally sufficient, or appropriate for a particular use. Professional judgment, applicable standards, and independent review still govern reliance on any VWP package.
