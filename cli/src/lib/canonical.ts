import canonicalizeJson from 'canonicalize';

/**
 * Object kinds of the integrity graph (spec/integrity.md section 3). The
 * memo format is hashed with its own kind so the narrative's formatHash is
 * domain-separated like every other self hash.
 */
export type ObjectKind =
  | 'evidence'
  | 'fact'
  | 'calculation'
  | 'judgment'
  | 'claim'
  | 'narrative'
  | 'approval'
  | 'attestation'
  | 'workproduct'
  | 'provenance'
  | 'memo-format';

/**
 * RFC 8785 (JSON Canonicalization Scheme) serialization of a JSON value,
 * returned as a string. Encode as UTF-8 before hashing.
 */
export function canonicalJson(value: unknown): string {
  const out = canonicalizeJson(value);
  if (out === undefined) throw new Error('failed to canonicalize: value is not JSON serializable');
  return out;
}

/**
 * Review-layer fields removed before an object is hashed (spec/integrity.md
 * section 2): a judgment's status and a work product's conclusionSummary.
 * Every other kind is hashed as is. Returns a shallow copy; the input is
 * never mutated.
 */
export function stripReviewFields<T>(kind: ObjectKind, obj: T): T {
  if (obj === null || typeof obj !== 'object' || Array.isArray(obj)) return obj;
  const copy: Record<string, unknown> = { ...(obj as Record<string, unknown>) };
  if (kind === 'judgment') delete copy.status;
  if (kind === 'workproduct') delete copy.conclusionSummary;
  return copy as T;
}
