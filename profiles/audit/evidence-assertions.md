# Audit Profile: Mapping Packaged Evidence to Audit Assertions

> **Status:** Experimental draft (v0.1). All examples are synthetic.

VWP packages carry claims that may be tagged with assertions from the finance profile
vocabulary (`existence`, `occurrence`, `completeness`, `valuationAllocation`,
`rightsAndObligations`, `presentationDisclosure`). This document explains how that
tagging can help an auditor navigate a package, and where the mapping stops.

## How the mapping works

1. A preparer packages evidence items (statements, extracts, confirmations, on-chain
   snapshots) under `evidence/`.
2. Facts extracted from evidence reference their sources via `evidenceRefs`.
3. Claims reference supporting facts, evidence, calculations, and judgments, and carry
   the assertions they are intended to address.
4. A reviewer or tool can therefore answer, mechanically: *"For assertion X on claim Y,
   what evidence does the preparer point to?"*

Example (synthetic):

```json
{
  "id": "claim-cash-001",
  "statement": "Cash at Example Bank account ...1234 was 2,500,000.00 USD at 2025-12-31.",
  "assertions": ["existence", "valuationAllocation"],
  "supportingEvidence": ["ev-bank-stmt-2025-12", "ev-confirmation-2026-01"],
  "supportingCalculations": ["calc-recon-cash-001"]
}
```

## Where the mapping stops

The mapping is a **navigational aid**, not an evaluation of the evidence:

- **Sufficiency and appropriateness of evidence remain the auditor's judgment.** VWP
  can show that a claim points at evidence; it cannot determine whether that evidence
  is enough, relevant, or reliable for the assertion.
- Tagging a claim with an assertion is the preparer's characterization, not a finding.
- Validators check structure (every claim has support, hashes match, calculations
  recompute), not the persuasiveness of the underlying documents.
- Auditors must still perform their own procedures under applicable standards,
  including assessing the reliability of information produced by the entity.

VWP validation is not an audit opinion, legal opinion, accounting opinion, or
assurance engagement.
