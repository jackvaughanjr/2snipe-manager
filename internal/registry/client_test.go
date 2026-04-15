package registry_test

import (
	"testing"

	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
)

// validManifest returns a fully valid manifest for use as a baseline in tests.
func validManifest() registry.Manifest {
	return registry.Manifest{
		Name:        "example2snipe",
		DisplayName: "Example",
		Description: "Sync Example users to Snipe-IT license seats",
		Version:     "1.0.0",
		ConfigSchema: []registry.ConfigField{
			{Key: "example.api_key", Label: "Example API Key"},
		},
		Releases: registry.Releases{
			GitHubReleases: true,
			AssetPattern:   "example2snipe_{os}_{arch}",
		},
	}
}

func TestValidateManifest_Valid(t *testing.T) {
	if err := registry.ValidateManifest(validManifest()); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateManifest_MissingName(t *testing.T) {
	m := validManifest()
	m.Name = ""
	if err := registry.ValidateManifest(m); err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

func TestValidateManifest_MissingVersion(t *testing.T) {
	m := validManifest()
	m.Version = ""
	if err := registry.ValidateManifest(m); err == nil {
		t.Error("expected error for missing version, got nil")
	}
}

func TestValidateManifest_MissingConfigSchema(t *testing.T) {
	m := validManifest()
	m.ConfigSchema = nil
	if err := registry.ValidateManifest(m); err == nil {
		t.Error("expected error for empty config_schema, got nil")
	}
}

func TestValidateManifest_BadAssetPattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
	}{
		{"missing {os}", "example2snipe_{arch}"},
		{"missing {arch}", "example2snipe_{os}"},
		{"missing both", "example2snipe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			m.Releases.AssetPattern = tc.pattern
			if err := registry.ValidateManifest(m); err == nil {
				t.Errorf("expected error for asset_pattern %q, got nil", tc.pattern)
			}
		})
	}
}

func TestValidateManifest_BadVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
	}{
		{"letters only", "not-a-version"},
		{"missing patch", "1.0"},
		{"v prefix", "v1.0.0"},
		{"empty after trim", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			m.Version = tc.version
			if err := registry.ValidateManifest(m); err == nil {
				t.Errorf("expected error for version %q, got nil", tc.version)
			}
		})
	}
}
