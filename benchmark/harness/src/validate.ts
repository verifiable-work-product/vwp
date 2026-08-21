/**
 * The full offline check battery (harness DESIGN §3 'Finishing'): everything
 * computable without vault material.
 *
 *   verify — package verification via the reference CLI (schemas, link
 *            resolution, verbatim-quote anchors) on the assembled .vwp
 *   G1     — every fact evidence ref resolves; no ref-less facts
 *   G2     — every quote anchor verifies against its text layer; anchored-fact
 *            rate >= 0.9
 *   C1     — claim support links resolve; no orphan (support-less) claims
 *   K1     — conventional calculation types recompute (tolerance 0.01)
 *   S2     — internal consistency: no duplicate factType with conflicting values
 *   P1     — every required-facts slot is filled by a fact of the manifest's
 *            factType whose value conforms to the declared valueShape
 *
 * Never anything key-derived: pre-submit feedback is limited to what a
 * diligent human could equivalently self-check.
 */
import { openPackageBuffer, verifyPackage, type VerifyReport } from '@vwp/cli/lib';
import type { Calculation, Draft, Fact } from './draft.js';
import type { Packet, RequiredFactSlot } from './packet.js';
import { assembleToBuffer, type AssembleOptions } from './assemble.js';

/**
 * Normalization applied to both quote and evidence text before matching —
 * identical to the reference CLI's normalizeForQuoteMatch (spec/facts.md):
 * NFC, ASCII quotes/dashes, collapsed whitespace.
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

export function quoteAppearsIn(quote: string, text: string): boolean {
  return normalizeForQuoteMatch(text).includes(normalizeForQuoteMatch(quote));
}

// ---------------------------------------------------------------------------
// K1 — conventional calculation recomputation
// ---------------------------------------------------------------------------

export const CONVENTIONAL_CALC_TYPES = [
  'sum',
  'schedule',
  'journalEntries',
  'relativeStandaloneSellingPriceAllocation',
] as const;

export const RECOMPUTE_TOLERANCE = 0.01;

/** Parse a number from a number or decimal string ("1234.56", "1,234.56"). */
export function toNumber(v: unknown): number | undefined {
  if (typeof v === 'number') return Number.isFinite(v) ? v : undefined;
  if (typeof v === 'string') {
    const cleaned = v.trim().replace(/,/g, '');
    if (/^-?\d+(\.\d+)?$/.test(cleaned)) return Number.parseFloat(cleaned);
  }
  return undefined;
}

function near(a: number, b: number, tol = RECOMPUTE_TOLERANCE): boolean {
  return Math.abs(a - b) <= tol;
}

/* eslint-disable @typescript-eslint/no-explicit-any */

type CalcLike = Pick<Calculation, 'type' | 'inputs' | 'outputs'> & { id?: string };

export interface RecomputeResult {
  /** false when the type is not a conventional recomputable type. */
  applicable: boolean;
  problems: string[];
}

function recomputeSum(calc: CalcLike, problems: string[]): void {
  const values: unknown[] = Array.isArray((calc.inputs as any).values) ? (calc.inputs as any).values : [];
  if (values.length === 0) {
    problems.push("sum: inputs.values must be a non-empty array of numbers");
    return;
  }
  let total = 0;
  for (const [i, v] of values.entries()) {
    const n = toNumber(v);
    if (n === undefined) {
      problems.push(`sum: inputs.values[${i}] is not numeric: ${JSON.stringify(v)}`);
      return;
    }
    total += n;
  }
  const declared = toNumber((calc.outputs as any).total);
  if (declared === undefined) {
    problems.push('sum: outputs.total is missing or not numeric');
  } else if (!near(total, declared)) {
    problems.push(`sum: outputs.total = ${declared} but inputs sum to ${total}`);
  }
}

