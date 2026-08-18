import { describe, expect, it } from 'vitest';
import Ajv2020 from 'ajv/dist/2020';
import addFormats from 'ajv-formats';
import * as fs from 'fs';
import * as path from 'path';

const schema = JSON.parse(
  fs.readFileSync(path.resolve(__dirname, '..', '..', 'schemas', 'v0.2', 'manifest.schema.json'), 'utf8')
);
const ajv = new Ajv2020({ strict: false });
addFormats(ajv);
const validate = ajv.compile(schema);

const digest = 'a'.repeat(64);

function validManifest(): any {
  return {
    vwpVersion: '0.2',
    packageId: 'pkg_nyc_allocation',
    createdAt: '2026-08-06T12:00:00Z',
    profile: 'vwp-finance',
    workproduct: {
      path: 'workproduct.json',
      mediaType: 'application/json',
      sha256: digest,
      role: 'workproduct',
    },
    provenance: {
      path: 'provenance/provenance.json',
      mediaType: 'application/json',
      sha256: digest,
      role: 'provenance',
    },
    parts: [
      {
        path: 'renderings/nyc-revenue-allocation.md',
        mediaType: 'text/markdown',
        sha256: digest,
        role: 'readable',
      },
    ],
  };
}

describe('proposed v0.2 manifest schema', () => {
  it('requires a Markdown readable part and accepts the preferred role', () => {
    expect(validate(validManifest())).toBe(true);

    const missing = validManifest();
    missing.parts = [];
    expect(validate(missing)).toBe(false);

    const duplicate = validManifest();
    duplicate.parts.push({ ...duplicate.parts[0], path: 'renderings/appendix.md' });
    expect(validate(duplicate)).toBe(false);
  });

  it('accepts legacy rendering roles, including multiple Markdown renderings', () => {
    const legacy = validManifest();
    legacy.parts[0].role = 'rendering';
    expect(validate(legacy)).toBe(true);

    legacy.parts.push({ ...legacy.parts[0], path: 'renderings/appendix.md' });
    expect(validate(legacy)).toBe(true);
  });

  it('allows legacy renderings alongside one explicit readable', () => {
    const value = validManifest();
    value.parts.push({
      path: 'renderings/legacy-report.html',
      mediaType: 'text/html',
      sha256: digest,
      role: 'rendering',
    });
    expect(validate(value)).toBe(true);
  });

  it.each([
    ['renderings/allocation.html', 'text/html'],
    ['renderings/allocation.pdf', 'application/pdf'],
    ['renderings/allocation', 'text/markdown'],
    ['renderings/../allocation.md', 'text/markdown'],
  ])('rejects readable path %s with media type %s', (readablePath, mediaType) => {
    const value = validManifest();
    value.parts[0].path = readablePath;
    value.parts[0].mediaType = mediaType;
    expect(validate(value)).toBe(false);
  });

  it('accepts legacy readable placement and extension casing', () => {
    const value = validManifest();
    value.parts[0].path = 'legacy/allocation.MD';
    expect(validate(value)).toBe(true);
  });

  it('does not treat a non-Markdown legacy rendering as the readable', () => {
    const value = validManifest();
    value.parts[0].role = 'rendering';
    value.parts[0].path = 'renderings/allocation.html';
    value.parts[0].mediaType = 'text/html';
    expect(validate(value)).toBe(false);
  });

  it('accepts embedded native artifacts without adding legacy cardinality or placement failures', () => {
    const value = validManifest();
    value.parts.push({
      path: 'artifacts/nyc-revenue-allocation.xlsx',
      mediaType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      sha256: digest,
      role: 'primary-artifact',
    });
    value.parts.push({
      path: 'legacy/nyc-revenue-support.pdf',
      mediaType: 'application/pdf',
      sha256: digest,
      role: 'primary-artifact',
    });
    expect(validate(value)).toBe(true);
  });
});
