// Package cmd provides CLI command implementations for ProxyPilot.
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	claudeauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/claude"
	codexauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/codex"
	kiroauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	qwenauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/qwen"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// RefreshResult holds the result of a token refresh operation
type RefreshResult struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Email     string `json:"email,omitempty"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// RefreshTokens refreshes tokens for matching accounts.
// If identifier is empty, refresh all accounts.
// If identifier is provided, match by email or ID.
func RefreshTokens(cfg *config.Config, identifier string, jsonOutput bool) error {
	store := sdkAuth.NewFileTokenStore()
	store.SetBaseDir(util.DefaultAuthDir())

	auths, err := store.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	// Filter accounts if identifier provided
	var toRefresh []*cliproxyauth.Auth
	identifier = strings.TrimSpace(strings.ToLower(identifier))

	for _, auth := range auths {
		if identifier == "" {
			toRefresh = append(toRefresh, auth)
			continue
		}

		// Match by ID, email, or label
		id := strings.ToLower(auth.ID)
		email := strings.ToLower(auth.Attributes["email"])
		label := strings.ToLower(auth.Label)

		if id == identifier || strings.Contains(id, identifier) ||
			email == identifier || strings.Contains(email, identifier) ||
			label == identifier || strings.Contains(label, identifier) {
			toRefresh = append(toRefresh, auth)
		}
	}

	if len(toRefresh) == 0 {
		if identifier != "" {
			return fmt.Errorf("no matching accounts found for: %s", identifier)
		}
		if jsonOutput {
			return outputJSON([]RefreshResult{})
		}
		fmt.Printf("%sNo accounts found to refresh%s\n", colorYellow, colorReset)
		return nil
	}

	results := make([]RefreshResult, 0, len(toRefresh))

	if !jsonOutput {
		fmt.Printf("\n%s%sRefreshing tokens...%s\n", colorBold, colorCyan, colorReset)
		fmt.Printf("%s─────────────────────────────%s\n\n", colorDim, colorReset)
	}

	for _, auth := range toRefresh {
		result := refreshSingleAuth(cfg, auth, store)
		results = append(results, result)

		if !jsonOutput {
			printRefreshResult(result)
		}
	}

	if jsonOutput {
		return outputJSON(results)
	}

	// Print summary
	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}

	fmt.Printf("\n%s─────────────────────────────%s\n", colorDim, colorReset)
	fmt.Printf("Refreshed: %s%d succeeded%s", colorGreen, succeeded, colorReset)
	if failed > 0 {
		fmt.Printf(", %s%d failed%s", colorRed, failed, colorReset)
	}
	fmt.Printf("\n\n")

	return nil
}

func refreshSingleAuth(cfg *config.Config, auth *cliproxyauth.Auth, store *sdkAuth.FileTokenStore) RefreshResult {
	result := RefreshResult{
		ID:       auth.ID,
		Provider: auth.Provider,
		Email:    auth.Attributes["email"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var refreshErr error
	var updated *cliproxyauth.Auth

	switch strings.ToLower(auth.Provider) {
	case "claude":
		updated, refreshErr = refreshClaude(ctx, cfg, auth)
	case "codex":
		updated, refreshErr = refreshCodex(ctx, cfg, auth)
	case "gemini", "gemini-cli":
		// Gemini tokens are refreshed automatically by the OAuth2 library
		refreshErr = fmt.Errorf("gemini tokens refresh automatically; use re-login if expired")
	case "kiro":
		updated, refreshErr = refreshKiro(ctx, cfg, auth)
	case "qwen":
		updated, refreshErr = refreshQwen(ctx, cfg, auth)
	case "antigravity":
		// Antigravity uses short-lived tokens - needs re-import
		refreshErr = fmt.Errorf("antigravity requires re-import (use --antigravity-import)")
	case "vertex":
		// Vertex uses service account - no refresh needed
		result.Success = true
		return result
	case "minimax", "zhipu":
		// API key based - no refresh needed
		result.Success = true
		return result
	default:
		refreshErr = fmt.Errorf("unsupported provider: %s", auth.Provider)
	}

	if refreshErr != nil {
		result.Error = refreshErr.Error()
		return result
	}

	if updated != nil {
		// Save updated auth
		if _, saveErr := store.Save(ctx, updated); saveErr != nil {
			result.Error = fmt.Sprintf("refresh succeeded but save failed: %v", saveErr)
			return result
		}

		// Extract new expiry
		if expiry, ok := updated.ExpirationTime(); ok {
			result.ExpiresAt = expiry.Format(time.RFC3339)
		}
	}

	result.Success = true
	return result
}

func refreshClaude(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth.Metadata == nil {
		return nil, fmt.Errorf("no metadata")
	}

	refreshToken, ok := auth.Metadata["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}

	svc := claudeauth.NewClaudeAuth(cfg)
	td, err := svc.RefreshTokens(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	updated := auth.Clone()
	updated.Metadata["access_token"] = td.AccessToken
	if td.RefreshToken != "" {
		updated.Metadata["refresh_token"] = td.RefreshToken
	}
	updated.Metadata["email"] = td.Email
	updated.Metadata["expired"] = td.Expire
	updated.Metadata["last_refresh"] = time.Now().Format(time.RFC3339)

	if expiry, ok := updated.ExpirationTime(); ok {
		updated.TokenExpiresAt = expiry
	}

	return updated, nil
}

func refreshCodex(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth.Metadata == nil {
		return nil, fmt.Errorf("no metadata")
	}

	refreshToken, ok := auth.Metadata["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}

	svc := codexauth.NewCodexAuth(cfg)
	td, err := svc.RefreshTokensWithRetry(ctx, refreshToken, 3)
	if err != nil {
		return nil, err
	}

	updated := auth.Clone()
	updated.Metadata["id_token"] = td.IDToken
	updated.Metadata["access_token"] = td.AccessToken
	if td.RefreshToken != "" {
		updated.Metadata["refresh_token"] = td.RefreshToken
	}
	if td.AccountID != "" {
		updated.Metadata["account_id"] = td.AccountID
	}
	updated.Metadata["email"] = td.Email
	updated.Metadata["expired"] = td.Expire
	updated.Metadata["last_refresh"] = time.Now().Format(time.RFC3339)

	if expiry, ok := updated.ExpirationTime(); ok {
		updated.TokenExpiresAt = expiry
	}

	return updated, nil
}

func refreshKiro(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth.Metadata == nil {
		return nil, fmt.Errorf("no metadata")
	}

	refreshToken, ok := auth.Metadata["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}

	clientID, _ := auth.Metadata["client_id"].(string)
	clientSecret, _ := auth.Metadata["client_secret"].(string)
	authMethod, _ := auth.Metadata["auth_method"].(string)
	startURL, _ := auth.Metadata["start_url"].(string)
	region, _ := auth.Metadata["region"].(string)

	var tokenData *kiroauth.KiroTokenData
	var err error

	ssoClient := kiroauth.NewSSOOIDCClient(cfg)

	switch {
	case clientID != "" && clientSecret != "" && authMethod == "idc" && region != "":
		tokenData, err = ssoClient.RefreshTokenWithRegion(ctx, clientID, clientSecret, refreshToken, region, startURL)
	case clientID != "" && clientSecret != "" && authMethod == "builder-id":
		tokenData, err = ssoClient.RefreshToken(ctx, clientID, clientSecret, refreshToken)
	default:
		oauth := kiroauth.NewKiroOAuth(cfg)
		tokenData, err = oauth.RefreshToken(ctx, refreshToken)
	}

	if err != nil {
		return nil, err
	}

	updated := auth.Clone()
	now := time.Now()
	updated.UpdatedAt = now
	updated.LastRefreshedAt = now
	updated.Metadata["access_token"] = tokenData.AccessToken
	updated.Metadata["refresh_token"] = tokenData.RefreshToken
	updated.Metadata["expires_at"] = tokenData.ExpiresAt
	updated.Metadata["last_refresh"] = now.Format(time.RFC3339)

	expiresAt, _ := time.Parse(time.RFC3339, tokenData.ExpiresAt)
	if !expiresAt.IsZero() {
		updated.NextRefreshAfter = expiresAt.Add(-5 * time.Minute)
	}

	return updated, nil
}

func refreshQwen(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth.Metadata == nil {
		return nil, fmt.Errorf("no metadata")
	}

	refreshToken, ok := auth.Metadata["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}

	svc := qwenauth.NewQwenAuth(cfg)
	td, err := svc.RefreshTokens(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	updated := auth.Clone()
	updated.Metadata["access_token"] = td.AccessToken
	if td.RefreshToken != "" {
		updated.Metadata["refresh_token"] = td.RefreshToken
	}
	updated.Metadata["expired"] = td.Expire
	updated.Metadata["last_refresh"] = time.Now().Format(time.RFC3339)

	if expiry, ok := updated.ExpirationTime(); ok {
		updated.TokenExpiresAt = expiry
	}

	return updated, nil
}

func printRefreshResult(result RefreshResult) {
	email := result.Email
	if email == "" {
		email = result.ID
	}
	if len(email) > 35 {
		email = email[:32] + "..."
	}

	if result.Success {
		expiry := ""
		if result.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, result.ExpiresAt); err == nil {
				expiry = fmt.Sprintf(" (expires: %s)", t.Format("2006-01-02 15:04"))
			}
		}
		fmt.Printf("  %s+%s %-12s %-35s %srefreshed%s%s\n",
			colorGreen, colorReset,
			result.Provider, email,
			colorGreen, colorReset, expiry)
	} else {
		errMsg := result.Error
		if len(errMsg) > 40 {
			errMsg = errMsg[:37] + "..."
		}
		fmt.Printf("  %sx%s %-12s %-35s %sfailed%s: %s\n",
			colorRed, colorReset,
			result.Provider, email,
			colorRed, colorReset, errMsg)
	}
}
