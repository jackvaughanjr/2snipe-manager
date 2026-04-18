package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackvaughanjr/2snipe-manager/internal/scheduler"
	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Trigger an integration's Cloud Run Job immediately",
	Long: `Trigger the Cloud Run Job for an installed integration outside of its
scheduled time. The job runs asynchronously — check results with 'snipemgr status'.`,
	Args: cobra.ExactArgs(1),
	RunE: silentUsage(runRun),
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(_ *cobra.Command, args []string) error {
	name := args[0]

	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}
	s, err := state.ReadState(statePath)
	if err != nil {
		return fatal("reading state: %v", err)
	}

	intg, ok := s.Integrations[name]
	if !ok {
		return fatal("integration %q is not installed", name)
	}
	if intg.SecretsBackend != "gcp" {
		return fatal("integration %q uses the local backend — 'run' requires the GCP backend", name)
	}
	if intg.CloudRunJob == "" {
		return fatal("integration %q has no Cloud Run Job — reinstall with --secrets-backend gcp", name)
	}

	project := viper.GetString("gcp.project")
	region := viper.GetString("gcp.region")
	credFile := viper.GetString("gcp.credentials_file")

	if project == "" {
		return fatal("gcp.project is not set in snipemgr.yaml")
	}
	if region == "" {
		region = "us-central1"
	}

	// If this integration has never run successfully, proactively print
	// the container image build+push instructions before triggering.
	if intg.LastRunResult == "" {
		fmt.Fprintln(os.Stderr, imageInstructions(name, project, region))
	}

	ctx := context.Background()
	sched, err := scheduler.NewGCPScheduler(ctx, credFile)
	if err != nil {
		return fatalGCP("connecting to GCP", err)
	}
	defer sched.Close()

	fmt.Printf("Triggering %s...\n", name)
	execName, err := sched.TriggerJob(ctx, name, project, region)
	if err != nil {
		if intg.LastRunResult == "" {
			// Image instructions were already printed above. Give a targeted hint
			// rather than a raw gRPC error — the job is likely in error state
			// because the container image hasn't been pushed yet.
			fmt.Fprintf(os.Stderr, "Error: job is not ready to run — push the container image first (see instructions above).\n")
			return fmt.Errorf("job not ready")
		}
		if isImageError(err) {
			fmt.Fprintln(os.Stderr, imageInstructions(name, project, region))
		}
		return fatal("triggering job: %v", err)
	}

	// Extract the short execution ID from the resource name.
	parts := strings.Split(execName, "/")
	shortID := parts[len(parts)-1]

	fmt.Printf("✓ Execution started: %s\n", shortID)
	fmt.Printf("  Check status:  snipemgr status\n")
	fmt.Printf("  Full name:     %s\n", execName)

	// Update state with the trigger time.
	intg.LastRunAt = time.Now().UTC().Format(time.RFC3339)
	s.Integrations[name] = intg
	if err := state.WriteState(statePath, s); err != nil {
		return fatal("writing state: %v", err)
	}

	return nil
}

// isImageError returns true when the error message suggests a container image
// pull failure (image not found in Artifact Registry).
func isImageError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "image") ||
		strings.Contains(msg, "container") ||
		strings.Contains(msg, "registry") ||
		strings.Contains(msg, "pull")
}

// imageInstructions returns the step-by-step Docker build+push guide for an
// integration, formatted for terminal output.
func imageInstructions(name, project, region string) string {
	image := scheduler.ImagePath(project, region, name)
	repo := fmt.Sprintf("https://github.com/jackvaughanjr/%s.git", name)

	return fmt.Sprintf(`
─────────────────────────────────────────────────────────────────────────────
  Container image required for Cloud Run Jobs
─────────────────────────────────────────────────────────────────────────────
  Before %s can run via Cloud Run Jobs, its container image must be built
  and pushed to Artifact Registry. Follow these steps once per integration:

  1. Create the Artifact Registry repository (one-time per project):
       gcloud artifacts repositories create snipe-integrations \
         --repository-format=docker \
         --location=%s \
         --project=%s \
         --description="snipe-integrations container images"

  2. Clone the integration source:
       git clone %s
       cd %s

  3. Create a Dockerfile if one does not already exist:
       cat > Dockerfile <<'EOF'
       FROM golang:1.23-alpine AS builder
       WORKDIR /src
       COPY . .
       RUN go build -o /app/%s .
       FROM alpine:3.21
       COPY --from=builder /app/%s /app/%s
       ENTRYPOINT ["/app/%s"]
       EOF

  4. Build and push — choose one method:

     Option A: Docker (requires Docker installed and running)
       gcloud auth configure-docker %s-docker.pkg.dev
       docker build -t %s .
       docker push %s

     Option B: Cloud Build (no Docker required — builds via GCP)
       gcloud services enable cloudbuild.googleapis.com --project=%s
       gcloud builds submit --tag %s --project=%s .

  5. Re-run:
       snipemgr run %s
─────────────────────────────────────────────────────────────────────────────
`,
		name, region, project,
		repo, name,
		name, name, name, name,
		region, image, image,
		project, image, project,
		name,
	)
}
