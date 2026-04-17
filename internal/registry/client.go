package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	githubAPIBase = "https://api.github.com"
	manifestFile  = "2snipe.json"
)

// semVerRe matches a bare SemVer string: major.minor.patch with optional
// pre-release (-rc.1) and build metadata (+build.1).
var semVerRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// Client fetches and validates integration manifests from GitHub.
type Client struct {
	sources       []Source
	token         string
	httpClient    *http.Client
	manifestCache map[string]*Manifest // key: "owner/repo"
}

// NewClient creates a registry Client for the given sources and optional GitHub
// token. Pass an empty token for unauthenticated access (60 req/hr limit).
func NewClient(sources []Source, token string) *Client {
	return &Client{
		sources:       sources,
		token:         token,
		httpClient:    &http.Client{},
		manifestCache: make(map[string]*Manifest),
	}
}

// List returns all integrations discoverable from the configured sources,
// cross-referenced against installed. installed is a map of repo name to
// installed version (from state.State.InstalledVersions).
func (c *Client) List(installed map[string]string) ([]Integration, error) {
	var integrations []Integration

	for _, src := range c.sources {
		repos, err := c.searchRepos(src.Owner)
		if err != nil {
			return nil, fmt.Errorf("searching repos for %s: %w", src.Owner, err)
		}

		for _, repo := range repos {
			manifest, err := c.fetchManifest(src.Owner, repo.Name, repo.DefaultBranch)
			if err != nil {
				slog.Debug("skipping repo: manifest fetch failed", "repo", repo.FullName, "err", err)
				continue
			}
			if err := ValidateManifest(*manifest); err != nil {
				slog.Debug("skipping repo: manifest invalid", "repo", repo.FullName, "err", err)
				continue
			}
			if manifest.Name != repo.Name {
				slog.Debug("skipping repo: manifest name mismatch",
					"manifest_name", manifest.Name, "repo_name", repo.Name)
				continue
			}

			intg := Integration{
				Manifest:      *manifest,
				RepoName:      repo.Name,
				Owner:         src.Owner,
				RepoURL:       repo.HTMLURL,
				DefaultBranch: repo.DefaultBranch,
			}
			if v, ok := installed[repo.Name]; ok {
				intg.Installed = true
				intg.InstalledVersion = v
				intg.UpdateAvail = CompareVersions(v, manifest.Version) < 0
			}
			integrations = append(integrations, intg)
		}
	}

	return integrations, nil
}

// ValidateManifest checks that m satisfies all required fields and constraints.
// Returns a non-nil error describing the first violation found.
func ValidateManifest(m Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("name is empty")
	}
	if m.DisplayName == "" {
		return fmt.Errorf("display_name is empty")
	}
	if m.Description == "" {
		return fmt.Errorf("description is empty")
	}
	if m.Version == "" {
		return fmt.Errorf("version is empty")
	}
	if !semVerRe.MatchString(m.Version) {
		return fmt.Errorf("version %q is not a valid semver string", m.Version)
	}
	if len(m.ConfigSchema) == 0 {
		return fmt.Errorf("config_schema is empty")
	}
	for i, f := range m.ConfigSchema {
		if f.Key == "" {
			return fmt.Errorf("config_schema[%d].key is empty", i)
		}
		if f.Label == "" {
			return fmt.Errorf("config_schema[%d].label is empty", i)
		}
	}
	if !m.Releases.GitHubReleases {
		return fmt.Errorf("releases.github_releases must be true")
	}
	if !strings.Contains(m.Releases.AssetPattern, "{os}") ||
		!strings.Contains(m.Releases.AssetPattern, "{arch}") {
		return fmt.Errorf("releases.asset_pattern must contain {os} and {arch}")
	}
	return nil
}

// ---- GitHub API types -------------------------------------------------------

type ghRepo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
}

type ghSearchResult struct {
	TotalCount int      `json:"total_count"`
	Items      []ghRepo `json:"items"`
}

type ghContents struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// ---- internal helpers -------------------------------------------------------

func (c *Client) searchRepos(owner string) ([]ghRepo, error) {
	url := fmt.Sprintf("%s/search/repositories?q=user:%s+topic:2snipe&per_page=100",
		githubAPIBase, owner)
	var result ghSearchResult
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	slog.Debug("github search", "owner", owner, "total_count", result.TotalCount)
	return result.Items, nil
}

func (c *Client) fetchManifest(owner, repo, branch string) (*Manifest, error) {
	cacheKey := owner + "/" + repo
	if m, ok := c.manifestCache[cacheKey]; ok {
		return m, nil
	}

	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		githubAPIBase, owner, repo, manifestFile)
	var contents ghContents
	if err := c.get(url, &contents); err != nil {
		return nil, err
	}
	if contents.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", contents.Encoding)
	}
	// GitHub base64 content includes newlines — strip them before decoding.
	raw, err := base64.StdEncoding.DecodeString(
		strings.ReplaceAll(contents.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	c.manifestCache[cacheKey] = &m
	return &m, nil
}

// CompareVersions compares two semver strings (major.minor.patch with optional
// pre-release/build suffixes, which are ignored). Returns -1 if a < b, 0 if
// a == b, 1 if a > b. Malformed or empty strings are treated as 0.0.0.
func CompareVersions(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := range 3 {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// parseSemver extracts the [major, minor, patch] integers from a semver string.
// Pre-release labels (-rc.1) and build metadata (+build.1) are stripped before
// parsing.
func parseSemver(v string) [3]int {
	v = strings.SplitN(v, "-", 2)[0]
	v = strings.SplitN(v, "+", 2)[0]
	parts := strings.SplitN(v, ".", 3)
	var nums [3]int
	for i := range 3 {
		if i < len(parts) {
			nums[i], _ = strconv.Atoi(parts[i])
		}
	}
	return nums
}

func (c *Client) get(url string, dst any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, dst)
}
