/**
 * VWP package builder (stub).
 *
 * Status: planned. See cli/ for the working reference implementation.
 */

import type {
  Approval,
  Calculation,
  Claim,
  Evidence,
  Fact,
  Judgment,
  Manifest,
  Provenance,
  ValidationResult,
  WorkProduct,
} from './types.js';

export interface BuilderOptions {
  profile: string;
  extension?: string;
  provenance: Provenance;
}

/** Fluent builder for assembling a .vwp package (stub). */
export class PackageBuilder {
  constructor(_options: BuilderOptions) {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  setWorkProduct(_workProduct: WorkProduct): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  addEvidence(_evidence: Evidence, _content?: Uint8Array): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  addFact(_fact: Fact): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  addClaim(_claim: Claim): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  addJudgment(_judgment: Judgment): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  addCalculation(_calculation: Calculation): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  addValidationResult(_result: ValidationResult): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  addApproval(_approval: Approval): this {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }

  /** Produce the manifest and ZIP bytes for the assembled package. */
  build(): Promise<{ manifest: Manifest; bytes: Uint8Array }> {
    throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
  }
}
