# vwp.ac frontend

Static marketing site + browser-based VWP reader and verifier, served by
**Firebase Hosting** at **vwp.ac**. A single self-contained
`public/index.html` (no build step): drag-and-drop a `.vwp` to verify and read
it, or open one of the pre-verified example packages.

The report renders package Markdown as a document and offers two downloads:

- **Download readable copy** creates a self-contained, script-free HTML
  snapshot that opens offline in any browser. It records when it was exported,
  the package ID, and the SHA-256 of the source `.vwp`. The snapshot does not
  re-run verification; the original package remains authoritative.
- **Download .vwp** downloads the exact source package. Share it alongside the
  readable copy when the recipient may want to re-run verification.

## How it talks to the backend

In production the page calls **`https://api.vwp.ac`**. The API allowlists the
`vwp.ac` origins for CORS. The endpoint can be overridden with `?api=<url>` or
`window.VWP_API`.

Uploaded packages are transmitted to the configured API for transient
verification. The API does not persist request bodies. A future fully local
verifier would be needed before the site could accurately claim that package
contents never leave the browser.

For local dev the page falls back to `http://localhost:8080`, and you can point
it anywhere with `?api=<url>`:

```bash
# terminal 1 — backend
cd ../backend && VWP_ALLOWED_ORIGINS="http://localhost:5000" PORT=8080 go run .

# terminal 2 — static site
cd public && python3 -m http.server 5000
# open http://localhost:5000/index.html   (dev fallback hits :8080)
```

## Deploy

Prereqs: the Cloud Run service `vwp-api` must exist (see `../backend/DEPLOY.md`),
and you must be authenticated: `firebase login`.

```bash
cd frontend
firebase deploy --only hosting --project verifiable-work-product
```

## Test

The frontend has no build step or third-party runtime dependencies. Its Node
test syntax-checks the inline application and exercises the safe Markdown
reader against tables, links, and hostile HTML:

```bash
node test.mjs
```

### Point vwp.ac at Hosting

In the Firebase console → Hosting → Add custom domain → `vwp.ac` (and
`www.vwp.ac`), then add the A/TXT records it gives you at the `vwp.ac`
registrar. Map `api.vwp.ac` to Cloud Run as described in
[`../backend/DEPLOY.md`](../backend/DEPLOY.md) §7.

## Design notes

The page is styled as a self-verifying workpaper: an "engineering paper" ledger
grid, monospace validator ids and hashes, and the format's own status
vocabulary as the visual system — verification green used *only* for
pass/valid, amber for `reviewRequired`, vermilion for `fail`. The hero specimen
verifies itself line-by-line on load; the report renders as a live audit
tick-list with a pass/invalid stamp.
