#!/usr/bin/env node
import { Command } from 'commander';
import { verifyCommand } from './commands/verify';
import { inspectCommand } from './commands/inspect';
import { packCommand } from './commands/pack';
import { unpackCommand } from './commands/unpack';
import { diffCommand } from './commands/diff';
import { renderCommand } from './commands/render';
import { originsCommand } from './commands/origins';
import {
  addCalculationCommand,
  addClaimCommand,
  addEvidenceCommand,
  addFactCommand,
  buildCommand,
  embedCommand,
  initCommand,
  setReadableCommand,
} from './commands/construct';

const program = new Command();

program
  .name('vwp')
  .description('Reference CLI for VWP (Verifiable Work Product) packages')
  .version('0.1.0');

function run(fn: () => number | Promise<number>): void {
  Promise.resolve()
    .then(fn)
    .then((code) => {
      process.exitCode = code;
    })
    .catch((err) => {
      console.error(`error: ${err instanceof Error ? err.message : String(err)}`);
      process.exitCode = 2;
    });
}

program
  .command('init')
  .description('initialize a valid, viewable VWP package directory')
  .argument('<directory>', 'new empty package directory')
  .option('--title <title>', 'work product title; defaults from the directory name')
  .option('--package-id <id>', 'package identifier; defaults to a UUID URN')
  .option('--workproduct-id <id>', 'work product identifier')
  .option('--profile <profile>', 'VWP profile', 'vwp-core')
  .option('--extension <extension>', 'profile extension')
  .option('--domain <domain>', 'work product domain', 'general')
  .option('--status <status>', 'work product status', 'draft')
  .option('--created-by <name>', 'preparer name; defaults to the local username')
  .option('--creator-type <type>', 'human, agent, or system', 'human')
  .action((directory: string, options) => run(() => initCommand(directory, options)));

const add = program.command('add').description('add a structured object to a package directory');

add
  .command('evidence')
  .description('add local or externally referenced evidence')
  .argument('<directory>', 'package directory')
  .option('--json <file>', 'complete evidence JSON object')
  .option('--id <id>', 'evidence id')
  .option('--title <title>', 'evidence title')
  .option('--type <type>', 'evidence type', 'document')
  .option('--file <file>', 'local evidence file to copy')
  .option('--path <path>', 'package path for the copied evidence')
  .option('--external-ref <locator>', 'external evidence locator')
  .option('--locator-type <type>', 'url, path, vwp-ref, or opaque')
  .option('--source-system <name>', 'source system name')
  .action((directory: string, options) => run(() => addEvidenceCommand(directory, options)));

add
  .command('fact')
  .description('add an evidence-backed fact')
  .argument('<directory>', 'package directory')
  .option('--json <file>', 'complete fact JSON object')
  .option('--id <id>', 'fact id')
  .option('--statement <statement>', 'fact statement')
  .option('--evidence <ids...>', 'supporting evidence ids (space- or comma-separated)')
  .option('--fact-type <type>', 'namespaced fact type')
  .option('--value <json>', 'structured JSON value')
  .option('--confidence <number>', 'confidence from 0 to 1')
  .option('--requires-review', 'mark the fact for review')
  .action((directory: string, options) => run(() => addFactCommand(directory, options)));

add
  .command('claim')
  .description('add a supported claim or conclusion')
  .argument('<directory>', 'package directory')
  .option('--json <file>', 'complete claim JSON object')
  .option('--id <id>', 'claim id')
  .option('--statement <statement>', 'claim statement')
  .option('--fact <ids...>', 'supporting fact ids')
  .option('--evidence <ids...>', 'supporting evidence ids')
  .option('--calculation <ids...>', 'supporting calculation ids')
  .option('--judgment <ids...>', 'supporting judgment ids')
  .option('--assertion <names...>', 'assertion vocabulary terms')
  .option('--conclusion <text>', 'claim conclusion')
  .option('--status <status>', 'proposed, supported, rejected, or superseded', 'proposed')
  .option('--requires-review', 'mark the claim for review')
  .action((directory: string, options) => run(() => addClaimCommand(directory, options)));

