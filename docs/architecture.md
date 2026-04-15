# snipemgr — Architecture

## Overview

`snipemgr` is a Go CLI tool structured as a thin orchestration layer over three
external systems: GitHub (registry), GCP (scheduling + secrets), and the local
filesystem (state). It never calls Snipe-IT or any vendor API directly — those
are entirely the integrations' domain.

```
┌─────────────────────────────────────────────────────────────┐
│                         snipemgr CLI                         │
│                                                              │
│  list  install  status  run  enable  disable  config  upgrade│
└──────────────────────────┬──────────────────────────────────┘
                           │
          ┌────────────────┼─────────────────┐
          ▼                ▼                  ▼
   ┌─────────────┐  ┌─────────────┐  ┌──────────────┐
   │   registry  │  │  scheduler  │  │    state     │
   │  (GitHub)   │  │   (GCP)     │  │  (local JSON)│
   └─────────────┘  └──────┬──────┘  └──────────────┘
                           │
                    ┌──────┴──────┐
                    │   secrets   │
                    │  (Secret    │
                    │  Manager)   │
                    └─────────────┘
```

---

## Component: Registry (`internal/registry`)

**Purpose:** Discover integrations and fetch their manifests.

**Discovery flow:**

1. Read `registry.sources` from `snipemgr.yaml` (list of GitHub owners/orgs)
2. For each source, call GitHub search API:
   `GET /search/repositories?q=topic:2snipe+user:{owner}`
   Topic `2snipe` is the explicit opt-in signal — repo owners add it deliberately.
3. For each repo found, attempt:
   `GET https://raw.githubusercontent.com/{owner}/{repo}/main/2snipe.json`
4. 404 → silently skip (not an opt-in integration)
5. Present but fails struct validation → skip with DEBUG log
6. Valid manifest → include in result set

**Rate limiting:**
- Unauthenticated: 60 req/hr — aggressively warn if no token configured
- Authenticated (PAT): 5000 req/hr — recommended, document in README
- Cache manifest responses for the session to avoid redundant fetches

**Key types (`internal/registry/types.go`):**

```go
type Source struct {
    Owner string
}

type Manifest struct {
    Schema       string        `json:"$schema"`
    Name         string        `json:"name"`
    DisplayName  string        `json:"display_name"`
    Description  string        `json:"description"`
    Version      string        `json:"version"`
    MinSnipemgr  string        `json:"min_snipemgr"`
    Tags         []string      `json:"tags"`
    ConfigSchema []ConfigField `json:"config_schema"`
    SharedConfig []string      `json:"shared_config"`
    Commands     Commands      `json:"commands"`
    Releases     Releases      `json:"releases"`
}

type ConfigField struct {
    Key      string `json:"key"`
    Label    string `json:"label"`
    Secret   bool   `json:"secret"`
    Required bool   `json:"required"`
    Default  string `json:"default"`
    Hint     string `json:"hint"`
}

type Releases struct {
    GitHubReleases bool   `json:"github_releases"`
    AssetPattern   string `json:"asset_pattern"`
    // asset_pattern tokens: {os}, {arch}
    // e.g. "github2snipe_{os}_{arch}"
}

// Integration is a registry result: repo metadata + manifest
type Integration struct {
    RepoName    string
    RepoURL     string
    Manifest    *Manifest
    Installed   bool    // cross-referenced against state
    LocalVersion string // from state, if installed
    UpdateAvail  bool   // manifest.Version > LocalVersion
}
```

---

## Component: State (`internal/state`)

**Purpose:** Track what is installed locally and the last known GCP resource names.

**File location:** `~/.snipemgr/state.json` (expandable to GCS in future)

**Rules:**
- Read at startup of any command that needs it
- Write atomically (write to `.tmp`, rename)
- Never store secret values — only Secret Manager resource names/paths
- Version field enables future migration

---

## Component: Wizard (`internal/wizard`)

**Purpose:** Drive the interactive install/config flow from a manifest's
`config_schema`. The manager has no hardcoded knowledge of any integration's
config fields — everything comes from the manifest.

**Flow for `snipemgr install <n>`:**

```
1. First-time check: is snipemgr.yaml configured?
   └─ No → run first-time setup wizard (GCP project, region, SA, GitHub token)

2. Fetch manifest for <n>

3. Display: name, version, description, tags

4. For each ConfigField in manifest.config_schema:
   a. If field.key starts with a prefix in manifest.shared_config
      AND that secret already exists in state/Secret Manager:
      → Offer to reuse ("Snipe-IT credentials already configured. Reuse? [Y/n]")
   b. If field.secret == true: render password input (masked)
   c. Otherwise: render text input with default pre-filled

5. Secrets backend choice:
   ○ GCP Secret Manager (recommended — required for scheduling)
   ○ Local settings.yaml only

6. Schedule choice:
   ○ Daily at 06:00
   ○ Daily at another time
   ○ Custom cron
   ○ Manual only (no Cloud Scheduler job created)

7. Write secrets to chosen backend

8. If GCP: create Cloud Run Job + Cloud Scheduler trigger

9. Update state.json

10. Print summary and next steps
```

