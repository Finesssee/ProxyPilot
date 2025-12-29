package auth

import (
	"context"
	"fmt"
	"time"

	amazonqauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/amazonq"
	kiroauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// AmazonQAuthenticator implements authentication via Amazon Q CLI tokens.
// Amazon Q CLI uses a separate quota pool from Kiro IDE, so these tokens
// are useful for getting additional usage capacity.
type AmazonQAuthenticator struct{}

// NewAmazonQAuthenticator constructs an Amazon Q CLI authenticator.
func NewAmazonQAuthenticator() *AmazonQAuthenticator {
	return &AmazonQAuthenticator{}
}

// Provider returns the provider key for the authenticator.
// Uses "kiro" as the provider since Amazon Q uses the same API.
func (a *AmazonQAuthenticator) Provider() string {
	return "kiro"
}

// RefreshLead indicates how soon before expiry a refresh should be attempted.
func (a *AmazonQAuthenticator) RefreshLead() *time.Duration {
	d := 5 * time.Minute
	return &d
}

// Login performs login by reading tokens from Amazon Q CLI.
// This requires Amazon Q CLI to be installed and authenticated.
func (a *AmazonQAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
	return a.ImportFromAmazonQCLI(ctx, cfg)
}

// ImportFromAmazonQCLI imports token from Amazon Q CLI's SQLite database.
// Amazon Q CLI stores tokens at ~/.local/share/amazon-q/data.sqlite3 (or WSL path on Windows).
func (a *AmazonQAuthenticator) ImportFromAmazonQCLI(ctx context.Context, cfg *config.Config) (*coreauth.Auth, error) {
	reader, err := amazonqauth.NewTokenReader()
	if err != nil {
		return nil, fmt.Errorf("failed to create token reader: %w", err)
	}

	kiroToken, err := reader.ReadKiroToken()
	if err != nil {
		return nil, fmt.Errorf("failed to read Amazon Q CLI token: %w", err)
	}

	// Parse expires_at
	expiresAt, err := time.Parse(time.RFC3339, kiroToken.ExpiresAt)
	if err != nil {
		expiresAt = time.Now().Add(1 * time.Hour)
	}

	// Extract email from JWT if available
	email := kiroauth.ExtractEmailFromJWT(kiroToken.AccessToken)

	// Create identifier for file naming
	idPart := "amazonq-cli"
	if email != "" {
		idPart = fmt.Sprintf("amazonq-cli-%s", kiroauth.SanitizeEmailForFilename(email))
	}

	now := time.Now()
	fileName := fmt.Sprintf("kiro-%s.json", idPart)

	record := &coreauth.Auth{
		ID:        fileName,
		Provider:  "kiro", // Uses kiro provider since same API
		FileName:  fileName,
		Label:     "kiro-amazonq-cli",
		Status:    coreauth.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]any{
			"type":             "kiro",
			"access_token":     kiroToken.AccessToken,
			"refresh_token":    kiroToken.RefreshToken,
			"expires_at":       kiroToken.ExpiresAt,
			"auth_method":      kiroToken.AuthMethod,
			"provider":         kiroToken.Provider,
			"client_id":        kiroToken.ClientID,
			"client_secret":    kiroToken.ClientSecret,
			"email":            email,
			"start_url":        kiroToken.StartURL,
			"region":           kiroToken.Region,
			"preferred_endpoint": "amazonq", // Force Amazon Q endpoint (CLI origin)
		},
		Attributes: map[string]string{
			"source":             "amazonq-cli-import",
			"email":              email,
			"preferred_endpoint": "amazonq", // Force Amazon Q endpoint (CLI origin)
		},
		NextRefreshAfter: expiresAt.Add(-5 * time.Minute),
	}

	if email != "" {
		fmt.Printf("\n✓ Imported Amazon Q CLI token (Account: %s)\n", email)
	} else {
		fmt.Println("\n✓ Imported Amazon Q CLI token")
	}
	fmt.Println("  Note: Amazon Q CLI has separate usage quota from Kiro IDE")

	return record, nil
}

// Refresh refreshes an expired Amazon Q CLI token.
// Uses the same refresh mechanism as Kiro since it's the same API.
func (a *AmazonQAuthenticator) Refresh(ctx context.Context, cfg *config.Config, auth *coreauth.Auth) (*coreauth.Auth, error) {
	if auth == nil || auth.Metadata == nil {
		return nil, fmt.Errorf("invalid auth record")
	}

	refreshToken, ok := auth.Metadata["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("refresh token not found")
	}

	clientID, _ := auth.Metadata["client_id"].(string)
	clientSecret, _ := auth.Metadata["client_secret"].(string)
	region, _ := auth.Metadata["region"].(string)
	startURL, _ := auth.Metadata["start_url"].(string)

	var tokenData *kiroauth.KiroTokenData
	var err error

	ssoClient := kiroauth.NewSSOOIDCClient(cfg)

	// Amazon Q CLI uses Builder ID auth
	if clientID != "" && clientSecret != "" {
		if region != "" {
			tokenData, err = ssoClient.RefreshTokenWithRegion(ctx, clientID, clientSecret, refreshToken, region, startURL)
		} else {
			tokenData, err = ssoClient.RefreshToken(ctx, clientID, clientSecret, refreshToken)
		}
	} else {
		// Fallback - try to re-import from CLI
		reader, readErr := amazonqauth.NewTokenReader()
		if readErr != nil {
			return nil, fmt.Errorf("cannot refresh without client credentials and cannot re-import: %w", readErr)
		}
		tokenData, err = reader.ReadKiroToken()
	}

	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	// Parse expires_at
	expiresAt, err := time.Parse(time.RFC3339, tokenData.ExpiresAt)
	if err != nil {
		expiresAt = time.Now().Add(1 * time.Hour)
	}

	// Clone auth to avoid mutating the input parameter
	updated := auth.Clone()
	now := time.Now()
	updated.UpdatedAt = now
	updated.LastRefreshedAt = now
	updated.Metadata["access_token"] = tokenData.AccessToken
	updated.Metadata["refresh_token"] = tokenData.RefreshToken
	updated.Metadata["expires_at"] = tokenData.ExpiresAt
	updated.Metadata["last_refresh"] = now.Format(time.RFC3339)
	updated.NextRefreshAfter = expiresAt.Add(-5 * time.Minute)

	return updated, nil
}

// IsAmazonQCLIAvailable checks if Amazon Q CLI tokens are available.
func IsAmazonQCLIAvailable() bool {
	return amazonqauth.IsAvailable()
}
