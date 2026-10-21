import { describe, expect, it } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import { canonicalJson, stripReviewFields } from '../src/lib/canonical';
import {
  analysisRoot,
  buildGraph,
  closureHash,
  collectionRoot,
  compareIds,
  computeDigests,
  contentRoot,
  dependencyIds,
  evidenceBytesSha256,
  evidenceClosureHash,
  reviewRoot,
  selfHash,
} from '../src/lib/integrity';
import { verifyPackage } from '../src/lib/validations';
import { buildPackage, baseFiles } from './helpers';

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const VECTORS = path.join(REPO_ROOT, 'testdata', 'hashing', 'vectors.json');

const A64 = 'a'.repeat(64);

describe('canonical JSON (RFC 8785)', () => {
  it('sorts keys, drops whitespace, and keeps unicode as is', () => {
    expect(canonicalJson({ b: 1, a: [true, null, 'x'], c: { z: 0.5, y: 'é' } })).toBe('{"a":[true,null,"x"],"b":1,"c":{"y":"é","z":0.5}}');
    expect(canonicalJson(1e21)).toBe('1e+21');
    expect(canonicalJson(100)).toBe('100');
  });

  it('strips review fields per kind without mutating the input', () => {
    const judgment = { id: 'j1', topic: 'T', question: 'Q', status: 'approved' };
    expect(stripReviewFields('judgment', judgment)).toEqual({ id: 'j1', topic: 'T', question: 'Q' });
    expect(judgment.status).toBe('approved');
    const wp = { id: 'wp', title: 'W', conclusionSummary: 'done' };
    expect(stripReviewFields('workproduct', wp)).toEqual({ id: 'wp', title: 'W' });
    expect(wp.conclusionSummary).toBe('done');
    const claim = { id: 'c1', status: 'supported', conclusionSummary: 'kept' };
    expect(stripReviewFields('claim', claim)).toEqual(claim);
  });
});

