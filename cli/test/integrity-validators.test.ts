import { describe, expect, it } from 'vitest';
import * as path from 'path';
import { openPackage } from '../src/lib/package';
import { verifyPackage } from '../src/lib/validations';
import { buildGraph, computeDigests, selfHash } from '../src/lib/integrity';
import { buildPackage, baseFiles, statuses } from './helpers';

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const ASC606 = path.join(REPO_ROOT, 'examples', 'asc606-revenue-contract', 'package');

type Files = Record<string, string | object | Buffer>;

function verify(files: Files) {
  const report = verifyPackage(buildPackage(files));
  return { report, byId: statuses(report.outcomes), outcome: (id: string) => report.outcomes.find((o) => o.id === id) };
}

const FORMAT = {
  version: 1,
  name: 'Test memo',
  sections: [
    { id: 'analysis', heading: 'Analysis', level: 1, kind: 'narrative', required: true, templateSection: 's2' },
    { id: 'conclusion', heading: 'Conclusion', level: 1, kind: 'conclusion', required: true },
    { id: 'claims', heading: 'Claims', level: 1, kind: 'claims' },
  ],
};

/** The analysis: evidence -> fact -> claim, a judgment weighing the fact, a calculation. */
function analysisFiles(): Files {
  return baseFiles({
    'evidence/evidence.json': { evidence: [{ id: 'e1', type: 'document', title: 'E', path: 'evidence/e1.txt' }] },
    'evidence/e1.txt': 'the evidence text',
    'facts/facts.json': { facts: [{ id: 's2_f1', statement: 'Fact one.', evidenceRefs: ['e1'] }] },
    'calculations/calc.json': { id: 'calc1', type: 'sum', inputs: { values: [1, 2] }, outputs: { total: 3 }, dependsOn: ['s2_f1'] },
    'judgments/judgments.json': {
      judgments: [{ id: 'j1', topic: 'T', question: 'Q', basis: 'Weighed s2_f1.', basisFacts: ['s2_f1'], status: 'reviewed' }],
    },
    'claims/claims.json': {
      claims: [{ id: 'c1', statement: 'Claim one.', status: 'supported', supportingFacts: ['s2_f1'], supportingCalculations: ['calc1'], judgments: ['j1'] }],
    },
  });
}

/** Write a narrative against the current state of the analysis, with fresh cite hashes. */
function withNarrative(files: Files, format: any = FORMAT): Files {
  const graph = buildGraph(buildPackage(files));
  const hashOf = (id: string) => graph.objects.get(id)!.closureHash;
  const digests = computeDigests(graph);
  return {
    ...files,
    'narrative/format.json': format,
    'narrative/narrative.json': {
      formatHash: selfHash('memo-format', format),
      analysisRoot: digests.analysisRoot,
      sections: [
        {
          id: 'analysis',
          paragraphs: [{ text: 'Fact one holds.', role: 'analysis', cites: [{ id: 's2_f1', hash: hashOf('s2_f1') }] }],
          subsections: [
            { id: 'detail', heading: 'Detail', paragraphs: [{ text: 'The sum is three.', role: 'analysis', cites: [{ id: 'calc1', hash: hashOf('calc1') }] }] },
          ],
        },
        {
          id: 'conclusion',
          paragraphs: [{ text: 'Claim one is supported.', role: 'conclusion', cites: [{ id: 'c1', hash: hashOf('c1') }, { id: 'j1', hash: hashOf('j1') }] }],
        },
      ],
    },
  };
}

function withDigests(files: Files): Files {
  const manifest = JSON.parse(JSON.stringify(files['manifest.json']));
  manifest.digests = computeDigests(buildPackage(files));
  return { ...files, 'manifest.json': manifest };
}

function narrativeOf(files: Files): any {
  return JSON.parse(JSON.stringify(files['narrative/narrative.json']));
}

