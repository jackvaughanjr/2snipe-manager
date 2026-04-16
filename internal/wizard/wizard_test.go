package wizard_test

import (
	"strings"
	"testing"

	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/wizard"
)

var testSchema = []registry.ConfigField{
	{Key: "snipe_it.url", Label: "Snipe-IT URL", Required: true},
	{Key: "snipe_it.api_key", Label: "Snipe-IT API Key", Required: true, Secret: true},
	{Key: "github.token", Label: "GitHub Token", Required: true, Secret: true},
	{Key: "slack.webhook_url", Label: "Slack Webhook URL", Required: false},
}

func TestBuildFlagDefaults(t *testing.T) {
	flags := map[string]string{
		"snipe_it.url":     "https://snipe.example.com",
		"snipe_it.api_key": "tok123",
		"github.token":     "ghp_abc",
	}

	values, err := wizard.BuildFlagDefaults(testSchema, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if values["snipe_it.url"] != "https://snipe.example.com" {
		t.Errorf("expected snipe_it.url to be set, got %q", values["snipe_it.url"])
	}
	if values["snipe_it.api_key"] != "tok123" {
		t.Errorf("expected snipe_it.api_key to be set, got %q", values["snipe_it.api_key"])
	}
	if values["github.token"] != "ghp_abc" {
		t.Errorf("expected github.token to be set, got %q", values["github.token"])
	}
	// Optional field with no value should be present but empty.
	if _, ok := values["slack.webhook_url"]; !ok {
		t.Error("expected slack.webhook_url key to be present in result")
	}
}

func TestBuildFlagDefaults_Default(t *testing.T) {
	schema := []registry.ConfigField{
		{Key: "app.region", Label: "Region", Required: true, Default: "us-east-1"},
	}
	values, err := wizard.BuildFlagDefaults(schema, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if values["app.region"] != "us-east-1" {
		t.Errorf("expected default value 'us-east-1', got %q", values["app.region"])
	}
}

func TestBuildFlagDefaults_MissingRequired(t *testing.T) {
	// Omit required field "snipe_it.api_key" from flags.
	flags := map[string]string{
		"snipe_it.url": "https://snipe.example.com",
		"github.token": "ghp_abc",
	}

	_, err := wizard.BuildFlagDefaults(testSchema, flags)
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
	// Error message must name the missing field.
	if !strings.Contains(err.Error(), "snipe_it.api_key") {
		t.Errorf("error message should mention the missing key, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Snipe-IT API Key") {
		t.Errorf("error message should mention the field label, got: %v", err)
	}
}
