# snipemgr — Order of Operations

This document is the canonical build plan. Work through phases in order.
Do not start a phase until all **Required** items are done AND the **Verification**
section passes cleanly.

At the start of each Claude Code session, load this file to orient on where we are.
Update the checkboxes as work is completed.

---

## Prerequisites by phase

Build entirely on your local machine. GCP is not needed until Phase 3.

| Phase | What you need before starting |
|-------|-------------------------------|
| 0 | Go 1.22+ installed locally. That's it. |
| 1 | Phase 0 complete. A GitHub PAT is optional but recommended — without one, GitHub limits unauthenticated API calls to 60/hr, which is enough for development but tight. Create one at `github.com/settings/tokens/new` with `public_repo` scope (or `repo` if your `*2snipe` repos are private). |
| 2 | Phase 1 complete. At least one `*2snipe` integration must have a `2snipe.json` manifest committed to its repo root so there's something real to install. Add `2snipe.json` to `claude2snipe` first — it's the most complete integration and a good test case. |
| 3 | Phase 2 complete. A GCP project with billing enabled. Complete the GCP setup checklist below before writing any Phase 3 code. |
| 4 | Phase 3 complete and at least one integration running successfully on its Cloud Run schedule. |

### GCP setup checklist (complete before starting Phase 3, not before)

- [ ] GCP project created with billing enabled
- [ ] Enable required APIs:
  ```bash
  gcloud services enable \
    run.googleapis.com \
    cloudscheduler.googleapis.com \
    secretmanager.googleapis.com \
    artifactregistry.googleapis.com \
    --project YOUR_PROJECT_ID
  ```
- [ ] Create a service account for Cloud Run Jobs:
  ```bash
  gcloud iam service-accounts create snipemgr-runner \
    --display-name="snipemgr Cloud Run Runner" \
    --project YOUR_PROJECT_ID
  ```
- [ ] Grant the runner SA access to Secret Manager:
  ```bash
  gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="serviceAccount:snipemgr-runner@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
  ```
- [ ] Authenticate your local machine with ADC:
  ```bash
  gcloud auth application-default login
  ```
- [ ] Set `gcp.project`, `gcp.region`, and `gcp.service_account` in `snipemgr.yaml`
- [ ] Verify the pre-flight checks pass:
  ```bash
  gcloud auth application-default print-access-token > /dev/null && echo "ADC OK"
  gcloud run jobs list --region=us-central1 --project=YOUR_PROJECT_ID > /dev/null && echo "CLOUD RUN OK"
  gcloud secrets list --project=YOUR_PROJECT_ID > /dev/null && echo "SECRET MANAGER OK"
  ```

**Docker / Artifact Registry note:** Cloud Run Jobs require a container image.
Phase 3 does not automate building or pushing images — that is a manual step.
Before `snipemgr run` will work for any integration, you must build and push its
Docker image to Artifact Registry yourself. The `run` command will detect a missing
image and print exact build+push instructions rather than failing silently.
Image automation is a Phase 4+ enhancement tracked in `docs/features-backlog.md`.

---

## Phase 0 — Repo bootstrap

**Goal:** Empty but runnable Go binary with correct module path, CLI skeleton,
and config loading. No GCP or GitHub calls yet.

### Required

- [ ] Create GitHub repo `jackvaughanjr/2snipe-manager` (private)
- [ ] `go.mod` — module `github.com/jackvaughanjr/2snipe-manager`, go 1.22
- [ ] `main.go` — version embedding pattern (same as integrations)
- [ ] `cmd/root.go` — cobra root, viper init, `PersistentPreRunE` logging init,
      `SilenceUsage`/`SilenceErrors`, `fatal()` helper, `--no-interactive` flag,
      `--config` flag pointing to `snipemgr.yaml`
- [ ] `.gitignore` — excludes `snipemgr.yaml`, binaries, `.DS_Store`
- [ ] `snipemgr.example.yaml` — fully commented config template
- [ ] `README.md` — placeholder with description and "coming soon" for commands

### Choices at this phase

- **Config file name:** `snipemgr.yaml` (decided — distinct from integration
  `settings.yaml` to avoid confusion when both live on the same machine).
  Confirm before coding.

### Verification

