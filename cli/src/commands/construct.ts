import { randomUUID } from 'crypto';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { sha256Hex } from '../lib/hashes';
import { isUnsafePath, openPackage } from '../lib/package';
import { schemaErrors, SchemaName } from '../lib/schema';
import { verifyPackage } from '../lib/validations';
import { packCommand } from './pack';

interface PartRef {
  path: string;
  mediaType: string;
  sha256?: string;
  role?: string;
}

interface Manifest {
  vwpVersion: string;
  packageId: string;
  createdAt: string;
  profile: string;
  extension?: string;
  title?: string;
  workproduct: PartRef;
  provenance: PartRef;
  parts: PartRef[];
  signatures?: PartRef[];
}

export interface InitOptions {
  title?: string;
  packageId?: string;
  workproductId?: string;
  profile?: string;
  extension?: string;
  domain?: string;
  status?: string;
  createdBy?: string;
  creatorType?: string;
}

export interface EvidenceOptions {
  json?: string;
  id?: string;
  title?: string;
  type?: string;
  file?: string;
  path?: string;
  externalRef?: string;
  locatorType?: string;
  sourceSystem?: string;
}

export interface FactOptions {
  json?: string;
  id?: string;
  statement?: string;
  evidence?: string[] | string;
  factType?: string;
  value?: string;
  confidence?: string;
  requiresReview?: boolean;
}

export interface ClaimOptions {
  json?: string;
  id?: string;
  statement?: string;
  fact?: string[] | string;
  evidence?: string[] | string;
  calculation?: string[] | string;
  judgment?: string[] | string;
  assertion?: string[] | string;
  conclusion?: string;
  status?: string;
  requiresReview?: boolean;
}

export interface CalculationOptions {
  json?: string;
  id?: string;
  type?: string;
  inputs?: string;
  outputs?: string;
  method?: string;
  validator?: string;
  recomputable?: boolean;
}

export interface ReadableOptions {
  path?: string;
}

export interface EmbedOptions {
  path?: string;
  mediaType?: string;
  role?: string;
}

export interface BuildOptions {
  verify?: boolean;
}

function writeJson(file: string, value: unknown): void {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const temporary = `${file}.tmp-${process.pid}-${randomUUID()}`;
  fs.writeFileSync(temporary, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
  fs.renameSync(temporary, file);
}

function canonicalJson(value: any): any {
  if (Array.isArray(value)) return value.map(canonicalJson);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonicalJson(value[key])]));
  }
  return value;
}

function jsonEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(canonicalJson(left)) === JSON.stringify(canonicalJson(right));
}

