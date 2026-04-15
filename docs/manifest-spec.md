# snipemgr — Integration Manifest Specification (`2snipe.json`)

## Purpose

The `2snipe.json` file is the **opt-in gate** for the `snipemgr` registry. A repo
is only discoverable and installable by `snipemgr` if this file is present at the
repo root and passes validation. Its presence signals: "this integration is built
to the `*2snipe` standard and is managed by `snipemgr`."

The file also drives the install wizard — `snipemgr` has no hardcoded knowledge
of any integration's config fields. All prompts come from `config_schema`.

---

## Schema location

The JSON Schema that validates this file is hosted at:
```
https://raw.githubusercontent.com/jackvaughanjr/2snipe-manager/main/2snipe.schema.json
```

Include `$schema` in every manifest to get editor autocomplete/validation.

---

## Full spec

```json
{
  "$schema": "https://raw.githubusercontent.com/jackvaughanjr/2snipe-manager/main/2snipe.schema.json",

  "name": "1password2snipe",
  "display_name": "1Password",
  "description": "Sync 1Password Business members to Snipe-IT license seats",
  "version": "1.1.0",
  "min_snipemgr": "0.1.0",
  "tags": ["saas", "identity", "password-manager"],
  "category": "Identity & Access Management",

  "config_schema": [
    {
      "key": "onepassword.url",
      "label": "1Password SCIM Bridge URL",
      "secret": false,
      "required": true,
      "hint": "e.g. https://your-scim-bridge.example.com"
    },
    {
      "key": "onepassword.api_token",
      "label": "1Password SCIM Bearer Token",
      "secret": true,
      "required": true,
      "hint": "Admin Console → Integrations → SCIM Bridge"
    },
    {
      "key": "snipe_it.url",
      "label": "Snipe-IT URL",
      "secret": false,
      "required": true,
      "hint": "e.g. https://snipe.example.com"
    },
    {
      "key": "snipe_it.api_key",
      "label": "Snipe-IT API Key",
      "secret": true,
      "required": true,
      "hint": "Admin → API Keys in your Snipe-IT instance"
    },
    {
      "key": "snipe_it.license_category_id",
      "label": "Snipe-IT License Category ID",
      "secret": false,
      "required": true,
      "hint": "Find at Admin → Categories (integer ID)"
    },
    {
      "key": "snipe_it.license_name",
      "label": "Snipe-IT License Name",
      "secret": false,
      "required": false,
      "default": "1Password Business",
      "hint": "Name of the license record in Snipe-IT; created automatically on first run"
    },
    {
      "key": "onepassword.include_guests",
      "label": "Include Guests",
      "secret": false,
      "required": false,
      "default": "false",
      "hint": "Sync Guest-role members (billed separately from full members)"
    },
    {
      "key": "slack.webhook_url",
      "label": "Slack Webhook URL",
      "secret": true,
      "required": false,
      "hint": "Optional. Incoming webhook for sync notifications"
    }
  ],

  "shared_config": ["snipe_it"],

  "commands": {
    "sync": true,
    "test": true
  },

  "releases": {
    "github_releases": true,
    "asset_pattern": "1password2snipe_{os}_{arch}"
  }
}
```

---

## Field reference

### Top-level fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `$schema` | string | recommended | URL to the JSON Schema — enables editor validation |
| `name` | string | **yes** | Must match the repo name exactly |
| `display_name` | string | **yes** | Human-readable name shown in `snipemgr list` |
| `description` | string | **yes** | One-line description shown in `snipemgr list` |
| `version` | string | **yes** | SemVer (e.g. `1.2.0`) — must match the latest GitHub Release tag |
| `min_snipemgr` | string | no | Minimum `snipemgr` version required to install this integration |
| `tags` | []string | no | Used for filtering in `snipemgr list --filter <tag>` |
| `category` | string | no | Snipe-IT license category this integration's licenses belong to — see below |
| `config_schema` | []ConfigField | **yes** | Drives the install wizard — see below |
| `shared_config` | []string | no | Config key prefixes that are shared across integrations |
| `commands` | Commands | no | Declares which standard commands the binary supports |
| `releases` | Releases | **yes** | How to find the binary in GitHub Releases |

### `category`

The name of the Snipe-IT license category this integration's licenses should be
assigned to. Must match a category name exactly as it appears (or should appear)
in Snipe-IT.

