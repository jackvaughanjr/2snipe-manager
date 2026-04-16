package installer

import (
	"strings"
	"testing"

	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
)

func TestResolveAssetURL_Darwin_ARM64(t *testing.T) {
	assets := []Asset{
		{Name: "foo-darwin-arm64", DownloadURL: "https://example.com/foo-darwin-arm64"},
		{Name: "foo-linux-amd64", DownloadURL: "https://example.com/foo-linux-amd64"},
		{Name: "foo-windows-amd64.exe", DownloadURL: "https://example.com/foo-windows-amd64.exe"},
	}
	url, ok := ResolveAssetURL(assets, "foo-{os}-{arch}", "darwin", "arm64")
	if !ok {
		t.Fatal("expected asset to be found for darwin/arm64")
	}
	if url != "https://example.com/foo-darwin-arm64" {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestResolveAssetURL_Linux_AMD64(t *testing.T) {
	assets := []Asset{
		{Name: "foo-darwin-arm64", DownloadURL: "https://example.com/foo-darwin-arm64"},
		{Name: "foo-linux-amd64", DownloadURL: "https://example.com/foo-linux-amd64"},
	}
	url, ok := ResolveAssetURL(assets, "foo-{os}-{arch}", "linux", "amd64")
	if !ok {
		t.Fatal("expected asset to be found for linux/amd64")
	}
	if url != "https://example.com/foo-linux-amd64" {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestResolveAssetURL_Windows(t *testing.T) {
	assets := []Asset{
		{Name: "foo-windows-amd64.exe", DownloadURL: "https://example.com/foo-windows-amd64.exe"},
	}
	url, ok := ResolveAssetURL(assets, "foo-{os}-{arch}", "windows", "amd64")
	if !ok {
		t.Fatal("expected .exe asset to be found for windows/amd64")
	}
	if url != "https://example.com/foo-windows-amd64.exe" {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestWriteSettingsYAML(t *testing.T) {
	schema := []registry.ConfigField{
		{Key: "snipe_it.url", Label: "Snipe-IT URL", Required: true, Hint: "Full URL including https://"},
		{Key: "snipe_it.api_key", Label: "API Key", Secret: true, Required: true},
		{Key: "github.token", Label: "GitHub Token", Required: true, Hint: "PAT with read:org scope"},
	}
	values := map[string]string{
		"snipe_it.url":    "https://snipe.example.com",
		"snipe_it.api_key": "tok123",
		"github.token":    "ghp_abc",
	}

	content := buildSettingsYAML(schema, values)
	s := string(content)

	// Section headers must be present.
	if !strings.Contains(s, "snipe_it:\n") {
		t.Error("expected 'snipe_it:' section in output")
	}
	if !strings.Contains(s, "github:\n") {
		t.Error("expected 'github:' section in output")
	}

	// Sub-keys must be present (indented).
	if !strings.Contains(s, "  url:") {
		t.Error("expected '  url:' in snipe_it section")
	}
	if !strings.Contains(s, "  api_key:") {
		t.Error("expected '  api_key:' in snipe_it section")
	}
	if !strings.Contains(s, "  token:") {
		t.Error("expected '  token:' in github section")
	}

	// Values must be present.
	if !strings.Contains(s, "https://snipe.example.com") {
		t.Error("expected snipe URL value in output")
	}

	// Hints must appear as comments.
	if !strings.Contains(s, "# Full URL including https://") {
		t.Error("expected hint comment in output")
	}
}
