import { describe, expect, it } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { openPackage } from '../src/lib/package';
import { verifyPackage } from '../src/lib/validations';
import { schemaErrors } from '../src/lib/schema';

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const EXAMPLES = path.join(REPO_ROOT, 'examples');
const TESTDATA = path.join(REPO_ROOT, 'testdata');

const SHA256_A = 'a'.repeat(64);
const SHA256_B = 'b'.repeat(64);

function validAttestation(): any {
  return {
    id: 'att-001',
    createdAt: '2026-07-08T09:30:00Z',
    performedBy: { type: 'human', name: 'demo-reviewer' },
    model: {
      provider: 'demo-challenger',
      name: 'challenger-model-mini',
      version: '2026-06-01',
      role: 'challenger',
    },
    scope: {
      evidenceExamined: [{ evidenceId: 'ev-001', sha256: SHA256_A }],
      claimsChallenged: ['c-001'],
      procedure: 'Read the cited evidence and challenge the claim.',
    },
    findings: [
      {
        id: 'find-001',
        severity: 'medium',
        summary: 'A clause may contradict the claim.',
        relatedClaims: ['c-001'],
        relatedEvidence: ['ev-001'],
        requiresDisposition: true,
      },
    ],
    limitations: ['Only the scoped evidence was examined.'],
    dispositions: [
      {
        findingId: 'find-001',
        decision: 'acknowledged',
        by: { name: 'demo-manager' },
        at: '2026-07-08T16:00:00Z',
        note: 'Routed to judgment review.',
      },
    ],
    notes: 'Synthetic fixture.',
  };
}

describe('analysis-attestation schema', () => {
  it('accepts a complete attestation', () => {
    expect(schemaErrors('analysis-attestation', validAttestation())).toEqual([]);
  });

  it('accepts an attestation without findings, limitations, or dispositions', () => {
    const att = validAttestation();
    delete att.findings;
    delete att.limitations;
    delete att.dispositions;
    delete att.notes;
    expect(schemaErrors('analysis-attestation', att)).toEqual([]);
  });

  it('requires the exact model triple', () => {
    const att = validAttestation();
    delete att.model;
    expect(schemaErrors('analysis-attestation', att)).not.toEqual([]);

    const att2 = validAttestation();
    delete att2.model.version;
    expect(schemaErrors('analysis-attestation', att2)).not.toEqual([]);
  });

  it('requires a scope with examined evidence hashes', () => {
    const att = validAttestation();
    delete att.scope;
    expect(schemaErrors('analysis-attestation', att)).not.toEqual([]);

    const att2 = validAttestation();
    delete att2.scope.evidenceExamined[0].sha256;
    expect(schemaErrors('analysis-attestation', att2)).not.toEqual([]);

    const att3 = validAttestation();
    att3.scope.evidenceExamined[0].sha256 = 'not-a-hash';
    expect(schemaErrors('analysis-attestation', att3)).not.toEqual([]);
  });

  it('rejects findings that do not require disposition', () => {
    const att = validAttestation();
    att.findings[0].requiresDisposition = false;
    expect(schemaErrors('analysis-attestation', att)).not.toEqual([]);
  });

  it('rejects unknown severities and disposition decisions', () => {
    const att = validAttestation();
    att.findings[0].severity = 'catastrophic';
    expect(schemaErrors('analysis-attestation', att)).not.toEqual([]);

    const att2 = validAttestation();
    att2.dispositions[0].decision = 'ignored';
    expect(schemaErrors('analysis-attestation', att2)).not.toEqual([]);
  });
});

describe('provenance toolchain sessionDigest', () => {
  function provenanceWithDigest(digest: any): any {
    return {
      createdAt: '2026-07-08T09:00:00Z',
      createdBy: { type: 'agent', name: 'demo-agent' },
      toolchain: [{ name: 'demo-agent-harness', version: '1.0.0', sessionDigest: digest }],
    };
  }

  it('accepts a toolchain entry with a session digest', () => {
    const errs = schemaErrors(
      'provenance',
      provenanceWithDigest({
        system: 'demo-agent-harness',
        sessionId: 'sess-001',
        transcriptSha256: SHA256_B,
        eventCount: 42,
      })
    );
    expect(errs).toEqual([]);
  });

  it('rejects a malformed transcript hash', () => {
    const errs = schemaErrors(
      'provenance',
      provenanceWithDigest({
        system: 'demo-agent-harness',
        sessionId: 'sess-001',
        transcriptSha256: 'XYZ',
        eventCount: 42,
      })
    );
    expect(errs).not.toEqual([]);
  });

  it('rejects a digest missing its session id', () => {
    const errs = schemaErrors(
      'provenance',
      provenanceWithDigest({
        system: 'demo-agent-harness',
        transcriptSha256: SHA256_B,
        eventCount: 42,
      })
    );
    expect(errs).not.toEqual([]);
  });
});