During `snipemgr install`, if the named category does not exist in Snipe-IT,
it is created automatically before the license is assigned. This means categories
are always in a consistent state after install.

During first-time setup, `snipemgr` offers to seed all known categories from its
default list in one operation — see `docs/architecture.md` for the default list
and seeding behavior.

**Recommended category values** (from the built-in default list):

| Category | Integrations |
|----------|-------------|
| `AI Tools` | *(no current integrations)* |
| `Identity & Access Management` | 1password2snipe, okta2snipe, googleworkspace2snipe |
| `Developer Tools & Hosting` | github2snipe |
| `Communication & Collaboration` | slack2snipe |
| `Endpoint Management & Security` | sentinelone2snipe, jamf2snipe |
| `Productivity` | (future integrations) |
| `Project & Knowledge Management` | (future integrations) |
| `Design & Creative` | (future integrations) |
| `Training & Learning` | (future integrations) |
| `Misc Software` | fallback for uncategorized integrations |

If `category` is omitted from the manifest, the license is created in Snipe-IT
without a category assignment. A warning is logged but install proceeds normally.

### `config_schema` — ConfigField

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `key` | string | **yes** | Dot-notation config key matching `settings.yaml` structure |
| `label` | string | **yes** | Human-readable prompt label |
| `secret` | bool | no | If true, value is masked in wizard and stored in Secret Manager |
| `required` | bool | no | If true, wizard rejects empty input |
| `default` | string | no | Pre-filled value in wizard input |
| `hint` | string | no | Help text shown below the input |

**Config key convention:**
Keys use dot notation matching the `settings.yaml` YAML path. Examples:
- `onepassword.api_token` → `settings.yaml` path `onepassword.api_token`
- `snipe_it.license_category_id` → nested YAML key

`snipemgr` uses these keys to both generate the `settings.yaml` skeleton and to
know which Secret Manager secret name to use (last segment of the key, kebab-cased).

### `shared_config`

List of key prefixes that represent config blocks shared between integrations.
When a prefix is listed here (e.g. `"snipe_it"`), the wizard will:
1. Check if any secret with that prefix already exists in Secret Manager
2. If yes, offer to reuse existing values instead of prompting again

All integrations should include `"snipe_it"` in `shared_config`.

### `releases`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `github_releases` | bool | **yes** | Must be `true` — only GitHub Releases supported |
| `asset_pattern` | string | **yes** | Asset filename template with `{os}` and `{arch}` tokens |

**`asset_pattern` tokens:**
- `{os}` → `darwin`, `linux`, `windows`
- `{arch}` → `amd64`, `arm64`

Example: `"1password2snipe_{os}_{arch}"` resolves to `1password2snipe_darwin_arm64` on
an Apple Silicon Mac. The `.exe` extension is appended automatically on Windows.

This pattern must match the asset names produced by the integration's
`.github/workflows/release.yml` (the standard release workflow from `docs/release.md`
in the parent `CLAUDE.md` already follows this convention).

---

## Validation rules

`snipemgr` validates manifests with these checks (in order):

1. File is valid JSON
2. `name` is non-empty and matches the GitHub repo name
3. `display_name`, `description`, `version` are non-empty
4. `version` is a valid SemVer string
5. `config_schema` is a non-empty array
6. Each `ConfigField` has non-empty `key` and `label`
7. `releases.github_releases` is `true`
8. `releases.asset_pattern` contains both `{os}` and `{arch}` tokens
9. If `category` is present, it must be a non-empty string

Manifests failing any check are silently excluded from `snipemgr list`.
With `--debug`, a reason is logged.

---

## Adding `2snipe.json` to a new integration

1. Copy the example above into the repo root as `2snipe.json`
2. Fill in all required fields
3. Set `category` to the appropriate Snipe-IT category name (see recommended
   values above)
4. Add all integration-specific config keys to `config_schema`
5. Always include `"snipe_it"` in `shared_config`
6. Set `releases.asset_pattern` to match the release workflow's asset naming
7. Add the GitHub topic `2snipe` to the repo (Settings → Topics)
8. Commit and push — the integration is now discoverable by `snipemgr list`

---

## Versioning the manifest spec

The `$schema` URL is pinned to `main` — when the schema changes, all existing
manifests referencing it will be validated against the new schema. Breaking
changes to required fields must be avoided. New optional fields are always safe
to add.

The `min_snipemgr` field exists for cases where a manifest uses features that
older `snipemgr` versions don't understand. Increment it only when necessary.