add
  .command('calculation')
  .description('add a deterministic calculation')
  .argument('<directory>', 'package directory')
  .option('--json <file>', 'complete calculation JSON object')
  .option('--id <id>', 'calculation id')
  .option('--type <type>', 'calculation type')
  .option('--inputs <json>', 'JSON object containing calculation inputs', '{}')
  .option('--outputs <json>', 'JSON object containing calculation outputs', '{}')
  .option('--method <text>', 'calculation method')
  .option('--validator <id>', 'recomputation validator id')
  .option('--recomputable', 'mark the calculation as recomputable')
  .action((directory: string, options) => run(() => addCalculationCommand(directory, options)));

program
  .command('set-readable')
  .description('copy a Markdown file into the package as its preferred readable')
  .argument('<directory>', 'package directory')
  .argument('<markdown>', 'Markdown source file')
  .option('--path <path>', 'package path', 'renderings/readable.md')
  .action((directory: string, markdown: string, options) => run(() => setReadableCommand(directory, markdown, options)));

program
  .command('embed')
  .description('embed a native work product or supporting artifact')
  .argument('<directory>', 'package directory')
  .argument('<file>', 'file to embed')
  .option('--path <path>', 'package path; defaults under artifacts/')
  .option('--media-type <type>', 'IANA media type; inferred from extension')
  .option('--role <role>', 'primary-artifact or artifact', 'primary-artifact')
  .action((directory: string, file: string, options) => run(() => embedCommand(directory, file, options)));

program
  .command('build')
  .description('hash, pack, and verify a package directory')
  .argument('<directory>', 'package directory')
  .argument('<output>', 'output .vwp file')
  .option('--no-verify', 'skip verification after packing')
  .action((directory: string, output: string, options) => run(() => buildCommand(directory, output, options)));

program
  .command('pack')
  .description('create a .vwp package from a package directory')
  .argument('<directory>', 'package directory containing manifest.json')
  .argument('<output>', 'output .vwp file')
  .option('--write-hashes', 'compute SHA-256 hashes for declared parts and update the manifest')
  .action((dir: string, output: string, options: { writeHashes?: boolean }) =>
    run(() => packCommand(dir, output, options))
  );

program
  .command('unpack')
  .description('extract a .vwp package (refuses unsafe paths)')
  .argument('<package>', '.vwp file')
  .argument('<directory>', 'output directory')
  .action((input: string, outDir: string) => run(() => unpackCommand(input, outDir)));

program
  .command('verify')
  .description('run core, profile, and extension validators against a package')
  .argument('<package>', '.vwp file or package directory')
  .option('--strict', 'treat warnings and review-required items as failures')
  .option('--online', 'also run online checks (fetch externalRef URLs with their declared auth and verify contentHash)')
  .action((input: string, options: { strict?: boolean; online?: boolean }) => run(() => verifyCommand(input, options)));

program
  .command('origins')
  .description('list the network origins external references want to reach, and the auth each needs (no network, no secrets)')
  .argument('<package>', '.vwp file or package directory')
  .action((input: string) => run(() => originsCommand(input)));

program
  .command('inspect')
  .description('print a summary of a package')
  .argument('<package>', '.vwp file or package directory')
  .action((input: string) => run(() => inspectCommand(input)));

program
  .command('diff')
  .description('structural diff between two packages')
  .argument('<old>', 'old .vwp file or package directory')
  .argument('<new>', 'new .vwp file or package directory')
  .action((oldInput: string, newInput: string) => run(() => diffCommand(oldInput, newInput)));

program
  .command('render')
  .description("print the package's markdown rendering")
  .argument('<package>', '.vwp file or package directory')
  .action((input: string) => run(() => renderCommand(input)));

program.parse();
