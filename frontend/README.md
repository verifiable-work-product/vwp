# vwp.ac frontend

Static marketing site + in-browser verifier for VWP, served by **Firebase
Hosting** at **vwp.ac**. A single self-contained `public/index.html` (no build
step): drag-and-drop a `.vwp` to verify it, or open one of the pre-verified
example packages.

## How it talks to the backend

In production the page calls **same-origin `/api/**`**, which Firebase Hosting
rewrites to the `vwp-api` Cloud Run service (see `firebase.json`). Because it's
same-origin, no CORS is involved in prod.

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

### Point vwp.ac at Hosting

In the Firebase console → Hosting → Add custom domain → `vwp.ac` (and
`www.vwp.ac`), then add the A/TXT records it gives you at the `vwp.ac`
registrar. The `/api` rewrite means the apex domain serves both the site and the
API under one origin; optionally also map `api.vwp.ac` straight to Cloud Run
(see `../backend/DEPLOY.md` §7) if you want a distinct API host.

## Design notes

The page is styled as a self-verifying workpaper: an "engineering paper" ledger
grid, monospace validator ids and hashes, and the format's own status
vocabulary as the visual system — verification green used *only* for
pass/valid, amber for `reviewRequired`, vermilion for `fail`. The hero specimen
verifies itself line-by-line on load; the report renders as a live audit
tick-list with a pass/invalid stamp.
