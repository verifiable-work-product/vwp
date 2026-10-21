import { PackageSource, readJson } from './package';
import { sha256Hex } from './hashes';
import { extensionSchemaErrors, schemaErrors, SchemaName } from './schema';
import {
  buildGraph,
  compareIds,
  computeDigests,
  DIGEST_ALGORITHM,
  Digests,
  IntegrityGraph,
  MEMO_FORMAT_PATH,
  NARRATIVE_PATH,
  selfHash,
} from './integrity';

export type OutcomeStatus = 'pass' | 'fail' | 'warning' | 'reviewRequired' | 'notRun';

export interface Outcome {
  id: string;
  status: OutcomeStatus;
  message?: string;
}

export interface VerifyReport {
  packageOrigin: string;
  manifest?: any;
  outcomes: Outcome[];
  /** true when no outcome has status 'fail' */
  valid: boolean;
}

const REQUIRED_PARTS = ['manifest.json', 'workproduct.json', 'claims/claims.json', 'provenance/provenance.json'];

interface PartRef {
  path: string;
  mediaType?: string;
  sha256?: string;
  role?: string;
}

function allPartRefs(manifest: any): PartRef[] {
  const refs: PartRef[] = [];
  if (manifest?.workproduct?.path) refs.push(manifest.workproduct);
  if (manifest?.provenance?.path) refs.push(manifest.provenance);
  for (const part of manifest?.parts ?? []) if (part?.path) refs.push(part);
  for (const sig of manifest?.signatures ?? []) if (sig?.path) refs.push(sig);
  return refs;
}

/** A fact evidence ref is a bare evidence id or an object with an optional anchor. */
function refEvidenceId(ref: any): string | undefined {
  if (typeof ref === 'string') return ref;
  if (ref && typeof ref.evidenceId === 'string') return ref.evidenceId;
  return undefined;
}

/**
 * Normalization applied to both quote and evidence text before matching, per
 * spec/facts.md: NFC, ASCII quotes/dashes, collapsed whitespace.
 */
export function normalizeForQuoteMatch(text: string): string {
  return text
    .normalize('NFC')
    .replace(/[‘’‚]/g, "'")
    .replace(/[“”„]/g, '"')
    .replace(/[–—]/g, '-')
    .replace(/\s+/g, ' ')
    .trim();
}

const TEXT_EVIDENCE_EXTENSIONS = ['.txt', '.md', '.csv', '.json'];

