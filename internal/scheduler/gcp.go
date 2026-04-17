// Package scheduler manages Cloud Run Jobs and Cloud Scheduler triggers for
// *2snipe integrations. The Scheduler interface enables mock-based unit testing
// without live API calls.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	scheduler "cloud.google.com/go/scheduler/apiv1"
	"cloud.google.com/go/scheduler/apiv1/schedulerpb"
	"github.com/jackvaughanjr/2snipe-manager/internal/registry"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ErrImageNotFound is returned by CreateJob when GCP cannot find the container
// image in Artifact Registry. The Cloud Run Job resource IS created in GCP (in
// a failed state) and will become usable once the image is pushed.
var ErrImageNotFound = errors.New("container image not found in Artifact Registry")

// JobSpec describes the Cloud Run Job + optional Cloud Scheduler trigger to create.
type JobSpec struct {
	// Integration name (e.g. "github2snipe") — used as the job ID.
	Name string
	// GCP project ID.
	Project string
	// GCP region (e.g. "us-central1").
	Region string
	// Full Artifact Registry image path, e.g.
	// "us-central1-docker.pkg.dev/my-project/2snipe/github2snipe:latest"
	Image string
	// Service account email for the Cloud Run Job runtime.
	ServiceAccount string
	// Cron expression for Cloud Scheduler (e.g. "0 6 * * *").
	// Empty or "manual" → no Cloud Scheduler trigger is created.
	Schedule string
	// IANA timezone for the Cloud Scheduler cron expression (e.g. "America/New_York").
	// Defaults to "UTC" when empty.
	Timezone string
	// Config fields from the manifest — drives secret env var mapping.
	ConfigFields []registry.ConfigField
	// SharedConfig prefixes from the manifest (e.g. ["snipe_it"]).
	SharedConfig []string
}

// Execution holds the result of a Cloud Run Job execution.
type Execution struct {
	// Short execution name (last segment of resource name).
	Name string
	// Full resource name.
	ResourceName string
	// "success", "failed", "running", "cancelled", "unknown"
	Status string
	// Zero if not yet completed.
	CompletedAt time.Time
}

// Scheduler manages Cloud Run Jobs and Cloud Scheduler triggers.
type Scheduler interface {
	CreateJob(ctx context.Context, spec JobSpec) error
	DeleteJob(ctx context.Context, name, project, region string) error
	EnableJob(ctx context.Context, schedulerJobName string) error
	DisableJob(ctx context.Context, schedulerJobName string) error
	TriggerJob(ctx context.Context, jobName, project, region string) (string, error)
	GetLastExecution(ctx context.Context, jobName, project, region string) (*Execution, error)
}

// GCPScheduler is the production Scheduler implementation.
type GCPScheduler struct {
	runClient  *run.JobsClient
	execClient *run.ExecutionsClient
	schClient  *scheduler.CloudSchedulerClient
}

// NewGCPScheduler creates a GCPScheduler. If credentialsFile is non-empty,
// that SA key file is used; otherwise Application Default Credentials are used.
func NewGCPScheduler(ctx context.Context, credentialsFile string) (*GCPScheduler, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	runClient, err := run.NewJobsClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Cloud Run Jobs client: %w", err)
	}
	execClient, err := run.NewExecutionsClient(ctx, opts...)
	if err != nil {
		runClient.Close()
		return nil, fmt.Errorf("creating Cloud Run Executions client: %w", err)
	}
	schClient, err := scheduler.NewCloudSchedulerClient(ctx, opts...)
	if err != nil {
		runClient.Close()
		execClient.Close()
		return nil, fmt.Errorf("creating Cloud Scheduler client: %w", err)
	}
	return &GCPScheduler{runClient: runClient, execClient: execClient, schClient: schClient}, nil
}

// Close releases the underlying gRPC connections.
func (g *GCPScheduler) Close() {
	g.runClient.Close()
	g.execClient.Close()
	g.schClient.Close()
}

