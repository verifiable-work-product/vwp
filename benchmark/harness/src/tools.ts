/**
 * The tool surface (harness DESIGN §3): VWP-semantic tools over a draft
 * document. Every tool carries a JSON schema (sent to the provider) and a
 * zod-validated handler. Violations and failed anchor checks return
 * structured, actionable errors — verbatim, uniformly, for every model.
 */
import { z } from 'zod';
import {
  type Anchor,
  type Claim,
  type Draft,
  type EvidenceRef,
  type ModelIdentity,
  emptyDraft,
} from './draft.js';
import type { Packet, PacketEvidence } from './packet.js';
import {
  CONVENTIONAL_CALC_TYPES,
  normalizeForQuoteMatch,
  quoteAppearsIn,
  recomputeCalculation,
  runValidation,
  type ValidationResult,
} from './validate.js';

export class ToolError extends Error {
  constructor(message: string, public details?: unknown) {
    super(message);
    this.name = 'ToolError';
  }
}

export interface ToolDef {
  name: string;
  description: string;
  /** JSON Schema for the provider API. */
  inputSchema: Record<string, unknown>;
  handler: (input: unknown) => unknown;
}

export type ToolOutcome =
  | { ok: true; result: unknown }
  | { ok: false; error: { message: string; details?: unknown } };

export interface ToolRegistry {
  tools: ToolDef[];
  draft: Draft;
  lastValidation?: ValidationResult;
  submitted: boolean;
  execute(name: string, input: unknown): ToolOutcome;
}

// --- shared (JSON) schema fragments ---------------------------------------

const linesJson = {
  type: 'object',
  properties: {
    from: { type: 'integer', minimum: 1 },
    to: { type: 'integer', minimum: 1 },
  },
  required: ['from'],
  additionalProperties: false,
};

const anchorJson = {
  type: 'object',
  properties: {
    evidenceId: { type: 'string', description: 'Evidence id the quote comes from.' },
    quote: {
      type: 'string',
      description:
        'VERBATIM excerpt from the evidence text (copy it exactly; whitespace and quote-mark style are normalized). Checked at call time — a non-matching quote is a tool error.',
    },
    page: { type: 'integer', minimum: 1, description: '1-based page within the evidence.' },
    lines: { ...linesJson, description: '1-based line range within the page.' },
  },
  required: ['evidenceId', 'quote'],
  additionalProperties: false,
};

const stringArrayJson = { type: 'array', items: { type: 'string' } };

// --- zod input schemas ------------------------------------------------------

const LinesZ = z.object({ from: z.number().int().min(1), to: z.number().int().min(1).optional() });
const AnchorInputZ = z.object({
  evidenceId: z.string().min(1),
  quote: z.string().min(1),
  page: z.number().int().min(1).optional(),
  lines: LinesZ.optional(),
});
type AnchorInput = z.infer<typeof AnchorInputZ>;

const FactFieldsZ = z.object({
  slot: z.string().optional(),
  factType: z.string().min(1),
  value: z.unknown(),
  statement: z.string().optional(),
  anchors: z.array(AnchorInputZ).min(1),
});

const JudgmentFieldsZ = z.object({
  topic: z.string().min(1),
  question: z.string().min(1),
  proposedConclusion: z.string().optional(),
  basis: z.string().optional(),
  alternatives: z.array(z.object({ conclusion: z.string().min(1), notes: z.string().optional() })).optional(),
  standardRefs: z.array(z.string()).optional(),
  riskLevel: z.enum(['low', 'medium', 'high']).optional(),
});

const AssessmentFieldsZ = z.object({
  stepKey: z.string().optional(),
  statement: z.string().min(1),
  standardRefs: z.array(z.string()).optional(),
  assertions: z.array(z.string()).optional(),
  supportingFacts: z.array(z.string()).optional(),
  supportingEvidence: z.array(z.string()).optional(),
  judgments: z.array(z.string()).optional(),
  supportingCalculations: z.array(z.string()).optional(),
});

