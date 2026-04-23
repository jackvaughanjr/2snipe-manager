package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/jackvaughanjr/2snipe-manager/internal/installer"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/scheduler"
	"github.com/jackvaughanjr/2snipe-manager/internal/secrets"
	"github.com/jackvaughanjr/2snipe-manager/internal/snipeit"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/jackvaughanjr/2snipe-manager/internal/wizard"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Install/config command flags — shared by installCmd and configCmd.
var (
	installFlagSnipeURL        string
	installFlagSnipeToken      string
	installFlagFields          []string
	installFlagSchedule        string
	installFlagSecretsBackend  string
)

var installCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Download, configure, and install a *2snipe integration",
	Long: `Install a *2snipe integration by name. Downloads the binary for your OS
and arch, runs the configuration wizard (or reads flags in --no-interactive mode),
ensures the Snipe-IT license category exists, writes settings.yaml, and optionally
creates a GCP Cloud Run Job + Cloud Scheduler trigger.

When called without a name, an interactive list of available integrations is
shown so you can pick one. Pass --no-interactive or pipe stdin to require an
explicit name argument instead.`,
	Args: cobra.MaximumNArgs(1),
	RunE: silentUsage(runInstall),
}

func init() {
	rootCmd.AddCommand(installCmd)
	addInstallFlags(installCmd)
}

// addInstallFlags binds the shared install/config flags to cmd.
func addInstallFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&installFlagSnipeURL, "snipe-url", "",
		"Snipe-IT instance URL (sets snipe_it.url config field)")
	cmd.Flags().StringVar(&installFlagSnipeToken, "snipe-token", "",
		"Snipe-IT API key (sets snipe_it.api_key config field)")
	cmd.Flags().StringArrayVar(&installFlagFields, "field", nil,
		"Set a config field value in key=value format (repeatable)")
	cmd.Flags().StringVar(&installFlagSchedule, "schedule", "manual",
		`Sync schedule: cron expression (e.g. "0 6 * * *") or "manual"`)
	cmd.Flags().StringVar(&installFlagSecretsBackend, "secrets-backend", "local",
		`Secrets storage backend: "gcp" (Secret Manager + Cloud Run) or "local" (settings.yaml only)`)
}

