# snipemgr

[![Latest Release](https://img.shields.io/github/v/release/jackvaughanjr/2snipe-manager)](https://github.com/jackvaughanjr/2snipe-manager/releases/latest) [![Go Version](https://img.shields.io/github/go-mod/go-version/jackvaughanjr/2snipe-manager)](go.mod) [![License](https://img.shields.io/github/license/jackvaughanjr/2snipe-manager)](LICENSE) [![Build](https://github.com/jackvaughanjr/2snipe-manager/actions/workflows/release.yml/badge.svg)](https://github.com/jackvaughanjr/2snipe-manager/actions/workflows/release.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/jackvaughanjr/2snipe-manager)](https://goreportcard.com/report/github.com/jackvaughanjr/2snipe-manager) [![Downloads](https://img.shields.io/github/downloads/jackvaughanjr/2snipe-manager/total)](https://github.com/jackvaughanjr/2snipe-manager/releases)

A Go CLI tool that acts as a package manager and orchestrator for the [`*2snipe`](https://github.com/jackvaughanjr) integration suite — a collection of tools that sync vendor software licenses and assets into [Snipe-IT](https://snipeitapp.com).

`snipemgr` lets you discover, install, configure, schedule, and monitor all `*2snipe` integrations from a single place. It handles secrets via GCP Secret Manager, scheduling via Cloud Run Jobs and Cloud Scheduler, and discovery via a manifest-based GitHub registry.

> **Status:** Under active development. See [Build phases](#build-phases) for current progress.

---

## What this is not

`snipemgr` does not sync anything to Snipe-IT itself. It manages the tools that do. The individual `*2snipe` integrations are standalone Go binaries — they know nothing about this manager and require no changes to work with it.

---

## Repository layout

```
CONTEXT.md                  Claude Code context — read this before any AI-assisted session
docs/
  architecture.md           Component design, data types, wizard flow, dependency rationale
  order-of-operations.md    Phased build plan with verification steps and open choices
  manifest-spec.md          Full spec for the 2snipe.json integration manifest file
  gcp-infra.md              GCP setup, IAM requirements, API references, cost estimate
  features-backlog.md       Post-core enhancement ideas, tiered by value and complexity
2snipe.schema.json          JSON Schema for validating integration manifests (created in Phase 1)
snipemgr.example.yaml       Manager config template — copy to snipemgr.yaml and fill in values
```

Source code lives under `cmd/` and `internal/` and is built out across Phases 0–4. See [Build phases](#build-phases) for what exists at any given point.

---

## For contributors and implementors

**Read these before writing any code, in this order:**

1. `CONTEXT.md` — what this repo is, which parts of the parent `CLAUDE.md` apply, key decisions already made, and a reference table for the docs below. This is the file Claude Code loads at session start.
2. `docs/order-of-operations.md` — the phased build plan. Start here to understand where the project currently stands, what to build next, what choices are still open, and how to verify each phase before moving on.
3. `docs/architecture.md` — full component design. Load this when working on any specific component or command.
4. The relevant `docs/` file for the area you're working in (manifest, GCP, features).

---

## Prerequisites

### To start building (Phases 0–2)

- **Go 1.22+** — verify with `go version`
- **Git**
- A **GitHub personal access token** (optional but recommended for Phase 1+)
  - Create at `github.com/settings/tokens/new`
  - Scope: `public_repo` (or `repo` if the `*2snipe` repos are private)
  - Without a token, GitHub limits API calls to 60/hr — sufficient for development but tight
  - Set in `snipemgr.yaml` under `registry.github_token`

### Before starting Phase 3 (GCP integration)

- A **GCP project** with billing enabled
- The **`gcloud` CLI** installed and authenticated
- Complete the GCP setup checklist in `docs/order-of-operations.md` — it's a single `gcloud services enable` command, one service account, one IAM binding, and one `gcloud auth` call

### Before Phase 3's `run` command will work end-to-end

Each integration needs a Docker image pushed to Artifact Registry. Building and pushing images is a manual step in Phase 3. The `snipemgr run` command will detect a missing image and print exact instructions rather than failing silently. Image automation is a Phase 4+ feature.

---

## Build phases

The project is built in four phases. Each phase has a defined goal, required tasks, open choices that must be confirmed before coding, and a verification checklist. Do not start a phase until the previous phase's verification passes.

| Phase | Goal | Status | GCP required |
|-------|------|--------|-------------|
| 0 | Repo bootstrap — runnable binary, CLI skeleton, config loading | ✓ Complete | No |
| 1 | `snipemgr list` — GitHub registry discovery, manifest validation, table output | ✓ Complete | No |
| 2 | `snipemgr install` — binary download, config wizard, category management, local secrets | ✓ Complete | No |
| 3 | GCP integration — Secret Manager, Cloud Run Jobs, Cloud Scheduler, `status`/`run`/`enable`/`disable` | Not started | Yes |
| 4 | `snipemgr upgrade`, release workflow, README polish | Not started | No (uses existing GCP setup) |

Full details, verification commands, Go test targets, and open choices for each phase are in `docs/order-of-operations.md`.

---

## Commands

Commands marked ✓ are available now. Remaining commands ship in Phase 3–4.

```
snipemgr list                     ✓  Discover available integrations from the registry
snipemgr install <name>           ✓  Download, configure, and install an integration locally
snipemgr config <name>            ✓  Re-run the configuration wizard for an installed integration
snipemgr uninstall <name>         ✓  Remove an installed integration (binary, config, state)
snipemgr categories list          ✓  List all Snipe-IT license categories
snipemgr categories seed          ✓  Seed default license categories (idempotent, --dry-run)
snipemgr status                      Show installed integrations with last-run result and schedule
snipemgr run <name>                  Trigger a Cloud Run Job immediately
snipemgr enable <name>               Resume a paused Cloud Scheduler job
snipemgr disable <name>              Pause scheduling without removing the integration
snipemgr upgrade                     Check for and apply newer versions of installed integrations
```

Global flags on all commands: `--config`, `-v/--verbose`, `-d/--debug`, `--log-file`, `--log-format`, `--no-interactive`

### `install` flags

```
--snipe-url <url>         Snipe-IT instance URL (sets snipe_it.url config field)
--snipe-token <key>       Snipe-IT API key (sets snipe_it.api_key config field)
--field key=value         Set any config field by key (repeatable)
--schedule <cron|manual>  Sync schedule, e.g. "0 6 * * *" or "manual" (default: manual)
```

`config` accepts the same flags. Pass `--no-interactive` to skip wizard prompts and use flags only.

---

## How integration discovery works

`snipemgr list` searches GitHub for repositories matching `*2snipe` under configured owners, then attempts to fetch a `2snipe.json` manifest from each repo root. **Repos without a valid manifest are silently excluded** — the manifest file is the opt-in gate, not just metadata.

This means:
- Publishing a new integration to the registry is just committing a `2snipe.json` to its repo
- Third-party or experimental repos are excluded unless they deliberately opt in
- The manager has no hardcoded knowledge of any specific integration

The manifest also drives the install wizard — all config prompts come from the manifest's `config_schema`, so the manager never needs updating when a new integration is added.

Full manifest specification is in `docs/manifest-spec.md`. The JSON Schema for editor validation and programmatic checking is in `2snipe.schema.json` (created in Phase 1).

---

## How secrets work

In local mode (Phases 0–2), secrets are written to a `settings.yaml` file per integration under `~/.snipemgr/config/{name}/settings.yaml`. This file is never committed.

In GCP mode (Phase 3+), secrets are stored in GCP Secret Manager and injected as environment variables at Cloud Run Job execution time. The integration binaries pick them up via their existing viper env var bindings — no changes to the integrations are needed.

Shared secrets (Snipe-IT URL and token) are stored once and reused across all integrations. The install wizard detects existing shared secrets and offers to reuse them.

---

## State

`snipemgr` tracks installed integrations in `~/.snipemgr/state.json`. This file records the installed version, enabled status, cron schedule, and GCP resource names for each integration. It is never committed to the repo.

---

## Tech stack

- **Language:** Go 1.22+
- **CLI:** `cobra` + `viper` (same as all `*2snipe` integrations)
- **Logging:** `log/slog`
- **Interactive forms:** `charmbracelet/huh`
- **Table rendering:** `charmbracelet/lipgloss`
- **GitHub API:** `net/http` (stdlib) — GitHub Search + Contents API directly
- **GCP:** `cloud.google.com/go/run`, `cloud.google.com/go/scheduler`, `cloud.google.com/go/secretmanager`

---

## Adding `2snipe.json` to an existing integration

Before `snipemgr install` can work with an integration, that integration's repo needs a `2snipe.json` manifest at its root. See `docs/manifest-spec.md` for the full spec and a complete example.

The short version:

```json
{
  "$schema": "https://raw.githubusercontent.com/jackvaughanjr/2snipe-manager/main/2snipe.schema.json",
  "name": "yourvendor2snipe",
  "display_name": "Your Vendor",
  "description": "One-line description for snipemgr list",
  "version": "1.0.0",
  "config_schema": [
    { "key": "vendor.api_key", "label": "API Key", "secret": true, "required": true }
  ],
  "shared_config": ["snipe_it"],
  "releases": {
    "github_releases": true,
    "asset_pattern": "yourvendor2snipe-{os}-{arch}"
  }
}
```

Also add the GitHub topic `2snipe` to the repo (Settings → Topics) — this is the primary discovery signal used by `snipemgr list`.

---

## Installation

Pre-built binaries are available from the [latest release](https://github.com/jackvaughanjr/2snipe-manager/releases/latest) (available after Phase 4):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/jackvaughanjr/2snipe-manager/releases/latest/download/snipemgr-darwin-arm64 -o snipemgr
chmod +x snipemgr

# Linux (amd64)
curl -L https://github.com/jackvaughanjr/2snipe-manager/releases/latest/download/snipemgr-linux-amd64 -o snipemgr
chmod +x snipemgr

# Linux (arm64)
curl -L https://github.com/jackvaughanjr/2snipe-manager/releases/latest/download/snipemgr-linux-arm64 -o snipemgr
chmod +x snipemgr
```

Or build from source:

```bash
git clone https://github.com/jackvaughanjr/2snipe-manager
cd 2snipe-manager
go build -o snipemgr .
```

---

## Version History

| Version | Key changes |
|---------|-------------|
| v1.0.0 | *(planned — Phase 4)* Full release with all commands, release workflow, and README polish |
| v0.3.0 | *(planned — Phase 3)* GCP integration: Secret Manager, Cloud Run Jobs, Cloud Scheduler, `status`/`run`/`enable`/`disable` |
| v0.2.0 | Phase 2 — `snipemgr install` end to end: GitHub Releases download, config wizard (huh forms + `--no-interactive` mode), Snipe-IT category management (`categories list`, `categories seed --dry-run`), `settings.yaml` generation, atomic state writes. Also: `uninstall`, `config` (re-run wizard), `● installed` / `○ available` status in `list`. |
| v0.1.0 | Phase 1 — `snipemgr list` end to end: GitHub registry discovery (topic `2snipe` + Contents API), manifest validation, lipgloss table in terminal, plain text when piped, state file creation. Manifests shipped for all five initial integrations. |
| v0.0.1 | Phase 0 bootstrap — runnable `snipemgr` binary with cobra+viper CLI skeleton, all global flags (`--config`, `--verbose`, `--debug`, `--log-file`, `--log-format`, `--no-interactive`), `PersistentPreRunE` logging init, `snipemgr.yaml` config loading, `fatal()` helper, and version embedding |

---

## Related repos

- [`jackvaughanjr/1password2snipe`](https://github.com/jackvaughanjr/1password2snipe) — 1Password Business member sync
- [`jackvaughanjr/github2snipe`](https://github.com/jackvaughanjr/github2snipe) — GitHub Enterprise / org member sync
- [`jackvaughanjr/googleworkspace2snipe`](https://github.com/jackvaughanjr/googleworkspace2snipe) — Google Workspace license sync
- [`jackvaughanjr/okta2snipe`](https://github.com/jackvaughanjr/okta2snipe) — Okta member sync
- [`jackvaughanjr/slack2snipe`](https://github.com/jackvaughanjr/slack2snipe) — Slack billable member sync
