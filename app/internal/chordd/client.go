// Package chordd is the HTTP client for the Python analysis service.
package chordd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client talks to one chordd deployment.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// Health is GET /healthz.
type Health struct {
	OK       bool     `json:"ok"`
	Analyzer string   `json:"analyzer"`
	Models   []string `json:"models"`
}

// New returns a client with a short timeout, suitable for status checks.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 3 * time.Second},
	}
}

// Check calls GET /healthz.
func (c *Client) Check(ctx context.Context) (Health, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/healthz", nil)
	if err != nil {
		return Health{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Health{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Health{}, fmt.Errorf("chordd health: %s", resp.Status)
	}
	var health Health
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return Health{}, err
	}
	if !health.OK {
		return Health{}, fmt.Errorf("chordd health: not ok")
	}
	return health, nil
}
