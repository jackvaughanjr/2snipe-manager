# snipemgr

[![Latest Release](https://img.shields.io/github/v/release/jackvaughanjr/2snipe-manager)](https://github.com/jackvaughanjr/2snipe-manager/releases/latest) [![Go Version](https://img.shields.io/github/go-mod/go-version/jackvaughanjr/2snipe-manager)](go.mod) [![License](https://img.shields.io/github/license/jackvaughanjr/2snipe-manager)](LICENSE) [![Build](https://github.com/jackvaughanjr/2snipe-manager/actions/workflows/ci.yml/badge.svg)](https://github.com/jackvaughanjr/2snipe-manager/actions/workflows/ci.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/jackvaughanjr/2snipe-manager)](https://goreportcard.com/report/github.com/jackvaughanjr/2snipe-manager) [![Downloads](https://img.shields.io/github/downloads/jackvaughanjr/2snipe-manager/total)](https://github.com/jackvaughanjr/2snipe-manager/releases)

A Go CLI tool that acts as a package manager and orchestrator for the [`*2snipe`](https://github.com/jackvaughanjr) integration suite — a collection of tools that sync vendor software licenses and assets into [Snipe-IT](https://snipeitapp.com).

`snipemgr` lets you discover, install, configure, schedule, and monitor all `*2snipe` integrations from a single place. It handles secrets via GCP Secret Manager, scheduling via Cloud Run Jobs and Cloud Scheduler, and discovery via a manifest-based GitHub registry.

> **Note:** `snipemgr` does not sync anything to Snipe-IT itself. It manages the tools that do. The individual `*2snipe` integrations are standalone Go binaries — they know nothing about this manager and require no changes to work with it.

---

## Quick start

**1. Download the binary:**

```bash
# macOS (Apple Silicon)
curl -L https://github.com/jackvaughanjr/2snipe-manager/releases/latest/download/snipemgr-darwin-arm64 -o snipemgr && chmod +x snipemgr

# macOS (Intel)
curl -L https://github.com/jackvaughanjr/2snipe-manager/releases/latest/download/snipemgr-darwin-amd64 -o snipemgr && chmod +x snipemgr

# Linux (amd64)
curl -L https://github.com/jackvaughanjr/2snipe-manager/releases/latest/download/snipemgr-linux-amd64 -o snipemgr && chmod +x snipemgr

# Linux (arm64)
curl -L https://github.com/jackvaughanjr/2snipe-manager/releases/latest/download/snipemgr-linux-arm64 -o snipemgr && chmod +x snipemgr
```

Move the binary somewhere on your `$PATH` (e.g. `/usr/local/bin/snipemgr`).

**2. Run the setup wizard:**

```bash
snipemgr init
```

This creates `snipemgr.yaml` interactively — it walks you through your GitHub token, Snipe-IT credentials, and optional GCP config. Run it once; re-running prompts for confirmation before overwriting.

**3. See what's available:**

```bash
snipemgr list
```

**4. Install an integration:**

```bash
snipemgr install
```

Without a name argument, `install` fetches the live registry and shows a scrollable picker. Select an integration, fill in the prompts, and you're done. The integration binary is downloaded to `~/.snipemgr/bin/` and its config is written to `~/.snipemgr/config/{name}/settings.yaml`.

**5. Run it manually or set a schedule:**

To run immediately (GCP backend required):
```bash
snipemgr run github2snipe
```

To check status across all installed integrations:
```bash
snipemgr status
```

> **Want automated scheduling?** Install with `--secrets-backend gcp` to store secrets in Secret Manager and create a Cloud Run Job + Cloud Scheduler trigger. See [GCP setup](#gcp-setup-required-for---secrets-backend-gcp) first.

---

## Requirements

**To use pre-built binaries** — nothing. Download and run.

**To use the GCP backend** (`--secrets-backend gcp`):
- A GCP project with billing enabled
- The [`gcloud` CLI](https://cloud.google.com/sdk/docs/install) installed and authenticated
- See [GCP setup](#gcp-setup-required-for---secrets-backend-gcp) below

**To build from source** — Go 1.22+

**GitHub token (optional)** — without one, GitHub rate-limits API calls to 60/hr. Sufficient for occasional use but tight if you run `snipemgr list` frequently. Set `registry.github_token` in `snipemgr.yaml` with a token scoped to `public_repo` (or `repo` for private integration repos).

---

## Commands

```
snipemgr init                     First-time setup wizard — creates snipemgr.yaml interactively
snipemgr list                     Discover available integrations from the registry
snipemgr install [name]           Download, configure, and install an integration
snipemgr config <name>            Re-run the configuration wizard for an installed integration
snipemgr uninstall <name>         Remove an integration (binary, config, GCP resources, state)
snipemgr categories list          List all Snipe-IT license categories
snipemgr categories seed          Seed default license categories (idempotent, --dry-run)
snipemgr status                   Show installed integrations with version, schedule, and last-run result
snipemgr run <name>               Trigger a Cloud Run Job immediately
snipemgr enable <name>            Resume a paused Cloud Scheduler job
snipemgr disable <name>           Pause scheduling without removing the integration
snipemgr upgrade                  Check for and apply newer versions of installed integrations
```

Global flags on all commands: `--config`, `-v/--verbose`, `-d/--debug`, `--log-file`, `--log-format`, `--no-interactive`

### `init` flags

```
--force    Overwrite existing snipemgr.yaml without prompting for confirmation
```

`init` requires an interactive terminal. Re-running it on an existing config prompts for confirmation — it completely replaces `snipemgr.yaml` but does not affect installed integrations, integration config files, or state.

### `install` flags

```
--snipe-url <url>              Snipe-IT instance URL (sets snipe_it.url config field)
--snipe-token <key>            Snipe-IT API key (sets snipe_it.api_key config field)
--field key=value              Set any config field by key (repeatable)
--schedule <cron|manual>       Sync schedule, e.g. "0 6 * * *" or "manual" (default: manual)
--secrets-backend <gcp|local>  Secrets backend: "gcp" creates Cloud Run Job + Scheduler;
                               "local" writes settings.yaml only (default: local)
```

When called without a name argument in an interactive terminal, `install` fetches the registry and shows a scrollable picker so you can choose an integration without memorising its name. A **"My integration is not listed..."** option at the bottom explains how to add a new owner to `registry.sources`. Pass `--no-interactive` or pipe stdin to require an explicit name instead.

`config` accepts the same flags. Pass `--no-interactive` to skip wizard prompts and use flags only.

### `upgrade` flags

```
--all    Upgrade all outdated integrations without prompting (non-interactive)
```

Pass `--no-interactive` without `--all` to list available upgrades without applying them.

---

## GCP setup (required for `--secrets-backend gcp`)

GCP Secret Manager, Cloud Run Jobs, and Cloud Scheduler are used when you install an integration with `--secrets-backend gcp`. Complete this one-time setup before your first GCP-backend install:

**1. Enable required APIs** (one-time per project):

```bash
gcloud services enable \
  run.googleapis.com \
  cloudscheduler.googleapis.com \
  secretmanager.googleapis.com \
  artifactregistry.googleapis.com \
  --project YOUR_PROJECT_ID
```

**2. Create the Cloud Run runner service account**:

```bash
gcloud iam service-accounts create snipemgr-runner \
  --display-name="snipemgr Cloud Run Runner" \
  --project YOUR_PROJECT_ID

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:snipemgr-runner@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

**3. Authenticate your local machine**:

```bash
gcloud auth application-default login
```

**4. Set in `snipemgr.yaml`**:

```yaml
gcp:
  project: "YOUR_PROJECT_ID"
  region: "us-central1"
  service_account: "snipemgr-runner@YOUR_PROJECT_ID.iam.gserviceaccount.com"
```

---

## Building container images for Cloud Run Jobs

Cloud Run Jobs require a Docker image in Artifact Registry. This is a one-time step per integration. Run `snipemgr run <name>` after installing — it prints these instructions automatically if no image exists yet.

**1. Create the Artifact Registry repository** (one-time per project):

```bash
gcloud artifacts repositories create 2snipe \
  --repository-format=docker \
  --location=us-central1 \
  --project=YOUR_PROJECT_ID \
  --description="2snipe integration images"
```

**2. Authenticate Docker**:

```bash
gcloud auth configure-docker us-central1-docker.pkg.dev
```

**3. Build and push the integration image** (example: `github2snipe`):

```bash
git clone https://github.com/jackvaughanjr/github2snipe.git
cd github2snipe
```

If the repo doesn't have a `Dockerfile`, create a minimal one:

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY . .
RUN go build -o /app/github2snipe .

FROM alpine:3.21
COPY --from=builder /app/github2snipe /app/github2snipe
ENTRYPOINT ["/app/github2snipe"]
```

Then build and push:

```bash
docker build -t us-central1-docker.pkg.dev/YOUR_PROJECT_ID/2snipe/github2snipe:latest .
docker push us-central1-docker.pkg.dev/YOUR_PROJECT_ID/2snipe/github2snipe:latest
```

Replace `YOUR_PROJECT_ID` and `github2snipe` with your project and integration name. The image path pattern is:
```
{region}-docker.pkg.dev/{project}/2snipe/{integration-name}:latest
```

After pushing, trigger the job:

```bash
snipemgr run github2snipe
```

---

## How integration discovery works

`snipemgr list` searches GitHub for repositories matching `*2snipe` under configured owners, then attempts to fetch a `2snipe.json` manifest from each repo root. **Repos without a valid manifest are silently excluded** — the manifest file is the opt-in gate, not just metadata.

This means:
- Publishing a new integration to the registry is just committing a `2snipe.json` to its repo
- Third-party or experimental repos are excluded unless they deliberately opt in
- The manager has no hardcoded knowledge of any specific integration

The manifest also drives the install wizard — all config prompts come from the manifest's `config_schema`, so the manager never needs updating when a new integration is added.

Full manifest specification is in `docs/manifest-spec.md`. The JSON Schema for editor validation and programmatic checking is in `2snipe.schema.json`.

---

## How secrets work

In local mode, secrets are written to a `settings.yaml` file per integration under `~/.snipemgr/config/{name}/settings.yaml`. This file is never committed.

In GCP mode, secrets are stored in GCP Secret Manager and injected as environment variables at Cloud Run Job execution time. The integration binaries pick them up via their existing viper env var bindings — no changes to the integrations are needed.

Shared secrets (Snipe-IT URL and token) are stored once and reused across all integrations. The install wizard detects existing shared secrets and offers to reuse them.

---

## State

`snipemgr` tracks installed integrations in `~/.snipemgr/state.json`. This file records the installed version, enabled status, cron schedule, and GCP resource names for each integration. It is never committed to the repo.

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

## Tech stack

- **Language:** Go 1.22+
- **CLI:** `cobra` + `viper` (same as all `*2snipe` integrations)
- **Logging:** `log/slog`
- **Interactive forms:** `charmbracelet/huh`
- **Table rendering:** `charmbracelet/lipgloss`
- **GitHub API:** `net/http` (stdlib) — GitHub Search + Contents API directly
- **GCP:** `cloud.google.com/go/run`, `cloud.google.com/go/scheduler`, `cloud.google.com/go/secretmanager`

---

## Repository layout

```
snipemgr.example.yaml       Manager config template — copy to snipemgr.yaml and fill in values
2snipe.schema.json          JSON Schema for validating integration manifests
docs/
  architecture.md           Component design, data types, wizard flow, dependency rationale
  manifest-spec.md          Full spec for the 2snipe.json integration manifest file
  gcp-infra.md              GCP setup, IAM requirements, API references, cost estimate
  features-backlog.md       Post-core enhancement ideas, tiered by value and complexity
```

Source code lives under `cmd/` and `internal/`.

---

## Contributing

Read these before writing any code, in this order:

1. `CONTEXT.md` — what this repo is, key decisions already made, and a reference table for the docs below
2. `docs/architecture.md` — full component design
3. `docs/order-of-operations.md` — build history, phase gotchas, and verification logs
4. The relevant `docs/` file for the area you're working in

---

## Version History

| Version | Key changes |
|---------|-------------|
| v1.1.0 | First public release. `install` name argument made optional — omitting it in an interactive terminal shows a scrollable picker populated from the live registry; includes a "not listed" option with instructions for adding a new owner to `registry.sources`. Also ships: `upgrade` command (binary-only updates, new-settings detection); `↑ update` indicator in `list` and `status`; VERSION column in `status`; cross-platform release workflow (macOS arm64/amd64, Linux amd64/arm64, Windows amd64) with SHA256 checksums; race-safe atomic state writes; timezone-aware Cloud Scheduler (`gcp.scheduler_timezone` + wizard prompt) |
| v0.3.0 | GCP integration: `--secrets-backend gcp` writes credentials to Secret Manager; `install` creates Cloud Run Jobs and Cloud Scheduler triggers; `status` fetches live last-run data from executions API; `run`, `enable`, `disable` manage jobs. `env_var` field added to manifest ConfigField for explicit env var mapping. |
| v0.2.0 | `snipemgr install` end to end: GitHub Releases download, config wizard (huh forms + `--no-interactive` mode), Snipe-IT category management (`categories list`, `categories seed --dry-run`), `settings.yaml` generation, atomic state writes. Also: `uninstall`, `config` (re-run wizard), `● installed` / `○ available` status in `list`. |
| v0.1.0 | `snipemgr list` end to end: GitHub registry discovery (topic `2snipe` + Contents API), manifest validation, lipgloss table in terminal, plain text when piped, state file creation. Manifests shipped for all five initial integrations. |
| v0.0.1 | Bootstrap — runnable `snipemgr` binary with cobra+viper CLI skeleton, all global flags, `PersistentPreRunE` logging init, `snipemgr.yaml` config loading, `fatal()` helper, and version embedding. |

---

## Related repos

- [`jackvaughanjr/1password2snipe`](https://github.com/jackvaughanjr/1password2snipe) — 1Password Business member sync
- [`jackvaughanjr/github2snipe`](https://github.com/jackvaughanjr/github2snipe) — GitHub Enterprise / org member sync
- [`jackvaughanjr/googleworkspace2snipe`](https://github.com/jackvaughanjr/googleworkspace2snipe) — Google Workspace license sync
- [`jackvaughanjr/okta2snipe`](https://github.com/jackvaughanjr/okta2snipe) — Okta member sync
- [`jackvaughanjr/slack2snipe`](https://github.com/jackvaughanjr/slack2snipe) — Slack billable member sync