func runInstall(_ *cobra.Command, args []string) error {
	// Load registry sources.
	var sources []registry.Source
	if err := viper.UnmarshalKey("registry.sources", &sources); err != nil {
		return fatal("invalid registry.sources config: %v", err)
	}
	if len(sources) == 0 {
		return fatal("registry.sources is empty — set at least one owner in snipemgr.yaml")
	}
	token := viper.GetString("registry.github_token")

	// Load state.
	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}
	s, err := state.ReadState(statePath)
	if err != nil {
		return fatal("reading state: %v", err)
	}

	// Fetch the registry — needed for the picker and for the install itself.
	client := registry.NewClient(sources, token)
	integrations, err := client.List(s.InstalledVersions())
	if err != nil {
		return fatal("fetching registry: %v", err)
	}

	// Resolve the integration name — from arg or interactive picker.
	var name string
	if len(args) == 1 {
		name = args[0]
	} else if !noInteractive && isTerminal() {
		selected, err := pickIntegration(integrations)
		if err != nil {
			return err // pickIntegration already called fatal or returned nil on cancel
		}
		if selected == "" {
			return nil // user cancelled or chose "not listed"
		}
		name = selected
	} else {
		return fatal("integration name required — run 'snipemgr install <name>' or omit the name in an interactive terminal")
	}

	// Find integration in registry.
	target := findIntegration(integrations, name)
	if target == nil {
		return fatal("integration %q not found in registry — run 'snipemgr list' to see available integrations", name)
	}

	// Check if already installed.
	if target.Installed {
		if noInteractive {
			return fatal("integration %q is already installed — use 'snipemgr config %s' to reconfigure", name, name)
		}
		overwrite := false
		confirm := huh.NewConfirm().
			Title(fmt.Sprintf("%s is already installed (v%s). Reconfigure?", name, target.InstalledVersion)).
			Value(&overwrite)
		if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil || !overwrite {
			fmt.Println("Install cancelled.")
			return nil
		}
	}

	// Collect config values.
	var values map[string]string
	schedule := installFlagSchedule
	backend := installFlagSecretsBackend
	timezone := viper.GetString("gcp.scheduler_timezone")
	if timezone == "" {
		timezone = "UTC"
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
		// Seed wizard with Snipe-IT credentials from snipemgr.yaml, then overlay
		// any explicit flags so flags always win.
		existing := viperSnipeDefaults()
		if fm, err := buildFieldMap(); err == nil {
			for k, v := range fm {
				if v != "" {
					existing[k] = v
				}
			}
		}
		result, err := wizard.RunInteractive(target.Manifest, existing)
		if err != nil {
			return fatal("wizard: %v", err)
		}
		values = result.Values
		schedule = result.Schedule
		backend = result.Backend
		timezone = result.Timezone
	}

	// Ensure Snipe-IT category if the manifest specifies one.
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
		} else {
			slog.Info("skipping category check — Snipe-IT credentials not configured")
		}
	}

	// Download binary and write settings.yaml.
	binDir := viper.GetString("install.bin_dir")
	if binDir == "" {
		binDir = "~/.snipemgr/bin"
	}
	ins := &installer.Installer{
		BinDir:    binDir,
		ConfigDir: "~/.snipemgr/config",
		Token:     token,
	}
	fmt.Printf("Downloading %s v%s...\n", name, target.Manifest.Version)

	// For GCP backend, mask secret values in settings.yaml.
	settingsValues := values
	if backend == "gcp" {
		settingsValues = maskSecrets(target.Manifest.ConfigSchema, values)
	}
	if err := ins.Install(*target, settingsValues); err != nil {
		return fatal("install failed: %v", err)
	}

	// Build the state entry before GCP work so we can populate resource names.
	entry := state.InstalledIntegration{
		Version:        target.Manifest.Version,
		Enabled:        true,
		Schedule:       schedule,
		Timezone:       timezone,
		SecretsBackend: backend,
		InstalledAt:    time.Now().UTC().Format(time.RFC3339),
	}

	// GCP backend: write secrets to Secret Manager and create Cloud Run Job.
	if backend == "gcp" {
		project := viper.GetString("gcp.project")
		region := viper.GetString("gcp.region")
		serviceAccount := viper.GetString("gcp.service_account")
		credFile := viper.GetString("gcp.credentials_file")

		if project == "" {
			return fatal("gcp.project is not set in snipemgr.yaml — required for GCP backend")
		}
		if region == "" {
			region = "us-central1"
		}

		fmt.Println("Writing secrets to GCP Secret Manager...")
		if err := writeSecretsToGCP(name, target.Manifest, values, project, credFile); err != nil {
			return fatalGCP("writing secrets to GCP", err)
		}

		fmt.Println("Creating Cloud Run Job...")
		image := scheduler.ImagePath(project, region, name)
		spec := scheduler.JobSpec{
			Name:           name,
			Project:        project,
			Region:         region,
			Image:          image,
			ServiceAccount: serviceAccount,
			Schedule:       schedule,
			Timezone:       timezone,
			ConfigFields:   target.Manifest.ConfigSchema,
			SharedConfig:   target.Manifest.SharedConfig,
		}
		ctx := context.Background()
		sched, err := scheduler.NewGCPScheduler(ctx, credFile)
		if err != nil {
			return fatalGCP("connecting to GCP", err)
		}
		defer sched.Close()

		if err := sched.CreateJob(ctx, spec); err != nil {
			if errors.Is(err, scheduler.ErrImageNotFound) {
				// Image not yet in Artifact Registry — the Cloud Run Job resource
				// was created in GCP (in a failed state). Record it in state so
				// subsequent commands (run, status, uninstall) can find it. A
				// re-install after pushing the image will hit AlreadyExists and
				// proceed to scheduler creation.
				entry.CloudRunJob = scheduler.CloudRunJobName(project, region, name)
				fmt.Fprintln(os.Stderr, imageInstructions(name, project, region))
				fmt.Fprintf(os.Stderr, "Warning: Cloud Run Job created but image is missing — push the image before running.\n")
				fmt.Fprintf(os.Stderr, "After pushing the image, re-run this install command to attach a scheduler trigger.\n")
			} else {
				return fatal("creating GCP resources: %v", err)
			}
		} else {
			entry.CloudRunJob = scheduler.CloudRunJobName(project, region, name)
			if schedule != "manual" && schedule != "" {
				entry.SchedulerJob = scheduler.SchedulerJobName(project, region, name)
			}
		}
	}

	// Persist state.
	if s.Integrations == nil {
		s.Integrations = make(map[string]state.InstalledIntegration)
	}
	s.Integrations[name] = entry
	if err := state.WriteState(statePath, s); err != nil {
		return fatal("writing state: %v", err)
	}

	fmt.Printf("✓ Installed %s v%s\n", name, target.Manifest.Version)
	binDir, _ = expandHome(binDir)
	fmt.Printf("  Binary:   %s/%s\n", binDir, name)
	configBase, _ := expandHome("~/.snipemgr/config")
	fmt.Printf("  Config:   %s/%s/settings.yaml\n", configBase, name)
	fmt.Printf("  Backend:  %s\n", backend)
	fmt.Printf("  Schedule: %s\n", schedule)

	if backend == "gcp" && entry.CloudRunJob != "" {
		fmt.Printf("  Job:      %s\n", entry.CloudRunJob)
		if entry.SchedulerJob != "" {
			fmt.Printf("  Trigger:  %s\n", entry.SchedulerJob)
		}
		fmt.Printf("\nNote: build and push the container image before running:\n")
		fmt.Printf("  snipemgr run %s  (prints image build+push instructions)\n", name)
	}
	return nil
}