const CalculationFieldsZ = z.object({
  type: z.string().min(1),
  inputs: z.record(z.unknown()),
  outputs: z.record(z.unknown()),
  method: z.string().optional(),
  recomputable: z.boolean().optional(),
});

export interface CreateToolRegistryOptions {
  modelIdentity: ModelIdentity;
  /** Called when the model calls submit(); the loop ends the run. */
  onSubmit?: () => void;
}

export function createToolRegistry(packet: Packet, opts: CreateToolRegistryOptions): ToolRegistry {
  const draft = emptyDraft();
  const counters = { fact: 0, claim: 0, judgment: 0, calculation: 0 };
  const evidenceById = new Map<string, PacketEvidence>(packet.evidence.map((e) => [e.id, e]));

  const requireEvidence = (evidenceId: string): PacketEvidence => {
    const ev = evidenceById.get(evidenceId);
    if (!ev) {
      throw new ToolError(`unknown evidence id: ${evidenceId}`, {
        knownEvidenceIds: [...evidenceById.keys()],
      });
    }
    return ev;
  };

  /** Verify every anchor quote against its evidence text layer; group into evidenceRefs. */
  const verifyAndGroupAnchors = (anchors: AnchorInput[]): EvidenceRef[] => {
    for (const anchor of anchors) {
      const ev = requireEvidence(anchor.evidenceId);
      if (!quoteAppearsIn(anchor.quote, ev.text)) {
        throw new ToolError(
          `anchor quote not found in evidence ${anchor.evidenceId}. The quote must appear VERBATIM in the evidence text (whitespace and quote-mark style are normalized). Failed quote: "${anchor.quote}"`,
          {
            evidenceId: anchor.evidenceId,
            failedQuote: anchor.quote,
            hint: 'Use read_evidence or search_evidence to copy the exact wording, then retry.',
          }
        );
      }
      if (anchor.page !== undefined && anchor.page > ev.pages.length) {
        throw new ToolError(`evidence ${anchor.evidenceId} has ${ev.pages.length} page(s); page ${anchor.page} does not exist`, {
          evidenceId: anchor.evidenceId,
          pages: ev.pages.length,
        });
      }
    }
    const byEvidence = new Map<string, Anchor[]>();
    for (const a of anchors) {
      const anchor: Anchor = { quote: a.quote, ...(a.page ? { page: a.page } : {}), ...(a.lines ? { lines: a.lines } : {}) };
      byEvidence.set(a.evidenceId, [...(byEvidence.get(a.evidenceId) ?? []), anchor]);
    }
    return [...byEvidence.entries()].map(([evidenceId, anchorList]) => ({ evidenceId, anchors: anchorList }));
  };

  const checkAssessmentRefs = (fields: z.infer<typeof AssessmentFieldsZ>): void => {
    const missing: string[] = [];
    for (const ref of fields.supportingFacts ?? []) {
      if (!draft.facts.some((f) => f.id === ref)) missing.push(`fact ${ref}`);
    }
    for (const ref of fields.judgments ?? []) {
      if (!draft.judgments.some((j) => j.id === ref)) missing.push(`judgment ${ref}`);
    }
    for (const ref of fields.supportingCalculations ?? []) {
      if (!draft.calculations.some((c) => c.id === ref)) missing.push(`calculation ${ref}`);
    }
    for (const ref of fields.supportingEvidence ?? []) {
      if (!evidenceById.has(ref)) missing.push(`evidence ${ref}`);
    }
    if (missing.length > 0) {
      throw new ToolError(`unknown reference(s): ${missing.join(', ')}. Create the referenced objects first.`, { missing });
    }
  };

  const checkCalculation = (fields: z.infer<typeof CalculationFieldsZ>, id: string): void => {
    const result = recomputeCalculation({ id, type: fields.type, inputs: fields.inputs, outputs: fields.outputs });
    if (result.applicable && result.problems.length > 0) {
      throw new ToolError(
        `calculation does not recompute: ${result.problems.join('; ')}`,
        { problems: result.problems, tolerance: 0.01 }
      );
    }
  };

  const findOrThrow = <T extends { id: string }>(list: T[], id: string, kind: string): T => {
    const item = list.find((x) => x.id === id);
    if (!item) throw new ToolError(`no such ${kind}: ${id}`, { knownIds: list.map((x) => x.id) });
    return item;
  };

  const removeOrThrow = <T extends { id: string }>(list: T[], id: string, kind: string): void => {
    const idx = list.findIndex((x) => x.id === id);
    if (idx === -1) throw new ToolError(`no such ${kind}: ${id}`, { knownIds: list.map((x) => x.id) });
    list.splice(idx, 1);
  };

  const numberLines = (page: string, pageNo: number, withHeader: boolean): string => {
    const lines = page.split('\n');
    const body = lines.map((l, i) => `${String(i + 1).padStart(4, ' ')}│ ${l}`).join('\n');
    return withHeader ? `===== page ${pageNo} =====\n${body}` : body;
  };

  const registry: ToolRegistry = {
    tools: [],
    draft,
    submitted: false,
    execute(name: string, input: unknown): ToolOutcome {
      const tool = this.tools.find((t) => t.name === name);
      if (!tool) {
        return { ok: false, error: { message: `unknown tool: ${name}`, details: { knownTools: this.tools.map((t) => t.name) } } };
      }
      try {
        return { ok: true, result: tool.handler(input ?? {}) };
      } catch (err) {
        if (err instanceof ToolError) {
          return { ok: false, error: { message: err.message, details: err.details } };
        }
        return { ok: false, error: { message: `tool ${name} failed: ${(err as Error).message}` } };
      }
    },
  };

  /** zod-parse a tool input; violations become structured tool errors. */
  const parse = <T>(schema: z.ZodType<T>, input: unknown): T => {
    const parsed = schema.safeParse(input);
    if (!parsed.success) {
      throw new ToolError(
        `invalid tool input: ${parsed.error.issues.map((i) => `${i.path.join('.') || '(root)'}: ${i.message}`).join('; ')}`,
        { issues: parsed.error.issues }
      );
    }
    return parsed.data;
  };

  const tools: ToolDef[] = [
    {
      name: 'read_brief',
      description:
        'Return the engagement brief, the deliverable spec, the directed method steps, and the required-facts manifest (the slots the work product must fill) — or, for catalog-disclosure challenges, the public fact catalog instead.',
      inputSchema: { type: 'object', properties: {}, additionalProperties: false },
      handler: () => ({
        brief: packet.brief,
        deliverableSpec: packet.deliverableSpec,
        methodSteps: packet.methodSteps,
        requiredFacts: packet.requiredFacts,
        ...(packet.factCatalog ? { factCatalog: packet.factCatalog } : {}),
      }),
    },
    {
      name: 'list_evidence',
      description: 'List the evidence index: id, title, type, and page count for every evidence document.',
      inputSchema: { type: 'object', properties: {}, additionalProperties: false },
      handler: () =>
        packet.evidence.map((e) => ({
          id: e.id,
          title: e.title,
          type: e.type,
          pages: e.pages.length,
        })),
    },
    {
      name: 'read_evidence',
      description:
        'Read an evidence document as line-numbered text — the exact text layer that quote anchors verify against. Omit page to read the whole document.',
      inputSchema: {
        type: 'object',
        properties: {
          evidenceId: { type: 'string' },
          page: { type: 'integer', minimum: 1, description: '1-based page; omit for the whole document.' },
        },
        required: ['evidenceId'],
        additionalProperties: false,
      },
      handler: (input) => {
        const { evidenceId, page } = parse(
          z.object({ evidenceId: z.string().min(1), page: z.number().int().min(1).optional() }),
          input
        );
        const ev = requireEvidence(evidenceId);
        if (page !== undefined) {
          if (page > ev.pages.length) {
            throw new ToolError(`evidence ${evidenceId} has ${ev.pages.length} page(s); page ${page} does not exist`);
          }
          return { evidenceId, page, pages: ev.pages.length, text: numberLines(ev.pages[page - 1], page, false) };
        }
        const text =
          ev.pages.length === 1
            ? numberLines(ev.pages[0], 1, false)
            : ev.pages.map((p, i) => numberLines(p, i + 1, true)).join('\n');
        return { evidenceId, pages: ev.pages.length, text };
      },
    },
    {
      name: 'search_evidence',
      description:
        'Search all evidence text for a phrase (normalized substring match: case, whitespace, quote-mark, and dash style are ignored). Returns evidenceId/page/line hits with the matching line as a snippet.',
      inputSchema: {
        type: 'object',
        properties: { query: { type: 'string', minLength: 2 } },
        required: ['query'],
        additionalProperties: false,
      },
      handler: (input) => {
        const { query } = parse(z.object({ query: z.string().min(2) }), input);
        // Search folds case on top of quote normalization: it is a finding
        // aid, not the verification surface (quote anchors stay case-exact).
        const nq = normalizeForQuoteMatch(query).toLowerCase();
        if (nq.length === 0) throw new ToolError('query is empty after normalization');
        const hits: Array<{ evidenceId: string; page: number; line: number; snippet: string }> = [];
        const MAX_HITS = 40;
        for (const ev of packet.evidence) {
          for (const [p, pageText] of ev.pages.entries()) {
            const lines = pageText.split('\n');
            for (const [i, line] of lines.entries()) {
              if (normalizeForQuoteMatch(line).toLowerCase().includes(nq)) {
                hits.push({ evidenceId: ev.id, page: p + 1, line: i + 1, snippet: line.trim() });
                if (hits.length >= MAX_HITS) {
                  return { hits, truncated: true, note: `more than ${MAX_HITS} hits; narrow the query` };
                }
              }
            }
          }
        }
        return { hits, truncated: false };
      },
    },
    {
      name: 'add_fact',
      description:
        'Add a fact: an anchored observation extracted from evidence. Each anchor quote is checked verbatim against the evidence text at call time. For required-facts slots, set slot and use the manifest\'s exact factType and value shape (money: {amount, currency}; quantity: {value, unit}; enum: one of the listed values; date: "YYYY-MM-DD"; period: {from, to}). Returns the new fact id (f_N).',
      inputSchema: {
        type: 'object',
        properties: {
          slot: { type: 'string', description: 'Required-facts manifest slot this fact fills, when applicable.' },
          factType: { type: 'string', description: "Namespaced fact type, e.g. 'asc606.transactionPrice'." },
          value: { description: 'Structured value conforming to the slot value shape when filling a slot.' },
          statement: { type: 'string', description: 'One-sentence statement of the observation.' },
          anchors: { type: 'array', items: anchorJson, minItems: 1 },
        },
        required: ['factType', 'value', 'anchors'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(FactFieldsZ, input);
        const evidenceRefs = verifyAndGroupAnchors(fields.anchors);
        const id = `f_${++counters.fact}`;
        draft.facts.push({
          id,
          slot: fields.slot,
          statement: fields.statement,
          factType: fields.factType,
          value: fields.value,
          evidenceRefs,
        });
        return { id, factCount: draft.facts.length };
      },
    },
    {
      name: 'update_fact',
      description: 'Update an existing fact. Provided fields replace the old ones; anchors are re-verified against the evidence.',
      inputSchema: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          slot: { type: 'string' },
          factType: { type: 'string' },
          value: {},
          statement: { type: 'string' },
          anchors: { type: 'array', items: anchorJson, minItems: 1 },
        },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(FactFieldsZ.partial().extend({ id: z.string().min(1) }), input);
        const fact = findOrThrow(draft.facts, fields.id, 'fact');
        if (fields.anchors) fact.evidenceRefs = verifyAndGroupAnchors(fields.anchors);
        if (fields.slot !== undefined) fact.slot = fields.slot;
        if (fields.factType !== undefined) fact.factType = fields.factType;
        if ('value' in (input as object)) fact.value = fields.value;
        if (fields.statement !== undefined) fact.statement = fields.statement;
        return { id: fact.id, updated: true };
      },
    },
    {
      name: 'remove_fact',
      description: 'Remove a fact by id. Claims referencing it will fail validation until updated.',
      inputSchema: {
        type: 'object',
        properties: { id: { type: 'string' } },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const { id } = parse(z.object({ id: z.string().min(1) }), input);
        removeOrThrow(draft.facts, id, 'fact');
        return { id, removed: true };
      },
    },
    {
      name: 'add_judgment',
      description:
        'Record a material professional judgment point: the question, your proposed conclusion, its basis with authority citations, and — itemized, not flattened into prose — the alternative conclusions considered with why each was set aside. Returns the new judgment id (j_N).',
      inputSchema: {
        type: 'object',
        properties: {
          topic: { type: 'string' },
          question: { type: 'string' },
          proposedConclusion: { type: 'string' },
          basis: { type: 'string', description: 'Reasoning with authority citations (e.g. ASC references).' },
          alternatives: {
            type: 'array',
            items: {
              type: 'object',
              properties: { conclusion: { type: 'string' }, notes: { type: 'string' } },
              required: ['conclusion'],
              additionalProperties: false,
            },
            description: 'Each alternative position considered, with notes on why it was set aside.',
          },
          standardRefs: stringArrayJson,
          riskLevel: { type: 'string', enum: ['low', 'medium', 'high'] },
        },
        required: ['topic', 'question'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(JudgmentFieldsZ, input);
        const id = `j_${++counters.judgment}`;
        draft.judgments.push({
          id,
          topic: fields.topic,
          question: fields.question,
          proposedConclusion: fields.proposedConclusion,
          basis: fields.basis,
          alternatives: fields.alternatives ?? [],
          standardRefs: fields.standardRefs ?? [],
          riskLevel: fields.riskLevel,
        });
        return { id, judgmentCount: draft.judgments.length };
      },
    },
    {
      name: 'update_judgment',
      description: 'Update an existing judgment. Provided fields replace the old ones.',
      inputSchema: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          topic: { type: 'string' },
          question: { type: 'string' },
          proposedConclusion: { type: 'string' },
          basis: { type: 'string' },
          alternatives: {
            type: 'array',
            items: {
              type: 'object',
              properties: { conclusion: { type: 'string' }, notes: { type: 'string' } },
              required: ['conclusion'],
              additionalProperties: false,
            },
          },
          standardRefs: stringArrayJson,
          riskLevel: { type: 'string', enum: ['low', 'medium', 'high'] },
        },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(JudgmentFieldsZ.partial().extend({ id: z.string().min(1) }), input);
        const judgment = findOrThrow(draft.judgments, fields.id, 'judgment');
        if (fields.topic !== undefined) judgment.topic = fields.topic;
        if (fields.question !== undefined) judgment.question = fields.question;
        if (fields.proposedConclusion !== undefined) judgment.proposedConclusion = fields.proposedConclusion;
        if (fields.basis !== undefined) judgment.basis = fields.basis;
        if (fields.alternatives !== undefined) judgment.alternatives = fields.alternatives;
        if (fields.standardRefs !== undefined) judgment.standardRefs = fields.standardRefs;
        if (fields.riskLevel !== undefined) judgment.riskLevel = fields.riskLevel;
        return { id: judgment.id, updated: true };
      },
    },
    {
      name: 'remove_judgment',
      description: 'Remove a judgment by id.',
      inputSchema: {
        type: 'object',
        properties: { id: { type: 'string' } },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const { id } = parse(z.object({ id: z.string().min(1) }), input);
        removeOrThrow(draft.judgments, id, 'judgment');
        return { id, removed: true };
      },
    },
    {
      name: 'add_assessment',
      description:
        'Add an assessment (a claim): an assertion or conclusion, linked to the facts, judgments, calculations, and evidence that support it. Set stepKey to the method step it answers. Every assessment needs at least one support link; references are checked at call time. Returns the new id (c_N).',
      inputSchema: {
        type: 'object',
        properties: {
          stepKey: { type: 'string', description: 'Method step key this assessment answers, when applicable.' },
          statement: { type: 'string' },
          standardRefs: { ...stringArrayJson, description: "Authority references, e.g. 'ASC 606-10-25-1'." },
          assertions: { ...stringArrayJson, description: 'Finance-profile assertion terms (existence, occurrence, completeness, valuationAllocation, rightsAndObligations, presentationDisclosure).' },
          supportingFacts: stringArrayJson,
          supportingEvidence: stringArrayJson,
          judgments: stringArrayJson,
          supportingCalculations: stringArrayJson,
        },
        required: ['statement'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(AssessmentFieldsZ, input);
        checkAssessmentRefs(fields);
        const id = `c_${++counters.claim}`;
        draft.claims.push({
          id,
          stepKey: fields.stepKey,
          statement: fields.statement,
          standardRefs: fields.standardRefs ?? [],
          assertions: fields.assertions ?? [],
          supportingFacts: fields.supportingFacts ?? [],
          supportingEvidence: fields.supportingEvidence ?? [],
          judgments: fields.judgments ?? [],
          supportingCalculations: fields.supportingCalculations ?? [],
        });
        return { id, claimCount: draft.claims.length };
      },
    },
    {
      name: 'update_assessment',
      description: 'Update an existing assessment. Provided fields replace the old ones; references are re-checked.',
      inputSchema: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          stepKey: { type: 'string' },
          statement: { type: 'string' },
          standardRefs: stringArrayJson,
          assertions: stringArrayJson,
          supportingFacts: stringArrayJson,
          supportingEvidence: stringArrayJson,
          judgments: stringArrayJson,
          supportingCalculations: stringArrayJson,
        },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(AssessmentFieldsZ.partial().extend({ id: z.string().min(1) }), input);
        const claim = findOrThrow(draft.claims, fields.id, 'assessment');
        checkAssessmentRefs({ statement: claim.statement, ...fields });
        const apply = <K extends keyof Claim>(key: K, value: Claim[K] | undefined) => {
          if (value !== undefined) claim[key] = value;
        };
        apply('stepKey', fields.stepKey);
        apply('statement', fields.statement);
        apply('standardRefs', fields.standardRefs);
        apply('assertions', fields.assertions);
        apply('supportingFacts', fields.supportingFacts);
        apply('supportingEvidence', fields.supportingEvidence);
        apply('judgments', fields.judgments);
        apply('supportingCalculations', fields.supportingCalculations);
        return { id: claim.id, updated: true };
      },
    },
    {
      name: 'remove_assessment',
      description: 'Remove an assessment (claim) by id.',
      inputSchema: {
        type: 'object',
        properties: { id: { type: 'string' } },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const { id } = parse(z.object({ id: z.string().min(1) }), input);
        removeOrThrow(draft.claims, id, 'assessment');
        return { id, removed: true };
      },
    },
    {
      name: 'add_calculation',
      description:
        `Add a calculation with declared inputs and outputs. Conventional types are RECOMPUTED at call time (tolerance 0.01) and mismatches are tool errors: ` +
        `sum {inputs.values: number[] -> outputs.total}; ` +
        `schedule {inputs.rows: [{label?, values: number[]}] -> outputs.grandTotal (+ optional rowTotals[], columnTotals[])}; ` +
        `journalEntries {inputs.entries: [{date?, description?, lines: [{account, debit?, credit?}]}] -> outputs.totalDebits, outputs.totalCredits}; ` +
        `relativeStandaloneSellingPriceAllocation {inputs.transactionPrice, inputs.components: [{id, standaloneSellingPrice}] -> outputs.allocations: [{componentId, allocatedAmount}] (one per component, matched by componentId; must tie out to the transaction price)}. ` +
        `Use type 'custom' for anything else — but prefer a conventional type whenever the math fits (custom calculations cannot be machine-recomputed, which weakens the package). Returns the new id (calc_N).`,
      inputSchema: {
        type: 'object',
        properties: {
          type: {
            type: 'string',
            description: "One of 'sum', 'schedule', 'journalEntries', 'relativeStandaloneSellingPriceAllocation', or 'custom'.",
          },
          inputs: { type: 'object', description: 'Fully specified input values (numbers or decimal strings).' },
          outputs: { type: 'object', description: 'Computed results; must reproduce exactly from inputs for conventional types.' },
          method: { type: 'string' },
          recomputable: { type: 'boolean' },
        },
        required: ['type', 'inputs', 'outputs'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(CalculationFieldsZ, input);
        const id = `calc_${++counters.calculation}`;
        checkCalculation(fields, id);
        const conventional = (CONVENTIONAL_CALC_TYPES as readonly string[]).includes(fields.type);
        draft.calculations.push({
          id,
          type: fields.type,
          inputs: fields.inputs,
          outputs: fields.outputs,
          method: fields.method,
          recomputable: fields.recomputable ?? conventional,
        });
        return { id, calculationCount: draft.calculations.length };
      },
    },
    {
      name: 'update_calculation',
      description: 'Update an existing calculation. Provided fields replace the old ones; conventional types are recomputed.',
      inputSchema: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          type: { type: 'string' },
          inputs: { type: 'object' },
          outputs: { type: 'object' },
          method: { type: 'string' },
          recomputable: { type: 'boolean' },
        },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const fields = parse(CalculationFieldsZ.partial().extend({ id: z.string().min(1) }), input);
        const calc = findOrThrow(draft.calculations, fields.id, 'calculation');
        const next = {
          type: fields.type ?? calc.type,
          inputs: fields.inputs ?? calc.inputs,
          outputs: fields.outputs ?? calc.outputs,
          method: fields.method ?? calc.method,
          recomputable: fields.recomputable ?? calc.recomputable,
        };
        checkCalculation(next, calc.id);
        Object.assign(calc, next);
        return { id: calc.id, updated: true };
      },
    },
    {
      name: 'remove_calculation',
      description: 'Remove a calculation by id.',
      inputSchema: {
        type: 'object',
        properties: { id: { type: 'string' } },
        required: ['id'],
        additionalProperties: false,
      },
      handler: (input) => {
        const { id } = parse(z.object({ id: z.string().min(1) }), input);
        removeOrThrow(draft.calculations, id, 'calculation');
        return { id, removed: true };
      },
    },
    {
      name: 'write_memo',
      description:
        'Write (or replace) the professional memo in Markdown — the rendered deliverable per the deliverable spec. Include the schedules and journal entries as tables.',
      inputSchema: {
        type: 'object',
        properties: { markdown: { type: 'string', minLength: 1 } },
        required: ['markdown'],
        additionalProperties: false,
      },
      handler: (input) => {
        const { markdown } = parse(z.object({ markdown: z.string().min(1) }), input);
        draft.memoMd = markdown;
        return { bytes: Buffer.byteLength(markdown, 'utf8'), written: true };
      },
    },
    {
      name: 'validate',
      description:
        'Run the full offline check battery on the current draft: package verification (schemas, link resolution, verbatim-quote anchors) plus G1 evidence resolution, G2 anchor coverage, C1 claim support, K1 calculation recomputation, S2 internal consistency, and P1 required-facts slot fill. Findings are structured and fixable. A clean validate() is REQUIRED before submit().',
      inputSchema: { type: 'object', properties: {}, additionalProperties: false },
      handler: () => {
        const result = runValidation(draft, packet, { modelIdentity: opts.modelIdentity });
        registry.lastValidation = result;
        return {
          ok: result.ok,
          findings: result.findings,
          verify: {
            valid: result.verify.valid,
            failures: result.verify.outcomes
              .filter((o) => o.status === 'fail')
              .map((o) => ({ id: o.id, message: o.message })),
          },
        };
      },
    },
    {
      name: 'submit',
      description:
        'Submit the work product and end the run. Only call this after validate() returns ok: true — the output is submitted exactly as produced, with no touch-ups.',
      inputSchema: { type: 'object', properties: {}, additionalProperties: false },
      handler: () => {
        registry.submitted = true;
        opts.onSubmit?.();
        return { submitted: true };
      },
    },
  ];

  registry.tools = tools;
  return registry;
}
