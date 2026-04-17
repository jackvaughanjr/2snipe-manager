package secrets_test

import (
	"context"
	"testing"

	"github.com/jackvaughanjr/2snipe-manager/internal/secrets"
)

// mockSecretManager is an in-memory SecretManager for testing.
type mockSecretManager struct {
	store map[string]string
}

func newMock() *mockSecretManager {
	return &mockSecretManager{store: make(map[string]string)}
}

func (m *mockSecretManager) Get(_ context.Context, name string) (string, error) {
	v, ok := m.store[name]
	if !ok {
		return "", &mockNotFoundError{name: name}
	}
	return v, nil
}

func (m *mockSecretManager) Set(_ context.Context, name, value string) error {
	m.store[name] = value
	return nil
}

func (m *mockSecretManager) Exists(_ context.Context, name string) (bool, error) {
	_, ok := m.store[name]
	return ok, nil
}

func (m *mockSecretManager) ListByPrefix(_ context.Context, prefix string) ([]string, error) {
	var out []string
	for k := range m.store {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, k)
		}
	}
	return out, nil
}

type mockNotFoundError struct{ name string }

func (e *mockNotFoundError) Error() string { return "secret not found: " + e.name }

// Ensure mockSecretManager satisfies the interface.
var _ secrets.SecretManager = (*mockSecretManager)(nil)

func TestMockSecretManager_SetAndGet(t *testing.T) {
	ctx := context.Background()
	m := newMock()

	const name = "github2snipe/token"
	const value = "ghp_testtoken123"

	if err := m.Set(ctx, name, value); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := m.Get(ctx, name)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != value {
		t.Errorf("Get = %q, want %q", got, value)
	}
}

func TestMockSecretManager_Exists_Missing(t *testing.T) {
	ctx := context.Background()
	m := newMock()

	ok, err := m.Exists(ctx, "snipe/snipe-url")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if ok {
		t.Error("Exists returned true for a secret that was never set")
	}
}

func TestMockSecretManager_Overwrite(t *testing.T) {
	ctx := context.Background()
	m := newMock()

	const name = "snipe/snipe-token"
	if err := m.Set(ctx, name, "first"); err != nil {
		t.Fatalf("Set(first): %v", err)
	}
	if err := m.Set(ctx, name, "second"); err != nil {
		t.Fatalf("Set(second): %v", err)
	}
	got, err := m.Get(ctx, name)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "second" {
		t.Errorf("Get = %q, want %q", got, "second")
	}
}

func TestMockSecretManager_ListByPrefix(t *testing.T) {
	ctx := context.Background()
	m := newMock()

	_ = m.Set(ctx, "github2snipe/token", "tok1")
	_ = m.Set(ctx, "github2snipe/other", "tok2")
	_ = m.Set(ctx, "snipe/snipe-url", "url1")

	names, err := m.ListByPrefix(ctx, "github2snipe/")
	if err != nil {
		t.Fatalf("ListByPrefix: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("ListByPrefix returned %d results, want 2: %v", len(names), names)
	}
}
