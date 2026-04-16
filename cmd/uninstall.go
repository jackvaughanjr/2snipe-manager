package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Remove an installed integration (binary, config, and state entry)",
	Args:  cobra.ExactArgs(1),
	RunE:  runUninstall,
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

	if _, ok := s.Integrations[name]; !ok {
		return fatal("integration %q is not installed", name)
	}

	// Confirm in interactive mode.
	if !noInteractive && isTerminal() {
		confirmed := false
		confirm := huh.NewConfirm().
			Title(fmt.Sprintf("Uninstall %s? This removes the binary, config, and state entry.", name)).
			Value(&confirmed)
		if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil || !confirmed {
			fmt.Println("Uninstall cancelled.")
			return nil
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
