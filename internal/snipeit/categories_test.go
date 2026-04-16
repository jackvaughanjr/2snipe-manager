package snipeit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultCategories_Count(t *testing.T) {
	if len(DefaultCategories) != 10 {
		t.Errorf("expected 10 default categories, got %d", len(DefaultCategories))
	}
	for _, c := range DefaultCategories {
		if c == "" {
			t.Error("DefaultCategories contains an empty string")
		}
	}
}

func TestEnsureCategory_EmptyName(t *testing.T) {
	// No server needed — empty name is short-circuited before any HTTP call.
	client := NewClient("https://snipe.example.com", "token")
	id, err := client.EnsureCategory("")
	if err != nil {
		t.Errorf("expected no error for empty name, got: %v", err)
	}
	if id != 0 {
		t.Errorf("expected id=0 for empty name, got: %d", id)
	}
}

func TestCreateCategory_EnvelopeUnwrap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/categories" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(createResponse{
				Status:  "success",
				Payload: Category{ID: 42, Name: "AI Tools"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	id, err := client.CreateCategory("AI Tools")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected id=42, got %d", id)
	}
}

func TestSeedDefaults_Idempotent(t *testing.T) {
	postCount := 0

	// Server returns all DefaultCategories as already existing on every GET,
	// so SeedDefaults should make zero POST requests.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			cats := make([]Category, len(DefaultCategories))
			for i, n := range DefaultCategories {
				cats[i] = Category{ID: i + 1, Name: n}
			}
			_ = json.NewEncoder(w).Encode(listResponse{Total: len(cats), Rows: cats})
		case http.MethodPost:
			postCount++
			_ = json.NewEncoder(w).Encode(createResponse{
				Status:  "success",
				Payload: Category{ID: postCount},
			})
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	if err := client.SeedDefaults(); err != nil {
		t.Fatalf("unexpected error from SeedDefaults: %v", err)
	}
	if postCount != 0 {
		t.Errorf("expected 0 POST calls when all categories already exist, got %d", postCount)
	}
}
