package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var initFlagForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create snipemgr.yaml interactively (first-time setup)",
	Long: `Create snipemgr.yaml through a short interactive wizard.

This command is intended to be run once when you first install snipemgr.
Running it again will completely overwrite your existing snipemgr.yaml —
installed integrations, integration config files, and state are not affected.

Use --force to skip the overwrite confirmation (for scripted environments).`,
	RunE: silentUsage(runInit),
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initFlagForce, "force", false,
		"overwrite existing snipemgr.yaml without prompting for confirmation")
}

func runInit(_ *cobra.Command, _ []string) error {
	configPath := cfgFile // honours --config flag; defaults to "snipemgr.yaml"

	// Check for an existing config file and require confirmation before overwriting.
	if _, err := os.Stat(configPath); err == nil {
		if noInteractive || !isTerminal() {
			if !initFlagForce {
				return fatal("%s already exists — pass --force to overwrite", configPath)
			}
		} else {
			overwrite := false
			confirm := huh.NewConfirm().
				Title(fmt.Sprintf("'%s' already exists. Overwrite?", configPath)).
				Description(
					"snipemgr init is intended to be run once on first setup.\n\n" +
						"Continuing will completely replace your current snipemgr.yaml.\n" +
						"Installed integrations, integration configs, and state are not affected.\n\n" +
						"This action cannot be undone.").
				Affirmative("Yes, overwrite").
				Negative("Cancel").
				Value(&overwrite)
			if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil || !overwrite {
				fmt.Println("Init cancelled.")
				return nil
			}
		}
	}

	if noInteractive || !isTerminal() {
		return fatal("snipemgr init requires an interactive terminal — remove --no-interactive to proceed")
	}

	// --- Step 1: GitHub registry ---
	// jackvaughanjr is always written as the first source — it hosts the *2snipe suite.
	// This step asks only for an optional additional owner (e.g. a private org).
	// Users who don't want jackvaughanjr can remove that entry from snipemgr.yaml afterward.
	extraOwner := ""
	githubToken := ""

	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Additional GitHub owner (optional)").
			Description(
				"jackvaughanjr is included by default — that's where the *2snipe\n"+
					"integration suite lives. Enter an additional GitHub username or org\n"+
					"if you have private or custom integrations elsewhere, or leave blank.\n"+
					"To remove jackvaughanjr as a source, edit the registry.sources\n"+
					"list in snipemgr.yaml after init completes.").
			Value(&extraOwner),
		huh.NewInput().
			Title("GitHub personal access token (optional)").
			Description(
				"Raises the GitHub API rate limit from 60 to 5,000 req/hr.\n"+
					"Create at github.com/settings/tokens/new — scope: public_repo\n"+
					"(use the repo scope if your *2snipe repos are private).").
			EchoMode(huh.EchoModePassword).
			Value(&githubToken),
	)).Run(); err != nil {
		return fatal("init: %v", err)
	}
	extraOwner = strings.TrimSpace(extraOwner)

	// --- Step 2: Snipe-IT credentials (optional) ---
	configureSnipe := false
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Configure Snipe-IT credentials now?").
			Description(
				"Required for category management during 'snipemgr install'.\n"+
					"You can skip this and add the values to snipemgr.yaml later.").
			Affirmative("Yes").
			Negative("Skip for now").
			Value(&configureSnipe),
	)).Run(); err != nil {
		return fatal("init: %v", err)
	}

	snipeURL := ""
	snipeToken := ""
	if configureSnipe {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Snipe-IT URL").
				Description("e.g. https://snipe.example.com").
				Value(&snipeURL),
			huh.NewInput().
				Title("Snipe-IT API key").
				Description("Admin → API Keys in your Snipe-IT instance.").
				EchoMode(huh.EchoModePassword).
				Value(&snipeToken),
		)).Run(); err != nil {
			return fatal("init: %v", err)
		}
	}

	// --- Step 3: GCP (optional) ---
	configureGCP := false
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Configure GCP now?").
			Description(
				"Required for --secrets-backend gcp (Secret Manager + Cloud Run Jobs).\n"+
					"You can skip this and add the values to snipemgr.yaml later.").
			Affirmative("Yes").
			Negative("Skip for now").
			Value(&configureGCP),
	)).Run(); err != nil {
		return fatal("init: %v", err)
	}

	gcpProject := ""
	gcpRegion := "us-central1"
	gcpSA := ""
	gcpTimezone := "UTC"
	if configureGCP {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("GCP project ID").
				Value(&gcpProject),
			huh.NewInput().
				Title("GCP region").
				Description("Region for Cloud Run Jobs and Cloud Scheduler. Default: us-central1").
				Value(&gcpRegion),
			huh.NewInput().
				Title("Service account email").
				Description("e.g. snipemgr-runner@your-project-id.iam.gserviceaccount.com").
				Value(&gcpSA),
			huh.NewSelect[string]().
				Title("Schedule timezone").
				Description("Timezone used to interpret cron schedules for Cloud Scheduler triggers.").
				Options(
					huh.NewOption("UTC", "UTC"),
					huh.NewOption("Eastern  (America/New_York)", "America/New_York"),
					huh.NewOption("Central  (America/Chicago)", "America/Chicago"),
					huh.NewOption("Mountain (America/Denver)", "America/Denver"),
					huh.NewOption("Pacific  (America/Los_Angeles)", "America/Los_Angeles"),
					huh.NewOption("Other — enter IANA name", "other"),
				).
				Value(&gcpTimezone),
		)).Run(); err != nil {
			return fatal("init: %v", err)
		}
		gcpProject = strings.TrimSpace(gcpProject)
		gcpRegion = strings.TrimSpace(gcpRegion)
		if gcpRegion == "" {
			gcpRegion = "us-central1"
		}
		if gcpTimezone == "other" {
			customTZ := ""
			if err := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("Timezone name").
					Description("Enter a valid IANA timezone name.\n"+
						"Examples: Europe/London, Asia/Tokyo, Australia/Sydney\n"+
						"Full list: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones").
					Value(&customTZ),
			)).Run(); err != nil {
				return fatal("init: %v", err)
			}
			gcpTimezone = strings.TrimSpace(customTZ)
			if gcpTimezone == "" {
				gcpTimezone = "UTC"
			}
		}
	}

	// Write snipemgr.yaml (0600: may contain credentials).
	content := buildInitConfig(extraOwner, githubToken, snipeURL, snipeToken, gcpProject, gcpRegion, gcpSA, gcpTimezone)
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fatal("writing %s: %v", configPath, err)
	}

	fmt.Printf("✓ %s written\n", configPath)
	fmt.Println("  Run 'snipemgr list' to see available integrations.")
	return nil
}