// writeSecretsToGCP writes all config values for an integration to GCP Secret
// Manager. Shared-config secrets are written once (reused by other integrations).
func writeSecretsToGCP(integrationName string, manifest registry.Manifest, values map[string]string, project, credFile string) error {
	ctx := context.Background()
	sm, err := secrets.NewGCPSecretManager(ctx, project, credFile)
	if err != nil {
		return err
	}
	defer sm.Close()

	sharedSet := make(map[string]bool, len(manifest.SharedConfig))
	for _, p := range manifest.SharedConfig {
		sharedSet[p] = true
	}

	for _, f := range manifest.ConfigSchema {
		val := values[f.Key]
		if val == "" {
			continue // skip unset optional fields
		}

		logicalName := secretLogicalName(integrationName, f, sharedSet)

		// For shared secrets: check if they already exist and skip if present.
		if sharedSet[keyPrefix(f.Key)] {
			exists, err := sm.Exists(ctx, logicalName)
			if err != nil {
				return fmt.Errorf("checking secret %q: %w", logicalName, err)
			}
			if exists {
				slog.Info("shared secret already exists — skipping", "secret", logicalName)
				continue
			}
		}

		if err := sm.Set(ctx, logicalName, val); err != nil {
			return err
		}
		slog.Info("secret written", "name", logicalName)
	}
	return nil
}

// secretLogicalName returns the logical Secret Manager name for a config field,
// mirroring the convention in internal/scheduler.configFieldToSecretName.
func secretLogicalName(integrationName string, f registry.ConfigField, sharedPrefixes map[string]bool) string {
	prefix := keyPrefix(f.Key)
	if sharedPrefixes[prefix] {
		switch f.Key {
		case "snipe_it.url":
			return "snipe/snipe-url"
		case "snipe_it.api_key":
			return "snipe/snipe-token"
		}
		return "snipe/" + keyLastSegment(f.Key, prefix)
	}
	return integrationName + "/" + keyLastSegment(f.Key, prefix)
}

// maskSecrets returns a copy of values with secret fields replaced by a
// placeholder. Used when writing settings.yaml for a GCP-backend install
// so secret values are not stored in plain text on disk.
func maskSecrets(schema []registry.ConfigField, values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	for _, f := range schema {
		if f.Secret {
			out[f.Key] = "# managed by GCP Secret Manager"
		}
	}
	return out
}

