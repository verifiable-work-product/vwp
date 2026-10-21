# Extension Sketch: `vwp-digital-assets-stablecoin`

> **Status:** Experimental draft (v0.1). This is not accounting advice. All example
> data is synthetic. VWP validation is not an audit opinion, legal opinion, accounting
> opinion, or assurance engagement.

How a stablecoin classification memo maps onto VWP objects: for every stablecoin
holding, the classification (cash, cash equivalent, financial asset, intangible
asset, …) and the redemption right it rests on.

## Shape of a stablecoin package

- **Facts** — each holding (token, issuer, custody location, units) and the issuer's
  redemption terms.
- **Judgments** — the classification of each token/issuer pair, addressing the
  maturity test for anything cash-like (the AICPA Digital Assets Practice Aid: a
  stablecoin with no maturity is generally not a cash equivalent).
- **Evidence** — redemption terms, attestation or reserve reports.
- **Claims** — classification, presentation and disclosure.

## Data contract

`facts/stablecoin-holdings.json` — the **holdings register**, one entry per holding with
its classification, judgment reference and redemption evidence
([`stablecoin-holdings.schema.json`](../../schemas/v0.1/extensions/digital-assets-stablecoin/stablecoin-holdings.schema.json)).

## Validators (Go SDK)

| Id | Semantics |
|---|---|
| `stablecoin.holdings.registerPresent` | Fails when `facts/stablecoin-holdings.json` is missing or contains no holdings. |
| `stablecoin.holdings.schemaValid` | The register validates against the extension schema. |
| `stablecoin.classification.judgmentLinked` | Every holding links a resolvable classification judgment. |
| `stablecoin.redemption.evidence` | Cash-like classifications rest on evidenced redemption rights. |
| `stablecoin.classification.consistent` | One token from one issuer is classified the same way everywhere it is held. |
| `stablecoin.classification.cashEquivalentChallenged` | Reports `reviewRequired` for any holding classified as cash or a cash equivalent despite having no maturity; a reviewer must disposition it. |

This extension books nothing and requires no accounting schedule.

See [`examples/stablecoin-classification`](../../examples/stablecoin-classification).
