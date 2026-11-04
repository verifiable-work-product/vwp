#!/usr/bin/env node
/**
 * VWP benchmark harness runner.
 *
 * Primary invocation (platform mode):
 *   harness run --model anthropic:<id> --challenge <challengeId> \
 *     [--base-url https://vwp-tt.web.app] --api-key <ttk_...>
 *
 * Alternatives: --packet-url <url> (explicit packet endpoint) or
 * --packet-dir <dir> (local packet directory; no platform contact).
 *
 * Flow: run the agent loop; ALWAYS assemble the .vwp locally, run the full
 * offline battery on the finished artifact, print a results table, and write
 * the artifact to the run dir. Only then — behind a confirmation gate — POST
 * the built .vwp as application/zip to the platform's /submissions endpoint
 * (Flow C1 upload). --yes skips the prompt (CI); --no-upload never prompts;
 * non-TTY without --yes prints the exact curl instead of uploading.
 */
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as readline from 'node:readline/promises';
import { parseArgs } from 'node:util';
import type { ProviderAdapter } from './adapters/types.js';
import { AnthropicAdapter } from './adapters/anthropic.js';
import { OpenAIAdapter } from './adapters/openai.js';
import { FakeAdapter, type FakeStep } from './adapters/fake.js';
import type { ModelIdentity } from './draft.js';
import { loadLocalPacket, loadRemotePacket, type Packet } from './packet.js';
import { DEFAULT_MAX_STEPS, describeEnd, runLoop, type LoopResult } from './loop.js';
import { runValidation, type CheckId, type ValidationResult } from './validate.js';
import { HARNESS_VERSION } from './assemble.js';

const USAGE = `vwp-benchmark-harness ${HARNESS_VERSION}

Usage:
  npm run -s run -- --model anthropic:<id> --challenge <challengeId> [--base-url <url>] --api-key <ttk_...>
  npm run -s run -- --model openai:<id> --packet-url <url> --api-key <ttk_...>
  npm run -s run -- --model fake --packet-dir <dir> [--out <file.vwp>]

Options:
  --model <spec>       anthropic:<model-id> | openai:<model-id> | fake   (required)
  --challenge <id>     Challenge id; packet URL derived from --base-url
  --base-url <url>     Platform base URL (default https://vwp-tt.web.app)
  --api-key <key>      Enrolled llm_runner API key (or VWP_TT_API_KEY env)
  --packet-url <url>   Explicit packet endpoint (alternative to --challenge)
  --packet-dir <dir>   Local packet directory (no platform contact)
  --out <file>         Also copy the artifact to this path
  --runs <n>           Independent runs, fresh context each (default 1)
  --max-steps <n>      Tool-call budget per run (default ${DEFAULT_MAX_STEPS})
  --run-dir <dir>      Base directory for run outputs (default ./runs)
  --model-version <v>  Provenance model version (default: the model id)
  --yes                Upload without prompting (CI)
  --no-upload          Never upload; local artifact only
`;

interface CliOptions {
  model: string;
  challenge?: string;
  baseUrl: string;
  apiKey?: string;
  packetUrl?: string;
  packetDir?: string;
  out?: string;
  runs: number;
  maxSteps: number;
  runDirBase: string;
  modelVersion?: string;
  yes: boolean;
  noUpload: boolean;
}

function parseCli(argv: string[]): CliOptions {
  const { values } = parseArgs({
    args: argv,
    options: {
      model: { type: 'string' },
      challenge: { type: 'string' },
      'base-url': { type: 'string', default: 'https://vwp-tt.web.app' },
      'api-key': { type: 'string' },
      'packet-url': { type: 'string' },
      'packet-dir': { type: 'string' },
      out: { type: 'string' },
      runs: { type: 'string', default: '1' },
      'max-steps': { type: 'string', default: String(DEFAULT_MAX_STEPS) },
      'run-dir': { type: 'string', default: 'runs' },
      'model-version': { type: 'string' },
      yes: { type: 'boolean', default: false },
      'no-upload': { type: 'boolean', default: false },
      help: { type: 'boolean', default: false },
    },
  });
  if (values.help || !values.model) {
    process.stdout.write(USAGE);
    process.exit(values.help ? 0 : 2);
  }
  return {
    model: values.model,
    challenge: values.challenge,
    baseUrl: (values['base-url'] as string).replace(/\/$/, ''),
    apiKey: values['api-key'] ?? process.env.VWP_TT_API_KEY,
    packetUrl: values['packet-url'],
    packetDir: values['packet-dir'],
    out: values.out,
    runs: Number.parseInt(values.runs as string, 10),
    maxSteps: Number.parseInt(values['max-steps'] as string, 10),
    runDirBase: values['run-dir'] as string,
    modelVersion: values['model-version'],
    yes: values.yes as boolean,
    noUpload: values['no-upload'] as boolean,
  };
}

