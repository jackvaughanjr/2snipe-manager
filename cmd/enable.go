package cmd

import (
	"context"
	"fmt"

	"github.com/jackvaughanjr/2snipe-manager/internal/scheduler"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var enableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Resume a paused integration schedule",
	Long: `Resume the Cloud Scheduler trigger for an integration that was previously
disabled with 'snipemgr disable'. Has no effect on local-backend integrations.`,
	Args: cobra.ExactArgs(1),
	RunE: silentUsage(runEnable),
}

func init() {
	rootCmd.AddCommand(enableCmd)
}

func runEnable(_ *cobra.Command, args []string) error {
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
	if intg.SecretsBackend != "gcp" {
		return fatal("integration %q uses the local backend — enable/disable requires the GCP backend", name)
	}
	if intg.SchedulerJob == "" {
		return fatal("integration %q has no Cloud Scheduler trigger — it was installed with --schedule manual", name)
	}

	credFile := viper.GetString("gcp.credentials_file")
	ctx := context.Background()
	sched, err := scheduler.NewGCPScheduler(ctx, credFile)
	if err != nil {
		return fatalGCP("connecting to GCP", err)
	}
	defer sched.Close()

	if err := sched.EnableJob(ctx, intg.SchedulerJob); err != nil {
		return fatal("enabling scheduler job: %v", err)
	}

	// Update state.
	intg.Enabled = true
	s.Integrations[name] = intg
	if err := state.WriteState(statePath, s); err != nil {
		return fatal("writing state: %v", err)
	}

	fmt.Printf("✓ %s schedule enabled (%s)\n", name, intg.Schedule)
	return nil
}
