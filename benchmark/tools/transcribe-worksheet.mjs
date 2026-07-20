#!/usr/bin/env node
// Transcribe a CPA extraction worksheet (CSV) into VWP facts/facts.json.
//
// Usage:
//   node transcribe-worksheet.mjs <worksheet.csv> <required-facts.json> [--evidence <dir>] > facts.json
//
// With --evidence <dir> (the directory containing evidence.json), a row's
// page/line pin cite is resolved against the numbered-copy convention
// (lib/numbering.mjs) and the cited text is filled in as a machine-verifiable
// quote anchor. An explicit verbatim_quote wins over a resolved one.
// All logic lives in lib/worksheet.mjs; this file is argv/fs wiring.

import * as fs from 'fs';
import * as path from 'path';
import { transcribeWorksheet } from './lib/worksheet.mjs';

function fail(msg) {
  process.stderr.write(`error: ${msg}\n`);
  process.exit(1);
}

const args = process.argv.slice(2);
const positional = [];
let evidenceDir;
for (let i = 0; i < args.length; i++) {
  if (args[i] === '--evidence') {
    evidenceDir = args[++i];
    if (!evidenceDir) fail('--evidence requires a directory');
  } else if (args[i].startsWith('--')) {
    fail(`unknown option ${args[i]}`);
  } else {
    positional.push(args[i]);
  }
}
if (positional.length !== 2) {
  fail('usage: transcribe-worksheet.mjs <worksheet.csv> <required-facts.json> [--evidence <dir>] > facts.json');
}
const [worksheetPath, requiredFactsPath] = positional;

const required = JSON.parse(fs.readFileSync(requiredFactsPath, 'utf8'));

const evidenceText = new Map();
if (evidenceDir) {
  const indexPath = path.join(evidenceDir, 'evidence.json');
  if (!fs.existsSync(indexPath)) fail(`no evidence.json in ${evidenceDir}`);
  for (const ev of JSON.parse(fs.readFileSync(indexPath, 'utf8')).evidence ?? []) {
    if (!ev.path) continue;
    const file = path.join(evidenceDir, '..', ev.path);
    if (fs.existsSync(file)) evidenceText.set(ev.id, fs.readFileSync(file, 'utf8'));
  }
}

try {
  const { facts, missingSlots } = transcribeWorksheet(fs.readFileSync(worksheetPath, 'utf8'), required, evidenceText);
  if (missingSlots.length > 0) {
    process.stderr.write(`warning: worksheet is missing slots: ${missingSlots.join(', ')}\n`);
  }
  process.stdout.write(JSON.stringify({ facts }, null, 2) + '\n');
} catch (err) {
  fail(err instanceof Error ? err.message : String(err));
}
