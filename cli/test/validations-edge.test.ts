import { describe, expect, it } from 'vitest';
import { verifyPackage, normalizeForQuoteMatch } from '../src/lib/validations';
import { buildPackage, baseFiles, statuses } from './helpers';

function verify(files: Record<string, string | object | Buffer>) {
  const report = verifyPackage(buildPackage(files));
  return { report, byId: statuses(report.outcomes) };
}

describe('core validators, failure and warning paths', () => {
  it('fails when manifest.json is not valid JSON', () => {
    const { report, byId } = verify({ 'manifest.json': '{not json' });
    expect(report.valid).toBe(false);
    expect(byId.get('core.manifest.exists')).toBe('fail');
  });

  it('fails when manifest.json is absent', () => {
    const { byId } = verify({ 'workproduct.json': '{}' });
    expect(byId.get('core.manifest.exists')).toBe('fail');
  });

  it('fails when workproduct.json is missing or schema-invalid', () => {
    const files = baseFiles();
    delete (files as any)['workproduct.json'];
    expect(verify(files).byId.get('core.workproduct.schemaValid')).toBe('fail');
    expect(
      verify(baseFiles({ 'workproduct.json': { title: 'no id' } })).byId.get('core.workproduct.schemaValid')
    ).toBe('fail');
  });

  it('fails when claims.json is unreadable', () => {
    const { byId } = verify(baseFiles({ 'claims/claims.json': 'nope{' }));
    expect(byId.get('core.claims.schemaValid')).toBe('fail');
  });

  it('fails when provenance is schema-invalid', () => {
    const { byId } = verify(baseFiles({ 'provenance/provenance.json': { createdBy: { type: 'martian', name: 'x' } } }));
    expect(byId.get('core.provenance.schemaValid')).toBe('fail');
  });

  it('fails when a manifest-declared part is missing', () => {
    const files = baseFiles();
    (files['manifest.json'] as any).parts.push({ path: 'facts/facts.json', mediaType: 'application/json', role: 'facts' });
    const { byId } = verify(files);
    expect(byId.get('core.partHashes.match')).toBe('fail');
  });

  it('fails on hash mismatch and passes on matching declared hashes', () => {
    const files = baseFiles();
    (files['manifest.json'] as any).parts[0].sha256 = 'a'.repeat(64);
    expect(verify(files).byId.get('core.partHashes.match')).toBe('fail');
  });

  it('fails on dangling references: fact evidence, claim refs, derivedFrom', () => {
    const files = baseFiles({
      'evidence/evidence.json': {
        evidence: [
          { id: 'ev1', type: 'document', title: 'T', path: 'evidence/doc.txt' },
          { id: 'ev_derived', type: 'document', title: 'D', path: 'evidence/doc.txt', derivedFrom: 'ev_missing' },
        ],
      },
      'evidence/doc.txt': 'some text',
      'facts/facts.json': {
        facts: [
          { id: 'f1', statement: 'S', evidenceRefs: ['ev_unknown'] },
          { id: 'f2', statement: 'S', evidenceRefs: [{ evidenceId: 'ev_also_unknown', anchors: [{ page: 1 }] }] },
        ],
      },
      'claims/claims.json': {
        claims: [
          {
            id: 'c1',
            statement: 'S',
            status: 'supported',
            supportingEvidence: ['ev_unknown'],
            supportingFacts: ['f_unknown'],
            judgments: ['j_unknown'],
            supportingCalculations: ['calc_unknown'],
          },
        ],
      },
    });
    const { byId } = verify(files);
    expect(byId.get('core.evidenceRefs.resolve')).toBe('fail');
    expect(byId.get('core.claimRefs.resolve')).toBe('fail');
  });

  it('fails when evidence path is missing or its hash mismatches', () => {
    const files = baseFiles({
      'evidence/evidence.json': {
        evidence: [
          { id: 'ev_gone', type: 'document', title: 'T', path: 'evidence/gone.txt' },
          { id: 'ev_bad_hash', type: 'document', title: 'T', path: 'evidence/here.txt', sha256: 'b'.repeat(64) },
        ],
      },
      'evidence/here.txt': 'content',
    });
    expect(verify(files).byId.get('core.evidenceRefs.resolve')).toBe('fail');
  });

  it('fails on malformed and schema-invalid attestations', () => {
    expect(verify(baseFiles({ 'attestations/att-x.json': '{broken' })).byId.get('core.attestations.schemaValid')).toBe(
      'fail'
    );
    expect(
      verify(baseFiles({ 'attestations/att-x.json': { id: 'att-x' } })).byId.get('core.attestations.schemaValid')
    ).toBe('fail');
  });
});

