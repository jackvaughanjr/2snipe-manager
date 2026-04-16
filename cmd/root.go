package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile       string
	verbose       bool
	debug         bool
	logFile       string
	logFormat     string
	noInteractive bool
)

var rootCmd = &cobra.Command{
	Use:   "snipemgr",
	Short: "Package manager and orchestrator for the *2snipe integration suite",
	Long: `snipemgr discovers, installs, configures, schedules, and monitors
*2snipe integrations — tools that sync vendor software licenses and assets
into Snipe-IT — from a single place.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.Root().SilenceUsage = true  // suppress usage block on runtime errors
		cmd.Root().SilenceErrors = true // suppress cobra's duplicate "Error: ..." echo
		initLogging()
		return nil
	},
	// Print help when invoked with no subcommand.
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// SetVersion sets the version string shown by --version.
func SetVersion(v string) {
	rootCmd.Version = v
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "snipemgr.yaml", "path to snipemgr config file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "INFO-level logging")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "DEBUG-level logging")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "append logs to a file")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", `log format: "text" or "json"`)
	rootCmd.PersistentFlags().BoolVar(&noInteractive, "no-interactive", false, "disable interactive forms; use flags only (for scripted use)")
}

func initConfig() {
	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv()
	_ = viper.ReadInConfig() // silently ignore missing config; commands validate required fields
}

func initLogging() {
	level := slog.LevelWarn
	if debug {
		level = slog.LevelDebug
	} else if verbose {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var w io.Writer = os.Stderr
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot open log file: %v\n", err)
		} else {
			w = f
		}
	}

	var handler slog.Handler
	if logFormat == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// fatal prints an error to stderr and returns it. Use in RunE instead of bare
// return fmt.Errorf(...) so errors are visible when SilenceErrors is set.
func fatal(format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	return err
}

// expandHome expands a leading ~/ in path to the user's home directory.
func expandHome(path string) (string, error) {
	if len(path) >= 2 && path[0] == '~' && path[1] == '/' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