describe('fact evidence-ref anchors[]', () => {
  function factWithAnchors(anchors: any[]): any {
    return {
      id: 'f-001',
      statement: 'An observation.',
      evidenceRefs: [{ evidenceId: 'ev-001', anchors }],
    };
  }

  it('accepts every anchor field, composed per chunk', () => {
    const errs = schemaErrors(
      'fact',
      factWithAnchors([
        { page: 7, lines: { from: 12, to: 14 }, quote: 'a verbatim excerpt', section: '2(a)' },
        { charSpan: { start: 100, end: 240 } },
        { cells: { sheet: 'Q1', range: 'B7:D9' } },
        { cells: { range: 'B7' } },
        { byteRange: { start: 0, end: 1024 } },
        { region: { x: 10, y: 20, width: 200, height: 40 } },
        { jsonPath: '$.balances[2].amount' },
      ])
    );
    expect(errs).toEqual([]);
  });

  it('rejects unknown anchor fields and malformed shapes', () => {
    expect(schemaErrors('fact', factWithAnchors([{ paragraph: 3 }]))).not.toEqual([]);
    expect(schemaErrors('fact', factWithAnchors([{ lines: { to: 4 } }]))).not.toEqual([]);
    expect(schemaErrors('fact', factWithAnchors([{ cells: { sheet: 'Q1' } }]))).not.toEqual([]);
    expect(schemaErrors('fact', factWithAnchors([{ charSpan: { start: 100 } }]))).not.toEqual([]);
    expect(schemaErrors('fact', factWithAnchors([]))).not.toEqual([]); // anchors, when present, must be non-empty
  });

  it('keeps plain string refs valid alongside anchored refs, and multiple chunks per ref', () => {
    const fact = {
      id: 'f-001',
      statement: 'An observation.',
      evidenceRefs: [
        'ev-001',
        {
          evidenceId: 'ev-002',
          anchors: [
            { quote: 'verbatim excerpt', page: 2 },
            { page: 5, lines: { from: 3, to: 4 } },
          ],
        },
      ],
    };
    expect(schemaErrors('fact', fact)).toEqual([]);
  });

  it('rejects the retired singular anchor field', () => {
    const fact = {
      id: 'f-001',
      statement: 'An observation.',
      evidenceRefs: [{ evidenceId: 'ev-002', anchor: { quote: 'verbatim excerpt' } }],
    };
    expect(schemaErrors('fact', fact)).not.toEqual([]);
  });
});

describe('verifyPackage attestation parts', () => {
  it('schema-checks attestations in the ASC 606 example and stays valid', () => {
    const pkg = openPackage(path.join(EXAMPLES, 'asc606-revenue-contract', 'package'));
    const report = verifyPackage(pkg);
    expect(report.valid).toBe(true);
    const byId = new Map(report.outcomes.map((o) => [o.id, o.status]));
    expect(byId.get('core.attestations.schemaValid')).toBe('pass');
    expect(byId.get('core.partHashes.match')).toBe('pass');
  });

  it('emits no attestation outcome when the package has none', () => {
    const report = verifyPackage(openPackage(path.join(TESTDATA, 'valid', 'minimal-package')));
    expect(report.valid).toBe(true);
    expect(report.outcomes.some((o) => o.id === 'core.attestations.schemaValid')).toBe(false);
  });

  it('fails the attestation part check on a schema-invalid attestation', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-att-test-'));
    fs.cpSync(path.join(TESTDATA, 'valid', 'minimal-package'), dir, { recursive: true });
    fs.mkdirSync(path.join(dir, 'attestations'));
    fs.writeFileSync(
      path.join(dir, 'attestations', 'att-bad.json'),
      JSON.stringify({ id: 'att-bad', createdAt: '2026-07-08T09:30:00Z' }) + '\n'
    );
    const report = verifyPackage(openPackage(dir));
    const byId = new Map(report.outcomes.map((o) => [o.id, o.status]));
    expect(byId.get('core.attestations.schemaValid')).toBe('fail');
    fs.rmSync(dir, { recursive: true, force: true });
  });
});
