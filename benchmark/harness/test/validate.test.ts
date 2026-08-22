import { describe, expect, it } from 'vitest';
import { openPackageBuffer, verifyPackage } from '@vwp/cli/lib';
import type { Draft, Fact } from '../src/draft.js';
import { emptyDraft } from '../src/draft.js';
import { assembleToBuffer } from '../src/assemble.js';
import { createToolRegistry, type ToolRegistry } from '../src/tools.js';
import { runValidation, valueShapeProblem, type CheckId } from '../src/validate.js';
import { fixturePacket, TEST_MODEL } from './helpers.js';

const packet = fixturePacket();

function failures(draft: Draft, check: CheckId): string[] {
  const result = runValidation(draft, packet, { modelIdentity: TEST_MODEL });
  return result.findings.filter((f) => f.check === check && f.status === 'fail').map((f) => f.message);
}

function mkFact(partial: Partial<Fact> & Pick<Fact, 'id' | 'factType'>): Fact {
  return {
    statement: `${partial.factType} observation`,
    value: partial.value,
    evidenceRefs: [
      {
        evidenceId: 'ev_order_form_1',
        anchors: [{ quote: 'TOTAL FIXED FEES UNDER THIS ORDER FORM (36 months): $630,000' }],
      },
    ],
    ...partial,
  };
}

/** Build the full, clean draft through the tool registry (all 8 slots + graph). */
function buildCleanRegistry(): ToolRegistry {
  const reg = createToolRegistry(packet, { modelIdentity: TEST_MODEL });
  const call = (name: string, input: unknown) => {
    const outcome = reg.execute(name, input);
    if (!outcome.ok) throw new Error(`${name} failed: ${outcome.error.message}`);
    return outcome.result;
  };

  call('add_fact', {
    slot: 'transactionPrice',
    factType: 'asc606.transactionPrice',
    value: { amount: 630000, currency: 'USD' },
    statement: 'Total fixed fees under Order Form #OF-2026-011 (36 months) are $630,000.',
    anchors: [
      { evidenceId: 'ev_order_form_1', quote: 'TOTAL FIXED FEES UNDER THIS ORDER FORM (36 months): $630,000' },
    ],
  }); // f_1
  call('add_fact', {
    slot: 'contractTermMonths',
    factType: 'asc606.contractTerm',
    value: { value: 12, unit: 'months' },
    statement:
      'The customer may terminate for convenience on or after the first anniversary for a $10,000 administrative fee, limiting the enforceable term to 12 months.',
    anchors: [
      {
        evidenceId: 'ev_msa',
        quote:
          'Customer may, at its option, terminate this Agreement and any Order Form effective on any date on or after the first anniversary of the first subscription start date',
      },
      { evidenceId: 'ev_msa', quote: 'an administrative termination fee of ten thousand dollars ($10,000)' },
    ],
  }); // f_2
  call('add_fact', {
    slot: 'sspSubscription',
    factType: 'asc606.ssp.subscription',
    value: { amount: 180000, currency: 'USD' },
    statement: 'Concluded SSP for the annual platform subscription is $180,000 per subscription year.',
    anchors: [{ evidenceId: 'ev_ssp_study', quote: 'Concluded SSP: $180,000 per subscription year.' }],
  }); // f_3
  call('add_fact', {
    slot: 'sspImplementation',
    factType: 'asc606.ssp.implementation',
    value: { amount: 120000, currency: 'USD' },
    statement: 'Concluded SSP for implementation services is $120,000 (point estimate).',
    anchors: [{ evidenceId: 'ev_ssp_study', quote: 'Concluded SSP: $120,000 (point estimate).' }],
  }); // f_4
  call('add_fact', {
    slot: 'implementationDistinct',
    factType: 'asc606.performanceObligationConclusion',
    value: 'distinct',
    statement: 'Implementation services are a distinct performance obligation.',
    anchors: [
      {
        evidenceId: 'ev_order_form_1',
        quote: 'One-time Implementation Services fee of $90,000 for standard implementation of the Rivermark FA Platform',
      },
    ],
  }); // f_5
  call('add_fact', {
    slot: 'modificationTreatment',
    factType: 'asc606.modificationTreatment',
    value: 'separateContract',
    statement: 'The April analytics upsell is accounted for as a separate contract.',
    anchors: [
      {
        evidenceId: 'ev_order_form_2',
        quote:
          'Customer adds the Insight Analytics Module, a standard add-on module to the Rivermark FA Platform, at a fee of $4,000 per month',
      },
    ],
  }); // f_6
  call('add_fact', {
    slot: 'q1RevenueTotal',
    factType: 'asc606.periodRevenue',
    value: { amount: 66500, currency: 'USD' },
    statement: 'Q1 2026 revenue for the Sagebrook arrangement (illustrative test value).',
    anchors: [{ evidenceId: 'ev_invoice_1001', quote: 'TOTAL DUE 180,000.00' }],
  }); // f_7
  call('add_fact', {
    slot: 'q1DeferredRevenueBalance',
    factType: 'asc606.deferredRevenueBalance',
    value: { amount: 208500, currency: 'USD' },
    statement: 'Deferred revenue balance at March 31, 2026 (illustrative test value).',
    anchors: [{ evidenceId: 'ev_remittance', quote: 'TOTAL PAYMENT 275,000.00' }],
  }); // f_8

  call('add_judgment', {
    topic: 'Enforceable contract term',
    question: 'Does the elective termination clause limit the enforceable contract term?',
    proposedConclusion: 'The enforceable term is 12 months; the $10,000 fee is not a substantive termination penalty.',
    basis: 'ASC 606-10-25-3: rights and obligations exist only for the period in which the parties have enforceable rights.',
    alternatives: [
      { conclusion: '36-month term', notes: 'Set aside: the administrative fee is not substantive relative to contract value.' },
    ],
    standardRefs: ['ASC 606-10-25-3'],
    riskLevel: 'high',
  }); // j_1

  call('add_calculation', {
    type: 'relativeStandaloneSellingPriceAllocation',
    inputs: {
      transactionPrice: 270000,
      components: [
        { id: 'subscription_y1', standaloneSellingPrice: 180000 },
        { id: 'implementation', standaloneSellingPrice: 120000 },
      ],
    },
    outputs: {
      allocations: [
        { componentId: 'subscription_y1', allocatedAmount: 162000 },
        { componentId: 'implementation', allocatedAmount: 108000 },
      ],
    },
    method: 'allocatedAmount_i = transactionPrice * ssp_i / sum(ssp) per ASC 606-10-32-31',
  }); // calc_1

  call('add_assessment', {
    stepKey: 'step4_allocation',
    statement: 'The year-1 fixed consideration of $270,000 is allocated on a relative SSP basis.',
    standardRefs: ['ASC 606-10-32-28', 'ASC 606-10-32-31'],
    assertions: ['valuationAllocation'],
    supportingFacts: ['f_3', 'f_4'],
    judgments: ['j_1'],
    supportingCalculations: ['calc_1'],
  }); // c_1

  call('write_memo', { markdown: '# ASC 606 memo (test)\n\nAbbreviated memo used by the validation test.\n' });
  return reg;
}