function isTextEvidencePath(p: string): boolean {
  const lower = p.toLowerCase();
  return TEXT_EVIDENCE_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

function collection(pkg: PackageSource, relPath: string, key: string): any[] | undefined {
  try {
    const doc = readJson(pkg, relPath) as any;
    if (doc === undefined) return undefined;
    return Array.isArray(doc?.[key]) ? doc[key] : [];
  } catch {
    return undefined;
  }
}

function checkCollectionSchema(
  outcomes: Outcome[],
  outcomeId: string,
  items: any[] | undefined,
  schema: SchemaName,
  label: string
): void {
  if (items === undefined) return;
  const errors: string[] = [];
  items.forEach((item, i) => {
    for (const err of schemaErrors(schema, item)) {
      errors.push(`${label}[${item?.id ?? i}]: ${err}`);
    }
  });
  outcomes.push(
    errors.length === 0
      ? { id: outcomeId, status: 'pass' }
      : { id: outcomeId, status: 'fail', message: errors.slice(0, 5).join('; ') }
  );
}

/** Run core, profile, and extension validators against a package. */
/** Auth schemes with defined semantics in v0.1. Other non-empty strings are
 *  permitted (forward-compatible) but surface as a warning. */
const KNOWN_AUTH_SCHEMES = new Set(['bearer', 'none']);

/** Heuristic: does a tokenHint carry a literal secret rather than a source
 *  reference (env:/vault:/oauth:/keychain:/prompt:/none)? A literal secret in a
 *  shareable package is a leak and must fail. */
export function tokenHintLooksLikeSecret(hint: string): boolean {
  return !/^(env:|vault:|oauth:|keychain:|prompt:|none$)/.test(hint.trim());
}

function isValidUri(s: string): boolean {
  try {
    // eslint-disable-next-line no-new
    new URL(s);
    return true;
  } catch {
    return false;
  }
}

export function verifyPackage(pkg: PackageSource): VerifyReport {
  const outcomes: Outcome[] = [];

  // core.package.safePaths
  if (pkg.unsafeEntries.length > 0) {
    outcomes.push({
      id: 'core.package.safePaths',
      status: 'fail',
      message: `unsafe entries rejected: ${pkg.unsafeEntries.join(', ')}`,
    });
  } else {
    outcomes.push({ id: 'core.package.safePaths', status: 'pass' });
  }

  // core.manifest.exists
  let manifest: any;
  try {
    manifest = readJson(pkg, 'manifest.json');
  } catch (err) {
    outcomes.push({ id: 'core.manifest.exists', status: 'fail', message: `manifest.json is not valid JSON: ${err}` });
    return { packageOrigin: pkg.origin, outcomes, valid: false };
  }
  if (manifest === undefined) {
    outcomes.push({ id: 'core.manifest.exists', status: 'fail', message: 'manifest.json not found' });
    return { packageOrigin: pkg.origin, outcomes, valid: false };
  }
  outcomes.push({ id: 'core.manifest.exists', status: 'pass' });

  // core.manifest.schemaValid
  const manifestErrors = schemaErrors('manifest', manifest);
  outcomes.push(
    manifestErrors.length === 0
      ? { id: 'core.manifest.schemaValid', status: 'pass' }
      : { id: 'core.manifest.schemaValid', status: 'fail', message: manifestErrors.slice(0, 5).join('; ') }
  );

  // core.requiredParts.exist
  const missing = REQUIRED_PARTS.filter((p) => !pkg.has(p));
  outcomes.push(
    missing.length === 0
      ? { id: 'core.requiredParts.exist', status: 'pass' }
      : { id: 'core.requiredParts.exist', status: 'fail', message: `missing: ${missing.join(', ')}` }
  );

  // core.partHashes.match
  const hashProblems: string[] = [];
  let declaredHashes = 0;
  for (const part of allPartRefs(manifest)) {
    if (!pkg.has(part.path)) {
      hashProblems.push(`declared part not found: ${part.path}`);
      continue;
    }
    if (part.sha256) {
      declaredHashes++;
      const actual = sha256Hex(pkg.read(part.path));
      if (actual !== part.sha256) hashProblems.push(`hash mismatch: ${part.path}`);
    }
  }
  if (hashProblems.length > 0) {
    outcomes.push({ id: 'core.partHashes.match', status: 'fail', message: hashProblems.join('; ') });
  } else if (declaredHashes === 0) {
    outcomes.push({ id: 'core.partHashes.match', status: 'warning', message: 'no part hashes declared in manifest' });
  } else {
    outcomes.push({ id: 'core.partHashes.match', status: 'pass' });
  }

  // core.workproduct.schemaValid
  let workproduct: any;
  try {
    workproduct = readJson(pkg, 'workproduct.json');
  } catch {
    workproduct = undefined;
  }
  if (workproduct === undefined) {
    outcomes.push({ id: 'core.workproduct.schemaValid', status: 'fail', message: 'workproduct.json missing or unreadable' });
  } else {
    const errs = schemaErrors('workproduct', workproduct);
    outcomes.push(
      errs.length === 0
        ? { id: 'core.workproduct.schemaValid', status: 'pass' }
        : { id: 'core.workproduct.schemaValid', status: 'fail', message: errs.slice(0, 5).join('; ') }
    );
  }

  // core.claims.schemaValid
  const claims = collection(pkg, 'claims/claims.json', 'claims');
  if (claims === undefined) {
    outcomes.push({ id: 'core.claims.schemaValid', status: 'fail', message: 'claims/claims.json missing or unreadable' });
  } else {
    checkCollectionSchema(outcomes, 'core.claims.schemaValid', claims, 'claim', 'claim');
  }

  // core.provenance.schemaValid
  let provenance: any;
  try {
    provenance = readJson(pkg, 'provenance/provenance.json');
  } catch {
    provenance = undefined;
  }
  if (provenance === undefined) {
    outcomes.push({ id: 'core.provenance.schemaValid', status: 'fail', message: 'provenance/provenance.json missing or unreadable' });
  } else {
    const errs = schemaErrors('provenance', provenance);
    outcomes.push(
      errs.length === 0
        ? { id: 'core.provenance.schemaValid', status: 'pass' }
        : { id: 'core.provenance.schemaValid', status: 'fail', message: errs.slice(0, 5).join('; ') }
    );
  }

  // Optional collections: schema-check when present.
  const evidence = collection(pkg, 'evidence/evidence.json', 'evidence');
  checkCollectionSchema(outcomes, 'core.evidence.schemaValid', evidence, 'evidence', 'evidence');
  const facts = collection(pkg, 'facts/facts.json', 'facts');
  checkCollectionSchema(outcomes, 'core.facts.schemaValid', facts, 'fact', 'fact');
  const judgments = collection(pkg, 'judgments/judgments.json', 'judgments');
  checkCollectionSchema(outcomes, 'core.judgments.schemaValid', judgments, 'judgment', 'judgment');
  const approvals = collection(pkg, 'approvals/approvals.json', 'approvals');
  checkCollectionSchema(outcomes, 'core.approvals.schemaValid', approvals, 'approval', 'approval');
  const validations = collection(pkg, 'validations/validation-report.json', 'validations');
  checkCollectionSchema(outcomes, 'core.validations.schemaValid', validations, 'validation', 'validation');

  // core.attestations.schemaValid — analysis attestations are optional parts
  // under attestations/. Each is schema-checked like any other part; beyond
  // that, attestation content never affects package validity (findings and
  // dispositions are for humans, not for verification).
  const attestationPaths = pkg.list().filter((p) => /^attestations\/[^/]+\.json$/.test(p));
  if (attestationPaths.length > 0) {
    const attErrors: string[] = [];
    for (const attPath of attestationPaths) {
      let attestation: unknown;
      try {
        attestation = readJson(pkg, attPath);
      } catch {
        attErrors.push(`${attPath}: not valid JSON`);
        continue;
      }
      for (const err of schemaErrors('analysis-attestation', attestation)) {
        attErrors.push(`${attPath}: ${err}`);
      }
    }
    outcomes.push(
      attErrors.length === 0
        ? {
            id: 'core.attestations.schemaValid',
            status: 'pass',
            message: `${attestationPaths.length} attestation${attestationPaths.length === 1 ? '' : 's'} schema-checked`,
          }
        : { id: 'core.attestations.schemaValid', status: 'fail', message: attErrors.slice(0, 5).join('; ') }
    );
  }

  // core.evidenceRefs.resolve
  if (evidence !== undefined) {
    const evidenceIds = new Set(evidence.map((e: any) => e.id));
    const problems: string[] = [];
    for (const ev of evidence) {
      if (ev.path) {
        if (!pkg.has(ev.path)) {
          problems.push(`evidence ${ev.id}: file not found: ${ev.path}`);
        } else if (ev.sha256 && sha256Hex(pkg.read(ev.path)) !== ev.sha256) {
          problems.push(`evidence ${ev.id}: hash mismatch: ${ev.path}`);
        }
      }
    }
    for (const ev of evidence) {
      if (ev.derivedFrom && !evidenceIds.has(ev.derivedFrom)) {
        problems.push(`evidence ${ev.id}: unknown derivedFrom ref ${ev.derivedFrom}`);
      }
    }
    for (const fact of facts ?? []) {
      for (const ref of fact.evidenceRefs ?? []) {
        const evidenceId = refEvidenceId(ref);
        if (evidenceId === undefined || !evidenceIds.has(evidenceId)) {
          problems.push(`fact ${fact.id}: unknown evidence ref ${evidenceId ?? JSON.stringify(ref)}`);
        }
      }
    }
    for (const claim of claims ?? []) {
      for (const ref of claim.supportingEvidence ?? []) {
        if (!evidenceIds.has(ref)) problems.push(`claim ${claim.id}: unknown evidence ref ${ref}`);
      }
    }
    outcomes.push(
      problems.length === 0
        ? { id: 'core.evidenceRefs.resolve', status: 'pass' }
        : { id: 'core.evidenceRefs.resolve', status: 'fail', message: problems.slice(0, 5).join('; ') }
    );
  }

  // core.externalRefs.wellFormed — offline check of externalRef retrieval/auth
  // metadata. Deterministic and network-free: it validates SHAPE only (is the
  // URL a URI, is the auth scheme known, does a tokenHint leak a literal
  // secret). Actually fetching the URL is core.externalRefs.reachable, an
  // opt-in online check. An embedded secret is a hard FAIL (leak); a malformed
  // URL or unknown scheme is a WARNING.
  if (evidence !== undefined) {
    const fails: string[] = [];
    const warns: string[] = [];
    let checked = 0;
    for (const ev of evidence) {
      const ref = ev.externalRef;
      if (!ref) continue;
      if (ref.locatorType === 'url' && typeof ref.locator === 'string' && !isValidUri(ref.locator)) {
        warns.push(`evidence ${ev.id}: locatorType 'url' but locator is not a valid URI`);
      }
      const r = ref.retrieval;
      if (!r) continue;
      checked++;
      if (typeof r.url === 'string' && !isValidUri(r.url)) {
        warns.push(`evidence ${ev.id}: retrieval.url is not a valid URI`);
      }
      const auth = r.auth;
      if (auth) {
        if (typeof auth.scheme === 'string' && !KNOWN_AUTH_SCHEMES.has(auth.scheme)) {
          warns.push(`evidence ${ev.id}: auth.scheme '${auth.scheme}' is not a v0.1-defined scheme (bearer|none)`);
        }
        if (typeof auth.tokenHint === 'string' && tokenHintLooksLikeSecret(auth.tokenHint)) {
          fails.push(
            `evidence ${ev.id}: auth.tokenHint appears to embed a literal secret; use a source reference (env:/vault:/oauth:/keychain:/prompt:)`
          );
        }
        if (typeof auth.discovery === 'string' && !isValidUri(auth.discovery)) {
          warns.push(`evidence ${ev.id}: auth.discovery is not a valid URI`);
        }
      }
    }
    if (fails.length > 0) {
      outcomes.push({ id: 'core.externalRefs.wellFormed', status: 'fail', message: fails.slice(0, 5).join('; ') });
    } else if (warns.length > 0) {
      outcomes.push({ id: 'core.externalRefs.wellFormed', status: 'warning', message: warns.slice(0, 5).join('; ') });
    } else if (checked > 0) {
      outcomes.push({
        id: 'core.externalRefs.wellFormed',
        status: 'pass',
        message: `${checked} external retrieval reference(s) well-formed`,
      });
    } else {
      outcomes.push({ id: 'core.externalRefs.wellFormed', status: 'notRun', message: 'no external retrieval metadata declared' });
    }
  }

  // core.factAnchors.quotesMatch — quote anchors must appear in the cited
  // text evidence (after normalization). Quotes against binary or external
  // evidence cannot be machine-checked and surface as a warning.
  if (facts !== undefined && evidence !== undefined) {
    const evidenceById = new Map(evidence.map((e: any) => [e.id, e]));
    const mismatches: string[] = [];
    const uncheckable: string[] = [];
    let checked = 0;
    for (const fact of facts) {
      for (const ref of fact.evidenceRefs ?? []) {
        const anchors: any[] = typeof ref === 'object' && Array.isArray(ref?.anchors) ? ref.anchors : [];
        for (const anchor of anchors) {
          const quote = anchor?.quote;
          if (typeof quote !== 'string') continue;
          const ev = evidenceById.get(refEvidenceId(ref));
          if (!ev?.path || !pkg.has(ev.path) || !isTextEvidencePath(ev.path)) {
            uncheckable.push(`fact ${fact.id} -> evidence ${ev?.id ?? refEvidenceId(ref)}`);
            continue;
          }
          checked++;
          const haystack = normalizeForQuoteMatch(pkg.read(ev.path).toString('utf8'));
          if (!haystack.includes(normalizeForQuoteMatch(quote))) {
            mismatches.push(`fact ${fact.id}: quote not found in ${ev.path}`);
          }
        }
      }
    }
    if (mismatches.length > 0) {
      outcomes.push({ id: 'core.factAnchors.quotesMatch', status: 'fail', message: mismatches.slice(0, 5).join('; ') });
    } else if (uncheckable.length > 0) {
      outcomes.push({
        id: 'core.factAnchors.quotesMatch',
        status: 'warning',
        message: `${checked} quote anchors verified; not machine-checkable against binary or external evidence (anchor to a derived text layer): ${uncheckable
          .slice(0, 3)
          .join(', ')}`,
      });
    } else if (checked > 0) {
      outcomes.push({ id: 'core.factAnchors.quotesMatch', status: 'pass', message: `${checked} quote anchors verified` });
    } else {
      outcomes.push({ id: 'core.factAnchors.quotesMatch', status: 'notRun', message: 'no quote anchors declared' });
    }
  }

  // core.claimRefs.resolve — claim links to facts/judgments/calculations must resolve.
  if (claims !== undefined) {
    const factIds = new Set((facts ?? []).map((f: any) => f.id));
    const judgmentIds = new Set((judgments ?? []).map((j: any) => j.id));
    const calcIds = new Set(
      pkg
        .list()
        .filter((p) => p.startsWith('calculations/') && p.endsWith('.json'))
        .map((p) => {
          try {
            return (readJson(pkg, p) as any)?.id;
          } catch {
            return undefined;
          }
        })
        .filter(Boolean)
    );
    const problems: string[] = [];
    for (const claim of claims) {
      for (const ref of claim.supportingFacts ?? []) {
        if (!factIds.has(ref)) problems.push(`claim ${claim.id}: unknown fact ref ${ref}`);
      }
      for (const ref of claim.judgments ?? []) {
        if (!judgmentIds.has(ref)) problems.push(`claim ${claim.id}: unknown judgment ref ${ref}`);
      }
      for (const ref of claim.supportingCalculations ?? []) {
        if (!calcIds.has(ref)) problems.push(`claim ${claim.id}: unknown calculation ref ${ref}`);
      }
    }
    outcomes.push(
      problems.length === 0
        ? { id: 'core.claimRefs.resolve', status: 'pass' }
        : { id: 'core.claimRefs.resolve', status: 'fail', message: problems.slice(0, 5).join('; ') }
    );
  }

  // Integrity graph (spec/integrity.md): reference resolution, acyclicity,
  // undeclared inputs, manifest digests; then the approval and narrative
  // commitments that rest on the recomputed roots.
  const graph = graphValidators(outcomes, pkg, manifest);
  approvalsCurrent(outcomes, graph);
  narrativeValidators(outcomes, pkg, graph);

  // core.signatures.present — informational in v0.1.
  const hasSignature = pkg.list().some((p) => p.startsWith('signatures/'));
  if (!hasSignature) {
    outcomes.push({ id: 'core.signatures.present', status: 'warning', message: 'no package signature found' });
  } else {
    outcomes.push({ id: 'core.signatures.present', status: 'pass' });
  }

  // Profile and extension validators.
  const profile = manifest?.profile as string | undefined;
  const extension = manifest?.extension as string | undefined;
  if (profile === 'vwp-finance') {
    financeValidators(outcomes, claims ?? [], judgments ?? []);
  }
  if (extension === 'vwp-accounting-asc606') {
    asc606Validators(outcomes, pkg);
  }
  if (extension === 'vwp-digital-assets-wallet-ownership-control') {
    walletValidators(outcomes, pkg, evidence ?? []);
  }
  if (extension === 'vwp-accounting-asc820') {
    asc820Validators(outcomes, pkg, evidence ?? [], judgments ?? []);
  }

  const valid = !outcomes.some((o) => o.status === 'fail');
  return { packageOrigin: pkg.origin, manifest, outcomes, valid };
}

/** One distinct network origin the package wants to reach, with the auth it
 *  requires. Secrets live NOWHERE in the package — this is gathered from the
 *  refs so a caller can request a credential per origin, once. */
export interface RetrievalOrigin {
  /** e.g. 'https://api.bitwave.io' */
  origin: string;
  /** true if any ref on this origin uses a credentialed scheme (e.g. bearer). */
  requiresAuth: boolean;
  /** Distinct auth schemes seen on this origin. */
  schemes: string[];
  /** Distinct tokenHints (source references, never secrets) seen on this origin. */
  tokenHints: string[];
  /** RFC 9728 discovery URL if any ref on this origin declared one. */
  discovery?: string;
  /** Distinct OAuth scopes requested across refs on this origin. */
  scopes: string[];
}

/** How a caller supplies credentials for the online check: a map from origin to
 *  the bearer token to use there. The verifier NEVER reads a secret from the
 *  package — the caller gathers origins (collectRetrievalOrigins), requests a
 *  token per origin over the top, and passes them here. */
export type CredentialResolver = (origin: RetrievalOrigin) => string | undefined | Promise<string | undefined>;

function originOf(url: string): string | undefined {
  try {
    return new URL(url).origin;
  } catch {
    return undefined;
  }
}

/**
 * Enumerate the distinct origins the package's externalRefs want to reach, and
 * what auth each needs — WITHOUT touching the network. This is the "gather all
 * the base URLs, then request secrets for them" step: a caller runs this,
 * obtains one credential per origin (env, prompt, vault, OAuth via the RFC 9728
 * discovery URL, ...), and only then runs verifyExternalRefsReachable.
 */
export function collectRetrievalOrigins(pkg: PackageSource): RetrievalOrigin[] {
  let evidence: any[] | undefined;
  try {
    evidence = pkg.has('evidence/evidence.json') ? (readJson(pkg, 'evidence/evidence.json') as any)?.evidence : undefined;
  } catch {
    evidence = undefined;
  }
  const byOrigin = new Map<string, RetrievalOrigin>();
  for (const ev of evidence ?? []) {
    const r = ev.externalRef?.retrieval;
    if (!r?.url) continue;
    const origin = originOf(r.url);
    if (!origin) continue;
    const auth = r.auth ?? {};
    const scheme = auth.scheme ?? 'none';
    const entry =
      byOrigin.get(origin) ??
      ({ origin, requiresAuth: false, schemes: [], tokenHints: [], scopes: [] } as RetrievalOrigin);
    if (scheme !== 'none') entry.requiresAuth = true;
    if (!entry.schemes.includes(scheme)) entry.schemes.push(scheme);
    if (typeof auth.tokenHint === 'string' && !entry.tokenHints.includes(auth.tokenHint)) entry.tokenHints.push(auth.tokenHint);
    if (typeof auth.discovery === 'string') entry.discovery = auth.discovery;
    for (const s of auth.scopes ?? []) if (!entry.scopes.includes(s)) entry.scopes.push(s);
    byOrigin.set(origin, entry);
  }
  return [...byOrigin.values()];
}

/** Default resolver: gather a bearer token per origin from the environment.
 *  Tries, in order, each ref's own 'env:NAME' tokenHint on that origin, then a
 *  conventional VWP_TOKEN_<HOST> variable. Never reads anything from the
 *  package. Non-env sources (vault:/oauth:/keychain:/prompt:) are left to a
 *  richer resolver supplied by the embedding app (e.g. Wavie). */
export function envCredentialResolver(origin: RetrievalOrigin): string | undefined {
  for (const hint of origin.tokenHints) {
    const [scheme, ...rest] = hint.split(':');
    if (scheme === 'env') {
      const v = process.env[rest.join(':')];
      if (v) return v;
    }
  }
  try {
    const host = new URL(origin.origin).host.replace(/[^A-Za-z0-9]/g, '_').toUpperCase();
    return process.env[`VWP_TOKEN_${host}`];
  } catch {
    return undefined;
  }
}

/**
 * core.externalRefs.reachable — ONLINE, opt-in. Fetches each
 * externalRef.retrieval.url, supplying a Bearer token (RFC 6750) obtained PER
 * ORIGIN from the caller-supplied resolver — credentials are gathered "over the
 * top" during verification, never embedded in the package — and, when
 * contentHash is declared, compares the SHA-256 of the retrieved bytes. This
 * check is environment-dependent (network, credentials) and therefore never
 * runs during the default offline `verify`. Returns outcomes to append to a
 * report.
 */
export async function verifyExternalRefsReachable(
  pkg: PackageSource,
  resolveCredential: CredentialResolver = envCredentialResolver
): Promise<Outcome[]> {
  let evidence: any[] | undefined;
  try {
    evidence = pkg.has('evidence/evidence.json') ? (readJson(pkg, 'evidence/evidence.json') as any)?.evidence : undefined;
  } catch {
    evidence = undefined;
  }
  if (evidence === undefined) {
    return [{ id: 'core.externalRefs.reachable', status: 'notRun', message: 'no evidence index' }];
  }

  const targets = evidence.filter((ev: any) => ev.externalRef?.retrieval?.url);
  if (targets.length === 0) {
    return [{ id: 'core.externalRefs.reachable', status: 'notRun', message: 'no retrievable external references' }];
  }

  // Step 1: gather origins and resolve one credential per origin, up front.
  const origins = collectRetrievalOrigins(pkg);
  const tokenByOrigin = new Map<string, string | undefined>();
  for (const o of origins) {
    tokenByOrigin.set(o.origin, o.requiresAuth ? await resolveCredential(o) : undefined);
  }

  // Step 2: fetch each ref, reusing the per-origin credential.
  const problems: string[] = [];
  let ok = 0;
  for (const ev of targets) {
    const r = ev.externalRef.retrieval;
    const headers: Record<string, string> = {};
    const auth = r.auth ?? {};
    const origin = originOf(r.url);
    if ((auth.scheme ?? 'none') === 'bearer') {
      const token = origin ? tokenByOrigin.get(origin) : undefined;
      if (!token) {
        problems.push(`evidence ${ev.id}: no bearer credential available for origin ${origin ?? '(unknown)'}`);
        continue;
      }
      headers[auth.header ?? 'Authorization'] = `Bearer ${token}`;
    }
    try {
      const res = await fetch(r.url, { method: r.method ?? 'GET', headers });
      if (!res.ok) {
        // Surface RFC 6750 WWW-Authenticate challenge when present.
        const challenge = res.headers.get('www-authenticate');
        problems.push(`evidence ${ev.id}: ${res.status} ${res.statusText}${challenge ? ` (${challenge})` : ''}`);
        continue;
      }
      if (r.contentHash) {
        const buf = Buffer.from(await res.arrayBuffer());
        const got = sha256Hex(buf);
        if (got !== r.contentHash) {
          problems.push(`evidence ${ev.id}: contentHash mismatch (expected ${r.contentHash.slice(0, 12)}…, got ${got.slice(0, 12)}…)`);
          continue;
        }
      }
      ok++;
    } catch (err) {
      problems.push(`evidence ${ev.id}: fetch failed: ${(err as Error).message}`);
    }
  }

  if (problems.length > 0) {
    return [
      {
        id: 'core.externalRefs.reachable',
        status: 'fail',
        message: `${ok}/${targets.length} reachable across ${origins.length} origin(s); ${problems.slice(0, 5).join('; ')}`,
      },
    ];
  }
  return [
    {
      id: 'core.externalRefs.reachable',
      status: 'pass',
      message: `${ok}/${targets.length} external reference(s) reachable across ${origins.length} origin(s); content-hash verified where declared`,
    },
  ];
}

/** Finance profile: assertion and support coverage, judgment review surfacing. */
function financeValidators(outcomes: Outcome[], claims: any[], judgments: any[]): void {
  const noAssertions = claims.filter((c) => !(c.assertions?.length > 0)).map((c) => c.id);
  outcomes.push(
    noAssertions.length === 0
      ? { id: 'finance.claims.haveAssertions', status: 'pass' }
      : {
          id: 'finance.claims.haveAssertions',
          status: 'warning',
          message: `claims without assertions: ${noAssertions.join(', ')}`,
        }
  );

  const unsupported = claims
    .filter(
      (c) =>
        !(
          c.supportingFacts?.length > 0 ||
          c.supportingEvidence?.length > 0 ||
          c.supportingCalculations?.length > 0 ||
          c.judgments?.length > 0
        )
    )
    .map((c) => c.id);
  outcomes.push(
    unsupported.length === 0
      ? { id: 'finance.claims.haveSupport', status: 'pass' }
      : {
          id: 'finance.claims.haveSupport',
          status: 'warning',
          message: `claims without supporting facts, evidence, calculations, or judgments: ${unsupported.join(', ')}`,
        }
  );

  const openJudgments = judgments.filter((j) => j.status === 'reviewRequired' || j.status === 'open');
  outcomes.push(
    openJudgments.length === 0
      ? { id: 'finance.requiredJudgments.complete', status: 'pass' }
      : {
          id: 'finance.requiredJudgments.complete',
          status: 'reviewRequired',
          message: `${openJudgments.length} judgment${openJudgments.length === 1 ? '' : 's'} require${
            openJudgments.length === 1 ? 's' : ''
          } review: ${openJudgments.map((j) => j.id).join(', ')}`,
        }
  );
}

function near(a: number, b: number, tolerance = 0.01): boolean {
  return Math.abs(a - b) <= tolerance;
}

function parseCsv(text: string): string[][] {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .map((line) => line.split(',').map((cell) => cell.trim()));
}

/** ASC 606 demo validators: allocation schema, then allocation and revenue schedule tie-outs. */
function asc606Validators(outcomes: Outcome[], pkg: PackageSource): void {
  let allocation: any;
  try {
    allocation = readJson(pkg, 'calculations/ssp-allocation.json');
  } catch {
    allocation = undefined;
  }

  let allocationValid = false;
  if (allocation === undefined) {
    outcomes.push({
      id: 'asc606.allocation.schemaValid',
      status: 'fail',
      message: 'calculations/ssp-allocation.json not present',
    });
    outcomes.push({ id: 'asc606.allocation.tieOut', status: 'fail', message: 'calculations/ssp-allocation.json not present' });
  } else {
    const errs = extensionSchemaErrors('extensions/accounting-asc606/ssp-allocation.schema.json', allocation);
    if (errs.length > 0) {
      outcomes.push({ id: 'asc606.allocation.schemaValid', status: 'fail', message: errs.slice(0, 5).join('; ') });
      outcomes.push({ id: 'asc606.allocation.tieOut', status: 'fail', message: 'allocation failed extension schema validation' });
    } else {
      outcomes.push({ id: 'asc606.allocation.schemaValid', status: 'pass' });
      allocationValid = true;
      const transactionPrice = Number(allocation.inputs.transactionPrice);
      const allocations: any[] = allocation.outputs.allocations;
      const total = allocations.reduce((sum, a) => sum + Number(a.allocatedAmount ?? 0), 0);
      outcomes.push(
        near(total, transactionPrice)
          ? { id: 'asc606.allocation.tieOut', status: 'pass', message: `allocated ${total} equals transaction price ${transactionPrice}` }
          : {
              id: 'asc606.allocation.tieOut',
              status: 'fail',
              message: `allocated total ${total} does not equal transaction price ${transactionPrice}`,
            }
      );
    }
  }

  if (!pkg.has('calculations/revenue-schedule.csv')) {
    outcomes.push({
      id: 'asc606.revenueSchedule.total',
      status: 'fail',
      message: 'calculations/revenue-schedule.csv not present',
    });
    return;
  }
  if (!allocationValid) {
    outcomes.push({
      id: 'asc606.revenueSchedule.total',
      status: 'fail',
      message: 'requires a schema-valid calculations/ssp-allocation.json to tie against',
    });
    return;
  }
  const rows = parseCsv(pkg.read('calculations/revenue-schedule.csv').toString('utf8'));
  const header = rows[0] ?? [];
  const totalCol = header.indexOf('total');
  if (totalCol === -1) {
    outcomes.push({ id: 'asc606.revenueSchedule.total', status: 'fail', message: "revenue-schedule.csv has no 'total' column" });
    return;
  }
  const scheduleTotal = rows.slice(1).reduce((sum, row) => sum + Number(row[totalCol] ?? 0), 0);
  const allocatedTotal = allocation.outputs.allocations.reduce(
    (sum: number, a: any) => sum + Number(a.allocatedAmount ?? 0),
    0
  );
  outcomes.push(
    near(scheduleTotal, allocatedTotal)
      ? {
          id: 'asc606.revenueSchedule.total',
          status: 'pass',
          message: `schedule total ${scheduleTotal} equals allocated consideration ${allocatedTotal}`,
        }
      : {
          id: 'asc606.revenueSchedule.total',
          status: 'fail',
          message: `schedule total ${scheduleTotal} does not equal allocated consideration ${allocatedTotal}`,
        }
  );
}

/**
 * Digital asset wallet ownership demo validators, run against the structured
 * wallet register at facts/wallets.json.
 */
function walletValidators(outcomes: Outcome[], pkg: PackageSource, evidence: any[]): void {
  let register: any;
  try {
    register = readJson(pkg, 'facts/wallets.json');
  } catch {
    register = undefined;
  }
  const wallets: any[] = register?.wallets ?? [];
  if (wallets.length === 0) {
    outcomes.push({
      id: 'digitalAssets.wallets.registerPresent',
      status: 'fail',
      message: 'facts/wallets.json missing or contains no wallets',
    });
    return;
  }
  const schemaErrs = extensionSchemaErrors(
    'extensions/digital-assets-wallet-ownership-control/wallets.schema.json',
    register
  );
  if (schemaErrs.length > 0) {
    outcomes.push({
      id: 'digitalAssets.wallets.schemaValid',
      status: 'fail',
      message: `${schemaErrs.slice(0, 5).join('; ')} (remaining wallet validators not run)`,
    });
    return;
  }
  outcomes.push({ id: 'digitalAssets.wallets.schemaValid', status: 'pass' });
  const evidenceIds = new Set(evidence.map((e: any) => e.id));
  const resolves = (refs: string[] | undefined) =>
    Array.isArray(refs) && refs.length > 0 && refs.every((r) => evidenceIds.has(r));
  const included = wallets.filter((w) => w.included === true);
  const excluded = wallets.filter((w) => w.included !== true);

  const noMapping = included.filter((w) => !resolves(w.entityMappingEvidence)).map((w) => w.id);
  outcomes.push(
    noMapping.length === 0
      ? { id: 'digitalAssets.wallets.entityMapping', status: 'pass', message: `${included.length} included wallets mapped to entities` }
      : {
          id: 'digitalAssets.wallets.entityMapping',
          status: 'fail',
          message: `included wallets without entity mapping evidence: ${noMapping.join(', ')}`,
        }
  );

  const selfCustody = included.filter((w) => w.custody === 'self');
  const noProof = selfCustody.filter((w) => !resolves(w.proofOfControlEvidence)).map((w) => w.id);
  outcomes.push(
    noProof.length === 0
      ? {
          id: 'digitalAssets.wallets.proofOfControl',
          status: 'pass',
          message: `${selfCustody.length} self-custody wallets have proof-of-control evidence`,
        }
      : {
          id: 'digitalAssets.wallets.proofOfControl',
          status: 'fail',
          message: `self-custody wallets without proof-of-control evidence: ${noProof.join(', ')}`,
        }
  );

  const noRationale = excluded
    .filter((w) => !(typeof w.exclusionRationale === 'string' && w.exclusionRationale.length > 0))
    .map((w) => w.id);
  outcomes.push(
    noRationale.length === 0
      ? { id: 'digitalAssets.wallets.exclusionRationale', status: 'pass', message: `${excluded.length} excluded wallets have rationale` }
      : {
          id: 'digitalAssets.wallets.exclusionRationale',
          status: 'fail',
          message: `excluded wallets without exclusion rationale: ${noRationale.join(', ')}`,
        }
  );

  const noBalance = included
    .filter((w) => w.periodEndBalance !== undefined && !resolves(w.balanceEvidence))
    .map((w) => w.id);
  outcomes.push(
    noBalance.length === 0
      ? { id: 'digitalAssets.wallets.balanceEvidence', status: 'pass', message: 'period-end balances tie to source evidence' }
      : {
          id: 'digitalAssets.wallets.balanceEvidence',
          status: 'fail',
          message: `period-end balances without source evidence: ${noBalance.join(', ')}`,
        }
  );
}

/**
 * ASC 820 demo validators, run against the fair value measurement register
 * at facts/fair-value-measurements.json.
 */
function asc820Validators(outcomes: Outcome[], pkg: PackageSource, evidence: any[], judgments: any[]): void {
  let register: any;
  try {
    register = readJson(pkg, 'facts/fair-value-measurements.json');
  } catch {
    register = undefined;
  }
  if (register === undefined) {
    outcomes.push({
      id: 'asc820.measurements.present',
      status: 'fail',
      message: 'facts/fair-value-measurements.json missing or unreadable',
    });
    return;
  }
  const schemaErrs = extensionSchemaErrors('extensions/accounting-asc820/fair-value-measurements.schema.json', register);
  if (schemaErrs.length > 0) {
    outcomes.push({
      id: 'asc820.measurements.schemaValid',
      status: 'fail',
      message: `${schemaErrs.slice(0, 5).join('; ')} (remaining asc820 validators not run)`,
    });
    return;
  }
  outcomes.push({ id: 'asc820.measurements.schemaValid', status: 'pass' });

  const measurements: any[] = register.measurements;
  const evidenceIds = new Set(evidence.map((e: any) => e.id));
  const judgmentIds = new Set(judgments.map((j: any) => j.id));

  const badSources = measurements
    .filter((m) => !(m.pricingSourceEvidence as string[]).every((ref) => evidenceIds.has(ref)))
    .map((m) => m.id);
  outcomes.push(
    badSources.length === 0
      ? {
          id: 'asc820.pricingSources.resolve',
          status: 'pass',
          message: `${measurements.length} measurements have resolvable pricing source evidence`,
        }
      : {
          id: 'asc820.pricingSources.resolve',
          status: 'fail',
          message: `measurements with unresolvable pricing source evidence: ${badSources.join(', ')}`,
        }
  );

  const level3 = measurements.filter((m) => m.level === 'level3');
  const unjudged = level3
    .filter((m) => !(typeof m.judgmentRef === 'string' && judgmentIds.has(m.judgmentRef)))
    .map((m) => m.id);
  outcomes.push(
    unjudged.length === 0
      ? {
          id: 'asc820.level3.judgmentLinked',
          status: 'pass',
          message: `${level3.length} level3 measurement${level3.length === 1 ? '' : 's'} link${
            level3.length === 1 ? 's' : ''
          } a hierarchy/unobservable-inputs judgment`,
        }
      : {
          id: 'asc820.level3.judgmentLinked',
          status: 'fail',
          message: `level3 measurements without a resolvable judgmentRef: ${unjudged.join(', ')}`,
        }
  );
}

/* ------------------------------------------------------------------------ */
/* Integrity graph validators (spec/integrity.md section 7)                  */
/* ------------------------------------------------------------------------ */

function capped(items: string[], limit = 5): string {
  return items.length > limit ? `${items.slice(0, limit).join('; ')}; and ${items.length - limit} more` : items.join('; ');
}

/** Characters that can be part of an object id; a fact id mentioned in prose is a whole token. */
const ID_CHAR = 'A-Za-z0-9_\\-';

function mentionsId(text: string, id: string): boolean {
  const escaped = id.replace(/[.*+?^${}()|[\]\\/-]/g, '\\$&');
  return new RegExp(`(?<![${ID_CHAR}])${escaped}(?![${ID_CHAR}])`).test(text);
}

/** Every string value (not key) under a JSON value, in document order. */
function stringValues(value: unknown, out: string[] = []): string[] {
  if (typeof value === 'string') out.push(value);
  else if (Array.isArray(value)) for (const v of value) stringValues(v, out);
  else if (value && typeof value === 'object') for (const v of Object.values(value)) stringValues(v, out);
  return out;
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === 'string') : [];
}

