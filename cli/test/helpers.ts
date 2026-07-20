import AdmZip from 'adm-zip';
import { openPackageBuffer, PackageSource } from '../src/lib/package';

/** Build an in-memory package from a path -> content map. */
export function buildPackage(files: Record<string, string | object | Buffer>): PackageSource {
  const zip = new AdmZip();
  for (const [p, content] of Object.entries(files)) {
    const buf = Buffer.isBuffer(content)
      ? content
      : Buffer.from(typeof content === 'string' ? content : JSON.stringify(content, null, 2));
    zip.addFile(p, buf);
  }
  return openPackageBuffer(zip.toBuffer());
}

/** The minimum valid part set; tests override or extend per case. */
export function baseFiles(overrides: Record<string, string | object | Buffer> = {}): Record<string, string | object | Buffer> {
  return {
    'manifest.json': {
      vwpVersion: '0.1',
      packageId: 'vwp_test_synthetic',
      createdAt: '2026-07-19T12:00:00Z',
      profile: 'vwp-core',
      workproduct: { path: 'workproduct.json', mediaType: 'application/json' },
      provenance: { path: 'provenance/provenance.json', mediaType: 'application/json' },
      parts: [{ path: 'claims/claims.json', mediaType: 'application/json', role: 'claims' }],
    },
    'workproduct.json': {
      id: 'wp_test_synthetic',
      title: 'Synthetic test package',
      domain: 'test',
      profile: 'vwp-core',
      status: 'draft',
      conclusionSummary: 'Synthetic fixture assembled in-memory by tests.',
    },
    'claims/claims.json': { claims: [{ id: 'c1', statement: 'A claim.', status: 'supported' }] },
    'provenance/provenance.json': {
      createdAt: '2026-07-19T12:00:00Z',
      createdBy: { type: 'human', name: 'Test Author' },
    },
    ...overrides,
  };
}

export function statuses(outcomes: { id: string; status: string }[]): Map<string, string> {
  return new Map(outcomes.map((o) => [o.id, o.status]));
}
