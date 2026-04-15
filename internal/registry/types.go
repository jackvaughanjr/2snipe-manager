package registry

// Manifest is the parsed content of a 2snipe.json file at the root of an
// integration repo. Its presence signals that the repo is built to the
// *2snipe standard and is managed by snipemgr.
type Manifest struct {
	Schema       string        `json:"$schema"`
	Name         string        `json:"name"`
	DisplayName  string        `json:"display_name"`
	Description  string        `json:"description"`
	Version      string        `json:"version"`
	MinSnipemgr  string        `json:"min_snipemgr"`
	Tags         []string      `json:"tags"`
	Category     string        `json:"category"`
	ConfigSchema []ConfigField `json:"config_schema"`
	SharedConfig []string      `json:"shared_config"`
	Commands     Commands      `json:"commands"`
	Releases     Releases      `json:"releases"`
}

// ConfigField is one entry in the manifest's config_schema array.
// It drives the install wizard — snipemgr has no hardcoded knowledge of any
// integration's config fields.
type ConfigField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Secret   bool   `json:"secret"`
	Required bool   `json:"required"`
	Default  string `json:"default"`
	Hint     string `json:"hint"`
}

// Commands declares which standard commands the integration binary supports.
type Commands struct {
	Sync bool `json:"sync"`
	Test bool `json:"test"`
}

// Releases describes how to locate the integration binary in GitHub Releases.
type Releases struct {
	GitHubReleases bool   `json:"github_releases"`
	AssetPattern   string `json:"asset_pattern"`
}

// Integration combines a validated manifest with discovery metadata and state.
type Integration struct {
	Manifest         Manifest
	RepoName         string
	Owner            string
	RepoURL          string
	DefaultBranch    string
	Installed        bool
	InstalledVersion string
}

// Source is one entry in the registry.sources config list.
type Source struct {
	Owner string `mapstructure:"owner"`
}