describe('a valid package with a narrative, digests and a current approval', () => {
  const files = withNarrative(analysisFiles());
  const digests = computeDigests(buildPackage(files));
  files['approvals/approvals.json'] = {
    approvals: [
      { id: 'a1', approver: { name: 'Reviewer' }, action: 'approved', timestamp: '2026-01-02T00:00:00Z', scope: 'package', contentRoot: digests.contentRoot, reviewRoot: digests.reviewRoot },
    ],
  };
  const { report, byId } = verify(withDigests(files));

  it('passes every graph, digest, approval and narrative validator', () => {
    expect(report.valid).toBe(true);
    for (const id of [
      'core.graph.refsResolve',
      'core.graph.acyclic',
      'core.graph.declaredInputs',
      'core.digests.match',
      'approvals.current',
      'narrative.schemaValid',
      'narrative.sections',
      'narrative.citations',
      'narrative.roles',
      'narrative.cites.fresh',
      'narrative.drift',
      'core.claimRefs.resolve',
    ]) {
      expect(byId.get(id), id).toBe('pass');
    }
  });
});

describe('core.graph validators', () => {
  it('fails core.graph.refsResolve on an unresolved basisFacts id while core.claimRefs.resolve stays as it was', () => {
    const files = analysisFiles();
    (files['judgments/judgments.json'] as any).judgments[0].basisFacts = ['f_nowhere'];
    const { byId, outcome } = verify(files);
    expect(byId.get('core.graph.refsResolve')).toBe('fail');
    expect(outcome('core.graph.refsResolve')!.message).toMatch(/judgment j1: unknown basisFacts ref f_nowhere/);
    expect(byId.get('core.claimRefs.resolve')).toBe('pass');
    expect(byId.get('core.graph.acyclic')).toBe('pass');
  });

  it('fails core.graph.refsResolve when an id is used by two collections', () => {
    const files = analysisFiles();
    (files['facts/facts.json'] as any).facts.push({ id: 'c1', statement: 'Same id as the claim.' });
    const { byId, outcome } = verify(files);
    expect(byId.get('core.graph.refsResolve')).toBe('fail');
    expect(outcome('core.graph.refsResolve')!.message).toMatch(/claim c1: id already used by a fact/);
  });

  it('fails core.graph.acyclic on a dependency cycle', () => {
    const files = analysisFiles();
    files['calculations/calc.json'] = { id: 'calc1', type: 'sum', inputs: {}, outputs: {}, dependsOn: ['calc2'] };
    files['calculations/calc2.json'] = { id: 'calc2', type: 'sum', inputs: {}, outputs: {}, dependsOn: ['calc1'] };
    const { report, byId, outcome } = verify(files);
    expect(byId.get('core.graph.acyclic')).toBe('fail');
    expect(outcome('core.graph.acyclic')!.message).toMatch(/dependency cycles: calc1 -> calc2 -> calc1/);
    expect(byId.get('core.graph.refsResolve')).toBe('pass');
    expect(report.valid).toBe(false);
  });

  it('warns on core.graph.declaredInputs for undeclared calculation inputs and judgment basis facts', () => {
    const files = analysisFiles();
    files['calculations/calc.json'] = {
      id: 'calc1',
      type: 'relativeStandaloneSellingPriceAllocation',
      inputs: { components: [{ id: 'a', sspFact: 's2_f1' }], evidence: 'e1' },
      outputs: {},
    };
    (files['judgments/judgments.json'] as any).judgments[0].basisFacts = [];
    const { byId, outcome } = verify(files);
    expect(byId.get('core.graph.declaredInputs')).toBe('warning');
    const message = outcome('core.graph.declaredInputs')!.message!;
    expect(message).toMatch(/calculation calc1: inputs reference e1, s2_f1 not declared in dependsOn/);
    expect(message).toMatch(/judgment j1: basis mentions s2_f1 not declared in basisFacts/);
  });

  it('does not warn when the mention is a longer token that merely contains a fact id', () => {
    const files = analysisFiles();
    (files['judgments/judgments.json'] as any).judgments[0].basis = 'See s2_f10 for context.';
    (files['judgments/judgments.json'] as any).judgments[0].basisFacts = [];
    expect(verify(files).byId.get('core.graph.declaredInputs')).toBe('pass');
  });
});