function makeAdapter(spec: string, packet: Packet): { adapter: ProviderAdapter; identity: ModelIdentity } {
  if (spec === 'fake') {
    return {
      adapter: new FakeAdapter(dryRunScript(packet)),
      identity: { provider: 'fake', name: 'scripted-dry-run', version: '0' },
    };
  }
  const sep = spec.indexOf(':');
  if (sep === -1) throw new Error(`--model must be anthropic:<id>, openai:<id>, or fake (got: ${spec})`);
  const provider = spec.slice(0, sep);
  const modelId = spec.slice(sep + 1);
  if (provider === 'anthropic') {
    return { adapter: new AnthropicAdapter(modelId), identity: { provider, name: modelId, version: modelId } };
  }
  if (provider === 'openai') {
    return { adapter: new OpenAIAdapter(modelId), identity: { provider, name: modelId, version: modelId } };
  }
  throw new Error(`unknown provider '${provider}' (expected anthropic or openai)`);
}

/** A minimal scripted sequence over the actual packet, for dry runs. */
function dryRunScript(packet: Packet): FakeStep[] {
  const ev = packet.evidence[0];
  if (!ev) throw new Error('packet has no evidence; cannot build a dry-run script');
  const quote = ev.text.split('\n').find((l) => l.trim().length > 10)?.trim();
  if (!quote) throw new Error(`evidence ${ev.id} has no quotable line`);
  return [
    { toolCalls: [{ name: 'read_brief', input: {} }] },
    { toolCalls: [{ name: 'list_evidence', input: {} }] },
    { toolCalls: [{ name: 'read_evidence', input: { evidenceId: ev.id } }] },
    {
      toolCalls: [
        {
          name: 'add_fact',
          input: {
            factType: 'harness.dryRun.observation',
            value: quote,
            statement: `Dry-run observation quoted from ${ev.id}.`,
            anchors: [{ evidenceId: ev.id, quote }],
          },
        },
      ],
    },
    {
      toolCalls: [
        {
          name: 'add_assessment',
          input: {
            statement: 'Dry-run assessment: the harness pipeline produces a verifiable artifact.',
            supportingFacts: ['f_1'],
          },
        },
      ],
    },
    { toolCalls: [{ name: 'write_memo', input: { markdown: '# Dry run\n\nScripted harness dry run; not a work product.\n' } }] },
    { toolCalls: [{ name: 'validate', input: {} }] },
    { toolCalls: [{ name: 'submit', input: {} }] },
  ];
}

const CHECKS: CheckId[] = ['G1', 'G2', 'C1', 'K1', 'S2', 'P1'];

function printResultsTable(result: LoopResult, validation: ValidationResult): void {
  const rows: Array<[string, string]> = [];
  rows.push(['verify', validation.verify.valid ? 'VALID' : 'INVALID']);
  for (const check of CHECKS) {
    const findings = validation.findings.filter((f) => f.check === check);
    const failed = findings.filter((f) => f.status === 'fail');
    rows.push([check, failed.length === 0 ? 'PASS' : `FAIL (${failed.length})`]);
  }
  const cutOff = result.truncatedCalls > 0 ? `, ${result.truncatedCalls} cut off` : '';
  rows.push(['steps', `${result.steps} tool calls / ${result.turns} turns (${result.toolErrors} tool errors${cutOff})`]);
  rows.push(['tokens', `${result.usage.inputTokens} in / ${result.usage.outputTokens} out`]);
  rows.push(['ended by', describeEnd(result)]);

  const width = Math.max(...rows.map(([k]) => k.length));
  console.log('┌' + '─'.repeat(width + 2) + '┬' + '─'.repeat(58) + '┐');
  for (const [k, v] of rows) {
    console.log(`│ ${k.padEnd(width)} │ ${v.padEnd(56)} │`);
  }
  console.log('└' + '─'.repeat(width + 2) + '┴' + '─'.repeat(58) + '┘');
  if (result.refusal) {
    console.log(`Provider refusal (${result.refusal.category ?? 'no category'}): ${result.refusal.explanation ?? '(no explanation given)'}`);
  }
  const failures = validation.findings.filter((f) => f.status === 'fail');
  if (failures.length > 0) {
    console.log('Failing findings:');
    for (const f of failures) console.log(`  [${f.check}] ${f.message}`);
  }
}

async function confirmUpload(target: string): Promise<boolean> {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  try {
    const answer = await rl.question(`Upload to ${target}? [y/N] `);
    return /^y(es)?$/i.test(answer.trim());
  } finally {
    rl.close();
  }
}

