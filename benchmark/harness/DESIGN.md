# VWP Benchmark Harness — Design

**Status:** design (v0.1). Implements the benchmark's LLM arm (VWP-TT SPEC Flow
C2) as a standalone, open-source runner so that "every model ran the identical
harness" is an inspectable claim, not an assertion.

## 1. Purpose and fairness contract

The harness exists to make model comparisons attributable to the *model*, not
the scaffolding. Its fairness contract:

1. **One harness, many models.** Identical system prompt, tool surface,
   iteration policy, and budgets for every model; only the provider adapter
   differs. No model-specific prompt tuning — ever. Any prompt or tool change
   bumps the harness version, and results across harness versions are not
   comparable.
2. **Versioned and recorded.** The harness version, run parameters, and exact
   model identity (provider, model id, version — spec/provenance.md §3) are
   embedded in every submission's provenance and the platform's
   model-exposure ledger.
3. **Transcripts retained.** Every request/response and tool call is logged
   (prompt hash, tokens, latency) and stored with the run, so judges and
   third parties can audit what the model actually saw and did.
4. **No touch-ups.** Output is submitted as produced (DESIGN.md §4). Tool
   errors are returned to the model verbatim, uniformly; there is no
   out-of-band repair.

## 2. Architecture decision: agentic tool loop over a semantic tool surface

**Decision: a tool-calling agent loop with VWP-semantic tools, not single-shot
generation and not raw CLI/shell access.**

- Single-shot fails the task shape: verbatim quote anchors over ~10 evidence
  documents require reading tools; context windows differ across models and
  would otherwise become the variable under test.
- Raw shell (`vwp` CLI in a sandbox) measures shell fluency and escaping
  hygiene alongside accounting — noise for this benchmark, and a sandboxing
  burden for arbitrary models. (A future "raw mode" may exist for
  agent-competence studies; it is out of scope for v0.1.)
- Semantic tools mirror the human workbench's affordances (slot-aware fact
  entry, anchor capture, grid calculations), which is the parity we actually
  want: both arms get format mechanics for free; what's measured is the
  accounting.

Tool-use competence still varies by model; that is considered part of the
model's capability and is documented in methods, not compensated for.

## 3. Tool surface (v0.1)

State: the harness maintains a **draft document** (same shape as the human
workbench draft). Tools mutate it. All tool inputs are schema-validated;
violations return structured, actionable errors.

Reading:
- `read_brief()` → brief, deliverable spec, method steps, required-facts manifest
- `list_evidence()` → evidence index (id, title, type, pages)
- `read_evidence(evidenceId, page?)` → line-numbered text (the same extracted
  text layer humans anchor against and G2 verifies against)
- `search_evidence(query)` → normalized substring hits with (evidenceId, page,
  line) refs

Writing (mirror the workbench editors):
- `add_fact({slot?, factType, value, statement?, anchors: [{evidenceId, quote,
  page?, lines?}]})` — quote is checked against the text layer at call time;
  a non-matching quote is an immediate tool error the model can fix
- `add_judgment({topic, question, proposedConclusion, basis, alternatives:
  [{conclusion, notes}], standardRefs})` — alternatives itemization is part of
  the deliverable contract
- `add_assessment({stepKey?, statement, standardRefs, supportingFacts,
  judgments, supportingCalculations})` (claims, in workbench vocabulary)
- `add_calculation({type: sum|schedule|journalEntries|
  relativeStandaloneSellingPriceAllocation|custom, inputs, outputs, method})`
  — recomputable types are recomputed at call time; mismatches are tool errors
- `write_memo(markdown)`
- `update_*` / `remove_*` variants with the same schemas

