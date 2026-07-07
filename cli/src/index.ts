#!/usr/bin/env node
import { Command } from 'commander';
import { verifyCommand } from './commands/verify';
import { inspectCommand } from './commands/inspect';
import { packCommand } from './commands/pack';
import { unpackCommand } from './commands/unpack';
import { diffCommand } from './commands/diff';
import { renderCommand } from './commands/render';

const program = new Command();

program
  .name('vwp')
  .description('Reference CLI for VWP (Verifiable Work Product) packages')
  .version('0.1.0');

function run(fn: () => number): void {
  try {
    process.exitCode = fn();
  } catch (err) {
    console.error(`error: ${err instanceof Error ? err.message : String(err)}`);
    process.exitCode = 2;
  }
}

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
  .action((input: string, options: { strict?: boolean }) => run(() => verifyCommand(input, options)));

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
