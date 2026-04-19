package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/jackvaughanjr/2snipe-manager/internal/scheduler"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var uninstallCmd = &cobra.Command{
	Use:               "uninstall <name>",
	Short:             "Remove an installed integration (binary, config, GCP resources, and state entry)",
	ValidArgsFunction: integrationNameCompletion,
	Args:              cobra.ExactArgs(1),
	RunE:              silentUsage(runUninstall),
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(_ *cobra.Command, args []string) error {
	name := args[0]

	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}
	s, err := state.ReadState(statePath)
	if err != nil {
		return fatal("reading state: %v", err)
	}

	intg, ok := s.Integrations[name]
	if !ok {
		return fatal("integration %q is not installed", name)
	}

	// Confirm in interactive mode.
	if !noInteractive && isTerminal() {
		msg := fmt.Sprintf("Uninstall %s? This removes the binary, config, and state entry.", name)
		if intg.SecretsBackend == "gcp" {
			msg = fmt.Sprintf("Uninstall %s? This removes the binary, config, GCP Cloud Run Job, Cloud Scheduler trigger, and state entry.", name)
		}
		confirmed := false
		confirm := huh.NewConfirm().
			Title(msg).
			Value(&confirmed)
		if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil || !confirmed {
			fmt.Println("Uninstall cancelled.")
			return nil
		}
	}

	// Delete GCP resources if this integration used the GCP backend.
	if intg.SecretsBackend == "gcp" && intg.CloudRunJob != "" {
		project := viper.GetString("gcp.project")
		region := viper.GetString("gcp.region")
		credFile := viper.GetString("gcp.credentials_file")
		if region == "" {
			region = "us-central1"
		}
		if project == "" {
			fmt.Fprintf(os.Stderr, "Warning: gcp.project not set — skipping GCP resource deletion\n")
		} else {
			fmt.Printf("Deleting GCP resources for %s...\n", name)
			ctx := context.Background()
			sched, err := scheduler.NewGCPScheduler(ctx, credFile)
			if err != nil {
				warnGCP("could not connect to GCP — skipping resource deletion", err)
			} else {
				defer sched.Close()
				if err := sched.DeleteJob(ctx, name, project, region); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not delete GCP resources: %v\n", err)
				} else {
					fmt.Printf("✓ Cloud Run Job and Scheduler trigger deleted\n")
				}
			}
		}
	}

	// Remove binary.
	binDir := viper.GetString("install.bin_dir")
	if binDir == "" {
		binDir = "~/.snipemgr/bin"
	}
	binDir, err = expandHome(binDir)
	if err != nil {
		return fatal("expanding bin dir: %v", err)
	}
	binPath := filepath.Join(binDir, name)
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: could not remove binary %s: %v\n", binPath, err)
	}

	// Remove config directory.
	configBase, err := expandHome("~/.snipemgr/config")
	if err != nil {
		return fatal("expanding config dir: %v", err)
	}
	integConfigDir := filepath.Join(configBase, name)
	if err := os.RemoveAll(integConfigDir); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: could not remove config dir %s: %v\n", integConfigDir, err)
	}

	// Remove from state.
	delete(s.Integrations, name)
	if err := state.WriteState(statePath, s); err != nil {
		return fatal("writing state: %v", err)
	}

	fmt.Printf("✓ Uninstalled %s\n", name)
	return nil
}
