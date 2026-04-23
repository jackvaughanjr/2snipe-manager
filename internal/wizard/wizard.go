// Package wizard drives the interactive install/config flow from a manifest's
// config_schema. The manager has no hardcoded knowledge of any integration's
// config fields — everything is read from the manifest.
package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
)

// Result holds the collected configuration values from the wizard.
type Result struct {
	Values   map[string]string // config_schema key → value
	Schedule string            // cron expression or "manual"
	Backend  string            // "local" for Phase 2; "gcp" in Phase 3
	Timezone string            // IANA timezone for cron schedule (e.g. "America/New_York")
}

// BuildFlagDefaults constructs a config values map from CLI flags provided in
// non-interactive mode. Returns an error naming the field if a required field
// has no value.
func BuildFlagDefaults(schema []registry.ConfigField, flags map[string]string) (map[string]string, error) {
	values := make(map[string]string, len(schema))
	for _, f := range schema {
		val := flags[f.Key]
		if val == "" {
			val = f.Default
		}
		if f.Required && val == "" {
			return nil, fmt.Errorf("required field %q (%s) is missing — pass --field %s=<value>",
				f.Key, f.Label, strings.ReplaceAll(f.Key, ".", "-"))
		}
		values[f.Key] = val
	}
	return values, nil
}

// RunInteractive presents huh forms in three sequential pages:
//  1. Backend selection
//  2. Schedule + timezone (GCP backend only)
//  3. Config fields
//
// Returns an error if any form is cancelled or the terminal does not support
// interactive input.
func RunInteractive(manifest registry.Manifest, existing map[string]string) (*Result, error) {
	// Page 1: Backend selection.
	backend := "local"
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Secrets backend").
			Description("GCP Secret Manager is required for scheduled Cloud Run Jobs.\nLocal mode stores credentials in settings.yaml only.").
			Options(
				huh.NewOption("GCP Secret Manager (recommended — required for scheduling)", "gcp"),
				huh.NewOption("Local settings.yaml only (manual runs only)", "local"),
			).
			Value(&backend),
	)).Run(); err != nil {
		return nil, err
	}

	// Page 2: Schedule + timezone — only shown when GCP backend was selected.
	schedule := "manual"
	timezone := "UTC"
	if backend == "gcp" {
		timezoneChoice := "UTC"
		if err := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Sync schedule").
				Description("A Cloud Scheduler trigger will be created for the chosen schedule.\nSelect 'Manual only' to skip scheduling.").
				Options(
					huh.NewOption("Manual only (trigger by hand)", "manual"),
					huh.NewOption("Daily at 06:00", "0 6 * * *"),
					huh.NewOption("Daily at 07:00", "0 7 * * *"),
					huh.NewOption("Daily at 08:00", "0 8 * * *"),
				).
				Value(&schedule),
			huh.NewSelect[string]().
				Title("Schedule timezone").
				Description("Timezone used to interpret the cron schedule above.\nIgnored when schedule is set to Manual only.").
				Options(
					huh.NewOption("UTC", "UTC"),
					huh.NewOption("Eastern  (America/New_York)", "America/New_York"),
					huh.NewOption("Central  (America/Chicago)", "America/Chicago"),
					huh.NewOption("Mountain (America/Denver)", "America/Denver"),
					huh.NewOption("Pacific  (America/Los_Angeles)", "America/Los_Angeles"),
					huh.NewOption("Other — enter IANA name", "other"),
				).
				Value(&timezoneChoice),
		)).Run(); err != nil {
			return nil, err
		}

		timezone = timezoneChoice
		if timezoneChoice == "other" {
			customTZ := ""
			if err := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("Timezone name").
					Description("Enter a valid IANA timezone name.\n"+
						"Examples: Europe/London, Asia/Tokyo, Australia/Sydney\n"+
						"Full list: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones").
					Value(&customTZ),
			)).Run(); err != nil {
				return nil, err
			}
			timezone = strings.TrimSpace(customTZ)
			if timezone == "" {
				timezone = "UTC"
			}
		}
	}

	// Page 3: Config fields.
	vals := make([]string, len(manifest.ConfigSchema))
	for i, f := range manifest.ConfigSchema {
		if v, ok := existing[f.Key]; ok {
			vals[i] = v
		} else {
			vals[i] = f.Default
		}
	}

	var configFields []huh.Field
	for i, f := range manifest.ConfigSchema {
		input := huh.NewInput().
			Title(f.Label).
			Value(&vals[i])
		if f.Hint != "" {
			input = input.Description(f.Hint)
		}
		if f.Secret {
			input = input.EchoMode(huh.EchoModePassword)
		}
		configFields = append(configFields, input)
	}

	if len(configFields) > 0 {
		if err := huh.NewForm(huh.NewGroup(configFields...)).Run(); err != nil {
			return nil, err
		}
	}

	values := make(map[string]string, len(manifest.ConfigSchema))
	for i, f := range manifest.ConfigSchema {
		values[f.Key] = vals[i]
	}

	return &Result{
		Values:   values,
		Schedule: schedule,
		Backend:  backend,
		Timezone: timezone,
	}, nil
}