/**
 * core.graph.refsResolve, core.graph.acyclic, core.graph.declaredInputs and
 * core.digests.match. Returns the built graph for the validators that
 * follow.
 */
function graphValidators(outcomes: Outcome[], pkg: PackageSource, manifest: any): IntegrityGraph {
  const graph = buildGraph(pkg);

  const unresolved = graph.problems.filter((p) => p.kind === 'unresolved' || p.kind === 'duplicate').map((p) => p.message);
  outcomes.push(
    unresolved.length === 0
      ? { id: 'core.graph.refsResolve', status: 'pass' }
      : { id: 'core.graph.refsResolve', status: 'fail', message: capped(unresolved) }
  );

  const cycles = graph.problems.filter((p) => p.kind === 'cycle').map((p) => p.message);
  outcomes.push(
    cycles.length === 0
      ? { id: 'core.graph.acyclic', status: 'pass' }
      : { id: 'core.graph.acyclic', status: 'fail', message: `dependency cycles: ${capped(cycles)}` }
  );

  // core.graph.declaredInputs: an input the closure hash is blind to.
  const inputKinds = new Set(['fact', 'calculation', 'evidence']);
  const warnings: string[] = [];
  for (const calc of graph.byKind.calculation) {
    const declared = new Set(calc.deps);
    const used = [...new Set(stringValues(calc.obj?.inputs))].filter((s) => {
      const node = graph.objects.get(s);
      return node !== undefined && inputKinds.has(node.kind) && s !== calc.id;
    });
    const missing = used.filter((id) => !declared.has(id)).sort(compareIds);
    if (missing.length > 0) {
      warnings.push(`calculation ${calc.id}: inputs reference ${missing.join(', ')} not declared in dependsOn`);
    }
  }
  const factIds = graph.byKind.fact.map((f) => f.id).sort(compareIds);
  for (const judgment of graph.byKind.judgment) {
    const basis = typeof judgment.obj?.basis === 'string' ? judgment.obj.basis : '';
    if (basis.length === 0) continue;
    const declared = new Set(stringList(judgment.obj?.basisFacts));
    const missing = factIds.filter((id) => !declared.has(id) && mentionsId(basis, id));
    if (missing.length > 0) {
      warnings.push(`judgment ${judgment.id}: basis mentions ${missing.join(', ')} not declared in basisFacts`);
    }
  }
  outcomes.push(
    warnings.length === 0
      ? { id: 'core.graph.declaredInputs', status: 'pass' }
      : { id: 'core.graph.declaredInputs', status: 'warning', message: capped(warnings) }
  );

  // core.digests.match
  const declaredDigests = manifest?.digests;
  if (!declaredDigests || typeof declaredDigests !== 'object') {
    outcomes.push({ id: 'core.digests.match', status: 'notRun', message: 'manifest declares no digests block' });
  } else if (declaredDigests.algorithm !== DIGEST_ALGORITHM) {
    outcomes.push({
      id: 'core.digests.match',
      status: 'fail',
      message: `unknown digests algorithm "${declaredDigests.algorithm}" (this verifier implements ${DIGEST_ALGORITHM})`,
    });
  } else {
    let computed: Digests | undefined;
    try {
      computed = computeDigests(graph);
    } catch (err) {
      // The manifest commits to roots that are undefined for this package.
      outcomes.push({ id: 'core.digests.match', status: 'fail', message: (err as Error).message });
    }
    if (computed) {
      const mismatches: string[] = [];
      for (const key of ['analysisRoot', 'contentRoot', 'reviewRoot'] as const) {
        if (declaredDigests[key] !== computed[key]) mismatches.push(key);
      }
      for (const kind of Object.keys(declaredDigests.collections ?? {}).sort(compareIds)) {
        const expected = (computed.collections as Record<string, string | undefined>)[kind];
        if (declaredDigests.collections[kind] !== expected) mismatches.push(`collections.${kind}`);
      }
      outcomes.push(
        mismatches.length === 0
          ? { id: 'core.digests.match', status: 'pass' }
          : {
              id: 'core.digests.match',
              status: 'fail',
              message: `manifest digests do not match the recomputed roots: ${mismatches.join(', ')}`,
            }
      );
    }
  }
  return graph;
}

