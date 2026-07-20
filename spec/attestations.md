# VWP Analysis Attestations

**Status:** v0.1 draft, experimental.

## 1. Overview

Analysis attestations are structured records of model-assisted review procedures
performed over a defined evidence scope: which model was directed at which evidence, what
it was asked to challenge, what it found, what it could not assess, and how humans
disposed of the findings. They give the *reviewing* role of models its own inspectable
artifact, distinct from deterministic validation (see the whitepaper, section 4.5).
Attestation objects live under `attestations/` (one JSON object per file) and validate
against `https://vwp.dev/schemas/v0.1/analysis-attestation.schema.json`.

Two boundaries keep this layer honest:

- **Attestations NEVER affect package validity.** Core verification remains deterministic;
  an attestation cannot make a package valid or invalid. Verifiers schema-check
  attestation parts exactly like any other part, and nothing more.
- **Findings route to humans, not to conclusions.** Every finding carries
  `requiresDisposition: true`; disposition is a human act recorded in `dispositions`,
  never produced by the attesting model.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. |
| `createdAt` | string | yes | RFC 3339 timestamp of the attestation. |
| `performedBy` | object | yes | Person, team, or service that ran the procedure. Same shape conventions as provenance `createdBy` (`type` in `human`/`agent`/`system`, `name`, optional `version`). |
| `model` | object | yes | The exact model that performed the analysis: `provider`, `name` (the provider's published model identifier, not a family or marketing name), and `version` are required — the same triple as provenance `models` entries — plus an optional `role` (e.g. `challenger`). |
| `scope` | object | yes | The declared review scope: `evidenceExamined` (non-empty array of `{ evidenceId, sha256 }`, pinning the review to exact evidence bytes), `claimsChallenged` (array of claim `id`s; MAY be empty for scope-wide procedures), and an optional `procedure` description. |
| `findings` | array | no | What the procedure found. Each finding: `id`, `severity` (`info`, `low`, `medium`, `high`, `critical`), `summary`, optional `relatedClaims` and `relatedEvidence` id arrays, and `requiresDisposition` (always `true`). |
| `limitations` | array | no | Strings describing what the procedure could not assess (evidence out of scope, questions requiring professional judgment, model constraints). |
| `dispositions` | array | no | Human decisions on findings. Each: `findingId`, `decision` (`accepted`, `rejected`, `remediated`, `acknowledged`), `by` (person, `name` required), `at` (RFC 3339), optional `note`. |
| `notes` | string | no | Free-form notes. |

## 3. Semantics

- **Model diversity.** The challenger SHOULD be a different model than the preparer models
  recorded in `provenance/provenance.json` `models` — challenging model-assisted work with
  the same model that produced it inherits that model's correlated blind spots (see
  [provenance.md](provenance.md) §3). The required exact-model triple on both sides is
  what makes this checkable.
- **Attestations are package content.** Attestation files under `attestations/` SHOULD be
  declared in the manifest `parts` array with a `sha256`, like any other part, so they fall
  inside the package integrity envelope and are tamper-evident once the manifest is signed.
- **Scope is a pin, not a pointer.** `evidenceExamined` entries record the SHA-256 of the
  evidence bytes as examined. Each `evidenceId` SHOULD resolve to an evidence object in the
  package, and the hash SHOULD match that evidence's declared `sha256`; a mismatch means
  the attestation reviewed different bytes than the package carries.
- **Findings are analytical output, not validation results.** A finding is a model's
  claim about the package, subject to the same skepticism as any model output. Verifiers
  MUST NOT change any validation status because of a finding or a disposition; profile
  validators and organizational policy MAY surface undispositioned findings for review.
- Attestations MAY be produced by the preparer, an auditor, a specialist, or any other
  reviewer, and a package MAY accumulate attestations from multiple parties over time.

## 4. Limitations

> An analysis attestation is not an audit opinion and does not replace professional
> judgment. It records that a model-assisted review procedure was performed and what it
> reported — scope declared, evidence pinned, model disclosed, limitations stated,
> disposition owned by a person. Reliance on any finding or disposition remains a matter
> of professional judgment and applicable standards.

## 5. Example

```json
{
  "id": "att-001",
  "createdAt": "2026-07-08T09:30:00Z",
  "performedBy": { "type": "human", "name": "demo-reviewer" },
  "model": {
    "provider": "demo-challenger",
    "name": "challenger-model-mini",
    "version": "2026-06-01",
    "role": "challenger"
  },
  "scope": {
    "evidenceExamined": [
      { "evidenceId": "ev_msa_excerpt", "sha256": "ac24…" }
    ],
    "claimsChallenged": ["c_step2_performance_obligations"],
    "procedure": "Read the MSA excerpt; challenge the claim that implementation services are distinct."
  },
  "findings": [
    {
      "id": "att-001-f1",
      "severity": "medium",
      "summary": "The excerpt does not state whether the customer can use the hosted service before data migration completes; a go-live dependency would weigh against distinctness.",
      "relatedClaims": ["c_step2_performance_obligations"],
      "relatedEvidence": ["ev_msa_excerpt"],
      "requiresDisposition": true
    }
  ],
  "limitations": [
    "Only the scoped evidence was examined; the full agreement was out of scope."
  ],
  "dispositions": [
    {
      "findingId": "att-001-f1",
      "decision": "acknowledged",
      "by": { "name": "demo-revenue-manager" },
      "at": "2026-07-08T16:00:00Z",
      "note": "Routed into the open distinctness judgment review."
    }
  ]
}
```
