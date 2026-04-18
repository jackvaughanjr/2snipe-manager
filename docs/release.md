# Releases and versioning

> **Note:** This document covers releases for `*2snipe` **integration** repos
> (e.g. `github2snipe`, `okta2snipe`). The `snipemgr` release workflow is more
> involved — it includes macOS code signing and notarization. See
> `.github/workflows/release.yml` in this repo for that workflow.

---

## How releases work

Releases are created by GitHub Actions on a `v*` tag push. The workflow:
1. Builds four platform binaries with the version embedded via ldflags.
2. Creates a GitHub release with auto-generated notes from commits.
3. Attaches all four binaries as release assets.

To cut a new release:
```bash
git tag v1.2.3
git push origin v1.2.3
```

The workflow can also be triggered manually from the Actions tab (useful for
re-releasing a tag or testing the workflow).

---

## Versioning convention

All `*2snipe` integrations use [semver](https://semver.org) with the following rules:

- **First production commit = v1.0.0.** All integrations are deployed and used before
  a v1.0.0 tag, so the first working commit counts as v1.0.0 regardless of maturity.
- **Increment minor** (`v1.X.0`) for new features: new flags, new commands, new config
  keys that change sync behaviour, new auto-create patterns.
- **Increment patch** (`v1.0.X`) for bug fixes, dependency updates, and documentation
  changes.
- **Never auto-shrink** patch numbers for doc-only commits — each commit to `main`
  that ships should increment either minor or patch.

Every integration README must include a `## Version History` table. Format:

```markdown
## Version History

| Version | Key changes |
|---------|-------------|
| v1.2.0 | Added `--create-users` flag to automatically provision missing Snipe-IT accounts |
| v1.1.0 | ... |
| v1.0.0 | Initial release — ... |
```

List versions newest-first. Keep each row to one sentence or a short comma-separated
list of the key changes. Update this table in the same commit as the version tag.

---

## Release workflow — .github/workflows/release.yml

Replace `<repo>` with your binary name:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      tag:
        description: "Tag to release (e.g. v1.0.0)"
        required: true

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          ref: ${{ github.event.inputs.tag || github.ref }}

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build binaries
        run: |
          VERSION="${{ github.event.inputs.tag || github.ref_name }}"
          LDFLAGS="-s -w -X main.version=${VERSION}"
          mkdir -p dist
          GOOS=darwin  GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o "dist/<repo>-darwin-arm64"       .
          GOOS=linux   GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "dist/<repo>-linux-amd64"        .
          GOOS=linux   GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o "dist/<repo>-linux-arm64"        .
          GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "dist/<repo>-windows-amd64.exe"  .
          cd dist && sha256sum * > checksums.txt

      - name: Create release
        uses: softprops/action-gh-release@v2
        with:
          tag_name: ${{ github.event.inputs.tag || github.ref_name }}
          generate_release_notes: true
          files: dist/*
```

Key points:
- `permissions: contents: write` is required for the release step.
- `-s -w` strips debug info and symbol tables — reduces binary size significantly.
- `-X main.version=${VERSION}` embeds the tag name into the binary.
- `generate_release_notes: true` uses GitHub's automatic release notes from commits.
- `workflow_dispatch` with a tag input allows manual re-releases.
- Integration binaries run as Docker containers in Cloud Run — macOS signing is
  not needed. The simple Ubuntu workflow above is sufficient.

---

## README badge row

Every integration README must include a badge row immediately after the `# Title` heading.
Replace `<owner>` and `<repo>` with the actual values:

```markdown
[![Latest Release](https://img.shields.io/github/v/release/<owner>/<repo>)](https://github.com/<owner>/<repo>/releases/latest) [![Go Version](https://img.shields.io/github/go-mod/go-version/<owner>/<repo>)](go.mod) [![License](https://img.shields.io/github/license/<owner>/<repo>)](LICENSE) [![Build](https://github.com/<owner>/<repo>/actions/workflows/release.yml/badge.svg)](https://github.com/<owner>/<repo>/actions/workflows/release.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/<owner>/<repo>)](https://goreportcard.com/report/github.com/<owner>/<repo>) [![Downloads](https://img.shields.io/github/downloads/<owner>/<repo>/total)](https://github.com/<owner>/<repo>/releases)
```

Badges included:
- **Latest Release** — current version tag from GitHub releases
- **Go Version** — pulled automatically from `go.mod`
- **License** — pulled from the repo's license file
- **Build** — status of the `release.yml` workflow
- **Go Report Card** — static analysis score (requires a one-time visit to goreportcard.com to trigger the first scan)
- **Downloads** — total binary downloads across all releases

---

## README installation section

Every integration README must include an Installation section with curl commands.
Pattern (replace `<repo>` and `<owner>`):

```markdown
## Installation

**Download a pre-built binary** from the [latest release](https://github.com/<owner>/<repo>/releases/latest):

    # macOS (Apple Silicon)
    curl -L https://github.com/<owner>/<repo>/releases/latest/download/<repo>-darwin-arm64 -o <repo>
    chmod +x <repo>

    # Linux (amd64)
    curl -L https://github.com/<owner>/<repo>/releases/latest/download/<repo>-linux-amd64 -o <repo>
    chmod +x <repo>

    # Linux (arm64)
    curl -L https://github.com/<owner>/<repo>/releases/latest/download/<repo>-linux-arm64 -o <repo>
    chmod +x <repo>

Or build from source:

    git clone https://github.com/<owner>/<repo>
    cd <repo>
    go build -o <repo> .
```
