# Phase 0 — Repo Bootstrap ✓ COMPLETE (2026-04-14)

> **This file is archived and frozen.** Phase 0 is complete. Do not modify this
> file. Load it only if a current phase task requires context from Phase 0.

---

## Goal

Empty but runnable Go binary with correct module path, CLI skeleton, and config
loading. No GCP or GitHub calls yet.

---

## Required ✓ all complete

- [x] Create GitHub repo `jackvaughanjr/2snipe-manager` (private)
- [x] `go.mod` — module `github.com/jackvaughanjr/2snipe-manager`, go 1.23 (see Gotchas)
- [x] `main.go` — version embedding pattern (same as integrations)
- [x] `cmd/root.go` — cobra root, viper init, `PersistentPreRunE` logging init,
      `SilenceUsage`/`SilenceErrors`, `fatal()` helper, `--no-interactive` flag,
      `--config` flag pointing to `snipemgr.yaml`
- [x] `.gitignore` — excludes `snipemgr.yaml`, binaries, `.DS_Store`
- [x] `snipemgr.example.yaml` — fully commented config template
- [x] `README.md` — badge row, description, build phases, installation, version history

---

## Choices confirmed

- **Config file name:** `snipemgr.yaml` — distinct from integration `settings.yaml`
  to avoid confusion when both live on the same machine.

---

## Gotchas / deviations from plan

**1. Go version bumped to 1.23 (planned: 1.22)**
`viper v1.21.0` requires `go 1.23.0`. `go get` automatically updated `go.mod` from
`1.22` to `1.23.0`. All code is fully compatible; the version in `go.mod` is the
minimum required, not a constraint on the build machine.

**2. Root command needs a `Run` field to display flags in `--help`**
Cobra suppresses the "Flags:" section when the root command has no `Run`/`RunE`
and no subcommands. Without `Run`, `./snipemgr --help` only printed the `Long`
description. Fixed by adding:
```go
Run: func(cmd *cobra.Command, args []string) {
    _ = cmd.Help()
},
```
This is also correct UX: running `snipemgr` with no subcommand prints help rather
than silently exiting. Once subcommands are added in Phase 1, this `Run` continues
to make sense.

**3. Repo init — `2snipe-manager/` was an untracked subdirectory of `2snipe-config`**
The local `~/Documents/GitHub/` directory is the `2snipe-config` git repo, whose
`.gitignore` uses a `*` catch-all that already excluded `2snipe-manager/`. A fresh
`git init` was run inside `2snipe-manager/` to create an independent repo, then
connected to the new `jackvaughanjr/2snipe-manager` remote. The `claude-code-kickoff.md`
session file was left untracked and is not committed to the repo.

---

## Verification ✓ all passed (2026-04-14)

```bash
go build -o snipemgr . && echo "BUILD OK"        # BUILD OK ✓
./snipemgr --help                                 # shows Usage + all 7 global flags ✓
./snipemgr --version                              # "snipemgr version dev" ✓
./snipemgr --bad-flag 2>&1 | grep -i "unknown"   # "Error: unknown flag: --bad-flag" ✓
./snipemgr --verbose --help                       # no error ✓
./snipemgr --config /tmp/nonexistent.yaml --help  # no panic ✓
go vet ./...                                      # no output, exit 0 ✓
go build ./... 2>&1                               # no output ✓
go test ./... -v                                  # all packages compile ✓
```
