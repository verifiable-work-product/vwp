#!/usr/bin/env node
// Deterministically score a work product's keyed items against a challenge
// answer key. Works identically for agent packages and transcribed CPA
// worksheets — both are facts/facts.json by scoring time.
//
// Usage:
//   node score-key.mjs <package-dir | facts.json> <answer-key.json> <required-facts.json>
//
// Output: a JSON report on stdout. Exit 0 always (scoring, not verification);
// exit 2 on usage/input errors.

import * as fs from 'fs';
import * as path from 'path';

function fail(msg) {
  process.stderr.write(`error: ${msg}\n`);
  process.exit(2);
}

const args = process.argv.slice(2);
if (args.length !== 3) {
  fail('usage: score-key.mjs <package-dir | facts.json> <answer-key.json> <required-facts.json>');
}
const [factsArg, keyPath, requiredPath] = args;

const factsFile = fs.statSync(factsArg).isDirectory() ? path.join(factsArg, 'facts', 'facts.json') : factsArg;
const facts = JSON.parse(fs.readFileSync(factsFile, 'utf8')).facts ?? [];
const key = JSON.parse(fs.readFileSync(keyPath, 'utf8'));
const required = JSON.parse(fs.readFileSync(requiredPath, 'utf8'));

if (key.challengeId !== required.challengeId) {
  fail(`challengeId mismatch: key is '${key.challengeId}', required-facts is '${required.challengeId}'`);
}

const slotsByName = new Map(required.slots.map((s) => [s.slot, s]));

/** Extract the comparable scalar from a value in a declared shape. */
function comparable(shape, value) {
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

function matches(shape, actual, expected, tolerance) {
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
process.stdout.write(
  JSON.stringify(
    {
      challengeId: key.challengeId,
      factsFile,
      keyedItems: results.length,
      tally,
      note:
        'acceptableAlternative results earn credit only if the work product documents the position (requiredDocumentation) — a grader decision, not a scorer decision.',
      results,
    },
    null,
    2
  ) + '\n'
);
