/**
 * Local assembly: turn a Draft + Packet into a spec-valid .vwp package
 * (in memory). Used by validate() for the offline battery and by the runner
 * to write the artifact.
 *
 * Layout follows spec/package.md / the reference CLI:
 *   manifest.json                 — parts with sha256 digests
 *   workproduct.json
 *   provenance/provenance.json    — models[] (exact identity) + toolchain
 *   claims/claims.json, facts/facts.json, judgments/judgments.json
 *   calculations/<id>.json
 *   evidence/evidence.json        — originals + derived text layers
 *   evidence/files/<id><ext>      — original evidence bytes
 *   evidence/text/<id>.txt        — derived text layers (non-text originals)
 *   renderings/memo.md
 *
 * Anchoring rule: facts anchor to the byte surface that quote verification
 * checks. When the original evidence is itself the text layer (a text file
 * whose bytes equal the layer), anchors point at the original id; otherwise
 * the text layer is embedded as derived evidence `<id>_text`
 * (derivedFrom + transform, per spec/evidence.md §4) and anchors point at it.
 */
import AdmZip from 'adm-zip';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { randomUUID } from 'node:crypto';
import { sha256Hex } from '@vwp/cli/lib';
import type { Draft } from './draft.js';
import type { ModelIdentity } from './draft.js';
import type { Packet, PacketEvidence } from './packet.js';
import { isTextPath } from './packet.js';

export const HARNESS_NAME = 'vwp-benchmark-harness';
export const HARNESS_VERSION = '0.1.0';

export interface AssembleOptions {
  modelIdentity: ModelIdentity;
  /** Free-text provenance notes (e.g. budget-exhaustion flag). */
  notes?: string;
  createdAt?: string;
  packageId?: string;
}

function mediaTypeFor(p: string): string {
  const lower = p.toLowerCase();
  if (lower.endsWith('.json')) return 'application/json';
  if (lower.endsWith('.md')) return 'text/markdown';
  if (lower.endsWith('.txt')) return 'text/plain';
  if (lower.endsWith('.csv')) return 'text/csv';
  if (lower.endsWith('.pdf')) return 'application/pdf';
  return 'application/octet-stream';
}

interface EvidencePlan {
  /** Package evidence objects (originals + derived text layers). */
  entries: Record<string, unknown>[];
  /** Package files to embed: relative path -> bytes. */
  files: Map<string, Buffer>;
  /** Packet evidence id -> id facts should anchor to. */
  anchorTarget: Map<string, string>;
}

function planEvidence(evidence: PacketEvidence[]): EvidencePlan {
  const entries: Record<string, unknown>[] = [];
  const files = new Map<string, Buffer>();
  const anchorTarget = new Map<string, string>();
  for (const ev of evidence) {
    const originalBytes = fs.readFileSync(ev.originalPath);
    const ext = path.extname(ev.originalPath) || '.bin';
    const originalRel = `evidence/files/${ev.id}${ext}`;
    files.set(originalRel, originalBytes);
    entries.push({
      id: ev.id,
      type: ev.type,
      title: ev.title,
      path: originalRel,
      sha256: sha256Hex(originalBytes),
    });
    const textBytes = Buffer.from(ev.text, 'utf8');
    if (isTextPath(ev.originalPath) && originalBytes.equals(textBytes)) {
      // The original is its own text layer; quotes verify against it directly.
      anchorTarget.set(ev.id, ev.id);
      continue;
    }
    const textRel = `evidence/text/${ev.id}.txt`;
    files.set(textRel, textBytes);
    entries.push({
      id: `${ev.id}_text`,
      type: 'document',
      title: `${ev.title} (extracted text layer)`,
      path: textRel,
      sha256: sha256Hex(textBytes),
      derivedFrom: ev.id,
      transform: {
        tool: HARNESS_NAME,
        version: HARNESS_VERSION,
        method: isTextPath(ev.originalPath) ? 'text-normalization' : 'pdf-text-extraction',
      },
    });
    anchorTarget.set(ev.id, `${ev.id}_text`);
  }
  return { entries, files, anchorTarget };
}