describe('P1 — value shape conformance', () => {
  const moneySlot = packet.requiredFacts.find((s) => s.slot === 'transactionPrice')!;
  const quantitySlot = packet.requiredFacts.find((s) => s.slot === 'contractTermMonths')!;
  const enumSlot = packet.requiredFacts.find((s) => s.slot === 'modificationTreatment')!;

  it('accepts conforming shapes', () => {
    expect(valueShapeProblem(moneySlot, { amount: 630000, currency: 'USD' })).toBeUndefined();
    expect(valueShapeProblem(moneySlot, { amount: '630000.00', currency: 'USD' })).toBeUndefined();
    expect(valueShapeProblem(quantitySlot, { value: 12, unit: 'months' })).toBeUndefined();
    expect(valueShapeProblem(enumSlot, 'separateContract')).toBeUndefined();
  });

  it('rejects non-conforming shapes', () => {
    expect(valueShapeProblem(moneySlot, 630000)).toBeDefined();
    expect(valueShapeProblem(moneySlot, { amount: 630000 })).toBeDefined();
    expect(valueShapeProblem(quantitySlot, { value: 12, unit: 'years' })).toBeDefined();
    expect(valueShapeProblem(enumSlot, 'somethingElse')).toBeDefined();
    expect(valueShapeProblem({ slot: 'd', factType: 't', valueShape: 'date' }, '03/31/2026')).toBeDefined();
    expect(valueShapeProblem({ slot: 'd', factType: 't', valueShape: 'date' }, '2026-03-31')).toBeUndefined();
    expect(valueShapeProblem({ slot: 'p', factType: 't', valueShape: 'period' }, { from: '2026-01-01', to: '2026-03-31' })).toBeUndefined();
    expect(valueShapeProblem({ slot: 'b', factType: 't', valueShape: 'boolean' }, true)).toBeUndefined();
    expect(valueShapeProblem({ slot: 'b', factType: 't', valueShape: 'boolean' }, 'true')).toBeDefined();
  });

  it('fails P1 for unfilled slots and wrong shapes in the battery', () => {
    const draft = emptyDraft();
    draft.facts.push(mkFact({ id: 'f_1', factType: 'asc606.transactionPrice', value: { amount: 630000 } }));
    const p1 = failures(draft, 'P1');
    // transactionPrice filled but malformed; the other 7 slots unfilled.
    expect(p1.some((m) => m.includes("slot 'transactionPrice'") && m.includes('money'))).toBe(true);
    expect(p1.some((m) => m.includes("slot 'contractTermMonths' unfilled"))).toBe(true);
  });
});

