# Controls Profile: Control Evidence Types

> **Status:** Experimental stub (v0.1). All examples are synthetic. VWP validation is
> not an audit opinion, legal opinion, accounting opinion, or assurance engagement.

Common evidence types packaged in a control test package, each stored under
`evidence/` with source system, retrieval time, period covered, and hash.

## Reconciliation reports

The output of a reconciliation control: prepared reconciliations, supporting extracts
from both sides (e.g., subledger and bank/custodian), and difference schedules.
Typically support "control performed" and "evidence covers period" claims. Where the
reconciliation math is included as a `Calculation`, validators can recompute it.

## Signoffs

Preparer and reviewer signoff records: workflow approvals, ticket state changes,
e-signatures, or dated sign-off exports. Typically support "control performed" and
"reviewer authorized" claims; pairs naturally with `Approval` objects capturing who,
in what role, approved what, and when. Role authorization support (access listings,
delegation-of-authority extracts) can accompany signoffs.

## Exception logs

Logs of exceptions identified during control performance or testing, with disposition:
resolution evidence (corrected entries, follow-up tickets) or escalation records.
Support "exceptions resolved or escalated" claims; validators can check that every
logged exception links to a disposition.

## Other common types

- System configuration extracts (e.g., approval thresholds, access rules)
- Access review listings and change logs
- Meeting minutes or escalation communications (as referenced documents)

Evidence type labels are informative. Whether a given item is sufficient or
appropriate evidence for any control conclusion remains a matter of professional
judgment.
