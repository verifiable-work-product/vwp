# VWP Approvals

**Status:** v0.1 draft, experimental.

## 1. Overview

Approval objects record human review actions over a package or parts of it: who reviewed
what, in what role, and with what outcome. Approvals make the review trail portable with
the work product itself. Approval objects live under `approvals/` and validate against
`https://vwp.dev/schemas/v0.1/approval.schema.json`.

Approvals capture review state; they do not imply audit signoff, professional
certification, or any regulatory attestation.

## 2. Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the package. |
| `approver` | string / object | yes | Identity of the reviewer (name, email, or structured identity). |
| `role` | string | yes | Capacity in which the action was taken (e.g. `preparer`, `reviewer`, `controller`). |
| `action` | string | yes | One of `approved`, `rejected`, `requestedChanges`, `acknowledged`. |
| `timestamp` | string | yes | RFC 3339 timestamp of the action. |
| `scope` | string / object | no | What was reviewed: whole package, specific claim/judgment `id`s, or part paths. Absent scope means the whole package. |
| `comments` | string | no | Free-form reviewer comments. |

## 3. Actions

| Action | Meaning |
|---|---|
| `approved` | The reviewer accepts the scoped content in the stated role. |
| `rejected` | The reviewer does not accept the scoped content. |
| `requestedChanges` | The reviewer requires changes before acceptance. |
| `acknowledged` | The reviewer has seen the scoped content without expressing acceptance. |

## 4. Semantics

- Approvals in v0.1 are unauthenticated records unless covered by a package signature
  (see [signatures.md](signatures.md)). Consumers SHOULD treat unsigned approvals as
  informational, not as proof the named person acted.
- Judgments with a `requiredApproverRole` SHOULD be covered by an `approved` action from
  a matching `role` (see [judgments.md](judgments.md)). Profile validators MAY enforce
  role-coverage rules; core verification does not.
- Multiple approvals over the same scope are permitted; later actions do not delete
  earlier ones. Consumers SHOULD interpret the record chronologically (a `rejected`
  followed by `approved` after changes is a normal sequence).
- A package edited after an approval was recorded no longer reflects what was approved.
  Producers SHOULD record approvals against a package state and re-approve after material
  changes; signed manifests make this tamper-evident.

## 5. Example

```json
{
  "id": "app-002",
  "approver": { "name": "R. Chen", "email": "r.chen@example.com" },
  "role": "controller",
  "action": "approved",
  "timestamp": "2026-07-02T21:14:33Z",
  "scope": { "judgmentIds": ["jdg-001"], "claimIds": ["claim-001"] },
  "comments": "Basis consistent with policy DA-4.2; threshold confirmed."
}
```
