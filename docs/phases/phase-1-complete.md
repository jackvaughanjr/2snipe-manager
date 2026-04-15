# Phase 1 — Registry + `list` command ✓ COMPLETE (2026-04-14)

> **This file is archived and frozen.** Phase 1 is complete. Do not modify this
> file. Load it only if a current phase task requires context from Phase 1.

---

## Goal

`snipemgr list` works end to end — hits GitHub, validates manifests, renders a
table of available integrations.

---

## Required ✓ all complete

- [x] `internal/registry/types.go` — `Manifest`, `ConfigField`, `Commands`,
      `Releases`, `Integration`, `Source` structs
- [x] `2snipe.schema.json` — JSON Schema (draft-07) for manifest validation;
      referenced via `$schema` in every `2snipe.json`
- [x] `internal/registry/client.go`
  - GitHub repo search by owner + topic filter (`topic:2snipe`) via GitHub
    Search API; `per_page=100`
  - Fetch `2snipe.json` from each repo via GitHub Contents API
    (`/repos/{owner}/{repo}/contents/2snipe.json`) — see Gotchas
  - Validate manifest via `ValidateManifest(Manifest) error` (exported);
    struct field checks, no external schema dep
  - Return `[]Integration` with `Installed`/`InstalledVersion` cross-referenced
    against state
  - Session-level in-memory cache of manifest fetches (map on Client struct)
  - `NewClient(sources []Source, token string) *Client`
- [x] `internal/state/store.go` — `ReadState(path string) (*State, error)`;
      creates empty state file (and `~/.snipemgr/` dir) if missing;
      `InstalledVersions() map[string]string` helper on State
- [x] `cmd/list.go` — renders lipgloss table in terminal; plain `tabwriter`
      output when `--no-interactive` or stdout is not a TTY
- [x] `go vet ./...` clean

---

## Optional (deferred to Phase 3+)

- [ ] `--filter <tag>` flag
- [ ] `--json` output flag

---

## Choices confirmed

- **Manifest validation:** `ValidateManifest(Manifest) error` — exported function,
  struct field checks. No `gojsonschema` dependency. The `2snipe.schema.json` file
  exists for editor tooling only; Go validation is struct-based.

- **GitHub search filter:** Topic `2snipe` + manifest presence gate. Topic is the
  explicit opt-in signal; manifest presence (valid `2snipe.json` at repo root) is
  the secondary confirmation. Repos failing either check are silently excluded
  (debug-logged with reason).

---

## Gotchas / deviations from plan

**1. GitHub Contents API used instead of raw.githubusercontent.com**
The architecture doc specified fetching `2snipe.json` via
`https://raw.githubusercontent.com/{owner}/{repo}/main/2snipe.json`. We used the
GitHub Contents API instead: `GET /repos/{owner}/{repo}/contents/2snipe.json`.
Reason: same auth token as the search call, works with private repos, and the
base64-encoded response body is trivially decoded. The raw URL approach requires
a second auth domain and fails on private repos with unauthenticated requests.

**2. `google/go-github` not added — stdlib `net/http` used throughout**
The architecture doc and README listed `google/go-github` as a planned dependency.
In practice, we needed only two GitHub API endpoints (search and contents), both
straightforward JSON GET calls. Per CLAUDE.md, stdlib is preferred where practical.
The registry client uses `net/http` + `encoding/json` directly. `google/go-github`
remains appropriate if Phase 3+ work requires a richer GitHub surface; add it then.

**3. `v` prefix rejected by SemVer validation**
`semVerRe` in `client.go` matches `^\d+\.\d+\.\d+` — bare semver only. `v1.0.0`
fails validation. All integration manifests must use bare semver (`1.0.0`), which
matches the format in GitHub Release tags once the leading `v` is stripped. The
regex is in `internal/registry/client.go:18` — single change point if needed.

**4. Rate limit warning goes to stderr via `fmt.Fprintln`, not `slog`**
The warning is user-facing and actionable ("add a token"), not a debug log.
Routing through `slog.Warn` would be suppressed at the default log level; routing
through `slog.Warn` and setting the default to WARN would work but is fragile.
Printed unconditionally to stderr before any API calls begin.

**5. Integration `.gitignore` files were blocking `2snipe.json`**
Three of the four new integration repos (`github2snipe`, `googleworkspace2snipe`,
`slack2snipe`) had `*.json` in `.gitignore` to exclude service account key files.
A `!2snipe.json` negation was added to each `.gitignore` before committing the
manifest. `okta2snipe` did not have this issue.

**6. `1password2snipe` `asset_pattern` used underscores (existing typo)**
The manifest committed in a prior session had `"1password2snipe_{os}_{arch}"` but
the release workflow produces `1password2snipe-darwin-arm64` (dashes). Fixed to
`"1password2snipe-{os}-{arch}"`. All four new manifests use dashes from the start.

**7. State file creation is gated on config validation passing**
`ReadState` is called after config validation (sources check). With no valid
`snipemgr.yaml`, the command fails at "registry.sources is empty" before
`ReadState` is ever reached — the state file is not created on a first run with
a missing config. The state creation verification test requires a valid config.

---

## Verification ✓ all passed (2026-04-14)

```bash
# Build and vet clean
go build -o snipemgr . && echo "BUILD OK"   # BUILD OK ✓
go vet ./...                                 # no output, exit 0 ✓

# list command visible in root help
./snipemgr --help | grep list               # "  list   List available integrations..." ✓

# list command has own help with all global flags
./snipemgr list --help                      # shows usage + all 7 global flags ✓

# Live registry run (snipemgr.yaml configured, owner: jackvaughanjr)
./snipemgr list -v
# Rate limit warning printed; 5 integrations discovered ✓
# 1Password, Google Workspace, GitHub, Okta, Slack all appear

# Piped output is plain text, no ANSI codes
./snipemgr list --no-interactive 2>/dev/null | cat
# tabwriter table, no escape sequences ✓

# Missing config produces clear error, not panic
./snipemgr --config /tmp/does-not-exist.yaml list 2>&1
# "Error: registry.sources is empty — set at least one owner in snipemgr.yaml" ✓
# Exit 1 ✓

# Missing state file is created automatically (requires valid config)
rm -f ~/.snipemgr/state.json && ./snipemgr list 2>/dev/null
ls ~/.snipemgr/state.json && echo "STATE FILE CREATED"
# STATE FILE CREATED ✓
```

---

## Go tests ✓ all passed (2026-04-14)

```bash
go test ./internal/... -v
# 12 tests, 0 failures, 0 skipped ✓
```

**`internal/registry/client_test.go`** (9 tests across 5 functions):
- `TestValidateManifest_Valid` ✓
- `TestValidateManifest_MissingName` ✓
- `TestValidateManifest_MissingVersion` ✓
- `TestValidateManifest_MissingConfigSchema` ✓
- `TestValidateManifest_BadAssetPattern` (3 subtests: missing {os}, missing {arch}, missing both) ✓
- `TestValidateManifest_BadVersion` (4 subtests: letters only, missing patch, v prefix, empty) ✓

**`internal/state/store_test.go`** (3 tests):
- `TestReadState_Missing` ✓
- `TestReadState_Empty` ✓
- `TestReadState_Valid` ✓
