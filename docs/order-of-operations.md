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
| 2 | Phase 1 complete. All five initial integrations (`1password2snipe`, `github2snipe`, `googleworkspace2snipe`, `okta2snipe`, `slack2snipe`) have valid `2snipe.json` manifests and the `2snipe` topic set. Use any of them as test integrations. |
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

## Phase 0 — Repo bootstrap ✓ COMPLETE (2026-04-14)

**Goal:** Empty but runnable Go binary with correct module path, CLI skeleton,
and config loading. No GCP or GitHub calls yet.

### Required

- [x] Create GitHub repo `jackvaughanjr/2snipe-manager` (private)
- [x] `go.mod` — module `github.com/jackvaughanjr/2snipe-manager`, go 1.23 (see Gotchas)
- [x] `main.go` — version embedding pattern (same as integrations)
- [x] `cmd/root.go` — cobra root, viper init, `PersistentPreRunE` logging init,
      `SilenceUsage`/`SilenceErrors`, `fatal()` helper, `--no-interactive` flag,
      `--config` flag pointing to `snipemgr.yaml`
- [x] `.gitignore` — excludes `snipemgr.yaml`, binaries, `.DS_Store`
- [x] `snipemgr.example.yaml` — fully commented config template
- [x] `README.md` — badge row, description, build phases, installation, version history

### Choices at this phase

- **Config file name:** `snipemgr.yaml` ✓ confirmed — distinct from integration
  `settings.yaml` to avoid confusion when both live on the same machine.

### Gotchas / deviations from plan

**1. Go version bumped to 1.23 (planned: 1.22)**
`viper v1.21.0` requires `go 1.23.0`. `go get` automatically updated `go.mod` from
`1.22` to `1.23.0`. All code is fully compatible; the version in `go.mod` is the
minimum required, not a constraint on the build machine.

**2. Root command needs a `Run` field to display flags in `--help`**
Cobra suppresses the "Flags:" section when the root command has no `Run`/`RunE`
and no subcommands. Without `Run`, `./snipemgr --help` only printed the `Long`
description. Fixed by adding:
```go
Run: func(cmd *cobra.Command, args []string) {
    _ = cmd.Help()
},
```
This is also correct UX: running `snipemgr` with no subcommand prints help rather
than silently exiting. Once subcommands are added in Phase 1, this `Run` continues
to make sense.

**3. Repo init — `2snipe-manager/` was an untracked subdirectory of `2snipe-config`**
The local `~/Documents/GitHub/` directory is the `2snipe-config` git repo, whose
`.gitignore` uses a `*` catch-all that already excluded `2snipe-manager/`. A fresh
`git init` was run inside `2snipe-manager/` to create an independent repo, then
connected to the new `jackvaughanjr/2snipe-manager` remote. The `claude-code-kickoff.md`
session file was left untracked and is not committed to the repo.

### Verification ✓ all passed (2026-04-14)

```bash
# Binary builds cleanly
go build -o snipemgr . && echo "BUILD OK"
# Result: BUILD OK ✓

# Help output shows root command and global flags
./snipemgr --help
# Result: shows Usage, all 7 global flags ✓

# Version flag works
./snipemgr --version
# Result: "snipemgr version dev" ✓

# Unknown flag produces usage (SilenceUsage is off before PersistentPreRunE runs)
./snipemgr --bad-flag 2>&1 | grep -i "unknown flag"
# Result: "Error: unknown flag: --bad-flag" ✓

# Verbose flag is accepted without error
./snipemgr --verbose --help
# Result: no error, help displayed ✓

# Config flag is accepted
./snipemgr --config /tmp/nonexistent.yaml --help
# Result: no panic — missing config file handled gracefully ✓

# Go vet clean
go vet ./...
# Result: no output, exit 0 ✓

# No warnings from go build
go build ./... 2>&1
# Result: no output ✓
```

### Go tests ✓ passed (2026-04-14)

```bash
go test ./... -v
# Result: all packages compile; "no test files" is acceptable at this phase ✓
```

---

## Phase 1 — Registry + `list` command ✓ COMPLETE (2026-04-14)

> **Load `docs/phases/phase-1-complete.md` for full details, gotchas, and
> verification results.**

**Goal:** `snipemgr list` works end to end — hits GitHub, validates manifests,
renders a table of available integrations.

### Required ✓ all complete

- [x] `internal/registry/types.go` — `Manifest`, `ConfigField`, `Commands`,
      `Releases`, `Integration`, `Source` structs
