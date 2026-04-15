# Phase 2 — `install` command (local mode) + category management

> **Load this file when working on Phase 2.**
> If information needed to complete a task is missing here, check completed phase
> files for relevant gotchas. Do not load Phase 3 or Phase 4 files.

---

## Goal

`snipemgr install <n>` downloads the binary, runs the config wizard, ensures the
integration's Snipe-IT category exists, and writes a local `settings.yaml`. No GCP
yet — secrets backend is local file only.

---

## Required

- [ ] `internal/installer/installer.go`
  - Resolve binary download URL from manifest `releases.asset_pattern` + GitHub
    Releases API
  - Download to `~/.snipemgr/bin/{name}` (create dir if needed)
  - Make downloaded binary executable (`chmod +x`)
  - Write `settings.yaml` skeleton to `~/.snipemgr/config/{name}/settings.yaml`
- [ ] `internal/snipeit/categories.go`
  - `Client` struct (Snipe-IT base URL + API key, stdlib `net/http`)
  - `ListCategories() ([]Category, error)` — `GET /api/v1/categories?limit=500`
  - `CreateCategory(name string) (int, error)` — `POST /api/v1/categories`,
    type `"license"`, unwrap `payload` envelope
  - `EnsureCategory(name string) (int, error)` — GET then POST if missing;
    returns 0 + warning (not error) if name is empty
  - `SeedDefaults() error` — calls `EnsureCategory` for each entry in
    `DefaultCategories`; skips existing silently; non-fatal on individual failures
  - `DefaultCategories` — package-level `[]string` with the ten default
    categories (see `docs/architecture.md` for the full list)
- [ ] `internal/wizard/wizard.go`
  - First-time setup detection: if `snipemgr.yaml` missing required fields, run
    setup wizard before integration install; collect Snipe-IT URL + API key,
    GitHub token; offer to seed default categories before exiting setup
  - Config form driven entirely by manifest `config_schema`
  - Shared config reuse prompt (if `shared_config` prefix already has values)
  - If `manifest.category` is set: call `EnsureCategory` after collecting
    Snipe-IT credentials; log `✓ Category '<n>' ready`
  - TTY detection: fall back to flag input when `--no-interactive` or piped
  - Password masking for `secret: true` fields
- [ ] `internal/state/store.go` — add write support (atomic write via tmp+rename)
- [ ] `cmd/install.go`
  - Accept integration name as positional arg
  - Flag-based equivalents for all wizard fields (for `--no-interactive` use)
  - Graceful handling of already-installed: prompt to reconfigure or abort
- [ ] `cmd/categories.go` — `categories list` and `categories seed` subcommands;
      `seed` supports `--dry-run`
- [ ] `cmd/config.go` — re-run wizard for an installed integration
- [ ] `cmd/uninstall.go` — remove binary, config dir, state entry (local only)
- [ ] `go vet ./...` clean

---

## Optional (defer)

- [ ] SHA-256 checksum verification of downloaded binary
- [ ] Rollback on partial install failure
- [ ] `categories seed --json` output for scripted use

---

## Choices — confirm before coding

- **Binary install location:**
  Option A: `~/.snipemgr/bin/` (default, configurable)
  Option B: `/usr/local/bin/` (system-wide, requires sudo)
  Option C: Configurable in `snipemgr.yaml`, default `~/.snipemgr/bin/`
  **Recommended: Option C.**

- **Config storage location:**
  Option A: `~/.snipemgr/config/{name}/settings.yaml`
  Option B: `~/.snipemgr/config/settings.{name}.yaml`
  **Recommended: Option A.**

---

## Verification

```bash
go build -o snipemgr . && echo "BUILD OK"
go vet ./...

# install help
./snipemgr install --help
# Expected: usage, positional arg, all wizard field flags listed

# Non-interactive install using a real integration with a 2snipe.json
./snipemgr install <integration-name> \
  --no-interactive \
  --snipe-url "https://snipe.example.com" \
  --snipe-token "fake-token" \
  --schedule manual
  # add --<field-key> flags per the integration's config_schema
# Expected: binary downloaded, settings.yaml written, state updated, no panic

ls -la ~/.snipemgr/bin/<integration-name>
# Expected: file exists, +x permission

cat ~/.snipemgr/config/<integration-name>/settings.yaml
# Expected: YAML with passed values, no empty required fields

cat ~/.snipemgr/state.json | python3 -m json.tool
# Expected: entry present with correct version and installed_at

./snipemgr list | grep -i "installed"
# Expected: "● installed" shown for the integration

./snipemgr install <integration-name> --no-interactive 2>&1
# Expected: "already installed" message, not a panic

./snipemgr uninstall <integration-name> --no-interactive
ls ~/.snipemgr/bin/<integration-name> 2>&1 | grep "No such file"
ls ~/.snipemgr/config/<integration-name> 2>&1 | grep "No such file"
# Expected: both absent; state.json no longer contains the entry

# categories
./snipemgr categories --help
./snipemgr categories seed --dry-run
# Expected: lists DefaultCategories, marks each [exists] or [would create], no writes

./snipemgr categories seed
./snipemgr categories seed   # idempotent — second run produces no changes

./snipemgr categories list
# Expected: table of Snipe-IT license categories

# install auto-ensures category (requires integration with category in 2snipe.json)
./snipemgr install <integration-name> --no-interactive ...flags...
# Expected: "✓ Category '<category>' ready" in output
```

---

## Go tests

```bash
go test ./internal/... -v
```

**`internal/installer/installer_test.go`:**
- `TestResolveAssetURL_Darwin_ARM64`
- `TestResolveAssetURL_Linux_AMD64`
- `TestResolveAssetURL_Windows` — appends `.exe`
- `TestWriteSettingsYAML` — all config_schema keys present in output YAML

**`internal/state/store_test.go`:**
- `TestWriteState_Atomic`
- `TestWriteState_RoundTrip`
- `TestWriteState_ConcurrentSafe`

**`internal/wizard/wizard_test.go`:**
- `TestBuildFlagDefaults`
- `TestBuildFlagDefaults_MissingRequired`

**`internal/snipeit/categories_test.go`:**
- `TestDefaultCategories_Count` — exactly 10 entries, none empty
- `TestEnsureCategory_EmptyName` — returns 0, no error
- `TestCreateCategory_EnvelopeUnwrap` — unwraps `payload`, returns correct ID
- `TestSeedDefaults_Idempotent` — no duplicate POSTs on second call

```bash
go test ./... -v
# Expected: all tests pass
```
