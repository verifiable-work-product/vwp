# VWP CLI

The reference CLI can construct, build, inspect, and verify Verifiable Work
Products. Construction commands edit an unpacked package directory; `build`
then hashes every declared part, creates the `.vwp` archive, and verifies it.

## Construct a package

```bash
# Creates the manifest, work product, provenance, empty claims collection,
# and a starter renderings/readable.md.
vwp init ./nyc-allocation \
  --title "NYC Revenue Allocation" \
  --domain tax \
  --created-by "Pat White"

vwp add evidence ./nyc-allocation \
  --id ev_revenue \
  --title "Revenue by region" \
  --type dataset \
  --file ./revenue.csv

vwp add fact ./nyc-allocation \
  --id f_nyc_revenue \
  --statement "NYC revenue was 125000 USD." \
  --evidence ev_revenue \
  --value '{"amount":125000,"currency":"USD"}'

vwp add calculation ./nyc-allocation \
  --id calc_nyc_share \
  --type ratio \
  --inputs '{"nycRevenue":125000,"totalRevenue":500000}' \
  --outputs '{"nycShare":0.25}' \
  --recomputable

vwp add claim ./nyc-allocation \
  --id c_nyc_share \
  --statement "NYC revenue represents 25% of total revenue." \
  --fact f_nyc_revenue \
  --calculation calc_nyc_share \
  --status supported

vwp set-readable ./nyc-allocation ./memo.md
vwp embed ./nyc-allocation ./allocation.xlsx
vwp build ./nyc-allocation ./nyc-allocation.vwp
```

`init` currently emits v0.1 packages because v0.2 verifier support is still a
draft. Its starter Markdown uses the forward-compatible `readable` role; v0.1
tools that know only `rendering` remain supported by the readers.

## Rich JSON objects

Every structured `add` command accepts a complete object when flags are too
limited:

```bash
vwp add evidence ./package --json ./evidence-record.json --file ./source.pdf
vwp add fact ./package --json ./fact.json
vwp add claim ./package --json ./claim.json
vwp add calculation ./package --json ./calculation.json
```

The object is validated against its VWP schema before it is written. Local
evidence and embedded artifacts are copied into the package, hashed, and added
to the manifest. Re-adding an identical object is safe; reusing an id or path
with different content fails rather than overwriting it. `set-readable` is the
explicit replacement operation for the readable Markdown.

Run `vwp <command> --help` for all options.

## Existing package operations

```bash
vwp verify package.vwp
vwp inspect package.vwp
vwp render package.vwp
vwp diff old.vwp new.vwp
vwp unpack package.vwp ./unpacked
vwp pack ./package package.vwp --write-hashes
```
