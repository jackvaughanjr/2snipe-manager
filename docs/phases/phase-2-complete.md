# Phase 2 — `install` command (local mode) + category management ✓ COMPLETE (2026-04-15)

> **This file is archived and frozen.** Phase 2 is complete. Do not modify this
> file. Load it only if a current phase task requires context from Phase 2.

---

## Goal

`snipemgr install <n>` downloads the binary, runs the config wizard, ensures the
integration's Snipe-IT category exists, and writes a local `settings.yaml`. No GCP
— secrets backend is local file only.

---

## Required ✓ all complete

- [x] `internal/installer/installer.go` — `ResolveAssetURL`, `FetchLatestRelease`,
      `Installer.Install`, `buildSettingsYAML`
- [x] `internal/snipeit/categories.go` — `Client`, `DefaultCategories` (10 entries),
      `ListCategories`, `CreateCategory`, `EnsureCategory`, `SeedDefaults`
- [x] `internal/wizard/wizard.go` — `Result`, `BuildFlagDefaults`, `RunInteractive`
      (huh forms); `charmbracelet/huh v1.0.0` added to go.mod
- [x] `internal/state/store.go` — `WriteState` exported; `writeState` made atomic
      (write to `.tmp`, rename into place)
- [x] `cmd/install.go` — `--snipe-url`, `--snipe-token`, `--field key=value`
      (repeatable), `--schedule`; already-installed guard
- [x] `cmd/categories.go` — `categories list`, `categories seed`, `seed --dry-run`
- [x] `cmd/config.go` — re-runs wizard; same flags as install
- [x] `cmd/uninstall.go` — removes binary, config dir, state entry
- [x] `cmd/list.go` — updated to show `● installed` / `○ available` STATUS column
- [x] `go vet ./...` clean

---

## Choices confirmed

- **Binary install location:** Option C — `install.bin_dir` in `snipemgr.yaml`,
  default `~/.snipemgr/bin/`
- **Config storage location:** Option A — `~/.snipemgr/config/{name}/settings.yaml`

---

## Gotchas / deviations from plan

**1. Non-interactive field input uses `--field key=value` (not `--<field-key>` flags)**
The manifest's `config_schema` is dynamic — you can't pre-define per-field cobra
flags before fetching the manifest. Instead, the install command uses a repeatable
`--field key=value` flag for arbitrary config fields, plus two static convenience
shortcuts: `--snipe-url` (sets `snipe_it.url`) and `--snipe-token` (sets
`snipe_it.api_key`). These cover the most common shared fields; everything else
uses `--field`.

**2. Snipe-IT GET /api/v1/categories returns HTML-encoded names**
The API returns `&amp;` instead of `&` (and similar entities) in category names.
`EnsureCategory` compared against plain names and never matched, so every category
appeared missing and every creation attempt failed (already exists under the
decoded name). Fixed by calling `html.UnescapeString` on each category name after
decoding the list response. The test mock returns plain names and is unaffected.

**3. `categories seed` command uses list-first pattern (not `SeedDefaults` method)**
`SeedDefaults()` calls `EnsureCategory` per category (one GET + one possible POST
each). The CLI command instead fetches all categories once, checks membership, then
calls `CreateCategory` only for missing ones. This avoids printing `✓ name` for
categories that were found-not-created, so the second run correctly says "All default
categories already exist — nothing to do." `SeedDefaults()` is still useful for
programmatic callers (e.g., the wizard's first-time setup offer).

**4. `list` command STATUS column replaces INSTALLED column**
The previous INSTALLED column showed the installed version or "-". Updated to show
`● installed` / `○ available` to match the architecture doc and make
`grep -i installed` meaningful on integration rows (not just the header).

**5. `charmbracelet/huh v1.0.0` brought in several new transitive deps**
(`bubbletea`, `bubbles`, `catppuccin/go`, etc.). All approved per CONTEXT.md.

**6. `resolveSnipeCredentials` priority: viper > wizard values > flags**
For the Snipe-IT category check in `install`, credentials are resolved in this
order so that `snipemgr.yaml` values take precedence, avoiding the need to pass
`--snipe-url`/`--snipe-token` when they're already configured globally.

---

## Verification ✓ all passed (2026-04-15)

```bash
go build -o snipemgr . && echo "BUILD OK"   # ✓
go vet ./...                                 # ✓
go test ./... -v                             # 28 tests, 0 failures ✓

./snipemgr install --help                   # all flags shown ✓
./snipemgr categories --help                # list + seed subcommands shown ✓
./snipemgr categories seed --help           # --dry-run flag shown ✓
./snipemgr config --help                    # same flags as install ✓
./snipemgr uninstall --help                 # usage shown ✓

# Non-interactive install (github2snipe v1.6.0, darwin/arm64)
./snipemgr install github2snipe \
  --no-interactive \
  --snipe-url "https://snipe.example.com" \
  --snipe-token "fake-token" \
  --field "github.token=ghp_test123" \
  --field "snipe_it.license_category_id=5" \
  --schedule manual
# "✓ Category 'Developer Tools & Hosting' ready (id=15)" ✓
# "✓ Installed github2snipe v1.6.0" ✓

ls -la ~/.snipemgr/bin/github2snipe         # -rwxr-xr-x ✓
cat ~/.snipemgr/config/github2snipe/settings.yaml
# all config_schema keys present with hints as comments ✓

cat ~/.snipemgr/state.json | python3 -m json.tool
# github2snipe entry with version 1.6.0 and installed_at ✓

./snipemgr list --no-interactive 2>/dev/null | grep github
# "GitHub   ● installed  1.6.0  ..." ✓

./snipemgr install github2snipe --no-interactive 2>&1
# "already installed — use 'snipemgr config'" ✓

./snipemgr uninstall github2snipe --no-interactive
# binary, config dir, state entry all absent ✓

./snipemgr categories seed --dry-run
# all 10 DefaultCategories shown as [exists] or [would create] ✓

./snipemgr categories seed && ./snipemgr categories seed
# first: creates missing / warns on failures; second: "nothing to do" ✓

./snipemgr categories list
# table of Snipe-IT license categories ✓
```

---

## Go tests ✓ all passed (2026-04-15)

```
go test ./... -v
# 28 tests, 0 failures ✓
```

**`internal/installer/installer_test.go`** (4 tests):
- `TestResolveAssetURL_Darwin_ARM64` ✓
- `TestResolveAssetURL_Linux_AMD64` ✓
- `TestResolveAssetURL_Windows` ✓
- `TestWriteSettingsYAML` ✓

**`internal/state/store_test.go`** (6 tests, 3 new):
- `TestWriteState_Atomic` ✓
- `TestWriteState_RoundTrip` ✓
- `TestWriteState_ConcurrentSafe` ✓

**`internal/wizard/wizard_test.go`** (3 tests):
- `TestBuildFlagDefaults` ✓
- `TestBuildFlagDefaults_Default` ✓
- `TestBuildFlagDefaults_MissingRequired` ✓

**`internal/snipeit/categories_test.go`** (4 tests):
- `TestDefaultCategories_Count` ✓
- `TestEnsureCategory_EmptyName` ✓
- `TestCreateCategory_EnvelopeUnwrap` ✓
- `TestSeedDefaults_Idempotent` ✓
