# VWP conceptual model

```mermaid
flowchart LR
    E[Evidence<br/>source records] --> F[Facts<br/>extracted observations]
    F --> C[Claims<br/>assertions and conclusions]
    E --> C
    J[Judgments<br/>professional judgment points] --> C
    K[Calculations<br/>deterministic computations] --> C
    C --> V[Validations<br/>deterministic checks]
    J --> V
    K --> V
    V --> X[Conclusion<br/>workproduct.json]
    X --> R[Renderings<br/>memo / workpaper<br/>derived views only]
```

The package is the source of truth; renderings are derived views. Validators
check structure, integrity, links, and tie-outs — they do not judge whether
the professional conclusion is correct.

```mermaid
flowchart TD
    subgraph vwp [example.vwp - ZIP archive]
        M[manifest.json<br/>identity, profile, parts, SHA-256]
        W[workproduct.json]
        CL[claims/claims.json]
        P[provenance/provenance.json]
        EV[evidence/]
        FA[facts/]
        JU[judgments/]
        CA[calculations/]
        VA[validations/]
        AP[approvals/]
        RE[renderings/]
        SI[signatures/ - experimental]
    end
    M -->|declares + hashes| W
    M -->|declares + hashes| CL
    M -->|declares + hashes| P
```