/** approvals.current: an approved action whose contentRoot is no longer the content root is superseded. */
function approvalsCurrent(outcomes: Outcome[], graph: IntegrityGraph): void {
  if (!graph.present.approval) return;
  const withRoot = graph.byKind.approval.filter((a) => typeof a.obj?.contentRoot === 'string');
  if (withRoot.length === 0) {
    outcomes.push({ id: 'approvals.current', status: 'notRun', message: 'no approval records a contentRoot' });
    return;
  }
  let current: string;
  try {
    current = computeDigests(graph).contentRoot;
  } catch (err) {
    outcomes.push({ id: 'approvals.current', status: 'notRun', message: `content root not recomputable: ${(err as Error).message}` });
    return;
  }
  const superseded = withRoot.filter((a) => a.obj.action === 'approved' && a.obj.contentRoot !== current).map((a) => a.id);
  outcomes.push(
    superseded.length === 0
      ? { id: 'approvals.current', status: 'pass' }
      : {
          id: 'approvals.current',
          status: 'reviewRequired',
          message: `the content changed after these approvals were recorded; they are superseded and a new approval is required: ${superseded.join(', ')}`,
        }
  );
}

/* ------------------------------------------------------------------------ */
/* Narrative validators (spec/narrative.md section 6)                        */
/* ------------------------------------------------------------------------ */