/** Assemble the draft into a .vwp (ZIP) buffer. */
export function assembleToBuffer(draft: Draft, packet: Packet, opts: AssembleOptions): Buffer {
  const createdAt = opts.createdAt ?? new Date().toISOString();
  const files = new Map<string, Buffer>();
  const putJson = (rel: string, data: unknown) =>
    files.set(rel, Buffer.from(JSON.stringify(data, null, 2) + '\n', 'utf8'));

  const plan = planEvidence(packet.evidence);
  for (const [rel, bytes] of plan.files) files.set(rel, bytes);
  putJson('evidence/evidence.json', { evidence: plan.entries });

  const facts = draft.facts.map((f) => ({
    id: f.id,
    ...(f.slot ? { slot: f.slot } : {}),
    statement: f.statement && f.statement.length > 0 ? f.statement : `${f.factType} = ${JSON.stringify(f.value)}`,
    factType: f.factType,
    ...(f.value === undefined ? {} : { value: f.value }),
    evidenceRefs: f.evidenceRefs.map((ref) => ({
      evidenceId: plan.anchorTarget.get(ref.evidenceId) ?? ref.evidenceId,
      anchors: ref.anchors,
    })),
  }));
  putJson('facts/facts.json', { facts });

  const claims = draft.claims.map((c) => ({
    id: c.id,
    ...(c.stepKey ? { stepKey: c.stepKey } : {}),
    statement: c.statement,
    standardRefs: c.standardRefs,
    assertions: c.assertions,
    supportingFacts: c.supportingFacts,
    supportingEvidence: c.supportingEvidence,
    judgments: c.judgments,
    supportingCalculations: c.supportingCalculations,
  }));
  putJson('claims/claims.json', { claims });

  putJson('judgments/judgments.json', { judgments: draft.judgments });

  const calcPaths: string[] = [];
  for (const calc of draft.calculations) {
    const rel = `calculations/${calc.id}.json`;
    calcPaths.push(rel);
    putJson(rel, calc);
  }

  files.set('renderings/memo.md', Buffer.from(draft.memoMd, 'utf8'));

  putJson('workproduct.json', {
    id: `wp_${packet.challengeId}`,
    title: packet.title,
    domain: 'finance',
    profile: packet.profile,
    status: 'final',
    reviewState: 'reviewed',
  });

  putJson('provenance/provenance.json', {
    createdAt,
    createdBy: { type: 'agent', name: HARNESS_NAME, version: HARNESS_VERSION },
    models: [{ ...opts.modelIdentity, role: 'work product production' }],
    toolchain: [{ name: HARNESS_NAME, version: HARNESS_VERSION }],
    ...(opts.notes ? { notes: opts.notes } : {}),
  });

  const part = (rel: string) => ({
    path: rel,
    mediaType: mediaTypeFor(rel),
    sha256: sha256Hex(files.get(rel)!),
  });
  const partPaths = [
    'claims/claims.json',
    'facts/facts.json',
    'judgments/judgments.json',
    'evidence/evidence.json',
    ...calcPaths,
    ...[...plan.files.keys()].sort(),
    'renderings/memo.md',
  ];
  const manifest = {
    vwpVersion: '0.1',
    packageId: opts.packageId ?? `vwp_${packet.challengeId}_${randomUUID()}`,
    createdAt,
    profile: packet.profile,
    title: packet.title,
    workproduct: part('workproduct.json'),
    provenance: part('provenance/provenance.json'),
    parts: partPaths.map(part),
  };
  putJson('manifest.json', manifest);

  const zip = new AdmZip();
  for (const rel of [...files.keys()].sort()) {
    zip.addFile(rel, files.get(rel)!);
  }
  return zip.toBuffer();
}
