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
- `validate()` → local draft checks equivalent to the human workbench's
  warnings: unfilled required slots, unanchored facts, unsupported assessment
  items, calculation mismatches. Never any answer-key material; never machine
  test results (anti-gaming parity with the human receipt).
- `submit()` → posts the draft to the platform and ends the run.

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

## 6. Run policy

- `--runs N` (default 3) independent runs, fresh context each.
- Temperature 0 where supported (else provider default, recorded); fixed max
  completion tokens; step budget (default 200 tool calls) and wall-clock
  budget aligned to the human time budget's spirit; exhaustion → best-effort
  `submit()` of the current draft, flagged in provenance notes.
- Seeds recorded when the provider supports them.

## 7. Provider adapters

Thin interface: `complete(messages, tools) → {message, toolCalls, usage}`.
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

## 9. Methods notes to publish with results

Harness + prompt version; per-model run configs; token/step usage per run;
tool-error counts (a capability signal in itself); any run that hit budgets;
the statement that no model-specific adjustments were made.

## 10. Open questions

1. Step/token budgets: fixed absolute, or scaled to provider limits? (v0.1:
   fixed absolute, documented.)
2. Should `validate()` include package-level `vwp verify` outcomes, or only
   workbench-warning parity? (v0.1: workbench parity only.)
3. Multimodal mode criteria and labeling.
4. Raw-CLI mode as a separate agent-competence track.
