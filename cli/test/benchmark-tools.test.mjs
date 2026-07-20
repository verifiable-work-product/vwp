import { describe, expect, it } from 'vitest';
import { evidenceLines, renderNumberedCopy, resolvePinCite, LINES_PER_PAGE } from '../../benchmark/tools/lib/numbering.mjs';
import { parseCsv, slotValue, transcribeWorksheet } from '../../benchmark/tools/lib/worksheet.mjs';
import { comparable, matches, scoreKeyedItems } from '../../benchmark/tools/lib/scoring.mjs';

describe('numbering', () => {
  const doc = Array.from({ length: LINES_PER_PAGE + 3 }, (_, i) => `line ${i + 1}`).join('\n') + '\n';

  it('splits citable lines, dropping only the trailing newline artifact', () => {
    expect(evidenceLines('a\nb\n')).toEqual(['a', 'b']);
    expect(evidenceLines('a\r\nb')).toEqual(['a', 'b']);
    expect(evidenceLines('')).toEqual([]);
  });

  it('renders page headers and page-relative margin numbers', () => {
    const numbered = renderNumberedCopy(doc, 'DOC');
    expect(numbered).toContain('==== DOC — PAGE 1 ====');
    expect(numbered).toContain('==== DOC — PAGE 2 ====');
    expect(numbered).toContain(`  1 | line ${LINES_PER_PAGE + 1}`); // page 2 restarts at 1
  });

  it('resolves pin cites page-relatively and rejects out-of-document cites', () => {
    expect(resolvePinCite(doc, 1, 2, 3)).toBe('line 2\nline 3');
    expect(resolvePinCite(doc, 2, 1, undefined)).toBe(`line ${LINES_PER_PAGE + 1}`);
    expect(resolvePinCite(doc, 3, 1, undefined)).toBeUndefined(); // past the end
    expect(resolvePinCite(doc, 1, 5, 4)).toBeUndefined(); // inverted range
  });
});

describe('worksheet parsing', () => {
  it('parses quoted fields, escaped quotes, and CRLF', () => {
    const rows = parseCsv('a,b\r\n"1,5","say ""hi"""\n');
    expect(rows).toEqual([
      ['a', 'b'],
      ['1,5', 'say "hi"'],
    ]);
  });

  it('converts slot values by declared shape and rejects garbage', () => {
    expect(slotValue({ valueShape: 'money', slot: 's' }, '$1,250.50', 'USD', 2)).toEqual({ amount: 1250.5, currency: 'USD' });
    expect(slotValue({ valueShape: 'money', slot: 's' }, '100', '', 2)).toEqual({ amount: 100, currency: 'USD' });
    expect(slotValue({ valueShape: 'quantity', slot: 's', unit: 'months' }, '24', '', 2)).toEqual({ value: 24, unit: 'months' });
    expect(slotValue({ valueShape: 'boolean', slot: 's' }, 'TRUE', '', 2)).toBe(true);
    expect(slotValue({ valueShape: 'string', slot: 's' }, 'text', '', 2)).toBe('text');
    expect(() => slotValue({ valueShape: 'money', slot: 's' }, 'abc', '', 2)).toThrow(/not a number/);
    expect(() => slotValue({ valueShape: 'quantity', slot: 's' }, 'abc', '', 2)).toThrow(/not a number/);
    expect(() => slotValue({ valueShape: 'enum', slot: 's', enumValues: ['a', 'b'] }, 'c', '', 2)).toThrow(/not one of/);
  });
});

const REQUIRED = {
  challengeId: 'test-challenge',
  slots: [
    { slot: 'price', factType: 't.price', valueShape: 'money', description: 'd' },
    { slot: 'term', factType: 't.term', valueShape: 'quantity', unit: 'months', description: 'd' },
    { slot: 'conclusion', factType: 't.conclusion', valueShape: 'enum', enumValues: ['yes', 'no'], description: 'd' },
  ],
};

const HEADER = 'slot,statement,value,currency_or_unit,evidence_id,page,lines,cells,section,verbatim_quote,notes';

