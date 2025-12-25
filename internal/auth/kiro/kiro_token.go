// Package kiro provides authentication and token management functionality
// for AWS CodeWhisperer (Kiro) services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Kiro API.
package kiro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
)

// KiroTokenStorage stores OAuth2 token information for AWS Kiro API authentication.
// It maintains compatibility with the existing auth system while adding Kiro-specific fields
// for managing access tokens, refresh tokens, and user account information.
type KiroTokenStorage struct {
	// Type indicates the authentication provider type, always "kiro" for this storage
	Type string `json:"type"`
	// AccessToken is the OAuth2 access token used for authenticating API requests
	AccessToken string `json:"access_token"`
	// RefreshToken is used to obtain new access tokens when the current one expires
	RefreshToken string `json:"refresh_token"`
	// IDToken is the JWT ID token containing user claims and identity information (optional)
	IDToken string `json:"id_token,omitempty"`
	// Email is the user's email address
	Email string `json:"email"`
	// ExpiresAt is the timestamp when the token expires
	ExpiresAt string `json:"expires_at"`
	// AuthMethod indicates the authentication method used ("builder_id" or "google")
	AuthMethod string `json:"auth_method"`
	// ProfileArn is the AWS CodeWhisperer profile ARN (optional)
	ProfileArn string `json:"profile_arn,omitempty"`
	// ClientID is the AWS SSO client ID (for Builder ID auth)
	ClientID string `json:"client_id,omitempty"`
	// ClientSecret is the AWS SSO client secret (for Builder ID auth)
	ClientSecret string `json:"client_secret,omitempty"`
	// Provider indicates the OAuth provider ("google" or "aws")
	Provider string `json:"provider,omitempty"`
	// LastRefresh is the timestamp of the last token refresh operation
	LastRefresh string `json:"last_refresh"`
}

// SaveTokenToFile serializes the Kiro token storage to a JSON file.
// This method creates the necessary directory structure and writes the token
// data in JSON format to the specified file path for persistent storage.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *KiroTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	ts.Type = "kiro"

	// Create directory structure if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(authFilePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create the token file
	f, err := os.Create(authFilePath)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Encode and write the token data as JSON with indentation
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(ts); err != nil {
		return fmt.Errorf("failed to write token to file: %w", err)
	}
	return nil
}