describe('core.digests.match', () => {
  it('is skipped when the manifest carries no digests block', () => {
    const { byId } = verify(analysisFiles());
    expect(byId.get('core.digests.match')).toBe('notRun');
  });

  it('fails and names the mismatching roots when the package changed after the digests were written', () => {
    const files = withDigests(analysisFiles());
    (files['facts/facts.json'] as any).facts[0].statement = 'Fact one, edited.';
    const { report, byId, outcome } = verify(files);
    expect(byId.get('core.digests.match')).toBe('fail');
    expect(report.valid).toBe(false);
    const message = outcome('core.digests.match')!.message!;
    expect(message).toMatch(/analysisRoot/);
    expect(message).toMatch(/collections\.fact/);
    expect(message).not.toMatch(/reviewRoot/);
  });

  it('fails when the manifest stores a wrong review root', () => {
    const files = withDigests(analysisFiles());
    (files['manifest.json'] as any).digests.reviewRoot = 'f'.repeat(64);
    const { outcome } = verify(files);
    expect(outcome('core.digests.match')!.status).toBe('fail');
    expect(outcome('core.digests.match')!.message).toMatch(/reviewRoot/);
  });
});

describe('approvals.current', () => {
  const approval = (contentRoot?: string, action = 'approved') => ({
    approvals: [{ id: 'a1', approver: { name: 'R' }, action, timestamp: '2026-01-02T00:00:00Z', ...(contentRoot ? { contentRoot } : {}) }],
  });

  it('is skipped when no approval records a contentRoot', () => {
    const files = analysisFiles();
    files['approvals/approvals.json'] = approval();
    expect(verify(files).byId.get('approvals.current')).toBe('notRun');
  });

  it('requires review for an approved action whose contentRoot is superseded', () => {
    const files = analysisFiles();
    const before = computeDigests(buildPackage(files)).contentRoot;
    (files['facts/facts.json'] as any).facts[0].statement = 'Fact one, edited after approval.';
    files['approvals/approvals.json'] = approval(before);
    const { report, byId, outcome } = verify(files);
    expect(byId.get('approvals.current')).toBe('reviewRequired');
    expect(outcome('approvals.current')!.message).toMatch(/a1/);
    expect(report.valid).toBe(true);
  });

  it('passes once a later approval covers the current content, listing the superseded one', () => {
    const files = analysisFiles();
    const before = computeDigests(buildPackage(files)).contentRoot;
    (files['facts/facts.json'] as any).facts[0].statement = 'Fact one, edited after approval.';
    const after = computeDigests(buildPackage(files)).contentRoot;
    files['approvals/approvals.json'] = {
      approvals: [
        { id: 'a1', approver: { name: 'R' }, action: 'approved', timestamp: '2026-01-02T00:00:00Z', contentRoot: before },
        { id: 'a2', approver: { name: 'R' }, action: 'approved', timestamp: '2026-01-03T00:00:00Z', contentRoot: after },
      ],
    };
    const { byId, outcome } = verify(files);
    expect(byId.get('approvals.current')).toBe('pass');
    expect(outcome('approvals.current')!.message).toMatch(/a1/);
  });

  it('ignores non-approved actions with a stale contentRoot', () => {
    const files = analysisFiles();
    files['approvals/approvals.json'] = approval('0'.repeat(64), 'requestedChanges');
    expect(verify(files).byId.get('approvals.current')).toBe('pass');
  });
});