// CreateJob creates a Cloud Run Job and, if spec.Schedule is set, a Cloud
// Scheduler trigger that fires the job on the given cron schedule.
//
// If the container image is not yet in Artifact Registry, GCP creates the job
// resource in a degraded state and the LRO returns an image-not-found error.
// In that case CreateJob returns ErrImageNotFound — the caller should print
// build+push instructions and record the job in state (it exists in GCP and
// will work once the image is pushed). The Cloud Scheduler trigger is NOT
// created when ErrImageNotFound is returned.
//
// AlreadyExists errors (re-install after a prior attempt) are silently
// treated as success.
func (g *GCPScheduler) CreateJob(ctx context.Context, spec JobSpec) error {
	// Build the Cloud Run Job.
	job := buildCloudRunJob(spec)
	parent := fmt.Sprintf("projects/%s/locations/%s", spec.Project, spec.Region)

	op, err := g.runClient.CreateJob(ctx, &runpb.CreateJobRequest{
		Parent: parent,
		JobId:  spec.Name,
		Job:    job,
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			// Job already exists from a prior install attempt — skip creation.
			return g.createSchedulerIfNeeded(ctx, spec, parent)
		}
		return fmt.Errorf("creating Cloud Run Job %q: %w", spec.Name, err)
	}

	if _, err := op.Wait(ctx); err != nil {
		if isImageNotFoundErr(err) {
			// Job resource was created by GCP but is in a failed state because
			// the image doesn't exist yet. Report this as ErrImageNotFound so the
			// caller can print instructions. No scheduler trigger is created.
			return ErrImageNotFound
		}
		if status.Code(err) == codes.AlreadyExists {
			return g.createSchedulerIfNeeded(ctx, spec, parent)
		}
		return fmt.Errorf("waiting for Cloud Run Job creation %q: %w", spec.Name, err)
	}

	return g.createSchedulerIfNeeded(ctx, spec, parent)
}

// createSchedulerIfNeeded creates the Cloud Scheduler trigger when a non-manual
// schedule is configured. AlreadyExists is silently ignored.
func (g *GCPScheduler) createSchedulerIfNeeded(ctx context.Context, spec JobSpec, parent string) error {
	if spec.Schedule == "" || spec.Schedule == "manual" {
		return nil
	}
	schJob := buildSchedulerJob(spec)
	_, err := g.schClient.CreateJob(ctx, &schedulerpb.CreateJobRequest{
		Parent: parent,
		Job:    schJob,
	})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("creating Cloud Scheduler trigger for %q: %w", spec.Name, err)
	}
	return nil
}

// isImageNotFoundErr returns true when the error message from GCP indicates
// that the container image was not found in Artifact Registry.
func isImageNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "image") && strings.Contains(msg, "not found")
}

// DeleteJob deletes the Cloud Run Job and its Cloud Scheduler trigger.
// NotFound errors are silently ignored so that partial cleanups are safe.
func (g *GCPScheduler) DeleteJob(ctx context.Context, name, project, region string) error {
	jobName := fmt.Sprintf("projects/%s/locations/%s/jobs/%s", project, region, name)
	op, err := g.runClient.DeleteJob(ctx, &runpb.DeleteJobRequest{Name: jobName})
	if err != nil && status.Code(err) != codes.NotFound {
		return fmt.Errorf("deleting Cloud Run Job %q: %w", name, err)
	}
	if err == nil {
		if _, err := op.Wait(ctx); err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("waiting for Cloud Run Job deletion %q: %w", name, err)
		}
	}

	// Delete the scheduler trigger (name-trigger convention).
	triggerName := fmt.Sprintf("projects/%s/locations/%s/jobs/%s-trigger", project, region, name)
	err = g.schClient.DeleteJob(ctx, &schedulerpb.DeleteJobRequest{Name: triggerName})
	if err != nil && status.Code(err) != codes.NotFound {
		return fmt.Errorf("deleting Cloud Scheduler trigger for %q: %w", name, err)
	}
	return nil
}

// EnableJob resumes a paused Cloud Scheduler job.
func (g *GCPScheduler) EnableJob(ctx context.Context, schedulerJobName string) error {
	_, err := g.schClient.ResumeJob(ctx, &schedulerpb.ResumeJobRequest{Name: schedulerJobName})
	if err != nil {
		return fmt.Errorf("enabling scheduler job %q: %w", schedulerJobName, err)
	}
	return nil
}

// DisableJob pauses a Cloud Scheduler job without deleting it.
func (g *GCPScheduler) DisableJob(ctx context.Context, schedulerJobName string) error {
	_, err := g.schClient.PauseJob(ctx, &schedulerpb.PauseJobRequest{Name: schedulerJobName})
	if err != nil {
		return fmt.Errorf("disabling scheduler job %q: %w", schedulerJobName, err)
	}
	return nil
}

