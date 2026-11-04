/**
 * The agent loop (harness DESIGN §2, §5, §6): one leveled loop for every
 * model. System prompt = prompts/system.md + prompts/vwp-primer.md; the
 * packet is the first user message; the model works exclusively through the
 * tool registry. The run ends on submit(), step-budget exhaustion, a stall
 * (three turns without an executed tool call), or a provider refusal; all
 * but submit() produce a best-effort assembly, flagged in provenance notes.
 *
 * Every request/response, tool call, and usage record is appended to
 * <runDir>/transcript.jsonl so judges and third parties can audit what the
 * model actually saw and did.
 */
import * as fs from 'node:fs';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import { sha256Hex } from '@vwp/cli/lib';
import type { ChatMessage, ProviderAdapter, Refusal, ToolSpec, Usage } from './adapters/types.js';
import { assembleToBuffer, HARNESS_VERSION } from './assemble.js';
import type { Draft, ModelIdentity } from './draft.js';
import type { Packet } from './packet.js';
import { createToolRegistry } from './tools.js';
import type { ValidationResult } from './validate.js';

export const PROMPT_VERSION = '2';
export const DEFAULT_MAX_STEPS = 200;
const MAX_STALLS = 3;
const TRUNCATED_CALL_MESSAGE =
  'Not executed: the response hit the output token limit while this call was being written, so its input is incomplete. Re-issue the call; if one response holds many calls, split them across turns.';
const TOOLS_ONLY_NUDGE =
  'You must interact exclusively through the provided tools. Continue the work; when the work product is complete, run validate() and then call submit().';
const OUTPUT_CAP_NUDGE =
  'Your previous response hit the output token limit before making a tool call, so nothing was done. Continue the work through the tools, keeping each response shorter.';

export interface RunLoopOptions {
  adapter: ProviderAdapter;
  packet: Packet;
  modelIdentity: ModelIdentity;
  /** Tool-call budget (DESIGN §6; default 200). */
  maxSteps?: number;
  /** Directory for the transcript and run artifacts (created if missing). */
  runDir: string;
}

/** Why the run ended (DESIGN §6). Anything but 'submit' is a best-effort assembly. */
export type EndReason = 'submit' | 'budget' | 'stall' | 'refusal';

export interface LoopResult {
  draft: Draft;
  /** The assembled .vwp — as produced, no touch-ups. */
  artifact: Buffer;
  submitted: boolean;
  /** True when the step budget ran out before submit(); same as endReason === 'budget'. */
  exhausted: boolean;
  endReason: EndReason;
  /** The provider refusal that ended the run, when endReason is 'refusal'. */
  refusal?: Refusal;
  steps: number;
  turns: number;
  toolErrors: number;
  /** Tool calls cut off by the response token cap: not executed, not tool errors. */
  truncatedCalls: number;
  /** Turns whose response hit the output token cap (with or without a cut-off call). */
  outputCapTurns: number;
  usage: Usage;
  lastValidation?: ValidationResult;
  transcriptPath: string;
}

function findPromptsDir(): string {
  const here = path.dirname(fileURLToPath(import.meta.url));
  for (const candidate of [path.join(here, '..', 'prompts'), path.join(here, '..', '..', 'prompts')]) {
    if (fs.existsSync(path.join(candidate, 'system.md'))) return candidate;
  }
  throw new Error('could not locate prompts/system.md relative to the harness');
}

export function buildSystemPrompt(): string {
  const dir = findPromptsDir();
  const system = fs.readFileSync(path.join(dir, 'system.md'), 'utf8');
  const primer = fs.readFileSync(path.join(dir, 'vwp-primer.md'), 'utf8');
  return `${system.trim()}\n\n${primer.trim()}\n`;
}

