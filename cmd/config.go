package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jackvaughanjr/2snipe-manager/internal/installer"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/snipeit"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/jackvaughanjr/2snipe-manager/internal/wizard"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config <name>",
	Short: "Re-run the configuration wizard for an installed integration",
	Long:  `Reconfigure an installed integration by re-running the wizard or passing updated flags.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	// Reuse the same flag set as install so --help shows identical options.
	addInstallFlags(configCmd)
}

func runConfig(_ *cobra.Command, args []string) error {
	name := args[0]

	// Load state to confirm the integration is installed.
	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}
	s, err := state.ReadState(statePath)
	if err != nil {
		return fatal("reading state: %v", err)
	}
	if _, ok := s.Integrations[name]; !ok {
		return fatal("integration %q is not installed — run 'snipemgr install %s' first", name, name)
	}

	// Fetch manifest from registry to get config_schema.
	var sources []registry.Source
	if err := viper.UnmarshalKey("registry.sources", &sources); err != nil {
		return fatal("invalid registry.sources config: %v", err)
	}
	if len(sources) == 0 {
		return fatal("registry.sources is empty — set at least one owner in snipemgr.yaml")
	}
	token := viper.GetString("registry.github_token")

	client := registry.NewClient(sources, token)
	integrations, err := client.List(s.InstalledVersions())
	if err != nil {
		return fatal("fetching registry: %v", err)
	}
	target := findIntegration(integrations, name)
	if target == nil {
		return fatal("integration %q not found in registry", name)
	}

	// Collect new config values.
	var values map[string]string
	schedule := installFlagSchedule
	if schedule == "" || schedule == "manual" {
		if existing, ok := s.Integrations[name]; ok && existing.Schedule != "" {
			schedule = existing.Schedule
		}
	}

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
		result, err := wizard.RunInteractive(target.Manifest, nil)
		if err != nil {
			return fatal("wizard: %v", err)
		}
		values = result.Values
		schedule = result.Schedule
	}

	// Ensure Snipe-IT category if needed.
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
		}
	}

	// Re-write settings.yaml.
	binDir := viper.GetString("install.bin_dir")
	if binDir == "" {
		binDir = "~/.snipemgr/bin"
	}
	ins := &installer.Installer{
		BinDir:    binDir,
		ConfigDir: "~/.snipemgr/config",
		Token:     token,
	}
	if err := ins.Install(*target, values); err != nil {
		return fatal("reconfigure failed: %v", err)
	}

	// Update schedule in state.
	entry := s.Integrations[name]
	entry.Schedule = schedule
	s.Integrations[name] = entry
	if err := state.WriteState(statePath, s); err != nil {
		return fatal("writing state: %v", err)
	}

	fmt.Printf("✓ Reconfigured %s\n", name)
	return nil
}
