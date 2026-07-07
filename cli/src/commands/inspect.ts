import { openPackage, readJson, PackageSource } from '../lib/package';

function count(pkg: PackageSource, relPath: string, key: string): number {
  try {
    const doc = readJson(pkg, relPath) as any;
    return Array.isArray(doc?.[key]) ? doc[key].length : 0;
  } catch {
    return 0;
  }
}

export function inspectCommand(input: string): number {
  const pkg = openPackage(input);
  const manifest = (readJson(pkg, 'manifest.json') as any) ?? {};
  const workproduct = (readJson(pkg, 'workproduct.json') as any) ?? {};

  console.log(`Package:    ${pkg.origin}`);
  console.log(`Package ID: ${manifest.packageId ?? 'unknown'}`);
  console.log(`Profile:    ${manifest.profile ?? 'unknown'}`);
  if (manifest.extension) console.log(`Extension:  ${manifest.extension}`);
  console.log(`Title:      ${workproduct.title ?? manifest.title ?? 'unknown'}`);
  if (workproduct.status) console.log(`Status:     ${workproduct.status}${workproduct.reviewState ? ` (${workproduct.reviewState})` : ''}`);
  const subject = workproduct.subject ?? manifest.subject;
  if (subject) console.log(`Subject:    ${JSON.stringify(subject)}`);
  console.log('');
  console.log(`Evidence:     ${count(pkg, 'evidence/evidence.json', 'evidence')}`);
  console.log(`Facts:        ${count(pkg, 'facts/facts.json', 'facts')}`);
  console.log(`Claims:       ${count(pkg, 'claims/claims.json', 'claims')}`);
  console.log(`Judgments:    ${count(pkg, 'judgments/judgments.json', 'judgments')}`);
  console.log(`Approvals:    ${count(pkg, 'approvals/approvals.json', 'approvals')}`);
  console.log(`Calculations: ${pkg.list().filter((p) => p.startsWith('calculations/')).length}`);

  const validations = (readJson(pkg, 'validations/validation-report.json') as any)?.validations;
  if (Array.isArray(validations)) {
    const byStatus = new Map<string, number>();
    for (const v of validations) byStatus.set(v.status, (byStatus.get(v.status) ?? 0) + 1);
    const parts = [...byStatus.entries()].map(([status, n]) => `${n} ${status}`).join(', ');
    console.log(`Validations:  ${validations.length} (${parts})`);
  } else {
    console.log('Validations:  none recorded');
  }

  const renderings = pkg.list().filter((p) => p.startsWith('renderings/'));
  console.log(`Renderings:   ${renderings.length > 0 ? renderings.join(', ') : 'none'}`);
  return 0;
}
