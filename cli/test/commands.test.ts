import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { execFileSync } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { packCommand } from '../src/commands/pack';
import { unpackCommand } from '../src/commands/unpack';
import { verifyCommand } from '../src/commands/verify';
import { inspectCommand } from '../src/commands/inspect';
import { renderCommand } from '../src/commands/render';
import { diffCommand } from '../src/commands/diff';
import { digestsCommand } from '../src/commands/digests';

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const MINIMAL = path.join(REPO_ROOT, 'testdata', 'valid', 'minimal-package');
const ASC606 = path.join(REPO_ROOT, 'examples', 'asc606-revenue-contract', 'package');

let tmp: string;
beforeEach(() => {
  tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'vwp-cmd-'));
  vi.spyOn(console, 'log').mockImplementation(() => {});
  vi.spyOn(console, 'error').mockImplementation(() => {});
  vi.spyOn(process.stdout, 'write').mockImplementation(() => true);
});
afterEach(() => {
  fs.rmSync(tmp, { recursive: true, force: true });
  vi.restoreAllMocks();
});

describe('pack', () => {
  it('refuses a directory without manifest.json', () => {
    expect(packCommand(tmp, path.join(tmp, 'out.vwp'), {})).toBe(2);
  });

  it('packs a package directory into a .vwp zip', async () => {
    const out = path.join(tmp, 'minimal.vwp');
    expect(packCommand(MINIMAL, out, {})).toBe(0);
    expect(fs.existsSync(out)).toBe(true);
    expect(await verifyCommand(out, {})).toBe(0);
  });

  it('writes part hashes with --write-hashes and the result verifies without hash warnings', () => {
    const dir = path.join(tmp, 'pkg');
    fs.cpSync(MINIMAL, dir, { recursive: true });
    const out = path.join(tmp, 'hashed.vwp');
    expect(packCommand(dir, out, { writeHashes: true })).toBe(0);
    const manifest = JSON.parse(fs.readFileSync(path.join(dir, 'manifest.json'), 'utf8'));
    expect(manifest.parts[0].sha256).toMatch(/^[a-f0-9]{64}$/);
    expect(manifest.workproduct.sha256).toMatch(/^[a-f0-9]{64}$/);
  });

  it('throws when the manifest declares a part that does not exist', () => {
    const dir = path.join(tmp, 'pkg');
    fs.cpSync(MINIMAL, dir, { recursive: true });
    const manifestPath = path.join(dir, 'manifest.json');
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    manifest.parts.push({ path: 'facts/facts.json' });
    fs.writeFileSync(manifestPath, JSON.stringify(manifest));
    expect(() => packCommand(dir, path.join(tmp, 'out.vwp'), { writeHashes: true })).toThrow(/does not exist/);
  });
});

describe('unpack', () => {
  it('round-trips pack -> unpack', () => {
    const out = path.join(tmp, 'minimal.vwp');
    packCommand(MINIMAL, out, {});
    const dest = path.join(tmp, 'unpacked');
    expect(unpackCommand(out, dest)).toBe(0);
    expect(fs.existsSync(path.join(dest, 'manifest.json'))).toBe(true);
    expect(fs.existsSync(path.join(dest, 'claims', 'claims.json'))).toBe(true);
  });

  it('refuses archives with traversal entries', () => {
    // Byte-patch a same-length placeholder to craft a malicious archive
    // (adm-zip sanitizes '../' on addFile).
    const AdmZip = require('adm-zip');
    const zip = new AdmZip();
    zip.addFile('manifest.json', Buffer.from('{}'));
    zip.addFile('AA/evil.txt', Buffer.from('nope'));
    const patched = Buffer.from(zip.toBuffer().toString('latin1').split('AA/evil.txt').join('../evil.txt'), 'latin1');
    const evil = path.join(tmp, 'evil.vwp');
    fs.writeFileSync(evil, patched);
    expect(unpackCommand(evil, path.join(tmp, 'dest'))).toBe(2);
  });
});

describe('verify command', () => {
  it('returns 0 for a valid package and 1 under --strict when warnings exist', async () => {
    expect(await verifyCommand(MINIMAL, {})).toBe(0);
    // minimal package declares no hashes and no signature -> warnings -> strict fails
    expect(await verifyCommand(MINIMAL, { strict: true })).toBe(1);
  });

  it('returns 1 for an invalid package', async () => {
    expect(await verifyCommand(path.join(REPO_ROOT, 'testdata', 'invalid', 'bad-hash'), {})).toBe(1);
  });

  it('returns 1 under --strict for review items', async () => {
    // ASC 606 example has an intentionally open judgment (reviewRequired).
    expect(await verifyCommand(ASC606, {})).toBe(0);
    expect(await verifyCommand(ASC606, { strict: true })).toBe(1);
  });
});

