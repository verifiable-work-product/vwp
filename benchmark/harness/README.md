# @vwp/benchmark-harness

The VWP-Bench **LLM-arm runner**: a standalone, open-source agent harness so
that "every model ran the identical harness" is an inspectable claim, not an
assertion. Design and rationale: [DESIGN.md](./DESIGN.md).

One agent loop, one system prompt ([prompts/system.md](./prompts/system.md) +
[prompts/vwp-primer.md](./prompts/vwp-primer.md)), one VWP-semantic tool
surface, fixed budgets — for every model. Only the provider adapter differs.

## Fairness contract (summary)

1. **One harness, many models.** No model-specific prompt or tool tuning,
   ever. Any prompt/tool change bumps the harness version; results are not
   comparable across harness versions.
2. **Versioned and recorded.** Harness version, run parameters, and exact
   model identity (provider, model id, version) are embedded in every
   artifact's provenance.
3. **Transcripts retained.** Every request, response, tool call, and usage
   record is written to `<run-dir>/transcript.jsonl`.
4. **No touch-ups.** The artifact is assembled exactly from what the model
   produced; tool errors are returned to the model verbatim. Budget
   exhaustion produces a best-effort assembly flagged in provenance notes.
5. **No keyed feedback.** The in-run `validate()` battery is strictly the
   offline, structure-level checks a diligent human could self-check
   (package verify + G1/G2/C1/K1/S2/P1) — never answer-key material.

## Setup

Requires Node 20+. The reference CLI must be built first (the harness reuses
its verifier and packager via `@vwp/cli/lib`):

```bash
cd ../../cli && npm ci && npm run build
cd ../benchmark/harness && npm install
npm test          # offline; no provider calls
```

## Quickstart — local mode (no platform contact)

Run against a packet directory on disk. Zero model-exposure footprint; ideal
for harness development and third-party reproduction.

```bash
# dry run with the scripted adapter (no API keys, no network)
npm run -s run -- --model fake --packet-dir test/fixtures/packet

# a real model against the local fixture packet
export ANTHROPIC_API_KEY=sk-ant-...
npm run -s run -- --model anthropic:<model-id> --packet-dir test/fixtures/packet --out out/workproduct.vwp
```

A local packet directory contains `packet.json` (brief, deliverable spec,
method steps, required-facts manifest, evidence index), `evidence/` (original
files), and `textlayers/<evidenceId>.txt` (extracted text layers; optional
for text-format originals). See `test/fixtures/packet/`.

## Quickstart — platform mode (benchmark runs)

```bash
export ANTHROPIC_API_KEY=sk-ant-...
npm run -s run -- \
  --model anthropic:<model-id> \
  --challenge <challengeId> \
  --base-url https://vwp-tt.web.app \
  --api-key ttk_...            # enrolled llm_runner key (or VWP_TT_API_KEY)
```

The packet URL is derived as `<base-url>/api/challenges/<challengeId>/packet`
(pass `--packet-url` to override). After the loop finishes the harness
**always**: assembles the `.vwp` locally, runs the full offline battery on
it, prints a results table (verify + G1/G2/C1/K1/S2/P1, steps, tokens), and
writes the artifact + transcript to the run directory. Only then does it ask
`Upload to <url>? [y/N]` before POSTing the artifact (as `application/zip`)
to `<base-url>/api/challenges/<challengeId>/submissions` — the Flow C1 upload
that timestamps model exposure. `--yes` skips the prompt (CI); `--no-upload`
never uploads; a non-interactive session without `--yes` prints the exact
`curl` to upload manually instead.

## Options

| Flag | Meaning |
|---|---|
| `--model <spec>` | `anthropic:<id>`, `openai:<id>`, or `fake` (scripted dry run) |
| `--challenge <id>` | Challenge id (platform mode) |
| `--base-url <url>` | Platform base URL (default `https://vwp-tt.web.app`) |
| `--api-key <key>` | Enrolled `llm_runner` key (or `VWP_TT_API_KEY` env) |
| `--packet-url <url>` | Explicit packet endpoint |
| `--packet-dir <dir>` | Local packet directory |
| `--out <file>` | Also copy the artifact here |
| `--runs <n>` | Independent runs, fresh context each (default 1) |
| `--max-steps <n>` | Tool-call budget per run (default 200) |
| `--run-dir <dir>` | Base directory for run outputs (default `./runs`) |
| `--model-version <v>` | Provenance model version (default: the model id) |
| `--yes` / `--no-upload` | Upload gate: skip the prompt / never upload |

## Environment variables

| Variable | Used by |
|---|---|
| `ANTHROPIC_API_KEY` | `anthropic:*` models |
| `OPENAI_API_KEY` | `openai:*` models |
| `OPENAI_BASE_URL` | OpenAI-compatible servers (vLLM/Ollama); default `https://api.openai.com/v1` |
| `VWP_TT_API_KEY` | Platform key fallback for `--api-key` |

Temperature is fixed at 0 for all providers. Tokens/steps and tool-error
counts are recorded per run for the methods notes.

## Layout

- `src/loop.ts` — the ~200-line auditable agent loop
- `src/tools.ts` — the VWP-semantic tool surface (anchor-checked `add_fact`,
  recomputed `add_calculation`, `validate`, `submit`, …)
- `src/validate.ts` — the offline battery (verify + G1/G2/C1/K1/S2/P1)
- `src/assemble.ts` — draft → `.vwp` packaging (same layout the CLI verifies)
- `src/packet.ts` — remote/local packet acquisition
- `src/adapters/` — Anthropic, OpenAI-compatible, and scripted adapters
- `prompts/` — the versioned prompt artifacts
- `test/fixtures/packet/` — a complete local packet built from the public
  `rev-606-saas` challenge data
