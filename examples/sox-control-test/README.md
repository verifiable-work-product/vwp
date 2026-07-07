# Example: SOX control test (synthetic stub)

A minimal stub VWP package showing that VWP extends to internal control
evidence: a monthly review of the digital asset subledger to ERP
reconciliation. Claims cover the control being performed, reviewer
authorization, exception resolution, and evidence period coverage.

**All data is synthetic. VWP validation is not an audit opinion or assurance
engagement.**

```bash
cd cli && npm run build
node dist/index.js verify ../examples/sox-control-test/package
```

The `vwp-controls-evidence` extension has no demo validators in the v0.1 CLI,
so `verify` runs core and finance-profile validators only.
