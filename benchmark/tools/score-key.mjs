#!/usr/bin/env node
// Deterministically score a work product's keyed items against a challenge
// answer key. Works identically for agent packages and transcribed CPA
// worksheets — both are facts/facts.json by scoring time.
//
// Usage:
//   node score-key.mjs <package-dir | facts.json> <answer-key.json> <required-facts.json>
//
// Output: a JSON report on stdout. Exit 0 always (scoring, not verification);
// exit 2 on usage/input errors. All logic lives in lib/scoring.mjs.

import * as fs from 'fs';
import * as path from 'path';
import { scoreKeyedItems } from './lib/scoring.mjs';

function fail(msg) {
  process.stderr.write(`error: ${msg}\n`);
  process.exit(2);
}

const args = process.argv.slice(2);
if (args.length !== 3) {
  fail('usage: score-key.mjs <package-dir | facts.json> <answer-key.json> <required-facts.json>');
}
const [factsArg, keyPath, requiredPath] = args;

try {
  const factsFile = fs.statSync(factsArg).isDirectory() ? path.join(factsArg, 'facts', 'facts.json') : factsArg;
  const facts = JSON.parse(fs.readFileSync(factsFile, 'utf8')).facts ?? [];
  const key = JSON.parse(fs.readFileSync(keyPath, 'utf8'));
  const required = JSON.parse(fs.readFileSync(requiredPath, 'utf8'));
  const report = scoreKeyedItems(facts, key, required);
  process.stdout.write(JSON.stringify({ factsFile, ...report }, null, 2) + '\n');
} catch (err) {
  fail(err instanceof Error ? err.message : String(err));
}
