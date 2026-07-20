import { PackageSource, readJson } from './package';
import { sha256Hex } from './hashes';
import { extensionSchemaErrors, schemaErrors, SchemaName } from './schema';

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
      status: 'notRun',
      message: 'calculations/ssp-allocation.json not present',
    });
    outcomes.push({ id: 'asc606.allocation.tieOut', status: 'notRun', message: 'calculations/ssp-allocation.json not present' });
  } else {
    const errs = extensionSchemaErrors('extensions/accounting-asc606/ssp-allocation.schema.json', allocation);
    if (errs.length > 0) {
      outcomes.push({ id: 'asc606.allocation.schemaValid', status: 'fail', message: errs.slice(0, 5).join('; ') });
      outcomes.push({ id: 'asc606.allocation.tieOut', status: 'notRun', message: 'allocation failed extension schema validation' });
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
      status: 'notRun',
      message: 'calculations/revenue-schedule.csv not present',
    });
    return;
  }
  if (!allocationValid) {
    outcomes.push({
      id: 'asc606.revenueSchedule.total',
      status: 'notRun',
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
