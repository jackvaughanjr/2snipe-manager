package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile          string
	verbose          bool
	debug            bool
	logFile          string
	logFormat        string
	noInteractive    bool
	configFileMissing bool // set by initConfig when the config file does not exist
)

var rootCmd = &cobra.Command{
	Use:   "snipemgr",
	Short: "Package manager and orchestrator for the *2snipe integration suite",
	Long: `snipemgr discovers, installs, configures, schedules, and monitors
*2snipe integrations — tools that sync vendor software licenses and assets
into Snipe-IT — from a single place.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.Root().SilenceErrors = true // suppress cobra's duplicate "Error: ..." echo
		initLogging()
		if configFileMissing && cmd.Name() != "init" {
			fmt.Fprintf(os.Stderr, "Note: %s not found — run 'snipemgr init' to create it\n\n", cfgFile)
		}
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
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		configFileMissing = true
	}
	_ = viper.ReadInConfig() // silently ignore missing/invalid config; commands validate required fields
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

// silentUsage wraps a RunE function so that SilenceUsage is set before the
// function runs. Because cobra validates Args before calling RunE, arg/flag
// errors still print the usage block; only errors returned from RunE itself
// are silenced. Use this instead of setting cmd.Root().SilenceUsage in
// PersistentPreRunE, which would also suppress usage on arg validation errors.
func silentUsage(fn func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cmd.Root().SilenceUsage = true
		return fn(cmd, args)
	}
}

// fatal prints an error to stderr and returns it. Use in RunE instead of bare
// return fmt.Errorf(...) so errors are visible when SilenceErrors is set.
func fatal(format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	return err
}

// fatalGCP prints a GCP error to stderr, appends an auth hint when the error
// looks like a credentials failure, and returns the error. Use in RunE wherever
// a GCP client connection or API call is required.
func fatalGCP(action string, err error) error {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n", action, err)
	if hint := gcpHint(err); hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// warnGCP prints a non-fatal GCP warning to stderr, appending an auth hint
// when the error looks like a credentials failure. Use when GCP is optional
// and the command should continue without it.
func warnGCP(context string, err error) {
	fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", context, err)
	if hint := gcpHint(err); hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
}

// gcpHint returns an actionable remediation message when err looks like a GCP
// credentials or authentication failure. Returns empty string for other errors.
func gcpHint(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "credentials") ||
		strings.Contains(msg, "unauthenticated") ||
		strings.Contains(msg, "oauth") ||
		strings.Contains(msg, "401") {
		return "  Hint: run 'gcloud auth application-default login' to refresh credentials,\n" +
			"  or set gcp.credentials_file in snipemgr.yaml for explicit auth."
	}
	return ""
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
