# VWP Viewer for macOS

Native, read-only `.vwp` document viewer. It registers the VWP file type so a
package opens by double-click, displays the canonical Markdown `readable` and
keeps it visible in the center column while facts, claims, calculations,
evidence, the optional `primary-artifact`, verification results, and raw
package metadata open in a right-hand inspector selected from the sidebar.
Older v0.1 packages fall back to their
preferred `rendering`, or to the primary artifact when no rendering exists.
The divider between the readable and inspector is draggable, so reviewers can
resize the supporting-material column without hiding the memo. Its width stays
unchanged when switching sidebar sections and persists across app launches;
double-clicking the divider resets it.

The app uses the repository's Go verifier rather than reimplementing package
opening in Swift. The helper enforces the same ZIP traversal and decompression
limits, verifies the package, and extracts only the selected readable and
primary-artifact previews into a private temporary directory. HTML has
JavaScript disabled; other native files use macOS Quick Look. Nothing is
uploaded.

## Build

Requires Go and the macOS Swift toolchain:

```bash
./viewer/build-macos.sh
```

The build produces `viewer/build/VWP Viewer.app` and the downloadable archive
`viewer/build/VWP-Viewer-macOS.zip`. Move the app to Applications or open it
once in place; macOS will then offer it for `.vwp` files. CI uploads the ZIP as
the `VWP-Viewer-macOS` workflow artifact. This local build is ad-hoc signed.
Distribution outside the development team will require Developer ID signing
and notarization.

## Canonical readable and embedded native work products

The proposed v0.2 manifest asks new producers for one explicit portable
readable file:

```json
{
  "path": "renderings/nyc-revenue-allocation.md",
  "mediaType": "text/markdown",
  "sha256": "…",
  "role": "readable"
}
```

New producers should put the file under `renderings/` with a lowercase `.md`
extension. This is the file the Readable view opens. For backwards
compatibility, an existing `text/markdown` part with role `rendering`, legacy
safe placement, or uppercase extension remains valid and is selected when no
explicit `readable` exists.

Put the native deliverable under `artifacts/` and declare it in
`manifest.json`:

```json
{
  "path": "artifacts/nyc-revenue-allocation.xlsx",
  "mediaType": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  "sha256": "…",
  "role": "primary-artifact"
}
```

New producers should identify one preferred `primary-artifact`, shown in the
Artifact view. Other embedded deliverables can use role `artifact`. The schema
and viewer tolerate legacy placement or duplicate role hints; native artifacts
do not replace the required Markdown readable.

## Links back to facts

The proposed v0.2 `presentation-map` sidecar will let the viewer navigate from
a fact, claim, calculation, or judgment to its statement in the canonical
readable—or to a PDF page, spreadsheet cell, or region in an optional native
artifact—and back again. The link stays out of
the authoritative fact so reformatting a derived document does not change the
fact's hash. See [ADR 0004](../docs/decisions/0004-use-a-sidecar-presentation-map.md).