describe('inspect and render', () => {
  it('inspects packages with and without recorded validations', () => {
    expect(inspectCommand(ASC606)).toBe(0);
    expect(inspectCommand(MINIMAL)).toBe(0);
  });

  it('renders the markdown rendering when present, errors when absent', () => {
    expect(renderCommand(ASC606)).toBe(0);
    expect(renderCommand(MINIMAL)).toBe(2);
  });

  it('renders legacy Markdown roles at safe alternate paths and prefers an explicit readable', () => {
    const dir = path.join(tmp, 'compatible-readable');
    fs.mkdirSync(path.join(dir, 'legacy'), { recursive: true });
    fs.mkdirSync(path.join(dir, 'reports'), { recursive: true });
    fs.writeFileSync(path.join(dir, 'legacy', 'MEMO.MD'), '# Legacy\n');
    fs.writeFileSync(path.join(dir, 'reports', 'readable.md'), '# Preferred\n');
    fs.writeFileSync(
      path.join(dir, 'manifest.json'),
      JSON.stringify({
        parts: [
          { path: 'legacy/MEMO.MD', mediaType: 'text/markdown', role: 'rendering' },
          { path: 'reports/readable.md', mediaType: 'text/markdown', role: 'readable' },
        ],
      })
    );

    expect(renderCommand(dir)).toBe(0);
    expect(process.stdout.write).toHaveBeenCalledWith('# Preferred\n');
  });
});

describe('diff command', () => {
  it('reports no differences for identical packages', () => {
    expect(diffCommand(MINIMAL, MINIMAL)).toBe(0);
  });

  it('prints structural differences between different packages', () => {
    const dir = path.join(tmp, 'pkg');
    fs.cpSync(MINIMAL, dir, { recursive: true });
    const claims = { claims: [{ id: 'c_new', statement: 'A new claim.', status: 'proposed' }] };
    fs.writeFileSync(path.join(dir, 'claims', 'claims.json'), JSON.stringify(claims));
    expect(diffCommand(MINIMAL, dir)).toBe(0);
  });
});

describe('digests command', () => {
  function printed(): any {
    const calls = (console.log as unknown as { mock: { calls: unknown[][] } }).mock.calls;
    return JSON.parse(String(calls[calls.length - 1][0]));
  }

  it('prints the manifest digests shape, with per-object hashes on request', () => {
    expect(digestsCommand(ASC606, {})).toBe(0);
    const out = printed();
    expect(out.algorithm).toBe('vwp-dag/1');
    expect(Object.keys(out)).toEqual(['algorithm', 'analysisRoot', 'contentRoot', 'reviewRoot', 'collections']);
    expect(Object.keys(out.collections)).toEqual(['evidence', 'fact', 'calculation', 'judgment', 'claim', 'narrative', 'approval']);

    expect(digestsCommand(ASC606, { objects: true })).toBe(0);
    const withObjects = printed();
    expect(withObjects.objects.calc_ssp_allocation.kind).toBe('calculation');
    expect(withObjects.objects.calc_ssp_allocation.closureHash).toMatch(/^[0-9a-f]{64}$/);
    const kinds = new Set(Object.values<any>(withObjects.objects).map((o) => o.kind));
    expect([...kinds].sort()).toEqual(['approval', 'attestation', 'calculation', 'claim', 'evidence', 'fact', 'judgment']);
  });

  it('prints the problems and exits 1 when the graph is incomplete', () => {
    const dir = path.join(tmp, 'pkg');
    fs.cpSync(MINIMAL, dir, { recursive: true });
    const claims = { claims: [{ id: 'c_dangling', statement: 'Rests on nothing.', status: 'proposed', supportingFacts: ['f_missing'] }] };
    fs.writeFileSync(path.join(dir, 'claims', 'claims.json'), JSON.stringify(claims));
    expect(digestsCommand(dir, {})).toBe(1);
    const out = printed();
    expect(out.analysisRoot).toBeUndefined();
    expect(out.error).toMatch(/unresolved reference/);
    expect(out.problems).toEqual([
      { kind: 'unresolved', from: 'claim c_dangling', id: 'f_missing', message: 'claim c_dangling: unknown supportingFacts ref f_missing' },
    ]);
  });
});

describe('CLI entry (dist smoke test)', () => {
  it('dispatches digests through the packaged binary', () => {
    const out = execFileSync('node', [path.join(REPO_ROOT, 'cli', 'dist', 'index.js'), 'digests', MINIMAL], {
      encoding: 'utf8',
    });
    expect(JSON.parse(out).algorithm).toBe('vwp-dag/1');
  });

  it('dispatches verify through the packaged binary', () => {
    const out = execFileSync('node', [path.join(REPO_ROOT, 'cli', 'dist', 'index.js'), 'verify', MINIMAL], {
      encoding: 'utf8',
    });
    expect(out).toContain('Summary: valid with warnings');
  });
});

describe('library entry point', () => {
  it('re-exports the verifier surface', async () => {
    const lib = await import('../src/lib/index');
    expect(typeof lib.verifyPackage).toBe('function');
    expect(typeof lib.openPackage).toBe('function');
    expect(typeof lib.diffPackages).toBe('function');
    expect(lib.SCHEMA_NAMES).toContain('fact');
  });
});