describe('narrative validators', () => {
  it('fails narrative.cites.fresh naming the stale object and the citing sections, and skips drift', () => {
    const files = withNarrative(analysisFiles());
    (files['facts/facts.json'] as any).facts[0].statement = 'Fact one, edited after the narrative.';
    const { report, byId, outcome } = verify(files);
    expect(byId.get('narrative.cites.fresh')).toBe('fail');
    const message = outcome('narrative.cites.fresh')!.message!;
    expect(message).toMatch(/s2_f1/);
    expect(message).toMatch(/analysis/);
    // The claim's closure moved too (it rests on the fact), so the conclusion section is also stale.
    expect(message).toMatch(/c1/);
    expect(message).toMatch(/conclusion/);
    expect(byId.get('narrative.drift')).toBe('notRun');
    expect(report.valid).toBe(false);
  });

  it('requires review on narrative.drift when cites are fresh but something new and uncited exists', () => {
    const files = withNarrative(analysisFiles());
    (files['facts/facts.json'] as any).facts.push({ id: 's2_f2', statement: 'A new fact nobody cites.', evidenceRefs: ['e1'] });
    const { report, byId } = verify(files);
    expect(byId.get('narrative.cites.fresh')).toBe('pass');
    expect(byId.get('narrative.drift')).toBe('reviewRequired');
    expect(report.valid).toBe(true);
  });

  it('fails narrative.schemaValid on schema errors, multi-line headings and control characters', () => {
    const base = withNarrative(analysisFiles());

    const noRoot = narrativeOf(base);
    delete noRoot.analysisRoot;
    expect(verify({ ...base, 'narrative/narrative.json': noRoot }).byId.get('narrative.schemaValid')).toBe('fail');

    const badHeading = narrativeOf(base);
    badHeading.sections[0].subsections[0].heading = 'Detail\n# Forged heading';
    const h = verify({ ...base, 'narrative/narrative.json': badHeading });
    expect(h.byId.get('narrative.schemaValid')).toBe('fail');
    expect(h.outcome('narrative.schemaValid')!.message).toMatch(/single line/);

    const badText = narrativeOf(base);
    badText.sections[0].paragraphs[0].text = 'Fact one holds.';
    const t = verify({ ...base, 'narrative/narrative.json': badText });
    expect(t.byId.get('narrative.schemaValid')).toBe('fail');
    expect(t.outcome('narrative.schemaValid')!.message).toMatch(/control characters/);

    expect(verify({ ...base, 'narrative/narrative.json': '{not json' }).byId.get('narrative.schemaValid')).toBe('fail');
  });

  it('runs narrative.sections when only the format snapshot exists and a required section is unfilled', () => {
    const files = { ...analysisFiles(), 'narrative/format.json': FORMAT };
    const { byId, outcome } = verify(files);
    expect(byId.get('narrative.schemaValid')).toBeUndefined();
    expect(byId.get('narrative.sections')).toBe('fail');
    expect(outcome('narrative.sections')!.message).toMatch(/required section "analysis" has no paragraphs/);
  });

  it('fails narrative.sections on a formatHash mismatch, an unknown section, a non-prose section, or an invalid snapshot', () => {
    const base = withNarrative(analysisFiles());

    const wrongHash = narrativeOf(base);
    wrongHash.formatHash = '0'.repeat(64);
    const w = verify({ ...base, 'narrative/narrative.json': wrongHash });
    expect(w.byId.get('narrative.sections')).toBe('fail');
    expect(w.outcome('narrative.sections')!.message).toMatch(/formatHash/);

    const unknownSection = narrativeOf(base);
    unknownSection.sections[0].id = 'appendix';
    expect(verify({ ...base, 'narrative/narrative.json': unknownSection }).outcome('narrative.sections')!.message).toMatch(
      /section "appendix" is not a section of the memo format/
    );

    const structural = narrativeOf(base);
    structural.sections.push({ id: 'claims', paragraphs: [{ text: 'Prose in a claims table.', role: 'analysis', cites: [{ id: 'c1', hash: 'a'.repeat(64) }] }] });
    expect(verify({ ...base, 'narrative/narrative.json': structural }).outcome('narrative.sections')!.message).toMatch(
      /section "claims" is of kind claims, not prose/
    );

    const badFormat = verify({ ...base, 'narrative/format.json': { version: 2, sections: [] } });
    expect(badFormat.byId.get('narrative.sections')).toBe('fail');
    expect(badFormat.outcome('narrative.sections')!.message).toMatch(/narrative\/format\.json/);
  });

  it('fails narrative.citations on an unresolved cite and requires review for uncited claims', () => {
    const base = withNarrative(analysisFiles());

    const unknown = narrativeOf(base);
    unknown.sections[0].paragraphs[0].cites = [{ id: 'f_ghost', hash: 'a'.repeat(64) }];
    const u = verify({ ...base, 'narrative/narrative.json': unknown });
    expect(u.byId.get('narrative.citations')).toBe('fail');
    expect(u.outcome('narrative.citations')!.message).toMatch(/unknown cite f_ghost/);

    const empty = narrativeOf(base);
    empty.sections[0].paragraphs[0].cites = [];
    expect(verify({ ...base, 'narrative/narrative.json': empty }).byId.get('narrative.citations')).toBe('fail');

    const files = withNarrative(analysisFiles());
    (files['claims/claims.json'] as any).claims.push({ id: 'c2', statement: 'Uncited claim.', status: 'supported' });
    const r = verify(files);
    expect(r.byId.get('narrative.citations')).toBe('reviewRequired');
    expect(r.outcome('narrative.citations')!.message).toMatch(/c2/);
  });

  it('fails narrative.roles for a conclusion citing no claim or judgment and requires review for an uncovered template section', () => {
    const base = withNarrative(analysisFiles());

    const factOnly = narrativeOf(base);
    factOnly.sections[1].paragraphs[0].cites = [factOnly.sections[0].paragraphs[0].cites[0]];
    const f = verify({ ...base, 'narrative/narrative.json': factOnly });
    expect(f.byId.get('narrative.roles')).toBe('fail');
    expect(f.outcome('narrative.roles')!.message).toMatch(/conclusion paragraph cites no claim or judgment/);

    const uncovered = narrativeOf(base);
    uncovered.sections[0].paragraphs[0].cites = [uncovered.sections[1].paragraphs[0].cites[0]];
    uncovered.sections[0].subsections = [];
    const t = verify({ ...base, 'narrative/narrative.json': uncovered });
    expect(t.byId.get('narrative.roles')).toBe('reviewRequired');
    expect(t.outcome('narrative.roles')!.message).toMatch(/template section s2/);
  });

  it('emits no narrative outcomes for a package without narrative parts', () => {
    const { byId } = verify(analysisFiles());
    for (const id of ['narrative.schemaValid', 'narrative.sections', 'narrative.citations', 'narrative.roles', 'narrative.cites.fresh', 'narrative.drift']) {
      expect(byId.has(id), id).toBe(false);
    }
  });
});

