/**
 * The harness's draft document — the same shape as the human workbench draft.
 * Tools mutate a Draft; assembly (assemble.ts) turns it into a .vwp package.
 *
 * Ids are harness-assigned and prefixed by kind: f_ (facts), c_ (claims),
 * j_ (judgments), calc_ (calculations).
 */
import { z } from 'zod';

export const AnchorSchema = z.object({
  /** Verbatim excerpt from the cited evidence's text layer. */
  quote: z.string().min(1),
  page: z.number().int().min(1).optional(),
  lines: z
    .object({
      from: z.number().int().min(1),
      to: z.number().int().min(1).optional(),
    })
    .optional(),
});
export type Anchor = z.infer<typeof AnchorSchema>;

export const EvidenceRefSchema = z.object({
  evidenceId: z.string().min(1),
  anchors: z.array(AnchorSchema).min(1),
});
export type EvidenceRef = z.infer<typeof EvidenceRefSchema>;

export const FactSchema = z.object({
  id: z.string().min(1),
  /** Required-facts manifest slot this fact fills, when applicable. */
  slot: z.string().optional(),
  statement: z.string().optional(),
  factType: z.string().min(1),
  value: z.unknown(),
  evidenceRefs: z.array(EvidenceRefSchema),
});
export type Fact = z.infer<typeof FactSchema>;

export const ClaimSchema = z.object({
  id: z.string().min(1),
  /** Method step this claim answers, when applicable. */
  stepKey: z.string().optional(),
  statement: z.string().min(1),
  standardRefs: z.array(z.string()),
  assertions: z.array(z.string()),
  supportingFacts: z.array(z.string()),
  supportingEvidence: z.array(z.string()),
  judgments: z.array(z.string()),
  supportingCalculations: z.array(z.string()),
});
export type Claim = z.infer<typeof ClaimSchema>;

export const JudgmentAlternativeSchema = z.object({
  conclusion: z.string().min(1),
  notes: z.string().optional(),
});
export type JudgmentAlternative = z.infer<typeof JudgmentAlternativeSchema>;

export const JudgmentSchema = z.object({
  id: z.string().min(1),
  topic: z.string().min(1),
  question: z.string().min(1),
  proposedConclusion: z.string().optional(),
  basis: z.string().optional(),
  alternatives: z.array(JudgmentAlternativeSchema),
  standardRefs: z.array(z.string()),
  riskLevel: z.enum(['low', 'medium', 'high']).optional(),
});
export type Judgment = z.infer<typeof JudgmentSchema>;

export const CalculationSchema = z.object({
  id: z.string().min(1),
  type: z.string().min(1),
  inputs: z.record(z.unknown()),
  outputs: z.record(z.unknown()),
  method: z.string().optional(),
  recomputable: z.boolean(),
});
export type Calculation = z.infer<typeof CalculationSchema>;

export const DraftSchema = z.object({
  facts: z.array(FactSchema),
  claims: z.array(ClaimSchema),
  judgments: z.array(JudgmentSchema),
  calculations: z.array(CalculationSchema),
  memoMd: z.string(),
});
export type Draft = z.infer<typeof DraftSchema>;

export function emptyDraft(): Draft {
  return { facts: [], claims: [], judgments: [], calculations: [], memoMd: '' };
}

/** Exact model identity per spec/provenance.md §3 — required in every submission. */
export interface ModelIdentity {
  provider: string;
  name: string;
  version: string;
}