**Non-interactive mode (`--no-interactive`):**
All wizard fields must be expressible as CLI flags on the `install` command.
The wizard detects non-TTY or `--no-interactive` and reads from flags instead
of rendering huh forms. Required fields with no value → fatal error with clear message.

---

## Component: Installer (`internal/installer`)

**Purpose:** Download the correct binary for the current OS/arch and write a
`settings.yaml` skeleton.

**Binary resolution:**
1. Read `manifest.releases.asset_pattern` (e.g. `github2snipe_{os}_{arch}`)
2. Substitute `{os}` (darwin/linux/windows) and `{arch}` (amd64/arm64)
3. Fetch GitHub Releases for the repo to find the matching asset URL
4. Download to `~/.snipemgr/bin/{name}` (or a configurable path)

**settings.yaml skeleton:**
Generated from `config_schema` with placeholder values and comment hints.
Written to `~/.snipemgr/config/{name}/settings.yaml`.
Populated with actual values after wizard completes.

---

## Component: Scheduler (`internal/scheduler`)

**Purpose:** Create and manage Cloud Run Jobs and Cloud Scheduler triggers.

**GCP APIs used:**
- Cloud Run Jobs: `run.googleapis.com/v2` — `projects.locations.jobs`
- Cloud Scheduler: `cloudscheduler.googleapis.com/v1` — `projects.locations.jobs`

**Cloud Run Job spec:**
```
image: {artifact_registry_path}/{name}:latest
env vars: injected from Secret Manager at execution time
service_account: from snipemgr.yaml gcp.service_account
```

**Note on container images:**
Each integration needs a Docker image in Artifact Registry for Cloud Run Jobs.
The `install` command should detect whether an image exists and warn if not.
Building and pushing images is out of scope for phase 1 — document the manual
step and add automation as a phase 3+ enhancement.

---

## Component: Secrets (`internal/secrets`)

**Purpose:** Read and write secrets to GCP Secret Manager.

**Naming convention:**
- Shared: `snipe/snipe-url`, `snipe/snipe-token`
- Per-integration: `{name}/{field_key_last_segment}`
  e.g. `github2snipe/token` for field key `github.token`

**Operations needed:**
- `Get(name string) (string, error)`
- `Set(name, value string) error`
- `Exists(name string) (bool, error)`
- `ListByPrefix(prefix string) ([]string, error)`

---

## `list` command output

```
  INTEGRATION         STATUS        VERSION   DESCRIPTION
  ────────────────────────────────────────────────────────────────────────
  github2snipe        ● installed   v1.0.0    Sync GitHub members → Snipe-IT
  1password2snipe     ● installed   v1.0.0    Sync 1Password users → Snipe-IT
  googleworkspace     ○ available   v1.1.0    Sync Google Workspace → Snipe-IT
  oktagov2snipe       ○ available   v1.0.0    Sync Okta identities → Snipe-IT
```

Status indicators:
- `● installed` — in state.json
- `● installed ↑ update` — installed, newer version in manifest
- `○ available` — not installed
- `⚠ manifest error` — repo found but manifest invalid (debug only)

---

## `status` command output

```
  INTEGRATION       ENABLED   SCHEDULE     LAST RUN              RESULT
  ──────────────────────────────────────────────────────────────────────────
  github2snipe      ✓         0 6 * * *    2026-04-14 06:00 UTC  ✓ success
  1password2snipe   ✓         0 7 * * *    2026-04-14 07:00 UTC  ✗ failed
  oktagov2snipe     ✗ paused  0 8 * * *    2026-04-13 08:00 UTC  ✓ success
```

Last-run data comes from Cloud Run Jobs executions list API (not Cloud Logging).

---

## Dependency rationale

| Package | Use | Notes |
|---------|-----|-------|
| `cobra` | CLI framework | Same as all integrations |
| `viper` | Config/env | Same as all integrations |
| `log/slog` | Logging | Stdlib, same as all integrations |
| `charmbracelet/huh` | Interactive forms | Install wizard only |
| `charmbracelet/lipgloss` | Table styling | list + status commands |
| `google/go-github` | GitHub API | Registry search + release fetch |
| `cloud.google.com/go/run` | Cloud Run Jobs API | Scheduler component |
| `cloud.google.com/go/scheduler` | Cloud Scheduler API | Scheduler component |
| `cloud.google.com/go/secretmanager` | Secret Manager API | Secrets component |

No other external packages without explicit approval.