describe('memo format additions (header, sources, signoff roles)', () => {
  const header = (n: number) => Array.from({ length: n }, (_, i) => ({ label: `Field ${i + 1}`, source: 'literal', value: `v${i + 1}` }));

  it('accepts a header of 24 fields and rejects 25', () => {
    const ok = withNarrative(analysisFiles(), { ...FORMAT, header: header(24) });
    expect(verify(ok).byId.get('narrative.sections')).toBe('pass');

    const tooMany = withNarrative(analysisFiles(), { ...FORMAT, header: header(25) });
    const r = verify(tooMany);
    expect(r.byId.get('narrative.sections')).toBe('fail');
    expect(r.outcome('narrative.sections')!.message).toMatch(/narrative\/format\.json/);
    expect(r.outcome('narrative.sections')!.message).toMatch(/header/);
  });

  it('accepts the date and standardRefs header sources', () => {
    const format = {
      ...FORMAT,
      header: [
        { label: 'Date', source: 'date' },
        { label: 'Authoritative literature', source: 'standardRefs' },
      ],
    };
    expect(verify(withNarrative(analysisFiles(), format)).byId.get('narrative.sections')).toBe('pass');

    const unknown = { ...FORMAT, header: [{ label: 'X', source: 'weather' }] };
    expect(verify(withNarrative(analysisFiles(), unknown)).byId.get('narrative.sections')).toBe('fail');
  });

  it('allows roles on signoff sections only, each a non-empty single line', () => {
    const signoff = { ...FORMAT, sections: [...FORMAT.sections, { id: 'signoff', heading: 'Sign-off', level: 1, kind: 'signoff', roles: ['Preparer', 'Global Controller'] }] };
    expect(verify(withNarrative(analysisFiles(), signoff)).byId.get('narrative.sections')).toBe('pass');

    const onProse = { ...FORMAT, sections: FORMAT.sections.map((s) => (s.id === 'analysis' ? { ...s, roles: ['Preparer'] } : s)) };
    const p = verify(withNarrative(analysisFiles(), onProse));
    expect(p.byId.get('narrative.sections')).toBe('fail');
    expect(p.outcome('narrative.sections')!.message).toMatch(/section "analysis" roles are allowed only on signoff sections/);

    const multiLine = { ...FORMAT, sections: [...FORMAT.sections, { id: 'signoff', heading: 'Sign-off', level: 1, kind: 'signoff', roles: ['Global\nController'] }] };
    const m = verify(withNarrative(analysisFiles(), multiLine));
    expect(m.byId.get('narrative.sections')).toBe('fail');
    expect(m.outcome('narrative.sections')!.message).toMatch(/section "signoff" role 1 must be a non-empty single line/);

    const blank = { ...FORMAT, sections: [...FORMAT.sections, { id: 'signoff', heading: 'Sign-off', level: 1, kind: 'signoff', roles: ['Preparer', '  '] }] };
    const b = verify(withNarrative(analysisFiles(), blank));
    expect(b.outcome('narrative.sections')!.message).toMatch(/section "signoff" role 2 must be a non-empty single line/);

    const tooMany = { ...FORMAT, sections: [...FORMAT.sections, { id: 'signoff', heading: 'Sign-off', level: 1, kind: 'signoff', roles: Array.from({ length: 25 }, (_, i) => `Role ${i + 1}`) }] };
    expect(verify(withNarrative(analysisFiles(), tooMany)).byId.get('narrative.sections')).toBe('fail');
  });
});

