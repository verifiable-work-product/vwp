# Contributing to VWP

Thanks for your interest in VWP (Verifiable Work Product). VWP is **experimental
(v0.1)**: the package format, schemas, and validators are drafts and may change. Early
feedback is especially valuable, and early contributions may need rework as the spec
settles.

## Proposing spec changes

- Open a **GitHub issue** describing the problem or gap first. Real-world scenarios
  ("this work product can't be represented because...") are the most useful input.
- For concrete proposals, open a **pull request** against the relevant files in
  `spec/` and/or `schemas/`, referencing the issue. Small, focused PRs are easier to
  review than omnibus changes.
- Substantive design decisions should be captured as a decision record in
  `docs/decisions/` (see the existing ADRs for format).

## Schema change rules

- During **v0.x**, schema changes should be **additive**: new optional fields, new
  enum values, new objects. Renaming or removing fields, or making optional fields
  required, needs an explicit versioning discussion first.
- Schemas live in **versioned directories** (e.g., `schemas/v0/`). Breaking changes go
  into a new version directory rather than mutating a published one.
- Any schema change must be reflected in the examples and covered by CLI tests where
  practical.

## CLI development setup

The reference implementation is the TypeScript CLI in `cli/`:

```sh
cd cli
npm install
npm run build
npm test
```

Please make sure `npm run build` and `npm test` pass before opening a PR that touches
the CLI.

## Examples must be synthetic

All example packages, documents, values, entities, contracts, wallet addresses, and
names in this repository must be **synthetic**. Never contribute real customer data,
real financial records, real wallet addresses tied to real holdings, or any
confidential material. See [`SECURITY.md`](./SECURITY.md).

## Careful-language requirement

VWP is verification infrastructure, not professional assurance. Documentation and code
comments must not claim or imply that VWP validation constitutes an audit opinion, a
legal opinion, an accounting opinion, an assurance engagement, or sufficiency of
evidence for any regulatory or professional purpose. When writing profile or extension
docs, state the limits explicitly. PRs that overclaim will be asked to soften the
language before merge.

## Exact model identification

Anywhere this project records, documents, or exemplifies the use of an AI model —
provenance entries, fact extraction records, analysis attestations, spec examples,
example packages — identify the exact model: provider, the provider's published model
identifier (e.g. `claude-fable-5`, not a family or marketing name), and the most
specific version or snapshot identifier available. This is what allows a downstream
reviewer or auditor to deliberately challenge model-assisted work with a *different*
model, so preparation and review do not share correlated failure modes. PRs that record
model use vaguely ("an LLM", an unversioned name) will be asked to make it exact.

## Sign-off (optional)

We use a DCO-style sign-off convention. Adding a `Signed-off-by:` line to your commits
(`git commit -s`) certifies that you have the right to submit the contribution under
the project license (Apache-2.0). Sign-off is **optional** in v0.x but appreciated.

## Code of conduct

Participation in this project is governed by our
[Code of Conduct](./CODE_OF_CONDUCT.md).

## License

By contributing, you agree that your contributions will be licensed under the
[Apache License 2.0](./LICENSE).
