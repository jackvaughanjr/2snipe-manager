# snipemgr — Order of Operations

> **Applies to:** `snipemgr` contributors — historical build plan and phase log; not updated after v1.1.0

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

**1. Go version bumped to 1.23, then 1.25 (planned: 1.22)**
`viper v1.21.0` requires `go 1.23.0`. `go get` automatically updated `go.mod` from
`1.22` to `1.23.0`. Subsequently bumped to `1.25.0` when GCP client library
dependencies required it. All code is fully compatible; the version in `go.mod` is the
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

# Unknown flag produces usage (flag parsing happens before PersistentPreRunE, so SilenceUsage is still off)
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

```bash
go build -o snipemgr . && echo "BUILD OK"   # ✓
go vet ./...                                 # ✓
go test ./... -v                             # 28 tests, 0 failures ✓
```

See `docs/phases/phase-2-complete.md` for the full verification log and test breakdown.

---

## Phase 3 — GCP integration ✓ COMPLETE (2026-04-16)

> **Load `docs/phases/phase-3-complete.md` for full details, gotchas, and
> verification results.**

**Goal:** Secrets go to Secret Manager. Cloud Run Jobs and Cloud Scheduler are
created at install time. `enable`, `disable`, `run`, and `status` work.

### Required ✓ all complete

- [x] Complete the GCP setup checklist in the Prerequisites section above
- [x] `internal/secrets/manager.go` — GCP Secret Manager `Get`, `Set`, `Exists`,
      `ListByPrefix`; ADC with service account key file fallback
- [x] `internal/scheduler/gcp.go`
  - Create Cloud Run Job
  - Create Cloud Scheduler trigger
  - Delete job + trigger
  - Enable / disable scheduler job
  - Get last execution status (executions list API)
  - Trigger job immediately
- [x] Update `cmd/install.go` — `--secrets-backend gcp|local`, schedule step, calls scheduler
- [x] Update `cmd/uninstall.go` — delete GCP resources when backend is GCP
- [x] `cmd/enable.go`
- [x] `cmd/disable.go`
- [x] `cmd/run.go` — trigger Cloud Run Job; proactive image build+push instructions
- [x] `cmd/status.go` — table with live last-run data from executions API
- [x] Update `internal/wizard/wizard.go` — secrets backend + schedule steps
- [x] `go vet ./...` clean
- [x] Document manual Docker image build+push step in README and in `run` command
      output when integration has never run successfully

### Optional (defer)

- [ ] `snipemgr run --tail` — stream Cloud Logging in real time
- [ ] GCS-backed state file

### Choices confirmed

- **GCP authentication order:** Option B — ADC first, `gcp.credentials_file` fallback.
  `snipemgr.example.yaml` documents the `credentials_file` field.
- **Docker image management:** Manual build+push; `install` and `run` detect a
  missing image and print detailed step-by-step instructions.

### Verification ✓ all passed (2026-04-16)

```bash
go build -o snipemgr . && echo "BUILD OK"   # ✓
go vet ./...                                 # ✓
go test ./... -v                             # 37 tests, 0 failures ✓
```

See `docs/phases/phase-3-complete.md` for the full live GCP verification log,
all 12 gotchas, and test breakdown.

---

## Phase 4 — `upgrade` command + release polish ✓ COMPLETE (2026-04-17)

> **Load `docs/phases/phase-4-complete.md` for full details, gotchas, and
> verification results.**

**Goal:** Upgrade detection works. Binary is releasable.

### Required ✓ all complete

- [x] `cmd/upgrade.go` — compare state versions against manifest versions;
      prompt per outdated integration; download + replace binary only (settings.yaml untouched)
- [x] `snipemgr list` and `snipemgr status` show `↑ update` when manifest > installed;
      `status` adds a VERSION column
- [x] Error handling audit — all `cmd/*.go` bare returns verified correct
- [x] README: all commands ✓, v1.0.0 version history, Phase 4 row ✓
- [x] `.github/workflows/release.yml` — cross-platform binaries on `v*` tag
- [x] `go vet ./...` clean

### Optional

- [x] `upgrade --all` non-interactive — implemented
- [ ] Changelog display from GitHub Release notes — deferred to post-v1.0.0

### Verification ✓ all passed (2026-04-17)

```bash
go build -o snipemgr . && echo "BUILD OK"   # ✓
go vet ./...                                 # ✓
go test ./... -v -count=1                   # 35 tests, 0 failures ✓
go test -race ./...                         # 0 data races ✓
```

See `docs/phases/phase-4-complete.md` for the full verification log, all 6 gotchas,
and named test breakdown.

---

## Post-Phase 4 — v1.1.0 additions (2026-04-17)

These features were added after Phase 4 completed, before the v1.1.0 tag was pushed.

### `snipemgr init` — first-time setup wizard

- [x] `cmd/init.go` — three-step huh wizard: additional GitHub owner/token
      (`jackvaughanjr` is always the default first source), Snipe-IT creds
      (skippable), GCP project/region/SA (skippable); writes `snipemgr.yaml`
- [x] Re-run guard: interactive confirm ("This is intended to be run once…") or
      `--force` flag for scripted environments; scope is `snipemgr.yaml` only —
      state and integration configs are not touched
- [x] `cmd/root.go` — `configFileMissing` flag set by `initConfig()` via `os.Stat`;
      `PersistentPreRunE` prints nudge when config is absent and command is not `init`
- [x] README, CONTEXT.md, architecture.md updated

### Timezone-aware Cloud Scheduler

- [x] `internal/wizard/wizard.go` — timezone select in install/config wizard:
      UTC, Eastern, Central, Mountain, Pacific, Other (free-text IANA follow-up)
- [x] `internal/scheduler/gcp.go` — `Timezone string` added to `JobSpec`;
      `buildSchedulerJob` uses it instead of hardcoded `"UTC"`
- [x] `internal/state/store.go` — `Timezone string` (`omitempty`) added to
      `InstalledIntegration`
- [x] `cmd/install.go` — reads `gcp.scheduler_timezone` from viper for
      non-interactive default; wizard result populates both state and `JobSpec`
- [x] `snipemgr.example.yaml` — `gcp.scheduler_timezone: "UTC"` added with docs
- [x] README, CONTEXT.md, architecture.md updated

### Release infrastructure

- [x] `.github/release.yml` — PR-label categorization for auto-generated release notes
- [x] README `## Version History` table updated to include all v1.1.0 additions

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