describe('literature paragraphs', () => {
  /** The analysis with the claim resting on ASC 606 literature. */
  function literatureFiles(): Files {
    const files = analysisFiles();
    (files['claims/claims.json'] as any).claims[0].standardRefs = ['ASC 606-10-25-1', 'ASC 606-10-32-5'];
    return files;
  }

  function withLiterature(text: string, citeIds: string[]): Files {
    const base = withNarrative(literatureFiles());
    const graph = buildGraph(buildPackage(base));
    const narrative = narrativeOf(base);
    narrative.sections[0].paragraphs.push({
      text,
      role: 'literature',
      cites: citeIds.map((id) => ({ id, hash: graph.objects.get(id)!.closureHash })),
    });
    return { ...base, 'narrative/narrative.json': narrative };
  }

  it('passes when the paragraph names a cited claim reference, with or without the ASC prefix', () => {
    const full = verify(withLiterature('ASC 606-10-25-1 states that a contract exists when the parties have approved it.', ['c1']));
    expect(full.byId.get('narrative.schemaValid')).toBe('pass');
    expect(full.byId.get('narrative.roles')).toBe('pass');

    const stripped = verify(withLiterature('Under 606-10-32-5, variable consideration is estimated.', ['c1']));
    expect(stripped.byId.get('narrative.roles')).toBe('pass');
  });

  it('passes when the paragraph cites a judgment and names a reference of a claim listing that judgment', () => {
    const r = verify(withLiterature('Per 606-10-25-1 the contract criteria are met.', ['j1']));
    expect(r.byId.get('narrative.roles')).toBe('pass');
  });

  it('fails when the paragraph names none of the cited references', () => {
    const r = verify(withLiterature('A contract exists when the parties have approved it.', ['c1']));
    expect(r.byId.get('narrative.roles')).toBe('fail');
    expect(r.outcome('narrative.roles')!.message).toMatch(/section "analysis": a literature paragraph names none of its cited references/);

    // A partial number is not a reference.
    const partial = verify(withLiterature('See 606-10 generally.', ['c1']));
    expect(partial.byId.get('narrative.roles')).toBe('fail');
  });

  it('fails when the paragraph cites no claim or judgment', () => {
    const r = verify(withLiterature('ASC 606-10-25-1 states that a contract exists.', ['s2_f1']));
    expect(r.byId.get('narrative.roles')).toBe('fail');
    expect(r.outcome('narrative.roles')!.message).toMatch(/section "analysis": a literature paragraph cites no claim or judgment/);
  });

  it('fails when the cited claims carry no standardRefs', () => {
    const base = withNarrative(analysisFiles());
    const graph = buildGraph(buildPackage(base));
    const narrative = narrativeOf(base);
    narrative.sections[0].paragraphs.push({ text: 'ASC 606-10-25-1 says so.', role: 'literature', cites: [{ id: 'c1', hash: graph.objects.get('c1')!.closureHash }] });
    const r = verify({ ...base, 'narrative/narrative.json': narrative });
    expect(r.byId.get('narrative.roles')).toBe('fail');
    expect(r.outcome('narrative.roles')!.message).toMatch(/names none of its cited references/);
  });
});