describe('quote anchors against unverifiable evidence', () => {
  it('warns when quotes point at binary or external evidence', () => {
    const files = baseFiles({
      'evidence/evidence.json': {
        evidence: [
          { id: 'ev_pdf', type: 'document', title: 'Binary', path: 'evidence/doc.pdf' },
          { id: 'ev_ext', type: 'document', title: 'External', externalRef: { system: 'S', locator: 'x' } },
        ],
      },
      'evidence/doc.pdf': Buffer.from('%PDF-1.4 fake'),
      'facts/facts.json': {
        facts: [
          { id: 'f1', statement: 'S', evidenceRefs: [{ evidenceId: 'ev_pdf', anchors: [{ quote: 'anything' }] }] },
          { id: 'f2', statement: 'S', evidenceRefs: [{ evidenceId: 'ev_ext', anchors: [{ quote: 'anything' }] }] },
        ],
      },
    });
    const { byId } = verify(files);
    expect(byId.get('core.factAnchors.quotesMatch')).toBe('warning');
  });

  it('reports notRun when facts exist but declare no quote anchors', () => {
    const files = baseFiles({
      'evidence/evidence.json': { evidence: [{ id: 'ev1', type: 'document', title: 'T', path: 'evidence/a.txt' }] },
      'evidence/a.txt': 'text',
      'facts/facts.json': { facts: [{ id: 'f1', statement: 'S', evidenceRefs: ['ev1'] }] },
    });
    expect(verify(files).byId.get('core.factAnchors.quotesMatch')).toBe('notRun');
  });

  it('normalizes curly quotes, dashes, and whitespace before matching', () => {
    expect(normalizeForQuoteMatch('“smart” — quotes  and\n spaces')).toBe('"smart" - quotes and spaces');
  });
});

describe('finance profile warnings', () => {
  it('warns on claims without assertions or support', () => {
    const files = baseFiles();
    (files['manifest.json'] as any).profile = 'vwp-finance';
    (files['workproduct.json'] as any).profile = 'vwp-finance';
    files['claims/claims.json'] = { claims: [{ id: 'c1', statement: 'Unsupported claim.', status: 'proposed' }] };
    const { byId } = verify(files);
    expect(byId.get('finance.claims.haveAssertions')).toBe('warning');
    expect(byId.get('finance.claims.haveSupport')).toBe('warning');
  });
});

function asc606Files(overrides: Record<string, string | object | Buffer> = {}) {
  const files = baseFiles(overrides);
  (files['manifest.json'] as any).extension = 'vwp-accounting-asc606';
  return files;
}

const VALID_ALLOCATION = {
  id: 'calc_ssp_allocation',
  type: 'relativeStandaloneSellingPriceAllocation',
  inputs: {
    transactionPrice: 100,
    components: [{ id: 'a', standaloneSellingPrice: 125 }],
  },
  outputs: { allocations: [{ componentId: 'a', allocatedAmount: 100 }] },
};

