# ADR 0004: Use a Sidecar Map for Links into Human-Readable Artifacts

**Status:** proposed for v0.2

## Context

A reviewer reading an embedded spreadsheet, PDF, HTML report, or derived memo should be
able to select a presented statement and see the authoritative fact, claim, calculation,
or judgment behind it. The inverse is equally useful: selecting a fact should navigate to
every place it appears in the readable work product.

Putting a `renderingRef` directly on a fact looks simple, but it reverses the dependency
established by ADR 0002. Facts are authoritative and renderings are derived. Reformatting
a memo would then require changing and re-hashing an otherwise unchanged fact. It would
also solve the problem only for facts, even though claims, calculations, and judgments are
presented too, and a single object may appear in several artifacts or locations.

## Decision

- VWP v0.2 will require a `text/markdown` part under `renderings/`. New
  producers use one explicit manifest role `readable`; packages using the v0.1
  role `rendering` remain valid. The original native work product remains an
  optional `primary-artifact`.
- VWP v0.2 will define an optional `presentation/presentation-map.json` part with manifest
  role `presentation-map`.
- The map is a derived, hash-covered bidirectional index. Each entry names an authoritative
  object (`fact`, `claim`, `calculation`, or `judgment`), a manifest-declared artifact path,
  and one or more locators inside that artifact.
- Locators reuse the vocabulary already used for evidence anchors where it fits: `quote`,
  `page`, `cells`, `section`, `region`, `jsonPath`, and character spans. Presentation
  locators additionally allow `fragment` (HTML/Markdown id) and `bookmark` (document/PDF
  bookmark).
- The authoritative object does not point back to the presentation map. Viewers index the
  map on open and expose navigation in both directions.
- Verifiers check that object ids and artifact paths resolve. A `quote` against a text
  artifact is normalized and matched deterministically. Format-specific locations such as
  PDF pages, spreadsheet cells, bookmarks, and visual regions remain advisory until a
  deterministic validator for that media type is declared.
- Changing or regenerating a readable artifact changes the artifact and presentation-map
  hashes, not the facts or claims it presents.

## Example

```json
{
  "presentationMapVersion": "0.1",
  "links": [
    {
      "objectType": "fact",
      "objectId": "f_nyc_revenue",
      "artifactPath": "renderings/nyc-revenue-allocation.md",
      "anchors": [
        {
          "section": "Allocation conclusion",
          "quote": "New York City revenue represents 24.7% of total revenue."
        }
      ]
    },
    {
      "objectType": "claim",
      "objectId": "c_nyc_allocation_percentage",
      "artifactPath": "artifacts/nyc-revenue-allocation.xlsx",
      "anchors": [
        { "cells": { "sheet": "Allocation", "range": "B14" } }
      ]
    }
  ]
}
```

## Versioning

The v0.1 container already permits an opaque, hash-covered part with this role, so tools
may experiment with the map without invalidating older packages. Its locator semantics,
schema validation, and referential-integrity validator become normative with VWP v0.2;
that is the point at which producers may rely on cross-viewer behavior.

## Consequences

- Viewer navigation can be genuinely bidirectional without making derived presentation
  data authoritative.
- Every v0.2 package has a predictable, format-independent Markdown surface for human
  review and statement-level links.
- The same mapping mechanism works for native embedded artifacts and generated renderings.
- Producers need to regenerate the map when layout changes.
- Some locators are advisory at first. A viewer may navigate to a spreadsheet cell even
  when the core verifier cannot independently validate that cell's displayed value.