function readJsonObject(file: string, label: string): any {
  let value: any;
  try {
    value = JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (error) {
    throw new Error(`${label} is not valid JSON: ${error instanceof Error ? error.message : String(error)}`);
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${label} must contain a JSON object`);
  }
  return value;
}

function validateObject(schema: SchemaName, value: unknown): void {
  const errors = schemaErrors(schema, value);
  if (errors.length > 0) throw new Error(`${schema} schema violation: ${errors.join('; ')}`);
}

function packageRoot(directory: string): string {
  if (!fs.existsSync(directory) || !fs.statSync(directory).isDirectory()) {
    throw new Error(`package directory does not exist: ${directory}`);
  }
  const root = fs.realpathSync(directory);
  if (!fs.existsSync(path.join(root, 'manifest.json'))) {
    throw new Error(`${directory} does not contain manifest.json; run vwp init first`);
  }
  return root;
}

function packagePath(root: string, relPath: string): string {
  if (isUnsafePath(relPath) || relPath.endsWith('/')) {
    throw new Error(`unsafe package path: ${relPath}`);
  }
  let current = root;
  for (const segment of relPath.split('/')) {
    current = path.join(current, segment);
    if (fs.existsSync(current) && fs.lstatSync(current).isSymbolicLink()) {
      throw new Error(`package path traverses a symbolic link: ${relPath}`);
    }
  }
  return path.join(root, ...relPath.split('/'));
}

function readManifest(root: string): Manifest {
  return readJsonObject(path.join(root, 'manifest.json'), 'manifest.json') as Manifest;
}

function saveManifest(root: string, manifest: Manifest): void {
  validateObject('manifest', manifest);
  writeJson(path.join(root, 'manifest.json'), manifest);
}

function allPartRefs(manifest: Manifest): PartRef[] {
  return [manifest.workproduct, manifest.provenance, ...(manifest.parts ?? []), ...(manifest.signatures ?? [])];
}

function upsertPart(manifest: Manifest, ref: PartRef): void {
  manifest.parts ??= [];
  const existing = manifest.parts.find((part) => part.path === ref.path);
  if (existing) {
    Object.assign(existing, ref);
  } else {
    manifest.parts.push(ref);
  }
  manifest.parts.sort((left, right) => left.path.localeCompare(right.path));
}

function hashedRef(relPath: string, mediaType: string, role: string, bytes: Buffer): PartRef {
  return { path: relPath, mediaType, sha256: sha256Hex(bytes), role };
}

function putBytes(root: string, relPath: string, bytes: Buffer, replace = false): void {
  const destination = packagePath(root, relPath);
  if (fs.existsSync(destination)) {
    const existing = fs.readFileSync(destination);
    if (existing.equals(bytes)) return;
    if (!replace) throw new Error(`package file already exists with different content: ${relPath}`);
  }
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  fs.writeFileSync(destination, bytes, { mode: 0o600 });
}

function addCollectionObject(
  root: string,
  manifest: Manifest,
  relPath: string,
  key: string,
  role: string,
  value: any
): boolean {
  const file = packagePath(root, relPath);
  const document = fs.existsSync(file) ? readJsonObject(file, relPath) : { [key]: [] };
  if (!Array.isArray(document[key])) throw new Error(`${relPath} must contain a ${key} array`);
  const existing = document[key].find((item: any) => item?.id === value.id);
  if (existing) {
    if (jsonEqual(existing, value)) return false;
    throw new Error(`${key} id already exists with different content: ${value.id}`);
  }
  document[key].push(value);
  document[key].sort((left: any, right: any) => String(left.id).localeCompare(String(right.id)));
  const bytes = Buffer.from(`${JSON.stringify(document, null, 2)}\n`);
  writeJson(file, document);
  upsertPart(manifest, hashedRef(relPath, 'application/json', role, bytes));
  return true;
}

function values(value: string[] | string | undefined): string[] {
  const input = value === undefined ? [] : Array.isArray(value) ? value : [value];
  return input.flatMap((item) => item.split(',')).map((item) => item.trim()).filter(Boolean);
}

function required(value: string | undefined, option: string): string {
  if (!value?.trim()) throw new Error(`${option} is required unless --json is used`);
  return value.trim();
}

function parseJsonOption(value: string | undefined, option: string, fallback: object = {}): any {
  if (value === undefined) return fallback;
  let parsed: any;
  try {
    parsed = JSON.parse(value);
  } catch {
    throw new Error(`${option} must be valid JSON`);
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${option} must be a JSON object`);
  }
  return parsed;
}

function objectFromJson(file: string | undefined): any | undefined {
  return file ? readJsonObject(path.resolve(file), file) : undefined;
}

function mediaTypeFor(file: string): string {
  const extension = path.extname(file).toLowerCase();
  return {
    '.md': 'text/markdown',
    '.txt': 'text/plain',
    '.csv': 'text/csv',
    '.json': 'application/json',
    '.html': 'text/html',
    '.pdf': 'application/pdf',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    '.docx': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    '.pptx': 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  }[extension] ?? 'application/octet-stream';
}

function safeIdFilename(id: string): string {
  const name = id.replace(/[^A-Za-z0-9._-]/g, '_');
  if (!name || name === '.' || name === '..') throw new Error(`id cannot form a safe filename: ${id}`);
  return name;
}

function knownIds(root: string, relPath: string, key: string): Set<string> {
  const file = packagePath(root, relPath);
  if (!fs.existsSync(file)) return new Set();
  const doc = readJsonObject(file, relPath);
  return new Set(Array.isArray(doc[key]) ? doc[key].map((item: any) => item?.id).filter(Boolean) : []);
}

function knownCalculationIds(root: string, manifest: Manifest): Set<string> {
  const ids = new Set<string>();
  for (const part of manifest.parts.filter((ref) => ref.role === 'calculation' && ref.mediaType === 'application/json')) {
    const file = packagePath(root, part.path);
    if (!fs.existsSync(file)) continue;
    const value = readJsonObject(file, part.path);
    if (typeof value.id === 'string') ids.add(value.id);
  }
  return ids;
}

function assertKnown(label: string, refs: string[], known: Set<string>): void {
  const missing = refs.filter((ref) => !known.has(ref));
  if (missing.length > 0) throw new Error(`unknown ${label}: ${missing.join(', ')}`);
}