describe('S2 — duplicate factType conflict detection', () => {
  it('fails when the same factType carries conflicting values', () => {
    const draft = emptyDraft();
    draft.facts.push(mkFact({ id: 'f_1', factType: 'asc606.transactionPrice', value: { amount: 630000, currency: 'USD' } }));
    draft.facts.push(mkFact({ id: 'f_2', factType: 'asc606.transactionPrice', value: { amount: 635000, currency: 'USD' } }));
    const s2 = failures(draft, 'S2');
    expect(s2).toHaveLength(1);
    expect(s2[0]).toContain('asc606.transactionPrice');
  });

  it('treats numerically equal representations as consistent', () => {
    const draft = emptyDraft();
    draft.facts.push(mkFact({ id: 'f_1', factType: 'asc606.transactionPrice', value: { amount: 630000, currency: 'USD' } }));
    draft.facts.push(mkFact({ id: 'f_2', factType: 'asc606.transactionPrice', value: { currency: 'USD', amount: '630000.00' } }));
    expect(failures(draft, 'S2')).toEqual([]);
  });
});

describe('C1 — claim support', () => {
  it('detects orphan claims', () => {
    const draft = emptyDraft();
    draft.claims.push({
      id: 'c_1',
      statement: 'A conclusion with no support.',
      standardRefs: [],
      assertions: [],
      supportingFacts: [],
      supportingEvidence: [],
      judgments: [],
      supportingCalculations: [],
    });
    const c1 = failures(draft, 'C1');
    expect(c1.some((m) => m.includes('orphan'))).toBe(true);
  });

  it('detects dangling refs', () => {
    const draft = emptyDraft();
    draft.claims.push({
      id: 'c_1',
      statement: 'A conclusion.',
      standardRefs: [],
      assertions: [],
      supportingFacts: ['f_404'],
      supportingEvidence: [],
      judgments: ['j_404'],
      supportingCalculations: [],
    });
    const c1 = failures(draft, 'C1');
    expect(c1.some((m) => m.includes('f_404'))).toBe(true);
    expect(c1.some((m) => m.includes('j_404'))).toBe(true);
  });
});

describe('G1/G2 — evidence resolution and anchors', () => {
  it('fails G1 on unknown evidence and ref-less facts', () => {
    const draft = emptyDraft();
    draft.facts.push({ id: 'f_1', factType: 'x', value: 1, statement: 's', evidenceRefs: [] });
    draft.facts.push(mkFact({ id: 'f_2', factType: 'y', value: 2, evidenceRefs: [{ evidenceId: 'ev_nope', anchors: [{ quote: 'q' }] }] }));
    const g1 = failures(draft, 'G1');
    expect(g1.some((m) => m.includes('f_1'))).toBe(true);
    expect(g1.some((m) => m.includes('ev_nope'))).toBe(true);
  });

  it('fails G2 when a hand-written quote does not verify', () => {
    const draft = emptyDraft();
    draft.facts.push(
      mkFact({
        id: 'f_1',
        factType: 'x',
        value: 1,
        evidenceRefs: [{ evidenceId: 'ev_order_form_1', anchors: [{ quote: 'this text is nowhere in the order form' }] }],
      })
    );
    const g2 = failures(draft, 'G2');
    expect(g2.some((m) => m.includes('quote not found'))).toBe(true);
  });
});

describe('full battery on a complete draft', () => {
  it('comes back clean, and the assembled package verifies with the reference CLI', () => {
    const reg = buildCleanRegistry();

    // The validate() tool itself reports clean.
    const outcome = reg.execute('validate', {});
    expect(outcome.ok).toBe(true);
    if (outcome.ok) {
      const r = outcome.result as { ok: boolean; findings: Array<{ check: string; status: string; message: string }> };
      expect(r.findings.filter((f) => f.status === 'fail')).toEqual([]);
      expect(r.ok).toBe(true);
    }

    // And the battery result agrees.
    const result = runValidation(reg.draft, packet, { modelIdentity: TEST_MODEL });
    expect(result.findings.filter((f) => f.status === 'fail')).toEqual([]);
    expect(result.ok).toBe(true);
    expect(result.verify.valid).toBe(true);

    // Belt and braces: verify the assembled artifact independently.
    const artifact = assembleToBuffer(reg.draft, packet, { modelIdentity: TEST_MODEL });
    const report = verifyPackage(openPackageBuffer(artifact));
    expect(report.valid).toBe(true);
    expect(report.outcomes.find((o) => o.id === 'core.factAnchors.quotesMatch')?.status).toBe('pass');
    expect(report.outcomes.find((o) => o.id === 'core.claimRefs.resolve')?.status).toBe('pass');
  });
});