```bash
# Binary builds cleanly
go build -o snipemgr . && echo "BUILD OK"

# Help output shows root command and global flags
./snipemgr --help

# Version flag works
./snipemgr --version
# Expected: "snipemgr version dev" (or injected version)

# Unknown flag produces usage (SilenceUsage is off before PersistentPreRunE runs)
./snipemgr --bad-flag 2>&1 | grep -i "unknown flag"

# Verbose flag is accepted without error
./snipemgr --verbose --help

# Config flag is accepted
./snipemgr --config /tmp/nonexistent.yaml --help
# Expected: no panic — missing config file is handled gracefully at command run time

# Go vet clean
go vet ./...
# Expected: no output, exit 0

# No warnings from go build
go build ./... 2>&1
# Expected: no output
```

### Go tests

```bash
# Scaffold the test file even if minimal — confirm test harness works
go test ./... -v
# Expected: all packages compile; "no test files" is acceptable at this phase
```

---

## Phase 1 — Registry + `list` command

**Goal:** `snipemgr list` works end to end — hits GitHub, validates manifests,
renders a table of available integrations.

### Required

- [ ] `internal/registry/types.go` — `Manifest`, `ConfigField`, `Integration`,
      `Source` structs
- [ ] `2snipe.schema.json` — JSON Schema for manifest validation
- [ ] `internal/registry/client.go`
  - GitHub repo search by owner + topic/name filter
  - Fetch `2snipe.json` from each repo's main branch
  - Validate manifest (struct validation — see Choices)
  - Return `[]Integration` with `Installed` cross-referenced against state
  - Session-level in-memory cache of manifest fetches
  - GitHub unauthenticated rate limit warning (print if no token configured)
- [ ] `internal/state/store.go` — read `~/.snipemgr/state.json`; create empty
      state if file doesn't exist (write support comes in Phase 2)
- [ ] `cmd/list.go` — renders table using `lipgloss`; respects `--no-interactive`
      (plain text / no color when piped or flag set)
- [ ] `go vet ./...` clean

### Optional (defer to Phase 3+)

- [ ] `--filter <tag>` flag
- [ ] `--json` output flag

### Choices at this phase

- **Manifest validation approach:**
  Option A: `github.com/xeipuuv/gojsonschema` — full JSON Schema validation.
  Option B: Unmarshal into struct + check required fields manually (no extra dep).
  **Recommended: Option B.** Confirm before coding.

- **GitHub search filter:**
  Option A: Topic `2snipe` only.
  Option B: Name pattern `*2snipe` only.
  Option C: Name pattern + manifest presence gate (recommended).
  Confirm before coding.

### Verification

```bash
# Build still clean after new packages
go build -o snipemgr . && echo "BUILD OK"
go vet ./...

# list command appears in help
./snipemgr --help | grep list

# list command has its own help
./snipemgr list --help

# Dry registry run against real GitHub (requires network + snipemgr.yaml configured)
# Set registry.sources[0].owner = jackvaughanjr in snipemgr.yaml first
./snipemgr list -v
# Expected:
#   - Table renders without panic
#   - At minimum: claude2snipe appears if its 2snipe.json exists
#   - Repos without 2snipe.json are absent from the list
#   - If no GitHub token: rate limit warning is printed

# Piped output is plain text (no ANSI escape codes)
./snipemgr list --no-interactive | cat
# Expected: readable plain text table, no color codes

# Missing snipemgr.yaml produces a clear error (not a panic)
./snipemgr --config /tmp/does-not-exist.yaml list 2>&1
# Expected: "Error: ..." message, exit 1

# State file is created if missing
rm -f ~/.snipemgr/state.json
./snipemgr list
# Expected: no crash; state.json created with empty integrations map
ls ~/.snipemgr/state.json && echo "STATE FILE CREATED"
```

### Go tests

```bash
go test ./internal/registry/... -v
```

Tests to write in `internal/registry/client_test.go`:

- `TestValidateManifest_Valid` — a well-formed manifest struct passes validation
- `TestValidateManifest_MissingName` — manifest missing `name` returns error
- `TestValidateManifest_MissingVersion` — manifest missing `version` returns error
- `TestValidateManifest_MissingConfigSchema` — empty `config_schema` returns error
- `TestValidateManifest_BadAssetPattern` — pattern missing `{os}` or `{arch}` returns error
- `TestValidateManifest_BadVersion` — non-SemVer version string returns error

Tests to write in `internal/state/store_test.go`:

- `TestReadState_Missing` — reading a nonexistent path returns empty state, no error
- `TestReadState_Empty` — reading `{}` returns empty state, no error
- `TestReadState_Valid` — reading a valid JSON fixture returns correct struct

