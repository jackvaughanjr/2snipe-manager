// Package secrets provides a GCP Secret Manager client for reading and writing
// integration credentials. The SecretManager interface enables mock-based unit
// testing without live API calls.
package secrets

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SecretManager is the interface for reading and writing secrets.
// All operations accept a logical secret name using "/" as the separator
// (e.g. "snipe/snipe-url", "github2snipe/token"). The implementation handles
// encoding the name to a valid GCP Secret Manager ID.
type SecretManager interface {
	Get(ctx context.Context, name string) (string, error)
	Set(ctx context.Context, name, value string) error
	Exists(ctx context.Context, name string) (bool, error)
	ListByPrefix(ctx context.Context, prefix string) ([]string, error)
}

// GCPSecretManager is the production Secret Manager implementation backed by
// GCP Secret Manager. Auth order: ADC first → credentials_file fallback.
type GCPSecretManager struct {
	client  *secretmanager.Client
	project string
}

// NewGCPSecretManager creates a GCPSecretManager. If credentialsFile is
// non-empty, that service account key file is used; otherwise Application
// Default Credentials are used.
func NewGCPSecretManager(ctx context.Context, project, credentialsFile string) (*GCPSecretManager, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Secret Manager client: %w", err)
	}
	return &GCPSecretManager{client: client, project: project}, nil
}

// Close releases the underlying gRPC connection.
func (m *GCPSecretManager) Close() error {
	return m.client.Close()
}

// Get retrieves the latest version of the named secret. Returns an error if
// the secret does not exist.
func (m *GCPSecretManager) Get(ctx context.Context, name string) (string, error) {
	versionName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest",
		m.project, encodeSecretID(name))
	req := &secretmanagerpb.AccessSecretVersionRequest{Name: versionName}
	result, err := m.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("accessing secret %q: %w", name, err)
	}
	return string(result.Payload.Data), nil
}

// Set creates or updates the named secret. If the secret does not yet exist it
// is created with automatic replication; a new version is always added.
func (m *GCPSecretManager) Set(ctx context.Context, name, value string) error {
	secretName := fmt.Sprintf("projects/%s/secrets/%s", m.project, encodeSecretID(name))

	// Create the secret resource if it does not already exist.
	exists, err := m.Exists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		_, err = m.client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
			Parent:   fmt.Sprintf("projects/%s", m.project),
			SecretId: encodeSecretID(name),
			Secret: &secretmanagerpb.Secret{
				Replication: &secretmanagerpb.Replication{
					Replication: &secretmanagerpb.Replication_Automatic_{
						Automatic: &secretmanagerpb.Replication_Automatic{},
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("creating secret %q: %w", name, err)
		}
	}

	// Add a new secret version with the payload.
	_, err = m.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(value),
		},
	})
	if err != nil {
		return fmt.Errorf("adding version for secret %q: %w", name, err)
	}
	return nil
}

// Exists reports whether the named secret exists (regardless of whether it has
// any versions).
func (m *GCPSecretManager) Exists(ctx context.Context, name string) (bool, error) {
	secretName := fmt.Sprintf("projects/%s/secrets/%s", m.project, encodeSecretID(name))
	_, err := m.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: secretName})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("checking secret %q: %w", name, err)
	}
	return true, nil
}

// ListByPrefix returns the logical names of all secrets whose encoded name
// begins with the encoded form of prefix. The returned names use "/" as the
// separator, matching the input convention.
func (m *GCPSecretManager) ListByPrefix(ctx context.Context, prefix string) ([]string, error) {
	encodedPrefix := encodeSecretID(prefix)
	req := &secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", m.project),
	}
	it := m.client.ListSecrets(ctx, req)

	var names []string
	for {
		secret, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("listing secrets: %w", err)
		}
		// Extract the secret ID from the full resource name.
		parts := strings.Split(secret.Name, "/")
		id := parts[len(parts)-1]
		if strings.HasPrefix(id, encodedPrefix) {
			names = append(names, decodeSecretID(id))
		}
	}
	return names, nil
}

// encodeSecretID converts a logical name (e.g. "snipe/snipe-url") to a valid
// GCP Secret Manager ID by replacing "/" with "--".
func encodeSecretID(logicalName string) string {
	return strings.ReplaceAll(logicalName, "/", "--")
}

// decodeSecretID reverses encodeSecretID.
func decodeSecretID(id string) string {
	return strings.ReplaceAll(id, "--", "/")
}
