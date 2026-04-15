# snipemgr — Features Backlog

Items here are **not required for core functionality** and should only be
implemented once Phases 0–4 are complete and working. They are ordered roughly
by value, not by difficulty.

---

## Tier 1 — High value, low complexity (tackle first after core)

### `--json` output on all commands
All commands (`list`, `status`, `install`, `run`, `upgrade`) emit clean JSON
when `--json` is passed. Enables scripting, dashboards, and piping into `jq`.
Particularly useful for `status` — lets other tools check integration health.

### `snipemgr doctor`
Pre-flight check command. Verifies:
- `snipemgr.yaml` is present and all required fields are set
- GCP APIs are enabled (Run, Scheduler, Secret Manager, Artifact Registry)
- Service account exists and has required roles
- Each installed integration's binary is present and executable
- Each installed integration's secrets exist in Secret Manager
- Each installed integration's Cloud Run Job and Scheduler job exist
- GitHub token is configured (warns if not, explains rate limit impact)

Outputs a pass/fail checklist. First command to run when something breaks.

### `snipemgr run --tail`
After triggering a Cloud Run Job execution, stream its logs from Cloud Logging
in real time until the job completes. Shows structured log output from the
integration binary (same format as running it locally with `-v`).

### Shared secret upgrade prompt
When `snipemgr upgrade` detects a new integration version, if the integration's
`config_schema` has new fields not present in the current config, prompt the user
to provide values for the new fields before upgrading.

---

## Tier 2 — Solid features, moderate complexity

### Slack notification on job failure
Configure a Slack webhook in `snipemgr.yaml`. When a Cloud Run Job execution
fails, `snipemgr` posts a notification to the configured channel with the
integration name, failure time, and a link to Cloud Logging.

This runs as a separate lightweight Cloud Run service that subscribes to
Pub/Sub events from Cloud Run Jobs — not as a polling loop.

```yaml
notifications:
  slack_webhook: "https://hooks.slack.com/..."
  on_failure: true
  on_success: false
```

### `snipemgr logs <n>`
Fetch and display the last N log lines from Cloud Logging for a given
integration's most recent execution. Useful for debugging failures without
opening the GCP console.

```bash
snipemgr logs claude2snipe
snipemgr logs claude2snipe --lines 100
snipemgr logs claude2snipe --execution latest-1   # second-most-recent
```

### GCS-backed state file
Allow `snipemgr.yaml` to specify a GCS bucket path instead of
`~/.snipemgr/state.json`. Enables multiple machines or team members to share
the same manager state — everyone sees the same installed integrations and
can trigger or check status from any machine.

```yaml
state:
  gcs_bucket: "gs://your-org-snipemgr/state.json"
```

### `snipemgr list --filter <tag>`
Filter the `list` output by manifest tag. Useful as the integration count grows.

```bash
snipemgr list --filter saas
snipemgr list --filter identity
snipemgr list --filter endpoint
```

Tag taxonomy suggestion for manifests:
- `saas` — SaaS license management (Claude, 1Password, Slack, etc.)
- `identity` — Identity providers (Okta, Google Workspace)
- `endpoint` — Endpoint/device management (Jamf, SentinelOne)
- `devtools` — Developer tools (GitHub, Snyk)
- `productivity` — Productivity suites

### Checksum verification on binary download
During `install` and `upgrade`, verify the SHA-256 checksum of the downloaded
binary against the checksum file published in the GitHub Release. Fail loudly
if verification fails. Adds meaningful security for a supply-chain-sensitive tool.

---

## Tier 3 — Larger scope, implement when there's clear demand

### Web UI (read-only status dashboard)
A lightweight web server (`snipemgr serve`) that renders the `status` command
output as an HTML page. Shows integration health, last-run timestamps, schedules,
and enable/disable status. Read-only — no management actions via the UI.

Useful for sharing status with non-technical stakeholders without GCP console
access. A simple `net/http` server with `html/template` is sufficient — no
framework needed.

### Multi-org / multi-GCP-project support
Configure multiple GCP projects and/or registry sources in `snipemgr.yaml`.
Useful if your-org expands to multiple environments (prod/staging) or if
managing integrations for multiple clients.

```yaml
registry:
  sources:
    - owner: jackvaughanjr
    - owner: some-partner-org   # explicitly trusted external source
```

### Automated Docker image build + push
`snipemgr install` builds the integration binary into a Docker image and pushes
it to Artifact Registry automatically, removing the current manual step.

Requires Docker to be installed and running locally, or an integration with
Cloud Build. Significant complexity — defer until the manual step becomes a
real friction point.

### Terraform output mode
`snipemgr install --terraform-only` generates a Terraform `.tf` file for the
Cloud Run Job and Scheduler resources instead of creating them directly via API.
Useful for teams that manage all GCP resources via Terraform.

### `snipemgr upgrade --all`
Non-interactive mass upgrade of all installed integrations that have newer
versions available. Useful for scripted maintenance. Should require explicit
confirmation or `--yes` flag before proceeding.

### Integration health history
Persist the last N execution results per integration in the state file (or a
separate history file). `snipemgr status --history 7` shows a 7-day run history
per integration. Useful for spotting intermittent failures.

---

## Tier 4 — Nice to have, low priority

### `snipemgr init <vendor>`
Scaffold a new `*2snipe` integration repo from within `snipemgr`. Essentially
wraps the "Starting a new integration" workflow from the parent `CLAUDE.md` into
a single command: creates the GitHub repo, generates `CONTEXT.md`, copies
scaffolding files, and creates a `2snipe.json` template.

### Shell completion
`snipemgr completion bash/zsh/fish` — cobra supports this natively, just needs
to be wired and documented in the README.

### `snipemgr edit <n>`
Open the installed integration's `settings.yaml` in `$EDITOR`. Shortcut for
users who prefer direct config editing over the wizard.

### Integration-level dry-run from manager
`snipemgr run <n> --dry-run` passes the `--dry-run` flag to the integration
binary when triggering the Cloud Run Job. Useful for testing config changes
before a real sync.
