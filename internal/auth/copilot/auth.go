package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

const (
	// GitHub Device Flow endpoints
	DeviceCodeEndpoint = "https://github.com/login/device/code"
	TokenEndpoint      = "https://github.com/login/oauth/access_token"

	// Client ID for GitHub Copilot (using VS Code's ID which is commonly used for this)
	ClientID = "01ab8ac9400c4e429b23"
	// Scope required for Copilot
	Scope = "read:user copilot"

	GrantType = "urn:ietf:params:oauth:grant-type:device_code"
)

// CopilotAuth manages authentication for GitHub Copilot.
type CopilotAuth struct {
	httpClient *http.Client
}

// NewCopilotAuth creates a new CopilotAuth instance.
func NewCopilotAuth(cfg *config.Config) *CopilotAuth {
	return &CopilotAuth{
		httpClient: util.SetProxy(&cfg.SDKConfig, &http.Client{}),
	}
}

// InitiateDeviceFlow starts the OAuth 2.0 device authorization flow.
func (ca *CopilotAuth) InitiateDeviceFlow(ctx context.Context) (*DeviceFlow, error) {
	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("scope", Scope)

	req, err := http.NewRequestWithContext(ctx, "POST", DeviceCodeEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := ca.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization failed: %d %s. Response: %s", resp.StatusCode, resp.Status, string(body))
	}

	var result DeviceFlow
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse device flow response: %w", err)
	}

	if result.DeviceCode == "" {
		return nil, fmt.Errorf("device authorization failed: device_code not found in response")
	}

	return &result, nil
}

// PollForToken polls the token endpoint with the device code to obtain an access token.
func (ca *CopilotAuth) PollForToken(deviceCode string) (*CopilotTokenData, error) {
	// GitHub's default interval is usually 5 seconds
	pollInterval := 5 * time.Second
	maxAttempts := 60 // 5 minutes max

	for attempt := 0; attempt < maxAttempts; attempt++ {
		data := url.Values{}
		data.Set("client_id", ClientID)
		data.Set("device_code", deviceCode)
		data.Set("grant_type", GrantType)

		req, err := http.NewRequest("POST", TokenEndpoint, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := ca.httpClient.Do(req)
		if err != nil {
			fmt.Printf("Polling attempt %d/%d failed: %v\n", attempt+1, maxAttempts, err)
			time.Sleep(pollInterval)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			time.Sleep(pollInterval)
			continue
		}

		var tokenResp CopilotTokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse token response: %w", err)
		}

		if tokenResp.Error != "" {
			switch tokenResp.Error {
			case "authorization_pending":
				// Continue polling
			case "slow_down":
				pollInterval += 2 * time.Second
			case "expired_token":
				return nil, fmt.Errorf("device code expired")
			case "access_denied":
				return nil, fmt.Errorf("access denied by user")
			default:
				return nil, fmt.Errorf("polling error: %s", tokenResp.Error)
			}
			time.Sleep(pollInterval)
			continue
		}

		if tokenResp.AccessToken != "" {
			return &CopilotTokenData{
				AccessToken:  tokenResp.AccessToken,
				TokenType:    tokenResp.TokenType,
				RefreshToken: tokenResp.RefreshToken,
				Scope:        tokenResp.Scope,
				Expire:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339),
			}, nil
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("authentication timeout")
}

// CreateTokenStorage creates a CopilotTokenStorage object from CopilotTokenData.
func (ca *CopilotAuth) CreateTokenStorage(tokenData *CopilotTokenData) *CopilotTokenStorage {
	return &CopilotTokenStorage{
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
		LastRefresh:  time.Now().Format(time.RFC3339),
		Expire:       tokenData.Expire,
	}
}
