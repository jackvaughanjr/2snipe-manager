# Phase 4 — `upgrade` command + release polish ✓ COMPLETE (2026-04-17)

> **This file is archived and frozen.** Phase 4 is complete. Do not modify this
> file. Load it only if a current task requires context from Phase 4.

---

## Goal

Upgrade detection works. Binary is releasable with cross-platform builds and a
complete README.

---

## Required ✓ all complete

- [x] `cmd/upgrade.go` — compare state versions against manifest versions;
      prompt per outdated integration; download + replace binary only
- [x] `snipemgr list` and `snipemgr status` show `↑ update` indicator when
      manifest version > installed version; `status` adds a VERSION column
- [x] Consistent error handling audit across all `cmd/*.go` (grep pass)
- [x] README: `upgrade` marked ✓, all commands ✓, v1.0.0 version history,
      Phase 4 row marked ✓, "under active development" banner removed
- [x] `.github/workflows/release.yml` — cross-platform binaries on `v*` tag
- [x] `go vet ./...` clean

---

## Optional

- [x] `upgrade --all` — non-interactive bulk upgrade (implemented, not deferred)
- [ ] Changelog display from GitHub Release notes (deferred to post-v1.0.0)

---

## Choices confirmed

**Choice A — Upgrade behavior: binary-only with smart settings check**

`upgrade` replaces the binary and does not touch `settings.yaml`. After
upgrading, it reads the new manifest's `config_schema` and checks each field's
last key segment against the existing `settings.yaml` text:
- New fields found → warning listing each field by label and key; "Run
  `snipemgr config <n>` to configure these fields."
- No new fields → simpler note: "Run `snipemgr config <n>` to update settings
  if needed."

Implemented in `cmd/upgrade.go` as `checkNewSettings()` and `printSettingsNote()`.

**Choice B — Error handling audit: grep only**

Grepped all `cmd/*.go` for bare `return err` and `return fmt.Errorf` on
runtime paths. All bare returns were either:
- In private helper functions (errors propagate up and are wrapped in `fatal()`
  at the `RunE` call site — correct per CLAUDE.md), or
- Preceded by an explicit `fmt.Fprintf(os.Stderr, ...)` before returning
  (also correct).

No code changes needed.

---

## Gotchas / deviations from plan

**1. Duplicate `status` variable in `renderLipglossTable` (list.go)**
When adding `UpdateAvail` support to the styled list renderer, a redundant
`status` declaration was left in the loop alongside the new one. Fixed by
removing the old declaration and keeping the new one that checks
`intg.UpdateAvail`.

**2. Race condition in `internal/state/writeState` (pre-existing, fixed here)**
The race detector revealed a genuine bug: all concurrent goroutines wrote to
the same `path + ".tmp"` filename, so two overlapping writes could corrupt the
JSON file. The `TestWriteState_ConcurrentSafe` test was flaky (passed in
isolation, failed under `-race`). Fixed by switching to
`os.CreateTemp(dir, "state-*.tmp")` which generates a unique filename per
write; `os.Rename` is still used for the final atomic swap. The bug was latent
since Phase 1.

**3. `golang.org/x/mod/semver` is not a dependency**
The kickoff suggested `golang.org/x/mod/semver` for version comparison. It is
not in `go.mod` (not a transitive dep). Rather than add a dependency,
`CompareVersions` was implemented as a simple integer-split comparison:
strip pre-release/build metadata, parse `major.minor.patch` as integers, and
compare pairwise. This is sufficient for the bare semver format that
`ValidateManifest` already enforces (no `v` prefix).

**4. `status` command needed a VERSION column for the `↑` indicator**
Phase 4 requires both `list` and `status` to show the `↑ update` indicator.
`status` previously had no version column. Added VERSION between ENABLED and
SCHEDULE in both the lipgloss (terminal) and tabwriter (piped) renderers.
Registry fetch in `status` is optional and graceful: if `registry.sources` is
unconfigured or the fetch fails, the column still shows the installed version
from state without the `↑`.

**5. `upgrade --all` + `--no-interactive` interaction**
`--no-interactive` disables interactive prompts globally. When set without
`--all`, `upgrade` shows available updates but exits with a hint rather than
prompting or upgrading — the expected behaviour for scripted/piped contexts.
`--all` bypasses prompting regardless of `--no-interactive`.

