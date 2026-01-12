package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ErrEngineNotRunning indicates the proxy engine is not running.
var ErrEngineNotRunning = errors.New("engine not running - start ProxyPilot first")

// Client handles API calls to the ProxyPilot management API.
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// doRequest performs an authenticated API request.
func (c *Client) doRequest(endpoint string, result interface{}) error {
	url := c.baseURL + endpoint

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	if c.apiKey != "" {
		req.Header.Set("X-Management-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		// Check for connection refused errors
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			return ErrEngineNotRunning
		}
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "No connection could be made") {
			return ErrEngineNotRunning
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// FetchStatus fetches proxy status.
func (c *Client) FetchStatus() (ProxyStatus, error) {
	var status ProxyStatus

	// First check if proxy is running by fetching healthz
	resp, err := c.client.Get(c.baseURL + "/healthz")
	if err != nil {
		return status, nil // Proxy not running
	}
	defer resp.Body.Close()
	status.Running = resp.StatusCode == http.StatusOK

	// Get account count
	var authResp struct {
		Files []interface{} `json:"files"`
	}
	if err := c.doRequest("/v0/management/auth-files", &authResp); err == nil {
		status.Accounts = len(authResp.Files)
	}

	// Get model count
	var modelsResp struct {
		Data []interface{} `json:"data"`
	}
	if err := c.doRequest("/v1/models", &modelsResp); err == nil {
		status.Models = len(modelsResp.Data)
	}

	// Parse port from baseURL
	status.Port = 8318 // Default

	return status, nil
}

// FetchAccounts fetches account list.
func (c *Client) FetchAccounts() ([]AccountInfo, error) {
	var resp struct {
		Files []struct {
			AuthIndex      string `json:"auth_index"`
			Provider       string `json:"provider"`
			Email          string `json:"email"`
			Label          string `json:"label"`
			Status         string `json:"status"`
			Disabled       bool   `json:"disabled"`
			TokenExpiresAt string `json:"token_expires_at"`
			Usage          struct {
				DailyInputTokens  int64 `json:"daily_input_tokens"`
				DailyOutputTokens int64 `json:"daily_output_tokens"`
			} `json:"usage"`
		} `json:"files"`
	}

	if err := c.doRequest("/v0/management/auth-files", &resp); err != nil {
		return nil, err
	}

	accounts := make([]AccountInfo, len(resp.Files))
	for i, auth := range resp.Files {
		email := auth.Email
		if email == "" {
			email = auth.Label
		}
		if email == "" && len(auth.AuthIndex) >= 8 {
			email = auth.AuthIndex[:8]
		}

		status := auth.Status
		if auth.Disabled {
			status = "disabled"
		}

		expires := ""
		if auth.TokenExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, auth.TokenExpiresAt); err == nil {
				if t.After(time.Now()) {
					expires = t.Format("Jan 02 15:04")
				} else {
					expires = "Expired"
				}
			}
		}

		usage := fmt.Sprintf("%dK in / %dK out",
			auth.Usage.DailyInputTokens/1000,
			auth.Usage.DailyOutputTokens/1000)

		accounts[i] = AccountInfo{
			ID:       auth.AuthIndex,
			Provider: auth.Provider,
			Email:    email,
			Status:   status,
			Expires:  expires,
			Usage:    usage,
		}
	}

	return accounts, nil
}

// FetchRateLimits fetches rate limit summary.
func (c *Client) FetchRateLimits() (RateLimitSummary, error) {
	var resp struct {
		Total          int    `json:"total"`
		Available      int    `json:"available"`
		CoolingDown    int    `json:"cooling_down"`
		Disabled       int    `json:"disabled"`
		NextRecoveryIn string `json:"next_recovery_in"`
	}

	if err := c.doRequest("/v0/management/rate-limits/summary", &resp); err != nil {
		return RateLimitSummary{}, err
	}

	return RateLimitSummary{
		Total:        resp.Total,
		Available:    resp.Available,
		CoolingDown:  resp.CoolingDown,
		Disabled:     resp.Disabled,
		NextRecovery: resp.NextRecoveryIn,
	}, nil
}

// FetchUsage fetches usage statistics.
func (c *Client) FetchUsage() (UsageStats, error) {
	var resp struct {
		Usage struct {
			TotalRequests     int64   `json:"total_requests"`
			TotalInputTokens  int64   `json:"total_input_tokens"`
			TotalOutputTokens int64   `json:"total_output_tokens"`
			EstimatedCost     float64 `json:"estimated_cost"`
			ByModel           map[string]struct {
				Requests     int64 `json:"requests"`
				InputTokens  int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
			} `json:"by_model"`
		} `json:"usage"`
	}

	if err := c.doRequest("/v0/management/usage", &resp); err != nil {
		return UsageStats{}, err
	}

	stats := UsageStats{
		TotalRequests:     resp.Usage.TotalRequests,
		TotalInputTokens:  resp.Usage.TotalInputTokens,
		TotalOutputTokens: resp.Usage.TotalOutputTokens,
		EstimatedCost:     resp.Usage.EstimatedCost,
	}

	// Get top 5 models by requests
	for model, usage := range resp.Usage.ByModel {
		stats.TopModels = append(stats.TopModels, ModelUsage{
			Model:    model,
			Requests: usage.Requests,
			Tokens:   usage.InputTokens + usage.OutputTokens,
		})
	}

	// Sort by requests (simple bubble sort for small slice)
	for i := 0; i < len(stats.TopModels); i++ {
		for j := i + 1; j < len(stats.TopModels); j++ {
			if stats.TopModels[j].Requests > stats.TopModels[i].Requests {
				stats.TopModels[i], stats.TopModels[j] = stats.TopModels[j], stats.TopModels[i]
			}
		}
	}
	if len(stats.TopModels) > 5 {
		stats.TopModels = stats.TopModels[:5]
	}

	return stats, nil
}

// FetchLogs fetches recent log entries.
func (c *Client) FetchLogs() ([]string, error) {
	var resp struct {
		Entries []struct {
			Timestamp string `json:"timestamp"`
			Level     string `json:"level"`
			Message   string `json:"message"`
		} `json:"entries"`
	}

	if err := c.doRequest("/v0/management/logs?limit=100", &resp); err != nil {
		return nil, err
	}

	logs := make([]string, len(resp.Entries))
	for i, entry := range resp.Entries {
		logs[i] = fmt.Sprintf("[%s] %s: %s", entry.Timestamp, entry.Level, entry.Message)
	}

	return logs, nil
}

// StartAuth initiates OAuth login for a provider.
// Returns the auth URL and state token.
func (c *Client) StartAuth(provider string) (url string, state string, err error) {
	endpoint := fmt.Sprintf("/v0/management/%s-auth-url", provider)

	var resp struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}

	if err := c.doRequest(endpoint, &resp); err != nil {
		return "", "", err
	}

	return resp.URL, resp.State, nil
}

// GetAuthStatus checks the status of an OAuth login.
func (c *Client) GetAuthStatus(state string) (status string, message string, err error) {
	endpoint := fmt.Sprintf("/v0/management/get-auth-status?state=%s", state)

	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	if err := c.doRequest(endpoint, &resp); err != nil {
		// If endpoint doesn't exist or returns error, assume still pending
		return "pending", "Waiting for auth...", nil
	}

	return resp.Status, resp.Message, nil
}