```bash
go test ./internal/... -v
# Expected: all tests pass; no skipped tests
```

---

## Phase 2 — `install` command (local mode)

**Goal:** `snipemgr install <n>` downloads the binary, runs the config wizard,
and writes a local `settings.yaml`. No GCP yet — secrets backend is local file only.

### Required

- [ ] `internal/installer/installer.go`
  - Resolve binary download URL from manifest `releases.asset_pattern` + GitHub
    Releases API
  - Download to `~/.snipemgr/bin/{name}` (create dir if needed)
  - Make downloaded binary executable (`chmod +x`)
  - Write `settings.yaml` skeleton to `~/.snipemgr/config/{name}/settings.yaml`
- [ ] `internal/wizard/wizard.go`
  - First-time setup detection: if `snipemgr.yaml` missing required fields, run
    setup wizard before integration install
  - Config form driven entirely by manifest `config_schema`
  - Shared config reuse prompt (if `shared_config` prefix already has values)
  - TTY detection: fall back to flag input when `--no-interactive` or piped
  - Password masking for `secret: true` fields
- [ ] `internal/state/store.go` — add write support (atomic write via tmp+rename)
- [ ] `cmd/install.go`
  - Accept integration name as positional arg
  - Flag-based equivalents for all wizard fields (for `--no-interactive` use)
  - Graceful handling of already-installed: prompt to reconfigure or abort
- [ ] `cmd/config.go` — re-run wizard for an installed integration
- [ ] `cmd/uninstall.go` — remove binary, config dir, state entry (local only)
- [ ] `go vet ./...` clean

### Optional (defer)

- [ ] SHA-256 checksum verification of downloaded binary
- [ ] Rollback on partial install failure

### Choices at this phase

- **Binary install location:**
  Option A: `~/.snipemgr/bin/` (default, configurable)
  Option B: `/usr/local/bin/` (system-wide, requires sudo)
  Option C: Configurable in `snipemgr.yaml`, default `~/.snipemgr/bin/`
  **Recommended: Option C.** Confirm before coding.

- **Config storage location:**
  Option A: `~/.snipemgr/config/{name}/settings.yaml`
  Option B: `~/.snipemgr/config/settings.{name}.yaml`
  **Recommended: Option A.** Confirm before coding.

### Verification

```bash
# Build clean
go build -o snipemgr . && echo "BUILD OK"
go vet ./...

# install command appears in help
./snipemgr install --help
# Expected: usage, positional arg description, all wizard field flags listed

# Non-interactive install of a real integration (use claude2snipe if 2snipe.json exists)
./snipemgr install claude2snipe \
  --no-interactive \
  --claude-session-key "sk-ant-test-fake" \
  --snipe-url "https://snipe.example.com" \
  --snipe-token "fake-token" \
  --schedule manual
# Expected: binary downloaded, settings.yaml written, state updated, no panic

# Binary exists and is executable
ls -la ~/.snipemgr/bin/claude2snipe
# Expected: file exists, has +x permission

# settings.yaml was written with correct values
cat ~/.snipemgr/config/claude2snipe/settings.yaml
# Expected: YAML with the values passed above; no empty required fields

# State file updated
cat ~/.snipemgr/state.json | python3 -m json.tool
# Expected: claude2snipe entry present with correct version and installed_at

# list now shows claude2snipe as installed
./snipemgr list | grep -i "installed"
# Expected: claude2snipe shows "● installed"

# Re-install prompts for reconfiguration (interactive) or errors cleanly (non-interactive)
./snipemgr install claude2snipe --no-interactive 2>&1
# Expected: clear message "already installed" — not a panic

# config command re-runs wizard
./snipemgr config claude2snipe --help
# Expected: same flags as install

# uninstall removes binary, config, and state entry
./snipemgr uninstall claude2snipe --no-interactive
ls ~/.snipemgr/bin/claude2snipe 2>&1 | grep "No such file"
ls ~/.snipemgr/config/claude2snipe 2>&1 | grep "No such file"
cat ~/.snipemgr/state.json | python3 -m json.tool
# Expected: claude2snipe absent from state
```

### Go tests

```bash
go test ./internal/... -v
```

New tests to write:

