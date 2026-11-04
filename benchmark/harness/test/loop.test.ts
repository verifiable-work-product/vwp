import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { describe, expect, it } from 'vitest';
import { openPackageBuffer, verifyPackage } from '@vwp/cli/lib';
import { FakeAdapter, type FakeStep } from '../src/adapters/fake.js';
import type { ChatMessage, CompletionResult, ProviderAdapter, ToolSpec } from '../src/adapters/types.js';
import { describeEnd, runLoop, type LoopResult } from '../src/loop.js';
import { fixturePacket, TEST_MODEL } from './helpers.js';

const SCRIPT: FakeStep[] = [
  { text: 'Reading the brief.', toolCalls: [{ name: 'read_brief', input: {} }] },
  { toolCalls: [{ name: 'read_evidence', input: { evidenceId: 'ev_order_form_1' } }] },
  {
    toolCalls: [
      {
        name: 'add_fact',
        input: {
          slot: 'transactionPrice',
          factType: 'asc606.transactionPrice',
          value: { amount: 630000, currency: 'USD' },
          statement: 'Total fixed fees under Order Form #OF-2026-011 are $630,000.',
          anchors: [
            { evidenceId: 'ev_order_form_1', quote: 'TOTAL FIXED FEES UNDER THIS ORDER FORM (36 months): $630,000' },
          ],
        },
      },
    ],
  },
  {
    toolCalls: [
      {
        name: 'add_assessment',
        input: {
          stepKey: 'step3_price',
          statement: 'The fixed consideration in the initial contract is $630,000 over 36 months.',
          standardRefs: ['ASC 606-10-32-2'],
          assertions: ['valuationAllocation'],
          supportingFacts: ['f_1'],
        },
      },
    ],
  },
  { toolCalls: [{ name: 'write_memo', input: { markdown: '# Memo (scripted loop test)\n\nAbbreviated.\n' } }] },
  { toolCalls: [{ name: 'validate', input: {} }] },
  { text: 'Submitting.', toolCalls: [{ name: 'submit', input: {} }] },
];

/** Records every conversation the loop sends, then answers like FakeAdapter. */
class RecordingAdapter extends FakeAdapter {
  readonly seen: ChatMessage[][] = [];
  override async complete(messages: ChatMessage[], tools: ToolSpec[]): Promise<CompletionResult> {
    this.seen.push(structuredClone(messages));
    return super.complete(messages, tools);
  }
}

async function run(adapter: ProviderAdapter, maxSteps?: number): Promise<LoopResult> {
  const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-loop-'));
  return runLoop({ adapter, packet: fixturePacket(), modelIdentity: TEST_MODEL, runDir, ...(maxSteps ? { maxSteps } : {}) });
}

function readTranscript(transcriptPath: string): Array<Record<string, unknown>> {
  return fs
    .readFileSync(transcriptPath, 'utf8')
    .trim()
    .split('\n')
    .map((line) => JSON.parse(line));
}

function event(result: LoopResult, name: string): Record<string, unknown> | undefined {
  return readTranscript(result.transcriptPath).find((e) => e.event === name);
}

function provenanceNotes(result: LoopResult): string {
  return JSON.parse(openPackageBuffer(result.artifact).read('provenance/provenance.json').toString('utf8')).notes;
}

