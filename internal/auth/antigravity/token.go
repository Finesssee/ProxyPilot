// Package antigravity provides token loading utilities for importing
// credentials from Gemini CLI into the Antigravity provider.
package antigravity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// GeminiCLIToken represents the OAuth token structure stored by Gemini CLI.
// The token is stored in ~/.gemini/oauth_creds.json on Unix systems
// or %USERPROFILE%\.gemini\oauth_creds.json on Windows.
type GeminiCLIToken struct {
	// Token contains the OAuth2 token data from Gemini CLI.
	Token *OAuthToken `json:"token,omitempty"`

	// Legacy fields for backwards compatibility with older token formats.
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	TokenType    string `json:"token_type,omitempty"`

	// Email is the email address associated with the token.
	Email string `json:"email,omitempty"`

	// ProjectID is the Google Cloud project ID.
	ProjectID string `json:"project_id,omitempty"`
}

// OAuthToken represents the nested OAuth2 token structure.
type OAuthToken struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Expiry       string `json:"expiry,omitempty"`

	// ClientID and ClientSecret for token refresh.
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// GetAccessToken returns the access token, checking both nested and legacy fields.
func (t *GeminiCLIToken) GetAccessToken() string {
	if t.Token != nil && t.Token.AccessToken != "" {
		return t.Token.AccessToken
	}
	return t.AccessToken
}

// GetRefreshToken returns the refresh token, checking both nested and legacy fields.
func (t *GeminiCLIToken) GetRefreshToken() string {
	if t.Token != nil && t.Token.RefreshToken != "" {
		return t.Token.RefreshToken
	}
	return t.RefreshToken
}

// GetExpiry returns the token expiry time.
func (t *GeminiCLIToken) GetExpiry() time.Time {
	// Try nested token first
	if t.Token != nil {
		if t.Token.Expiry != "" {
			if parsed, err := time.Parse(time.RFC3339, t.Token.Expiry); err == nil {
				return parsed
			}
		}
		if t.Token.ExpiresAt != "" {
			if parsed, err := time.Parse(time.RFC3339, t.Token.ExpiresAt); err == nil {
				return parsed
			}
		}
		if t.Token.ExpiresIn > 0 {
			// Assume token was just issued
			return time.Now().Add(time.Duration(t.Token.ExpiresIn) * time.Second)
		}
	}

	// Try legacy fields
	if t.ExpiresAt != "" {
		if parsed, err := time.Parse(time.RFC3339, t.ExpiresAt); err == nil {
			return parsed
		}
	}
	if t.ExpiresIn > 0 {
		return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	}

	return time.Time{}
}

// IsExpired returns true if the token has expired.
func (t *GeminiCLIToken) IsExpired() bool {
	expiry := t.GetExpiry()
	if expiry.IsZero() {
		return false // Can't determine, assume not expired
	}
	return time.Now().After(expiry)
}

// geminiCLITokenPath returns the path to Gemini CLI's oauth_creds.json file.
func geminiCLITokenPath() (string, error) {
	var homeDir string

	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("USERPROFILE")
		if homeDir == "" {
			homeDir = os.Getenv("HOME")
		}
	} else {
		homeDir = os.Getenv("HOME")
	}

	if homeDir == "" {
		return "", fmt.Errorf("cannot determine home directory")
	}

	return filepath.Join(homeDir, ".gemini", "oauth_creds.json"), nil
}

// LoadGeminiCLIToken loads the OAuth token from Gemini CLI's storage location.
// It reads from ~/.gemini/oauth_creds.json (or %USERPROFILE%\.gemini\oauth_creds.json on Windows).
//
// Returns:
//   - *GeminiCLIToken: The loaded token data
//   - error: An error if the file cannot be read or parsed
func LoadGeminiCLIToken() (*GeminiCLIToken, error) {
	tokenPath, err := geminiCLITokenPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine token path: %w", err)
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Gemini CLI token not found at %s. Please run 'gemini login' first", tokenPath)
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token GeminiCLIToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	// Validate that we have at least an access token
	if token.GetAccessToken() == "" {
		return nil, fmt.Errorf("token file exists but contains no access token")
	}

	return &token, nil
}

// LoadGeminiCLITokenFromPath loads the OAuth token from a specific file path.
// This is useful for testing or when the token is stored in a non-standard location.
func LoadGeminiCLITokenFromPath(path string) (*GeminiCLIToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token GeminiCLIToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}
