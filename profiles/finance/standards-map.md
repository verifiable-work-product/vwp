# Finance Profile: Standards Mapping (`standardRefs`)

> **Status:** Experimental draft (v0.1). All examples are synthetic.

Claims and work products in a VWP package may reference professional standards using
the `standardRefs` field. A standard reference is a short, human-recognizable string
identifying the standard or regulatory framework a claim relates to, for example:

```json
{
  "id": "claim-rev-001",
  "statement": "Revenue for contract C-1042 is recognized over time as the service is delivered.",
  "standardRefs": ["ASC 606"],
  "assertions": ["occurrence", "valuationAllocation"]
}
```

Common examples:

| `standardRefs` value | Typical subject matter |
| --- | --- |
| `ASC 606` | Revenue from contracts with customers |
| `ASC 842` | Leases |
| `ASC 820` | Fair value measurement |
| `SOX 404` | Internal control over financial reporting |

## Mapping is informative, not normative

Standard references are **informative metadata**. They help reviewers, tools, and
extensions route a claim to the right context (for example, the `vwp-accounting-asc606`
extension validates claims tagged `ASC 606`). They do **not** mean:

- that the claim complies with the referenced standard,
- that VWP or any validator has assessed compliance, or
- that the reference itself is complete or authoritative.

VWP does not define or interpret accounting standards. Determining whether a
conclusion complies with a standard remains a matter of professional judgment. VWP
validation is not an audit opinion, legal opinion, accounting opinion, or assurance
engagement.

## Conventions

- Use the widely recognized short citation (e.g., `ASC 606`, `IFRS 15`, `SOX 404`).
- Prefer one reference per distinct standard rather than compound strings.
- Extensions may define stricter conventions (e.g., paragraph-level citations); those
  conventions are scoped to the extension that defines them.