describe('asc606 extension validators', () => {
  it('reports notRun when the allocation calculation is absent', () => {
    const { byId } = verify(asc606Files());
    expect(byId.get('asc606.allocation.schemaValid')).toBe('notRun');
    expect(byId.get('asc606.allocation.tieOut')).toBe('notRun');
    expect(byId.get('asc606.revenueSchedule.total')).toBe('notRun');
  });

  it('fails schema check and skips tie-out on an invalid allocation', () => {
    const { byId } = verify(asc606Files({ 'calculations/ssp-allocation.json': { id: 'x' } }));
    expect(byId.get('asc606.allocation.schemaValid')).toBe('fail');
    expect(byId.get('asc606.allocation.tieOut')).toBe('notRun');
  });

  it('fails tie-out when allocations do not sum to the transaction price', () => {
    const bad = JSON.parse(JSON.stringify(VALID_ALLOCATION));
    bad.outputs.allocations[0].allocatedAmount = 90;
    const { byId } = verify(asc606Files({ 'calculations/ssp-allocation.json': bad }));
    expect(byId.get('asc606.allocation.tieOut')).toBe('fail');
  });

  it('checks the revenue schedule: missing total column and total mismatch fail', () => {
    const noTotalCol = verify(
      asc606Files({
        'calculations/ssp-allocation.json': VALID_ALLOCATION,
        'calculations/revenue-schedule.csv': 'month,amount\n2026-01,100\n',
      })
    );
    expect(noTotalCol.byId.get('asc606.revenueSchedule.total')).toBe('fail');

    const mismatch = verify(
      asc606Files({
        'calculations/ssp-allocation.json': VALID_ALLOCATION,
        'calculations/revenue-schedule.csv': 'month,total\n2026-01,42\n',
      })
    );
    expect(mismatch.byId.get('asc606.revenueSchedule.total')).toBe('fail');

    const ties = verify(
      asc606Files({
        'calculations/ssp-allocation.json': VALID_ALLOCATION,
        'calculations/revenue-schedule.csv': 'month,total\n2026-01,60\n2026-02,40\n',
      })
    );
    expect(ties.byId.get('asc606.revenueSchedule.total')).toBe('pass');
  });
});

describe('wallet extension validators', () => {
  function walletFiles(register: object | undefined) {
    const files = baseFiles(register ? { 'facts/wallets.json': register } : {});
    (files['manifest.json'] as any).extension = 'vwp-digital-assets-wallet-ownership-control';
    return files;
  }

  it('fails when the register is absent', () => {
    expect(verify(walletFiles(undefined)).byId.get('digitalAssets.wallets.registerPresent')).toBe('fail');
  });

  it('fails schema check on an invalid register', () => {
    expect(
      verify(walletFiles({ wallets: [{ id: 'w1' }] })).byId.get('digitalAssets.wallets.schemaValid')
    ).toBe('fail');
  });

  it('fails each wallet completeness check independently', () => {
    const { byId } = verify(
      walletFiles({
        wallets: [
          { id: 'w_no_mapping', custody: 'self', included: true, proofOfControlEvidence: [] },
          { id: 'w_excluded_no_rationale', custody: 'custodian', included: false },
          {
            id: 'w_no_balance_evidence',
            custody: 'custodian',
            included: true,
            entityMappingEvidence: ['ev1'],
            periodEndBalance: { asset: 'BTC', amount: 1, asOf: '2026-06-30' },
          },
        ],
      })
    );
    // ev1 does not resolve because the package declares no evidence at all.
    expect(byId.get('digitalAssets.wallets.entityMapping')).toBe('fail');
    expect(byId.get('digitalAssets.wallets.proofOfControl')).toBe('fail');
    expect(byId.get('digitalAssets.wallets.exclusionRationale')).toBe('fail');
    expect(byId.get('digitalAssets.wallets.balanceEvidence')).toBe('fail');
  });
});

describe('asc820 extension validators', () => {
  function asc820Files(register: object | undefined, extra: Record<string, string | object | Buffer> = {}) {
    const files = baseFiles({ ...(register ? { 'facts/fair-value-measurements.json': register } : {}), ...extra });
    (files['manifest.json'] as any).extension = 'vwp-accounting-asc820';
    return files;
  }

  it('fails when the measurement register is absent', () => {
    expect(verify(asc820Files(undefined)).byId.get('asc820.measurements.present')).toBe('fail');
  });

  it('fails schema check on an invalid register', () => {
    expect(verify(asc820Files({ measurements: [{ id: 'm1' }] })).byId.get('asc820.measurements.schemaValid')).toBe(
      'fail'
    );
  });

  it('fails on unresolvable pricing sources and unjudged level3 measurements', () => {
    const { byId } = verify(
      asc820Files({
        measurements: [
          {
            id: 'm_l3',
            asset: 'LCK',
            measurementDate: '2026-06-30',
            level: 'level3',
            fairValue: { amount: 1000, currency: 'USD' },
            pricingSourceEvidence: ['ev_nowhere'],
          },
        ],
      })
    );
    expect(byId.get('asc820.pricingSources.resolve')).toBe('fail');
    expect(byId.get('asc820.level3.judgmentLinked')).toBe('fail');
  });
});
