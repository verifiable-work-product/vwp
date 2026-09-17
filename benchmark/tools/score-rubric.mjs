#!/usr/bin/env node
// Recompute a VWP-Bench score from published grade sheets — so anyone holding
// the rubric and the sheets can check every number in a results table.
//
// Usage:
//   node score-rubric.mjs <rubric.json> <sheet.json>
//       one grader's sheet → its score
//   node score-rubric.mjs <rubric.json> <sheetA.json> <sheetB.json> [--tiebreak <sheetT.json>]
//       two blind primaries (+ optional third-grader sheet) → item-level
//       disagreements, the consensus sheet, and its score when fully resolved
//   node score-rubric.mjs --lint <rubric.json>
//       structural checks on the rubric itself (points vs items/multiplier)
//
// Output: JSON on stdout. Exit 0 on a computed result, 1 when a consensus is
// requested but disagreements remain unresolved, 2 on usage/input errors.
// All logic lives in lib/rubric.mjs.

import * as fs from 'fs';
import { computeGradeScore, diffGradeSheets, mergeGradeSheets, lintRubric } from './lib/rubric.mjs';

function fail(msg) {
  process.stderr.write(`error: ${msg}\n`);
  process.exit(2);
}
const readJson = (p) => JSON.parse(fs.readFileSync(p, 'utf8'));

const args = process.argv.slice(2);
if (args.length === 0) fail('usage: score-rubric.mjs <rubric.json> <sheet.json> [<sheet2.json>] [--tiebreak <sheet.json>] | --lint <rubric.json>');

try {
  if (args[0] === '--lint') {
    if (args.length !== 2) fail('usage: score-rubric.mjs --lint <rubric.json>');
    const rubric = readJson(args[1]);
    const problems = lintRubric(rubric);
    process.stdout.write(JSON.stringify({ rubric: `${rubric.name} v${rubric.version}`, ok: problems.length === 0, problems }, null, 2) + '\n');
    process.exit(problems.length === 0 ? 0 : 1);
  }

  let tiebreakPath = null;
  const positional = [];
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--tiebreak') tiebreakPath = args[++i] ?? fail('--tiebreak needs a path');
    else positional.push(args[i]);
  }
  if (positional.length < 2 || positional.length > 3) fail('expected <rubric.json> and one or two sheets');
  const [rubricPath, ...sheetPaths] = positional;
  const rubric = readJson(rubricPath);
  const lint = lintRubric(rubric);
  if (lint.length) fail(`rubric fails lint: ${lint.join('; ')}`);
  const sheets = sheetPaths.map(readJson);
  for (const s of sheets) {
    if (s.rubricName !== rubric.name || s.rubricVersion !== rubric.version) {
      fail(`sheet is for ${s.rubricName} v${s.rubricVersion}, rubric is ${rubric.name} v${rubric.version}`);
    }
  }

  if (sheets.length === 1) {
    process.stdout.write(JSON.stringify({ rubric: `${rubric.name} v${rubric.version}`, score: computeGradeScore(rubric, sheets[0]) }, null, 2) + '\n');
    process.exit(0);
  }

  const tiebreak = tiebreakPath ? readJson(tiebreakPath) : null;
  const disagreements = diffGradeSheets(rubric, sheets[0], sheets[1]);
  const { sheet, unresolved } = mergeGradeSheets(rubric, sheets[0], sheets[1], tiebreak);
  const report = {
    rubric: `${rubric.name} v${rubric.version}`,
    primaries: sheets.map((s) => computeGradeScore(rubric, s)),
    disagreements,
    unresolved,
    consensus: unresolved.length === 0 ? computeGradeScore(rubric, sheet) : null,
    consensusSheet: sheet,
  };
  process.stdout.write(JSON.stringify(report, null, 2) + '\n');
  process.exit(unresolved.length === 0 ? 0 : 1);
} catch (err) {
  fail(err instanceof Error ? err.message : String(err));
}