const PROSE_KINDS = new Set(['narrative', 'conclusion']);
const CITABLE_KINDS = new Set(['fact', 'claim', 'judgment', 'calculation', 'evidence']);

/**
 * The layout of a package whose organization never configured a memo
 * format (spec/narrative.md section 5). A narrative written against it
 * without a snapshot is checked against this object's self hash.
 */
export const DEFAULT_MEMO_FORMAT = {
  version: 1,
  name: 'Bitwave default',
  sections: [
    { id: 'conclusion', heading: 'Conclusion', level: 1, kind: 'conclusion' },
    { id: 'claims', heading: 'Claims', level: 1, kind: 'claims' },
    { id: 'judgments', heading: 'Judgments', level: 1, kind: 'judgments' },
    { id: 'validations', heading: 'Validations', level: 1, kind: 'validations' },
    { id: 'review-actions', heading: 'Review actions', level: 1, kind: 'signoff' },
    { id: 'attestations', heading: 'Analysis attestations', level: 1, kind: 'attestations' },
  ],
};

/** No line breaks or other control characters: the shape of a heading. */
export function isSingleLine(s: string): boolean {
  return !/[\x00-\x1f\x7f]/.test(s);
}

/** No control characters other than line breaks and tabs: the shape of prose. */
export function isPlainText(s: string): boolean {
  return !/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/.test(s);
}

