# Phase 3 — GCP Integration

> **Load this file when working on Phase 3.**
> Complete the GCP setup checklist in `docs/order-of-operations.md` before writing
> any code. Load `docs/gcp-infra.md` for API references, resource path patterns,
> and IAM requirements. Do not load Phase 4 unless a task explicitly requires it.

---

## Goal

Secrets go to Secret Manager. Cloud Run Jobs and Cloud Scheduler are created at
install time. `enable`, `disable`, `run`, and `status` work end to end.

---

## Required

- [ ] Complete the GCP setup checklist in `docs/order-of-operations.md`
- [ ] `internal/secrets/manager.go` — GCP Secret Manager `Get`, `Set`, `Exists`,
      `ListByPrefix`; ADC with service account key file fallback
- [ ] `internal/scheduler/gcp.go`
  - Create Cloud Run Job
  - Create Cloud Scheduler trigger
  - Delete job + trigger
  - Enable / disable scheduler job
  - Get last execution status (executions list API)
  - Trigger job immediately
- [ ] Update `cmd/install.go` — GCP backend option, schedule step, calls scheduler
- [ ] Update `cmd/uninstall.go` — delete GCP resources when backend is GCP
- [ ] `cmd/enable.go`
- [ ] `cmd/disable.go`
- [ ] `cmd/run.go` — trigger Cloud Run Job; tail logs optional
- [ ] `cmd/status.go` — table with last-run data from executions API
- [ ] Update `internal/wizard/wizard.go` — schedule step + GCP backend choice
- [ ] `go vet ./...` clean
- [ ] Document manual Docker image build+push step in README and in `run` command
      error output when image is missing

---

## Optional (defer)

- [ ] `snipemgr run --tail` — stream Cloud Logging in real time
- [ ] GCS-backed state file

---

## Choices — confirm before coding

- **GCP authentication order:**
  Option A: ADC only.
  Option B: ADC first, service account key file fallback.
  **Recommended: Option B.**

- **Docker image management:** Document manual build+push for Phase 3. The `install`
  command should detect a missing image and print clear instructions rather than
  failing silently. Automation is Phase 4+.

---

## Verification

```bash
gcloud auth application-default print-access-token > /dev/null && echo "ADC OK"

# Install with GCP backend
./snipemgr install <integration-name> \
  --no-interactive \
  --secrets-backend gcp \
  --snipe-url "https://snipe.example.com" \
  --snipe-token "fake-token" \
  --schedule "0 6 * * *"

# Verify Secret Manager
gcloud secrets list --filter="name:<integration-name>" --project=YOUR_PROJECT
# Expected: secrets present

# Verify Cloud Run Job
gcloud run jobs list --region=us-central1 --project=YOUR_PROJECT | grep <integration-name>
# Expected: job present

# Verify Cloud Scheduler trigger
gcloud scheduler jobs list --location=us-central1 --project=YOUR_PROJECT | grep <integration-name>
# Expected: trigger present with correct schedule

./snipemgr status
# Expected: table renders, last run shows "never" or actual execution

./snipemgr disable <integration-name>
gcloud scheduler jobs describe <integration-name>-trigger \
  --location=us-central1 --project=YOUR_PROJECT --format="value(state)"
# Expected: PAUSED

./snipemgr enable <integration-name>
gcloud scheduler jobs describe <integration-name>-trigger \
  --location=us-central1 --project=YOUR_PROJECT --format="value(state)"
# Expected: ENABLED

./snipemgr run <integration-name>
# Expected: execution triggered, ID printed, exit 0
# If image missing: clear instructions printed, not a panic

./snipemgr uninstall <integration-name> --no-interactive
gcloud run jobs list --region=us-central1 --project=YOUR_PROJECT | grep -c <integration-name>
gcloud scheduler jobs list --location=us-central1 --project=YOUR_PROJECT | grep -c <integration-name>
# Expected: 0 and 0
```

---

## Go tests

GCP calls require interface-based mocks — no live API calls in unit tests.

**`internal/secrets/manager_test.go`:**
- Define `SecretManager` interface (`Get`, `Set`, `Exists`, `ListByPrefix`)
- `TestMockSecretManager_SetAndGet`
- `TestMockSecretManager_Exists_Missing`

**`internal/scheduler/gcp_test.go`:**
- Define `Scheduler` interface (`CreateJob`, `DeleteJob`, `EnableJob`, `DisableJob`,
  `TriggerJob`, `GetLastExecution`)
- `TestMockScheduler_CreateAndDelete`
- `TestBuildCloudRunJobSpec` — correct env var names and secret refs
- `TestBuildSchedulerJobSpec` — valid cron output

```bash
go test ./... -v
# Expected: all tests pass; GCP tests are mock-only
```
