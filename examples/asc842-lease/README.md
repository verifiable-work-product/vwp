# Example: ASC 842 lease (synthetic stub)

A minimal stub VWP package for a lease workpaper: a fictional 36-month office
lease with draft operating-lease classification and initial ROU asset / lease
liability measurement. It exists to show that the VWP shape extends beyond
ASC 606 and digital assets; it intentionally leaves the discount-rate support
in `reviewRequired`.

**All data is synthetic. This example is not accounting advice.**

```bash
cd cli && npm run build
node dist/index.js verify ../examples/asc842-lease/package
```

There is no `vwp-accounting-asc842` demo validator in the v0.1 CLI, so `verify`
runs core and finance-profile validators only.
