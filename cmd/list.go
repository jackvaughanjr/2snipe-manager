package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available integrations from the registry",
	Long: `Discover and display all available *2snipe integrations from configured
GitHub sources. Integrations are identified by the topic "2snipe" and a
valid 2snipe.json manifest in the repo root.`,
	RunE: silentUsage(runList),
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(_ *cobra.Command, _ []string) error {
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
		fmt.Fprintln(os.Stderr, "  Set registry.github_token in snipemgr.yaml to increase to 5000 req/hr.")
	}

	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}
	s, err := state.ReadState(statePath)
	if err != nil {
		return fatal("reading state: %v", err)
	}

	client := registry.NewClient(sources, token)
	integrations, err := client.List(s.InstalledVersions())
	if err != nil {
		return fatal("listing integrations: %v", err)
	}

	if len(integrations) == 0 {
		fmt.Println("No integrations found. Check registry.sources in snipemgr.yaml.")
		return nil
	}

	if !noInteractive && isTerminal() {
		fmt.Print(renderLipglossTable(integrations))
	} else {
		renderPlainTable(integrations)
	}
	return nil
}

// isTerminal reports whether stdout is connected to a terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// renderPlainTable writes a plain-text tab-aligned table to stdout.
// Used in --no-interactive mode and when stdout is piped.
func renderPlainTable(integrations []registry.Integration) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tVERSION\tCATEGORY\tDESCRIPTION")
	for _, intg := range integrations {
		status := "○ available"
		if intg.Installed {
			status = "● installed"
			if intg.UpdateAvail {
				status += " ↑ update"
			}
		}
		category := intg.Manifest.Category
		if category == "" {
			category = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			intg.Manifest.DisplayName,
			status,
			intg.Manifest.Version,
			category,
			intg.Manifest.Description,
		)
	}
	_ = w.Flush()
}

// renderLipglossTable renders a styled table for terminal output.
func renderLipglossTable(integrations []registry.Integration) string {
	const padding = 2
	const maxDesc = 55

	headers := []string{"NAME", "STATUS", "VERSION", "CATEGORY", "DESCRIPTION"}
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	type row struct{ cells [5]string }
	rows := make([]row, len(integrations))

	for i, intg := range integrations {
		category := intg.Manifest.Category
		if category == "" {
			category = "-"
		}
		desc := intg.Manifest.Description
		if len(desc) > maxDesc {
			desc = desc[:maxDesc-3] + "..."
		}
		status := "○ available"
		if intg.Installed {
			status = "● installed"
			if intg.UpdateAvail {
				status += " ↑ update"
			}
		}
		cells := [5]string{
			intg.Manifest.DisplayName,
			status,
			intg.Manifest.Version,
			category,
			desc,
		}
		rows[i] = row{cells: cells}
		for j, c := range cells {
			if len(c) > colWidths[j] {
				colWidths[j] = len(c)
			}
		}
	}

	headerStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	baseStyle := lipgloss.NewStyle()

	renderCell := func(s lipgloss.Style, text string, width int) string {
		return s.Width(width + padding).Render(text)
	}

	var sb strings.Builder

	// Header row
	for i, h := range headers {
		sb.WriteString(renderCell(headerStyle, h, colWidths[i]))
	}
	sb.WriteString("\n")

	// Separator
	for _, w := range colWidths {
		sb.WriteString(renderCell(dimStyle, strings.Repeat("─", w), w))
	}
	sb.WriteString("\n")

	// Data rows
	for _, r := range rows {
		for i, cell := range r.cells {
			s := baseStyle
			if i == 1 && strings.HasPrefix(cell, "●") { // STATUS column — highlight installed
				s = greenStyle
			}
			sb.WriteString(renderCell(s, cell, colWidths[i]))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
