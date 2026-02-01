package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	kiroauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// kiroCredentials extracts access token and profile ARN from auth.
func kiroCredentials(auth *cliproxyauth.Auth) (accessToken, profileArn string) {
	if auth == nil {
		return "", ""
	}

	// Try Metadata first (wrapper format)
	if auth.Metadata != nil {
		if token, ok := auth.Metadata["access_token"].(string); ok {
			accessToken = token
		}
		if arn, ok := auth.Metadata["profile_arn"].(string); ok {
			profileArn = arn
		}
	}

	// Try Attributes
	if accessToken == "" && auth.Attributes != nil {
		accessToken = auth.Attributes["access_token"]
		profileArn = auth.Attributes["profile_arn"]
	}

	// Try direct fields from flat JSON format (new AWS Builder ID format)
	if accessToken == "" && auth.Metadata != nil {
		if token, ok := auth.Metadata["accessToken"].(string); ok {
			accessToken = token
		}
		if arn, ok := auth.Metadata["profileArn"].(string); ok {
			profileArn = arn
		}
	}

	return accessToken, profileArn
}

// isIDCAuth checks if the auth uses IDC (Identity Center) authentication method.
func isIDCAuth(auth *cliproxyauth.Auth) bool {
	if auth == nil || auth.Metadata == nil {
		return false
	}
	authMethod, _ := auth.Metadata["auth_method"].(string)
	return authMethod == "idc"
}

// getTokenKey returns a unique key for rate limiting based on auth credentials.
// Uses auth ID if available, otherwise falls back to a hash of the access token.
func getTokenKey(auth *cliproxyauth.Auth) string {
	if auth != nil && auth.ID != "" {
		return auth.ID
	}
	accessToken, _ := kiroCredentials(auth)
	if len(accessToken) > 16 {
		return accessToken[:16]
	}
	return accessToken
}