`internal/installer/installer_test.go`:
- `TestResolveAssetURL_Darwin_ARM64` — pattern `foo_{os}_{arch}` resolves correctly
- `TestResolveAssetURL_Linux_AMD64` — same for linux/amd64
- `TestResolveAssetURL_Windows` — appends `.exe`
- `TestWriteSettingsYAML` — given a manifest with config_schema, output YAML
  has all keys present with correct placeholder values

`internal/state/store_test.go`:
- `TestWriteState_Atomic` — write succeeds; file is valid JSON after write
- `TestWriteState_RoundTrip` — write then read returns identical struct
- `TestWriteState_ConcurrentSafe` — two writes don't corrupt the file

`internal/wizard/wizard_test.go`:
- `TestBuildFlagDefaults` — given a manifest, `--no-interactive` flag set produces
  correct settings map with all required fields populated
- `TestBuildFlagDefaults_MissingRequired` — missing required field returns error
  with the field's label in the message

```bash
go test ./... -v
# Expected: all tests pass
```

---

## Phase 3 — GCP integration

**Goal:** Secrets go to Secret Manager. Cloud Run Jobs and Cloud Scheduler are
created at install time. `enable`, `disable`, `run`, and `status` work.

### Required

- [ ] Complete the GCP setup checklist in the Prerequisites section above
- [ ] `internal/secrets/manager.go` — GCP Secret Manager `Get`, `Set`, `Exists`,
      `ListByPrefix`; uses Application Default Credentials with service account
      key file fallback
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
- [ ] `cmd/run.go` — trigger Cloud Run Job; optionally tail logs
- [ ] `cmd/status.go` — table with last-run data from executions API
- [ ] Update `internal/wizard/wizard.go` — schedule step + GCP backend choice
- [ ] `go vet ./...` clean
- [ ] Document manual Docker image build+push step in README and in `run` command
      error output when image is missing

### Optional (defer)

- [ ] `snipemgr run --tail` — stream Cloud Logging in real time
- [ ] GCS-backed state file

### Choices at this phase

- **GCP authentication order:**
  Option A: ADC only.
  Option B: ADC first, service account key file fallback.
  **Recommended: Option B.** Confirm before coding.

- **Docker image management:**
  Phase 3 documents the manual build+push step. `install` should detect a missing
  image and print clear instructions rather than failing silently. Automation is
  Phase 4+.

### Verification

```bash
# GCP credentials are available
gcloud auth application-default print-access-token > /dev/null && echo "ADC OK"

# Secret Manager: set and retrieve a test secret
./snipemgr install claude2snipe \
  --no-interactive \
  --secrets-backend gcp \
  --claude-session-key "sk-ant-test-fake" \
  --snipe-url "https://snipe.example.com" \
  --snipe-token "fake-token" \
  --schedule "0 6 * * *"

# Verify secret was written to Secret Manager
gcloud secrets list --filter="name:claude2snipe" --project=YOUR_PROJECT
# Expected: claude2snipe/session-key (or equivalent) present

# Verify Cloud Run Job was created
gcloud run jobs list --region=us-central1 --project=YOUR_PROJECT | grep claude2snipe
# Expected: job present

# Verify Cloud Scheduler trigger was created
gcloud scheduler jobs list --location=us-central1 --project=YOUR_PROJECT | grep claude2snipe
# Expected: trigger present with correct schedule

# status command renders without panic
./snipemgr status
# Expected: table with claude2snipe row; last run shows "never" or actual execution

# disable command pauses the scheduler job
./snipemgr disable claude2snipe
gcloud scheduler jobs describe claude2snipe-trigger \
  --location=us-central1 --project=YOUR_PROJECT \
  --format="value(state)"
# Expected: PAUSED

# enable command resumes it
./snipemgr enable claude2snipe
gcloud scheduler jobs describe claude2snipe-trigger \
  --location=us-central1 --project=YOUR_PROJECT \
  --format="value(state)"
# Expected: ENABLED

# run command triggers the job (image must exist in Artifact Registry)
./snipemgr run claude2snipe
# Expected: execution triggered; execution ID printed; exit 0
# If image missing: clear error message with build+push instructions, not a panic

# uninstall removes GCP resources
./snipemgr uninstall claude2snipe --no-interactive
gcloud run jobs list --region=us-central1 --project=YOUR_PROJECT | grep -c claude2snipe
# Expected: 0
gcloud scheduler jobs list --location=us-central1 --project=YOUR_PROJECT | grep -c claude2snipe
# Expected: 0
```

### Go tests