interface MemoFormatInUse {
  format: any;
  /** selfHash('memo-format', format), the value a narrative's formatHash must equal. */
  hash?: string;
  error?: string;
}

function memoFormatInUse(pkg: PackageSource): MemoFormatInUse {
  if (!pkg.has(MEMO_FORMAT_PATH)) {
    return { format: DEFAULT_MEMO_FORMAT, hash: selfHash('memo-format', DEFAULT_MEMO_FORMAT) };
  }
  let format: any;
  try {
    format = readJson(pkg, MEMO_FORMAT_PATH);
  } catch (err) {
    return { format: undefined, error: `${MEMO_FORMAT_PATH} is not valid JSON: ${err}` };
  }
  const errors = schemaErrors('memo-format', format);
  if (errors.length === 0) {
    if (!(format.sections as any[]).some((s) => PROSE_KINDS.has(s?.kind))) {
      errors.push('at least one section of kind narrative or conclusion is required');
    }
    for (const s of format.sections as any[]) {
      if (!isSingleLine(s.heading)) errors.push(`section "${s.id}" heading must be a single line`);
    }
  }
  if (errors.length > 0) return { format, error: `${MEMO_FORMAT_PATH}: ${capped(errors)}` };
  return { format, hash: selfHash('memo-format', format) };
}

