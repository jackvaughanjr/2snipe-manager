package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var whereCmd = &cobra.Command{
	Use:   "where",
	Short: "Print the paths snipemgr is using for config, state, and binaries",
	Long: `Print the resolved paths for snipemgr's config file, state file, and binary
directory. Reports "(not found)" for files that do not exist. Always exits 0.`,
	RunE: silentUsage(runWhere),
}

func init() {
	rootCmd.AddCommand(whereCmd)
}

func runWhere(cmd *cobra.Command, _ []string) error {
	exePath := "(unknown)"
	if exe, err := os.Executable(); err == nil {
		exePath = exe
	}

	configPath := cfgFile
	if !cmd.Root().PersistentFlags().Changed("config") {
		configPath = resolveConfigPath()
	}

	statePath := viper.GetString("state.path")
	if statePath == "" {
		home, _ := os.UserHomeDir()
		statePath = filepath.Join(home, ".snipemgr", "state.json")
	} else if expanded, err := expandHome(statePath); err == nil {
		statePath = expanded
	}

	binDir := viper.GetString("install.bin_dir")
	if binDir == "" {
		home, _ := os.UserHomeDir()
		binDir = filepath.Join(home, ".snipemgr", "bin")
	} else if expanded, err := expandHome(binDir); err == nil {
		binDir = expanded
	}

	fmt.Printf("Binary:   %s\n", exePath)
	fmt.Printf("Config:   %s\n", pathDisplay(configPath))
	fmt.Printf("State:    %s\n", pathDisplay(statePath))
	fmt.Printf("Bin dir:  %s\n", pathDisplay(binDir))
	return nil
}

// pathDisplay appends "(not found)" when the path does not exist on disk.
func pathDisplay(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path + "  (not found)"
	}
	return path
}
