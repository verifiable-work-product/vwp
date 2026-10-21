import { createHash } from 'crypto';
import { canonicalJson, ObjectKind, stripReviewFields } from './canonical';
import { PackageSource, readJson } from './package';

/**
 * VWP integrity graph (spec/integrity.md): self hashes over canonical
 * bytes, closure hashes over an object's support chain, collection roots,
 * and the analysis, content and review roots the manifest's digests block
 * records. Everything here is derivable from the package alone.
 */

export const DIGEST_ALGORITHM = 'vwp-dag/1';

/** The bytes dependency of an evidence entry that declares no byte hash (spec section 3). */
export const EVIDENCE_BYTES_SENTINEL = '-';

/** Collections that have a root (spec section 5), in manifest order. */
export const COLLECTION_KINDS = ['evidence', 'fact', 'calculation', 'judgment', 'claim', 'narrative', 'approval'] as const;
export type CollectionKind = (typeof COLLECTION_KINDS)[number];

/** Kinds of objects the graph indexes by id (referenceable objects). */
export type IndexedKind = Exclude<CollectionKind, 'narrative'> | 'attestation';

export const NARRATIVE_PATH = 'narrative/narrative.json';
export const MEMO_FORMAT_PATH = 'narrative/format.json';

function sha256(text: string): string {
  return createHash('sha256').update(Buffer.from(text, 'utf8')).digest('hex');
}

