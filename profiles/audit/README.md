# VWP Audit Profile

> **Status:** Experimental draft (v0.1). All examples are synthetic.

The audit profile describes how VWP packages can be used to organize and transmit
evidence-backed work products for review in an audit context: prepared-by-client (PBC)
packages, workpapers, reconciliation support, and control test documentation.

## What VWP does in an audit context

- Packages evidence, extracted facts, claims, calculations, judgments, and approvals in
  a structured, machine-verifiable container.
- Lets deterministic validators check internal consistency: every claim has declared
  support, calculations recompute, hashes match declared content, review-required items
  are surfaced.
- Gives reviewers a clear map from conclusions back to the evidence and reasoning that
  supports them.

## What VWP does not do

VWP can package evidence for review, but it **does not replace auditor procedures or
auditor judgment**. Specifically:

- **VWP validation is not an audit opinion or an assurance engagement.** A passing
  validation means the package is internally consistent and structurally complete, not
  that its conclusions are correct, its evidence is reliable, or any standard has been
  met.
- The **sufficiency and appropriateness of evidence** remain matters of the auditor's
  professional judgment. VWP cannot determine whether packaged evidence is enough, or
  of the right kind, for any audit assertion.
- Auditors remain responsible for their own procedures, including evaluating the
  reliability of information produced by the entity, testing controls, and forming
  conclusions under applicable auditing standards.
- VWP validation is likewise not a legal opinion or an accounting opinion.

## Contents

- [`evidence-assertions.md`](./evidence-assertions.md) — how packaged evidence maps to
  audit assertions, and the limits of that mapping.

See also the finance profile ([`../finance/README.md`](../finance/README.md)) for the
assertion vocabulary used by finance claims.