async function uploadArtifact(submissionsUrl: string, apiKey: string, artifact: Buffer): Promise<void> {
  const res = await fetch(submissionsUrl, {
    method: 'POST',
    headers: { Authorization: `Bearer ${apiKey}`, 'Content-Type': 'application/zip' },
    body: new Uint8Array(artifact),
  });
  const body = await res.text().catch(() => '');
  if (!res.ok) {
    throw new Error(`upload failed: ${res.status} ${res.statusText} ${body.slice(0, 500)}`);
  }
  let submissionId: string | undefined;
  try {
    const parsed = JSON.parse(body);
    submissionId = parsed.id ?? parsed.submissionId;
  } catch {
    /* non-JSON response */
  }
  console.log(`Uploaded. Submission: ${submissionId ?? body.slice(0, 200)}`);
}

async function main(): Promise<void> {
  const cli = parseCli(process.argv.slice(2));
  if (!cli.packetDir && !cli.packetUrl && !cli.challenge) {
    process.stderr.write('error: one of --packet-dir, --packet-url, or --challenge is required\n\n' + USAGE);
    process.exit(2);
  }

  const stamp = new Date().toISOString().replace(/[:.]/g, '-');
  const modelSafe = cli.model.replace(/[^A-Za-z0-9._-]/g, '_');
  const baseRunDir = path.resolve(cli.runDirBase);

  // Resolve the packet source.
  let packet: Packet;
  let submissionsUrl: string | undefined;
  if (cli.packetDir) {
    packet = loadLocalPacket(cli.packetDir);
  } else {
    const packetUrl = cli.packetUrl ?? `${cli.baseUrl}/api/challenges/${cli.challenge}/packet`;
    if (!cli.apiKey) {
      process.stderr.write('error: --api-key (or VWP_TT_API_KEY) is required for platform packets\n');
      process.exit(2);
    }
    const packetWorkDir = path.join(baseRunDir, `${stamp}-packet`);
    packet = await loadRemotePacket(packetUrl, cli.apiKey!, packetWorkDir);
    submissionsUrl = packetUrl.replace(/\/packet\/?$/, '') + '/submissions';
  }

  const { adapter, identity } = makeAdapter(cli.model, packet);
  if (cli.modelVersion) identity.version = cli.modelVersion;

  for (let runNo = 1; runNo <= cli.runs; runNo++) {
    const runDir = path.join(baseRunDir, `${stamp}-${packet.challengeId}-${modelSafe}-run${runNo}`);
    console.log(`\n=== run ${runNo}/${cli.runs} — ${cli.model} on ${packet.challengeId} ===`);
    console.log(`run dir: ${runDir}`);

    const result = await runLoop({
      adapter,
      packet,
      modelIdentity: identity,
      maxSteps: cli.maxSteps,
      runDir,
    });

    // Always: offline battery on the finished artifact + results table.
    const validation = runValidation(result.draft, packet, { modelIdentity: identity });
    printResultsTable(result, validation);

    // Always: write the artifact locally.
    const artifactPath = path.join(runDir, 'workproduct.vwp');
    fs.writeFileSync(artifactPath, result.artifact);
    console.log(`artifact: ${artifactPath}`);
    console.log(`transcript: ${result.transcriptPath}`);
    if (cli.out) {
      const outPath = cli.runs > 1 ? cli.out.replace(/(\.vwp)?$/, `-run${runNo}$1`) : cli.out;
      fs.mkdirSync(path.dirname(path.resolve(outPath)), { recursive: true });
      fs.writeFileSync(outPath, result.artifact);
      console.log(`artifact copy: ${outPath}`);
    }

    // Confirmation gate before anything leaves the machine.
    if (!submissionsUrl || cli.noUpload) continue;
    let doUpload = false;
    if (cli.yes) {
      doUpload = true;
    } else if (process.stdin.isTTY && process.stdout.isTTY) {
      doUpload = await confirmUpload(submissionsUrl);
    } else {
      console.log('Non-interactive session and no --yes: not uploading. To upload manually:');
      console.log(
        `  curl -X POST '${submissionsUrl}' -H 'Authorization: Bearer <api-key>' -H 'Content-Type: application/zip' --data-binary @${artifactPath}`
      );
      continue;
    }
    if (doUpload) {
      await uploadArtifact(submissionsUrl, cli.apiKey!, result.artifact);
    } else {
      console.log('Not uploaded.');
    }
  }
}

main().catch((err) => {
  process.stderr.write(`error: ${(err as Error).stack ?? err}\n`);
  process.exit(1);
});