function defaultTitle(directory: string): string {
  const base = path.basename(path.resolve(directory)).replace(/[-_]+/g, ' ').trim();
  return base ? base.replace(/\b\w/g, (letter) => letter.toUpperCase()) : 'Verifiable Work Product';
}

function defaultObjectId(prefix: string, title: string): string {
  const slug = title.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 60);
  return `${prefix}_${slug || randomUUID()}`;
}

function canonicalProspectivePath(file: string): string {
  let ancestor = path.resolve(file);
  const missingSegments: string[] = [];
  while (!fs.existsSync(ancestor)) {
    const parent = path.dirname(ancestor);
    if (parent === ancestor) break;
    missingSegments.unshift(path.basename(ancestor));
    ancestor = parent;
  }
  return path.join(fs.realpathSync(ancestor), ...missingSegments);
}

export function initCommand(directory: string, options: InitOptions): number {
  const target = path.resolve(directory);
  if (fs.existsSync(target) && (!fs.statSync(target).isDirectory() || fs.readdirSync(target).length > 0)) {
    throw new Error(`refusing to initialize non-empty path: ${directory}`);
  }
  fs.mkdirSync(target, { recursive: true });

  const title = options.title?.trim() || defaultTitle(directory);
  const profile = options.profile || 'vwp-core';
  const createdAt = new Date().toISOString();
  const creatorType = options.creatorType || 'human';
  if (!['human', 'agent', 'system'].includes(creatorType)) {
    throw new Error('--creator-type must be human, agent, or system');
  }
  const status = options.status || 'draft';
  const workproduct = {
    id: options.workproductId || defaultObjectId('wp', title),
    title,
    domain: options.domain || 'general',
    profile,
    ...(options.extension ? { extension: options.extension } : {}),
    status,
    reviewState: 'notStarted',
  };
  const provenance = {
    createdAt,
    createdBy: {
      type: creatorType,
      name: options.createdBy || os.userInfo().username || 'VWP CLI user',
    },
    toolchain: [{ name: '@vwp/cli', version: '0.1.0' }],
  };
  const claims = { claims: [] };
  const readable = Buffer.from(`# ${title}\n\n> Draft readable generated by VWP CLI.\n\n## Conclusion\n\nAdd the work product conclusion here.\n`);
  validateObject('workproduct', workproduct);
  validateObject('provenance', provenance);

  writeJson(path.join(target, 'workproduct.json'), workproduct);
  writeJson(path.join(target, 'provenance', 'provenance.json'), provenance);
  writeJson(path.join(target, 'claims', 'claims.json'), claims);
  putBytes(target, 'renderings/readable.md', readable);

  const workproductBytes = fs.readFileSync(path.join(target, 'workproduct.json'));
  const provenanceBytes = fs.readFileSync(path.join(target, 'provenance', 'provenance.json'));
  const claimsBytes = fs.readFileSync(path.join(target, 'claims', 'claims.json'));
  const manifest: Manifest = {
    vwpVersion: '0.1',
    packageId: options.packageId || `urn:uuid:${randomUUID()}`,
    createdAt,
    profile,
    ...(options.extension ? { extension: options.extension } : {}),
    title,
    workproduct: hashedRef('workproduct.json', 'application/json', 'workproduct', workproductBytes),
    provenance: hashedRef('provenance/provenance.json', 'application/json', 'provenance', provenanceBytes),
    parts: [
      hashedRef('claims/claims.json', 'application/json', 'claims', claimsBytes),
      hashedRef('renderings/readable.md', 'text/markdown', 'readable', readable),
    ],
  };
  saveManifest(target, manifest);
  console.log(`initialized VWP package in ${target}`);
  return 0;
}

