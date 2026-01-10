// Package antigravity provides token loading utilities for importing
// credentials from Antigravity IDE into the Antigravity provider.
package antigravity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// AntigravityToken represents the OAuth token structure stored by Antigravity IDE.
// Storage locations:
//   - Linux: ~/.antigravity/oauth_creds.json
//   - macOS: ~/Library/Application Support/Antigravity/oauth_creds.json
//   - Windows: %APPDATA%\Antigravity\oauth_creds.json
type AntigravityToken struct {
	// Token contains the OAuth2 token data from Antigravity IDE.
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
func (t *AntigravityToken) GetAccessToken() string {
	if t.Token != nil && t.Token.AccessToken != "" {
		return t.Token.AccessToken
	}
	return t.AccessToken
}

// GetRefreshToken returns the refresh token, checking both nested and legacy fields.
func (t *AntigravityToken) GetRefreshToken() string {
	if t.Token != nil && t.Token.RefreshToken != "" {
		return t.Token.RefreshToken
	}
	return t.RefreshToken
}

// GetExpiry returns the token expiry time.
func (t *AntigravityToken) GetExpiry() time.Time {
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
func (t *AntigravityToken) IsExpired() bool {
	expiry := t.GetExpiry()
	if expiry.IsZero() {
		return false // Can't determine, assume not expired
	}
	return time.Now().After(expiry)
}

// antigravityTokenPath returns the path to Antigravity IDE's oauth_creds.json file.
// It checks OS-specific locations where Antigravity stores its credentials.
func antigravityTokenPath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\Antigravity\oauth_creds.json
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "Antigravity", "oauth_creds.json"), nil

	case "darwin":
		// macOS: ~/Library/Application Support/Antigravity/oauth_creds.json
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			return "", fmt.Errorf("HOME environment variable not set")
		}
		return filepath.Join(homeDir, "Library", "Application Support", "Antigravity", "oauth_creds.json"), nil

	default:
		// Linux and others: ~/.antigravity/oauth_creds.json
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			return "", fmt.Errorf("HOME environment variable not set")
		}
		return filepath.Join(homeDir, ".antigravity", "oauth_creds.json"), nil
	}
}

// LoadAntigravityToken loads the OAuth token from Antigravity IDE's storage location.
// It reads from the OS-specific location where Antigravity stores credentials.
//
// Returns:
//   - *AntigravityToken: The loaded token data
//   - error: An error if the file cannot be read or parsed
func LoadAntigravityToken() (*AntigravityToken, error) {
	tokenPath, err := antigravityTokenPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine token path: %w", err)
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Antigravity IDE token not found at %s. Please login to Antigravity IDE first", tokenPath)
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token AntigravityToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	// Validate that we have at least an access token
	if token.GetAccessToken() == "" {
		return nil, fmt.Errorf("token file exists but contains no access token")
	}

	return &token, nil
}

// LoadAntigravityTokenFromPath loads the OAuth token from a specific file path.
// This is useful for testing or when the token is stored in a non-standard location.
func LoadAntigravityTokenFromPath(path string) (*AntigravityToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token AntigravityToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}

// Backwards compatibility aliases for existing code
// TODO: Remove these after updating all callers

// GeminiCLIToken is an alias for AntigravityToken for backwards compatibility.
type GeminiCLIToken = AntigravityToken

// LoadGeminiCLIToken is an alias for LoadAntigravityToken for backwards compatibility.
func LoadGeminiCLIToken() (*AntigravityToken, error) {
	return LoadAntigravityToken()
}

// LoadGeminiCLITokenFromPath is an alias for LoadAntigravityTokenFromPath for backwards compatibility.
func LoadGeminiCLITokenFromPath(path string) (*AntigravityToken, error) {
	return LoadAntigravityTokenFromPath(path)
}