describe('hashes against values computed with shasum -a 256', () => {
  // printf 'vwp/1/self/fact\n{"id":"f1","statement":"S"}' | shasum -a 256
  it('self hash of a fact', () => {
    expect(selfHash('fact', { statement: 'S', id: 'f1' })).toBe(
      '27ff88b7285bec003d50069ff5e454ba0aa2e325d19000982b3b5147e5c873ff'
    );
  });

  // printf 'vwp/1/self/judgment\n{"id":"j1","question":"Q","topic":"T"}' | shasum -a 256
  it('self hash of a judgment ignores status', () => {
    const expected = 'c0e4b7586dddc316d2f4fac9deaf8250bb5b794ac9ee658fa8f709395f775aa1';
    expect(selfHash('judgment', { id: 'j1', topic: 'T', question: 'Q' })).toBe(expected);
    expect(selfHash('judgment', { id: 'j1', topic: 'T', question: 'Q', status: 'approved' })).toBe(expected);
    expect(selfHash('judgment', { id: 'j1', topic: 'T', question: 'Q', status: 'open' })).toBe(expected);
  });

  it('self hash of a work product ignores conclusionSummary', () => {
    const expected = '7298ed15eda5aeb6784157b18711b61a79930e48cf523fa8ca8edff6b5703287';
    expect(selfHash('workproduct', { id: 'wp', title: 'W' })).toBe(expected);
    expect(selfHash('workproduct', { id: 'wp', title: 'W', conclusionSummary: 'x' })).toBe(expected);
    expect(selfHash('provenance', { createdAt: '2026-01-01T00:00:00Z' })).toBe(
      '48ea3b2a96f4942b4eeb56bb28d61650ffd1bd0b88b0623f9322d41587477ca9'
    );
  });

  it('evidence closure binds to the declared bytes hash', () => {
    const ev = { id: 'e1', type: 'document', title: 'E', sha256: A64 };
    const self = selfHash('evidence', ev);
    expect(self).toBe('05f3c296043a55ca835681b621293f4e4ce3b5097a9dc93e5ee98e98fdc45c95');
    expect(evidenceClosureHash(self, evidenceBytesSha256(ev))).toBe(
      '08ba54832525bb189d9de0fd2dc117eebd8c77ce189297b098fc5873ccb108df'
    );
  });

  it('evidence bytes hash falls back to retrieval.contentHash, then "-"', () => {
    expect(evidenceBytesSha256({ sha256: A64 })).toBe(A64);
    expect(evidenceBytesSha256({ externalRef: { retrieval: { contentHash: 'b'.repeat(64) } } })).toBe('b'.repeat(64));
    expect(evidenceBytesSha256({ externalRef: { locator: 'x' } })).toBe('-');
  });

  it('closure of a fact over its evidence, and with no deps', () => {
    const evClosure = '08ba54832525bb189d9de0fd2dc117eebd8c77ce189297b098fc5873ccb108df';
    const self = selfHash('fact', { id: 'f1', statement: 'S', evidenceRefs: ['e1'] });
    expect(self).toBe('151b049333b9e4a6a968a46f0402428aef8e2d399a864e0736929dfc73c13e33');
    expect(closureHash('fact', self, [{ id: 'e1', hash: evClosure }])).toBe(
      'ca7884f8ab48003ed6e7b64abba3cd74c0ba5ec1e7fa6759b55674889c73afb5'
    );
    expect(closureHash('fact', '27ff88b7285bec003d50069ff5e454ba0aa2e325d19000982b3b5147e5c873ff', [])).toBe(
      '134966bb668ccf39cefe454fe8bccb38b5719f0399ed77c3fa72618e99e79ae4'
    );
  });

  it('collection roots, including the empty collection', () => {
    expect(
      collectionRoot('fact', [{ id: 'f1', hash: 'ca7884f8ab48003ed6e7b64abba3cd74c0ba5ec1e7fa6759b55674889c73afb5' }])
    ).toBe('bc9b17493f861a815d516f648518dc44ef50d0dd83b8655aba13a0176b375f40');
    expect(collectionRoot('claim', [])).toBe('a6d5ffb95cea7b7b39c9fe18f83a552d97780c79cb64e550ac9131fbc5f18155');
  });

  it('memo format self hash uses the memo-format kind', () => {
    expect(
      selfHash('memo-format', { version: 1, sections: [{ id: 'conclusion', heading: 'Conclusion', level: 1, kind: 'conclusion' }] })
    ).toBe('d8c243c2df49ea52a0f555380a63d562a745a1fbfa34b304f093811964622a5a');
  });

  it('sorts dependency lines by id bytes regardless of input order', () => {
    const a = closureHash('claim', 'x', [
      { id: 'f2', hash: '2' },
      { id: 'f1', hash: '1' },
    ]);
    const b = closureHash('claim', 'x', [
      { id: 'f1', hash: '1' },
      { id: 'f2', hash: '2' },
    ]);
    expect(a).toBe(b);
  });
});

describe('dependency edges', () => {
  it('follows the spec section 4 fields and de-duplicates', () => {
    expect(dependencyIds('fact', { evidenceRefs: ['e1', { evidenceId: 'e2', anchors: [] }, 'e1'] })).toEqual(['e1', 'e2']);
    expect(dependencyIds('calculation', { dependsOn: ['f1', 'e1'], inputs: { x: 'f9' } })).toEqual(['f1', 'e1']);
    expect(dependencyIds('judgment', { basisFacts: ['f1'], basisCalculations: ['calc1'] })).toEqual(['f1', 'calc1']);
    expect(
      dependencyIds('claim', { supportingFacts: ['f1'], supportingEvidence: ['e1'], supportingCalculations: ['c1'], judgments: ['j1'] })
    ).toEqual(['f1', 'e1', 'c1', 'j1']);
    // A narrative object is one top-level section; its cites include the subsections'.
    expect(
      dependencyIds('narrative', {
        id: 's1',
        paragraphs: [{ cites: [{ id: 'c1', hash: 'h' }] }],
        subsections: [{ id: 'x', paragraphs: [{ cites: [{ id: 'f1', hash: 'h' }, { id: 'c1', hash: 'h' }] }] }],
      })
    ).toEqual(['c1', 'f1']);
    expect(dependencyIds('approval', { scope: 'package' })).toEqual([]);
    expect(dependencyIds('evidence', { derivedFrom: 'e0' })).toEqual([]);
  });
});

