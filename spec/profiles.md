# VWP Profiles and Extensions

**Status:** v0.1 draft, experimental.

## 1. Overview

VWP Core is domain-neutral: it defines the container, the manifest, the object model
(evidence, facts, claims, judgments, calculations, validations, approvals, provenance),
and deterministic core verification. Everything domain-specific lives in **profiles** and
**extensions**.

- A **profile** adds domain vocabulary and domain-level validators. Example:
  `vwp-finance` defines the financial assertion vocabulary used in claims
  (`existence`, `occurrence`, `completeness`, `valuationAllocation`,
  `rightsAndObligations`, `presentationDisclosure`).
- An **extension** layers narrower vocabularies and validators on top of a profile.
  Examples: `vwp-accounting-asc606` (revenue-recognition–specific rules),
  `vwp-digital-assets-wallet-ownership-control` (wallet control evidence rules).

## 2. Declaration

Profiles and extensions are identified by string identifiers in the manifest:

```json
{
  "profile": "vwp-finance",
  "extension": "vwp-digital-assets-wallet-ownership-control"
}
```

- `profile` is required on every manifest. Packages with no domain vocabulary use
  `"vwp-core"`.
- `extension` is optional and, when present, MUST be an extension defined against the
  declared profile.
- Identifiers are lowercase, hyphen-separated strings. Identifiers under the `vwp-`
  prefix are reserved for specifications published in this project; third parties SHOULD
  namespace their identifiers (e.g. reverse-DNS or organization-prefixed).

## 3. Verification model

- **Core verification never requires a profile.** Every v0.1 verifier can structurally
  verify every v0.1 package — schemas, hashes, path safety, reference-graph integrity —
  regardless of the declared profile or extension.
- **Profile validators run additionally when declared.** A verifier that recognizes the
  declared profile runs its validators (e.g. assertion-vocabulary membership for
  `vwp-finance`) and reports results using the standard validation statuses
  (see [validations.md](validations.md)).
- A verifier that does not recognize a declared profile or extension MUST still perform
  core verification, MUST NOT fail the package for the unrecognized identifier alone, and
  SHOULD report profile validation as `notRun`.
- Profile and extension validators are bound by the same rules as core validators:
  deterministic, no network access, no model inference, no execution of package contents.

## 4. What a profile may define

A profile or extension specification SHOULD define:

1. Its string identifier and the profile it extends (extensions only).
2. Controlled vocabularies (e.g. claim `assertions` values, evidence `type` values).
3. Additional schema constraints on core objects (via schemas that extend, never relax,
   the core schemas).
4. Validators, each with an identifier and version, and the statuses/severities they emit.
5. Any conventions for `subject`, `standardRefs`, and object field usage in the domain.

Profiles MUST NOT change the container layout, the manifest's required fields, core
verification rules, or the semantics of validation statuses.

## 5. Status of named profiles

`vwp-finance`, `vwp-accounting-asc606`, and `vwp-digital-assets-wallet-ownership-control`
are referenced throughout v0.1 as motivating examples. Only the `vwp-finance` assertion
vocabulary is normatively defined in v0.1 (see [claims.md](claims.md)); full profile and
extension specifications are expected in later drafts.
