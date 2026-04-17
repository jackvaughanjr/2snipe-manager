package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
)

// Asset is one downloadable file in a GitHub release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Release is a GitHub release with its list of assets.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// ResolveAssetURL finds the download URL in a release asset list for the given
// asset_pattern, GOOS, and GOARCH. Returns the URL and true if a match is found.
func ResolveAssetURL(assets []Asset, pattern, goos, goarch string) (string, bool) {
	name := resolveAssetName(pattern, goos, goarch)
	for _, a := range assets {
		if a.Name == name {
			return a.DownloadURL, true
		}
	}
	return "", false
}

// resolveAssetName substitutes {os} and {arch} in pattern and appends .exe on Windows.
func resolveAssetName(pattern, goos, goarch string) string {
	name := strings.ReplaceAll(pattern, "{os}", goos)
	name = strings.ReplaceAll(name, "{arch}", goarch)
	if goos == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}
	return name
}

// FetchLatestRelease fetches the latest GitHub release for owner/repo.
func FetchLatestRelease(owner, repo, token string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d for %s/%s releases", resp.StatusCode, owner, repo)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}
	return &rel, nil
}

// Installer downloads integration binaries and writes settings.yaml skeletons.
type Installer struct {
	BinDir    string // e.g. ~/.snipemgr/bin (configurable via snipemgr.yaml)
	ConfigDir string // e.g. ~/.snipemgr/config
	Token     string // GitHub API token for downloading private releases
}

// Install downloads the integration binary for the current OS/arch and writes a
// settings.yaml skeleton populated with the provided values.
func (ins *Installer) Install(intg registry.Integration, values map[string]string) error {
	binDir, err := expandPath(ins.BinDir)
	if err != nil {
		return fmt.Errorf("expanding bin dir: %w", err)
	}
	configDir, err := expandPath(ins.ConfigDir)
	if err != nil {
		return fmt.Errorf("expanding config dir: %w", err)
	}

	// Fetch latest release from GitHub.
	rel, err := FetchLatestRelease(intg.Owner, intg.RepoName, ins.Token)
	if err != nil {
		return fmt.Errorf("fetching release for %s: %w", intg.RepoName, err)
	}

	// Resolve the asset URL for this platform.
	assetURL, ok := ResolveAssetURL(
		rel.Assets,
		intg.Manifest.Releases.AssetPattern,
		runtime.GOOS,
		runtime.GOARCH,
	)
	if !ok {
		return fmt.Errorf("no release asset found for %s/%s matching pattern %q",
			runtime.GOOS, runtime.GOARCH, intg.Manifest.Releases.AssetPattern)
	}

	// Create bin dir and download the binary.
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("creating bin dir: %w", err)
	}
	binPath := filepath.Join(binDir, intg.RepoName)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if err := downloadFile(assetURL, binPath, ins.Token); err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		return fmt.Errorf("chmod +x binary: %w", err)
	}

	// Create per-integration config dir and write settings.yaml.
	integConfigDir := filepath.Join(configDir, intg.RepoName)
	if err := os.MkdirAll(integConfigDir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	settingsPath := filepath.Join(integConfigDir, "settings.yaml")
	content := buildSettingsYAML(intg.Manifest.ConfigSchema, values)
	if err := os.WriteFile(settingsPath, content, 0600); err != nil {
		return fmt.Errorf("writing settings.yaml: %w", err)
	}

	return nil
}

// UpgradeBinary downloads and replaces the integration binary for the current
// OS/arch. Unlike Install, it does not touch settings.yaml — the user's existing
// configuration is preserved. Returns the new version string (bare semver, e.g.
// "1.2.0") as found in the GitHub release tag.
func (ins *Installer) UpgradeBinary(intg registry.Integration) (string, error) {
	binDir, err := expandPath(ins.BinDir)
	if err != nil {
		return "", fmt.Errorf("expanding bin dir: %w", err)
	}

	rel, err := FetchLatestRelease(intg.Owner, intg.RepoName, ins.Token)
	if err != nil {
		return "", fmt.Errorf("fetching release for %s: %w", intg.RepoName, err)
	}

	assetURL, ok := ResolveAssetURL(
		rel.Assets,
		intg.Manifest.Releases.AssetPattern,
		runtime.GOOS,
		runtime.GOARCH,
	)
	if !ok {
		return "", fmt.Errorf("no release asset found for %s/%s matching pattern %q",
			runtime.GOOS, runtime.GOARCH, intg.Manifest.Releases.AssetPattern)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("creating bin dir: %w", err)
	}
	binPath := filepath.Join(binDir, intg.RepoName)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if err := downloadFile(assetURL, binPath, ins.Token); err != nil {
		return "", fmt.Errorf("downloading binary: %w", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		return "", fmt.Errorf("chmod +x binary: %w", err)
	}

	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// BuildSettingsYAML generates a commented YAML settings file from the config schema
// and provided values. Exported so cmd/install.go can call it for display/debug.
func BuildSettingsYAML(schema []registry.ConfigField, values map[string]string) []byte {
	return buildSettingsYAML(schema, values)
}

// buildSettingsYAML generates a nested YAML file matching the viper key layout.
// Fields are grouped by the top-level section of their dotted key.
func buildSettingsYAML(schema []registry.ConfigField, values map[string]string) []byte {
	var sb strings.Builder
	sb.WriteString("# Generated by snipemgr install — edit as needed\n\n")

	type group struct {
		name   string
		fields []registry.ConfigField
	}

	var groups []group
	seen := map[string]int{}

	for _, f := range schema {
		parts := strings.SplitN(f.Key, ".", 2)
		gName := ""
		if len(parts) == 2 {
			gName = parts[0]
		}
		idx, ok := seen[gName]
		if !ok {
			groups = append(groups, group{name: gName})
			idx = len(groups) - 1
			seen[gName] = idx
		}
		groups[idx].fields = append(groups[idx].fields, f)
	}

	for _, g := range groups {
		if g.name != "" {
			sb.WriteString(g.name + ":\n")
		}
		for _, f := range g.fields {
			val := values[f.Key]
			if val == "" {
				val = f.Default
			}

			subKey := f.Key
			indent := ""
			if g.name != "" {
				parts := strings.SplitN(f.Key, ".", 2)
				if len(parts) == 2 {
					subKey = parts[1]
				}
				indent = "  "
			}

			if f.Hint != "" {
				sb.WriteString(fmt.Sprintf("%s# %s\n", indent, f.Hint))
			}
			sb.WriteString(fmt.Sprintf("%s%s: %q\n", indent, subKey, val))
		}
		sb.WriteString("\n")
	}

	return []byte(sb.String())
}

func downloadFile(url, path, token string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
