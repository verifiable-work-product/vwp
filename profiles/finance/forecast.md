# Extension Sketch: `vwp-finance-forecast` (Forward-Looking Estimates)

> **Status:** Experimental stub (v0.1). This is not accounting, investment, or planning
> advice. All example data is synthetic. VWP validation is not an audit opinion, legal
> opinion, accounting opinion, or assurance engagement — and for this extension in
> particular, validation says nothing about whether a forecast will come true.

A sketch of how FP&A-style forecasts — validator reward projections, revenue run-rates,
budget build-ups — map onto VWP objects.

## Why forecasts fit VWP

A forecast's conclusion cannot be verified against reality at production time; only its
*chain* can be. That makes the evidence → facts → judgments/calculations → claims
decomposition more load-bearing here, not less:

- The **calculable** part of a forecast (a published emission curve, a contracted rate,
  a run-rate extrapolation) belongs in recomputable calculations, where an independent
  verifier can reproduce it.
- The **discretionary** part (future participation, dilution, activity levels, pricing)
  belongs in explicit judgments with alternatives and risk levels — the short list of
  things a reviewer actually signs off on.
- The verification split is exactly the FP&A review split: validators check the math and
  the chain; humans approve the named assumptions.

The classic failure mode of a spreadsheet forecast is **assumption laundering**: a
discretionary guess typed into an input cell where it is indistinguishable from an
observed fact. This extension exists mainly to make that structurally detectable.

## Shape of a forecast package

- **Claims**
  - The estimated quantity over the stated horizon, per scenario, carrying the
    `estimate` assertion (see below). Statements SHOULD name the horizon and as-of date.
- **Judgments**
  - One judgment per discretionary assumption: dilution/participation changes, activity
    levels, whether to overlay a price at all. Even "hold current conditions constant"
    is a judgment, with the alternatives considered recorded.
- **Evidence**
  - Source records for every model input: protocol/tokenomics documents, system-of-record
    extracts, historical actuals from the entity's own systems (the anchoring evidence
    that turns a guess into a work paper), and — when a currency overlay is needed for a
    non-cash item — a fair value package consumed as hash-pinned external evidence
    rather than a price asserted inline.
- **Calculations**
  - The projection itself, per scenario: recomputable, with inputs resolving to facts.
    Sensitivity is modeled as multiple scenario calculations over the shared fact base,
    not as knobs inside one opaque model.
- **Renderings**
  - An executive summary memo; the structured register and calculations are the
    drillable workbook.

Interactive "tweak a parameter and re-run" workflows live in producing tools, not in the
package: the package is the reviewed snapshot, and a changed assumption produces a new
package that supersedes it.

## The `estimate` assertion

This extension layers one assertion value onto the `vwp-finance` vocabulary
(see [spec/claims.md](../../spec/claims.md) §4):

- **`estimate`** — The stated amount is a forward-looking estimate for a defined
  horizon, derived as of the forecast date from declared inputs, explicit assumption
  judgments, and recomputable calculations. Unlike the retrospective assertions
  (`existence`, `occurrence`, …), an `estimate` claim asserts the integrity of its
  derivation, not the eventual outcome.

Claims carrying `estimate` SHOULD reference at least one supporting calculation and at
least one judgment.

## Data contract and specified validators

Packages declaring `vwp-finance-forecast` carry a **forecast register** at
`facts/forecast.json`, validating against
[`schemas/v0.1/extensions/finance-forecast/forecast.schema.json`](../../schemas/v0.1/extensions/finance-forecast/forecast.schema.json):
the subject, as-of date, horizon, key inputs with a freshness policy, one or more
scenarios each bound to a calculation and its assumption judgments, and an optional
back-test block linking a predecessor forecast.

The following validators are specified by this draft (reference implementations in the
CLI and Go SDK are roadmap items; see [ROADMAP.md](../../ROADMAP.md)):

- **`forecast.register.present`** — fails when `facts/forecast.json` is missing or
  unreadable.
- **`forecast.register.schemaValid`** — the register conforms to the extension schema.
- **`forecast.scenarios.calculationLinked`** — every scenario's `calculationRef`
  resolves to a calculation in the package; warns when that calculation is not
  recomputable, because a non-recomputable projection is a black box.
- **`forecast.assumptions.judgmentLinked`** — every `assumptionJudgmentRefs` entry
  resolves; reports `reviewRequired` for any scenario declaring **no** assumption
  judgments, since a forecast with zero acknowledged discretion is the signature of
  assumption laundering, not of certainty.
- **`forecast.inputs.freshness`** — every input's `factRef` resolves; when `maxAgeDays`
  is declared and `asOfDate − asOf` exceeds it, reports `reviewRequired`. Dates are
  compared as ISO 8601; non-ISO dates surface as a warning (uncheckable).
- **`forecast.outputs.withinHorizon`** — scenario output periods fall inside the
  declared horizon (ISO 8601 prefix/range comparison); warns otherwise.
- **`forecast.backTest.resolves`** — when `backTest` is declared, its evidence, fact,
  and calculation references all resolve, and the predecessor evidence is external
  evidence pinned by SHA-256.
- **`forecast.claims.estimateAssertion`** — warns when no claim in the package carries
  the `estimate` assertion, and when a claim carrying it references no calculation or
  no judgment.

## Determinism and freshness

Validators are deterministic and MUST NOT consult wall-clock time
(see [spec/validations.md](../../spec/validations.md) §4). Input freshness is therefore
measured **against the register's `asOfDate`**, never against verification time: the
check asks "were the inputs fresh when this forecast was made?", and the same package
bytes always produce the same result. Whether a forecast is *now* too old to rely on is
a consumer reliance policy — informed by `asOfDate` and the advisory `refreshTrigger`
fields — not a validation status.

## The back-test loop

Forecasts have a property retrospective work papers lack: they generate their own future
evidence. When the horizon lapses, a successor package for the same subject SHOULD:

1. include the predecessor `.vwp` as external evidence pinned by SHA-256 (the same
   mechanism by which an ASC 606 package consumes an ASC 820 conclusion — see
   [`examples/dtcc-canton-milestone`](../../examples/dtcc-canton-milestone));
2. extract observed actuals for the lapsed horizon as facts with their own evidence;
3. carry a variance calculation comparing the predecessor's scenario outputs to actuals
   (`variance` is already a core calculation type); and
4. declare all three in the register's `backTest` block.

Over successive periods this produces an evidence-linked chain of forecast → actuals →
variance → revised forecast, in which the judgments that missed are individually
identifiable — the review loop closes in a way a point-in-time memo never allows.

## What this extension is not

A validated forecast package is not a prediction guarantee, a fairness opinion, or a
substitute for the preparer's own planning process. Validation demonstrates that the
projection is recomputable, its assumptions are explicit and reviewable, its inputs were
fresh and evidence-linked as of preparation, and its track record is auditable. The
future remains under no obligation to cooperate.