Finishing:
- `validate()` → the full **offline check battery**, i.e. everything
  computable without vault material: package verification (schemas, link
  resolution, verbatim-quote anchors — the CLI's core validators), the
  structure-free machine tests (G1 evidence resolution, G2 anchor coverage,
  C1 support, K1 recomputation of the conventional calculation types, S2
  internal consistency), and P1 slot-fill against the public required-facts
  manifest. Construct-and-test is the harness's mandated procedure: the
  prompt requires a clean `validate()` before `submit()`, and validation
  failures are returned as structured, fixable findings. Never any keyed
  scoring (K2) or answer-key material — pre-submit feedback is limited to
  what a diligent human can equivalently self-check.
- `submit()` → posts the draft to the platform (or writes the local artifact)
  and ends the run.

## 4. Evidence access: text layers only (v0.1)

Models receive the extracted, line-numbered text layer — never raw PDF bytes —
because (a) provider-native PDF/vision support varies and would reintroduce
scaffold variance, and (b) it is the byte surface quote anchors verify
against. A future "multimodal mode" (original documents attached where
supported) is a separate, labeled comparison, not a default.

## 5. Prompt structure (versioned artifact in this directory)

1. Role framing (staff accountant persona, DESIGN.md §4).
2. **VWP primer** — a curated ~2k-token description of the construct chain
   (evidence → facts → claims → judgments → calculations), anchoring rules,
   and documentation expectations. The primer is a versioned file; the full
   spec is deliberately NOT inlined (token-budget variance, and the tool
   schemas carry the mechanics).
3. The packet: brief, deliverable spec, method steps, manifest, evidence index.
4. Procedure: work the method steps; anchor every fact with a verbatim quote;
   itemize alternatives per judgment; `validate()` before `submit()`.

Prompt history:
- **v2 (harness 0.2.0):** judgment wording now asks for "the alternative
  accounting positions … each with the reason it does not apply" instead of
  "the alternative conclusions you considered … why it was set aside", and an
  empty alternatives list "reads as incomplete documentation" instead of "work
  not shown". The v1 phrasing reads as a request for the model's own
  deliberation and drew deterministic `reasoning_extraction` refusals from
  Claude Opus 5.5, Sonnet 5.5, and Opus 5 (9 of 9 identical requests, no
  server-side fallback for that category). The deliverable contract is
  unchanged: judgments still itemize alternatives. The separate
  `claude+vwp-cli` scaffold (`prompts/claude-cli-run.md`) moved to v2 with
  the same wording change.

## 6. Run policy

- `--runs N` (default 3) independent runs, fresh context each.
- Temperature 0 where supported (else provider default, recorded in the
  transcript and provenance notes; Claude models that reject sampling
  parameters are listed in `src/adapters/anthropic.ts`); fixed max
  completion tokens (16,000 per response, thinking included, on the
  Anthropic adapter, which streams so a long response never hits an HTTP
  timeout; the OpenAI-compatible adapter does not yet send a cap); step
  budget (default 200 tool calls) and wall-clock budget aligned to the human
  time budget's spirit; exhaustion → best-effort `submit()` of the current
  draft, flagged in provenance notes.
- Thinking and effort are not sent, so each model runs at its provider
  defaults (on current Claude models, adaptive thinking at the model's
  default effort). Thinking blocks are not replayed in later turns.
- A response cut off by the token cap inside a tool call: that call's input
  is incomplete, so it is not executed and not counted as a tool error; the
  model is told it was cut off and re-issues it. A response cut off before
  any tool call gets a cap-specific nudge. Cut-off calls and turns that hit
  the cap are counted in the transcript and results table.
- Three consecutive turns without an executed tool call (none, or only a
  cut-off one) end the run as a stall; a provider refusal (Anthropic
  `stop_reason: "refusal"`, OpenAI `content_filter` or `message.refusal`)
  ends it at once, since a refusal on an unchanged conversation repeats.
  Both get the same best-effort assembly, with the end reason (and refusal
  category) in the transcript, provenance notes, and results table. Refused
  runs are submitted like any other, so they stay in the record.
- Seeds recorded when the provider supports them.

## 7. Provider adapters

Thin interface: `complete(messages, tools) → {message, toolCalls, usage}`,
plus the `temperature` each adapter sends (`null` = provider default).
Adapters: Anthropic, OpenAI, and generic OpenAI-compatible (vLLM/Ollama —
open-weight models run on the same rails). No agent frameworks; the loop is
~200 lines and fully auditable.

## 8. Platform interface (what the closed platform exposes)

The harness is platform-agnostic: it needs only an enrolled `llm_runner` API
key and three endpoints —
- `GET /api/challenges/:id/packet` (+ per-evidence text-layer URLs),
- `POST /api/challenges/:id/submissions` accepting a **draft JSON + model
  metadata** for the LLM arm (the platform assembles the .vwp with the same
  assembler as the human arm — identical provenance discipline, derived text
  layers, and packaging for both arms),
- `GET /api/jobs/:id` where applicable.

Platform-side deltas required: packet text-layer URLs; llm-arm draft-JSON
submission carrying `models[{provider, name, version}]` + harness version
(rejected if absent).

## 8a. Output modes: platform submit vs local artifact

Two run modes, one agent loop:

- **`--submit` (benchmark mode, default):** fetch the packet from a platform
  with an enrolled key; on `submit()` post the draft; the platform assembles
  the `.vwp` with the same assembler as the human arm and records the run in
  the model-exposure ledger. Use for all scored runs — the upload moment is
  what timestamps model exposure and enforces humans-before-models ordering.
- **`--out <file.vwp> [--packet-dir <dir>]` (local-artifact mode):** run
  against a packet directory on disk (brief, deliverable spec, manifest,
  evidence files + text layers) with no platform contact; assemble the
  package locally via the reference CLI and write the artifact. Use for
  harness development, dry runs, and third-party reproduction — zero
  model-exposure footprint. The artifact is Flow C1-compatible: it can be
  uploaded to a platform later (provenance rules apply identically).

Local-artifact mode also supports **offline pre-scoring**: `vwp verify` plus
the structure-free machine tests (G1, G2, C1, K1, S2) and P1 against the
public manifest need no vault material. Only keyed scoring (K2) requires the
platform. Assembly parity note: platform-assembled and CLI-assembled packages
are both spec-valid but may differ cosmetically; scored comparisons within a
results table use a single mode.

## 9. Methods notes to publish with results

Harness + prompt version; per-model run configs; token/step usage per run;
tool-error counts (a capability signal in itself) and, separately, calls cut
off by the token cap (the harness's limit, not a model error); any run that
hit budgets;
the statement that no model-specific adjustments were made beyond
provider-mandated request parameters (temperature omitted where the model
rejects it), which are recorded per run.

## 10. Open questions

1. Step/token budgets: fixed absolute, or scaled to provider limits? (v0.1:
   fixed absolute, documented.)
2. ~~Should `validate()` include package-level `vwp verify` outcomes?~~
   **Resolved:** yes — the full offline battery (verify + structure-free
   machine tests + P1); never keyed scoring.
3. Multimodal mode criteria and labeling.
4. Raw-CLI mode as a separate agent-competence track.
