# VWP Controls Profile

> **Status:** Experimental draft (v0.1). All examples are synthetic. VWP validation is
> not an audit opinion, legal opinion, accounting opinion, or assurance engagement.

The controls profile applies VWP to internal control testing and evidence — for
example, controls documented and tested under SOX 404. A control test package renders
as a control test workpaper: the control's performance, the evidence it produced, the
review that occurred, and the disposition of exceptions, all in verifiable structure.

## Typical claims

- **Control performed** — the control activity was performed for each instance in the
  test period (e.g., the monthly reconciliation was prepared each month).
- **Reviewer authorized** — the person who performed or reviewed the control held the
  required role or authorization at the time.
- **Exceptions resolved or escalated** — every exception identified during performance
  or testing has a documented resolution or escalation.
- **Evidence covers period** — the packaged evidence spans the full period under test,
  with no uncovered gaps (or documented rationale for gaps).

## Typical objects

- **Evidence** — reconciliation reports, signoff records, exception logs, system access
  extracts, tickets. See [`control-evidence.md`](./control-evidence.md).
- **Judgments** — exception severity assessments, compensating control reasoning.
- **Approvals** — reviewer and approver actions with roles and timestamps.
- **Calculations/validators** — period coverage checks, population completeness,
  performer-vs-reviewer segregation checks.

## Illustrative validators

- Every control instance in the period has performance evidence.
- Every performance record has a signoff by an authorized role, and performer and
  reviewer differ where the control requires segregation.
- Every exception log entry links to a resolution or escalation record.
- Evidence period coverage equals the declared test period.

## Limits

A passing package demonstrates that the documentation is structurally complete and
internally consistent. It does not conclude on control design or operating
effectiveness — those conclusions belong to management's assessment and the auditor's
own testing and judgment.