// Refresh refreshes the Kiro OAuth token.
// Supports both AWS Builder ID (SSO OIDC) and Google OAuth (social login).
// Uses mutex to prevent race conditions when multiple concurrent requests try to refresh.
func (e *KiroExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	// Serialize token refresh operations to prevent race conditions
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	var authID string
	if auth != nil {
		authID = auth.ID
	} else {
		authID = "<nil>"
	}
	log.Debugf("kiro executor: refresh called for auth %s", authID)
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}

	// Double-check: After acquiring lock, verify token still needs refresh
	// Another goroutine may have already refreshed while we were waiting
	// NOTE: This check has a design limitation - it reads from the auth object passed in,
	// not from persistent storage. If another goroutine returns a new Auth object (via Clone),
	// this check won't see those updates. The mutex still prevents truly concurrent refreshes,
	// but queued goroutines may still attempt redundant refreshes. This is acceptable as
	// the refresh operation is idempotent and the extra API calls are infrequent.
	if auth.Metadata != nil {
		if lastRefresh, ok := auth.Metadata["last_refresh"].(string); ok {
			if refreshTime, err := time.Parse(time.RFC3339, lastRefresh); err == nil {
				// If token was refreshed within the last 30 seconds, skip refresh
				if time.Since(refreshTime) < 30*time.Second {
					log.Debugf("kiro executor: token was recently refreshed by another goroutine, skipping")
					return auth, nil
				}
			}
		}
		// Also check if expires_at is now in the future with sufficient buffer
		if expiresAt, ok := auth.Metadata["expires_at"].(string); ok {
			if expTime, err := time.Parse(time.RFC3339, expiresAt); err == nil {
				// If token expires more than 20 minutes from now, it's still valid
				if time.Until(expTime) > 20*time.Minute {
					log.Debugf("kiro executor: token is still valid (expires in %v), skipping refresh", time.Until(expTime))
					// CRITICAL FIX: Set NextRefreshAfter to prevent frequent refresh checks
					// Without this, shouldRefresh() will return true again in 30 seconds
					updated := auth.Clone()
					// Set next refresh to 20 minutes before expiry, or at least 30 seconds from now
					nextRefresh := expTime.Add(-20 * time.Minute)
					minNextRefresh := time.Now().Add(30 * time.Second)
					if nextRefresh.Before(minNextRefresh) {
						nextRefresh = minNextRefresh
					}
					updated.NextRefreshAfter = nextRefresh
					log.Debugf("kiro executor: setting NextRefreshAfter to %v (in %v)", nextRefresh.Format(time.RFC3339), time.Until(nextRefresh))
					return updated, nil
				}
			}
		}
	}

	var refreshToken string
	var clientID, clientSecret string
	var authMethod string
	var region, startURL string

	if auth.Metadata != nil {
		if rt, ok := auth.Metadata["refresh_token"].(string); ok {
			refreshToken = rt
		}
		if cid, ok := auth.Metadata["client_id"].(string); ok {
			clientID = cid
		}
		if cs, ok := auth.Metadata["client_secret"].(string); ok {
			clientSecret = cs
		}
		if am, ok := auth.Metadata["auth_method"].(string); ok {
			authMethod = am
		}
		if r, ok := auth.Metadata["region"].(string); ok {
			region = r
		}
		if su, ok := auth.Metadata["start_url"].(string); ok {
			startURL = su
		}
	}

	if refreshToken == "" {
		return nil, fmt.Errorf("kiro executor: refresh token not found")
	}

	var tokenData *kiroauth.KiroTokenData
	var err error

	ssoClient := kiroauth.NewSSOOIDCClient(e.cfg)

	// Use SSO OIDC refresh for AWS Builder ID or IDC, otherwise use Kiro's OAuth refresh endpoint
	switch {
	case clientID != "" && clientSecret != "" && authMethod == "idc" && region != "":
		// IDC refresh with region-specific endpoint
		log.Debugf("kiro executor: using SSO OIDC refresh for IDC (region=%s)", region)
		tokenData, err = ssoClient.RefreshTokenWithRegion(ctx, clientID, clientSecret, refreshToken, region, startURL)
	case clientID != "" && clientSecret != "" && authMethod == "builder-id":
		// Builder ID refresh with default endpoint
		log.Debugf("kiro executor: using SSO OIDC refresh for AWS Builder ID")
		tokenData, err = ssoClient.RefreshToken(ctx, clientID, clientSecret, refreshToken)
	default:
		// Fallback to Kiro's OAuth refresh endpoint (for social auth: Google/GitHub)
		log.Debugf("kiro executor: using Kiro OAuth refresh endpoint")
		oauth := kiroauth.NewKiroOAuth(e.cfg)
		tokenData, err = oauth.RefreshToken(ctx, refreshToken)
	}

	if err != nil {
		return nil, fmt.Errorf("kiro executor: token refresh failed: %w", err)
	}

	updated := auth.Clone()
	now := time.Now()
	updated.UpdatedAt = now
	updated.LastRefreshedAt = now

	if updated.Metadata == nil {
		updated.Metadata = make(map[string]any)
	}
	updated.Metadata["access_token"] = tokenData.AccessToken
	updated.Metadata["refresh_token"] = tokenData.RefreshToken
	updated.Metadata["expires_at"] = tokenData.ExpiresAt
	updated.Metadata["last_refresh"] = now.Format(time.RFC3339)
	if tokenData.ProfileArn != "" {
		updated.Metadata["profile_arn"] = tokenData.ProfileArn
	}
	if tokenData.AuthMethod != "" {
		updated.Metadata["auth_method"] = tokenData.AuthMethod
	}
	if tokenData.Provider != "" {
		updated.Metadata["provider"] = tokenData.Provider
	}
	// Preserve client credentials for future refreshes (AWS Builder ID)
	if tokenData.ClientID != "" {
		updated.Metadata["client_id"] = tokenData.ClientID
	}
	if tokenData.ClientSecret != "" {
		updated.Metadata["client_secret"] = tokenData.ClientSecret
	}
	// Preserve region and start_url for IDC token refresh
	if tokenData.Region != "" {
		updated.Metadata["region"] = tokenData.Region
	}
	if tokenData.StartURL != "" {
		updated.Metadata["start_url"] = tokenData.StartURL
	}

	if updated.Attributes == nil {
		updated.Attributes = make(map[string]string)
	}
	updated.Attributes["access_token"] = tokenData.AccessToken
	if tokenData.ProfileArn != "" {
		updated.Attributes["profile_arn"] = tokenData.ProfileArn
	}

	// NextRefreshAfter is aligned with RefreshLead (20min)
	if expiresAt, parseErr := time.Parse(time.RFC3339, tokenData.ExpiresAt); parseErr == nil {
		updated.NextRefreshAfter = expiresAt.Add(-20 * time.Minute)
	}

	log.Infof("kiro executor: token refreshed successfully, expires at %s", tokenData.ExpiresAt)
	return updated, nil
}