describe('agent loop with the scripted adapter', () => {
  it('runs the sequence, produces a verifiable .vwp, and writes the transcript', async () => {
    const result = await run(new FakeAdapter(SCRIPT));

    expect(result.submitted).toBe(true);
    expect(result.exhausted).toBe(false);
    expect(result.endReason).toBe('submit');
    expect(describeEnd(result)).toBe('submit()');
    expect(result.steps).toBe(7);
    expect(result.toolErrors).toBe(0);
    expect(result.draft.facts).toHaveLength(1);
    expect(result.draft.claims).toHaveLength(1);

    // validate() ran mid-loop and its result is retained (P1 fails on the
    // 7 unfilled slots — pre-submit feedback, not a block).
    expect(result.lastValidation).toBeDefined();
    expect(result.lastValidation!.verify.valid).toBe(true);

    // The artifact is a spec-valid .vwp the reference verifier accepts.
    const outPath = path.join(path.dirname(result.transcriptPath), 'out.vwp');
    fs.writeFileSync(outPath, result.artifact);
    const report = verifyPackage(openPackageBuffer(fs.readFileSync(outPath)));
    expect(report.valid).toBe(true);

    // Transcript JSONL: every request/response/toolcall/usage logged.
    expect(fs.existsSync(result.transcriptPath)).toBe(true);
    const events = readTranscript(result.transcriptPath);
    const kinds = new Set(events.map((e) => e.event));
    for (const expected of ['run.start', 'request', 'response', 'tool.call', 'tool.result', 'run.end']) {
      expect(kinds).toContain(expected);
    }
    expect(events.filter((e) => e.event === 'tool.call')).toHaveLength(7);
    const end = events.find((e) => e.event === 'run.end')!;
    expect(end.submitted).toBe(true);
    expect(end.artifactSha256).toMatch(/^[a-f0-9]{64}$/);

    // The temperature actually sent is recorded in the transcript and provenance.
    expect(event(result, 'run.start')?.temperature).toBe(0);
    expect(provenanceNotes(result)).toContain('temperature 0,');
  });

  it('records the provider default when the adapter sends no temperature', async () => {
    class ProviderDefaultAdapter extends FakeAdapter {
      override readonly temperature = null;
    }
    const result = await run(new ProviderDefaultAdapter(SCRIPT));
    expect(result.submitted).toBe(true);
    expect(event(result, 'run.start')?.temperature).toBeNull();
    expect(provenanceNotes(result)).toContain('temperature provider default,');
    expect(provenanceNotes(result)).not.toContain('temperature 0');
  });

  it('flags budget exhaustion in provenance on a script that never submits', async () => {
    const result = await run(
      new FakeAdapter([{ toolCalls: [{ name: 'read_brief', input: {} }] }, { toolCalls: [{ name: 'list_evidence', input: {} }] }]),
      2,
    );
    expect(result.submitted).toBe(false);
    expect(result.exhausted).toBe(true);
    expect(result.endReason).toBe('budget');
    expect(describeEnd(result)).toBe('budget exhaustion (best-effort assembly)');
    expect(provenanceNotes(result)).toContain('Budget exhausted');
    expect(provenanceNotes(result)).toContain('temperature 0.');
    const provenance = JSON.parse(openPackageBuffer(result.artifact).read('provenance/provenance.json').toString('utf8'));
    expect(provenance.models[0]).toMatchObject(TEST_MODEL);
  });
});

describe('provider refusals', () => {
  const refusal = { category: 'reasoning_extraction', explanation: 'Blocked by policy.' };

  it('ends at the first refusal and records it as a refusal, not budget exhaustion', async () => {
    const result = await run(
      new FakeAdapter([{ toolCalls: [{ name: 'read_brief', input: {} }] }, { refusal }, { toolCalls: [{ name: 'list_evidence', input: {} }] }]),
    );
    expect(result.submitted).toBe(false);
    expect(result.exhausted).toBe(false);
    expect(result.endReason).toBe('refusal');
    expect(result.refusal).toEqual(refusal);
    expect(result.turns).toBe(2);
    expect(describeEnd(result)).toBe('refusal: reasoning_extraction (best-effort)');

    const events = readTranscript(result.transcriptPath);
    expect(events.filter((e) => e.event === 'request')).toHaveLength(2); // no re-asking after the refusal
    expect(event(result, 'refusal.abort')).toMatchObject({ turn: 2, ...refusal });
    expect(event(result, 'run.end')).toMatchObject({ endReason: 'refusal', refusal, exhausted: false });

    expect(provenanceNotes(result)).toContain('Provider refused the request before submit() (turn 2, refusal category reasoning_extraction)');
    expect(provenanceNotes(result)).toContain('best-effort submission');
    expect(provenanceNotes(result)).not.toContain('Budget exhausted');
  });

  it('still assembles a verifiable best-effort artifact when the very first turn is refused', async () => {
    const result = await run(new FakeAdapter([{ refusal: { category: null, explanation: null } }]));
    expect(result.endReason).toBe('refusal');
    expect(result.turns).toBe(1);
    expect(result.steps).toBe(0);
    expect(describeEnd(result)).toBe('refusal: no category (best-effort)');
    expect(verifyPackage(openPackageBuffer(result.artifact)).valid).toBe(true);
    expect(provenanceNotes(result)).toContain('refusal category not given');
  });
});