interface ParagraphVisit {
  sectionId: string;
  paragraph: any;
}

/** Every paragraph with the id of the top-level section it belongs to. */
function paragraphsOf(narrative: any): ParagraphVisit[] {
  const out: ParagraphVisit[] = [];
  const walk = (top: string, section: any) => {
    for (const paragraph of section?.paragraphs ?? []) out.push({ sectionId: top, paragraph });
    for (const sub of section?.subsections ?? []) walk(top, sub);
  };
  for (const s of narrative?.sections ?? []) if (typeof s?.id === 'string') walk(s.id, s);
  return out;
}

function narrativeShapeErrors(narrative: any): string[] {
  const errors: string[] = [];
  const seen = new Set<string>();
  const walk = (section: any) => {
    if (typeof section?.heading === 'string' && !isSingleLine(section.heading)) {
      errors.push(`section "${section.id}" heading must be a single line`);
    }
    (section?.paragraphs ?? []).forEach((p: any, i: number) => {
      if (typeof p?.text === 'string' && !isPlainText(p.text)) {
        errors.push(`section "${section.id}" paragraph ${i + 1} contains control characters`);
      }
    });
    for (const sub of section?.subsections ?? []) walk(sub);
  };
  for (const s of narrative?.sections ?? []) {
    if (typeof s?.id === 'string') {
      if (seen.has(s.id)) errors.push(`duplicate section id "${s.id}"`);
      seen.add(s.id);
    }
    walk(s);
  }
  return errors;
}

function formatSection(format: any, id: string): any | undefined {
  return (format?.sections ?? []).find((s: any) => s?.id === id);
}