// persistRefreshedAuth persists a refreshed auth record to disk.
// This ensures token refreshes from inline retry are saved to the auth file.
func (e *KiroExecutor) persistRefreshedAuth(auth *cliproxyauth.Auth) error {
	if auth == nil || auth.Metadata == nil {
		return fmt.Errorf("kiro executor: cannot persist nil auth or metadata")
	}

	// Determine the file path from auth attributes or filename
	var authPath string
	if auth.Attributes != nil {
		if p := strings.TrimSpace(auth.Attributes["path"]); p != "" {
			authPath = p
		}
	}
	if authPath == "" {
		fileName := strings.TrimSpace(auth.FileName)
		if fileName == "" {
			return fmt.Errorf("kiro executor: auth has no file path or filename")
		}
		if filepath.IsAbs(fileName) {
			authPath = fileName
		} else if e.cfg != nil && e.cfg.AuthDir != "" {
			authPath = filepath.Join(e.cfg.AuthDir, fileName)
		} else {
			return fmt.Errorf("kiro executor: cannot determine auth file path")
		}
	}

	// Marshal metadata to JSON
	raw, err := json.Marshal(auth.Metadata)
	if err != nil {
		return fmt.Errorf("kiro executor: marshal metadata failed: %w", err)
	}

	// Write to temp file first, then rename (atomic write)
	tmp := authPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("kiro executor: write temp auth file failed: %w", err)
	}
	if err := os.Rename(tmp, authPath); err != nil {
		return fmt.Errorf("kiro executor: rename auth file failed: %w", err)
	}

	log.Debugf("kiro executor: persisted refreshed auth to %s", authPath)
	return nil
}

