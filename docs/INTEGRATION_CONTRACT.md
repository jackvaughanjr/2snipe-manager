# `*2snipe` Integration Contract

This document defines the stable contract between `snipemgr` and any compliant
`*2snipe` integration. It is the authoritative reference for:

- What `snipemgr` will always rely on (the stable surface)
- What constitutes a breaking change in either direction
- When and how to use `min_snipemgr` to signal incompatibility

Anything not listed here is **outside the contract** and may change freely in an
integration without requiring a `min_snipemgr` bump.

---

## Contract Surfaces

### 1. The Manifest (`2snipe.json`)

The manifest must exist at the **repo root** on the default branch. Its absence
silently excludes the repo from `snipemgr` discovery.

#### Required fields

| Field | Type | Description |
|---|---|---|
| `name` | `string` | Repo name (e.g. `claude2snipe`) |
| `display_name` | `string` | Human-readable vendor name |
| `version` | `string` | Integration version (semver) |
| `min_snipemgr` | `string` | Minimum `snipemgr` version required (semver) |
| `commands.sync` | `bool` | Whether the `sync` command is implemented |
| `commands.test` | `bool` | Whether the `test` command is implemented |
| `releases.asset_pattern` | `string` | Binary name pattern in GitHub Releases |
| `config_schema` | `array` | Config fields the install wizard must prompt for |

#### Unknown field behavior

`snipemgr` uses a **tolerant reader** approach: unknown fields in `2snipe.json` are
ignored rather than treated as errors. This allows newer integrations to add optional
fields without breaking older manager versions.

However, any unknown field encountered during manifest parsing emits a **standard-level
warning** (visible without `-v` or `--debug`) so that typos and stale fields don't
silently disappear:

```
WARN  claude2snipe: unrecognized manifest field "min_snipemgr_max" — ignoring
```

This mirrors the behavior for a missing GitHub token in `snipemgr.yaml`.

#### Optional fields

| Field | Type | Description |
|---|---|---|
| `description` | `string` | One-line summary |
| `tags` | `[]string` | Freeform tags (e.g. `saas`, `licensing`, `mdm`) |
| `shared_config` | `[]string` | Config blocks provided by `snipemgr` (e.g. `snipe_it`) |
| `$schema` | `string` | JSON Schema URL for editor validation |

#### `config_schema` entry shape

```json
{
  "key":      "claude.session_key",
  "label":    "Session Cookie (sk-ant-sid...)",
  "secret":   true,
  "required": true,
  "default":  "",
  "hint":     "Copy from browser DevTools → Application → Cookies",
  "env_var":  "CLAUDE_SESSION_KEY"
}
```

Fields `key` and `label` are required per entry. `secret`, `required`, `default`,
`hint`, and `env_var` are optional.

`env_var` overrides the default env var name injected into Cloud Run Jobs (derived
as `strings.ToUpper(strings.ReplaceAll(key, ".", "_"))` when omitted). The
well-known shared keys `snipe_it.url` and `snipe_it.api_key` always map to
`SNIPE_URL` and `SNIPE_TOKEN` respectively, regardless of `env_var`.

---

### 2. CLI Surface

`snipemgr` invokes integration binaries directly. The following must be stable:

#### Commands

| Command | Required | Description |
|---|---|---|
| `sync` | if `commands.sync: true` | Run the sync |
| `test` | if `commands.test: true` | Validate connections, report state |

#### Standard flags (must be accepted without error)

| Flag | Type | Description |
|---|---|---|
| `--dry-run` | `bool` | Simulate without making changes |
| `--force` | `bool` | Re-sync even if already up to date |
| `--email` | `string` | Scope sync to a single user by email |
| `--config` | `string` | Path to config file |
| `--log-format` | `string` | `text` or `json` |
| `--log-file` | `string` | Append logs to a file |
| `-v` / `--verbose` | `bool` | INFO-level logging |
| `-d` / `--debug` | `bool` | DEBUG-level logging |

An integration may add additional flags freely. It must not remove or redefine
any flag listed here.

#### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success — sync or test completed without error |
| non-zero | Failure — `snipemgr` treats any non-zero exit as a failed run |

