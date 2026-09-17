// node --test benchmark/tools/
// The proposal's worked examples against lib/rubric.mjs, plus the published
// rubric artifact itself (lint + totals).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { computeGradeScore, diffGradeSheets, mergeGradeSheets, lintRubric } from './lib/rubric.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const rubric = JSON.parse(fs.readFileSync(path.join(here, '..', 'rubric', 'vwp-bench-rubric.v1.json'), 'utf8'));

/** 6/8 facts, every analysis/traceability/judgment item yes, 2/2 calcs,
 * memo 4, signability 5 → 15 + 25 + 10 + 10 + 10 + 4 + 20 = 94. */
function completeSheet(mutate) {
  const s = {
    rubricName: rubric.name,
    rubricVersion: rubric.version,
    areas: {
      facts: { kind: 'proportional', captured: 6, total: 8 },
      analysis: { kind: 'yesNoItems', answers: { contract: true, obligations: true, transactionPrice: true, allocation: true, recognition: true } },
      calculations: { kind: 'proportional', captured: 2, total: 2 },
      traceability: { kind: 'yesNoItems', answers: { linksResolve: true, sourcesSupport: true, calcInputsLinked: true, conclusionsLinked: true } },
      judgments: { kind: 'yesNoItems', answers: { identified: true, investigated: true, contraryEvidence: true, documented: true, resolved: true } },
      memo: { kind: 'rating', rating: 4 },
      signability: { kind: 'rating', rating: 5 },
    },
    capFlags: {},
  };
  mutate?.(s);
  return s;
}

test('the published rubric lints clean and totals 100', () => {
  assert.deepEqual(lintRubric(rubric), []);
  assert.equal(rubric.areas.reduce((s, a) => s + a.points, 0), 100);
  assert.deepEqual(rubric.areas.map((a) => a.key), ['facts', 'analysis', 'calculations', 'traceability', 'judgments', 'memo', 'signability']);
});

test('proportional credit: 6 of 8 facts earns 15 of 20 (proposal example)', () => {
  const score = computeGradeScore(rubric, completeSheet());
  assert.equal(score.areaPoints.facts, 15);
  assert.equal(score.total, 94);
  assert.equal(score.effectiveSignability, 5);
  assert.equal(score.capApplied, false);
  assert.equal(score.complete, true);
});

test('yes/no items are full or zero credit', () => {
  const score = computeGradeScore(rubric, completeSheet((s) => { s.areas.analysis.answers.contract = false; }));
  assert.equal(score.areaPoints.analysis, 20);
  assert.equal(score.total, 89);
});

test('a cap condition holds signability at 3 regardless of total (20 → 12 points)', () => {
  const score = computeGradeScore(rubric, completeSheet((s) => { s.capFlags.materiallyWrongConclusion = true; }));
  assert.equal(score.signabilityRating, 5);
  assert.equal(score.effectiveSignability, 3);
  assert.equal(score.areaPoints.signability, 12);
  assert.equal(score.total, 86);
  assert.equal(score.capApplied, true);
});

test('an unanswered item leaves the sheet incomplete', () => {
  const score = computeGradeScore(rubric, completeSheet((s) => { s.areas.memo.rating = null; }));
  assert.equal(score.complete, false);
});

test('two graders disagree on an item; the third decides only that item', () => {
  const a = completeSheet();
  const b = completeSheet((s) => { s.areas.analysis.answers.contract = false; s.areas.memo.rating = 3; });
  assert.deepEqual(diffGradeSheets(rubric, a, b), [
    { areaKey: 'analysis', itemKey: 'contract', field: 'answer' },
    { areaKey: 'memo', itemKey: null, field: 'rating' },
  ]);
  const partial = mergeGradeSheets(rubric, a, b, { rubricName: rubric.name, rubricVersion: rubric.version, areas: { memo: { kind: 'rating', rating: 3 } } });
  assert.equal(partial.unresolved.length, 1);
  const full = mergeGradeSheets(rubric, a, b, {
    rubricName: rubric.name, rubricVersion: rubric.version,
    areas: { analysis: { kind: 'yesNoItems', answers: { contract: true } }, memo: { kind: 'rating', rating: 3 } },
  });
  assert.deepEqual(full.unresolved, []);
  assert.equal(computeGradeScore(rubric, full.sheet).total, 93); // sheet A with memo 3
});

test('lint catches points that disagree with items or multiplier', () => {
  const bad = JSON.parse(JSON.stringify(rubric));
  bad.areas.find((a) => a.key === 'analysis').points = 30;
  bad.areas.find((a) => a.key === 'signability').ratingMultiplier = 3;
  const problems = lintRubric(bad);
  assert.equal(problems.length, 2);
});
