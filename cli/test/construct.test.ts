import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { execFileSync } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import {
  addCalculationCommand,
  addClaimCommand,
  addEvidenceCommand,
  addFactCommand,
  buildCommand,
  embedCommand,
  initCommand,
  setReadableCommand,
} from '../src/commands/construct';
import { openPackage } from '../src/lib/package';
import { verifyPackage } from '../src/lib/validations';

const CLI = path.resolve(__dirname, '..', 'dist', 'index.js');
let tmp: string;

beforeEach(() => {
  tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-construct-'));
  vi.spyOn(console, 'log').mockImplementation(() => {});
  vi.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
  fs.rmSync(tmp, { recursive: true, force: true });
  vi.restoreAllMocks();
});

describe('construction commands', () => {
  it('constructs, hashes, packs, and verifies a complete package', () => {
    const packageDir = path.join(tmp, 'nyc-allocation');
    expect(initCommand(packageDir, {
      title: 'NYC Revenue Allocation',
      packageId: 'pkg_nyc_allocation',
      workproductId: 'wp_nyc_allocation',
      profile: 'vwp-core',
      domain: 'tax',
      createdBy: 'Test Preparer',
    })).toBe(0);

    const initialManifest = JSON.parse(fs.readFileSync(path.join(packageDir, 'manifest.json'), 'utf8'));
    expect(initialManifest.parts).toEqual(expect.arrayContaining([
      expect.objectContaining({ path: 'renderings/readable.md', mediaType: 'text/markdown', role: 'readable' }),
    ]));
    expect(verifyPackage(openPackage(packageDir)).valid).toBe(true);

    const evidenceFile = path.join(tmp, 'revenue.csv');
    fs.writeFileSync(evidenceFile, 'region,revenue\nNYC,125000\n');
    expect(addEvidenceCommand(packageDir, {
      id: 'ev_revenue',
      title: 'Revenue by region',
      type: 'dataset',
      file: evidenceFile,
    })).toBe(0);
    expect(addFactCommand(packageDir, {
      id: 'f_nyc_revenue',
      statement: 'NYC revenue was 125000 USD.',
      evidence: ['ev_revenue'],
      value: '{"amount":125000,"currency":"USD"}',
    })).toBe(0);
    expect(addCalculationCommand(packageDir, {
      id: 'calc_nyc_share',
      type: 'ratio',
      inputs: '{"nycRevenue":125000,"totalRevenue":500000}',
      outputs: '{"nycShare":0.25}',
      method: 'NYC revenue divided by total revenue',
      recomputable: true,
    })).toBe(0);
    expect(addClaimCommand(packageDir, {
      id: 'c_nyc_share',
      statement: 'NYC revenue represents 25% of total revenue.',
      fact: ['f_nyc_revenue'],
      calculation: ['calc_nyc_share'],
      status: 'supported',
    })).toBe(0);

    const memo = path.join(tmp, 'memo.md');
    fs.writeFileSync(memo, '# NYC Revenue Allocation\n\nNYC revenue represents **25%** of total revenue.\n');
    expect(setReadableCommand(packageDir, memo, {})).toBe(0);

    const workbook = path.join(tmp, 'allocation.xlsx');
    fs.writeFileSync(workbook, 'synthetic workbook bytes');
    expect(embedCommand(packageDir, workbook, {})).toBe(0);

    const output = path.join(tmp, 'nyc-allocation.vwp');
    expect(buildCommand(packageDir, output, { verify: true })).toBe(0);
    expect(fs.existsSync(output)).toBe(true);
    const report = verifyPackage(openPackage(output));
    expect(report.valid).toBe(true);
    expect(report.outcomes.filter((outcome) => outcome.status === 'fail')).toEqual([]);

    const manifest = JSON.parse(fs.readFileSync(path.join(packageDir, 'manifest.json'), 'utf8'));
    expect(manifest.parts).toEqual(expect.arrayContaining([
      expect.objectContaining({ path: 'evidence/revenue.csv', role: 'evidence', sha256: expect.stringMatching(/^[a-f0-9]{64}$/) }),
      expect.objectContaining({ path: 'facts/facts.json', role: 'facts', sha256: expect.stringMatching(/^[a-f0-9]{64}$/) }),
      expect.objectContaining({ path: 'calculations/calc_nyc_share.json', role: 'calculation' }),
      expect.objectContaining({ path: 'artifacts/allocation.xlsx', role: 'primary-artifact' }),
    ]));
  });

  it('is idempotent for identical objects and rejects conflicts or unknown support', () => {
    const packageDir = path.join(tmp, 'package');
    initCommand(packageDir, { title: 'Conflict test' });
    addEvidenceCommand(packageDir, {
      id: 'ev_external',
      title: 'External record',
      externalRef: 'record-123',
      locatorType: 'opaque',
    });
    expect(addEvidenceCommand(packageDir, {
      id: 'ev_external',
      title: 'External record',
      externalRef: 'record-123',
      locatorType: 'opaque',
    })).toBe(0);
    expect(() => addEvidenceCommand(packageDir, {
      id: 'ev_external',
      title: 'Changed record',
      externalRef: 'record-123',
    })).toThrow(/already exists with different content/);
    expect(() => addFactCommand(packageDir, {
      id: 'f_unknown',
      statement: 'Unsupported fact.',
      evidence: ['ev_missing'],
    })).toThrow(/unknown evidence id/);
  });

  it('rejects unsafe package paths and non-empty init targets', () => {
    const occupied = path.join(tmp, 'occupied');
    fs.mkdirSync(occupied);
    fs.writeFileSync(path.join(occupied, 'keep.txt'), 'do not overwrite');
    expect(() => initCommand(occupied, { title: 'Unsafe' })).toThrow(/non-empty/);

    const packageDir = path.join(tmp, 'package');
    initCommand(packageDir, { title: 'Safe package' });
    const artifact = path.join(tmp, 'artifact.pdf');
    fs.writeFileSync(artifact, 'pdf');
    expect(() => embedCommand(packageDir, artifact, { path: '../artifact.pdf' })).toThrow(/unsafe package path/);
    expect(() => buildCommand(packageDir, path.join(packageDir, 'nested.vwp'), {})).toThrow(/outside the package directory/);
  });

  it('wires init and build through the packaged CLI', () => {
    const packageDir = path.join(tmp, 'cli-package');
    const output = path.join(tmp, 'cli-package.vwp');
    execFileSync('node', [CLI, 'init', packageDir, '--title', 'CLI Package', '--created-by', 'CLI Test']);
    execFileSync('node', [CLI, 'build', packageDir, output]);
    expect(verifyPackage(openPackage(output)).valid).toBe(true);
  });
});
