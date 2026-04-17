package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackvaughanjr/2snipe-manager/internal/installer"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var upgradeAll bool

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Check for and apply newer versions of installed integrations",
	Long: `Compare installed integration versions against the registry and upgrade
any that have a newer version available.

In interactive mode (default), you are prompted before each upgrade.
Pass --all to upgrade all outdated integrations without prompting.
Pass --no-interactive to list available upgrades without applying them.`,
	RunE: silentUsage(runUpgrade),
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().BoolVar(&upgradeAll, "all", false, "upgrade all outdated integrations without prompting")
}

func runUpgrade(_ *cobra.Command, _ []string) error {
	var sources []registry.Source
	if err := viper.UnmarshalKey("registry.sources", &sources); err != nil {
		return fatal("invalid registry.sources config: %v", err)
	}
	if len(sources) == 0 {
		return fatal("registry.sources is empty — set at least one owner in snipemgr.yaml")
	}

	token := viper.GetString("registry.github_token")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Warning: no GitHub token configured — unauthenticated rate limit is 60 req/hr.")
	}

	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}
	s, err := state.ReadState(statePath)
	if err != nil {
		return fatal("reading state: %v", err)
	}

	if len(s.Integrations) == 0 {
		fmt.Println("No integrations installed. Run 'snipemgr list' to see available integrations.")
		return nil
	}

	client := registry.NewClient(sources, token)
	integrations, err := client.List(s.InstalledVersions())
	if err != nil {
		return fatal("fetching registry: %v", err)
	}

	// Collect integrations that need upgrading.
	var outdated []registry.Integration
	for _, intg := range integrations {
		if intg.UpdateAvail {
			outdated = append(outdated, intg)
		}
		// Warn when installed version is ahead of manifest (unusual).
		if intg.Installed {
			_, warn := upgradeNeeded(intg.InstalledVersion, intg.Manifest.Version)
			if warn != "" {
				fmt.Fprintf(os.Stderr, "Warning: %s — %s\n", intg.RepoName, warn)
			}
		}
	}

	if len(outdated) == 0 {
		fmt.Println("All installed integrations are up to date.")
		return nil
	}

	fmt.Printf("%d update(s) available:\n\n", len(outdated))
	for _, intg := range outdated {
		fmt.Printf("  %-30s %s → %s\n", intg.RepoName, intg.InstalledVersion, intg.Manifest.Version)
	}
	fmt.Println()

	if noInteractive && !upgradeAll {
		fmt.Println("Run 'snipemgr upgrade --all' to apply all updates.")
		return nil
	}

	binDir := viper.GetString("install.bin_dir")
	if binDir == "" {
		binDir = "~/.snipemgr/bin"
	}
	configDir := viper.GetString("install.config_dir")
	if configDir == "" {
		configDir = "~/.snipemgr/config"
	}
	expandedConfigDir, err := expandHome(configDir)
	if err != nil {
		return fatal("expanding config dir: %v", err)
	}

	ins := &installer.Installer{
		BinDir: binDir,
		Token:  token,
	}

	upgraded := 0
	for _, intg := range outdated {
		if !upgradeAll && !noInteractive {
			confirmed, err := promptYesNo(fmt.Sprintf("Upgrade %s from %s to %s?", intg.RepoName, intg.InstalledVersion, intg.Manifest.Version))
			if err != nil {
				return fatal("reading input: %v", err)
			}
			if !confirmed {
				fmt.Printf("  Skipping %s\n", intg.RepoName)
				continue
			}
		}

		fmt.Printf("Upgrading %s...\n", intg.RepoName)
		newVersion, err := ins.UpgradeBinary(intg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: upgrading %s: %v\n", intg.RepoName, err)
			continue
		}

		// Update state with the new version.
		entry := s.Integrations[intg.RepoName]
		entry.Version = newVersion
		s.Integrations[intg.RepoName] = entry
		if writeErr := state.WriteState(statePath, s); writeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not update state for %s: %v\n", intg.RepoName, writeErr)
		}

		fmt.Printf("  ✓ %s upgraded to v%s\n", intg.RepoName, newVersion)
		upgraded++

		// Check for new config fields in the upgraded manifest.
		printSettingsNote(expandedConfigDir, intg)
	}

	if upgraded == 0 {
		fmt.Println("No integrations were upgraded.")
	} else {
		fmt.Printf("\n%d integration(s) upgraded.\n", upgraded)
	}
	return nil
}

// upgradeNeeded reports whether the installed version is older than the manifest
// version. Returns (needed, warning) — warning is non-empty when the installed
// version is ahead of the manifest (unexpected).
func upgradeNeeded(installedVersion, manifestVersion string) (bool, string) {
	cmp := registry.CompareVersions(installedVersion, manifestVersion)
	if cmp < 0 {
		return true, ""
	}
	if cmp > 0 {
		return false, fmt.Sprintf("installed version %s is ahead of manifest version %s — this is unexpected; check the manifest",
			installedVersion, manifestVersion)
	}
	return false, ""
}

// printSettingsNote checks the integration's settings.yaml for config fields
// from the new manifest that are not yet present. Prints a targeted note or
// warning depending on whether new fields were found.
func printSettingsNote(configDir string, intg registry.Integration) {
	newFields := checkNewSettings(configDir, intg.RepoName, intg.Manifest.ConfigSchema)
	if len(newFields) > 0 {
		fmt.Printf("  Warning: %s has new config fields not in your settings.yaml:\n", intg.RepoName)
		for _, f := range newFields {
			fmt.Printf("    • %s (%s)\n", f.Label, f.Key)
		}
		fmt.Printf("  Run 'snipemgr config %s' to configure these fields.\n", intg.RepoName)
	} else {
		fmt.Printf("  Run 'snipemgr config %s' to update settings if needed.\n", intg.RepoName)
	}
}

// checkNewSettings reads the integration's settings.yaml and returns config
// fields from schema that appear to be absent. Uses the key's last path segment
// to match against the generated settings.yaml format.
func checkNewSettings(configDir, repoName string, schema []registry.ConfigField) []registry.ConfigField {
	settingsPath := filepath.Join(configDir, repoName, "settings.yaml")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil // settings.yaml doesn't exist or can't be read — skip check
	}
	content := string(data)

	var missing []registry.ConfigField
	for _, f := range schema {
		parts := strings.Split(f.Key, ".")
		segment := parts[len(parts)-1]
		// buildSettingsYAML writes nested keys as "  segment: value" and
		// top-level keys as "segment: value".
		var pattern string
		if len(parts) > 1 {
			pattern = "  " + segment + ":"
		} else {
			pattern = segment + ":"
		}
		if !strings.Contains(content, pattern) {
			missing = append(missing, f)
		}
	}
	return missing
}

// promptYesNo prints a [y/N] prompt and returns true if the user types y or Y.
func promptYesNo(question string) (bool, error) {
	fmt.Printf("  %s [y/N] ", question)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
