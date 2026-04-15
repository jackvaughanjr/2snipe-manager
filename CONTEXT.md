# snipemgr — Context for Claude Code

## What this repo is

`snipemgr` is a Go CLI tool that acts as a **package manager and orchestrator** for
the `*2snipe` integration suite. It lets users discover, install, configure, schedule,
and monitor all `*2snipe` integrations from a single place, without touching individual
integration repos or GCP consoles directly.

This is **not** a `*2snipe` integration. It does not sync any vendor data to Snipe-IT.
It manages the tools that do — with one narrow exception: it calls the Snipe-IT
categories API to ensure license categories exist before an integration is installed.

**Binary name:** `snipemgr`
**Repo:** `github.com/jackvaughanjr/2snipe-manager`
**Org:** your-org (`your-org.example.com`)

---

## Parent CLAUDE.md — what to use and what to skip

Claude Code auto-loads `CLAUDE.md` from the parent directory
(`~/Documents/GitHub/CLAUDE.md`). That file is written for `*2snipe` integration
repos. Load and follow it with these caveats:

**Sections that apply here — follow them exactly:**
- Go conventions (no unnecessary dependencies, return errors, validate config early)
- CLI / config patterns (cobra + viper, standard global flags, `PersistentPreRunE`
  logging init, `SilenceUsage`/`SilenceErrors`, `fatal()` helper, version flag)
- Dry-run safety (applicable to the `run` and `categories seed` commands)
- Config & example files (no real org names, `settings.example.yaml` pattern)
- README conventions
- Release workflow (`.github/workflows/release.yml`, same cross-platform build)

**Sections that do NOT apply here — skip them:**
- Snipe-IT API sync patterns (`snipemgr` calls only the Snipe-IT categories API,
  not the license/asset/seat APIs — all sync logic stays in the integration binaries)
- Standard sync flags (`--email`, `--force` are not relevant)
- Snipe-IT license handling
- The "Starting a new integration" workflow
- `docs/source-files.md` and `docs/scaffolding.md` references

---

## Reference docs — load on demand

| File | Load when |
|------|-----------|
| `docs/architecture.md` | Working on any structural decision, adding a new command, or understanding how components relate |
| `docs/order-of-operations.md` | Starting a new phase, unsure what to build next, or checking what choices are still open |
| `docs/manifest-spec.md` | Working on registry discovery, install wizard config schema, or `2snipe.json` validation |
| `docs/gcp-infra.md` | Working on Cloud Run Jobs, Cloud Scheduler, or Secret Manager integration |
| `docs/features-backlog.md` | Phase 4+ work or when the user asks about future enhancements |

---

## Key decisions already made

- **Language:** Go 1.22+, same stack as the integrations (cobra + viper + slog)
- **Discovery mechanism:** GitHub API search for repos matching `*2snipe` under
  configured owner(s), then attempt to fetch `2snipe.json` from each repo root.
  Repos without a valid `2snipe.json` are silently excluded. The manifest file is
  the opt-in gate — no central registry needed.
- **Secrets backend:** GCP Secret Manager (required for scheduled/Cloud Run use).
  Local `settings.yaml`-only mode is supported for standalone/manual use.
- **Scheduler:** GCP Cloud Scheduler + Cloud Run Jobs. One job per integration.
- **Integration binaries stay dumb:** Integrations know nothing about the manager.
  The manager injects config as environment variables at Cloud Run Job execution
  time. Viper env var bindings already exist in all integrations — no changes to
  any `*2snipe` integration are needed.
- **Category management:** `snipemgr` ensures Snipe-IT license categories exist
  at install time using the Snipe-IT categories API (GET + POST only). A built-in
  default category list is seeded during
  first-time setup. See `docs/architecture.md` for the full list and behavior.
- **State file:** `~/.snipemgr/state.json` tracks installed integrations locally.
  Optionally portable to a GCS bucket for multi-machine use (future).
- **TUI library:** `charmbracelet/huh` for interactive install wizard forms.
  `charmbracelet/lipgloss` for table rendering in `list` and `status` commands.
  These are the only non-standard UI dependencies permitted initially.
- **No GUI initially.** The CLI is the product. A web UI is a future enhancement
  gated on real demand.

---

## Commands

| Command | Description |
|---------|-------------|
| `list` | Discover and display all available integrations from the registry |
| `install <n>` | Download, configure, and optionally schedule an integration |
| `uninstall <n>` | Remove integration, Cloud Run Job, and Scheduler trigger |
| `enable <n>` | Re-enable a paused Cloud Scheduler job |
| `disable <n>` | Pause scheduling without removing the integration |
| `run <n>` | Trigger a Cloud Run Job immediately and tail logs |
| `status` | Show all installed integrations with last-run result and schedule |
| `config <n>` | Re-run the configuration wizard for an installed integration |
| `upgrade` | Check for newer versions of all installed integrations |
| `categories list` | List all license categories currently in Snipe-IT |
| `categories seed` | Seed default license categories into Snipe-IT (idempotent, `--dry-run` supported) |

---

## Standard global flags (cobra + viper, same as all integrations)

| Flag | Description |
|------|-------------|
| `--config` | Path to snipemgr config file (default: `snipemgr.yaml`) |
| `-v, --verbose` | INFO-level logging |
| `-d, --debug` | DEBUG-level logging |
| `--log-file` | Append logs to a file |
| `--log-format` | `text` (default) or `json` |
| `--no-interactive` | Disable huh forms; use flags only (for scripted/piped use) |