Exit code semantics beyond this (e.g. `2` = partial failure) are outside the
contract unless a future manager version formalizes them.

---

### 3. Binary Release Artifact Naming

The `releases.asset_pattern` field in `2snipe.json` tells `snipemgr` how to find
the correct binary in a GitHub Release. The pattern supports two substitution tokens:

| Token | Value |
|---|---|
| `{os}` | `linux`, `darwin`, `windows` |
| `{arch}` | `amd64`, `arm64` |

Example: `"claude2snipe_{os}_{arch}"` resolves to `claude2snipe_darwin_arm64` on
Apple Silicon.

**Phase 2 implementation note:** Only plain binary assets are currently supported.
Archives (`.tar.gz`, `.zip`) are not yet handled — asset pattern matching compares
exact filename (with `.exe` appended on Windows). Archive support is planned for
Phase 4.

---

### 4. Configuration File Convention

Integrations read config from `settings.yaml` in the working directory by default,
or from the path provided via `--config`. The `snipe_it` block is always injected
by `snipemgr` from shared config and must not conflict with vendor-specific keys.

The top-level YAML structure is:

```yaml
<vendor>:          # vendor-specific keys from config_schema
  ...

snipe_it:          # always provided by snipemgr via shared_config
  url: ""
  api_key: ""

sync:
  dry_run: false
  rate_limit: true
```

Integrations may add sub-keys freely. They must not rename or remove `snipe_it`
as the Snipe-IT block key.

---

## Breaking Changes Reference

### Integration → Manager (bump `min_snipemgr`)

When an integration makes any of the following changes, it **must** bump
`min_snipemgr` to the first `snipemgr` version that handles the change correctly.
The manager will refuse to install or run the integration if its own version is
below `min_snipemgr`.

| Change | Why it breaks |
|---|---|
| Renaming or removing a required `2snipe.json` field | Manager fails to parse the manifest |
| Changing the `config_schema` entry shape (adding required fields, renaming keys) | Install wizard generates a malformed `settings.yaml` |
| Changing `releases.asset_pattern` format or token names | Manager downloads the wrong binary or fails to find a release |
| Renaming `sync` or `test` to something else | Manager invokes a command that no longer exists |
| Removing a standard flag listed in §2 | Manager passes a flag the binary rejects |
| Changing exit code `0` to mean anything other than success | Manager misreports run status |
| Renaming the `snipe_it` config block | Shared config injection produces an unrecognized key |

### Manager → Integration (coordinated update required)

When `snipemgr` itself makes the following changes, **all existing integrations
may break** and need to be reviewed for compatibility. These changes require a
manager minor or major version bump and a migration note in the `snipemgr`
changelog.

| Change | Why it breaks |
|---|---|
| Removing support for a `2snipe.json` field that older integrations emit | Manager silently drops config or errors on unknown keys |
| Changing how binaries are invoked (new required env vars, changed working directory) | Binary may not find its config or dependencies |
| Changing `shared_config` merging behavior or the `snipe_it` block structure | Injected config no longer matches what the binary expects |
| Changing `asset_pattern` token names (`{os}`, `{arch}`) | Existing manifests resolve to wrong artifact names |
| Adding a required field to `config_schema` entries without a fallback default | Older integrations omitting the field fail validation |

---

## Versioning Rules

- **`version`** in `2snipe.json` — the integration's own release version. Follows
  semver. Bumped on any release.
- **`min_snipemgr`** — the minimum `snipemgr` version this integration requires.
  Only bumped when a breaking change (per the table above) is introduced. Does not
  need to match `version`.
- `snipemgr` itself follows semver. Minor versions may add new optional manifest
  fields or manager features. Major versions may introduce breaking manager-side
  changes (see §Manager → Integration above).

---

## Adding a New Compliant Integration

A repo is considered compliant and will appear in `snipemgr list` when:

1. A valid `2snipe.json` exists at the repo root on the default branch
2. All required manifest fields are present and correctly typed
3. The binary release follows the `asset_pattern` naming convention
4. The binary accepts the standard CLI surface defined in §2

No registration or approval step is required. The manifest is the membership gate.
