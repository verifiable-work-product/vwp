import { describe, expect, it } from 'vitest';
import { diffPackages, DiffLine } from '../src/lib/diff';
import { buildPackage, baseFiles } from './helpers';

function lineFor(lines: DiffLine[], kind: string, id: string): DiffLine | undefined {
  return lines.find((l) => l.kind === kind && l.id === id);
}

describe('diffPackages', () => {
  it('returns no lines for identical packages', () => {
    const a = buildPackage(baseFiles());
    const b = buildPackage(baseFiles());
    expect(diffPackages(a, b)).toEqual([]);
  });

  it('reports changed workproduct fields with a detail', () => {
    const oldPkg = buildPackage(baseFiles());
    const files = baseFiles();
    (files['workproduct.json'] as any).status = 'final';
    const newPkg = buildPackage(files);
    const line = lineFor(diffPackages(oldPkg, newPkg), 'workproduct', 'status');
    expect(line?.change).toBe('changed');
    expect(line?.detail).toContain('"draft" -> "final"');
  });

  it('reports added, removed, and changed collection items', () => {
    const oldPkg = buildPackage(
      baseFiles({
        'evidence/evidence.json': {
          evidence: [
            { id: 'ev_kept', type: 'document', title: 'Kept', path: 'evidence/a.txt' },
            { id: 'ev_removed', type: 'document', title: 'Removed', path: 'evidence/a.txt' },
          ],
        },
        'evidence/a.txt': 'x',
      })
    );
    const newPkg = buildPackage(
      baseFiles({
        'evidence/evidence.json': {
          evidence: [
            { id: 'ev_kept', type: 'document', title: 'Kept, retitled', path: 'evidence/a.txt' },
            { id: 'ev_added', type: 'document', title: 'Added', path: 'evidence/a.txt' },
          ],
        },
        'evidence/a.txt': 'x',
      })
    );
    const lines = diffPackages(oldPkg, newPkg);
    expect(lineFor(lines, 'evidence', 'ev_removed')?.change).toBe('removed');
    expect(lineFor(lines, 'evidence', 'ev_added')?.change).toBe('added');
    expect(lineFor(lines, 'evidence', 'ev_kept')?.change).toBe('changed');
  });

  it('details judgment conclusion and status changes', () => {
    const oldPkg = buildPackage(
      baseFiles({
        'judgments/judgments.json': {
          judgments: [{ id: 'j1', question: 'Q', proposedConclusion: 'yes', status: 'open' }],
        },
      })
    );
    const newPkg = buildPackage(
      baseFiles({
        'judgments/judgments.json': {
          judgments: [{ id: 'j1', question: 'Q', proposedConclusion: 'no', status: 'resolved' }],
        },
      })
    );
    const line = lineFor(diffPackages(oldPkg, newPkg), 'judgment', 'j1');
    expect(line?.detail).toContain('conclusion: "yes" -> "no"');
    expect(line?.detail).toContain('status: open -> resolved');
  });

  it('flags calculations only when outputs change', () => {
    const calc = (amount: number, method: string) => ({
      'calculations/calc.json': { id: 'calc1', type: 't', inputs: {}, outputs: { total: amount }, method },
    });
    const base = buildPackage(baseFiles(calc(100, 'm')));
    const methodOnly = buildPackage(baseFiles(calc(100, 'reworded method')));
    expect(lineFor(diffPackages(base, methodOnly), 'calculation', 'calc1')).toBeUndefined();
    const outputsChanged = buildPackage(baseFiles(calc(200, 'm')));
    expect(lineFor(diffPackages(base, outputsChanged), 'calculation', 'calc1')?.detail).toBe('outputs changed');
  });

  it('details validation status transitions', () => {
    const val = (status: string) => ({
      'validations/validation-report.json': {
        validations: [{ id: 'v1', validator: 'core.partHashes.match', status, ranAt: '2026-07-19T12:00:00Z' }],
      },
    });
    const line = lineFor(
      diffPackages(buildPackage(baseFiles(val('pass'))), buildPackage(baseFiles(val('fail')))),
      'validation',
      'v1'
    );
    expect(line?.detail).toBe('status: pass -> fail');
  });
});