// buildInitConfig renders snipemgr.yaml content from collected wizard values.
// extraOwner is an optional additional registry source; jackvaughanjr is always
// written as the first source. Written as a string template rather than marshaled
// YAML so inline documentation comments are preserved.
func buildInitConfig(extraOwner, token, snipeURL, snipeToken, gcpProject, gcpRegion, gcpSA, gcpTimezone string) string {
	var b strings.Builder

	b.WriteString("# snipemgr configuration\n")
	b.WriteString("# Generated by 'snipemgr init' — see snipemgr.example.yaml for all options.\n")
	b.WriteString("# This file may contain credentials — never commit it to version control.\n")
	b.WriteString("\n")

	// Registry
	b.WriteString("registry:\n")
	b.WriteString("  sources:\n")
	b.WriteString("    - owner: jackvaughanjr  # default source — hosts the *2snipe integration suite\n")
	if extraOwner != "" {
		b.WriteString(fmt.Sprintf("    - owner: %s\n", extraOwner))
	} else {
		b.WriteString("    # - owner: your-org  # add private or custom integration sources here\n")
	}
	b.WriteString("  require_manifest: true\n")
	if token != "" {
		b.WriteString(fmt.Sprintf("  github_token: %q\n", token))
	} else {
		b.WriteString("  github_token: \"\"  # optional: GitHub PAT for higher API rate limits (60 → 5,000 req/hr)\n")
	}
	b.WriteString("\n")

	// Snipe-IT
	b.WriteString("snipe_it:\n")
	if snipeURL != "" {
		b.WriteString(fmt.Sprintf("  url: %q\n", snipeURL))
	} else {
		b.WriteString("  url: \"\"      # Snipe-IT instance URL (required for category management during install)\n")
	}
	if snipeToken != "" {
		b.WriteString(fmt.Sprintf("  api_key: %q\n", snipeToken))
	} else {
		b.WriteString("  api_key: \"\"  # Snipe-IT API key (Admin → API Keys)\n")
	}
	b.WriteString("\n")

	// Install
	b.WriteString("install:\n")
	b.WriteString("  bin_dir: \"~/.snipemgr/bin\"  # must be on your PATH\n")
	b.WriteString("\n")

	// GCP
	b.WriteString("gcp:\n")
	if gcpProject != "" {
		b.WriteString(fmt.Sprintf("  project: %q\n", gcpProject))
	} else {
		b.WriteString("  project: \"\"  # GCP project ID (required for --secrets-backend gcp)\n")
	}
	b.WriteString(fmt.Sprintf("  region: %q\n", gcpRegion))
	if gcpSA != "" {
		b.WriteString(fmt.Sprintf("  service_account: %q\n", gcpSA))
	} else {
		b.WriteString("  service_account: \"\"  # e.g. snipemgr-runner@your-project-id.iam.gserviceaccount.com\n")
	}
	b.WriteString(fmt.Sprintf("  scheduler_timezone: %q  # IANA timezone for Cloud Scheduler cron triggers\n", gcpTimezone))
	b.WriteString("\n")

	// State
	b.WriteString("state:\n")
	b.WriteString("  path: \"~/.snipemgr/state.json\"\n")

	return b.String()
}
