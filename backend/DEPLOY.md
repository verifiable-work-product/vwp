# Deploying the vwp-api backend to Cloud Run

The backend is a Go service (`backend/`) that wraps the `sdk/go` verifier. It
deploys to **Cloud Run** in the GCP project **`verifiable-work-product`**,
built by **Cloud Build** and pushed to **Artifact Registry**, triggered by a
**GitHub Actions** workflow authenticating via **Workload Identity Federation**
(no service-account JSON key ever leaves Google).

Everything below is run once by a project owner. Commands assume:

```bash
export PROJECT_ID=verifiable-work-product
export REGION=us-central1
export SERVICE=vwp-api
export AR_REPO=vwp
export REPO=verifiable-work-product/vwp        # GitHub owner/repo
gcloud config set project "$PROJECT_ID"
export PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
```

## 1. Enable APIs

```bash
gcloud services enable \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  artifactregistry.googleapis.com \
  iamcredentials.googleapis.com \
  --project "$PROJECT_ID"
```

## 2. Artifact Registry repo

```bash
gcloud artifacts repositories create "$AR_REPO" \
  --repository-format=docker \
  --location="$REGION" \
  --description="VWP container images" \
  --project "$PROJECT_ID"
```

## 3. Deployer service account

```bash
gcloud iam service-accounts create gh-deployer \
  --display-name="GitHub Actions deployer" \
  --project "$PROJECT_ID"

export DEPLOYER_SA="gh-deployer@${PROJECT_ID}.iam.gserviceaccount.com"

# Build, push, deploy, and act-as the runtime service account.
for role in \
  roles/run.admin \
  roles/cloudbuild.builds.editor \
  roles/artifactregistry.writer \
  roles/storage.admin \
  roles/iam.serviceAccountUser \
  roles/logging.viewer ; do
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${DEPLOYER_SA}" --role="$role" --quiet
done
```

> `storage.admin` is needed because `gcloud builds submit` stages the source in
> a GCS bucket. Scope it down later if you prefer a dedicated bucket.

## 4. Workload Identity Federation (keyless GitHub auth)

```bash
gcloud iam workload-identity-pools create github-pool \
  --location=global --display-name="GitHub pool" --project "$PROJECT_ID"

gcloud iam workload-identity-pools providers create-oidc github-provider \
  --location=global --workload-identity-pool=github-pool \
  --display-name="GitHub provider" \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
  --attribute-condition="assertion.repository=='${REPO}'" \
  --project "$PROJECT_ID"

# Let the specific repo impersonate the deployer SA.
gcloud iam service-accounts add-iam-policy-binding "$DEPLOYER_SA" \
  --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github-pool/attribute.repository/${REPO}" \
  --project "$PROJECT_ID"
```

## 5. GitHub Actions repository variables

In **GitHub → Settings → Secrets and variables → Actions → Variables**, add:

| Variable | Value |
|---|---|
| `GCP_PROJECT_ID` | `verifiable-work-product` |
| `GCP_PROJECT_NUMBER` | (output of `echo $PROJECT_NUMBER`) |
| `GCP_REGION` | `us-central1` |
| `CLOUD_RUN_SERVICE` | `vwp-api` |
| `AR_REPO` | `vwp` |
| `WIF_POOL` | `github-pool` |
| `WIF_PROVIDER` | `github-provider` |
| `DEPLOYER_SA` | `gh-deployer@verifiable-work-product.iam.gserviceaccount.com` |

No secrets are needed — WIF is keyless.

## 6. First deploy

Push to `main` (touching `backend/**` or `sdk/go/**`), or run the
**Deploy backend to Cloud Run** workflow manually via *Actions → Run workflow*.
The workflow builds `backend/Dockerfile` with the repo root as context (so the
`go.mod replace` to `sdk/go` resolves), pushes to Artifact Registry, and deploys.

To deploy once by hand instead:

```bash
IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}/${SERVICE}:manual"
gcloud builds submit . --config cloudbuild.yaml \
  --substitutions "_IMAGE=${IMAGE}" --region "$REGION" --project "$PROJECT_ID"
gcloud run deploy "$SERVICE" --image "$IMAGE" --region "$REGION" \
  --allow-unauthenticated --port 8080 --cpu 1 --memory 256Mi \
  --min-instances 0 --max-instances 4 \
  --set-env-vars "VWP_ALLOWED_ORIGINS=https://vwp.ac,https://www.vwp.ac" \
  --project "$PROJECT_ID"
```

Grab the service URL:

```bash
gcloud run services describe "$SERVICE" --region "$REGION" \
  --project "$PROJECT_ID" --format 'value(status.url)'
```

Smoke test:

```bash
URL=$(gcloud run services describe "$SERVICE" --region "$REGION" --project "$PROJECT_ID" --format 'value(status.url)')
curl -s "$URL/health"
curl -s "$URL/examples/asc606-revenue/verify" | head -c 400
```

## 7. Domain: api.vwp.ac (optional, recommended)

The frontend can call Cloud Run directly, but a stable custom subdomain is nicer.
Map `api.vwp.ac` to the service:

```bash
gcloud beta run domain-mappings create --service "$SERVICE" \
  --domain api.vwp.ac --region "$REGION" --project "$PROJECT_ID"
```

Then add the DNS records it prints at your `vwp.ac` registrar. (The apex
`vwp.ac` itself is served by Firebase Hosting — see the frontend setup.)

## API surface

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | liveness |
| POST | `/verify` | multipart field `package` (a `.vwp`) → full report JSON |
| POST | `/inspect` | multipart field `package` → manifest summary |
| GET | `/examples` | list bundled example packages |
| GET | `/examples/{id}/verify` | pre-run report for an example |
| GET | `/examples/{id}/download` | raw `.vwp` bytes for an example |

`VWP_ALLOWED_ORIGINS` (comma-separated) controls CORS; defaults include
`https://vwp.ac` and localhost dev ports.