function recomputeSchedule(calc: CalcLike, problems: string[]): void {
  const rows: any[] = Array.isArray((calc.inputs as any).rows) ? (calc.inputs as any).rows : [];
  if (rows.length === 0) {
    problems.push('schedule: inputs.rows must be a non-empty array of { values: number[] } rows');
    return;
  }
  const rowTotals: number[] = [];
  const columnTotals: number[] = [];
  let grandTotal = 0;
  for (const [i, row] of rows.entries()) {
    const values: unknown[] = Array.isArray(row?.values) ? row.values : [];
    if (values.length === 0) {
      problems.push(`schedule: inputs.rows[${i}].values must be a non-empty array of numbers`);
      return;
    }
    let rowTotal = 0;
    for (const [j, v] of values.entries()) {
      const n = toNumber(v);
      if (n === undefined) {
        problems.push(`schedule: inputs.rows[${i}].values[${j}] is not numeric: ${JSON.stringify(v)}`);
        return;
      }
      rowTotal += n;
      columnTotals[j] = (columnTotals[j] ?? 0) + n;
    }
    rowTotals.push(rowTotal);
    grandTotal += rowTotal;
  }
  const out = calc.outputs as any;
  const declaredGrand = toNumber(out.grandTotal);
  if (declaredGrand === undefined) {
    problems.push('schedule: outputs.grandTotal is missing or not numeric');
  } else if (!near(grandTotal, declaredGrand)) {
    problems.push(`schedule: outputs.grandTotal = ${declaredGrand} but rows sum to ${grandTotal}`);
  }
  const checkVector = (name: 'rowTotals' | 'columnTotals', expected: number[]) => {
    if (out[name] === undefined) return; // optional outputs; grandTotal is the required tie
    if (!Array.isArray(out[name]) || out[name].length !== expected.length) {
      problems.push(`schedule: outputs.${name} should have ${expected.length} entries`);
      return;
    }
    for (const [i, v] of (out[name] as unknown[]).entries()) {
      const n = toNumber(v);
      if (n === undefined || !near(n, expected[i])) {
        problems.push(`schedule: outputs.${name}[${i}] = ${JSON.stringify(v)} but recomputes to ${expected[i]}`);
      }
    }
  };
  checkVector('rowTotals', rowTotals);
  checkVector('columnTotals', columnTotals);
}

function recomputeJournalEntries(calc: CalcLike, problems: string[]): void {
  const entries: any[] = Array.isArray((calc.inputs as any).entries) ? (calc.inputs as any).entries : [];
  if (entries.length === 0) {
    problems.push('journalEntries: inputs.entries must be a non-empty array of { lines: [{ debit?, credit? }] }');
    return;
  }
  let totalDebits = 0;
  let totalCredits = 0;
  for (const [i, entry] of entries.entries()) {
    const lines: any[] = Array.isArray(entry?.lines) ? entry.lines : [];
    if (lines.length === 0) {
      problems.push(`journalEntries: inputs.entries[${i}].lines must be a non-empty array`);
      return;
    }
    for (const [j, line] of lines.entries()) {
      const debit = line?.debit === undefined ? 0 : toNumber(line.debit);
      const credit = line?.credit === undefined ? 0 : toNumber(line.credit);
      if (debit === undefined || credit === undefined) {
        problems.push(`journalEntries: inputs.entries[${i}].lines[${j}] debit/credit not numeric`);
        return;
      }
      totalDebits += debit;
      totalCredits += credit;
    }
  }
  const out = calc.outputs as any;
  const declaredDebits = toNumber(out.totalDebits);
  const declaredCredits = toNumber(out.totalCredits);
  if (declaredDebits === undefined || !near(totalDebits, declaredDebits)) {
    problems.push(`journalEntries: outputs.totalDebits = ${JSON.stringify(out.totalDebits)} but lines sum to ${totalDebits}`);
  }
  if (declaredCredits === undefined || !near(totalCredits, declaredCredits)) {
    problems.push(`journalEntries: outputs.totalCredits = ${JSON.stringify(out.totalCredits)} but lines sum to ${totalCredits}`);
  }
}

