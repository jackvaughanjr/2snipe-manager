# Phase 3 — GCP Integration ✓ COMPLETE (2026-04-16)

> **This file is archived and frozen.** Phase 3 is complete. Do not modify this
> file. Load it only if a current phase task requires context from Phase 3.

---

## Goal

Secrets go to GCP Secret Manager. Cloud Run Jobs and Cloud Scheduler are created
at install time. `enable`, `disable`, `run`, and `status` all work end to end.

---

## Required ✓ all complete

- [x] `internal/secrets/manager.go` — `SecretManager` interface + `GCPSecretManager`
      implementation (`Get`, `Set`, `Exists`, `ListByPrefix`); ADC first →
      `gcp.credentials_file` fallback
- [x] `internal/secrets/manager_test.go` — mock-based tests (4 tests)
- [x] `internal/scheduler/gcp.go` — `Scheduler` interface + `GCPScheduler`
      implementation; `JobSpec`, `Execution` types; `ConfigFieldToEnvVar`,
      `ImagePath`, `CloudRunJobName`, `SchedulerJobName` helpers
- [x] `internal/scheduler/gcp_test.go` — mock-based tests (5 tests)
- [x] `cmd/install.go` — `--secrets-backend gcp|local` flag; GCP path writes
      secrets, creates Cloud Run Job + optional Scheduler trigger
- [x] `cmd/uninstall.go` — deletes Cloud Run Job + Scheduler trigger when backend is GCP
- [x] `cmd/enable.go` — resumes Cloud Scheduler job; updates state
- [x] `cmd/disable.go` — pauses Cloud Scheduler job; updates state
- [x] `cmd/run.go` — triggers Cloud Run Job; proactive image build+push
      instructions on first run; prints execution name
- [x] `cmd/status.go` — table with live last-run data from executions API;
      lipgloss-styled in terminal, tabwriter when piped
- [x] `internal/wizard/wizard.go` — secrets backend select + schedule select
      steps added; `Result.Backend` now populated from wizard
- [x] `snipemgr.example.yaml` — `gcp.credentials_file` field added
- [x] `internal/registry/types.go` — `EnvVar string` added to `ConfigField`
- [x] `2snipe.schema.json` — `env_var` field added to config_schema items
- [x] `README.md` — GCP setup section, container image build+push instructions,
      all Phase 3 commands marked ✓, v0.3.0 version history entry

---

## Choices confirmed

- **GCP authentication order:** Option B — ADC first, `credentials_file` fallback.
  `snipemgr.example.yaml` was updated to document the new `gcp.credentials_file`
  field (confirmed: it was missing before Phase 3).
- **Docker image management:** Manual build+push for Phase 3. `snipemgr run`
  prints detailed step-by-step instructions (Dockerfile template, `docker build`,
  `docker push`) whenever an integration has never run successfully. The README
  also has a dedicated "Building container images for Cloud Run Jobs" section.

---

## Gotchas / deviations from plan

**1. `ListExecutions` is on `ExecutionsClient`, not `JobsClient`**
The Cloud Run Jobs v2 Go SDK has two separate clients: `run.JobsClient` (for job
CRUD and triggering) and `run.ExecutionsClient` (for listing/inspecting
executions). `GetLastExecution` requires `run.NewExecutionsClient` — calling
`g.runClient.ListExecutions` does not compile. `GCPScheduler` now holds both
clients and creates them both in `NewGCPScheduler`.

**2. `ListExecutionsRequest` has no `OrderBy` field**
The proto definition does not expose `OrderBy`. The API returns executions in
reverse chronological order by default (newest first), so `pageSize=1` reliably
gives the most recent execution without needing explicit ordering.

**3. `TaskTemplate.Retries` is a oneof, not a plain int field**
`runpb.TaskTemplate.MaxRetries` is accessed via the oneof wrapper:
```go
Retries: &runpb.TaskTemplate_MaxRetries{MaxRetries: 1},
```
A plain `MaxRetries: 1` does not compile.

**4. `Execution` has no `TerminalCondition` field**
Unlike `Job`, the `Execution` proto does not have a `TerminalCondition` field.
Status is determined instead from `SucceededCount`, `FailedCount`, and
`RunningCount` fields on the execution.

**5. `scheduler.DeleteJob` returns one value, not two**
The Cloud Scheduler client's `DeleteJob` returns only `error`, not
`(_, error)`. The blank identifier was removed.