describe('transcribeWorksheet', () => {
  const evidence = new Map([['ev1', Array.from({ length: 60 }, (_, i) => `evidence line ${i + 1}`).join('\n')]]);

  it('produces anchored, typed facts and reports missing slots', () => {
    const csv = [HEADER, 'price,"The price.",100,USD,ev1,1,3,,2(a),,note here'].join('\n');
    const { facts, missingSlots } = transcribeWorksheet(csv, REQUIRED, evidence);
    expect(facts).toHaveLength(1);
    const fact = facts[0];
    expect(fact.id).toBe('f_price');
    expect(fact.factType).toBe('t.price');
    expect(fact.value).toEqual({ amount: 100, currency: 'USD' });
    // pin cite resolved into a machine-verifiable quote
    expect(fact.evidenceRefs[0].anchors[0].quote).toBe('evidence line 3');
    expect(fact.evidenceRefs[0].anchors[0].section).toBe('2(a)');
    expect(fact.notes).toBe('note here');
    expect(missingSlots).toEqual(['term', 'conclusion']);
  });

  it('keeps an explicit verbatim_quote over the resolved cite and supports bare refs', () => {
    const csv = [HEADER, 'price,"P.",100,USD,ev1,1,3,,,"my exact quote",', 'term,"T.",24,months,ev1,,,,,,'].join('\n');
    const { facts } = transcribeWorksheet(csv, REQUIRED, evidence);
    expect(facts[0].evidenceRefs[0].anchors[0].quote).toBe('my exact quote');
    expect(facts[1].evidenceRefs[0]).toBe('ev1'); // no anchor columns -> bare ref
  });

  it('parses line ranges and page 2 pin cites', () => {
    const csv = [HEADER, 'price,"P.",100,USD,ev1,2,1-2,,,,'].join('\n');
    const { facts } = transcribeWorksheet(csv, REQUIRED, evidence);
    expect(facts[0].evidenceRefs[0].anchors[0].lines).toEqual({ from: 1, to: 2 });
    expect(facts[0].evidenceRefs[0].anchors[0].quote).toBe('evidence line 51\nevidence line 52');
  });

  it('rejects structural problems loudly', () => {
    expect(() => transcribeWorksheet('slot,statement\nprice,x', REQUIRED)).toThrow(/no 'value' column/);
    expect(() => transcribeWorksheet([HEADER, 'bogus,"S.",1,,ev1,,,,,,'].join('\n'), REQUIRED)).toThrow(/unknown slot/);
    expect(() =>
      transcribeWorksheet([HEADER, 'price,"S.",1,,ev1,,,,,,', 'price,"S.",2,,ev1,,,,,,'].join('\n'), REQUIRED)
    ).toThrow(/duplicate slot/);
    expect(() => transcribeWorksheet([HEADER, 'price,"S.",1,,ev1,1,x-y,,,,'].join('\n'), REQUIRED)).toThrow(/not N or N-M/);
    expect(() => transcribeWorksheet([HEADER, 'price,"S.",1,,ev1,9,1,,,,'].join('\n'), REQUIRED, evidence)).toThrow(
      /outside ev1/
    );
  });
});

describe('scoring', () => {
  const KEY = {
    challengeId: 'test-challenge',
    keyedItems: [
      { slot: 'price', expected: { amount: 100, currency: 'USD' }, tolerance: 1 },
      {
        slot: 'conclusion',
        expected: 'yes',
        alternatives: [{ expected: 'no', basis: 'documented alternative', requiredDocumentation: 'explain' }],
      },
      { slot: 'nonexistent', expected: 1 },
    ],
  };

  const fact = (factType, value, id = `f_${factType}`) => ({ id, statement: 's', factType, value, evidenceRefs: ['ev1'] });

  it('scores correct, alternative, missing, ambiguous, and keyError outcomes', () => {
    const report = scoreKeyedItems(
      [fact('t.price', { amount: 100.5, currency: 'USD' }), fact('t.conclusion', 'no')],
      KEY,
      REQUIRED
    );
    const bySlot = new Map(report.results.map((r) => [r.slot, r]));
    expect(bySlot.get('price').result).toBe('correct'); // within tolerance
    expect(bySlot.get('conclusion').result).toBe('acceptableAlternative');
    expect(bySlot.get('conclusion').basis).toBe('documented alternative');
    expect(bySlot.get('nonexistent').result).toBe('keyError');
    expect(report.tally.correct).toBe(1);

    const ambiguous = scoreKeyedItems(
      [fact('t.price', { amount: 100 }, 'f1'), fact('t.price', { amount: 100 }, 'f2')],
      { challengeId: 'test-challenge', keyedItems: [KEY.keyedItems[0]] },
      REQUIRED
    );
    expect(ambiguous.results[0].result).toBe('ambiguous');

    const missing = scoreKeyedItems([], { challengeId: 'test-challenge', keyedItems: [KEY.keyedItems[0]] }, REQUIRED);
    expect(missing.results[0].result).toBe('missing');
  });

  it('marks incorrect values and currency mismatches', () => {
    const wrong = scoreKeyedItems(
      [fact('t.price', { amount: 250, currency: 'USD' })],
      { challengeId: 'test-challenge', keyedItems: [KEY.keyedItems[0]] },
      REQUIRED
    );
    expect(wrong.results[0].result).toBe('incorrect');

    expect(matches('money', { amount: 100, currency: 'EUR' }, { amount: 100, currency: 'USD' }, 1)).toBe(false);
    expect(matches('money', { amount: 100 }, { amount: 100, currency: 'USD' }, 1)).toBe(true); // undeclared unit passes
    expect(matches('quantity', { value: 24, unit: 'months' }, { value: 24, unit: 'months' })).toBe(true);
    expect(matches('string', 'a', 'a')).toBe(true);
    expect(matches('money', undefined, { amount: 1 })).toBe(false);
    expect(comparable('money', 5)).toBe(5);
    expect(comparable('quantity', { value: 3 })).toBe(3);
  });

  it('throws on challengeId mismatch', () => {
    expect(() => scoreKeyedItems([], { challengeId: 'other', keyedItems: [] }, REQUIRED)).toThrow(/challengeId mismatch/);
  });
});
