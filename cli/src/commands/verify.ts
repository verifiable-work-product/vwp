import { openPackage } from '../lib/package';
import { verifyPackage, Outcome } from '../lib/validations';

const STATUS_LABEL: Record<Outcome['status'], string> = {
  pass: 'PASS',
  fail: 'FAIL',
  warning: 'WARN',
  reviewRequired: 'REVIEW',
  notRun: 'SKIP',
};

export function verifyCommand(input: string, options: { strict?: boolean }): number {
  const pkg = openPackage(input);
  const report = verifyPackage(pkg);

  console.log('VWP verification report');
  console.log('');
  console.log(`Package: ${report.packageOrigin}`);
  if (report.manifest) {
    console.log(`VWP version: ${report.manifest.vwpVersion ?? 'unknown'}`);
    console.log(`Profile: ${report.manifest.profile ?? 'unknown'}`);
    if (report.manifest.extension) console.log(`Extension: ${report.manifest.extension}`);
  }
  console.log('');
  for (const outcome of report.outcomes) {
    const label = STATUS_LABEL[outcome.status];
    console.log(`${label} ${outcome.id}${outcome.message ? ` - ${outcome.message}` : ''}`);
  }

  const fails = report.outcomes.filter((o) => o.status === 'fail').length;
  const warnings = report.outcomes.filter((o) => o.status === 'warning').length;
  const reviews = report.outcomes.filter((o) => o.status === 'reviewRequired').length;

  let summary: string;
  if (fails > 0) {
    summary = `invalid (${fails} failure${fails === 1 ? '' : 's'})`;
  } else if (reviews > 0) {
    summary = 'valid with review items';
  } else if (warnings > 0) {
    summary = 'valid with warnings';
  } else {
    summary = 'valid';
  }
  console.log('');
  console.log(`Summary: ${summary}`);

  if (fails > 0) return 1;
  if (options.strict && (warnings > 0 || reviews > 0)) return 1;
  return 0;
}
