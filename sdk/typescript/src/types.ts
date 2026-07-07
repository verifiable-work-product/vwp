/**
 * VWP (Verifiable Work Product) core object types.
 *
 * Status: stub / preview. These interfaces mirror the v0.1 core objects. The
 * normative definitions are the JSON Schemas in schemas/; if these types and the
 * schemas disagree, the schemas win.
 */

/** A part (file) declared in the package manifest. */
export interface PackagePart {
  path: string;
  mediaType: string;
  sha256?: string;
  role?: string;
}

/** Provenance of a package or work product. */
export interface Provenance {
  createdAt: string;
  createdBy: string;
  toolchain?: Record<string, unknown>;
  models?: Record<string, unknown>;
  policyVersions?: Record<string, unknown>;
  sourceSystems?: string[];
  environment?: Record<string, unknown>;
  notes?: string;
}

/** The package manifest (manifest.json). */
export interface Manifest {
  vwpVersion: string;
  packageId: string;
  createdAt: string;
  profile: string;
  extension?: string;
  subject?: Record<string, unknown>;
  workproduct: string;
  parts?: PackagePart[];
  provenance: Provenance;
  signatures?: unknown[];
}

/** The work product being packaged (workproduct.json). */
export interface WorkProduct {
  id: string;
  title: string;
  domain: string;
  profile: string;
  extension?: string;
  standardRefs?: string[];
  entity?: string;
  period?: string;
  subject?: Record<string, unknown>;
  conclusionSummary?: string;
  status: string;
  createdBy?: string;
  reviewState?: string;
}

/** An evidence item under evidence/. */
export interface Evidence {
  id: string;
  type: string;
  title: string;
  sourceSystem?: string;
  path?: string;
  externalRef?: string;
  sha256?: string;
  retrievedAt?: string;
  periodCovered?: string;
  accessNotes?: string;
}

/** A fact extracted from evidence, under facts/. */
export interface Fact {
  id: string;
  statement: string;
  value?: unknown;
  evidenceRefs?: string[];
  extractionMethod?: string;
  confidence?: number;
  requiresReview?: boolean;
}

/** A claim (evidence-backed conclusion), under claims/. */
export interface Claim {
  id: string;
  statement: string;
  domain?: string;
  standardRefs?: string[];
  assertions?: string[];
  supportingFacts?: string[];
  supportingEvidence?: string[];
  supportingCalculations?: string[];
  judgments?: string[];
  conclusion?: string;
  status?: string;
  requiresReview?: boolean;
}

/** A professional judgment, under judgments/. */
export interface Judgment {
  id: string;
  topic: string;
  question: string;
  proposedConclusion?: string;
  basis?: string;
  alternatives?: unknown[];
  riskLevel?: string;
  requiredApproverRole?: string;
  status?: string;
}

/** A recomputable calculation, under calculations/. */
export interface Calculation {
  id: string;
  type: string;
  inputs: Record<string, unknown>;
  outputs: Record<string, unknown>;
  method?: string;
  validator?: string;
  recomputable?: boolean;
}

/** Status of a validation result. */
export type ValidationStatus =
  | 'pass'
  | 'fail'
  | 'warning'
  | 'reviewRequired'
  | 'notRun';

/** The result of running a validator, under validations/. */
export interface ValidationResult {
  id: string;
  validator: string;
  scope?: string;
  status: ValidationStatus;
  severity?: string;
  message?: string;
  details?: Record<string, unknown>;
}

/** Action taken by an approver. */
export type ApprovalAction =
  | 'approved'
  | 'rejected'
  | 'requestedChanges'
  | 'acknowledged';

/** A review/approval event, under approvals/. */
export interface Approval {
  id: string;
  approver: string;
  role?: string;
  action: ApprovalAction;
  timestamp: string;
  scope?: string;
  comments?: string;
}