// reloadAuthFromFile reloads auth data from file (Fallback mechanism).
// When token in memory has expired, try to read the latest token from file.
// This solves the timing gap where background refresher has updated the file
// but the in-memory Auth object has not yet synchronized.
func (e *KiroExecutor) reloadAuthFromFile(auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: cannot reload nil auth")
	}

	// Determine file path
	var authPath string
	if auth.Attributes != nil {
		if p := strings.TrimSpace(auth.Attributes["path"]); p != "" {
			authPath = p
		}
	}
	if authPath == "" {
		fileName := strings.TrimSpace(auth.FileName)
		if fileName == "" {
			return nil, fmt.Errorf("kiro executor: auth has no file path or filename for reload")
		}
		if filepath.IsAbs(fileName) {
			authPath = fileName
		} else if e.cfg != nil && e.cfg.AuthDir != "" {
			authPath = filepath.Join(e.cfg.AuthDir, fileName)
		} else {
			return nil, fmt.Errorf("kiro executor: cannot determine auth file path for reload")
		}
	}

	// Read file
	raw, err := os.ReadFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("kiro executor: failed to read auth file %s: %w", authPath, err)
	}

	// Parse JSON
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("kiro executor: failed to parse auth file %s: %w", authPath, err)
	}

	// Check if token in file is newer than in memory
	fileExpiresAt, _ := metadata["expires_at"].(string)
	fileAccessToken, _ := metadata["access_token"].(string)
	memExpiresAt, _ := auth.Metadata["expires_at"].(string)
	memAccessToken, _ := auth.Metadata["access_token"].(string)

	// File must have a valid access_token
	if fileAccessToken == "" {
		return nil, fmt.Errorf("kiro executor: auth file has no access_token field")
	}

	// If has expires_at, check if expired
	if fileExpiresAt != "" {
		fileExpTime, parseErr := time.Parse(time.RFC3339, fileExpiresAt)
		if parseErr == nil {
			// If token in file is also expired, don't use it
			if time.Now().After(fileExpTime) {
				log.Debugf("kiro executor: file token also expired at %s, not using", fileExpiresAt)
				return nil, fmt.Errorf("kiro executor: file token also expired")
			}
		}
	}

	// Determine if token in file is newer than in memory
	// Condition 1: access_token is different (means refreshed)
	// Condition 2: expires_at is newer (means refreshed)
	isNewer := false

	// First check if access_token has changed
	if fileAccessToken != memAccessToken {
		isNewer = true
		log.Debugf("kiro executor: file access_token differs from memory, using file token")
	}

	// If access_token is same, check expires_at
	if !isNewer && fileExpiresAt != "" && memExpiresAt != "" {
		fileExpTime, fileParseErr := time.Parse(time.RFC3339, fileExpiresAt)
		memExpTime, memParseErr := time.Parse(time.RFC3339, memExpiresAt)
		if fileParseErr == nil && memParseErr == nil && fileExpTime.After(memExpTime) {
			isNewer = true
			log.Debugf("kiro executor: file expires_at (%s) is newer than memory (%s)", fileExpiresAt, memExpiresAt)
		}
	}

	// If file has no expires_at but access_token is same, can't determine if newer
	if !isNewer && fileExpiresAt == "" && fileAccessToken == memAccessToken {
		return nil, fmt.Errorf("kiro executor: cannot determine if file token is newer (no expires_at, same access_token)")
	}

	if !isNewer {
		log.Debugf("kiro executor: file token not newer than memory token")
		return nil, fmt.Errorf("kiro executor: file token not newer")
	}

	// Create updated auth object
	updated := auth.Clone()
	updated.Metadata = metadata
	updated.UpdatedAt = time.Now()

	// Sync update Attributes
	if updated.Attributes == nil {
		updated.Attributes = make(map[string]string)
	}
	if accessToken, ok := metadata["access_token"].(string); ok {
		updated.Attributes["access_token"] = accessToken
	}
	if profileArn, ok := metadata["profile_arn"].(string); ok {
		updated.Attributes["profile_arn"] = profileArn
	}

	log.Infof("kiro executor: reloaded auth from file %s, new expires_at: %s", authPath, fileExpiresAt)
	return updated, nil
}

// isTokenExpired checks if a JWT access token has expired.
// Returns true if the token is expired or cannot be parsed.
func (e *KiroExecutor) isTokenExpired(accessToken string) bool {
	if accessToken == "" {
		return true
	}

	// JWT tokens have 3 parts separated by dots
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		// Not a JWT token, treat as expired to trigger refresh
		return true
	}

	// Decode the payload (second part)
	// JWT uses base64url encoding without padding (RawURLEncoding)
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		// Try with padding added as fallback
		switch len(payload) % 4 {
		case 2:
			payload += "=="
		case 3:
			payload += "="
		}
		decoded, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			log.Debugf("kiro: failed to decode JWT payload: %v", err)
			// Parse failure - treat as expired per function contract
			return true
		}
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		log.Debugf("kiro: failed to parse JWT claims: %v", err)
		return false
	}

	if claims.Exp == 0 {
		// No expiration claim, assume not expired
		return false
	}

	expTime := time.Unix(claims.Exp, 0)
	now := time.Now()

	// Consider token expired if it expires within 1 minute (buffer for clock skew)
	isExpired := now.After(expTime) || expTime.Sub(now) < time.Minute
	if isExpired {
		log.Debugf("kiro: token expired at %s (now: %s)", expTime.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	return isExpired
}