/** The packet as the first user message (DESIGN §5 item 3). */
export function renderPacketMessage(packet: Packet): string {
  const lines: string[] = [];
  lines.push(`# Challenge packet: ${packet.title} (${packet.challengeId})`);
  lines.push('', '## Engagement brief', '', packet.brief.trim());
  if (packet.deliverableSpec.trim().length > 0) {
    lines.push('', '## Deliverable spec', '', packet.deliverableSpec.trim());
  }
  if (packet.methodSteps.length > 0) {
    lines.push('', '## Method steps (work these in order)', '');
    for (const step of packet.methodSteps) {
      lines.push(`- **${step.key}** — ${step.title}: ${step.question}`);
      if (step.authorityRefs?.length) lines.push(`  - Authority: ${step.authorityRefs.join(', ')}`);
      if (step.guidance) lines.push(`  - Guidance: ${step.guidance}`);
    }
  }
  if (packet.requiredFacts.length === 0 && packet.factCatalog?.length) {
    lines.push(
      '',
      '## Fact catalog',
      '',
      'No required-facts manifest is disclosed for this challenge. The catalog below names common accounting fact types; deciding which facts are material to this case is part of the assignment. Capture every material fact as an anchored fact using the catalog factType that fits (or a reasonable namespaced factType if none does).',
      '',
      '```json',
      JSON.stringify(packet.factCatalog, null, 2),
      '```'
    );
  } else {
    lines.push('', '## Required-facts manifest', '', '```json', JSON.stringify(packet.requiredFacts, null, 2), '```');
  }
  lines.push('', '## Evidence index', '');
  for (const ev of packet.evidence) {
    lines.push(`- ${ev.id} — ${ev.title} (${ev.type}, ${ev.pages.length} page${ev.pages.length === 1 ? '' : 's'})`);
  }
  lines.push(
    '',
    'Begin now. Work exclusively through the tools; anchor every fact with a verbatim quote; run validate() and fix all findings before submit().'
  );
  return lines.join('\n');
}

