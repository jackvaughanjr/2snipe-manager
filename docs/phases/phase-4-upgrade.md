# Phase 4 — `upgrade` command + release polish

> **Load this file when working on Phase 4.**
> Load completed phase files only if a specific gotcha or deviation is needed.

---

## Goal

Upgrade detection works. The binary is releasable with cross-platform builds and
a complete README.

---

## Required

- [ ] `cmd/upgrade.go` — compare state versions against manifest versions;
      prompt per outdated integration; download + replace binary
- [ ] `snipemgr list` and `snipemgr status` show `↑ update` indicator when
      manifest version > installed version
- [ ] Consistent error handling across all commands (audit `fatal()` usage)
- [ ] README complete: install curl one-liners, first-time setup, all commands
      with examples, how to add `2snipe.json` to a new integration
- [ ] `.github/workflows/release.yml` — cross-platform binaries on `v*` tag
- [ ] `go vet ./...` clean

---

## Optional (defer)

- [ ] `upgrade --all` non-interactive
- [ ] Changelog display from GitHub Release notes

---

## Verification

```bash
go build -o snipemgr . && echo "BUILD OK"
go vet ./...

./snipemgr upgrade --help

# Simulate an update being available
cat ~/.snipemgr/state.json | python3 -c "
import json,sys
s=json.load(sys.stdin)
s['integrations']['<integration-name>']['version']='0.0.1'
print(json.dumps(s,indent=2))
" > /tmp/state_old.json && mv /tmp/state_old.json ~/.snipemgr/state.json

./snipemgr list | grep -i "update\|↑"
# Expected: update indicator visible

# Validate release workflow YAML
cat .github/workflows/release.yml | python3 -c "import sys,yaml; yaml.safe_load(sys.stdin)" && echo "YAML OK"

# Cross-platform build smoke test
GOOS=linux   GOARCH=amd64 go build -o /tmp/snipemgr-linux-amd64   . && echo "LINUX AMD64 OK"
GOOS=darwin  GOARCH=arm64 go build -o /tmp/snipemgr-darwin-arm64  . && echo "DARWIN ARM64 OK"
GOOS=windows GOARCH=amd64 go build -o /tmp/snipemgr-windows-amd64.exe . && echo "WINDOWS AMD64 OK"
```

---

## Go tests

```bash
go test ./... -v -count=1
go test -race ./...
# Expected: all pass, no data races
```

**`cmd/upgrade_test.go`:**
- `TestUpgradeNeeded_OlderInstalled` — `0.0.1` vs `1.2.0` → needs upgrade
- `TestUpgradeNeeded_SameVersion` → no upgrade
- `TestUpgradeNeeded_NewerInstalled` → no upgrade, log warning