function recomputeRsspAllocation(calc: CalcLike, problems: string[]): void {
  const inp = calc.inputs as any;
  const tp = toNumber(inp.transactionPrice);
  const items: any[] = Array.isArray(inp.items) ? inp.items : [];
  if (tp === undefined || items.length === 0) {
    problems.push(
      'relativeStandaloneSellingPriceAllocation: inputs must carry transactionPrice and non-empty items[{name, ssp}]'
    );
    return;
  }
  const ssps: number[] = [];
  for (const [i, item] of items.entries()) {
    const ssp = toNumber(item?.ssp);
    if (ssp === undefined) {
      problems.push(`relativeStandaloneSellingPriceAllocation: inputs.items[${i}].ssp is not numeric`);
      return;
    }
    ssps.push(ssp);
  }
  const totalSSP = ssps.reduce((a, b) => a + b, 0);
  if (totalSSP === 0) {
    problems.push('relativeStandaloneSellingPriceAllocation: total SSP is zero');
    return;
  }
  const allocations: any[] = Array.isArray((calc.outputs as any).allocations) ? (calc.outputs as any).allocations : [];
  if (allocations.length !== items.length) {
    problems.push(
      `relativeStandaloneSellingPriceAllocation: outputs.allocations should have ${items.length} entries (one per input item, in order)`
    );
    return;
  }
  let allocatedTotal = 0;
  for (const [i, alloc] of allocations.entries()) {
    const declared = toNumber(alloc?.allocatedAmount);
    const expected = (tp * ssps[i]) / totalSSP;
    if (declared === undefined || !near(declared, expected)) {
      problems.push(
        `relativeStandaloneSellingPriceAllocation: outputs.allocations[${i}].allocatedAmount = ${JSON.stringify(
          alloc?.allocatedAmount
        )} but tp*ssp/totalSSP recomputes to ${expected.toFixed(2)}`
      );
    }
    allocatedTotal += declared ?? 0;
  }
  if (!near(allocatedTotal, tp)) {
    problems.push(
      `relativeStandaloneSellingPriceAllocation: allocations sum to ${allocatedTotal} but must tie out to transactionPrice ${tp}`
    );
  }
}

/** Recompute a conventional calculation type. Custom types are not applicable. */
export function recomputeCalculation(calc: CalcLike): RecomputeResult {
  const problems: string[] = [];
  switch (calc.type) {
    case 'sum':
      recomputeSum(calc, problems);
      break;
    case 'schedule':
      recomputeSchedule(calc, problems);
      break;
    case 'journalEntries':
      recomputeJournalEntries(calc, problems);
      break;
    case 'relativeStandaloneSellingPriceAllocation':
      recomputeRsspAllocation(calc, problems);
      break;
    default:
      return { applicable: false, problems: [] };
  }
  return { applicable: true, problems };
}

// ---------------------------------------------------------------------------
// P1 — value shape conformance
// ---------------------------------------------------------------------------

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/** Returns a problem string when value does not conform to the slot's valueShape. */
export function valueShapeProblem(slot: RequiredFactSlot, value: unknown): string | undefined {
  const v = value as any;
  switch (slot.valueShape) {
    case 'money':
      if (typeof v !== 'object' || v === null || toNumber(v.amount) === undefined || typeof v.currency !== 'string') {
        return `expected money shape { amount, currency }, got ${JSON.stringify(value)}`;
      }
      return undefined;
    case 'quantity':
      if (typeof v !== 'object' || v === null || toNumber(v.value) === undefined || typeof v.unit !== 'string') {
        return `expected quantity shape { value, unit }, got ${JSON.stringify(value)}`;
      }
      if (slot.unit && v.unit !== slot.unit) return `expected unit '${slot.unit}', got '${v.unit}'`;
      return undefined;
    case 'enum':
      if (typeof v !== 'string') return `expected one of ${JSON.stringify(slot.enumValues ?? [])}, got ${JSON.stringify(value)}`;
      if (slot.enumValues && !slot.enumValues.includes(v)) {
        return `'${v}' is not one of ${JSON.stringify(slot.enumValues)}`;
      }
      return undefined;
    case 'date':
      return typeof v === 'string' && DATE_RE.test(v) ? undefined : `expected an RFC 3339 date string, got ${JSON.stringify(value)}`;
    case 'boolean':
      return typeof v === 'boolean' ? undefined : `expected a boolean, got ${JSON.stringify(value)}`;
    case 'string':
      return typeof v === 'string' && v.length > 0 ? undefined : `expected a non-empty string, got ${JSON.stringify(value)}`;
    case 'period':
      if (typeof v !== 'object' || v === null || !DATE_RE.test(String(v.from)) || !DATE_RE.test(String(v.to))) {
        return `expected period shape { from, to } of RFC 3339 dates, got ${JSON.stringify(value)}`;
      }
      return undefined;
    default:
      return value === undefined ? 'expected a value' : undefined;
  }
}