// TriggerJob triggers the Cloud Run Job immediately and returns the execution
// resource name. It does not wait for the execution to complete.
func (g *GCPScheduler) TriggerJob(ctx context.Context, jobName, project, region string) (string, error) {
	fullName := fmt.Sprintf("projects/%s/locations/%s/jobs/%s", project, region, jobName)
	op, err := g.runClient.RunJob(ctx, &runpb.RunJobRequest{Name: fullName})
	if err != nil {
		return "", fmt.Errorf("triggering Cloud Run Job %q: %w", jobName, err)
	}
	// Extract the execution name from the operation metadata without blocking.
	meta, err := op.Metadata()
	if err != nil || meta == nil || meta.Name == "" {
		// Fall back to the operation name as the execution reference.
		return op.Name(), nil
	}
	return meta.Name, nil
}

// GetLastExecution fetches the most recent execution for the named job.
// Returns nil (with no error) if no executions exist yet.
// The API returns executions in reverse chronological order (newest first).
func (g *GCPScheduler) GetLastExecution(ctx context.Context, jobName, project, region string) (*Execution, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s/jobs/%s", project, region, jobName)
	req := &runpb.ListExecutionsRequest{
		Parent:   parent,
		PageSize: 1,
	}
	it := g.execClient.ListExecutions(ctx, req)
	exec, err := it.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing executions for %q: %w", jobName, err)
	}
	return executionFromProto(exec), nil
}

// --- helpers ---

// buildCloudRunJob constructs the Cloud Run Job spec from a JobSpec.
func buildCloudRunJob(spec JobSpec) *runpb.Job {
	envVars := buildEnvVars(spec)
	return &runpb.Job{
		Template: &runpb.ExecutionTemplate{
			Template: &runpb.TaskTemplate{
				Containers: []*runpb.Container{{
					Image:   spec.Image,
					Command: []string{"/app/" + spec.Name, "sync"},
					Env:     envVars,
					Resources: &runpb.ResourceRequirements{
						Limits: map[string]string{
							"cpu":    "1",
							"memory": "512Mi",
						},
					},
				}},
				ServiceAccount: spec.ServiceAccount,
				Retries:        &runpb.TaskTemplate_MaxRetries{MaxRetries: 1},
				Timeout:        durationpb.New(300 * time.Second),
			},
		},
	}
}

// buildEnvVars maps each config field to a Cloud Run Job env var backed by a
// Secret Manager secret reference.
func buildEnvVars(spec JobSpec) []*runpb.EnvVar {
	var envs []*runpb.EnvVar
	sharedSet := make(map[string]bool, len(spec.SharedConfig))
	for _, p := range spec.SharedConfig {
		sharedSet[p] = true
	}

	for _, f := range spec.ConfigFields {
		envVarName := ConfigFieldToEnvVar(f)
		secretLogicalName := configFieldToSecretName(spec.Name, f, sharedSet)
		secretResourceName := fmt.Sprintf("projects/%s/secrets/%s",
			spec.Project, encodeSecretID(secretLogicalName))

		envs = append(envs, &runpb.EnvVar{
			Name: envVarName,
			Values: &runpb.EnvVar_ValueSource{
				ValueSource: &runpb.EnvVarSource{
					SecretKeyRef: &runpb.SecretKeySelector{
						Secret:  secretResourceName,
						Version: "latest",
					},
				},
			},
		})
	}
	return envs
}

// buildSchedulerJob constructs the Cloud Scheduler job spec.
func buildSchedulerJob(spec JobSpec) *schedulerpb.Job {
	// Cloud Run Jobs v2 run endpoint.
	runURI := fmt.Sprintf(
		"https://run.googleapis.com/v2/projects/%s/locations/%s/jobs/%s:run",
		spec.Project, spec.Region, spec.Name,
	)
	jobName := fmt.Sprintf("projects/%s/locations/%s/jobs/%s-trigger",
		spec.Project, spec.Region, spec.Name)
	tz := spec.Timezone
	if tz == "" {
		tz = "UTC"
	}
	return &schedulerpb.Job{
		Name:     jobName,
		Schedule: spec.Schedule,
		TimeZone: tz,
		Target: &schedulerpb.Job_HttpTarget{
			HttpTarget: &schedulerpb.HttpTarget{
				Uri:        runURI,
				HttpMethod: schedulerpb.HttpMethod_POST,
				AuthorizationHeader: &schedulerpb.HttpTarget_OauthToken{
					OauthToken: &schedulerpb.OAuthToken{
						ServiceAccountEmail: spec.ServiceAccount,
					},
				},
			},
		},
	}
}

