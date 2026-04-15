package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jackvaughanjr/2snipe-manager/internal/state"
)

func TestReadState_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := state.ReadState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Integrations) != 0 {
		t.Errorf("expected empty integrations, got %d entries", len(s.Integrations))
	}
	// Verify the file was created.
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("expected state file to be created at %s: %v", path, statErr)
	}
}

func TestReadState_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := state.ReadState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Integrations) != 0 {
		t.Errorf("expected empty integrations, got %d entries", len(s.Integrations))
	}
}

func TestReadState_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	fixture := `{
  "version": "1",
  "integrations": {
    "github2snipe": {
      "version": "0.9.0",
      "enabled": true,
      "last_run_result": "success"
    }
  }
}`
	if err := os.WriteFile(path, []byte(fixture), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := state.ReadState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Version != "1" {
		t.Errorf("expected version 1, got %q", s.Version)
	}
	intg, ok := s.Integrations["github2snipe"]
	if !ok {
		t.Fatal("expected github2snipe in integrations")
	}
	if intg.Version != "0.9.0" {
		t.Errorf("expected version 0.9.0, got %q", intg.Version)
	}
	if !intg.Enabled {
		t.Error("expected enabled to be true")
	}
	if intg.LastRunResult != "success" {
		t.Errorf("expected last_run_result success, got %q", intg.LastRunResult)
	}

	versions := s.InstalledVersions()
	if versions["github2snipe"] != "0.9.0" {
		t.Errorf("InstalledVersions: expected 0.9.0, got %q", versions["github2snipe"])
	}
}
