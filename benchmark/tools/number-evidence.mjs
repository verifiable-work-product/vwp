#!/usr/bin/env node
// Produce the numbered copies of a challenge's text evidence — the versions
// CPAs read and cite by page/line, like a deposition transcript.
//
// Usage:
//   node number-evidence.mjs <evidence-dir> <output-dir>
//
// Reads <evidence-dir>/evidence.json, numbers every text evidence file
// (.txt, .md, .csv, .json), and writes <output-dir>/<evidenceId>.numbered.txt.
// Binary evidence must first get a derived text layer (spec/evidence.md §4);
// the numbered copy is then produced from the text layer.

import * as fs from 'fs';
import * as path from 'path';
import { renderNumberedCopy } from './lib/numbering.mjs';

function fail(msg) {
  process.stderr.write(`error: ${msg}\n`);
  process.exit(2);
}

const [evidenceDir, outputDir] = process.argv.slice(2);
if (!evidenceDir || !outputDir) fail('usage: number-evidence.mjs <evidence-dir> <output-dir>');

const indexPath = path.join(evidenceDir, 'evidence.json');
if (!fs.existsSync(indexPath)) fail(`no evidence.json in ${evidenceDir}`);
const evidence = JSON.parse(fs.readFileSync(indexPath, 'utf8')).evidence ?? [];

fs.mkdirSync(outputDir, { recursive: true });
const TEXT_EXTENSIONS = ['.txt', '.md', '.csv', '.json'];
let count = 0;
for (const ev of evidence) {
  if (!ev.path) continue;
  if (!TEXT_EXTENSIONS.some((ext) => ev.path.toLowerCase().endsWith(ext))) {
    process.stderr.write(`skip ${ev.id}: not text (${ev.path}) — add a derived text layer first\n`);
    continue;
  }
  // Evidence paths are package-relative (evidence/foo.txt); resolve against the dir's parent.
  const file = path.join(evidenceDir, '..', ev.path);
  if (!fs.existsSync(file)) {
    process.stderr.write(`skip ${ev.id}: file not found: ${ev.path}\n`);
    continue;
  }
  const numbered = renderNumberedCopy(fs.readFileSync(file, 'utf8'), `${ev.id}: ${ev.title ?? ev.path}`);
  fs.writeFileSync(path.join(outputDir, `${ev.id}.numbered.txt`), numbered);
  count++;
}
process.stderr.write(`numbered ${count} evidence file${count === 1 ? '' : 's'} -> ${outputDir}\n`);