describe('buildGraph and computeDigests', () => {
  it('computes closure hashes through the chain and the manifest digests shape', () => {
    const pkg = buildPackage(
      baseFiles({
        'evidence/evidence.json': { evidence: [{ id: 'e1', type: 'document', title: 'E', sha256: A64 }] },
        'facts/facts.json': { facts: [{ id: 'f1', statement: 'S', evidenceRefs: ['e1'] }] },
        'claims/claims.json': { claims: [{ id: 'c1', statement: 'C', status: 'supported', supportingFacts: ['f1'] }] },
      })
    );
    const graph = buildGraph(pkg);
    expect(graph.problems).toEqual([]);
    expect(graph.objects.get('e1')!.closureHash).toBe('08ba54832525bb189d9de0fd2dc117eebd8c77ce189297b098fc5873ccb108df');
    expect(graph.objects.get('f1')!.closureHash).toBe('ca7884f8ab48003ed6e7b64abba3cd74c0ba5ec1e7fa6759b55674889c73afb5');
    const c1 = graph.objects.get('c1')!;
    expect(c1.closureHash).toBe(closureHash('claim', c1.selfHash, [{ id: 'f1', hash: graph.objects.get('f1')!.closureHash }]));

    const digests = computeDigests(pkg);
    expect(digests.algorithm).toBe('vwp-dag/1');
    expect(digests.collections.fact).toBe('bc9b17493f861a815d516f648518dc44ef50d0dd83b8655aba13a0176b375f40');
    expect(digests.collections.judgment).toBe(collectionRoot('judgment', []));
    expect(Object.keys(digests.collections)).toEqual(['evidence', 'fact', 'calculation', 'judgment', 'claim', 'narrative', 'approval']);
    const wp = JSON.parse(JSON.stringify((baseFiles() as any)['workproduct.json']));
    const prov = (baseFiles() as any)['provenance/provenance.json'];
    expect(digests.analysisRoot).toBe(
      analysisRoot({
        workproductSelfHash: selfHash('workproduct', wp),
        provenanceSelfHash: selfHash('provenance', prov),
        roots: digests.collections,
      })
    );
    expect(digests.contentRoot).toBe(contentRoot(digests.analysisRoot, digests.collections.narrative));
    expect(digests.reviewRoot).toBe(reviewRoot([], digests.collections.approval));
    for (const v of [digests.analysisRoot, digests.contentRoot, digests.reviewRoot, ...Object.values(digests.collections)]) {
      expect(v).toMatch(/^[0-9a-f]{64}$/);
    }
  });

  it('review actions change only the review root', () => {
    const files = baseFiles({
      'judgments/judgments.json': { judgments: [{ id: 'j1', topic: 'T', question: 'Q', status: 'open' }] },
    });
    const before = computeDigests(buildPackage(files));
    (files['judgments/judgments.json'] as any).judgments[0].status = 'approved';
    files['approvals/approvals.json'] = {
      approvals: [{ id: 'a1', approver: { name: 'R' }, action: 'approved', timestamp: '2026-01-01T00:00:00Z' }],
    };
    (files['workproduct.json'] as any).conclusionSummary = 'rewritten by the narrative';
    const after = computeDigests(buildPackage(files));
    expect(after.analysisRoot).toBe(before.analysisRoot);
    expect(after.contentRoot).toBe(before.contentRoot);
    expect(after.reviewRoot).not.toBe(before.reviewRoot);
  });

  it('reports unresolved refs, duplicate ids and cycles as structured problems; the roots are then undefined', () => {
    const pkg = buildPackage(
      baseFiles({
        'facts/facts.json': { facts: [{ id: 'f1', statement: 'S', evidenceRefs: ['e_missing'] }, { id: 'c1', statement: 'dup' }] },
        'calculations/a.json': { id: 'calc_a', type: 'sum', inputs: {}, outputs: {}, dependsOn: ['calc_b'] },
        'calculations/b.json': { id: 'calc_b', type: 'sum', inputs: {}, outputs: {}, dependsOn: ['calc_a'] },
        'calculations/not-a-calc.json': { schemaVersion: 1, entries: [] },
      })
    );
    const graph = buildGraph(pkg);
    const kinds = graph.problems.map((p) => p.kind).sort();
    expect(kinds).toEqual(['cycle', 'duplicate', 'unresolved']);
    expect(graph.problems.find((p) => p.kind === 'unresolved')).toMatchObject({ from: 'fact f1', id: 'e_missing' });
    expect(graph.problems.find((p) => p.kind === 'duplicate')).toMatchObject({ id: 'c1' });
    expect(graph.problems.find((p) => p.kind === 'cycle')!.message).toBe('calc_a -> calc_b -> calc_a');
    expect(graph.byKind.calculation.map((c) => c.id)).toEqual(['calc_a', 'calc_b']);
    for (const id of ['f1', 'calc_a', 'calc_b']) {
      expect(graph.objects.get(id)!.tainted, id).toBe(true);
      expect(graph.objects.get(id)!.closureHash, id).toBe('');
    }
    expect(graph.objects.get('c1')!.tainted).toBe(false);
    expect(() => computeDigests(graph)).toThrow(/failed to compute digests/);
  });

  it('throws when the work product or provenance is missing', () => {
    const files = baseFiles();
    delete (files as any)['workproduct.json'];
    expect(() => computeDigests(buildPackage(files))).toThrow(/workproduct\.json is missing/);
  });
});

