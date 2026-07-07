/**
 * VWP package verifier (stub).
 *
 * Status: planned. See cli/ for the working reference implementation.
 */

import type { Manifest, ValidationResult } from './types.js';

export interface VerifyOptions {
  /** Profile to validate against; defaults to the profile declared in the manifest. */
  profile?: string;
  /** Additional validator identifiers to run. */
  validators?: string[];
}

export interface VerifyReport {
  packageId: string;
  manifest: Manifest;
  results: ValidationResult[];
  /** True when no result has status 'fail'. */
  ok: boolean;
}

/** Verify a .vwp package from its ZIP bytes (stub). */
export function verifyPackage(
  _bytes: Uint8Array,
  _options?: VerifyOptions,
): Promise<VerifyReport> {
  throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
}

/** Verify a .vwp package from a file path (stub). */
export function verifyPackageFile(
  _path: string,
  _options?: VerifyOptions,
): Promise<VerifyReport> {
  throw new Error('Not implemented: see cli/ for the reference verifier (v0.1)');
}
