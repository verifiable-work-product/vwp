# VWP Judgments

**Status:** v0.1 draft, experimental.

## 1. Overview

Judgments record the points in a work product where professional discretion was required —
where the answer does not follow mechanically from facts and calculations. Making
judgments explicit lets reviewers see what was decided, on what basis, what alternatives
were considered, and who needs to approve. Judgment objects live under `judgments/` and
validate against `https://vwp.dev/schemas/v0.1/judgment.schema.json`.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. Referenced by claims. |
| `topic` | string | yes | Short label for the judgment area. |
| `question` | string | yes | The question requiring discretion. |
| `proposedConclusion` | string | yes | The conclusion proposed by the preparer or tool. |
| `basis` | string | no | Reasoning and criteria supporting the proposed conclusion. |
| `alternatives` | array | no | Alternatives considered, each with a description and (optionally) why it was not selected. |
| `riskLevel` | string | no | e.g. `low`, `medium`, `high`. Signals required scrutiny. |
| `requiredApproverRole` | string | no | Role that must approve this judgment (e.g. `controller`, `partner`). |
| `status` | string | no | e.g. `proposed`, `approved`, `rejected`, `superseded`. |

## 3. Semantics

- A judgment is a *decision point*, not an observation (facts) or a mechanical result
  (calculations). If a conclusion could be recomputed deterministically, it belongs in a
  calculation, not a judgment.
- Claims that depend on discretion SHOULD reference the relevant judgment `id`s so the
  dependency is visible in the claim graph.
- When `requiredApproverRole` is set, an approval (see [approvals.md](approvals.md)) by
  a matching `role` with `action: "approved"` and a `scope` covering the judgment SHOULD
  exist before the judgment `status` is `approved`. Profile validators MAY enforce this;
  core verification does not.
- Judgments produced with AI assistance SHOULD say so in `basis` or via provenance, and
  SHOULD carry `status: "proposed"` until a human review action is recorded.

## 4. Example

```json
{
  "id": "jdg-001",
  "topic": "Wallet control threshold",
  "question": "Is a single signed message plus one observed outbound transfer sufficient to conclude control?",
  "proposedConclusion": "Yes, for wallets below the materiality threshold in policy DA-4.2.",
  "basis": "Policy DA-4.2 accepts signed-message plus transfer evidence below threshold; wallet balance is below threshold per fact-003.",
  "alternatives": [
    { "option": "Require a second signed message at period end", "rejectedBecause": "Not required by policy below threshold; adds cost without changing conclusion." }
  ],
  "riskLevel": "medium",
  "requiredApproverRole": "controller",
  "status": "approved"
}
```