/**
 * Shared conformance vectors (spec/integrity.md section 8), written by the Go
 * SDK. The file format is described in testdata/hashing/README.md. This
 * suite fails loudly when the file is absent: the vectors are part of the
 * contract, not an optional extra.
 */
describe('conformance vectors (testdata/hashing/vectors.json)', () => {
  it('exists', () => {
    expect(fs.existsSync(VECTORS), `${VECTORS} is missing; the Go SDK writes it (spec/integrity.md section 8)`).toBe(true);
  });

  const vectors: any = fs.existsSync(VECTORS) ? JSON.parse(fs.readFileSync(VECTORS, 'utf8')) : {};
  const cases = (name: string): any[] => (Array.isArray(vectors[name]) ? vectors[name] : []);

  it('pins the algorithm this CLI implements', () => {
    expect(vectors.algorithm).toBe('vwp-dag/1');
  });

  it('self hash cases: canonical bytes and self hash', () => {
    const list = cases('self');
    expect(list.length).toBeGreaterThan(0);
    for (const c of list) {
      expect(canonicalJson(stripReviewFields(c.kind, c.object)), `${c.name}: canonical`).toBe(c.canonical);
      expect(selfHash(c.kind, c.object), `${c.name}: selfHash`).toBe(c.selfHash);
    }
  });

  it('strip cases: stripped object, canonical bytes and self hash', () => {
    const list = cases('strip');
    expect(list.length).toBeGreaterThan(0);
    const byName = new Map<string, string>();
    for (const c of list) {
      const stripped = stripReviewFields(c.kind, c.input);
      expect(stripped, `${c.name}: stripped`).toEqual(c.stripped);
      expect(canonicalJson(stripped), `${c.name}: canonical`).toBe(c.canonical);
      expect(selfHash(c.kind, c.input), `${c.name}: selfHash`).toBe(c.selfHash);
      byName.set(c.name, c.selfHash);
    }
    expect(byName.get('judgment without status')).toBe(byName.get('judgment with status'));
    expect(byName.get('workproduct without conclusionSummary')).toBe(byName.get('workproduct with conclusionSummary'));
  });

  it('evidence closure cases: bytes line and closure hash', () => {
    const list = cases('evidenceClosure');
    expect(list.length).toBeGreaterThan(0);
    for (const c of list) {
      const self = selfHash('evidence', c.object);
      const bytes = evidenceBytesSha256(c.object);
      expect(self, `${c.name}: selfHash`).toBe(c.selfHash);
      expect(`bytes:${bytes}`, `${c.name}: bytesLine`).toBe(c.bytesLine);
      expect(evidenceClosureHash(self, bytes), `${c.name}: closureHash`).toBe(c.closureHash);
    }
  });

  it('closure package: every object, the collection roots, the format hash and the three roots', () => {
    const closure = vectors.closure;
    expect(closure, 'closure section').toBeDefined();
    const graph = buildGraph(buildPackage(closure.package));
    expect(graph.problems).toEqual([]);
    expect(closure.objects.length).toBeGreaterThan(0);
    for (const want of closure.objects) {
      const label = `${want.kind} ${JSON.stringify(want.id)}`;
      if (want.kind === 'workproduct' || want.kind === 'provenance') {
        const part = want.kind === 'workproduct' ? graph.workproduct : graph.provenance;
        expect(selfHash(want.kind, part), `${label}: selfHash`).toBe(want.selfHash);
        continue;
      }
      const got = want.kind === 'narrative' ? graph.byKind.narrative.find((s) => s.id === want.id) : graph.objects.get(want.id);
      expect(got, label).toBeDefined();
      expect(got!.kind, `${label}: kind`).toBe(want.kind);
      expect([...got!.deps].sort(compareIds), `${label}: deps`).toEqual(want.deps);
      expect(got!.selfHash, `${label}: selfHash`).toBe(want.selfHash);
      expect(got!.closureHash, `${label}: closureHash`).toBe(want.closureHash);
    }
    const digests = computeDigests(graph);
    expect(digests.collections).toEqual(closure.collections);
    expect(selfHash('memo-format', closure.package['narrative/format.json'])).toBe(closure.formatHash);
    expect(digests.analysisRoot).toBe(closure.analysisRoot);
    expect(digests.contentRoot).toBe(closure.contentRoot);
    expect(digests.reviewRoot).toBe(closure.reviewRoot);
  });

  it('closure package: the narrative and approval it carries verify as fresh and current', () => {
    const closure = vectors.closure;
    expect(closure, 'closure section').toBeDefined();
    const report = verifyPackage(buildPackage({ 'manifest.json': manifestFor(closure.package), ...closure.package }));
    const byId = new Map(report.outcomes.map((o) => [o.id, o.status]));
    for (const id of ['core.graph.refsResolve', 'core.graph.acyclic', 'narrative.sections', 'narrative.citations', 'narrative.cites.fresh', 'narrative.drift', 'approvals.current']) {
      expect(byId.get(id), id).toBe('pass');
    }
  });
});

/** A manifest declaring the vector package's parts, so the package verifies end to end. */
function manifestFor(parts: Record<string, unknown>): object {
  return {
    vwpVersion: '0.1',
    packageId: 'vwp_vectors',
    createdAt: '2026-10-20T00:00:00Z',
    profile: 'vwp-finance',
    workproduct: { path: 'workproduct.json', mediaType: 'application/json' },
    provenance: { path: 'provenance/provenance.json', mediaType: 'application/json' },
    parts: Object.keys(parts)
      .filter((p) => p !== 'workproduct.json' && p !== 'provenance/provenance.json')
      .map((p) => ({ path: p, mediaType: 'application/json' })),
  };
}