describe('stalls', () => {
  it('records three tool-less turns as a stall, not budget exhaustion', async () => {
    const adapter = new RecordingAdapter([{ toolCalls: [{ name: 'read_brief', input: {} }] }, { text: 'a' }, { text: 'b' }, { text: 'c' }]);
    const result = await run(adapter);
    expect(result.submitted).toBe(false);
    expect(result.exhausted).toBe(false);
    expect(result.endReason).toBe('stall');
    expect(result.refusal).toBeUndefined();
    expect(describeEnd(result)).toBe('stall: no tool calls executed (best-effort assembly)');
    expect(event(result, 'stall.abort')).toMatchObject({ turn: 4, stalls: 3 });
    expect(provenanceNotes(result)).toContain('No tool call executed in 3 consecutive turns before submit() (turn 4)');
    expect(provenanceNotes(result)).not.toContain('Budget exhausted');
    // Each tool-less turn before the abort is answered with the tools-only nudge.
    expect(adapter.seen[2].at(-1)).toMatchObject({ role: 'user', content: expect.stringContaining('exclusively through the provided tools') });
  });

  it('resets the stall count after a turn that executes a call', async () => {
    const result = await run(
      new FakeAdapter([{ text: 'a' }, { text: 'b' }, { toolCalls: [{ name: 'read_brief', input: {} }] }, { text: 'c' }, { text: 'd' }, { toolCalls: [{ name: 'submit', input: {} }] }]),
    );
    expect(result.endReason).toBe('submit');
    expect(result.turns).toBe(6);
  });
});

describe('output token cap', () => {
  it('does not execute a tool call cut off by the cap, tells the model, and counts it apart from tool errors', async () => {
    const adapter = new RecordingAdapter([
      // fake_1 completes; fake_2 is cut off mid-input (no anchors), as Opus 5.5 did on 2026-11-03.
      {
        truncated: true,
        toolCalls: [
          { name: 'read_brief', input: {} },
          { name: 'add_fact', input: { factType: 'asc606.transactionPrice' } },
        ],
      },
      { toolCalls: [{ name: 'submit', input: {} }] },
    ]);
    const result = await run(adapter);

    expect(result.submitted).toBe(true);
    expect(result.steps).toBe(2); // read_brief + submit; the cut-off call is not a step
    expect(result.toolErrors).toBe(0);
    expect(result.truncatedCalls).toBe(1);
    expect(result.outputCapTurns).toBe(1);
    expect(result.draft.facts).toHaveLength(0);

    const events = readTranscript(result.transcriptPath);
    expect(events.filter((e) => e.event === 'tool.call').map((e) => e.name)).toEqual(['read_brief', 'submit']);
    expect(event(result, 'tool.truncated')).toMatchObject({ turn: 1, id: 'fake_2', name: 'add_fact' });
    expect(event(result, 'run.end')).toMatchObject({ truncatedCalls: 1, outputCapTurns: 1, toolErrors: 0 });

    // The model's next request carries an error result for the cut-off call.
    const toolResult = adapter.seen[1].find((m) => m.role === 'tool' && m.toolCallId === 'fake_2');
    expect(toolResult).toMatchObject({ isError: true, content: expect.stringContaining('output token limit') });
  });

  it('nudges about the cap, not about tools, when the cap was hit before any tool call', async () => {
    const adapter = new RecordingAdapter([{ truncated: true, text: 'Let me think' }, { toolCalls: [{ name: 'submit', input: {} }] }]);
    const result = await run(adapter);
    expect(result.submitted).toBe(true);
    expect(result.outputCapTurns).toBe(1);
    expect(result.truncatedCalls).toBe(0);
    const nudge = adapter.seen[1].at(-1);
    expect(nudge).toMatchObject({ role: 'user', content: expect.stringContaining('hit the output token limit before making a tool call') });
    expect(nudge).not.toMatchObject({ content: expect.stringContaining('exclusively through the provided tools') });
  });

  it('treats repeated turns whose only call is cut off as a stall', async () => {
    const cutOnly: FakeStep = { truncated: true, toolCalls: [{ name: 'add_fact', input: { factType: 'x' } }] };
    const result = await run(new FakeAdapter([cutOnly, cutOnly, cutOnly, { toolCalls: [{ name: 'submit', input: {} }] }]));
    expect(result.endReason).toBe('stall');
    expect(result.turns).toBe(3);
    expect(result.steps).toBe(0);
    expect(result.truncatedCalls).toBe(3);
  });

  it('executes tool calls that have no id (some OpenAI-compatible servers omit them)', async () => {
    // Regression: matching the cut-off call by id treated every id-less call as cut off.
    class NoIdAdapter extends FakeAdapter {
      override async complete(messages: ChatMessage[], tools: ToolSpec[]): Promise<CompletionResult> {
        const res = await super.complete(messages, tools);
        return { ...res, toolCalls: res.toolCalls.map(({ name, input }) => ({ name, input }) as CompletionResult['toolCalls'][number]) };
      }
    }
    const result = await run(new NoIdAdapter([{ toolCalls: [{ name: 'read_brief', input: {} }, { name: 'submit', input: {} }] }]));
    expect(result.submitted).toBe(true);
    expect(result.steps).toBe(2);
    expect(result.truncatedCalls).toBe(0);
  });
});
