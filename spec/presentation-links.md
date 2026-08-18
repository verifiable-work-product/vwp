# VWP Presentation Links

**Status:** v0.2 proposal. Not normative in v0.1.

Presentation links connect authoritative structured objects to the places they appear in
human-readable artifacts. In v0.2, every package has a Markdown review surface—preferably
with role `readable`, while legacy `rendering` remains accepted—so a statement can always
be addressed with a quote, section, fragment, line span, or character span. Links live in
`presentation/presentation-map.json`, not on the fact or claim itself, because the
readable is a derived view.

The proposed object model and versioning decision are recorded in
[`docs/decisions/0004-use-a-sidecar-presentation-map.md`](../docs/decisions/0004-use-a-sidecar-presentation-map.md).

Viewer behavior:

1. Selecting a mapped fact, claim, calculation, or judgment offers **Show in work product**.
2. Selecting or focusing a mapped statement in the readable view offers **Show source**.
3. If an anchor cannot be navigated for that media type, the viewer still displays its
   locator and the linked structured object.
4. A missing or stale presentation map never changes the underlying fact or claim; it is
   a defect in the derived presentation layer.

The map may also target an optional embedded native artifact—for example a source cell in
the original spreadsheet—but a v0.2 verifier MUST resolve statement links against the
single manifest part whose role is `readable`.
