/**
 * The agent loop (harness DESIGN §2, §5, §6): one leveled loop for every
 * model. System prompt = prompts/system.md + prompts/vwp-primer.md; the
 * packet is the first user message; the model works exclusively through the
 * tool registry; the run ends on submit() or budget exhaustion (then a
 * best-effort assembly, flagged in provenance notes).
 *
 * Every request/response, tool call, and usage record is appended to
 * <runDir>/transcript.jsonl so judges and third parties can audit what the
 * model actually saw and did.
 */
import * as fs from 'node:fs';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import { sha256Hex } from '@vwp/cli/lib';
import type { ChatMessage, ProviderAdapter, ToolSpec, Usage } from './adapters/types.js';
import { assembleToBuffer, HARNESS_VERSION } from './assemble.js';
import type { Draft, ModelIdentity } from './draft.js';
import type { Packet } from './packet.js';
import { createToolRegistry } from './tools.js';
import type { ValidationResult } from './validate.js';

export const PROMPT_VERSION = '1';
export const DEFAULT_MAX_STEPS = 200;
const MAX_STALLS = 3;

export interface RunLoopOptions {
  adapter: ProviderAdapter;
  packet: Packet;
  modelIdentity: ModelIdentity;
  /** Tool-call budget (DESIGN §6; default 200). */
  maxSteps?: number;
  /** Directory for the transcript and run artifacts (created if missing). */
  runDir: string;
}

export interface LoopResult {
  draft: Draft;
  /** The assembled .vwp — as produced, no touch-ups. */
  artifact: Buffer;
  submitted: boolean;
  /** True when the step budget ran out before submit() (flagged in provenance). */
  exhausted: boolean;
  steps: number;
  turns: number;
  toolErrors: number;
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
  lines.push('', '## Required-facts manifest', '', '```json', JSON.stringify(packet.requiredFacts, null, 2), '```');
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
    temperature: 0,
  });

  let steps = 0;
  let turns = 0;
  let stalls = 0;
  let toolErrors = 0;
  let exhausted = false;
  const usage: Usage = { inputTokens: 0, outputTokens: 0 };

  while (!registry.submitted) {
    if (steps >= maxSteps) {
      exhausted = true;
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
    });

    messages.push({ role: 'assistant', content: res.text, toolCalls: res.toolCalls });

    if (res.toolCalls.length === 0) {
      stalls++;
      if (stalls >= MAX_STALLS) {
        exhausted = true;
        log('stall.abort', { turn: turns, stalls });
        break;
      }
      messages.push({
        role: 'user',
        content:
          'You must interact exclusively through the provided tools. Continue the work; when the work product is complete, run validate() and then call submit().',
      });
      continue;
    }
    stalls = 0;

    for (const call of res.toolCalls) {
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
  }

  const notes = exhausted
    ? `Budget exhausted before submit() (steps=${steps}, maxSteps=${maxSteps}); best-effort submission assembled by the harness from the current draft. Harness ${HARNESS_VERSION}, prompt v${PROMPT_VERSION}.`
    : `Produced by vwp-benchmark-harness ${HARNESS_VERSION}, prompt v${PROMPT_VERSION}, temperature 0, maxSteps ${maxSteps}. Submitted by the model via submit(); no touch-ups.`;
  const artifact = assembleToBuffer(registry.draft, opts.packet, {
    modelIdentity: opts.modelIdentity,
    notes,
  });

  log('run.end', {
    submitted: registry.submitted,
    exhausted,
    steps,
    turns,
    toolErrors,
    usage,
    artifactSha256: sha256Hex(artifact),
  });

  return {
    draft: registry.draft,
    artifact,
    submitted: registry.submitted,
    exhausted,
    steps,
    turns,
    toolErrors,
    usage,
    lastValidation: registry.lastValidation,
    transcriptPath,
  };
}
