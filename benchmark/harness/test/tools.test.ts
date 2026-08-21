import { describe, expect, it } from 'vitest';
import { createToolRegistry } from '../src/tools.js';
import { normalizeForQuoteMatch } from '../src/validate.js';
import { fixturePacket, TEST_MODEL } from './helpers.js';

const REAL_QUOTE = 'TOTAL FIXED FEES UNDER THIS ORDER FORM (36 months): $630,000';

function registry() {
  return createToolRegistry(fixturePacket(), { modelIdentity: TEST_MODEL });
}

describe('quote normalization', () => {
  it('maps curly quotes and dashes to ASCII and collapses whitespace', () => {
    expect(normalizeForQuoteMatch('a  “smart”  ‘quote’ — and – dashes\n over\tlines')).toBe(
      'a "smart" \'quote\' - and - dashes over lines'
    );
  });

  it('is NFC-normalized', () => {
    expect(normalizeForQuoteMatch('café')).toBe(normalizeForQuoteMatch('café'));
  });
});

describe('add_fact anchor checking', () => {
  it('accepts a verbatim quote from the evidence', () => {
    const reg = registry();
    const outcome = reg.execute('add_fact', {
      slot: 'transactionPrice',
      factType: 'asc606.transactionPrice',
      value: { amount: 630000, currency: 'USD' },
      statement: 'Total fixed fees under Order Form #OF-2026-011 are $630,000.',
      anchors: [{ evidenceId: 'ev_order_form_1', quote: REAL_QUOTE }],
    });
    expect(outcome.ok).toBe(true);
    expect(reg.draft.facts).toHaveLength(1);
    expect(reg.draft.facts[0].id).toBe('f_1');
  });

  it('accepts a quote that spans lines and uses curly quotes (normalization)', () => {
    const reg = registry();
    const outcome = reg.execute('add_fact', {
      factType: 'asc606.contractParties',
      value: 'Rivermark / Sagebrook',
      anchors: [
        {
          evidenceId: 'ev_msa',
          // Evidence uses straight quotes and wraps this across lines.
          quote: 'each of Rivermark and Customer, a “Party”',
        },
      ],
    });
    expect(outcome.ok).toBe(true);
  });

  it('rejects a fabricated quote with a structured error naming the quote', () => {
    const reg = registry();
    const badQuote = 'The total contract value is $999,999';
    const outcome = reg.execute('add_fact', {
      factType: 'asc606.transactionPrice',
      value: { amount: 999999, currency: 'USD' },
      anchors: [{ evidenceId: 'ev_order_form_1', quote: badQuote }],
    });
    expect(outcome.ok).toBe(false);
    if (!outcome.ok) {
      expect(outcome.error.message).toContain(badQuote);
      expect(outcome.error.message).toContain('ev_order_form_1');
      expect((outcome.error.details as { failedQuote: string }).failedQuote).toBe(badQuote);
    }
    expect(reg.draft.facts).toHaveLength(0);
  });

  it('rejects an unknown evidence id', () => {
    const reg = registry();
    const outcome = reg.execute('add_fact', {
      factType: 'x',
      value: 1,
      anchors: [{ evidenceId: 'ev_nope', quote: REAL_QUOTE }],
    });
    expect(outcome.ok).toBe(false);
    if (!outcome.ok) expect(outcome.error.message).toContain('ev_nope');
  });

  it('re-verifies anchors on update_fact', () => {
    const reg = registry();
    reg.execute('add_fact', {
      factType: 'x',
      value: 1,
      anchors: [{ evidenceId: 'ev_order_form_1', quote: REAL_QUOTE }],
    });
    const outcome = reg.execute('update_fact', {
      id: 'f_1',
      anchors: [{ evidenceId: 'ev_order_form_1', quote: 'not in the document at all' }],
    });
    expect(outcome.ok).toBe(false);
  });
});

describe('reading tools', () => {
  it('read_evidence returns line-numbered text', () => {
    const reg = registry();
    const outcome = reg.execute('read_evidence', { evidenceId: 'ev_order_form_1' });
    expect(outcome.ok).toBe(true);
    if (outcome.ok) {
      const { text } = outcome.result as { text: string };
      expect(text.startsWith('   1│ ')).toBe(true);
      expect(text).toContain('$630,000');
    }
  });

  it('search_evidence finds normalized substring hits with locations', () => {
    const reg = registry();
    const outcome = reg.execute('search_evidence', { query: 'total fixed fees' });
    expect(outcome.ok).toBe(true);
    if (outcome.ok) {
      const { hits } = outcome.result as { hits: Array<{ evidenceId: string; page: number; line: number; snippet: string }> };
      expect(hits.length).toBeGreaterThan(0);
      expect(hits[0].evidenceId).toBe('ev_order_form_1');
      expect(hits[0].page).toBe(1);
      expect(hits[0].line).toBeGreaterThan(0);
      expect(hits[0].snippet).toContain('$630,000');
    }
  });

  it('read_brief returns the packet material', () => {
    const reg = registry();
    const outcome = reg.execute('read_brief', {});
    expect(outcome.ok).toBe(true);
    if (outcome.ok) {
      const r = outcome.result as { brief: string; requiredFacts: unknown[]; methodSteps: unknown[] };
      expect(r.brief).toContain('Rivermark');
      expect(r.requiredFacts).toHaveLength(8);
      expect(r.methodSteps.length).toBeGreaterThan(0);
    }
  });
});

describe('reference checking on assessments', () => {
  it('rejects unknown supporting refs at call time', () => {
    const reg = registry();
    const outcome = reg.execute('add_assessment', {
      statement: 'Conclusion.',
      supportingFacts: ['f_99'],
    });
    expect(outcome.ok).toBe(false);
    if (!outcome.ok) expect(outcome.error.message).toContain('f_99');
  });
});

describe('structured input validation', () => {
  it('returns zod issues as a tool error, not an exception', () => {
    const reg = registry();
    const outcome = reg.execute('add_fact', { value: 1 });
    expect(outcome.ok).toBe(false);
    if (!outcome.ok) expect(outcome.error.message).toContain('factType');
  });

  it('reports unknown tools', () => {
    const reg = registry();
    const outcome = reg.execute('do_magic', {});
    expect(outcome.ok).toBe(false);
  });
});