/** Byte-wise (UTF-8) string order, so every implementation sorts ids alike. */
export function compareIds(a: string, b: string): number {
  return Buffer.compare(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'));
}

/** selfHash(kind, obj) = sha256("vwp/1/self/" + kind + "\n" + JCS(strip(obj))) */
export function selfHash(kind: ObjectKind, obj: unknown): string {
  return sha256(`vwp/1/self/${kind}\n${canonicalJson(stripReviewFields(kind, obj))}`);
}

export interface DepHash {
  id: string;
  hash: string;
}

/** The deps line list: sorted by id, "id:closureHash" joined by newlines. */
function depLines(deps: DepHash[]): string {
  return [...deps]
    .sort((a, b) => compareIds(a.id, b.id))
    .map((d) => `${d.id}:${d.hash}`)
    .join('\n');
}

/** closureHash(kind, obj) = sha256("vwp/1/closure/" + kind + "\n" + selfHash + "\n" + deps) */
export function closureHash(kind: ObjectKind, self: string, deps: DepHash[]): string {
  return sha256(`vwp/1/closure/${kind}\n${self}\n${depLines(deps)}`);
}

/**
 * An evidence entry's closure binds to its bytes: the single dependency is
 * keyed "bytes" and carries the entry's sha256, the external
 * retrieval.contentHash, or "-" when neither is declared.
 */
export function evidenceBytesSha256(evidence: any): string {
  if (typeof evidence?.sha256 === 'string' && evidence.sha256.length > 0) return evidence.sha256;
  const contentHash = evidence?.externalRef?.retrieval?.contentHash;
  if (typeof contentHash === 'string' && contentHash.length > 0) return contentHash;
  return EVIDENCE_BYTES_SENTINEL;
}

export function evidenceClosureHash(self: string, bytesSha256: string): string {
  return closureHash('evidence', self, [{ id: 'bytes', hash: bytesSha256 || EVIDENCE_BYTES_SENTINEL }]);
}

/** collectionRoot(kind) = sha256("vwp/1/root/" + kind + "\n" + join(sorted by id: id + ":" + closureHash, "\n")) */
export function collectionRoot(kind: CollectionKind, members: DepHash[]): string {
  return sha256(`vwp/1/root/${kind}\n${depLines(members)}`);
}

export interface AnalysisRootInputs {
  workproductSelfHash: string;
  provenanceSelfHash: string;
  roots: Record<'evidence' | 'fact' | 'calculation' | 'judgment' | 'claim', string>;
}

export function analysisRoot(inputs: AnalysisRootInputs): string {
  return sha256(
    [
      'vwp/1/analysis',
      `workproduct:${inputs.workproductSelfHash}`,
      `provenance:${inputs.provenanceSelfHash}`,
      `evidence:${inputs.roots.evidence}`,
      `fact:${inputs.roots.fact}`,
      `calculation:${inputs.roots.calculation}`,
      `judgment:${inputs.roots.judgment}`,
      `claim:${inputs.roots.claim}`,
    ].join('\n')
  );
}

export function contentRoot(analysis: string, narrativeRoot: string): string {
  return sha256(`vwp/1/content\n${analysis}\nnarrative:${narrativeRoot}`);
}

export interface JudgmentStatus {
  id: string;
  status: string;
}

export function reviewRoot(judgments: JudgmentStatus[], approvalRoot: string): string {
  const lines = [...judgments]
    .sort((a, b) => compareIds(a.id, b.id))
    .map((j) => `${j.id}:${j.status}`)
    .join('\n');
  return sha256(`vwp/1/review\n${lines}\napproval:${approvalRoot}`);
}

/** A fact evidence ref is a bare evidence id or an object with evidenceId. */
function refEvidenceId(ref: any): string {
  if (typeof ref === 'string') return ref;
  if (ref && typeof ref.evidenceId === 'string') return ref.evidenceId;
  return '';
}

/** One declared dependency: the field it was declared in and the id it names. */
export interface Edge {
  field: string;
  ref: string;
}

/** Every cite of a narrative section (subsections included), in document order. */
function sectionCites(section: any): string[] {
  const ids: string[] = [];
  const walk = (s: any) => {
    for (const p of s?.paragraphs ?? []) {
      for (const c of p?.cites ?? []) ids.push(typeof c?.id === 'string' ? c.id : '');
    }
    for (const sub of s?.subsections ?? []) walk(sub);
  };
  walk(section);
  return ids;
}

/**
 * Dependency edges of an object (spec section 4), in document order and
 * with duplicates. For kind narrative the object is one top-level section.
 * Evidence has no id-keyed dependencies (its bytes dependency is handled by
 * evidenceClosureHash); approvals and attestations have none.
 */
export function dependencyEdges(kind: ObjectKind, obj: any): Edge[] {
  const list = (field: string): Edge[] =>
    (Array.isArray(obj?.[field]) ? obj[field] : []).map((v: unknown) => ({ field, ref: typeof v === 'string' ? v : '' }));
  switch (kind) {
    case 'fact':
      return (Array.isArray(obj?.evidenceRefs) ? obj.evidenceRefs : []).map((r: unknown) => ({ field: 'evidenceRefs', ref: refEvidenceId(r) }));
    case 'calculation':
      return list('dependsOn');
    case 'judgment':
      return [...list('basisFacts'), ...list('basisCalculations')];
    case 'claim':
      return [...list('supportingFacts'), ...list('supportingEvidence'), ...list('supportingCalculations'), ...list('judgments')];
    case 'narrative':
      return sectionCites(obj).map((ref) => ({ field: 'cites', ref }));
    default:
      return [];
  }
}

/** Dependency ids of an object, de-duplicated, in document order. */
export function dependencyIds(kind: ObjectKind, obj: any): string[] {
  return [...new Set(dependencyEdges(kind, obj).map((e) => e.ref))];
}

export interface GraphObject {
  id: string;
  kind: IndexedKind | 'narrative';
  obj: any;
  /** Declared dependencies in document order. */
  edges: Edge[];
  /** De-duplicated dependency ids in document order. */
  deps: string[];
  selfHash: string;
  /** Empty when tainted. */
  closureHash: string;
  /** The object, or something it transitively depends on, is unresolved or cyclic. */
  tainted: boolean;
}

export interface GraphProblem {
  kind: 'unresolved' | 'duplicate' | 'cycle' | 'unreadable';
  /** The object (or file) the problem was found on. */
  from: string;
  /** The id (or path) the problem is about. */
  id: string;
  message: string;
}

export interface IntegrityGraph {
  /** Referenceable objects by id: evidence, facts, calculations, judgments, claims, approvals, attestations. */
  objects: Map<string, GraphObject>;
  /** Objects per collection kind, in document order. Narrative holds one object per top-level section. */
  byKind: Record<CollectionKind | 'attestation', GraphObject[]>;
  workproduct?: any;
  provenance?: any;
  narrative?: any;
  memoFormat?: any;
  problems: GraphProblem[];
  /** Which collection files exist (so validators can skip what is absent). */
  present: Record<CollectionKind, boolean>;
}

/** True when every dependency resolved, no id is ambiguous and no cycle exists: the roots are defined. */
export function isComplete(graph: IntegrityGraph): boolean {
  return !graph.problems.some((p) => p.kind === 'unresolved' || p.kind === 'cycle' || p.kind === 'duplicate');
}

function readCollection(
  pkg: PackageSource,
  relPath: string,
  key: string,
  problems: GraphProblem[]
): { present: boolean; items: any[] } {
  if (!pkg.has(relPath)) return { present: false, items: [] };
  try {
    const doc = readJson(pkg, relPath) as any;
    return { present: true, items: Array.isArray(doc?.[key]) ? doc[key] : [] };
  } catch (err) {
    problems.push({ kind: 'unreadable', from: relPath, id: relPath, message: `${relPath} is not valid JSON: ${err}` });
    return { present: true, items: [] };
  }
}

function readObject(pkg: PackageSource, relPath: string, problems: GraphProblem[]): any {
  if (!pkg.has(relPath)) return undefined;
  try {
    return readJson(pkg, relPath);
  } catch (err) {
    problems.push({ kind: 'unreadable', from: relPath, id: relPath, message: `${relPath} is not valid JSON: ${err}` });
    return undefined;
  }
}

/**
 * Objects stored one per file under a directory, keyed by their string id.
 * A file whose top-level object has no string id is not an object of the
 * graph (for example an accounting schedule under calculations/) and is
 * left to the other validators.
 */
function readPerFile(pkg: PackageSource, pattern: RegExp): { present: boolean; items: any[] } {
  const paths = pkg.list().filter((p) => pattern.test(p)).sort(compareIds);
  const items: any[] = [];
  for (const p of paths) {
    let doc: any;
    try {
      doc = readJson(pkg, p);
    } catch {
      continue;
    }
    if (doc && typeof doc === 'object' && typeof doc.id === 'string' && doc.id.length > 0) items.push(doc);
  }
  return { present: paths.length > 0, items };
}

export function readCalculations(pkg: PackageSource): { present: boolean; items: any[] } {
  return readPerFile(pkg, /^calculations\/[^/]+\.json$/);
}

/** Index every object of a package, compute its hashes, and report graph problems. */
export function buildGraph(pkg: PackageSource): IntegrityGraph {
  const problems: GraphProblem[] = [];
  const evidence = readCollection(pkg, 'evidence/evidence.json', 'evidence', problems);
  const facts = readCollection(pkg, 'facts/facts.json', 'facts', problems);
  const calculations = readCalculations(pkg);
  const attestations = readPerFile(pkg, /^attestations\/[^/]+\.json$/);
  const judgments = readCollection(pkg, 'judgments/judgments.json', 'judgments', problems);
  const claims = readCollection(pkg, 'claims/claims.json', 'claims', problems);
  const approvals = readCollection(pkg, 'approvals/approvals.json', 'approvals', problems);
  const workproduct = readObject(pkg, 'workproduct.json', problems);
  const provenance = readObject(pkg, 'provenance/provenance.json', problems);
  const narrative = readObject(pkg, NARRATIVE_PATH, problems);
  const memoFormat = readObject(pkg, MEMO_FORMAT_PATH, problems);

  const objects = new Map<string, GraphObject>();
  const byKind: IntegrityGraph['byKind'] = {
    evidence: [],
    fact: [],
    calculation: [],
    judgment: [],
    claim: [],
    narrative: [],
    approval: [],
    attestation: [],
  };
  const present: Record<CollectionKind, boolean> = {
    evidence: evidence.present,
    fact: facts.present,
    calculation: calculations.present,
    judgment: judgments.present,
    claim: claims.present,
    narrative: narrative !== undefined,
    approval: approvals.present,
  };

  const node = (kind: GraphObject['kind'], id: string, obj: any): GraphObject => {
    const edges = dependencyEdges(kind, obj);
    return {
      id,
      kind,
      obj,
      edges,
      deps: [...new Set(edges.map((e) => e.ref))],
      selfHash: selfHash(kind, obj),
      closureHash: '',
      tainted: false,
    };
  };
  const index = (kind: IndexedKind, obj: any) => {
    const id = obj?.id;
    if (typeof id !== 'string' || id.length === 0) return;
    const existing = objects.get(id);
    if (existing) {
      problems.push({
        kind: 'duplicate',
        from: `${kind} ${id}`,
        id,
        message: `${kind} ${id}: id already used by a ${existing.kind}`,
      });
      return;
    }
    const n = node(kind, id, obj);
    objects.set(id, n);
    byKind[kind].push(n);
  };

  for (const ev of evidence.items) index('evidence', ev);
  for (const f of facts.items) index('fact', f);
  for (const c of calculations.items) index('calculation', c);
  for (const a of attestations.items) index('attestation', a);
  for (const j of judgments.items) index('judgment', j);
  for (const c of claims.items) index('claim', c);
  for (const a of approvals.items) index('approval', a);
  for (const s of Array.isArray(narrative?.sections) ? narrative.sections : []) {
    byKind.narrative.push(node('narrative', typeof s?.id === 'string' ? s.id : '', s));
  }

  // Closure hashes bottom-up with cycle detection. An object whose
  // dependency is unresolved or on a cycle, directly or transitively, is
  // tainted and gets no closure hash: the roots are undefined then.
  const state = new Map<string, 'inProgress' | 'done'>();
  const resolve = (n: GraphObject, stack: string[]): void => {
    state.set(n.id, 'inProgress');
    for (const e of n.edges) {
      if (e.ref === '' || !objects.has(e.ref)) {
        problems.push({
          kind: 'unresolved',
          from: `${n.kind} ${n.id}`,
          id: e.ref,
          message: `${n.kind} ${n.id}: unknown ${e.field} ref ${e.ref}`,
        });
        n.tainted = true;
      }
    }
    const deps: DepHash[] = [];
    for (const id of n.deps) {
      const dep = objects.get(id);
      if (!dep) continue;
      const s = state.get(id);
      if (s === 'inProgress') {
        const start = Math.max(stack.indexOf(id), 0);
        const cycle = [...stack.slice(start), id];
        problems.push({ kind: 'cycle', from: `${n.kind} ${n.id}`, id, message: cycle.join(' -> ') });
        n.tainted = true;
        continue;
      }
      if (s === undefined) resolve(dep, [...stack, id]);
      if (dep.tainted) n.tainted = true;
      deps.push({ id, hash: dep.closureHash });
    }
    if (!n.tainted) {
      n.closureHash =
        n.kind === 'evidence' ? evidenceClosureHash(n.selfHash, evidenceBytesSha256(n.obj)) : closureHash(n.kind, n.selfHash, deps);
    }
    state.set(n.id, 'done');
  };
  for (const id of [...objects.keys()].sort(compareIds)) {
    if (!state.has(id)) resolve(objects.get(id)!, [id]);
  }
  // Narrative sections depend on objects only: nothing depends on them and
  // they cannot cycle.
  for (const section of byKind.narrative) resolve(section, [section.id]);

  return { objects, byKind, workproduct, provenance, narrative, memoFormat, problems, present };
}

export interface Digests {
  algorithm: typeof DIGEST_ALGORITHM;
  analysisRoot: string;
  contentRoot: string;
  reviewRoot: string;
  collections: Record<CollectionKind, string>;
}

function rootOf(graph: IntegrityGraph, kind: CollectionKind): string {
  return collectionRoot(
    kind,
    graph.byKind[kind].map((n) => ({ id: n.id, hash: n.closureHash }))
  );
}

/**
 * The manifest digests block (spec section 5) recomputed from a package or
 * a built graph. Throws when the roots are undefined: the graph has an
 * unresolved reference, an ambiguous id or a cycle, or the work product or
 * provenance part is missing.
 */
export function computeDigests(input: PackageSource | IntegrityGraph): Digests {
  const graph = 'objects' in input ? input : buildGraph(input);
  const blocker = graph.problems.find((p) => p.kind === 'unresolved' || p.kind === 'cycle' || p.kind === 'duplicate');
  if (blocker) {
    const what = blocker.kind === 'unresolved' ? 'unresolved reference' : blocker.kind === 'cycle' ? 'dependency cycle' : 'ambiguous id';
    throw new Error(`failed to compute digests: ${what}: ${blocker.message}`);
  }
  if (graph.workproduct === undefined) throw new Error('failed to compute digests: workproduct.json is missing');
  if (graph.provenance === undefined) throw new Error('failed to compute digests: provenance/provenance.json is missing');
  const collections = {
    evidence: rootOf(graph, 'evidence'),
    fact: rootOf(graph, 'fact'),
    calculation: rootOf(graph, 'calculation'),
    judgment: rootOf(graph, 'judgment'),
    claim: rootOf(graph, 'claim'),
    narrative: rootOf(graph, 'narrative'),
    approval: rootOf(graph, 'approval'),
  };
  const analysis = analysisRoot({
    workproductSelfHash: selfHash('workproduct', graph.workproduct),
    provenanceSelfHash: selfHash('provenance', graph.provenance),
    roots: collections,
  });
  const judgmentStatuses = graph.byKind.judgment.map((j) => ({
    id: j.id,
    status: typeof j.obj?.status === 'string' ? j.obj.status : '',
  }));
  return {
    algorithm: DIGEST_ALGORITHM,
    analysisRoot: analysis,
    contentRoot: contentRoot(analysis, collections.narrative),
    reviewRoot: reviewRoot(judgmentStatuses, collections.approval),
    collections,
  };
}