---

## Standard file structure

```
main.go
cmd/
  root.go           # cobra root, viper, logging (PersistentPreRunE pattern from CLAUDE.md)
  list.go
  install.go
  uninstall.go
  enable.go
  disable.go
  run.go
  status.go
  config.go
  upgrade.go
  categories.go     # subcommands: list, seed (--dry-run)
internal/
  registry/
    client.go       # GitHub API search + manifest fetch + schema validation
    types.go        # Integration, Manifest, ConfigField, Source structs
  installer/
    installer.go    # binary download, settings.yaml skeleton generation
  scheduler/
    gcp.go          # Cloud Run Jobs + Cloud Scheduler API wrappers
  secrets/
    manager.go      # GCP Secret Manager read/write
  snipeit/
    categories.go   # EnsureCategory, SeedDefaults, DefaultCategories list
  state/
    store.go        # ~/.snipemgr/state.json read/write
  wizard/
    wizard.go       # huh-based interactive config forms, driven by manifest schema
.github/
  workflows/
    release.yml     # same cross-platform build as integrations
go.mod              # module: github.com/jackvaughanjr/2snipe-manager, go 1.22
go.sum
snipemgr.example.yaml   # manager's own config (GCP project, registry sources, etc.)
README.md
CONTEXT.md
.gitignore          # excludes: snipemgr.yaml, ~/.snipemgr/ note, .DS_Store, binary
2snipe.schema.json  # JSON Schema for validating integration manifests — hosted here
```

---

## Manager config (`snipemgr.yaml`)

```yaml
registry:
  sources:
    - owner: jackvaughanjr   # GitHub user or org to scan
  require_manifest: true     # only show repos with a valid 2snipe.json
  github_token: ""           # optional: PAT for higher rate limits (60 → 5000 req/hr)

snipe_it:
  url: ""                    # Snipe-IT instance URL (required for category management)
  api_key: ""                # Snipe-IT API key (required for category management)

gcp:
  project: ""                # GCP project ID
  region: "us-central1"
  service_account: ""        # SA email for Cloud Run Jobs

state:
  path: "~/.snipemgr/state.json"   # local default
  # gcs_bucket: ""                 # optional: portable state across machines
```

**Note:** `snipe_it.url` and `snipe_it.api_key` are collected during first-time
setup. They are the same credentials shared by all integrations and stored in
Secret Manager under `snipe/snipe-url` and `snipe/snipe-token` — `snipemgr` reads
them back from Secret Manager at runtime so they don't need to live in
`snipemgr.yaml` in plain text once GCP is configured.

---

## State file (`~/.snipemgr/state.json`)

```json
{
  "version": "1",
  "integrations": {
    "github2snipe": {
      "version": "0.9.0",
      "enabled": true,
      "schedule": "0 6 * * *",
      "cloud_run_job": "projects/your-gcp-project/locations/us-central1/jobs/github2snipe",
      "scheduler_job": "projects/your-gcp-project/locations/us-central1/jobs/github2snipe-trigger",
      "secrets_backend": "gcp",
      "installed_at": "2026-04-09T12:00:00Z",
      "last_run_at": "2026-04-14T06:00:00Z",
      "last_run_result": "success"
    }
  }
}
```

---

## Shared secrets strategy (GCP Secret Manager)

All `*2snipe` integrations share a Snipe-IT instance and the same Okta tenant,
so shared secrets are stored once and referenced by all jobs:

```
snipe/snipe-url            shared across all integrations + snipemgr itself
snipe/snipe-token          shared across all integrations + snipemgr itself
github2snipe/token         integration-specific
1password2snipe/token      integration-specific
oktagov2snipe/token        integration-specific
```

The install wizard detects existing shared secrets and offers to reuse them.
`snipemgr` itself reads `snipe/snipe-url` and `snipe/snipe-token` when calling
the Snipe-IT categories API.

---

## Notes for future sessions

- Always gate on `snipemgr.yaml` being configured before any GCP or Snipe-IT
  calls; detect missing config early and prompt a first-time setup flow
- First-time setup collects: Snipe-IT URL + API key, GCP project/region/SA,
  GitHub token; offers to seed default categories before exiting
- The `2snipe.schema.json` lives in this repo and is referenced by the `$schema`
  field in each integration's `2snipe.json` — keep it backward compatible
- Cloud Run Jobs API is `run.googleapis.com/v2` — not the same as Cloud Run services
- For last-run status, use the Cloud Run Jobs executions list endpoint, not Cloud
  Logging (faster and structured)
- `huh` forms don't render in pipe/non-TTY mode; the `--no-interactive` flag must
  fall back to cobra flags for all wizard fields so scripted installs work
- GitHub unauthenticated rate limit is 60 req/hr — encourage adding a GitHub token
  in `snipemgr.yaml` early to avoid hitting it during `list` calls
- Snipe-IT POST responses wrap created objects in `{ "status", "messages", "payload" }`
  — unwrap `payload` to get the created category ID; GET list responses are direct
- `categories seed` must be idempotent: GET before POST, skip existing categories
  silently, treat seed failures as warnings not fatal errors
