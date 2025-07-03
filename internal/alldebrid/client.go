// Package alldebrid provides client functionality for the AllDebrid API
package alldebrid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultBaseURL is the base URL for the AllDebrid API
	DefaultBaseURL = "https://api.alldebrid.com/v4"
)

// Client represents an AllDebrid API client
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// UnrestrictResult represents the result of an unrestrict operation
type UnrestrictResult struct {
	UnrestrictedURL string `json:"link"`
	Filename        string `json:"filename"`
	FileSize        int64  `json:"filesize"`
}

// APIResponse represents a generic API response from AllDebrid
type APIResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  *APIError       `json:"error,omitempty"`
}

// APIError represents an error response from the API
type APIError struct {
	Message string      `json:"message"`
	Code    interface{} `json:"code,omitempty"`
}

// Error implements the error interface for APIError
func (e *APIError) Error() string {
	if e.Code != nil {
		return fmt.Sprintf("%s (code: %v)", e.Message, e.Code)
	}
	return e.Message
}

// AllDebridClient defines the interface for AllDebrid operations
type AllDebridClient interface {
	UnrestrictLink(ctx context.Context, link string) (*UnrestrictResult, error)
	CheckAPIKey(ctx context.Context) error
}

// New creates a new AllDebrid client
func New(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UnrestrictLink unrestricts a link using the AllDebrid API
func (c *Client) UnrestrictLink(ctx context.Context, link string) (*UnrestrictResult, error) {
	params := url.Values{}
	params.Set("agent", "debrid-downloader")
	params.Set("apikey", c.apiKey)
	params.Set("link", link)

	endpoint := fmt.Sprintf("%s/link/unlock?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Status != "success" {
		if apiResp.Error != nil {
			return nil, apiResp.Error
		}
		return nil, fmt.Errorf("API returned status: %s", apiResp.Status)
	}

	var result UnrestrictResult
	if err := json.Unmarshal(apiResp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse unrestrict result: %w", err)
	}

	return &result, nil
}

// CheckAPIKey validates the API key by making a test request
func (c *Client) CheckAPIKey(ctx context.Context) error {
	params := url.Values{}
	params.Set("agent", "debrid-downloader")
	params.Set("apikey", c.apiKey)

	endpoint := fmt.Sprintf("%s/user?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Status != "success" {
		if apiResp.Error != nil {
			return apiResp.Error
		}
		return fmt.Errorf("API returned status: %s", apiResp.Status)
	}

	return nil
}