// ConfigFieldToEnvVar derives the environment variable name for a config field.
// If ConfigField.EnvVar is set it is used directly. Otherwise, well-known
// shared snipe_it keys are mapped to their canonical env var names, and all
// other keys are uppercased with dots replaced by underscores.
func ConfigFieldToEnvVar(f registry.ConfigField) string {
	if f.EnvVar != "" {
		return f.EnvVar
	}
	// Well-known shared credentials that all integrations bind explicitly.
	switch f.Key {
	case "snipe_it.url":
		return "SNIPE_URL"
	case "snipe_it.api_key":
		return "SNIPE_TOKEN"
	}
	// General rule: "github.token" → "GITHUB_TOKEN"
	return strings.ToUpper(strings.ReplaceAll(f.Key, ".", "_"))
}

// configFieldToSecretName returns the logical Secret Manager name for a field.
// Shared-config fields use well-known paths (e.g. "snipe/snipe-url");
// integration-specific fields use "{integration}/{last-segment-as-kebab}".
func configFieldToSecretName(integrationName string, f registry.ConfigField, sharedPrefixes map[string]bool) string {
	prefix := keyPrefix(f.Key)
	if sharedPrefixes[prefix] {
		// Map well-known shared fields to their canonical secret names.
		switch f.Key {
		case "snipe_it.url":
			return "snipe/snipe-url"
		case "snipe_it.api_key":
			return "snipe/snipe-token"
		}
		// Fallback for unknown shared-config fields.
		return "snipe/" + keyLastSegment(f.Key, prefix)
	}
	// Integration-specific: "{name}/{last-segment-as-kebab}"
	return integrationName + "/" + keyLastSegment(f.Key, prefix)
}

// keyPrefix returns the dot-notation prefix (everything before the first dot).
func keyPrefix(key string) string {
	if i := strings.Index(key, "."); i >= 0 {
		return key[:i]
	}
	return key
}

// keyLastSegment returns the last dot-notation segment as kebab-case.
// e.g. "github.api_token" → "api-token"
func keyLastSegment(key, prefix string) string {
	seg := key
	if prefix != "" && strings.HasPrefix(key, prefix+".") {
		seg = key[len(prefix)+1:]
	}
	return strings.ReplaceAll(seg, "_", "-")
}

// encodeSecretID converts a logical name (e.g. "snipe/snipe-url") to a valid
// GCP Secret Manager ID by replacing "/" with "--".
func encodeSecretID(logicalName string) string {
	return strings.ReplaceAll(logicalName, "/", "--")
}

// executionFromProto converts a runpb.Execution to our summary Execution type.
func executionFromProto(e *runpb.Execution) *Execution {
	parts := strings.Split(e.Name, "/")
	shortName := parts[len(parts)-1]

	execStatus := "unknown"
	var completedAt time.Time

	// Determine status from task counts and completion state.
	switch {
	case e.SucceededCount > 0:
		execStatus = "success"
	case e.FailedCount > 0:
		execStatus = "failed"
	case e.RunningCount > 0:
		execStatus = "running"
	}

	if e.CompletionTime != nil {
		completedAt = e.CompletionTime.AsTime()
	}

	return &Execution{
		Name:         shortName,
		ResourceName: e.Name,
		Status:       execStatus,
		CompletedAt:  completedAt,
	}
}

// ImagePath returns the expected Artifact Registry image path for an integration.
// Pattern: {region}-docker.pkg.dev/{project}/2snipe/{name}:latest
func ImagePath(project, region, name string) string {
	return fmt.Sprintf("%s-docker.pkg.dev/%s/2snipe/%s:latest", region, project, name)
}

// CloudRunJobName returns the full resource name for a Cloud Run Job.
func CloudRunJobName(project, region, name string) string {
	return fmt.Sprintf("projects/%s/locations/%s/jobs/%s", project, region, name)
}

// SchedulerJobName returns the full resource name for a Cloud Scheduler trigger.
func SchedulerJobName(project, region, name string) string {
	return fmt.Sprintf("projects/%s/locations/%s/jobs/%s-trigger", project, region, name)
}