GCP API calls are not unit-testable without mocks. Write interface-based mocks.

`internal/secrets/manager_test.go`:
- Define `SecretManager` interface (`Get`, `Set`, `Exists`, `ListByPrefix`)
- `TestMockSecretManager_SetAndGet` — mock set then get returns same value
- `TestMockSecretManager_Exists_Missing` — missing key returns false, no error

`internal/scheduler/gcp_test.go`:
- Define `Scheduler` interface (`CreateJob`, `DeleteJob`, `EnableJob`, `DisableJob`,
  `TriggerJob`, `GetLastExecution`)
- `TestMockScheduler_CreateAndDelete` — create then delete is idempotent
- `TestBuildCloudRunJobSpec` — given a manifest + config, output job JSON has
  correct env var names and secret refs
- `TestBuildSchedulerJobSpec` — given a cron string, output scheduler JSON is valid

```bash
go test ./... -v
# Expected: all tests pass; GCP integration tests are mock-only (no live calls)
```

---

## Phase 4 — `upgrade` command + release polish

**Goal:** Upgrade detection works. Binary is releasable.

### Required

- [ ] `cmd/upgrade.go` — compare state versions against manifest versions;
      prompt per outdated integration; download + replace binary
- [ ] `snipemgr list` and `snipemgr status` show `↑ update` indicator when
      manifest version > installed version
- [ ] Consistent error handling across all commands (audit `fatal()` usage)
- [ ] README complete: install curl one-liners, first-time setup, all commands
      with examples, how to add `2snipe.json` to a new integration
- [ ] `.github/workflows/release.yml` — cross-platform binaries on `v*` tag
- [ ] `go vet ./...` clean

### Optional (defer)

- [ ] `upgrade --all` non-interactive
- [ ] Changelog display from GitHub Release notes

### Verification

```bash
# Build clean
go build -o snipemgr . && echo "BUILD OK"
go vet ./...

# upgrade help
./snipemgr upgrade --help

# Simulate upgrade available: manually set an older version in state.json,
# then run upgrade --no-interactive and confirm it offers the update
# (This requires a real integration with a published release to compare against)

# list shows update indicator when installed version < manifest version
# Manually set claude2snipe version to "0.0.1" in state.json
cat ~/.snipemgr/state.json | python3 -c "
import json,sys
s=json.load(sys.stdin)
s['integrations']['claude2snipe']['version']='0.0.1'
print(json.dumps(s,indent=2))
" > /tmp/state_old.json && mv /tmp/state_old.json ~/.snipemgr/state.json
./snipemgr list | grep -i "update\|↑"
# Expected: claude2snipe row shows update available indicator

# Release workflow file exists and is valid YAML
cat .github/workflows/release.yml | python3 -c "import sys,yaml; yaml.safe_load(sys.stdin)" && echo "YAML OK"

# Cross-platform build (smoke test release matrix locally)
GOOS=linux GOARCH=amd64 go build -o /tmp/snipemgr-linux-amd64 . && echo "LINUX AMD64 OK"
GOOS=darwin GOARCH=arm64 go build -o /tmp/snipemgr-darwin-arm64 . && echo "DARWIN ARM64 OK"
GOOS=windows GOARCH=amd64 go build -o /tmp/snipemgr-windows-amd64.exe . && echo "WINDOWS AMD64 OK"
```

### Go tests

```bash
go test ./... -v -count=1
# Expected: all tests pass; no flaky tests

# Race detector — run once before tagging a release
go test -race ./...
# Expected: no data race warnings
```

`cmd/upgrade_test.go`:
- `TestUpgradeNeeded_OlderInstalled` — installed `0.0.1`, manifest `1.2.0` → needs upgrade
- `TestUpgradeNeeded_SameVersion` — same version → no upgrade needed
- `TestUpgradeNeeded_NewerInstalled` — installed version ahead of manifest → no upgrade,
  log a warning

---

## Ongoing — all phases

The following checks should pass at the end of every phase and after every
significant change. Make these habitual.

```bash
# Vet and build
go vet ./...
go build ./...

# Full test suite
go test ./... -v -count=1

# No uncommitted changes to committed files after a session
git diff --exit-code
# Expected: exit 0 (settings.yaml and binaries are gitignored; all code is clean)

# settings.yaml and snipemgr.yaml are gitignored
git check-ignore snipemgr.yaml && echo "GITIGNORED OK"
```
