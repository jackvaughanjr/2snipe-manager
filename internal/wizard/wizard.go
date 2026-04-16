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

// RunInteractive presents huh forms for each config field and a schedule
// selection, then returns the collected values. Returns an error if the form is
// cancelled or the terminal does not support interactive input.
func RunInteractive(manifest registry.Manifest, existing map[string]string) (*Result, error) {
	// Pre-populate with existing values or defaults.
	vals := make([]string, len(manifest.ConfigSchema))
	for i, f := range manifest.ConfigSchema {
		if v, ok := existing[f.Key]; ok {
			vals[i] = v
		} else {
			vals[i] = f.Default
		}
	}

	// Build one huh.Input per config field.
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

	// Schedule selection.
	schedule := "manual"
	scheduleSelect := huh.NewSelect[string]().
		Title("Sync schedule").
		Description("Cloud Scheduler is configured in Phase 3; this value is recorded for later.").
		Options(
			huh.NewOption("Manual only (trigger by hand)", "manual"),
			huh.NewOption("Daily at 06:00 UTC", "0 6 * * *"),
			huh.NewOption("Daily at 07:00 UTC", "0 7 * * *"),
			huh.NewOption("Daily at 08:00 UTC", "0 8 * * *"),
		).
		Value(&schedule)

	// Build form: config fields in one group, schedule in another.
	var groups []*huh.Group
	if len(configFields) > 0 {
		groups = append(groups, huh.NewGroup(configFields...))
	}
	groups = append(groups, huh.NewGroup(scheduleSelect))

	form := huh.NewForm(groups...)
	if err := form.Run(); err != nil {
		return nil, err
	}

	// Collect results back into a map.
	values := make(map[string]string, len(manifest.ConfigSchema))
	for i, f := range manifest.ConfigSchema {
		values[f.Key] = vals[i]
	}

	return &Result{
		Values:   values,
		Schedule: schedule,
		Backend:  "local",
	}, nil
}