export function addEvidenceCommand(directory: string, options: EvidenceOptions): number {
  const root = packageRoot(directory);
  const manifest = readManifest(root);
  let evidence = objectFromJson(options.json);
  if (!evidence) {
    const id = required(options.id, '--id');
    const title = required(options.title, '--title');
    if (options.file && options.externalRef) throw new Error('use either --file or --external-ref, not both');
    if (!options.file && !options.externalRef) throw new Error('--file or --external-ref is required');
    evidence = {
      id,
      type: options.type || 'document',
      title,
      ...(options.sourceSystem ? { sourceSystem: options.sourceSystem } : {}),
    };
    if (options.externalRef) {
      evidence.externalRef = {
        locator: options.externalRef,
        locatorType: options.locatorType || (/^https?:\/\//.test(options.externalRef) ? 'url' : 'opaque'),
        ...(options.sourceSystem ? { system: options.sourceSystem } : {}),
      };
    }
  }

  if (options.file) {
    const source = path.resolve(options.file);
    if (!fs.existsSync(source) || !fs.statSync(source).isFile()) throw new Error(`evidence file not found: ${options.file}`);
    const relPath = options.path || evidence.path || `evidence/${path.basename(source)}`;
    const bytes = fs.readFileSync(source);
    evidence.path = relPath;
    evidence.sha256 = sha256Hex(bytes);
    validateObject('evidence', evidence);
    putBytes(root, relPath, bytes);
    upsertPart(manifest, hashedRef(relPath, mediaTypeFor(source), 'evidence', bytes));
  } else {
    if (evidence.path) {
      const localFile = packagePath(root, evidence.path);
      if (!fs.existsSync(localFile) || !fs.statSync(localFile).isFile()) {
        throw new Error(`evidence path does not exist in the package: ${evidence.path}; use --file to copy it`);
      }
      const bytes = fs.readFileSync(localFile);
      const digest = sha256Hex(bytes);
      if (evidence.sha256 && evidence.sha256 !== digest) throw new Error(`evidence hash mismatch: ${evidence.path}`);
      evidence.sha256 = digest;
      upsertPart(manifest, hashedRef(evidence.path, mediaTypeFor(evidence.path), 'evidence', bytes));
    }
    validateObject('evidence', evidence);
  }

  const changed = addCollectionObject(root, manifest, 'evidence/evidence.json', 'evidence', 'evidence-index', evidence);
  saveManifest(root, manifest);
  console.log(`${changed ? 'added' : 'already present'} evidence ${evidence.id}`);
  return 0;
}

export function addFactCommand(directory: string, options: FactOptions): number {
  const root = packageRoot(directory);
  const manifest = readManifest(root);
  let fact = objectFromJson(options.json);
  if (!fact) {
    const evidenceRefs = values(options.evidence);
    fact = {
      id: required(options.id, '--id'),
      statement: required(options.statement, '--statement'),
      evidenceRefs,
      ...(options.factType ? { factType: options.factType } : {}),
      ...(options.value !== undefined ? { value: JSON.parse(options.value) } : {}),
      ...(options.confidence !== undefined ? { confidence: Number(options.confidence) } : {}),
      ...(options.requiresReview ? { requiresReview: true } : {}),
    };
  }
  validateObject('fact', fact);
  assertKnown('evidence id', fact.evidenceRefs.map((ref: any) => typeof ref === 'string' ? ref : ref.evidenceId), knownIds(root, 'evidence/evidence.json', 'evidence'));
  const changed = addCollectionObject(root, manifest, 'facts/facts.json', 'facts', 'facts', fact);
  saveManifest(root, manifest);
  console.log(`${changed ? 'added' : 'already present'} fact ${fact.id}`);
  return 0;
}

export function addClaimCommand(directory: string, options: ClaimOptions): number {
  const root = packageRoot(directory);
  const manifest = readManifest(root);
  let claim = objectFromJson(options.json);
  if (!claim) {
    const supportingFacts = values(options.fact);
    const supportingEvidence = values(options.evidence);
    const supportingCalculations = values(options.calculation);
    const judgments = values(options.judgment);
    const assertions = values(options.assertion);
    claim = {
      id: required(options.id, '--id'),
      statement: required(options.statement, '--statement'),
      ...(supportingFacts.length ? { supportingFacts } : {}),
      ...(supportingEvidence.length ? { supportingEvidence } : {}),
      ...(supportingCalculations.length ? { supportingCalculations } : {}),
      ...(judgments.length ? { judgments } : {}),
      ...(assertions.length ? { assertions } : {}),
      ...(options.conclusion ? { conclusion: options.conclusion } : {}),
      status: options.status || 'proposed',
      ...(options.requiresReview ? { requiresReview: true } : {}),
    };
  }
  validateObject('claim', claim);
  assertKnown('fact id', claim.supportingFacts ?? [], knownIds(root, 'facts/facts.json', 'facts'));
  assertKnown('evidence id', claim.supportingEvidence ?? [], knownIds(root, 'evidence/evidence.json', 'evidence'));
  assertKnown('calculation id', claim.supportingCalculations ?? [], knownCalculationIds(root, manifest));
  assertKnown('judgment id', claim.judgments ?? [], knownIds(root, 'judgments/judgments.json', 'judgments'));
  const changed = addCollectionObject(root, manifest, 'claims/claims.json', 'claims', 'claims', claim);
  saveManifest(root, manifest);
  console.log(`${changed ? 'added' : 'already present'} claim ${claim.id}`);
  return 0;
}

export function addCalculationCommand(directory: string, options: CalculationOptions): number {
  const root = packageRoot(directory);
  const manifest = readManifest(root);
  let calculation = objectFromJson(options.json);
  if (!calculation) {
    calculation = {
      id: required(options.id, '--id'),
      type: required(options.type, '--type'),
      inputs: parseJsonOption(options.inputs, '--inputs'),
      outputs: parseJsonOption(options.outputs, '--outputs'),
      ...(options.method ? { method: options.method } : {}),
      ...(options.validator ? { validator: options.validator } : {}),
      ...(options.recomputable ? { recomputable: true } : {}),
    };
  }
  validateObject('calculation', calculation);
  const relPath = `calculations/${safeIdFilename(calculation.id)}.json`;
  const bytes = Buffer.from(`${JSON.stringify(calculation, null, 2)}\n`);
  putBytes(root, relPath, bytes);
  upsertPart(manifest, hashedRef(relPath, 'application/json', 'calculation', bytes));
  saveManifest(root, manifest);
  console.log(`added calculation ${calculation.id}`);
  return 0;
}

export function setReadableCommand(directory: string, markdownFile: string, options: ReadableOptions): number {
  const root = packageRoot(directory);
  const source = path.resolve(markdownFile);
  if (!fs.existsSync(source) || !fs.statSync(source).isFile()) throw new Error(`Markdown file not found: ${markdownFile}`);
  if (!/\.md$/i.test(source)) throw new Error('readable must be a .md file');
  const relPath = options.path || 'renderings/readable.md';
  if (!/\.md$/i.test(relPath)) throw new Error('readable package path must end in .md');
  const bytes = fs.readFileSync(source);
  const manifest = readManifest(root);
  for (const part of manifest.parts) {
    if (part.role === 'readable' && part.path !== relPath) part.role = 'rendering';
  }
  putBytes(root, relPath, bytes, true);
  upsertPart(manifest, hashedRef(relPath, 'text/markdown', 'readable', bytes));
  saveManifest(root, manifest);
  console.log(`set readable ${relPath}`);
  return 0;
}

export function embedCommand(directory: string, sourceFile: string, options: EmbedOptions): number {
  const root = packageRoot(directory);
  const source = path.resolve(sourceFile);
  if (!fs.existsSync(source) || !fs.statSync(source).isFile()) throw new Error(`artifact file not found: ${sourceFile}`);
  const relPath = options.path || `artifacts/${path.basename(source)}`;
  const role = options.role || 'primary-artifact';
  const manifest = readManifest(root);
  if (role === 'primary-artifact') {
    const other = manifest.parts.find((part) => part.role === role && part.path !== relPath);
    if (other) throw new Error(`primary artifact already set to ${other.path}; use --role artifact for additional files`);
  }
  const bytes = fs.readFileSync(source);
  putBytes(root, relPath, bytes);
  upsertPart(manifest, hashedRef(relPath, options.mediaType || mediaTypeFor(source), role, bytes));
  saveManifest(root, manifest);
  console.log(`embedded ${relPath} (${role})`);
  return 0;
}

export function buildCommand(directory: string, output: string, options: BuildOptions): number {
  const root = packageRoot(directory);
  const destination = canonicalProspectivePath(output);
  const relativeOutput = path.relative(root, destination);
  if (relativeOutput === '' || (!relativeOutput.startsWith('..' + path.sep) && relativeOutput !== '..' && !path.isAbsolute(relativeOutput))) {
    throw new Error('build output must be outside the package directory');
  }
  const packed = packCommand(directory, output, { writeHashes: true });
  if (packed !== 0 || options.verify === false) return packed;
  const report = verifyPackage(openPackage(output));
  const failures = report.outcomes.filter((outcome) => outcome.status === 'fail');
  if (failures.length > 0) {
    for (const failure of failures) console.error(`FAIL ${failure.id}${failure.message ? ` - ${failure.message}` : ''}`);
    console.error(`built package is invalid: ${output}`);
    return 1;
  }
  console.log(`verified ${output}`);
  return 0;
}