export async function runLoop(opts: RunLoopOptions): Promise<LoopResult> {
  const maxSteps = opts.maxSteps ?? DEFAULT_MAX_STEPS;
  // Transcript keeps the raw value (number, or null = provider default); notes get prose.
  const temperature = opts.adapter.temperature ?? 'provider default';
  fs.mkdirSync(opts.runDir, { recursive: true });
  const transcriptPath = path.join(opts.runDir, 'transcript.jsonl');
  const log = (event: string, data: Record<string, unknown>) => {
    fs.appendFileSync(transcriptPath, JSON.stringify({ ts: new Date().toISOString(), event, ...data }) + '\n');
  };

  const registry = createToolRegistry(opts.packet, { modelIdentity: opts.modelIdentity });
  const toolSpecs: ToolSpec[] = registry.tools.map((t) => ({
    name: t.name,
    description: t.description,
    inputSchema: t.inputSchema,
  }));

  const messages: ChatMessage[] = [
    { role: 'system', content: buildSystemPrompt() },
    { role: 'user', content: renderPacketMessage(opts.packet) },
  ];

  log('run.start', {
    harness: { name: 'vwp-benchmark-harness', version: HARNESS_VERSION, promptVersion: PROMPT_VERSION },
    model: opts.modelIdentity,
    provider: opts.adapter.provider,
    challengeId: opts.packet.challengeId,
    maxSteps,
    temperature: opts.adapter.temperature,
  });

  let steps = 0;
  let turns = 0;
  let stalls = 0;
  let toolErrors = 0;
  let truncatedCalls = 0;
  let outputCapTurns = 0;
  let endReason: EndReason = 'submit';
  let refusal: Refusal | undefined;
  const usage: Usage = { inputTokens: 0, outputTokens: 0 };

  while (!registry.submitted) {
    if (steps >= maxSteps) {
      endReason = 'budget';
      break;
    }
    turns++;
    const requestSha = sha256Hex(Buffer.from(JSON.stringify({ messages, tools: toolSpecs.map((t) => t.name) })));
    log('request', { turn: turns, promptSha256: requestSha, messageCount: messages.length, toolCount: toolSpecs.length });
    const started = Date.now();
    const res = await opts.adapter.complete(messages, toolSpecs);
    usage.inputTokens += res.usage.inputTokens;
    usage.outputTokens += res.usage.outputTokens;
    log('response', {
      turn: turns,
      latencyMs: Date.now() - started,
      stopReason: res.stopReason,
      text: res.text,
      toolCalls: res.toolCalls,
      usage: res.usage,
      refusal: res.refusal,
    });

    if (res.refusal) {
      // A refusal on an unchanged conversation repeats; re-asking only spends tokens.
      refusal = res.refusal;
      endReason = 'refusal';
      log('refusal.abort', { turn: turns, category: refusal.category, explanation: refusal.explanation });
      break;
    }

    messages.push({ role: 'assistant', content: res.text, toolCalls: res.toolCalls });
    if (res.hitOutputCap) outputCapTurns++;

    const cutIndex = res.lastToolCallTruncated ? res.toolCalls.length - 1 : -1;
    let executed = 0;
    for (const [index, call] of res.toolCalls.entries()) {
      if (index === cutIndex) {
        // Cut off by the response token cap: the input is incomplete, so the
        // call is not run or charged as a tool error; the model re-issues it.
        truncatedCalls++;
        log('tool.truncated', { turn: turns, id: call.id, name: call.name, input: call.input });
        messages.push({
          role: 'tool',
          toolCallId: call.id,
          name: call.name,
          content: JSON.stringify({ error: { message: TRUNCATED_CALL_MESSAGE } }),
          isError: true,
        });
        continue;
      }
      executed++;
      steps++;
      log('tool.call', { turn: turns, step: steps, id: call.id, name: call.name, input: call.input });
      const outcome = registry.execute(call.name, call.input);
      if (!outcome.ok) toolErrors++;
      log('tool.result', {
        turn: turns,
        step: steps,
        id: call.id,
        name: call.name,
        ok: outcome.ok,
        ...(outcome.ok ? { result: outcome.result } : { error: outcome.error }),
      });
      messages.push({
        role: 'tool',
        toolCallId: call.id,
        name: call.name,
        content: JSON.stringify(outcome.ok ? outcome.result : { error: outcome.error }),
        isError: !outcome.ok,
      });
      if (registry.submitted) break;
    }

    if (executed > 0) {
      stalls = 0;
      continue;
    }
    // No tool call ran this turn (none made, or only a cut-off one); repeated, it is a stall.
    if (++stalls >= MAX_STALLS) {
      endReason = 'stall';
      log('stall.abort', { turn: turns, stalls });
      break;
    }
    if (res.toolCalls.length === 0) {
      messages.push({ role: 'user', content: res.hitOutputCap ? OUTPUT_CAP_NUDGE : TOOLS_ONLY_NUDGE });
    }
  }
  const exhausted = endReason === 'budget';

  const bestEffort = `best-effort submission assembled by the harness from the current draft. Harness ${HARNESS_VERSION}, prompt v${PROMPT_VERSION}, temperature ${temperature}.`;
  const notes = {
    submit: `Produced by vwp-benchmark-harness ${HARNESS_VERSION}, prompt v${PROMPT_VERSION}, temperature ${temperature}, maxSteps ${maxSteps}. Submitted by the model via submit(); no touch-ups.`,
    budget: `Budget exhausted before submit() (steps=${steps}, maxSteps=${maxSteps}); ${bestEffort}`,
    stall: `No tool call executed in ${MAX_STALLS} consecutive turns before submit() (turn ${turns}); ${bestEffort}`,
    refusal: `Provider refused the request before submit() (turn ${turns}, refusal category ${refusal?.category ?? 'not given'}); ${bestEffort}`,
  }[endReason];
  const artifact = assembleToBuffer(registry.draft, opts.packet, {
    modelIdentity: opts.modelIdentity,
    notes,
  });

  log('run.end', {
    submitted: registry.submitted,
    exhausted,
    endReason,
    refusal,
    steps,
    turns,
    toolErrors,
    truncatedCalls,
    outputCapTurns,
    usage,
    artifactSha256: sha256Hex(artifact),
  });

  return {
    draft: registry.draft,
    artifact,
    submitted: registry.submitted,
    exhausted,
    endReason,
    refusal,
    steps,
    turns,
    toolErrors,
    truncatedCalls,
    outputCapTurns,
    usage,
    lastValidation: registry.lastValidation,
    transcriptPath,
  };
}

/** One-line "ended by" label for the results table. */
export function describeEnd(result: Pick<LoopResult, 'endReason' | 'refusal'>): string {
  switch (result.endReason) {
    case 'submit':
      return 'submit()';
    case 'budget':
      return 'budget exhaustion (best-effort assembly)';
    case 'stall':
      return 'stall: no tool calls executed (best-effort assembly)';
    case 'refusal':
      return `refusal: ${result.refusal?.category ?? 'no category'} (best-effort)`;
  }
}
