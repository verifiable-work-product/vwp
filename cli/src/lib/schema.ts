import Ajv2020 from 'ajv/dist/2020';
import type { ValidateFunction } from 'ajv';
import addFormats from 'ajv-formats';
import * as fs from 'fs';
import * as path from 'path';

export const SCHEMA_NAMES = [
  'manifest',
  'workproduct',
  'evidence',
  'fact',
  'claim',
  'judgment',
  'calculation',
  'validation',
  'approval',
  'provenance',
  'signature',
] as const;

export type SchemaName = (typeof SCHEMA_NAMES)[number];

/**
 * Locate schemas/v0.1 by walking up from this file, so the CLI works from
 * a repo checkout without a build-time copy step. VWP_SCHEMAS_DIR overrides.
 */
export function findSchemasDir(): string {
  const override = process.env.VWP_SCHEMAS_DIR;
  if (override) {
    if (!fs.existsSync(path.join(override, 'manifest.schema.json'))) {
      throw new Error(`VWP_SCHEMAS_DIR does not contain manifest.schema.json: ${override}`);
    }
    return override;
  }
  let dir = __dirname;
  for (let i = 0; i < 8; i++) {
    const candidate = path.join(dir, 'schemas', 'v0.1');
    if (fs.existsSync(path.join(candidate, 'manifest.schema.json'))) return candidate;
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  throw new Error('could not locate schemas/v0.1 (set VWP_SCHEMAS_DIR to override)');
}

let validators: Map<SchemaName, ValidateFunction> | undefined;

export function getValidator(name: SchemaName): ValidateFunction {
  if (!validators) {
    const dir = findSchemasDir();
    const ajv = new Ajv2020({ allErrors: true, strict: false });
    addFormats(ajv);
    validators = new Map();
    for (const schemaName of SCHEMA_NAMES) {
      const raw = fs.readFileSync(path.join(dir, `${schemaName}.schema.json`), 'utf8');
      validators.set(schemaName, ajv.compile(JSON.parse(raw)));
    }
  }
  const fn = validators.get(name);
  if (!fn) throw new Error(`unknown schema: ${name}`);
  return fn;
}

/** Validate data against a named schema; returns error strings (empty = valid). */
export function schemaErrors(name: SchemaName, data: unknown): string[] {
  const validate = getValidator(name);
  if (validate(data)) return [];
  return (validate.errors ?? []).map(
    (e) => `${e.instancePath || '(root)'} ${e.message ?? 'invalid'}`
  );
}
