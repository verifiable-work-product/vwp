import { describe, expect, it } from 'vitest';
import { recomputeCalculation } from '../src/validate.js';
import { createToolRegistry } from '../src/tools.js';
import { fixturePacket, TEST_MODEL } from './helpers.js';

describe('K1 recompute — sum', () => {
  it('passes when outputs.total equals the sum of inputs.values', () => {
    const r = recomputeCalculation({ type: 'sum', inputs: { values: [1, 2, 3.5] }, outputs: { total: 6.5 } });
    expect(r.applicable).toBe(true);
    expect(r.problems).toEqual([]);
  });

  it('accepts decimal strings (currency-safe representation)', () => {
    const r = recomputeCalculation({
      type: 'sum',
      inputs: { values: ['100.50', '0.50', '1,000.00'] },
      outputs: { total: '1101.00' },
    });
    expect(r.problems).toEqual([]);
  });

  it('fails on a mismatched total beyond the 0.01 tolerance', () => {
    const r = recomputeCalculation({ type: 'sum', inputs: { values: [1, 2, 3.5] }, outputs: { total: 6.4 } });
    expect(r.problems.length).toBeGreaterThan(0);
    expect(r.problems[0]).toContain('6.5');
  });
});

describe('K1 recompute — schedule', () => {
  const rows = [{ label: 'sub', values: [10, 20, 30] }, { label: 'impl', values: [5, 5, 5] }];

  it('passes with row, column, and grand totals', () => {
    const r = recomputeCalculation({
      type: 'schedule',
      inputs: { rows },
      outputs: { rowTotals: [60, 15], columnTotals: [15, 25, 35], grandTotal: 75 },
    });
    expect(r.problems).toEqual([]);
  });

  it('fails when a column total is off', () => {
    const r = recomputeCalculation({
      type: 'schedule',
      inputs: { rows },
      outputs: { rowTotals: [60, 15], columnTotals: [15, 25, 34], grandTotal: 75 },
    });
    expect(r.problems.some((p) => p.includes('columnTotals[2]'))).toBe(true);
  });

  it('fails when grandTotal is missing', () => {
    const r = recomputeCalculation({ type: 'schedule', inputs: { rows }, outputs: { rowTotals: [60, 15] } });
    expect(r.problems.some((p) => p.includes('grandTotal'))).toBe(true);
  });
});

describe('K1 recompute — journalEntries', () => {
  const entries = [
    {
      date: '2026-01-05',
      description: 'Billing',
      lines: [
        { account: 'Accounts receivable', debit: 275000 },
        { account: 'Deferred revenue', credit: 275000 },
      ],
    },
    {
      date: '2026-03-31',
      description: 'Recognition',
      lines: [
        { account: 'Deferred revenue', debit: 45000 },
        { account: 'Revenue', credit: 45000 },
      ],
    },
  ];

  it('passes when totals recompute from lines', () => {
    const r = recomputeCalculation({
      type: 'journalEntries',
      inputs: { entries },
      outputs: { totalDebits: 320000, totalCredits: 320000 },
    });
    expect(r.problems).toEqual([]);
  });

  it('fails when declared totals do not match the lines', () => {
    const r = recomputeCalculation({
      type: 'journalEntries',
      inputs: { entries },
      outputs: { totalDebits: 320000, totalCredits: 275000 },
    });
    expect(r.problems.some((p) => p.includes('totalCredits'))).toBe(true);
  });
});

describe('K1 recompute — relativeStandaloneSellingPriceAllocation', () => {
  const inputs = {
    transactionPrice: 270000,
    components: [
      { id: 'subscription_y1', standaloneSellingPrice: 180000 },
      { id: 'implementation', standaloneSellingPrice: 120000 },
    ],
  };

  it('passes when allocations equal tp*ssp/totalSSP and tie out', () => {
    const r = recomputeCalculation({
      type: 'relativeStandaloneSellingPriceAllocation',
      inputs,
      outputs: {
        allocations: [
          { componentId: 'subscription_y1', allocatedAmount: 162000 },
          { componentId: 'implementation', allocatedAmount: 108000 },
        ],
        totalStandaloneSellingPrice: 300000,
      },
    });
    expect(r.applicable).toBe(true);
    expect(r.problems).toEqual([]);
  });

  it('fails when an allocation is off or does not tie out', () => {
    const r = recomputeCalculation({
      type: 'relativeStandaloneSellingPriceAllocation',
      inputs,
      outputs: {
        allocations: [
          { componentId: 'subscription_y1', allocatedAmount: 162000 },
          { componentId: 'implementation', allocatedAmount: 107000 },
        ],
      },
    });
    expect(r.problems.some((p) => p.includes('implementation'))).toBe(true);
    expect(r.problems.some((p) => p.includes('tie out'))).toBe(true);
  });

  it('fails when the allocation count differs from the component count', () => {
    const r = recomputeCalculation({
      type: 'relativeStandaloneSellingPriceAllocation',
      inputs,
      outputs: { allocations: [{ allocatedAmount: 270000 }] },
    });
    expect(r.problems.length).toBeGreaterThan(0);
  });

  it('rejects the retired items/ssp dialect (must match the platform K1 contract)', () => {
    const r = recomputeCalculation({
      type: 'relativeStandaloneSellingPriceAllocation',
      inputs: { transactionPrice: 270000, items: [{ name: 'subscription', ssp: 270000 }] },
      outputs: { allocations: [{ name: 'subscription', allocatedAmount: 270000 }] },
    });
    expect(r.problems.some((p) => p.includes('components'))).toBe(true);
  });
});

describe('K1 — custom types are not recomputed', () => {
  it('marks custom types not applicable', () => {
    const r = recomputeCalculation({ type: 'custom', inputs: { anything: true }, outputs: { whatever: 1 } });
    expect(r.applicable).toBe(false);
    expect(r.problems).toEqual([]);
  });
});

describe('add_calculation recomputes at call time', () => {
  it('accepts a clean conventional calculation and defaults recomputable', () => {
    const reg = createToolRegistry(fixturePacket(), { modelIdentity: TEST_MODEL });
    const outcome = reg.execute('add_calculation', {
      type: 'sum',
      inputs: { values: [180000, 90000] },
      outputs: { total: 270000 },
      method: 'year-1 fixed consideration',
    });
    expect(outcome.ok).toBe(true);
    expect(reg.draft.calculations[0].recomputable).toBe(true);
  });

  it('returns a structured tool error on mismatch', () => {
    const reg = createToolRegistry(fixturePacket(), { modelIdentity: TEST_MODEL });
    const outcome = reg.execute('add_calculation', {
      type: 'sum',
      inputs: { values: [180000, 90000] },
      outputs: { total: 275000 },
    });
    expect(outcome.ok).toBe(false);
    if (!outcome.ok) {
      expect(outcome.error.message).toContain('does not recompute');
      expect(outcome.error.message).toContain('270000');
    }
    expect(reg.draft.calculations).toHaveLength(0);
  });
});
