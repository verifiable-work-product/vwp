import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { describe, expect, it } from 'vitest';
import { openPackageBuffer, verifyPackage } from '@vwp/cli/lib';
import { FakeAdapter, type FakeStep } from '../src/adapters/fake.js';
import type { ChatMessage, CompletionResult, ToolSpec } from '../src/adapters/types.js';
import { describeEnd, runLoop } from '../src/loop.js';
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

describe('agent loop with the scripted adapter', () => {
  it('runs the sequence, produces a verifiable .vwp, and writes the transcript', async () => {
    const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-loop-'));
    const packet = fixturePacket();
    const result = await runLoop({
      adapter: new FakeAdapter(SCRIPT),
      packet,
      modelIdentity: TEST_MODEL,
      runDir,
    });

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
    const outPath = path.join(runDir, 'out.vwp');
    fs.writeFileSync(outPath, result.artifact);
    const report = verifyPackage(openPackageBuffer(fs.readFileSync(outPath)));
    expect(report.valid).toBe(true);

    // Transcript JSONL: every request/response/toolcall/usage logged.
    expect(fs.existsSync(result.transcriptPath)).toBe(true);
    const events = fs
      .readFileSync(result.transcriptPath, 'utf8')
      .trim()
      .split('\n')
      .map((line) => JSON.parse(line));
    const kinds = new Set(events.map((e) => e.event));
    for (const expected of ['run.start', 'request', 'response', 'tool.call', 'tool.result', 'run.end']) {
      expect(kinds).toContain(expected);
    }
    expect(events.filter((e) => e.event === 'tool.call')).toHaveLength(7);
    const end = events.find((e) => e.event === 'run.end');
    expect(end.submitted).toBe(true);
    expect(end.artifactSha256).toMatch(/^[a-f0-9]{64}$/);

    // The temperature actually sent is recorded in the transcript and provenance.
    expect(events.find((e) => e.event === 'run.start').temperature).toBe(0);
    const provenance = JSON.parse(openPackageBuffer(result.artifact).read('provenance/provenance.json').toString('utf8'));
    expect(provenance.notes).toContain('temperature 0,');
  });

  it('records the provider default when the adapter sends no temperature', async () => {
    class ProviderDefaultAdapter extends FakeAdapter {
      override readonly temperature = null;
    }
    const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-provider-default-'));
    const result = await runLoop({
      adapter: new ProviderDefaultAdapter(SCRIPT),
      packet: fixturePacket(),
      modelIdentity: TEST_MODEL,
      runDir,
    });
    expect(result.submitted).toBe(true);
    const start = fs
      .readFileSync(result.transcriptPath, 'utf8')
      .trim()
      .split('\n')
      .map((line) => JSON.parse(line))
      .find((e) => e.event === 'run.start');
    expect(start.temperature).toBe('provider default');
    const provenance = JSON.parse(openPackageBuffer(result.artifact).read('provenance/provenance.json').toString('utf8'));
    expect(provenance.notes).toContain('temperature provider default,');
    expect(provenance.notes).not.toContain('temperature 0');
  });

  it('flags budget exhaustion in provenance on a script that never submits', async () => {
    const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-exhaust-'));
    const packet = fixturePacket();
    const result = await runLoop({
      adapter: new FakeAdapter([{ toolCalls: [{ name: 'read_brief', input: {} }] }, { toolCalls: [{ name: 'list_evidence', input: {} }] }]),
      packet,
      modelIdentity: TEST_MODEL,
      maxSteps: 2,
      runDir,
    });
    expect(result.submitted).toBe(false);
    expect(result.exhausted).toBe(true);
    expect(result.endReason).toBe('budget');
    expect(describeEnd(result)).toBe('budget exhaustion (best-effort assembly)');
    const pkg = openPackageBuffer(result.artifact);
    const provenance = JSON.parse(pkg.read('provenance/provenance.json').toString('utf8'));
    expect(provenance.notes).toContain('Budget exhausted');
    expect(provenance.notes).toContain('temperature 0.');
    expect(provenance.models[0]).toMatchObject(TEST_MODEL);
  });

  it('ends at the first provider refusal and records it as a refusal, not budget exhaustion', async () => {
    const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-refusal-'));
    const refusal = { category: 'reasoning_extraction', explanation: 'Blocked by policy.' };
    const result = await runLoop({
      adapter: new FakeAdapter([
        { toolCalls: [{ name: 'read_brief', input: {} }] },
        { refusal },
        { toolCalls: [{ name: 'list_evidence', input: {} }] },
      ]),
      packet: fixturePacket(),
      modelIdentity: TEST_MODEL,
      runDir,
    });
    expect(result.submitted).toBe(false);
    expect(result.exhausted).toBe(false);
    expect(result.endReason).toBe('refusal');
    expect(result.refusal).toEqual(refusal);
    expect(result.turns).toBe(2);
    expect(describeEnd(result)).toBe('refusal: reasoning_extraction (best-effort)');

    const events = readTranscript(result.transcriptPath);
    expect(events.filter((e) => e.event === 'request')).toHaveLength(2); // no re-asking after the refusal
    expect(events.find((e) => e.event === 'refusal.abort')).toMatchObject({ turn: 2, ...refusal });
    expect(events.find((e) => e.event === 'run.end')).toMatchObject({ endReason: 'refusal', refusal, exhausted: false });

    const provenance = JSON.parse(openPackageBuffer(result.artifact).read('provenance/provenance.json').toString('utf8'));
    expect(provenance.notes).toContain('Provider refused the request before submit() (turn 2, refusal category reasoning_extraction)');
    expect(provenance.notes).toContain('best-effort submission');
    expect(provenance.notes).not.toContain('Budget exhausted');
  });

  it('records three tool-less turns as a stall, not budget exhaustion', async () => {
    const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-stall-'));
    const result = await runLoop({
      adapter: new FakeAdapter([{ toolCalls: [{ name: 'read_brief', input: {} }] }, { text: 'a' }, { text: 'b' }, { text: 'c' }]),
      packet: fixturePacket(),
      modelIdentity: TEST_MODEL,
      runDir,
    });
    expect(result.submitted).toBe(false);
    expect(result.exhausted).toBe(false);
    expect(result.endReason).toBe('stall');
    expect(result.refusal).toBeUndefined();
    expect(describeEnd(result)).toBe('stall: no tool calls executed (best-effort assembly)');
    expect(readTranscript(result.transcriptPath).find((e) => e.event === 'stall.abort')).toMatchObject({ turn: 4, stalls: 3 });

    const provenance = JSON.parse(openPackageBuffer(result.artifact).read('provenance/provenance.json').toString('utf8'));
    expect(provenance.notes).toContain('No tool call executed in 3 consecutive turns before submit() (turn 4)');
    expect(provenance.notes).not.toContain('Budget exhausted');
  });

  it('does not execute a tool call cut off by the token cap, tells the model, and counts it apart from tool errors', async () => {
    class RecordingAdapter extends FakeAdapter {
      readonly seen: ChatMessage[][] = [];
      override async complete(messages: ChatMessage[], tools: ToolSpec[]): Promise<CompletionResult> {
        this.seen.push(structuredClone(messages));
        return super.complete(messages, tools);
      }
    }
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
    const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-truncated-'));
    const result = await runLoop({ adapter, packet: fixturePacket(), modelIdentity: TEST_MODEL, runDir });

    expect(result.submitted).toBe(true);
    expect(result.steps).toBe(2); // read_brief + submit; the cut-off call is not a step
    expect(result.toolErrors).toBe(0);
    expect(result.truncatedCalls).toBe(1);
    expect(result.draft.facts).toHaveLength(0);

    const events = readTranscript(result.transcriptPath);
    expect(events.filter((e) => e.event === 'tool.call').map((e) => e.name)).toEqual(['read_brief', 'submit']);
    expect(events.find((e) => e.event === 'tool.truncated')).toMatchObject({ turn: 1, id: 'fake_2', name: 'add_fact' });
    expect(events.find((e) => e.event === 'run.end')).toMatchObject({ truncatedCalls: 1, toolErrors: 0 });

    // The model's next request carries an error result for the cut-off call.
    const toolResult = adapter.seen[1].find((m) => m.role === 'tool' && m.toolCallId === 'fake_2');
    expect(toolResult).toMatchObject({ isError: true });
    expect(toolResult && 'content' in toolResult ? toolResult.content : '').toContain('output token limit');
  });

  it('treats repeated turns whose only call is cut off as a stall', async () => {
    const cutOnly = { truncated: true, toolCalls: [{ name: 'add_fact', input: { factType: 'x' } }] };
    const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-harness-truncated-stall-'));
    const result = await runLoop({
      adapter: new FakeAdapter([cutOnly, cutOnly, cutOnly, { toolCalls: [{ name: 'submit', input: {} }] }]),
      packet: fixturePacket(),
      modelIdentity: TEST_MODEL,
      runDir,
    });
    expect(result.endReason).toBe('stall');
    expect(result.turns).toBe(3);
    expect(result.steps).toBe(0);
    expect(result.truncatedCalls).toBe(3);
  });
});

function readTranscript(transcriptPath: string): Array<Record<string, unknown>> {
  return fs
    .readFileSync(transcriptPath, 'utf8')
    .trim()
    .split('\n')
    .map((line) => JSON.parse(line));
}