describe('notApplicable sections', () => {
  const TAX_FORMAT = {
    ...FORMAT,
    sections: [...FORMAT.sections, { id: 'tax', heading: 'Tax and treasury', level: 1, kind: 'narrative', required: true }],
  };

  function withTax(section: any): Files {
    const base = withNarrative(analysisFiles(), TAX_FORMAT);
    const narrative = narrativeOf(base);
    narrative.sections.push({ id: 'tax', ...section });
    return { ...base, 'narrative/narrative.json': narrative };
  }

  it('satisfies a required prose section and is ignored by citations and cites.fresh', () => {
    const missing = verify(withNarrative(analysisFiles(), TAX_FORMAT));
    expect(missing.byId.get('narrative.sections')).toBe('fail');
    expect(missing.outcome('narrative.sections')!.message).toMatch(/required section "tax" has no paragraphs/);

    const { report, byId } = verify(withTax({ paragraphs: [], notApplicable: 'No tax or treasury impact identified' }));
    expect(byId.get('narrative.schemaValid')).toBe('pass');
    expect(byId.get('narrative.sections')).toBe('pass');
    expect(byId.get('narrative.citations')).toBe('pass');
    expect(byId.get('narrative.roles')).toBe('pass');
    expect(byId.get('narrative.cites.fresh')).toBe('pass');
    expect(byId.get('narrative.drift')).toBe('pass');
    expect(report.valid).toBe(true);
  });

  it('fails narrative.schemaValid when a notApplicable section carries paragraphs or subsections', () => {
    const base = withNarrative(analysisFiles(), TAX_FORMAT);
    const graph = buildGraph(buildPackage(base));
    const cite = { id: 's2_f1', hash: graph.objects.get('s2_f1')!.closureHash };

    const withParagraphs = verify(withTax({ paragraphs: [{ text: 'Some prose.', role: 'analysis', cites: [cite] }], notApplicable: 'Not applicable' }));
    expect(withParagraphs.byId.get('narrative.schemaValid')).toBe('fail');
    expect(withParagraphs.outcome('narrative.schemaValid')!.message).toMatch(/section "tax": notApplicable sections carry no paragraphs/);

    const withSubsections = verify(
      withTax({ paragraphs: [], subsections: [{ id: 'sub', heading: 'Sub', paragraphs: [{ text: 'Some prose.', role: 'analysis', cites: [cite] }] }], notApplicable: 'Not applicable' })
    );
    expect(withSubsections.byId.get('narrative.schemaValid')).toBe('fail');
    expect(withSubsections.outcome('narrative.schemaValid')!.message).toMatch(/section "tax": notApplicable sections carry no paragraphs/);
  });

  it('fails narrative.schemaValid when notApplicable is multi-line, empty or over 512 bytes', () => {
    const multi = verify(withTax({ paragraphs: [], notApplicable: 'No impact\n# Forged' }));
    expect(multi.byId.get('narrative.schemaValid')).toBe('fail');
    expect(multi.outcome('narrative.schemaValid')!.message).toMatch(/section "tax" notApplicable must be a single line/);

    expect(verify(withTax({ paragraphs: [], notApplicable: '' })).byId.get('narrative.schemaValid')).toBe('fail');

    // 300 three-byte characters: within the schema's 512-character cap, over 512 bytes.
    const bytes = verify(withTax({ paragraphs: [], notApplicable: '€'.repeat(300) }));
    expect(bytes.byId.get('narrative.schemaValid')).toBe('fail');
    expect(bytes.outcome('narrative.schemaValid')!.message).toMatch(/section "tax" notApplicable exceeds 512 bytes/);
  });

  it('contributes to the narrative self hash through canonicalization', () => {
    const a = narrativeOf(withTax({ paragraphs: [], notApplicable: 'Reason A' }));
    const b = narrativeOf(withTax({ paragraphs: [], notApplicable: 'Reason B' }));
    expect(selfHash('narrative', a)).not.toBe(selfHash('narrative', b));
  });
});

describe('the ASC 606 example', () => {
  it('verifies with no new failures', () => {
    const report = verifyPackage(openPackage(ASC606));
    const byId = statuses(report.outcomes);
    expect(report.valid).toBe(true);
    expect(byId.get('core.graph.refsResolve')).toBe('pass');
    expect(byId.get('core.graph.acyclic')).toBe('pass');
    expect(['pass', 'notRun']).toContain(byId.get('core.digests.match'));
    expect(['pass', 'warning']).toContain(byId.get('core.graph.declaredInputs'));
  });
});
