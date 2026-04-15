package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// InstalledIntegration tracks the installed state of one integration.
type InstalledIntegration struct {
	Version        string `json:"version"`
	Enabled        bool   `json:"enabled"`
	Schedule       string `json:"schedule"`
	CloudRunJob    string `json:"cloud_run_job"`
	SchedulerJob   string `json:"scheduler_job"`
	SecretsBackend string `json:"secrets_backend"`
	InstalledAt    string `json:"installed_at"`
	LastRunAt      string `json:"last_run_at"`
	LastRunResult  string `json:"last_run_result"`
}

// State is the full content of ~/.snipemgr/state.json.
type State struct {
	Version      string                          `json:"version"`
	Integrations map[string]InstalledIntegration `json:"integrations"`
}

// InstalledVersions returns a map of integration name to installed version,
// suitable for cross-referencing with the registry.
func (s *State) InstalledVersions() map[string]string {
	m := make(map[string]string, len(s.Integrations))
	for name, intg := range s.Integrations {
		m[name] = intg.Version
	}
	return m
}

// ReadState reads the state file at path, creating an empty state file (and
// parent directory) if the file does not exist.  Returns an empty State, not
// an error, when the file is missing.
func ReadState(path string) (*State, error) {
	p, err := expandPath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		empty := emptyState()
		return empty, writeState(p, empty)
	}
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "{}" {
		return emptyState(), nil
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Integrations == nil {
		s.Integrations = make(map[string]InstalledIntegration)
	}
	return &s, nil
}

func emptyState() *State {
	return &State{
		Version:      "1",
		Integrations: make(map[string]InstalledIntegration),
	}
}

func writeState(path string, s *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
