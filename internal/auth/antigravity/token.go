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
// Storage locations (checked in order):
//  1. OS-specific Antigravity path:
//     - Linux: ~/.antigravity/oauth_creds.json
//     - macOS: ~/Library/Application Support/Antigravity/oauth_creds.json
//     - Windows: %APPDATA%\Antigravity\oauth_creds.json
//  2. Shared Gemini CLI path (fallback):
//     - Unix: ~/.gemini/oauth_creds.json
//     - Windows: %USERPROFILE%\.gemini\oauth_creds.json
type AntigravityToken struct {
	// Token contains the OAuth2 token data from Antigravity IDE.
	Token *OAuthToken `json:"token,omitempty"`

	// Legacy/flat fields for backwards compatibility with older token formats.
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	ExpiryDate   int64  `json:"expiry_date,omitempty"` // Unix timestamp in milliseconds
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

	// Try expiry_date (Unix timestamp in milliseconds) - used by Gemini CLI / Antigravity
	if t.ExpiryDate > 0 {
		return time.UnixMilli(t.ExpiryDate)
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

// antigravityTokenPaths returns possible paths to Antigravity token files.
// Returns paths in priority order: OS-specific Antigravity path first, then Gemini CLI fallback.
func antigravityTokenPaths() ([]string, error) {
	var paths []string

	switch runtime.GOOS {
	case "windows":
		// Primary: %APPDATA%\Antigravity\oauth_creds.json
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "Antigravity", "oauth_creds.json"))
		}
		// Fallback: %USERPROFILE%\.gemini\oauth_creds.json
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, ".gemini", "oauth_creds.json"))
		} else if home := os.Getenv("HOME"); home != "" {
			paths = append(paths, filepath.Join(home, ".gemini", "oauth_creds.json"))
		}

	case "darwin":
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			return nil, fmt.Errorf("HOME environment variable not set")
		}
		// Primary: ~/Library/Application Support/Antigravity/oauth_creds.json
		paths = append(paths, filepath.Join(homeDir, "Library", "Application Support", "Antigravity", "oauth_creds.json"))
		// Fallback: ~/.gemini/oauth_creds.json
		paths = append(paths, filepath.Join(homeDir, ".gemini", "oauth_creds.json"))

	default:
		// Linux and others
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			return nil, fmt.Errorf("HOME environment variable not set")
		}
		// Primary: ~/.antigravity/oauth_creds.json
		paths = append(paths, filepath.Join(homeDir, ".antigravity", "oauth_creds.json"))
		// Fallback: ~/.gemini/oauth_creds.json
		paths = append(paths, filepath.Join(homeDir, ".gemini", "oauth_creds.json"))
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("cannot determine token paths")
	}

	return paths, nil
}

// LoadAntigravityToken loads the OAuth token from Antigravity IDE's storage location.
// It tries multiple paths in order: OS-specific Antigravity path first, then Gemini CLI fallback.
//
// Returns:
//   - *AntigravityToken: The loaded token data
//   - error: An error if no token file can be read or parsed
func LoadAntigravityToken() (*AntigravityToken, error) {
	paths, err := antigravityTokenPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to determine token paths: %w", err)
	}

	var lastErr error
	for _, tokenPath := range paths {
		data, err := os.ReadFile(tokenPath)
		if err != nil {
			if os.IsNotExist(err) {
				lastErr = err
				continue
			}
			lastErr = fmt.Errorf("failed to read token file %s: %w", tokenPath, err)
			continue
		}

		var token AntigravityToken
		if err := json.Unmarshal(data, &token); err != nil {
			lastErr = fmt.Errorf("failed to parse token file %s: %w", tokenPath, err)
			continue
		}

		// Validate that we have at least an access token
		if token.GetAccessToken() == "" {
			lastErr = fmt.Errorf("token file %s exists but contains no access token", tokenPath)
			continue
		}

		return &token, nil
	}

	// No token found in any location
	if lastErr != nil {
		return nil, fmt.Errorf("Antigravity token not found. Tried: %v. Last error: %w", paths, lastErr)
	}
	return nil, fmt.Errorf("Antigravity token not found. Please login to Antigravity IDE or Gemini CLI first")
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
