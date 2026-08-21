import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { describe, expect, it } from 'vitest';
import { openPackageBuffer, verifyPackage } from '@vwp/cli/lib';
import { FakeAdapter, type FakeStep } from '../src/adapters/fake.js';
import { runLoop } from '../src/loop.js';
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
    const pkg = openPackageBuffer(result.artifact);
    const provenance = JSON.parse(pkg.read('provenance/provenance.json').toString('utf8'));
    expect(provenance.notes).toContain('Budget exhausted');
    expect(provenance.models[0]).toMatchObject(TEST_MODEL);
  });
});
