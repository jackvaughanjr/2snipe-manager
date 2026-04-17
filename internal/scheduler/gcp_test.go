package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"github.com/jackvaughanjr/2snipe-manager/internal/scheduler"
)

// mockScheduler is an in-memory Scheduler implementation for testing.
type mockScheduler struct {
	jobs      map[string]scheduler.JobSpec
	disabled  map[string]bool
	triggered []string
}

func newMockScheduler() *mockScheduler {
	return &mockScheduler{
		jobs:     make(map[string]scheduler.JobSpec),
		disabled: make(map[string]bool),
	}
}

func (m *mockScheduler) CreateJob(_ context.Context, spec scheduler.JobSpec) error {
	m.jobs[spec.Name] = spec
	return nil
}

func (m *mockScheduler) DeleteJob(_ context.Context, name, _, _ string) error {
	delete(m.jobs, name)
	return nil
}

func (m *mockScheduler) EnableJob(_ context.Context, schedulerJobName string) error {
	m.disabled[schedulerJobName] = false
	return nil
}

func (m *mockScheduler) DisableJob(_ context.Context, schedulerJobName string) error {
	m.disabled[schedulerJobName] = true
	return nil
}

func (m *mockScheduler) TriggerJob(_ context.Context, jobName, _, _ string) (string, error) {
	m.triggered = append(m.triggered, jobName)
	return "projects/test/locations/us-central1/jobs/" + jobName + "/executions/exec-001", nil
}

func (m *mockScheduler) GetLastExecution(_ context.Context, jobName, _, _ string) (*scheduler.Execution, error) {
	if len(m.triggered) == 0 {
		return nil, nil
	}
	return &scheduler.Execution{
		Name:         "exec-001",
		ResourceName: "projects/test/locations/us-central1/jobs/" + jobName + "/executions/exec-001",
		Status:       "success",
		CompletedAt:  time.Now(),
	}, nil
}

// Ensure mockScheduler satisfies the interface.
var _ scheduler.Scheduler = (*mockScheduler)(nil)

func TestMockScheduler_CreateAndDelete(t *testing.T) {
	ctx := context.Background()
	m := newMockScheduler()

	spec := scheduler.JobSpec{
		Name:           "github2snipe",
		Project:        "test-project",
		Region:         "us-central1",
		Image:          "us-central1-docker.pkg.dev/test-project/2snipe/github2snipe:latest",
		ServiceAccount: "runner@test-project.iam.gserviceaccount.com",
		Schedule:       "0 6 * * *",
	}

	if err := m.CreateJob(ctx, spec); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if _, ok := m.jobs["github2snipe"]; !ok {
		t.Error("job was not created")
	}

	if err := m.DeleteJob(ctx, "github2snipe", "test-project", "us-central1"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if _, ok := m.jobs["github2snipe"]; ok {
		t.Error("job was not deleted")
	}

	// Deleting again should be idempotent (mock: just returns nil).
	if err := m.DeleteJob(ctx, "github2snipe", "test-project", "us-central1"); err != nil {
		t.Fatalf("DeleteJob (idempotent): %v", err)
	}
}

func TestBuildCloudRunJobSpec_EnvVars(t *testing.T) {
	spec := scheduler.JobSpec{
		Name:           "github2snipe",
		Project:        "my-project",
		Region:         "us-central1",
		Image:          "us-central1-docker.pkg.dev/my-project/2snipe/github2snipe:latest",
		ServiceAccount: "runner@my-project.iam.gserviceaccount.com",
		Schedule:       "0 6 * * *",
		ConfigFields: []registry.ConfigField{
			{Key: "snipe_it.url", Label: "Snipe-IT URL", Secret: false, Required: true},
			{Key: "snipe_it.api_key", Label: "Snipe-IT API Key", Secret: true, Required: true},
			{Key: "github.token", Label: "GitHub Token", Secret: true, Required: true},
		},
		SharedConfig: []string{"snipe_it"},
	}

	// Verify env var derivation for each field.
	tests := []struct {
		key     string
		wantEnv string
	}{
		{"snipe_it.url", "SNIPE_URL"},
		{"snipe_it.api_key", "SNIPE_TOKEN"},
		{"github.token", "GITHUB_TOKEN"},
	}
	for _, tt := range tests {
		var f registry.ConfigField
		for _, cf := range spec.ConfigFields {
			if cf.Key == tt.key {
				f = cf
				break
			}
		}
		got := scheduler.ConfigFieldToEnvVar(f)
		if got != tt.wantEnv {
			t.Errorf("ConfigFieldToEnvVar(%q) = %q, want %q", tt.key, got, tt.wantEnv)
		}
	}
}

func TestBuildCloudRunJobSpec_ExplicitEnvVar(t *testing.T) {
	// If env_var is set on the field, it must be used verbatim.
	f := registry.ConfigField{
		Key:    "google.credentials_json",
		Label:  "Service Account JSON",
		Secret: true,
		EnvVar: "GOOGLE_APPLICATION_CREDENTIALS_JSON",
	}
	got := scheduler.ConfigFieldToEnvVar(f)
	want := "GOOGLE_APPLICATION_CREDENTIALS_JSON"
	if got != want {
		t.Errorf("ConfigFieldToEnvVar with explicit EnvVar = %q, want %q", got, want)
	}
}

func TestBuildSchedulerJobSpec_CronPassthrough(t *testing.T) {
	ctx := context.Background()
	m := newMockScheduler()

	const cron = "0 6 * * *"
	spec := scheduler.JobSpec{
		Name:     "okta2snipe",
		Project:  "test-project",
		Region:   "us-central1",
		Schedule: cron,
	}
	if err := m.CreateJob(ctx, spec); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	stored := m.jobs["okta2snipe"]
	if stored.Schedule != cron {
		t.Errorf("stored schedule = %q, want %q", stored.Schedule, cron)
	}
}

func TestImagePath(t *testing.T) {
	got := scheduler.ImagePath("my-project", "us-central1", "github2snipe")
	want := "us-central1-docker.pkg.dev/my-project/2snipe/github2snipe:latest"
	if got != want {
		t.Errorf("ImagePath = %q, want %q", got, want)
	}
}
