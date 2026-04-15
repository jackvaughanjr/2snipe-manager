# snipemgr — Architecture

## Overview

`snipemgr` is a Go CLI tool structured as a thin orchestration layer over four
external systems: GitHub (registry), GCP (scheduling + secrets), Snipe-IT
(category management), and the local filesystem (state).

The integrations themselves handle all Snipe-IT sync logic — `snipemgr` touches
Snipe-IT for one purpose only: ensuring license categories exist before an
integration is installed.

```
┌─────────────────────────────────────────────────────────────┐
│                         snipemgr CLI                         │
│                                                              │
│  list  install  status  run  enable  disable  config  upgrade│
└──────────────────────────┬──────────────────────────────────┘
                           │
          ┌────────────────┼──────────────────┬──────────────┐
          ▼                ▼                  ▼               ▼
   ┌─────────────┐  ┌─────────────┐  ┌──────────────┐ ┌──────────────┐
   │   registry  │  │  scheduler  │  │    state     │ │  categories  │
   │  (GitHub)   │  │   (GCP)     │  │  (local JSON)│ │  (Snipe-IT)  │
   └─────────────┘  └──────┬──────┘  └──────────────┘ └──────────────┘
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
    Category     string        `json:"category"`
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
    // e.g. "1password2snipe_{os}_{arch}"
}

// Integration is a registry result: repo metadata + manifest
type Integration struct {
    RepoName     string
    RepoURL      string
    Manifest     *Manifest
    Installed    bool   // cross-referenced against state
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
   └─ No → run first-time setup wizard (Snipe-IT URL + token, GCP project,
            region, SA, GitHub token, category seed offer)

2. Fetch manifest for <n>

3. Display: name, version, description, tags, category

4. For each ConfigField in manifest.config_schema:
   a. If field.key starts with a prefix in manifest.shared_config
      AND that secret already exists in state/Secret Manager:
      → Offer to reuse ("Snipe-IT credentials already configured. Reuse? [Y/n]")
   b. If field.secret == true: render password input (masked)
   c. Otherwise: render text input with default pre-filled

5. If manifest.category is set:
   → Check if category exists in Snipe-IT (GET /api/v1/categories)
   → If not: create it automatically (POST /api/v1/categories)
   → Log: "✓ Category 'AI Tools' ready"

6. Secrets backend choice:
   ○ GCP Secret Manager (recommended — required for scheduling)
   ○ Local settings.yaml only

7. Schedule choice:
   ○ Daily at 06:00
   ○ Daily at another time
   ○ Custom cron
   ○ Manual only (no Cloud Scheduler job created)

8. Write secrets to chosen backend

9. If GCP: create Cloud Run Job + Cloud Scheduler trigger

10. Update state.json

11. Print summary and next steps
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
1. Read `manifest.releases.asset_pattern` (e.g. `1password2snipe_{os}_{arch}`)
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
  e.g. `1password2snipe/api-token` for field key `onepassword.api_token`

**Operations needed:**
- `Get(name string) (string, error)`
- `Set(name, value string) error`
- `Exists(name string) (bool, error)`
- `ListByPrefix(prefix string) ([]string, error)`

---

## Component: Categories (`internal/snipeit`)

**Purpose:** Ensure Snipe-IT license categories exist before an integration is
installed. This is the only direct Snipe-IT API interaction `snipemgr` performs —
all sync logic remains entirely in the individual integration binaries.

**Snipe-IT credentials** are collected during first-time setup (they are
`shared_config` values already needed by every integration) and stored the same
way as other shared secrets.

**APIs used:**
- `GET  {snipe_url}/api/v1/categories?limit=500&offset=0` — list existing categories
- `POST {snipe_url}/api/v1/categories` — create a new category

**Package location:** `internal/snipeit/categories.go`

**Key function:**
```go
// EnsureCategory checks if a category with the given name exists in Snipe-IT.
// If it does not exist, it creates it with type "license". Returns the category
// ID in either case. Returns 0 and a warning (not an error) if name is empty.
func EnsureCategory(client *Client, name string) (int, error)
```

**Note on response envelope:** Snipe-IT POST responses wrap the created object
in `{ "status": "success", "messages": {...}, "payload": {...} }` — unwrap
`payload` to get the category ID. GET list responses are direct (no envelope).
This is the same pattern documented in the parent `CLAUDE.md`.

---

### Default category list

`snipemgr` ships with a built-in default category list. During first-time setup,
the wizard offers to seed all
categories at once:

```
Seed default license categories in Snipe-IT? [Y/n]

  The following categories will be created if they don't already exist:
  • AI Tools
  • Communication & Collaboration
  • Design & Creative
  • Developer Tools & Hosting
  • Endpoint Management & Security
  • Identity & Access Management
  • Misc Software
  • Productivity
  • Project & Knowledge Management
  • Training & Learning
```

The default list is defined as a package-level variable in
`internal/snipeit/categories.go` so it can be updated without touching
wizard or command logic:

```go
// DefaultCategories is the canonical list of Snipe-IT license categories
// for this organization. Seeded during first-time setup if accepted.
var DefaultCategories = []string{
    "AI Tools",
    "Communication & Collaboration",
    "Design & Creative",
    "Developer Tools & Hosting",
    "Endpoint Management & Security",
    "Identity & Access Management",
    "Misc Software",
    "Productivity",
    "Project & Knowledge Management",
    "Training & Learning",
}
```

**Seed behavior:**
- Each category is checked via GET before attempting POST — idempotent
- Categories that already exist are skipped silently
- The seed step is offered once during first-time setup and can be re-run
  with `snipemgr categories seed` at any time
- Seed failures are non-fatal: warn and continue rather than aborting setup

---

## `snipemgr categories` subcommand

A lightweight management subcommand for category operations:

```bash
snipemgr categories list           # List all categories currently in Snipe-IT
snipemgr categories seed           # Seed default categories (idempotent)
snipemgr categories seed --dry-run # Show what would be created without creating
```

This subcommand is the only surface where `snipemgr` talks to Snipe-IT directly.
It requires Snipe-IT credentials to be configured (either in `snipemgr.yaml` or
already stored as shared secrets).

---

## `list` command output

```
  INTEGRATION         STATUS        VERSION   CATEGORY                        DESCRIPTION
  ────────────────────────────────────────────────────────────────────────────────────────────
  github2snipe        ● installed   v0.9.0    Developer Tools & Hosting       Sync GitHub members → Snipe-IT
  1password2snipe     ● installed   v1.1.0    Identity & Access Management    Sync 1Password users → Snipe-IT
  oktagov2snipe       ○ available   v1.0.0    Identity & Access Management    Sync Okta users → Snipe-IT
  slack2snipe         ○ available   v1.1.0    Communication & Collaboration   Sync Slack members → Snipe-IT
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
| `net/http` (stdlib) | Snipe-IT categories API | No extra dependency needed |

No other external packages without explicit approval.
