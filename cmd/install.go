package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/jackvaughanjr/2snipe-manager/internal/installer"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/snipeit"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/jackvaughanjr/2snipe-manager/internal/wizard"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Install/config command flags — shared by installCmd and configCmd.
var (
	installFlagSnipeURL   string
	installFlagSnipeToken string
	installFlagFields     []string
	installFlagSchedule   string
)

var installCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Download, configure, and install a *2snipe integration",
	Long: `Install a *2snipe integration by name. Downloads the binary for your OS
and arch, runs the configuration wizard (or reads flags in --no-interactive mode),
ensures the Snipe-IT license category exists, and writes settings.yaml.`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	addInstallFlags(installCmd)
}

// addInstallFlags binds the shared install/config flags to cmd.
func addInstallFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&installFlagSnipeURL, "snipe-url", "",
		"Snipe-IT instance URL (sets snipe_it.url config field)")
	cmd.Flags().StringVar(&installFlagSnipeToken, "snipe-token", "",
		"Snipe-IT API key (sets snipe_it.api_key config field)")
	cmd.Flags().StringArrayVar(&installFlagFields, "field", nil,
		"Set a config field value in key=value format (repeatable)")
	cmd.Flags().StringVar(&installFlagSchedule, "schedule", "manual",
		`Sync schedule: cron expression (e.g. "0 6 * * *") or "manual"`)
}

func runInstall(_ *cobra.Command, args []string) error {
	name := args[0]

	// Load registry sources.
	var sources []registry.Source
	if err := viper.UnmarshalKey("registry.sources", &sources); err != nil {
		return fatal("invalid registry.sources config: %v", err)
	}
	if len(sources) == 0 {
		return fatal("registry.sources is empty — set at least one owner in snipemgr.yaml")
	}
	token := viper.GetString("registry.github_token")

	// Load state.
	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}
	s, err := state.ReadState(statePath)
	if err != nil {
		return fatal("reading state: %v", err)
	}

	// Find integration in registry.
	client := registry.NewClient(sources, token)
	integrations, err := client.List(s.InstalledVersions())
	if err != nil {
		return fatal("fetching registry: %v", err)
	}
	target := findIntegration(integrations, name)
	if target == nil {
		return fatal("integration %q not found in registry — run 'snipemgr list' to see available integrations", name)
	}

	// Check if already installed.
	if target.Installed {
		if noInteractive {
			return fatal("integration %q is already installed — use 'snipemgr config %s' to reconfigure", name, name)
		}
		overwrite := false
		confirm := huh.NewConfirm().
			Title(fmt.Sprintf("%s is already installed (v%s). Reconfigure?", name, target.InstalledVersion)).
			Value(&overwrite)
		if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil || !overwrite {
			fmt.Println("Install cancelled.")
			return nil
		}
	}

	// Collect config values.
	var values map[string]string
	schedule := installFlagSchedule

	if noInteractive || !isTerminal() {
		fieldMap, err := buildFieldMap()
		if err != nil {
			return fatal("%v", err)
		}
		values, err = wizard.BuildFlagDefaults(target.Manifest.ConfigSchema, fieldMap)
		if err != nil {
			return fatal("%v", err)
		}
	} else {
		// Populate wizard with any pre-supplied flag values.
		existing := make(map[string]string)
		if fm, err := buildFieldMap(); err == nil {
			for k, v := range fm {
				if v != "" {
					existing[k] = v
				}
			}
		}
		result, err := wizard.RunInteractive(target.Manifest, existing)
		if err != nil {
			return fatal("wizard: %v", err)
		}
		values = result.Values
		schedule = result.Schedule
	}

	// Ensure Snipe-IT category if the manifest specifies one.
	if target.Manifest.Category != "" {
		catURL, catToken := resolveSnipeCredentials(values)
		if catURL != "" && catToken != "" {
			catClient := snipeit.NewClient(catURL, catToken)
			id, err := catClient.EnsureCategory(target.Manifest.Category)
			if err != nil {
				slog.Warn("could not ensure Snipe-IT category",
					"category", target.Manifest.Category, "error", err)
				fmt.Fprintf(os.Stderr, "Warning: could not ensure category %q: %v\n",
					target.Manifest.Category, err)
			} else {
				fmt.Printf("✓ Category '%s' ready (id=%d)\n", target.Manifest.Category, id)
			}
		} else {
			slog.Info("skipping category check — Snipe-IT credentials not configured")
		}
	}

	// Download binary and write settings.yaml.
	binDir := viper.GetString("install.bin_dir")
	if binDir == "" {
		binDir = "~/.snipemgr/bin"
	}
	ins := &installer.Installer{
		BinDir:    binDir,
		ConfigDir: "~/.snipemgr/config",
		Token:     token,
	}
	fmt.Printf("Downloading %s v%s...\n", name, target.Manifest.Version)
	if err := ins.Install(*target, values); err != nil {
		return fatal("install failed: %v", err)
	}

	// Update state.
	if s.Integrations == nil {
		s.Integrations = make(map[string]state.InstalledIntegration)
	}
	s.Integrations[name] = state.InstalledIntegration{
		Version:        target.Manifest.Version,
		Enabled:        true,
		Schedule:       schedule,
		SecretsBackend: "local",
		InstalledAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := state.WriteState(statePath, s); err != nil {
		return fatal("writing state: %v", err)
	}

	fmt.Printf("✓ Installed %s v%s\n", name, target.Manifest.Version)
	binDir, _ = expandHome(binDir)
	fmt.Printf("  Binary:   %s/%s\n", binDir, name)
	configBase, _ := expandHome("~/.snipemgr/config")
	fmt.Printf("  Config:   %s/%s/settings.yaml\n", configBase, name)
	fmt.Printf("  Schedule: %s\n", schedule)
	return nil
}

// buildFieldMap constructs the key→value map from --snipe-url, --snipe-token,
// and --field flags.
func buildFieldMap() (map[string]string, error) {
	m := make(map[string]string)
	for _, kv := range installFlagFields {
		k, v, found := strings.Cut(kv, "=")
		if !found {
			return nil, fmt.Errorf("--field %q: expected key=value format", kv)
		}
		m[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if installFlagSnipeURL != "" {
		m["snipe_it.url"] = installFlagSnipeURL
	}
	if installFlagSnipeToken != "" {
		m["snipe_it.api_key"] = installFlagSnipeToken
	}
	return m, nil
}

// resolveSnipeCredentials returns the Snipe-IT URL and token to use for
// category management. Priority: viper (snipemgr.yaml) > wizard values > flags.
func resolveSnipeCredentials(wizardValues map[string]string) (url, token string) {
	url = viper.GetString("snipe_it.url")
	token = viper.GetString("snipe_it.api_key")
	if url == "" {
		url = wizardValues["snipe_it.url"]
	}
	if token == "" {
		token = wizardValues["snipe_it.api_key"]
	}
	if url == "" {
		url = installFlagSnipeURL
	}
	if token == "" {
		token = installFlagSnipeToken
	}
	return url, token
}

// findIntegration searches integrations by repo name or manifest name.
func findIntegration(integrations []registry.Integration, name string) *registry.Integration {
	for i := range integrations {
		if integrations[i].RepoName == name || integrations[i].Manifest.Name == name {
			return &integrations[i]
		}
	}
	return nil
}
