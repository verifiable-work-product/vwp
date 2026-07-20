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
  'analysis-attestation',
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

let ajvInstance: Ajv2020 | undefined;
let validators: Map<SchemaName, ValidateFunction> | undefined;
const extensionValidators = new Map<string, ValidateFunction>();

function getAjv(): Ajv2020 {
  if (!ajvInstance) {
    ajvInstance = new Ajv2020({ allErrors: true, strict: false });
    addFormats(ajvInstance);
  }
  return ajvInstance;
}

export function getValidator(name: SchemaName): ValidateFunction {
  if (!validators) {
    const dir = findSchemasDir();
    const ajv = getAjv();
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

function formatErrors(validate: ValidateFunction): string[] {
  return (validate.errors ?? []).map(
    (e) => `${e.instancePath || '(root)'} ${e.message ?? 'invalid'}`
  );
}

/** Validate data against a named core schema; returns error strings (empty = valid). */
export function schemaErrors(name: SchemaName, data: unknown): string[] {
  const validate = getValidator(name);
  if (validate(data)) return [];
  return formatErrors(validate);
}

/**
 * Validate data against an extension schema, addressed by its path relative
 * to schemas/v0.1 (e.g. 'extensions/accounting-asc606/ssp-allocation.schema.json').
 * Extension schemas constrain data the core schemas leave open; core schemas
 * still apply where the object is also a core object.
 */
export function extensionSchemaErrors(relPath: string, data: unknown): string[] {
  let validate = extensionValidators.get(relPath);
  if (!validate) {
    const file = path.join(findSchemasDir(), ...relPath.split('/'));
    if (!fs.existsSync(file)) {
      throw new Error(`extension schema not found: ${relPath}`);
    }
    validate = getAjv().compile(JSON.parse(fs.readFileSync(file, 'utf8')));
    extensionValidators.set(relPath, validate);
  }
  if (validate(data)) return [];
  return formatErrors(validate);
}
