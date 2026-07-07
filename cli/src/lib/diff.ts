import { PackageSource, readJson } from './package';

export interface DiffLine {
  kind: 'workproduct' | 'evidence' | 'claim' | 'judgment' | 'calculation' | 'validation';
  change: 'added' | 'removed' | 'changed';
  id: string;
  detail?: string;
}

function ids(items: any[]): Map<string, any> {
  const map = new Map<string, any>();
  for (const item of items) if (item?.id) map.set(item.id, item);
  return map;
}

function loadCollection(pkg: PackageSource, relPath: string, key: string): any[] {
  try {
    const doc = readJson(pkg, relPath) as any;
    return Array.isArray(doc?.[key]) ? doc[key] : [];
  } catch {
    return [];
  }
}

function loadCalculations(pkg: PackageSource): any[] {
  return pkg
    .list()
    .filter((p) => p.startsWith('calculations/') && p.endsWith('.json'))
    .map((p) => {
      try {
        return readJson(pkg, p);
      } catch {
        return undefined;
      }
    })
    .filter((c: any) => c?.id);
}

function diffById(
  lines: DiffLine[],
  kind: DiffLine['kind'],
  oldItems: any[],
  newItems: any[],
  changed?: (oldItem: any, newItem: any) => string | undefined
): void {
  const oldMap = ids(oldItems);
  const newMap = ids(newItems);
  for (const id of oldMap.keys()) {
    if (!newMap.has(id)) lines.push({ kind, change: 'removed', id });
  }
  for (const [id, newItem] of newMap) {
    const oldItem = oldMap.get(id);
    if (!oldItem) {
      lines.push({ kind, change: 'added', id });
    } else if (changed) {
      const detail = changed(oldItem, newItem);
      if (detail) lines.push({ kind, change: 'changed', id, detail });
    } else if (JSON.stringify(oldItem) !== JSON.stringify(newItem)) {
      lines.push({ kind, change: 'changed', id });
    }
  }
}

/** Structural diff between two packages (v0.1: ID-level adds/removes/changes). */
export function diffPackages(oldPkg: PackageSource, newPkg: PackageSource): DiffLine[] {
  const lines: DiffLine[] = [];

  const oldWp = (readJson(oldPkg, 'workproduct.json') as any) ?? {};
  const newWp = (readJson(newPkg, 'workproduct.json') as any) ?? {};
  const wpFields = new Set([...Object.keys(oldWp), ...Object.keys(newWp)]);
  for (const field of wpFields) {
    if (JSON.stringify(oldWp[field]) !== JSON.stringify(newWp[field])) {
      lines.push({
        kind: 'workproduct',
        change: 'changed',
        id: field,
        detail: `${JSON.stringify(oldWp[field])} -> ${JSON.stringify(newWp[field])}`,
      });
    }
  }

  diffById(lines, 'evidence', loadCollection(oldPkg, 'evidence/evidence.json', 'evidence'), loadCollection(newPkg, 'evidence/evidence.json', 'evidence'));
  diffById(lines, 'claim', loadCollection(oldPkg, 'claims/claims.json', 'claims'), loadCollection(newPkg, 'claims/claims.json', 'claims'));
  diffById(
    lines,
    'judgment',
    loadCollection(oldPkg, 'judgments/judgments.json', 'judgments'),
    loadCollection(newPkg, 'judgments/judgments.json', 'judgments'),
    (a, b) => {
      const changes: string[] = [];
      if (a.proposedConclusion !== b.proposedConclusion) changes.push(`conclusion: ${JSON.stringify(a.proposedConclusion)} -> ${JSON.stringify(b.proposedConclusion)}`);
      if (a.status !== b.status) changes.push(`status: ${a.status} -> ${b.status}`);
      return changes.length > 0 ? changes.join('; ') : undefined;
    }
  );
  diffById(lines, 'calculation', loadCalculations(oldPkg), loadCalculations(newPkg), (a, b) =>
    JSON.stringify(a.outputs) !== JSON.stringify(b.outputs) ? 'outputs changed' : undefined
  );
  diffById(
    lines,
    'validation',
    loadCollection(oldPkg, 'validations/validation-report.json', 'validations'),
    loadCollection(newPkg, 'validations/validation-report.json', 'validations'),
    (a, b) => (a.status !== b.status ? `status: ${a.status} -> ${b.status}` : undefined)
  );

  return lines;
}
