/**
 * Packet acquisition: remote (VWP-TT platform packet endpoint) or a local
 * packet directory. Either way the harness works from one normalized Packet.
 *
 * Local packet directory layout:
 *   packet.json                 — same shape as the remote packet JSON
 *   evidence/<file>             — original evidence bytes (paths per packet.json)
 *   textlayers/<evidenceId>.txt — extracted text layer per evidence (optional
 *                                 for text-format originals, which are their
 *                                 own text layer)
 *
 * Evidence text layers are the anchoring surface: quotes verify against these
 * bytes. Multi-page text layers separate pages with form feed (\f).
 */
import * as fs from 'node:fs';
import * as path from 'node:path';

export interface RequiredFactSlot {
  slot: string;
  factType: string;
  valueShape: string;
  unit?: string;
  enumValues?: string[];
  description?: string;
}

export interface MethodStep {
  key: string;
  title: string;
  question: string;
  authorityRefs?: string[];
  guidance?: string;
  expects?: string[];
}

export interface PacketEvidence {
  id: string;
  title: string;
  type: string;
  /** Text layer split into pages (form-feed separated in the raw layer). */
  pages: string[];
  /** The raw text layer (exact bytes quotes verify against). */
  text: string;
  /** Absolute path to the original evidence bytes on disk. */
  originalPath: string;
}

export interface Packet {
  challengeId: string;
  title: string;
  profile: string;
  brief: string;
  deliverableSpec: string;
  requiredFacts: RequiredFactSlot[];
  methodSteps: MethodStep[];
  evidence: PacketEvidence[];
}

const TEXT_EXTENSIONS = ['.txt', '.md', '.csv', '.json'];

export function isTextPath(p: string): boolean {
  const lower = p.toLowerCase();
  return TEXT_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

export function splitPages(text: string): string[] {
  return text.split('\f');
}

/* eslint-disable @typescript-eslint/no-explicit-any */

function normalizeSlot(raw: any): RequiredFactSlot {
  let enumValues = raw.enumValues ?? raw.enum_values;
  if (typeof enumValues === 'string') {
    try {
      enumValues = JSON.parse(enumValues);
    } catch {
      enumValues = undefined;
    }
  }
  return {
    slot: raw.slot,
    factType: raw.factType ?? raw.fact_type,
    valueShape: raw.valueShape ?? raw.value_shape,
    unit: raw.unit ?? undefined,
    enumValues: Array.isArray(enumValues) ? enumValues : undefined,
    description: raw.description ?? undefined,
  };
}

function normalizeMethodSteps(raw: any): MethodStep[] {
  let template = raw.methodTemplate ?? raw.method_template;
  if (typeof template === 'string') {
    try {
      template = JSON.parse(template);
    } catch {
      template = undefined;
    }
  }
  const steps = raw.methodSteps ?? template?.steps ?? [];
  return (Array.isArray(steps) ? steps : []).map((s: any) => ({
    key: s.key,
    title: s.title,
    question: s.question,
    authorityRefs: s.authorityRefs ?? undefined,
    guidance: s.guidance ?? undefined,
    expects: s.expects ?? undefined,
  }));
}

function packetFields(raw: any): Omit<Packet, 'evidence'> {
  return {
    challengeId: raw.challengeId ?? raw.slug ?? raw.id ?? 'challenge',
    title: raw.title ?? String(raw.challengeId ?? 'VWP benchmark challenge'),
    profile: raw.profile ?? 'vwp-finance',
    brief: raw.briefMd ?? raw.brief ?? '',
    deliverableSpec: raw.deliverableSpecMd ?? raw.deliverableSpec ?? '',
    requiredFacts: (raw.requiredFacts ?? raw.slots ?? []).map(normalizeSlot),
    methodSteps: normalizeMethodSteps(raw),
  };
}

/** Read a packet from a local directory (see layout above). */
export function loadLocalPacket(dir: string): Packet {
  const root = path.resolve(dir);
  const rawJson = fs.readFileSync(path.join(root, 'packet.json'), 'utf8');
  const raw = JSON.parse(rawJson);
  const evidence: PacketEvidence[] = [];
  for (const e of raw.evidence ?? []) {
    const id = e.id ?? e.evidenceId ?? e.evidence_id;
    if (!id || !e.path) throw new Error(`packet.json evidence entry needs id and path: ${JSON.stringify(e)}`);
    const originalPath = path.join(root, e.path);
    if (!fs.existsSync(originalPath)) throw new Error(`evidence file not found: ${originalPath}`);
    const textLayerPath = path.join(root, 'textlayers', `${id}.txt`);
    let text: string;
    if (fs.existsSync(textLayerPath)) {
      text = fs.readFileSync(textLayerPath, 'utf8');
    } else if (isTextPath(originalPath)) {
      text = fs.readFileSync(originalPath, 'utf8');
    } else {
      throw new Error(`evidence ${id} is not text and has no textlayers/${id}.txt`);
    }
    evidence.push({
      id,
      title: e.title ?? id,
      type: e.type ?? 'document',
      text,
      pages: splitPages(text),
      originalPath,
    });
  }
  return { ...packetFields(raw), evidence };
}

async function fetchOk(url: string, apiKey: string): Promise<Response> {
  const res = await fetch(url, { headers: { Authorization: `Bearer ${apiKey}` } });
  if (!res.ok) {
    const body = await res.text().catch(() => '');
    throw new Error(`GET ${url} failed: ${res.status} ${res.statusText} ${body.slice(0, 300)}`);
  }
  return res;
}

function extFor(entry: any, url: string): string {
  const fromUrl = path.extname(new URL(url, 'http://x/').pathname);
  if (fromUrl) return fromUrl;
  const mime: string | undefined = entry.mime;
  if (mime?.includes('pdf')) return '.pdf';
  if (mime?.startsWith('text/')) return '.txt';
  return '.bin';
}

/**
 * Fetch a packet from a platform packet URL with an enrolled llm_runner key,
 * downloading each evidence entry's original bytes and text layer into workDir.
 */
export async function loadRemotePacket(packetUrl: string, apiKey: string, workDir: string): Promise<Packet> {
  const res = await fetchOk(packetUrl, apiKey);
  const raw: any = await res.json();
  const evidenceDir = path.join(workDir, 'evidence');
  fs.mkdirSync(evidenceDir, { recursive: true });
  const evidence: PacketEvidence[] = [];
  for (const e of raw.evidence ?? []) {
    const id = e.evidenceId ?? e.evidence_id ?? e.id;
    if (!id || !e.url) throw new Error(`remote evidence entry needs an id and url: ${JSON.stringify(e)}`);
    const fileUrl = new URL(e.url, packetUrl).toString();
    const bytes = Buffer.from(await (await fetchOk(fileUrl, apiKey)).arrayBuffer());
    const originalPath = path.join(evidenceDir, `${id}${extFor(e, fileUrl)}`);
    fs.writeFileSync(originalPath, bytes);
    let text: string;
    const textUrl = e.textUrl ?? e.text_url;
    if (textUrl) {
      text = await (await fetchOk(new URL(textUrl, packetUrl).toString(), apiKey)).text();
    } else if (isTextPath(originalPath)) {
      text = bytes.toString('utf8');
    } else {
      throw new Error(`remote evidence ${id} is binary and exposes no text layer (textUrl)`);
    }
    evidence.push({
      id,
      title: e.title ?? id,
      type: e.type ?? 'document',
      text,
      pages: splitPages(text),
      originalPath,
    });
  }
  return { ...packetFields(raw), evidence };
}
