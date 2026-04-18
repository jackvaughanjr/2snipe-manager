# snipemgr — GCP Infrastructure

## Overview

`snipemgr` uses three GCP services to schedule and secure integrations:

| Service | Purpose |
|---------|---------|
| Cloud Run Jobs | Execute integration binaries in containers on demand or on schedule |
| Cloud Scheduler | Trigger Cloud Run Jobs on a cron schedule |
| Secret Manager | Store API keys and credentials securely |

All resources live in a single GCP project configured in `snipemgr.yaml`.

---

## Authentication

**Recommended: Application Default Credentials (ADC)**

```bash
gcloud auth application-default login
```

`snipemgr` uses ADC automatically via the GCP Go client libraries. No key file
needed for local use.

**Fallback: Service account key file**

If ADC is not available (e.g. CI, automated environments), set in `snipemgr.yaml`:

```yaml
gcp:
  credentials_file: "/path/to/sa-key.json"
```

Detection order: ADC first → credentials_file fallback → fatal error with setup instructions.

---

## Required GCP APIs

Enable these before first use (one-time, per project):

```bash
gcloud services enable \
  run.googleapis.com \
  cloudscheduler.googleapis.com \
  secretmanager.googleapis.com \
  artifactregistry.googleapis.com \
  --project YOUR_PROJECT_ID
```

`snipemgr doctor` (Phase 4+) will check these automatically.

---

## Required IAM permissions

The identity running `snipemgr` (user ADC or SA) needs:

| Role | Why |
|------|-----|
| `roles/run.admin` | Create/delete/trigger Cloud Run Jobs |
| `roles/cloudscheduler.admin` | Create/delete/enable/disable Scheduler jobs |
| `roles/secretmanager.admin` | Create and access secrets |
| `roles/iam.serviceAccountUser` | Attach SA to Cloud Run Jobs |

The Cloud Run Job's **runtime** service account (set in `snipemgr.yaml`
as `gcp.service_account`) needs:

| Role | Why |
|------|-----|
| `roles/secretmanager.secretAccessor` | Read secrets at job execution time |

---

## Cloud Run Jobs

**API version:** `run.googleapis.com/v2` (not v1 — Jobs are v2 only)

**Resource path:**
```
projects/{project}/locations/{region}/jobs/{name}
```

**Job spec created by `snipemgr install`:**

```json
{
  "name": "projects/{project}/locations/{region}/jobs/{integration_name}",
  "template": {
    "template": {
      "containers": [{
        "image": "{artifact_registry_path}/{name}:latest",
        "command": ["/app/{name}", "sync"],
        "env": [
          {"name": "SNIPE_URL",   "valueSource": {"secretKeyRef": {"secret": "snipe/snipe-url",   "version": "latest"}}},
          {"name": "SNIPE_TOKEN", "valueSource": {"secretKeyRef": {"secret": "snipe/snipe-token", "version": "latest"}}},
          {"name": "VENDOR_KEY",  "valueSource": {"secretKeyRef": {"secret": "{name}/vendor-key",  "version": "latest"}}}
        ],
        "resources": {"limits": {"cpu": "1", "memory": "512Mi"}}
      }],
      "serviceAccount": "{gcp.service_account}",
      "maxRetries": 1,
      "timeout": "300s"
    }
  }
}
```

Env var names are derived from the integration's viper env var bindings.
The `config_schema` field keys map to env var names following the pattern
already established in each integration (e.g. `snipe_it.url` → `SNIPE_URL`).

**Note on container images:**
Each integration must have a Docker image pushed to Artifact Registry before the
Cloud Run Job can execute. Phase 3 requires this to be done manually. Document
the build+push steps clearly in the `run` command error output when the image is
missing. Phase 4+ can automate this.

**Suggested Artifact Registry path:**
```
{region}-docker.pkg.dev/{project}/snipe-integrations/{name}:latest
```

---

## Cloud Scheduler

**API version:** `cloudscheduler.googleapis.com/v1`

**Resource path:**
```
projects/{project}/locations/{region}/jobs/{name}-trigger
```

**Job spec:**

```json
{
  "name": "projects/{project}/locations/{region}/jobs/{name}-trigger",
  "schedule": "0 6 * * *",
  "timeZone": "UTC",
  "httpTarget": {
    "uri": "https://run.googleapis.com/v2/projects/{project}/locations/{region}/jobs/{name}:run",
    "httpMethod": "POST",
    "oauthToken": {
      "serviceAccountEmail": "{gcp.service_account}"
    }
  }
}
```

**Note:** Use the Cloud Run Jobs v2 URI format (`run.googleapis.com/v2/projects/…`),
not the older v1 namespaces format (`{region}-run.googleapis.com/apis/…`). The v1
path is for Cloud Run *services*, not Jobs.

**Enable/disable** uses the `pause`/`resume` methods on the Scheduler job resource.

---

## Secret Manager

**Naming convention:**

```
snipe/snipe-url            → env var SNIPE_URL (shared)
snipe/snipe-token          → env var SNIPE_TOKEN (shared)
{name}/session-key         → env var CLAUDE_SESSION_KEY (integration-specific)
{name}/api-token           → env var {VENDOR}_TOKEN (integration-specific)
```

The env var name for each secret is determined by the integration's existing
viper env var bindings. The manager derives the correct env var name from the
manifest `config_schema` key using the following convention:

```
config key:  claude.session_key
env var:     CLAUDE_SESSION_KEY   (upper-snake of full dot path minus vendor prefix)
```

Where ambiguous, check the integration's `cmd/root.go` viper bindings directly.

**Secret version:** Always use `latest` in Cloud Run Job env var refs. Rotating a
secret just requires adding a new version in Secret Manager — no job update needed.

---

## Last-run status

To populate `snipemgr status`, fetch the most recent execution for each job:

```
GET https://run.googleapis.com/v2/projects/{project}/locations/{region}/jobs/{name}/executions
  ?pageSize=1
```

**Note:** `ListExecutionsRequest` has no `orderBy` field in the proto. The API
returns executions in reverse-chronological order by default (newest first), so
`pageSize=1` reliably returns the most recent execution without explicit sorting.

Map execution status to display values:
- `EXECUTION_SUCCEEDED` → `✓ success`
- `EXECUTION_FAILED` → `✗ failed`
- `EXECUTION_RUNNING` → `⟳ running`
- `EXECUTION_CANCELLED` → `⊘ cancelled`

---

## Cost estimate

For daily runs of up to 20 integrations:
- **Cloud Run Jobs:** Free tier covers ~180,000 vCPU-seconds/month. Each sync
  job runs in seconds. Cost: effectively $0.
- **Cloud Scheduler:** Free for first 3 jobs; $0.10/job/month after that.
  20 integrations ≈ $1.70/month.
- **Secret Manager:** $0.06/active secret version/month. ~50 secrets ≈ $3/month.

Total estimated cost: **under $5/month** for a full installation.