function narrativeSections(outcomes: Outcome[], narrative: any | undefined, inUse: MemoFormatInUse): void {
  const problems: string[] = [];
  if (inUse.error) {
    problems.push(inUse.error);
  } else {
    if (narrative !== undefined && narrative?.formatHash !== inUse.hash) {
      problems.push('narrative was written against a different memo format (formatHash mismatch)');
    }
    for (const s of narrative?.sections ?? []) {
      const fs = formatSection(inUse.format, s?.id);
      if (!fs) problems.push(`section "${s?.id}" is not a section of the memo format`);
      else if (!PROSE_KINDS.has(fs.kind)) problems.push(`section "${s.id}" is of kind ${fs.kind}, not prose`);
    }
    const counts = new Map<string, number>();
    for (const { sectionId } of paragraphsOf(narrative)) counts.set(sectionId, (counts.get(sectionId) ?? 0) + 1);
    for (const fs of inUse.format.sections ?? []) {
      if (fs?.required === true && PROSE_KINDS.has(fs.kind) && !(counts.get(fs.id) ?? 0)) {
        problems.push(`required section "${fs.id}" has no paragraphs`);
      }
    }
  }
  outcomes.push(
    problems.length === 0
      ? { id: 'narrative.sections', status: 'pass' }
      : { id: 'narrative.sections', status: 'fail', message: capped(problems) }
  );
}

function narrativeValidators(outcomes: Outcome[], pkg: PackageSource, graph: IntegrityGraph): void {
  const hasNarrative = pkg.has(NARRATIVE_PATH);
  if (!hasNarrative && !pkg.has(MEMO_FORMAT_PATH)) return;
  const inUse = memoFormatInUse(pkg);
  if (!hasNarrative) {
    // A format snapshot that demands prose still has to be honored.
    narrativeSections(outcomes, undefined, inUse);
    return;
  }

  // narrative.schemaValid
  let narrative: any;
  try {
    narrative = readJson(pkg, NARRATIVE_PATH);
  } catch (err) {
    outcomes.push({ id: 'narrative.schemaValid', status: 'fail', message: `${NARRATIVE_PATH} is not valid JSON: ${err}` });
    return;
  }
  const shapeErrors = [...schemaErrors('narrative', narrative), ...narrativeShapeErrors(narrative)];
  outcomes.push(
    shapeErrors.length === 0
      ? { id: 'narrative.schemaValid', status: 'pass' }
      : { id: 'narrative.schemaValid', status: 'fail', message: capped(shapeErrors) }
  );
  if (!narrative || typeof narrative !== 'object' || !Array.isArray(narrative.sections)) return;

  // narrative.sections
  narrativeSections(outcomes, narrative, inUse);

  const paragraphs = paragraphsOf(narrative);
  const kindOf = (id: string): string | undefined => {
    const node = graph.objects.get(id);
    return node && CITABLE_KINDS.has(node.kind) ? node.kind : undefined;
  };
  const cites = (p: any): any[] => (Array.isArray(p?.cites) ? p.cites : []);

  // narrative.citations
  const citeProblems: string[] = [];
  const cited = new Set<string>();
  for (const { sectionId, paragraph } of paragraphs) {
    if (cites(paragraph).length === 0) citeProblems.push(`section "${sectionId}": a paragraph cites nothing`);
    for (const c of cites(paragraph)) {
      if (typeof c?.id !== 'string' || kindOf(c.id) === undefined) {
        citeProblems.push(`section "${sectionId}": unknown cite ${typeof c?.id === 'string' ? c.id : JSON.stringify(c)}`);
      } else {
        cited.add(c.id);
      }
    }
  }
  if (citeProblems.length > 0) {
    outcomes.push({ id: 'narrative.citations', status: 'fail', message: capped(citeProblems) });
  } else {
    const uncited = graph.byKind.claim.map((c) => c.id).filter((id) => !cited.has(id));
    outcomes.push(
      uncited.length === 0
        ? { id: 'narrative.citations', status: 'pass' }
        : { id: 'narrative.citations', status: 'reviewRequired', message: `claims not cited by the narrative: ${uncited.join(', ')}` }
    );
  }

  // narrative.roles
  const roleFailures: string[] = [];
  const roleReviews: string[] = [];
  for (const { sectionId, paragraph } of paragraphs) {
    if (paragraph?.role !== 'conclusion') continue;
    const ok = cites(paragraph).some((c) => {
      const k = typeof c?.id === 'string' ? kindOf(c.id) : undefined;
      return k === 'claim' || k === 'judgment';
    });
    if (!ok) roleFailures.push(`section "${sectionId}": a conclusion paragraph cites no claim or judgment`);
  }
  for (const s of narrative.sections) {
    const fs = formatSection(inUse.format, s?.id);
    if (!fs || typeof fs.templateSection !== 'string' || fs.templateSection.length === 0) continue;
    const prefix = `${fs.templateSection}_`;
    const found = paragraphsOf({ sections: [s] }).some(({ paragraph }) =>
      cites(paragraph).some((c) => typeof c?.id === 'string' && c.id.startsWith(prefix))
    );
    if (!found) {
      roleReviews.push(`section "${s.id}" covers template section ${fs.templateSection} but cites nothing with id prefix ${prefix}`);
    }
  }
  if (roleFailures.length > 0) {
    outcomes.push({ id: 'narrative.roles', status: 'fail', message: capped([...roleFailures, ...roleReviews]) });
  } else if (roleReviews.length > 0) {
    outcomes.push({ id: 'narrative.roles', status: 'reviewRequired', message: capped(roleReviews) });
  } else {
    outcomes.push({ id: 'narrative.roles', status: 'pass' });
  }

  // narrative.cites.fresh: every cited hash equals the object's current
  // closure hash. Unresolved and tainted objects are skipped here:
  // narrative.citations and core.graph.refsResolve report them.
  const staleSections = new Map<string, string[]>();
  let hashed = 0;
  for (const { sectionId, paragraph } of paragraphs) {
    for (const c of cites(paragraph)) {
      if (typeof c?.id !== 'string' || typeof c?.hash !== 'string' || c.hash.length === 0) continue;
      const node = graph.objects.get(c.id);
      if (!node || node.tainted || !CITABLE_KINDS.has(node.kind)) continue;
      hashed++;
      if (node.closureHash !== c.hash) {
        const sections = staleSections.get(c.id) ?? [];
        if (!sections.includes(sectionId)) sections.push(sectionId);
        staleSections.set(c.id, sections);
      }
    }
  }
  if (hashed === 0) {
    outcomes.push({ id: 'narrative.cites.fresh', status: 'notRun', message: 'no cite hashes recorded' });
  } else if (staleSections.size > 0) {
    const stale = [...staleSections.keys()].sort(compareIds).map((id) => `${id} (cited by ${staleSections.get(id)!.join(', ')})`);
    outcomes.push({
      id: 'narrative.cites.fresh',
      status: 'fail',
      message: `cited objects changed after the narrative was written; rewrite the sections citing them: ${capped(stale)}`,
    });
    outcomes.push({ id: 'narrative.drift', status: 'notRun', message: 'not evaluated while narrative.cites.fresh fails' });
    return;
  } else {
    outcomes.push({ id: 'narrative.cites.fresh', status: 'pass' });
  }

  // narrative.drift: cites are fresh but the analysis root moved.
  let current: string;
  try {
    current = computeDigests(graph).analysisRoot;
  } catch (err) {
    outcomes.push({ id: 'narrative.drift', status: 'notRun', message: `analysis root not recomputable: ${(err as Error).message}` });
    return;
  }
  outcomes.push(
    narrative.analysisRoot === current
      ? { id: 'narrative.drift', status: 'pass' }
      : {
          id: 'narrative.drift',
          status: 'reviewRequired',
          message: 'the analysis changed since the narrative was written (analysis root moved): something new and uncited exists; review the narrative',
        }
  );
}
