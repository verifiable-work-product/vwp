# Public fact catalog

The VWP-Bench proposal (Aug 2026 draft) discloses a **public catalog of common
accounting fact types** to both arms — models and the CPA control group —
instead of the per-challenge required-facts manifest. The catalog names
possible facts (`contract.date`, `contract.renewalOption`,
`asc606.ssp.subscription`, …); it never reveals which facts matter in a case or
what their values are. The private case key of material facts stays with the
graders.

- `fact-catalog.v0.1.json` — starter catalog (draft; the study targets several
  hundred entries). Validates against
  `../schemas/fact-catalog.schema.json`.
- Entries reuse the fact types registered by the format extensions
  (`schemas/v0.1/extensions/*/fact-types.json`) and add common contract/general
  types under the `contract.*`, `entity.*`, and `general.*` namespaces.

A challenge on the platform chooses its **fact disclosure** mode:
`manifest` (existing behavior — slots disclosed, P1/K2 score by slot) or
`catalog` (proposal §Stage 1 — packet carries this catalog; slot-bound machine
tests fall back to factType matching). The harness's `read_brief()` returns
`factCatalog` instead of a populated `requiredFacts` for catalog-mode packets.

The prescriptive-manifest arm (telling the model exactly which fact types the
workpaper must capture) is retained as a future comparison study — proposal §5.
