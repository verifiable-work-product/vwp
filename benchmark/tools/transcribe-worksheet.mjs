#!/usr/bin/env node
// Transcribe a CPA extraction worksheet (CSV) into VWP facts/facts.json.
//
// Usage:
//   node transcribe-worksheet.mjs <worksheet.csv> <required-facts.json> [--evidence <dir>] > facts.json
//
// Each worksheet row becomes a fact with the slot's declared factType, a value
// in the slot's declared shape, and an anchored evidence reference. Provenance
// (createdBy: human, transcription tool in toolchain) is the packager's job.
//
// With --evidence <dir> (the directory containing evidence.json), a row's
// page/line pin cite is resolved against the numbered-copy convention
// (lib/numbering.mjs) and the cited text is filled in as anchor.quote — so
// CPAs cite page/line off the numbered copies and machine-verifiable quote
// anchors fall out of transcription. An explicit verbatim_quote wins over a
// resolved one.

import * as fs from 'fs';
import * as path from 'path';
import { resolvePinCite } from './lib/numbering.mjs';

function parseCsv(text) {
  const rows = [];
  let row = [];
  let field = '';
  let inQuotes = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inQuotes) {
      if (c === '"' && text[i + 1] === '"') {
        field += '"';
        i++;
      } else if (c === '"') {
        inQuotes = false;
      } else {
        field += c;
      }
    } else if (c === '"') {
      inQuotes = true;
    } else if (c === ',') {
      row.push(field);
      field = '';
    } else if (c === '\n' || c === '\r') {
      if (c === '\r' && text[i + 1] === '\n') i++;
      row.push(field);
      field = '';
      if (row.some((f) => f.length > 0)) rows.push(row);
      row = [];
    } else {
      field += c;
    }
  }
  row.push(field);
  if (row.some((f) => f.length > 0)) rows.push(row);
  return rows;
}

function fail(msg) {
  process.stderr.write(`error: ${msg}\n`);
  process.exit(1);
}

function slotValue(slot, raw, currencyOrUnit, rowNum) {
  switch (slot.valueShape) {
    case 'money': {
      const amount = Number(raw.replace(/[$,\s]/g, ''));
      if (!Number.isFinite(amount)) fail(`row ${rowNum} (${slot.slot}): '${raw}' is not a number`);
      return { amount, currency: currencyOrUnit || 'USD' };
    }
    case 'quantity': {
      const value = Number(raw.replace(/[,\s]/g, ''));
      if (!Number.isFinite(value)) fail(`row ${rowNum} (${slot.slot}): '${raw}' is not a number`);
      return { value, unit: currencyOrUnit || slot.unit || '' };
    }
    case 'enum':
      if (slot.enumValues && !slot.enumValues.includes(raw)) {
        fail(`row ${rowNum} (${slot.slot}): '${raw}' is not one of ${slot.enumValues.join(', ')}`);
      }
      return raw;
    case 'boolean':
      return raw.trim().toLowerCase() === 'true';
    default:
      return raw;
  }
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
const slotsByName = new Map(required.slots.map((s) => [s.slot, s]));

// evidence id -> file text, for resolving pin cites into quotes.
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

const rows = parseCsv(fs.readFileSync(worksheetPath, 'utf8'));
const header = rows[0].map((h) => h.trim());
const col = (name) => header.indexOf(name);
for (const requiredCol of ['slot', 'statement', 'value', 'evidence_id']) {
  if (col(requiredCol) === -1) fail(`worksheet has no '${requiredCol}' column`);
}

const facts = [];
const seenSlots = new Set();
rows.slice(1).forEach((row, i) => {
  const rowNum = i + 2;
  const get = (name) => (col(name) === -1 ? '' : (row[col(name)] ?? '').trim());
  const slotName = get('slot');
  const slot = slotsByName.get(slotName);
  if (!slot) fail(`row ${rowNum}: unknown slot '${slotName}'`);
  if (seenSlots.has(slotName)) fail(`row ${rowNum}: duplicate slot '${slotName}'`);
  seenSlots.add(slotName);

  const anchor = {};
  if (get('verbatim_quote')) anchor.quote = get('verbatim_quote');
  if (get('page')) anchor.page = Number(get('page'));
  if (get('lines')) {
    const m = get('lines').match(/^(\d+)(?:\s*-\s*(\d+))?$/);
    if (!m) fail(`row ${rowNum} (${slotName}): lines '${get('lines')}' is not N or N-M`);
    anchor.lines = m[2] ? { from: Number(m[1]), to: Number(m[2]) } : { from: Number(m[1]) };
  }
  if (get('cells')) anchor.cells = { range: get('cells') };
  if (get('section')) anchor.section = get('section');

  // Resolve the pin cite into a quote when the CPA gave page/lines but no quote.
  if (!anchor.quote && anchor.page && anchor.lines && evidenceText.has(get('evidence_id'))) {
    const cited = resolvePinCite(
      evidenceText.get(get('evidence_id')),
      anchor.page,
      anchor.lines.from,
      anchor.lines.to
    );
    if (cited === undefined) {
      fail(`row ${rowNum} (${slotName}): pin cite p.${anchor.page} ln.${get('lines')} is outside ${get('evidence_id')}`);
    }
    anchor.quote = cited;
  }

  const ref =
    Object.keys(anchor).length > 0
      ? { evidenceId: get('evidence_id'), anchors: [anchor] }
      : get('evidence_id');

  const fact = {
    id: `f_${slotName}`,
    statement: get('statement'),
    factType: slot.factType,
    value: slotValue(slot, get('value'), get('currency_or_unit'), rowNum),
    evidenceRefs: [ref],
    extractionMethod: 'manual',
  };
  if (get('notes')) fact.notes = get('notes');
  facts.push(fact);
});

const missing = [...slotsByName.keys()].filter((s) => !seenSlots.has(s));
if (missing.length > 0) {
  process.stderr.write(`warning: worksheet is missing slots: ${missing.join(', ')}\n`);
}

process.stdout.write(JSON.stringify({ facts }, null, 2) + '\n');
