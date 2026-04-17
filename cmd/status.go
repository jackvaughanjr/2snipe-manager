package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/scheduler"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show installed integrations with schedule and last-run result",
	Long: `Display a table of all installed integrations, their enabled/disabled state,
cron schedule, last run time, and last run result. For GCP-backend integrations,
last-run data is fetched live from the Cloud Run Jobs executions API.`,
	RunE: silentUsage(runStatus),
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
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

	// Try to build a GCP scheduler client if any GCP integrations are present.
	var sched scheduler.Scheduler
	project := viper.GetString("gcp.project")
	region := viper.GetString("gcp.region")
	if region == "" {
		region = "us-central1"
	}
	if project != "" && hasGCPIntegration(s) {
		credFile := viper.GetString("gcp.credentials_file")
		ctx := context.Background()
		g, gErr := scheduler.NewGCPScheduler(ctx, credFile)
		if gErr != nil {
			warnGCP("could not connect to GCP — last-run data unavailable", gErr)
		} else {
			sched = g
			defer g.Close()
		}
	}

	// Optionally fetch registry to populate update availability indicators.
	// Silently skipped if registry sources are not configured or unreachable.
	updateAvail := fetchUpdateAvailability(s.InstalledVersions())

	if noInteractive || !isTerminal() {
		return printStatusPlain(s, sched, project, region, updateAvail)
	}
	return printStatusStyled(s, sched, project, region, updateAvail)
}

func hasGCPIntegration(s *state.State) bool {
	for _, intg := range s.Integrations {
		if intg.SecretsBackend == "gcp" {
			return true
		}
	}
	return false
}

func printStatusStyled(s *state.State, sched scheduler.Scheduler, project, region string, updateAvail map[string]bool) error {
	// Header styles.
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	enabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	updateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow

	headers := []string{"INTEGRATION", "ENABLED", "VERSION", "SCHEDULE", "LAST RUN", "RESULT"}
	colWidths := []int{20, 12, 16, 14, 22, 12}

	printRow := func(cells []string, styles []lipgloss.Style) {
		for i, cell := range cells {
			w := colWidths[i]
			var styled string
			if i < len(styles) {
				styled = styles[i].Width(w).Render(cell)
			} else {
				styled = lipgloss.NewStyle().Width(w).Render(cell)
			}
			fmt.Print(styled)
		}
		fmt.Println()
	}

	// Header row.
	var headerStyles []lipgloss.Style
	for range headers {
		headerStyles = append(headerStyles, headerStyle)
	}
	printRow(headers, headerStyles)

	// Divider.
	total := 0
	for _, w := range colWidths {
		total += w
	}
	fmt.Println(dividerStyle.Render(repeatStr("─", total)))

	for name, intg := range s.Integrations {
		enabledStr := enabledStyle.Render("✓ enabled")
		if !intg.Enabled {
			enabledStr = disabledStyle.Render("✗ paused")
		}

		schedStr := intg.Schedule
		if schedStr == "" || schedStr == "manual" {
			schedStr = "manual"
		}

		version := intg.Version
		if version != "" && !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		if version == "" {
			version = "—"
		}
		var versionStyle lipgloss.Style
		if updateAvail[name] {
			version += " ↑"
			versionStyle = updateStyle
		} else {
			versionStyle = lipgloss.NewStyle()
		}

		lastRun, result := lastRunDisplay(name, intg, sched, project, region)

		var resultStyle lipgloss.Style
		switch result {
		case "✓ success":
			resultStyle = successStyle
		case "✗ failed":
			resultStyle = failStyle
		default:
			resultStyle = lipgloss.NewStyle()
		}

		enabledCellStyle := lipgloss.NewStyle()
		if intg.Enabled {
			enabledCellStyle = enabledStyle
		} else {
			enabledCellStyle = disabledStyle
		}

		cells := []string{name, enabledStr, version, schedStr, lastRun, result}
		styles := []lipgloss.Style{
			lipgloss.NewStyle(),
			enabledCellStyle,
			versionStyle,
			lipgloss.NewStyle(),
			lipgloss.NewStyle(),
			resultStyle,
		}
		printRow(cells, styles)
	}
	return nil
}

func printStatusPlain(s *state.State, sched scheduler.Scheduler, project, region string, updateAvail map[string]bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "INTEGRATION\tENABLED\tVERSION\tSCHEDULE\tLAST RUN\tRESULT")
	for name, intg := range s.Integrations {
		enabled := "✓ enabled"
		if !intg.Enabled {
			enabled = "✗ paused"
		}
		sched_ := intg.Schedule
		if sched_ == "" || sched_ == "manual" {
			sched_ = "manual"
		}
		version := intg.Version
		if version != "" && !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		if version == "" {
			version = "—"
		}
		if updateAvail[name] {
			version += " ↑"
		}
		lastRun, result := lastRunDisplay(name, intg, sched, project, region)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, enabled, version, sched_, lastRun, result)
	}
	return w.Flush()
}

// lastRunDisplay returns the formatted last-run time and result strings for a
// row. For GCP integrations, it tries to fetch live execution status.
func lastRunDisplay(name string, intg state.InstalledIntegration, sched scheduler.Scheduler, project, region string) (string, string) {
	result := intg.LastRunResult
	lastRun := intg.LastRunAt

	// Try to refresh from GCP executions API.
	if sched != nil && intg.SecretsBackend == "gcp" && intg.CloudRunJob != "" {
		ctx := context.Background()
		exec, err := sched.GetLastExecution(ctx, name, project, region)
		if err != nil {
			slog.Debug("could not fetch last execution", "name", name, "error", err)
		} else if exec != nil {
			result = formatResult(exec.Status)
			if !exec.CompletedAt.IsZero() {
				lastRun = exec.CompletedAt.UTC().Format("2006-01-02 15:04 UTC")
			}
		}
	}

	if lastRun == "" {
		lastRun = "never"
	} else if t, err := time.Parse(time.RFC3339, lastRun); err == nil {
		lastRun = t.UTC().Format("2006-01-02 15:04 UTC")
	}
	if result == "" {
		result = "—"
	}
	return lastRun, result
}

// formatResult converts an execution status string to a display string.
func formatResult(s string) string {
	switch s {
	case "success":
		return "✓ success"
	case "failed":
		return "✗ failed"
	case "running":
		return "⟳ running"
	case "cancelled":
		return "⊘ cancelled"
	default:
		return s
	}
}

// fetchUpdateAvailability queries the registry (if configured) and returns a
// map of integration name → true for each integration with a newer version
// available. If the registry is not configured or unreachable, an empty map is
// returned and no error is surfaced — update indicators are simply omitted.
func fetchUpdateAvailability(installed map[string]string) map[string]bool {
	result := map[string]bool{}
	var sources []registry.Source
	if err := viper.UnmarshalKey("registry.sources", &sources); err != nil || len(sources) == 0 {
		return result
	}
	token := viper.GetString("registry.github_token")
	client := registry.NewClient(sources, token)
	integrations, err := client.List(installed)
	if err != nil {
		slog.Debug("could not fetch registry for update check", "error", err)
		return result
	}
	for _, intg := range integrations {
		if intg.UpdateAvail {
			result[intg.RepoName] = true
		}
	}
	return result
}

func repeatStr(s string, n int) string {
	b := make([]byte, n*len(s))
	for i := range n {
		copy(b[i*len(s):], s)
	}
	return string(b)
}