- [x] `2snipe.schema.json` — JSON Schema (draft-07) for editor tooling
- [x] `internal/registry/client.go` — GitHub Search + Contents API, struct
      validation, in-memory cache, rate limit warning
- [x] `internal/state/store.go` — read state; create empty file if missing
- [x] `cmd/list.go` — lipgloss table in terminal; tabwriter in piped/`--no-interactive`
- [x] `go vet ./...` clean

### Choices confirmed

- **Manifest validation:** struct field checks via `ValidateManifest(Manifest) error`.
  No external JSON Schema library. `2snipe.schema.json` exists for editor tooling only.
- **GitHub search filter:** topic `2snipe` + manifest presence gate.
- **GitHub API:** stdlib `net/http` + Contents API (not `google/go-github` — see gotchas).

### Gotchas (summary — see phase-1-complete.md for details)

- GitHub Contents API used (not raw.githubusercontent.com) — works with private repos
- `google/go-github` not added; stdlib `net/http` used throughout
- `v` prefix (`v1.0.0`) rejected by SemVer validation; bare `1.0.0` required
- Three integration `.gitignore` files had `*.json` blocking `2snipe.json` — fixed
- `1password2snipe` `asset_pattern` used underscores (typo) — fixed to dashes
- All five initial integration manifests committed: `1password2snipe`, `github2snipe`,
  `googleworkspace2snipe`, `okta2snipe`, `slack2snipe`

### Verification ✓ all passed (2026-04-14)

```bash
go build -o snipemgr . && echo "BUILD OK"   # ✓
go vet ./...                                 # ✓
go test ./internal/... -v                    # 12 tests, 0 failures ✓
./snipemgr list --no-interactive 2>/dev/null # 5 integrations discovered ✓
```

---

## Phase 2 — `install` command (local mode) + category management ✓ COMPLETE (2026-04-15)

> **Load `docs/phases/phase-2-complete.md` for full details, gotchas, and
> verification results.**

**Goal:** `snipemgr install <n>` downloads the binary, runs the config wizard,
ensures the integration's Snipe-IT category exists, and writes a local
`settings.yaml`. No GCP yet — secrets backend is local file only.

### Required ✓ all complete

- [x] `internal/installer/installer.go`
- [x] `internal/snipeit/categories.go`
- [x] `internal/wizard/wizard.go`
- [x] `internal/state/store.go` — write support (atomic via tmp+rename)
- [x] `cmd/install.go`
- [x] `cmd/categories.go` — `categories list` and `categories seed` (with `--dry-run`)
- [x] `cmd/config.go`
- [x] `cmd/uninstall.go`
- [x] `go vet ./...` clean

### Choices confirmed

- **Binary install location:** Option C — configurable in `snipemgr.yaml` via
  `install.bin_dir`, default `~/.snipemgr/bin/`
- **Config storage location:** Option A — `~/.snipemgr/config/{name}/settings.yaml`

### Verification ✓ all passed (2026-04-15)

See `docs/phases/phase-2-complete.md` for the full verification log.

### Verification

