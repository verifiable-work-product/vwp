import { describe, expect, it } from 'vitest';
import AdmZip from 'adm-zip';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { openPackage, isUnsafePath } from '../src/lib/package';
import { verifyPackage } from '../src/lib/validations';

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const EXAMPLES = path.join(REPO_ROOT, 'examples');
const TESTDATA = path.join(REPO_ROOT, 'testdata');

function statuses(report: ReturnType<typeof verifyPackage>): Map<string, string> {
  return new Map(report.outcomes.map((o) => [o.id, o.status]));
}

describe('verifyPackage on example packages', () => {
  it('accepts the ASC 606 example with review items', () => {
    const pkg = openPackage(path.join(EXAMPLES, 'asc606-revenue-contract', 'package'));
    const report = verifyPackage(pkg);
    expect(report.valid).toBe(true);
    const byId = statuses(report);
    expect(byId.get('core.manifest.schemaValid')).toBe('pass');
    expect(byId.get('core.partHashes.match')).toBe('pass');
    expect(byId.get('asc606.allocation.schemaValid')).toBe('pass');
    expect(byId.get('asc606.allocation.tieOut')).toBe('pass');
    expect(byId.get('asc606.revenueSchedule.total')).toBe('pass');
    // The implementation-services judgment is intentionally left in review.
    expect(byId.get('finance.requiredJudgments.complete')).toBe('reviewRequired');
  });

  it('accepts the wallet ownership example', () => {
    const pkg = openPackage(path.join(EXAMPLES, 'digital-assets-wallet-ownership', 'package'));
    const report = verifyPackage(pkg);
    expect(report.valid).toBe(true);
    const byId = statuses(report);
    expect(byId.get('digitalAssets.wallets.schemaValid')).toBe('pass');
    expect(byId.get('digitalAssets.wallets.entityMapping')).toBe('pass');
    expect(byId.get('digitalAssets.wallets.proofOfControl')).toBe('pass');
    expect(byId.get('digitalAssets.wallets.exclusionRationale')).toBe('pass');
    expect(byId.get('digitalAssets.wallets.balanceEvidence')).toBe('pass');
  });

  it('accepts the ASC 820 fair value example', () => {
    const pkg = openPackage(path.join(EXAMPLES, 'asc820-fair-value', 'package'));
    const report = verifyPackage(pkg);
    expect(report.valid).toBe(true);
    const byId = statuses(report);
    expect(byId.get('asc820.measurements.schemaValid')).toBe('pass');
    expect(byId.get('asc820.pricingSources.resolve')).toBe('pass');
    expect(byId.get('asc820.level3.judgmentLinked')).toBe('pass');
  });

  it('accepts the minimal valid package from testdata', () => {
    const report = verifyPackage(openPackage(path.join(TESTDATA, 'valid', 'minimal-package')));
    expect(report.valid).toBe(true);
  });
});

describe('verifyPackage on invalid packages', () => {
  it('fails on a declared hash mismatch', () => {
    const report = verifyPackage(openPackage(path.join(TESTDATA, 'invalid', 'bad-hash')));
    expect(report.valid).toBe(false);
    expect(statuses(report).get('core.partHashes.match')).toBe('fail');
  });

  it('fails when required parts are missing', () => {
    const report = verifyPackage(openPackage(path.join(TESTDATA, 'invalid', 'missing-provenance')));
    expect(report.valid).toBe(false);
    expect(statuses(report).get('core.requiredParts.exist')).toBe('fail');
  });

  it('fails on a schema-invalid workproduct', () => {
    const report = verifyPackage(openPackage(path.join(TESTDATA, 'invalid', 'bad-workproduct-schema')));
    expect(report.valid).toBe(false);
    expect(statuses(report).get('core.workproduct.schemaValid')).toBe('fail');
  });
});

describe('ZIP path safety', () => {
  it('classifies traversal and absolute paths as unsafe', () => {
    expect(isUnsafePath('../evil.txt')).toBe(true);
    expect(isUnsafePath('a/../../evil.txt')).toBe(true);
    expect(isUnsafePath('/etc/passwd')).toBe(true);
    expect(isUnsafePath('C:evil.txt')).toBe(true);
    expect(isUnsafePath('a\\b.txt')).toBe(true);
    expect(isUnsafePath('manifest.json')).toBe(false);
    expect(isUnsafePath('claims/claims.json')).toBe(false);
  });

  it('rejects traversal entries when reading a .vwp file', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-test-'));
    const zipPath = path.join(dir, 'evil.vwp');
    const zip = new AdmZip();
    zip.addFile('manifest.json', Buffer.from('{}'));
    // adm-zip sanitizes '../' on addFile, so byte-patch a same-length
    // placeholder to craft a genuinely malicious archive.
    zip.addFile('AA/evil.txt', Buffer.from('nope'));
    const patched = Buffer.from(
      zip.toBuffer().toString('latin1').split('AA/evil.txt').join('../evil.txt'),
      'latin1'
    );
    fs.writeFileSync(zipPath, patched);

    const pkg = openPackage(zipPath);
    expect(pkg.unsafeEntries).toContain('../evil.txt');
    expect(pkg.has('../evil.txt')).toBe(false);
    const report = verifyPackage(pkg);
    expect(statuses(report).get('core.package.safePaths')).toBe('fail');
    fs.rmSync(dir, { recursive: true, force: true });
  });
});
