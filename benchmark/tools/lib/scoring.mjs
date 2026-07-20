// Keyed-item scoring logic. The score-key.mjs CLI is a thin wrapper over this.

/** Extract the comparable scalar from a value in a declared shape. */
export function comparable(shape, value) {
  if (value === undefined || value === null) return undefined;
  switch (shape) {
    case 'money':
      return typeof value === 'object' ? Number(value.amount) : Number(value);
    case 'quantity':
      return typeof value === 'object' ? Number(value.value) : Number(value);
    default:
      return value;
  }
}

export function matches(shape, actual, expected, tolerance) {
  const a = comparable(shape, actual);
  const e = comparable(shape, expected);
  if (a === undefined || e === undefined) return false;
  if (shape === 'money' || shape === 'quantity') {
    if (!Number.isFinite(a) || !Number.isFinite(e)) return false;
    if (Math.abs(a - e) > (tolerance ?? 0.01)) return false;
    // Currencies/units must agree when both sides declare them.
    const au = typeof actual === 'object' ? actual.currency ?? actual.unit : undefined;
    const eu = typeof expected === 'object' ? expected.currency ?? expected.unit : undefined;
    return au === undefined || eu === undefined || au === eu;
  }
  return a === e;
}

/**
 * Score a work product's keyed items against a challenge answer key.
 * Throws on challengeId mismatch between key and required-facts manifest.
 */
export function scoreKeyedItems(facts, key, required) {
  if (key.challengeId !== required.challengeId) {
    throw new Error(`challengeId mismatch: key is '${key.challengeId}', required-facts is '${required.challengeId}'`);
  }
  const slotsByName = new Map(required.slots.map((s) => [s.slot, s]));

  const results = [];
  for (const item of key.keyedItems) {
    const slot = slotsByName.get(item.slot);
    if (!slot) {
      results.push({ slot: item.slot, result: 'keyError', detail: 'slot not in required-facts manifest' });
      continue;
    }
    const candidates = facts.filter((f) => f.factType === slot.factType);
    if (candidates.length === 0) {
      results.push({ slot: item.slot, result: 'missing', detail: `no fact with factType ${slot.factType}` });
      continue;
    }
    if (candidates.length > 1) {
      results.push({
        slot: item.slot,
        result: 'ambiguous',
        detail: `${candidates.length} facts share factType ${slot.factType}: ${candidates.map((f) => f.id).join(', ')}`,
      });
      continue;
    }
    const fact = candidates[0];
    if (matches(slot.valueShape, fact.value, item.expected, item.tolerance)) {
      results.push({ slot: item.slot, result: 'correct', factId: fact.id, value: fact.value });
      continue;
    }
    const alternative = (item.alternatives ?? []).find((alt) =>
      matches(slot.valueShape, fact.value, alt.expected, item.tolerance)
    );
    if (alternative) {
      results.push({
        slot: item.slot,
        result: 'acceptableAlternative',
        factId: fact.id,
        value: fact.value,
        basis: alternative.basis,
        requiredDocumentation: alternative.requiredDocumentation ?? null,
      });
    } else {
      results.push({
        slot: item.slot,
        result: 'incorrect',
        factId: fact.id,
        value: fact.value,
        expected: item.expected,
      });
    }
  }

  const tally = results.reduce((acc, r) => ((acc[r.result] = (acc[r.result] ?? 0) + 1), acc), {});
  return {
    challengeId: key.challengeId,
    keyedItems: results.length,
    tally,
    note:
      'acceptableAlternative results earn credit only if the work product documents the position (requiredDocumentation) — a grader decision, not a scorer decision.',
    results,
  };
}