```bash
# Build clean
go build -o snipemgr . && echo "BUILD OK"
go vet ./...

# install command appears in help
./snipemgr install --help
# Expected: usage, positional arg description, all wizard field flags listed

# Non-interactive install of a real integration (use whichever integration has a 2snipe.json)
# Replace <integration-name> and flags with values from that integration's config_schema
./snipemgr install <integration-name> \
  --no-interactive \
  --snipe-url "https://snipe.example.com" \
  --snipe-token "fake-token" \
  --schedule manual
  # add --<field-key> flags for each required config_schema field
# Expected: binary downloaded, settings.yaml written, state updated, no panic

# Binary exists and is executable
ls -la ~/.snipemgr/bin/<integration-name>
# Expected: file exists, has +x permission

# settings.yaml was written with correct values
cat ~/.snipemgr/config/<integration-name>/settings.yaml
# Expected: YAML with the values passed above; no empty required fields

# State file updated
cat ~/.snipemgr/state.json | python3 -m json.tool
# Expected: <integration-name> entry present with correct version and installed_at

# list now shows the integration as installed
./snipemgr list | grep -i "installed"
# Expected: <integration-name> shows "● installed"

# Re-install prompts for reconfiguration (interactive) or errors cleanly (non-interactive)
./snipemgr install <integration-name> --no-interactive 2>&1
# Expected: clear message "already installed" — not a panic

# config command re-runs wizard
./snipemgr config <integration-name> --help
# Expected: same flags as install

# uninstall removes binary, config, and state entry
./snipemgr uninstall <integration-name> --no-interactive
ls ~/.snipemgr/bin/<integration-name> 2>&1 | grep "No such file"
ls ~/.snipemgr/config/<integration-name> 2>&1 | grep "No such file"
cat ~/.snipemgr/state.json | python3 -m json.tool
# Expected: <integration-name> absent from state

# categories subcommand appears in help
./snipemgr categories --help
./snipemgr categories list --help
./snipemgr categories seed --help

# categories list (requires valid Snipe-IT credentials in snipemgr.yaml)
./snipemgr categories list
# Expected: table of existing Snipe-IT license categories; no panic on empty instance

# categories seed dry-run shows what would be created
./snipemgr categories seed --dry-run
# Expected: lists all DefaultCategories; marks each as [exists] or [would create]; no API writes

# categories seed creates missing categories (idempotent)
./snipemgr categories seed
./snipemgr categories seed   # run twice — second run should produce no changes
# Expected: first run creates any missing categories; second run skips all silently

# install auto-ensures category when manifest.category is set
# (requires an integration with a category field in its 2snipe.json)
./snipemgr install <integration-name> --no-interactive ...flags...
# Expected: "✓ Category '<category>' ready" in output; category exists in Snipe-IT after install
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

`internal/snipeit/categories_test.go`:
- `TestDefaultCategories_Count` — `DefaultCategories` has exactly 10 entries; none empty
- `TestEnsureCategory_EmptyName` — empty name returns 0 and no error (warning only)
- `TestCreateCategory_EnvelopeUnwrap` — POST response with `payload` wrapper returns
  correct ID
- `TestSeedDefaults_Idempotent` — calling `SeedDefaults` twice against a mock that
  returns existing categories on second call produces no duplicate POST calls

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
# Replace <integration-name> and flags with values from that integration's config_schema
./snipemgr install <integration-name> \
  --no-interactive \
  --secrets-backend gcp \
  --snipe-url "https://snipe.example.com" \
  --snipe-token "fake-token" \
  --schedule "0 6 * * *"
  # add --<field-key> flags for each required config_schema field

# Verify secret was written to Secret Manager
gcloud secrets list --filter="name:<integration-name>" --project=YOUR_PROJECT
# Expected: <integration-name>/<field> secrets present

# Verify Cloud Run Job was created
gcloud run jobs list --region=us-central1 --project=YOUR_PROJECT | grep <integration-name>
# Expected: job present

# Verify Cloud Scheduler trigger was created
gcloud scheduler jobs list --location=us-central1 --project=YOUR_PROJECT | grep <integration-name>
# Expected: trigger present with correct schedule

# status command renders without panic
./snipemgr status
# Expected: table with <integration-name> row; last run shows "never" or actual execution

# disable command pauses the scheduler job
./snipemgr disable <integration-name>
gcloud scheduler jobs describe <integration-name>-trigger \
  --location=us-central1 --project=YOUR_PROJECT \
  --format="value(state)"
# Expected: PAUSED

# enable command resumes it
./snipemgr enable <integration-name>
gcloud scheduler jobs describe <integration-name>-trigger \
  --location=us-central1 --project=YOUR_PROJECT \
  --format="value(state)"
# Expected: ENABLED

# run command triggers the job (image must exist in Artifact Registry)
./snipemgr run <integration-name>
# Expected: execution triggered; execution ID printed; exit 0
# If image missing: clear error message with build+push instructions, not a panic

# uninstall removes GCP resources
./snipemgr uninstall <integration-name> --no-interactive
gcloud run jobs list --region=us-central1 --project=YOUR_PROJECT | grep -c <integration-name>
# Expected: 0
gcloud scheduler jobs list --location=us-central1 --project=YOUR_PROJECT | grep -c <integration-name>
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
# Manually set an installed integration's version to "0.0.1" in state.json
# Replace <integration-name> with whichever integration is installed
cat ~/.snipemgr/state.json | python3 -c "
import json,sys
s=json.load(sys.stdin)
s['integrations']['<integration-name>']['version']='0.0.1'
print(json.dumps(s,indent=2))
" > /tmp/state_old.json && mv /tmp/state_old.json ~/.snipemgr/state.json
./snipemgr list | grep -i "update\|↑"
# Expected: <integration-name> row shows update available indicator

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