// keyPrefix returns the dot-notation prefix (everything before the first dot).
func keyPrefix(key string) string {
	if i := strings.Index(key, "."); i >= 0 {
		return key[:i]
	}
	return key
}

// keyLastSegment returns the last dot-notation segment as kebab-case.
func keyLastSegment(key, prefix string) string {
	seg := key
	if prefix != "" && strings.HasPrefix(key, prefix+".") {
		seg = key[len(prefix)+1:]
	}
	return strings.ReplaceAll(seg, "_", "-")
}

// viperSnipeDefaults returns a map pre-seeded with any snipe_it.* values already
// present in snipemgr.yaml so the wizard does not re-prompt for credentials the
// user set during init.
func viperSnipeDefaults() map[string]string {
	m := make(map[string]string)
	if u := viper.GetString("snipe_it.url"); u != "" {
		m["snipe_it.url"] = u
	}
	if t := viper.GetString("snipe_it.api_key"); t != "" {
		m["snipe_it.api_key"] = t
	}
	return m
}

// buildFieldMap constructs the key→value map from --snipe-url, --snipe-token,
// and --field flags.
func buildFieldMap() (map[string]string, error) {
	m := make(map[string]string)
	for _, kv := range installFlagFields {
		k, v, found := strings.Cut(kv, "=")
		if !found {
			return nil, fmt.Errorf("--field %q: expected key=value format", kv)
		}
		m[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if installFlagSnipeURL != "" {
		m["snipe_it.url"] = installFlagSnipeURL
	}
	if installFlagSnipeToken != "" {
		m["snipe_it.api_key"] = installFlagSnipeToken
	}
	return m, nil
}

// resolveSnipeCredentials returns the Snipe-IT URL and token to use for
// category management. Priority: viper (snipemgr.yaml) > wizard values > flags.
func resolveSnipeCredentials(wizardValues map[string]string) (url, token string) {
	url = viper.GetString("snipe_it.url")
	token = viper.GetString("snipe_it.api_key")
	if url == "" {
		url = wizardValues["snipe_it.url"]
	}
	if token == "" {
		token = wizardValues["snipe_it.api_key"]
	}
	if url == "" {
		url = installFlagSnipeURL
	}
	if token == "" {
		token = installFlagSnipeToken
	}
	return url, token
}

const notListedSentinel = "__not_listed__"

// pickIntegration shows a huh.Select with all available integrations plus a
// "not listed" option. Returns the chosen RepoName, an empty string if the user
// cancelled or chose "not listed" (in which case guidance is printed), or an
// error on form failure.
func pickIntegration(integrations []registry.Integration) (string, error) {
	if len(integrations) == 0 {
		fmt.Println("No integrations found. Check registry.sources in snipemgr.yaml.")
		return "", nil
	}

	options := make([]huh.Option[string], 0, len(integrations)+1)
	for _, intg := range integrations {
		label := intg.Manifest.DisplayName
		if intg.Manifest.Description != "" {
			label += " — " + intg.Manifest.Description
		}
		options = append(options, huh.NewOption(label, intg.RepoName))
	}
	options = append(options, huh.NewOption("My integration is not listed...", notListedSentinel))

	var chosen string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select an integration to install").
				Options(options...).
				Value(&chosen),
		),
	)
	if err := form.Run(); err != nil {
		return "", fatal("selection cancelled: %v", err)
	}

	if chosen == notListedSentinel {
		fmt.Println()
		fmt.Println("To install an integration that is not listed, add the GitHub account")
		fmt.Println("name of its author to registry.sources in snipemgr.yaml:")
		fmt.Println()
		fmt.Println("  registry:")
		fmt.Println("    sources:")
		fmt.Println("      - owner: their-github-username")
		fmt.Println()
		fmt.Println("Then re-run 'snipemgr install' to see their integrations.")
		return "", nil
	}

	return chosen, nil
}

// findIntegration searches integrations by repo name or manifest name.
func findIntegration(integrations []registry.Integration, name string) *registry.Integration {
	for i := range integrations {
		if integrations[i].RepoName == name || integrations[i].Manifest.Name == name {
			return &integrations[i]
		}
	}
	return nil
}
