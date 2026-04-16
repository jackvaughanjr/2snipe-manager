package state_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestWriteState_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := &state.State{
		Version: "1",
		Integrations: map[string]state.InstalledIntegration{
			"github2snipe": {Version: "1.0.0", Enabled: true},
		},
	}
	if err := state.WriteState(path, s); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	// File must exist and be valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}
	var out state.State
	if err := json.Unmarshal(data, &out); err != nil {
		t.Errorf("state file is not valid JSON: %v\n%s", err, data)
	}

	// Temp file must not be left behind.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("expected .tmp file to be cleaned up after atomic write")
	}
}

func TestWriteState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	orig := &state.State{
		Version: "1",
		Integrations: map[string]state.InstalledIntegration{
			"okta2snipe": {Version: "2.3.1", Enabled: true, Schedule: "0 6 * * *"},
		},
	}
	if err := state.WriteState(path, orig); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	got, err := state.ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	intg, ok := got.Integrations["okta2snipe"]
	if !ok {
		t.Fatal("expected okta2snipe after round-trip")
	}
	if intg.Version != "2.3.1" {
		t.Errorf("expected version 2.3.1, got %q", intg.Version)
	}
	if intg.Schedule != "0 6 * * *" {
		t.Errorf("expected schedule '0 6 * * *', got %q", intg.Schedule)
	}
}

func TestWriteState_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			s := &state.State{
				Version: "1",
				Integrations: map[string]state.InstalledIntegration{
					fmt.Sprintf("intg%d", i): {Version: "1.0.0"},
				},
			}
			_ = state.WriteState(path, s)
		}(i)
	}
	wg.Wait()

	// After all concurrent writes, the file must be valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state file after concurrent writes: %v", err)
	}
	var s state.State
	if err := json.Unmarshal(data, &s); err != nil {
		t.Errorf("state file is not valid JSON after concurrent writes: %v\n%s", err, data)
	}
}