**6. Test count discrepancy vs Phase 3**
Phase 3 recorded 37 tests; Phase 4 starts from 32 (pre-Phase-4 run). The 5-
test gap is unexplained — no tests were intentionally removed. Phase 4 adds 3
new tests (`cmd/upgrade_test.go`) for a final count of 35. The state package
also now reliably passes `TestWriteState_ConcurrentSafe` (previously flaky,
now deterministic after the race fix in gotcha #2).

---

## Verification ✓ all passed (2026-04-17)

```bash
go build -o snipemgr . && echo "BUILD OK"
# Result: BUILD OK ✓

go vet ./...
# Result: no output, exit 0 ✓

go test ./... -v -count=1
# Result: 35 tests, 0 failures ✓

go test -race ./...
# Result: all packages clean, 0 data races ✓

./snipemgr upgrade --help
# Result: shows Usage, --all flag, global flags ✓

GOOS=linux   GOARCH=amd64 go build -o /tmp/snipemgr-linux-amd64   . && echo "LINUX AMD64 OK"
GOOS=darwin  GOARCH=arm64 go build -o /tmp/snipemgr-darwin-arm64  . && echo "DARWIN ARM64 OK"
GOOS=windows GOARCH=amd64 go build -o /tmp/snipemgr-windows-amd64.exe . && echo "WINDOWS AMD64 OK"
# Result: all three ✓
```

---

## Go tests ✓ all passed (2026-04-17)

35 tests, 0 failures.

**`cmd/upgrade_test.go`** (3 new tests):
- `TestUpgradeNeeded_OlderInstalled` — installed `0.0.1` vs manifest `1.2.0` → needs upgrade ✓
- `TestUpgradeNeeded_SameVersion` — installed == manifest → no upgrade ✓
- `TestUpgradeNeeded_NewerInstalled` — installed `1.3.0` vs manifest `1.2.0` → no upgrade, warning ✓

**`internal/installer/installer_test.go`** (4 tests, unchanged):
- `TestResolveAssetURL_Darwin_ARM64` ✓
- `TestResolveAssetURL_Linux_AMD64` ✓
- `TestResolveAssetURL_Windows` ✓
- `TestWriteSettingsYAML` ✓

**`internal/registry/client_test.go`** (6 tests, unchanged):
- `TestValidateManifest_Valid` ✓
- `TestValidateManifest_MissingName` ✓
- `TestValidateManifest_MissingVersion` ✓
- `TestValidateManifest_MissingConfigSchema` ✓
- `TestValidateManifest_BadAssetPattern` ✓
- `TestValidateManifest_BadVersion` ✓

**`internal/scheduler/gcp_test.go`** (5 tests, unchanged):
- `TestMockScheduler_CreateAndDelete` ✓
- `TestBuildCloudRunJobSpec_EnvVars` ✓
- `TestBuildCloudRunJobSpec_ExplicitEnvVar` ✓
- `TestBuildSchedulerJobSpec_CronPassthrough` ✓
- `TestImagePath` ✓

**`internal/secrets/manager_test.go`** (4 tests, unchanged):
- `TestMockSecretManager_SetAndGet` ✓
- `TestMockSecretManager_Exists_Missing` ✓
- `TestMockSecretManager_Overwrite` ✓
- `TestMockSecretManager_ListByPrefix` ✓

**`internal/snipeit/categories_test.go`** (4 tests, unchanged):
- `TestDefaultCategories_Count` ✓
- `TestEnsureCategory_EmptyName` ✓
- `TestCreateCategory_EnvelopeUnwrap` ✓
- `TestSeedDefaults_Idempotent` ✓

**`internal/state/store_test.go`** (6 tests; `ConcurrentSafe` now reliable after race fix):
- `TestReadState_Missing` ✓
- `TestReadState_Empty` ✓
- `TestReadState_Valid` ✓
- `TestWriteState_Atomic` ✓
- `TestWriteState_RoundTrip` ✓
- `TestWriteState_ConcurrentSafe` ✓

**`internal/wizard/wizard_test.go`** (3 tests, unchanged):
- `TestBuildFlagDefaults` ✓
- `TestBuildFlagDefaults_Default` ✓
- `TestBuildFlagDefaults_MissingRequired` ✓

---

## Files changed in Phase 4

| File | Change |
|------|--------|
| `internal/registry/types.go` | Added `UpdateAvail bool` to `Integration` |
| `internal/registry/client.go` | Added `CompareVersions`, `parseSemver`; set `UpdateAvail` in `List()` |
| `internal/state/store.go` | Fixed race: `os.CreateTemp` instead of fixed `.tmp` filename |
| `internal/installer/installer.go` | Added `UpgradeBinary` method (binary-only, no `settings.yaml`) |
| `cmd/list.go` | Appends `↑ update` to status cell when `UpdateAvail` |
| `cmd/status.go` | Added VERSION column; optional registry fetch for update indicators |
| `cmd/upgrade.go` | **New** — upgrade command with `--all`, interactive prompt, new-settings check |
| `cmd/upgrade_test.go` | **New** — 3 tests for `upgradeNeeded` |
| `.github/workflows/release.yml` | **New** — cross-platform release on `v*` tag |
| `README.md` | `upgrade` ✓; Phase 4 row ✓; v1.0.0 version history; status banner removed |
| `docs/order-of-operations.md` | Phase 4 marked ✓ COMPLETE |
| `docs/phases/phase-4-upgrade.md` | **Renamed** → `docs/phases/phase-4-complete.md` |