// ---------------------------------------------------------------------------
// The battery
// ---------------------------------------------------------------------------

export type CheckId = 'verify' | 'G1' | 'G2' | 'C1' | 'K1' | 'S2' | 'P1';

export interface Finding {
  check: CheckId;
  status: 'pass' | 'fail';
  message: string;
}

export interface ValidationResult {
  ok: boolean;
  findings: Finding[];
  verify: VerifyReport;
}

/** Canonical form of a fact value for S2 conflict detection. */
function canonicalValue(value: unknown): string {
  const canon = (v: unknown): unknown => {
    const n = toNumber(v);
    if (n !== undefined && (typeof v === 'number' || typeof v === 'string')) return n;
    if (Array.isArray(v)) return v.map(canon);
    if (typeof v === 'object' && v !== null) {
      const out: Record<string, unknown> = {};
      for (const key of Object.keys(v as Record<string, unknown>).sort()) {
        out[key] = canon((v as Record<string, unknown>)[key]);
      }
      return out;
    }
    return v;
  };
  return JSON.stringify(canon(value));
}

function push(findings: Finding[], check: CheckId, problems: string[], passMessage: string): void {
  if (problems.length === 0) {
    findings.push({ check, status: 'pass', message: passMessage });
  } else {
    for (const p of problems.slice(0, 10)) findings.push({ check, status: 'fail', message: p });
    if (problems.length > 10) {
      findings.push({ check, status: 'fail', message: `... and ${problems.length - 10} more ${check} problem(s)` });
    }
  }
}

function factHasQuoteAnchor(f: Fact): boolean {
  return f.evidenceRefs.some((ref) => ref.anchors.some((a) => typeof a.quote === 'string' && a.quote.length > 0));
}

/**
 * Run the full offline battery on the current draft: assemble to an in-memory
 * .vwp, verify it with the reference CLI, then compute G1/G2/C1/K1/S2/P1.
 */