**6. `env_var` added to `ConfigField` for explicit env var mapping**
The manifest spec did not include a way to override the default env var
derivation (`key.with.dots` → `KEY_WITH_DOTS`). An optional `env_var` field was
added to `ConfigField` (and to `2snipe.schema.json`) so integrations can specify
exact env var names when their viper bindings don't match the default rule.
This is backwards-compatible — existing manifests without `env_var` use the
default derivation. The well-known shared keys (`snipe_it.url` → `SNIPE_URL`,
`snipe_it.api_key` → `SNIPE_TOKEN`) are still hardcoded in `ConfigFieldToEnvVar`
so existing manifests work without updates.

**7. Secret IDs encode `/` as `--`**
GCP Secret Manager IDs don't support `/`. The logical names (e.g.
`snipe/snipe-url`, `github2snipe/token`) are encoded by replacing `/` with `--`
to produce valid IDs (`snipe--snipe-url`, `github2snipe--token`). The
`encodeSecretID`/`decodeSecretID` helpers handle this transparently.

**8. GCP backend `settings.yaml` masks secret values**
When `--secrets-backend gcp` is used, `settings.yaml` is still written (useful
for local testing) but secret fields are replaced with
`# managed by GCP Secret Manager` rather than the actual value. Non-secret
fields are written as normal.

**9. Shared secrets skip re-write if already present**
During install with GCP backend, shared secrets (`snipe/snipe-url`,
`snipe/snipe-token`) are checked with `Exists` before writing. If they already
exist (set by a previous integration install), they are silently skipped. This
preserves any existing shared secret rather than overwriting it.

**10. Cloud Run Job resource persists after failed image-not-found install**
GCP creates the Cloud Run Job resource in a failed/error state even when the
image is not found in Artifact Registry. The LRO completes with an error, but
the job resource exists. `ErrImageNotFound` is returned from `CreateJob` in this
case. `cmd/install.go` records `entry.CloudRunJob` even on `ErrImageNotFound` so
that subsequent `status`, `run`, and `uninstall` commands can find the resource.

**11. Re-install after image-not-found hits `AlreadyExists`**
Because the job resource was created in step 10, a second `install` call hits
`AlreadyExists` on `CreateJob`. `GCPScheduler.CreateJob` handles this silently
(proceeds to scheduler creation) so re-install after pushing the image works
without manual cleanup. The `AlreadyExists` handler also covers the re-configure
(`snipemgr config`) flow.

**12. `run` command shows clean error when job is in error state**
When `LastRunResult == ""` and `TriggerJob` fails (because the job is in GCP
error state due to missing image), `cmd/run.go` suppresses the raw gRPC
`FailedPrecondition` error and instead prints: "job is not ready to run — push
the container image first (see instructions above)." This avoids a confusing
gRPC error after image instructions have already been shown.

---

## Verification ✓ all passed (2026-04-16)

```bash
go build -o snipemgr . && echo "BUILD OK"   # ✓
go vet ./...                                 # ✓
go test ./... -v                             # 37 tests, 0 failures ✓
```

Live GCP verification (project `snipe-manager`, region `us-central1`):

```bash
snipemgr install github2snipe --no-interactive --secrets-backend gcp \
  --schedule "0 6 * * *" --snipe-url ... --snipe-token ... \
  --field github.token=... --field snipe_it.license_category_id=...
# ✓ Category 'Developer Tools & Hosting' ready (id=15)
# ✓ Installed github2snipe v1.6.0; Job + Trigger created

snipemgr status             # ✓ table with ENABLED / never / — row
snipemgr disable github2snipe  # ✓ Scheduler PAUSED
snipemgr enable github2snipe   # ✓ Scheduler ENABLED
snipemgr run github2snipe      # ✓ image instructions printed; clean error message
snipemgr uninstall github2snipe --no-interactive
# ✓ Cloud Run Job + Scheduler trigger deleted; 0 jobs in GCP; state cleared
```

---

## Go tests ✓ all passed (2026-04-15)

```
go test ./... -v
# 37 tests, 0 failures ✓
```

**`internal/secrets/manager_test.go`** (4 tests):
- `TestMockSecretManager_SetAndGet` ✓
- `TestMockSecretManager_Exists_Missing` ✓
- `TestMockSecretManager_Overwrite` ✓
- `TestMockSecretManager_ListByPrefix` ✓

**`internal/scheduler/gcp_test.go`** (5 tests):
- `TestMockScheduler_CreateAndDelete` ✓
- `TestBuildCloudRunJobSpec_EnvVars` ✓
- `TestBuildCloudRunJobSpec_ExplicitEnvVar` ✓
- `TestBuildSchedulerJobSpec_CronPassthrough` ✓
- `TestImagePath` ✓
