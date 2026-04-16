package snipeit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// DefaultCategories is the canonical list of Snipe-IT license categories seeded
// during first-time setup. Defined here so commands can reference it without
// touching wizard or install logic.
var DefaultCategories = []string{
	"AI Tools",
	"Communication & Collaboration",
	"Design & Creative",
	"Developer Tools & Hosting",
	"Endpoint Management & Security",
	"Identity & Access Management",
	"Misc Software",
	"Productivity",
	"Project & Knowledge Management",
	"Training & Learning",
}

// Category is a Snipe-IT license category.
type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Client is a minimal Snipe-IT API client for category management.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a Snipe-IT categories client. baseURL should be the full
// instance URL (e.g. https://snipe.example.com); trailing slashes are trimmed.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// listResponse is the shape of GET /api/v1/categories.
type listResponse struct {
	Total int        `json:"total"`
	Rows  []Category `json:"rows"`
}

// ListCategories returns all categories in Snipe-IT (up to 500).
func (c *Client) ListCategories() ([]Category, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/categories?limit=500&offset=0", nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Snipe-IT returned HTTP %d listing categories", resp.StatusCode)
	}

	var result listResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding categories response: %w", err)
	}
	// Snipe-IT HTML-encodes category names in GET responses (e.g. "&amp;" → "&").
	for i := range result.Rows {
		result.Rows[i].Name = html.UnescapeString(result.Rows[i].Name)
	}
	return result.Rows, nil
}

// createRequest is the POST /api/v1/categories body.
type createRequest struct {
	Name         string `json:"name"`
	CategoryType string `json:"category_type"`
}

// createResponse is the Snipe-IT POST response envelope.
type createResponse struct {
	Status   string   `json:"status"`
	Messages any      `json:"messages"`
	Payload  Category `json:"payload"`
}

// CreateCategory creates a new license category and returns its ID.
func (c *Client) CreateCategory(name string) (int, error) {
	body := createRequest{Name: name, CategoryType: "license"}
	data, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/categories", bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("Snipe-IT returned HTTP %d creating category %q", resp.StatusCode, name)
	}

	var result createResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding create category response: %w", err)
	}
	if result.Status != "success" {
		return 0, fmt.Errorf("Snipe-IT: create category %q failed: status=%q", name, result.Status)
	}
	return result.Payload.ID, nil
}

// EnsureCategory checks if a category with the given name already exists; if
// not, it creates one with type "license". Returns the category ID in either
// case. Returns 0 and no error when name is empty (logs a warning instead).
func (c *Client) EnsureCategory(name string) (int, error) {
	if name == "" {
		slog.Warn("EnsureCategory called with empty category name — skipping")
		return 0, nil
	}

	cats, err := c.ListCategories()
	if err != nil {
		return 0, fmt.Errorf("listing categories: %w", err)
	}

	for _, cat := range cats {
		if strings.EqualFold(cat.Name, name) {
			return cat.ID, nil
		}
	}

	return c.CreateCategory(name)
}

// SeedDefaults ensures every entry in DefaultCategories exists in Snipe-IT.
// Individual failures are logged as warnings and do not abort the operation.
func (c *Client) SeedDefaults() error {
	for _, name := range DefaultCategories {
		if _, err := c.EnsureCategory(name); err != nil {
			slog.Warn("failed to seed category", "name", name, "error", err)
		}
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
}
