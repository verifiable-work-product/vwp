# VWP Provenance

**Status:** v0.1 draft, experimental.

## 1. Overview

`provenance/provenance.json` is a required package part recording how the package was
produced: who created it, with what toolchain, which models (if any) assisted, under
which policy versions, and from which source systems. Provenance supports accountability
and reproducibility inquiries; it is descriptive, not a trust mechanism by itself. It
validates against `https://vwp.dev/schemas/v0.1/provenance.schema.json`.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `createdAt` | string | yes | RFC 3339 timestamp of package creation. |
| `createdBy` | string / object | yes | Person, team, or service that produced the package. |
| `toolchain` | array | no | Tools involved, each with name and version (e.g. `{ "name": "vwp-cli", "version": "0.1.3" }`). |
| `models` | array | no | AI models involved. Each entry MUST record the exact model: `provider`, `name` (the provider's published model identifier, e.g. `claude-fable-5` — not a family or marketing name), and `version` (the most specific version or snapshot identifier available), plus an optional `role` (e.g. extraction, drafting). |
| `policyVersions` | array / object | no | Versions of organizational policies or rulebooks applied. |
| `sourceSystems` | array | no | Systems evidence was drawn from (names, versions, environments). |
| `environment` | string / object | no | Execution environment detail (e.g. deployment, region, run id). |
| `notes` | string | no | Free-form production notes. |

## 3. Model-agnosticism

Provenance MAY record which model produced or assisted with a package, but verification
never requires that model. Package validation is deterministic (see
[validations.md](validations.md)); a package produced entirely by hand and a package
produced with heavy model assistance are verified identically. `models` entries exist for
transparency and audit-trail purposes, not as an input to verification.

Exact model identity serves a second purpose beyond transparency: **model diversity in
review**. A reviewer or auditor who challenges model-assisted work with the *same* model
that produced it inherits that model's correlated blind spots. Recording the exact
provider, model name, and version lets downstream reviewers deliberately select a
different model — or a different model family — for independent analysis. Vague entries
("an LLM", a marketing name without a version) defeat this and SHOULD be treated as a
documentation deficiency.

## 4. Semantics

- Producers SHOULD record every tool and model that materially shaped package content.
  Overstating is harmless; omitting is misleading.
- Every `models` entry MUST identify the exact model — `provider`, `name`, and `version`
  are required by the schema. When a provider exposes dated snapshots or build
  identifiers, producers SHOULD record the most specific one available.
- `createdAt` in provenance SHOULD match `createdAt` in the manifest; verifiers MAY warn
  on divergence.
- Provenance entries are claims by the producer. They are tamper-evident only when the
  manifest declares the provenance part's `sha256` and the manifest is signed (see
  [signatures.md](signatures.md)); they are not independently authenticated in v0.1.
- Provenance MUST NOT contain secrets (credentials, tokens, private endpoints). See the
  privacy considerations in [evidence.md](evidence.md).

## 5. Example

```json
{
  "createdAt": "2026-07-01T18:22:05Z",
  "createdBy": { "type": "system", "name": "Close Automation Service" },
  "toolchain": [
    { "name": "vwp-cli", "version": "0.1.3" },
    { "name": "close-pipeline", "version": "2026.6.2" }
  ],
  "models": [
    {
      "provider": "example-ai",
      "name": "example-model-large",
      "version": "2026-05-14",
      "role": "fact extraction"
    }
  ],
  "policyVersions": [
    { "id": "DA-4", "version": "4.2" }
  ],
  "sourceSystems": [
    { "name": "NetSuite" },
    { "name": "BankOfExample portal" },
    { "name": "Etherscan" }
  ],
  "environment": { "deployment": "prod", "runId": "close-2026-06-run-14" },
  "notes": "Facts extracted automatically; judgments proposed by preparer."
}
```