export function runValidation(draft: Draft, packet: Packet, opts: AssembleOptions): ValidationResult {
  const findings: Finding[] = [];

  // Package verification via the reference CLI on the assembled artifact.
  const buffer = assembleToBuffer(draft, packet, opts);
  const verify = verifyPackage(openPackageBuffer(buffer, '(draft assembly)'));
  const verifyFails = verify.outcomes.filter((o) => o.status === 'fail');
  push(
    findings,
    'verify',
    verifyFails.map((o) => `${o.id}: ${o.message ?? 'failed'}`),
    `package verification passed (${verify.outcomes.length} outcomes)`
  );

  const evidenceIds = new Set(packet.evidence.map((e) => e.id));
  const evidenceById = new Map(packet.evidence.map((e) => [e.id, e]));

  // G1 — evidence resolution.
  const g1: string[] = [];
  for (const f of draft.facts) {
    if (f.evidenceRefs.length === 0) g1.push(`fact ${f.id} has no evidence refs`);
    for (const ref of f.evidenceRefs) {
      if (!evidenceIds.has(ref.evidenceId)) g1.push(`fact ${f.id} references unknown evidence ${ref.evidenceId}`);
    }
  }
  push(findings, 'G1', g1, `all ${draft.facts.length} facts carry resolvable evidence refs`);

  // G2 — quote anchors verify; anchored-fact rate >= 0.9.
  const g2: string[] = [];
  for (const f of draft.facts) {
    for (const ref of f.evidenceRefs) {
      const ev = evidenceById.get(ref.evidenceId);
      if (!ev) continue; // reported under G1
      for (const anchor of ref.anchors) {
        if (!quoteAppearsIn(anchor.quote, ev.text)) {
          g2.push(`fact ${f.id}: quote not found in ${ref.evidenceId}: "${anchor.quote.slice(0, 80)}"`);
        }
      }
    }
  }
  if (draft.facts.length > 0) {
    const anchored = draft.facts.filter(factHasQuoteAnchor).length;
    const rate = anchored / draft.facts.length;
    if (rate < 0.9) {
      g2.push(`anchor rate ${(rate * 100).toFixed(0)}% (${anchored}/${draft.facts.length} facts) is below 90%`);
    }
  }
  push(findings, 'G2', g2, 'all quote anchors verify against their text layers');

  // C1 — claim support links resolve; no orphan claims.
  const factIds = new Set(draft.facts.map((f) => f.id));
  const judgmentIds = new Set(draft.judgments.map((j) => j.id));
  const calcIds = new Set(draft.calculations.map((c) => c.id));
  const c1: string[] = [];
  for (const c of draft.claims) {
    const supportCount =
      c.supportingFacts.length + c.supportingEvidence.length + c.judgments.length + c.supportingCalculations.length;
    if (supportCount === 0) c1.push(`claim ${c.id} is an orphan: no supporting facts, evidence, judgments, or calculations`);
    for (const ref of c.supportingFacts) if (!factIds.has(ref)) c1.push(`claim ${c.id}: unknown fact ref ${ref}`);
    for (const ref of c.judgments) if (!judgmentIds.has(ref)) c1.push(`claim ${c.id}: unknown judgment ref ${ref}`);
    for (const ref of c.supportingCalculations) if (!calcIds.has(ref)) c1.push(`claim ${c.id}: unknown calculation ref ${ref}`);
    for (const ref of c.supportingEvidence) if (!evidenceIds.has(ref)) c1.push(`claim ${c.id}: unknown evidence ref ${ref}`);
  }
  push(findings, 'C1', c1, `all ${draft.claims.length} claims are supported and their links resolve`);

  // K1 — recompute the conventional calculation types.
  const k1: string[] = [];
  let recomputed = 0;
  for (const calc of draft.calculations) {
    const result = recomputeCalculation(calc);
    if (!result.applicable) continue;
    recomputed++;
    for (const p of result.problems) k1.push(`calculation ${calc.id}: ${p}`);
  }
  push(findings, 'K1', k1, `${recomputed} conventional calculation(s) recompute cleanly`);

  // S2 — duplicate factType with conflicting values.
  const byType = new Map<string, Fact[]>();
  for (const f of draft.facts) {
    byType.set(f.factType, [...(byType.get(f.factType) ?? []), f]);
  }
  const s2: string[] = [];
  for (const [factType, group] of byType) {
    if (group.length < 2) continue;
    const distinct = new Map<string, string[]>();
    for (const f of group) {
      const key = canonicalValue(f.value);
      distinct.set(key, [...(distinct.get(key) ?? []), f.id]);
    }
    if (distinct.size > 1) {
      s2.push(
        `factType ${factType} has conflicting values across facts: ${[...distinct.values()].map((ids) => ids.join('/')).join(' vs ')}`
      );
    }
  }
  push(findings, 'S2', s2, 'no conflicting duplicate fact types');

  // P1 — slot fill against the public required-facts manifest.
  const p1: string[] = [];
  for (const slot of packet.requiredFacts) {
    const candidates = draft.facts.filter((f) => f.factType === slot.factType);
    if (candidates.length === 0) {
      p1.push(`slot '${slot.slot}' unfilled: no fact with factType ${slot.factType}`);
      continue;
    }
    const conforming = candidates.find((f) => valueShapeProblem(slot, f.value) === undefined);
    if (!conforming) {
      const problem = valueShapeProblem(slot, candidates[0].value);
      p1.push(`slot '${slot.slot}' (${slot.factType}): value shape '${slot.valueShape}' not satisfied — ${problem}`);
    }
  }
  push(findings, 'P1', p1, `all ${packet.requiredFacts.length} required-fact slots are filled with conforming values`);

  const ok = verify.valid && findings.every((f) => f.status !== 'fail');
  return { ok, findings, verify };
}
