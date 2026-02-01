package codex

import (
	"encoding/json"
	"testing"
	"time"
)

// TestCodexAuthBundle_Serialize tests JSON serialization round-trip.
func TestCodexAuthBundle_Serialize(t *testing.T) {
	original := CodexAuthBundle{
		APIKey: "sk-proj-test-api-key",
		TokenData: CodexTokenData{
			IDToken:      "test-id-token",
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			AccountID:    "acct_test123",
			Email:        "test@example.com",
			Expire:       "2025-01-15T11:00:00Z",
		},
		LastRefresh: "2025-01-15T10:00:00Z",
	}

	// Serialize to JSON
	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Failed to marshal bundle: %v", err)
	}

	// Deserialize back
	var restored CodexAuthBundle
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal bundle: %v", err)
	}

	// Verify all fields match
	if restored.APIKey != original.APIKey {
		t.Errorf("APIKey = %v, want %v", restored.APIKey, original.APIKey)
	}
	if restored.TokenData.IDToken != original.TokenData.IDToken {
		t.Errorf("TokenData.IDToken = %v, want %v", restored.TokenData.IDToken, original.TokenData.IDToken)
	}
	if restored.TokenData.AccessToken != original.TokenData.AccessToken {
		t.Errorf("TokenData.AccessToken = %v, want %v", restored.TokenData.AccessToken, original.TokenData.AccessToken)
	}
	if restored.TokenData.AccountID != original.TokenData.AccountID {
		t.Errorf("TokenData.AccountID = %v, want %v", restored.TokenData.AccountID, original.TokenData.AccountID)
	}
	if restored.TokenData.Email != original.TokenData.Email {
		t.Errorf("TokenData.Email = %v, want %v", restored.TokenData.Email, original.TokenData.Email)
	}
	if restored.LastRefresh != original.LastRefresh {
		t.Errorf("LastRefresh = %v, want %v", restored.LastRefresh, original.LastRefresh)
	}
}

// TestCodexToken_IsValid tests token validation logic.
func TestCodexToken_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		token     CodexTokenStorage
		wantValid bool
	}{
		{
			name: "valid token with all fields",
			token: CodexTokenStorage{
				IDToken:      "valid-id-token",
				AccessToken:  "valid-access-token",
				RefreshToken: "valid-refresh-token",
				AccountID:    "acct_123",
				Email:        "user@example.com",
				Expire:       time.Now().Add(time.Hour).Format(time.RFC3339),
			},
			wantValid: true,
		},
		{
			name: "token with empty access token",
			token: CodexTokenStorage{
				AccessToken:  "",
				RefreshToken: "valid-refresh-token",
			},
			wantValid: false,
		},
		{
			name: "token with only access token",
			token: CodexTokenStorage{
				AccessToken: "access-only",
			},
			wantValid: true,
		},
		{
			name: "token with whitespace access token",
			token: CodexTokenStorage{
				AccessToken: "   ",
			},
			wantValid: true, // Whitespace is technically non-empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.token.AccessToken != ""
			if isValid != tt.wantValid {
				t.Errorf("Token validity = %v, want %v", isValid, tt.wantValid)
			}
		})
	}
}

// TestCodexTokenStorage_Serialize tests JSON serialization round-trip.
func TestCodexTokenStorage_Serialize(t *testing.T) {
	original := CodexTokenStorage{
		IDToken:      "test-id-token",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		AccountID:    "acct_test",
		LastRefresh:  "2025-01-15T10:00:00Z",
		Email:        "test@example.com",
		Type:         "codex",
		Expire:       "2025-01-15T11:00:00Z",
	}

	// Serialize to JSON
	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Failed to marshal token: %v", err)
	}

	// Deserialize back
	var restored CodexTokenStorage
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal token: %v", err)
	}

	// Verify all fields match
	if restored.IDToken != original.IDToken {
		t.Errorf("IDToken = %v, want %v", restored.IDToken, original.IDToken)
	}
	if restored.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %v, want %v", restored.AccessToken, original.AccessToken)
	}
	if restored.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %v, want %v", restored.RefreshToken, original.RefreshToken)
	}
	if restored.AccountID != original.AccountID {
		t.Errorf("AccountID = %v, want %v", restored.AccountID, original.AccountID)
	}
	if restored.Email != original.Email {
		t.Errorf("Email = %v, want %v", restored.Email, original.Email)
	}
	if restored.Type != original.Type {
		t.Errorf("Type = %v, want %v", restored.Type, original.Type)
	}
	if restored.Expire != original.Expire {
		t.Errorf("Expire = %v, want %v", restored.Expire, original.Expire)
	}
}

// TestCodexTokenData_Serialize tests JSON serialization of token data.
func TestCodexTokenData_Serialize(t *testing.T) {
	original := CodexTokenData{
		IDToken:      "jwt-id-token",
		AccessToken:  "jwt-access-token",
		RefreshToken: "jwt-refresh-token",
		AccountID:    "acct_serialize",
		Email:        "serialize@example.com",
		Expire:       "2025-12-31T23:59:59Z",
	}

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Failed to marshal token data: %v", err)
	}

	var restored CodexTokenData
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal token data: %v", err)
	}

	if restored.IDToken != original.IDToken {
		t.Errorf("IDToken mismatch")
	}
	if restored.AccessToken != original.AccessToken {
		t.Errorf("AccessToken mismatch")
	}
	if restored.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken mismatch")
	}
	if restored.AccountID != original.AccountID {
		t.Errorf("AccountID mismatch")
	}
	if restored.Email != original.Email {
		t.Errorf("Email mismatch")
	}
	if restored.Expire != original.Expire {
		t.Errorf("Expire mismatch")
	}
}

// TestCodexPKCECodes_Serialize tests PKCE codes serialization.
func TestCodexPKCECodes_Serialize(t *testing.T) {
	original := PKCECodes{
		CodeVerifier:  "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
		CodeChallenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
	}

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Failed to marshal PKCE codes: %v", err)
	}

	var restored PKCECodes
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal PKCE codes: %v", err)
	}

	if restored.CodeVerifier != original.CodeVerifier {
		t.Errorf("CodeVerifier = %v, want %v", restored.CodeVerifier, original.CodeVerifier)
	}
	if restored.CodeChallenge != original.CodeChallenge {
		t.Errorf("CodeChallenge = %v, want %v", restored.CodeChallenge, original.CodeChallenge)
	}
}

// TestCodexToken_ExpireEdgeCases tests edge cases for token expiration.
func TestCodexToken_ExpireEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		expireTime string
		wantExpire bool
	}{
		{
			name:       "very old expiration date",
			expireTime: "2000-01-01T00:00:00Z",
			wantExpire: true,
		},
		{
			name:       "far future expiration date",
			expireTime: "2099-12-31T23:59:59Z",
			wantExpire: false,
		},
		{
			name:       "epoch time string",
			expireTime: "1970-01-01T00:00:00Z",
			wantExpire: true,
		},
		{
			name:       "timezone with offset",
			expireTime: time.Now().Add(time.Hour).Format(time.RFC3339),
			wantExpire: false,
		},
		{
			name:       "invalid date format - Unix timestamp",
			expireTime: "1704067200",
			wantExpire: true, // Parse error should be treated as expired
		},
		{
			name:       "invalid date format - random string",
			expireTime: "not-a-date",
			wantExpire: true, // Parse error should be treated as expired
		},
		{
			name:       "empty string",
			expireTime: "",
			wantExpire: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isExpired := isTokenExpired(tt.expireTime)
			if isExpired != tt.wantExpire {
				t.Errorf("isTokenExpired(%q) = %v, want %v", tt.expireTime, isExpired, tt.wantExpire)
			}
		})
	}
}

// TestCodexToken_RefreshThresholdEdgeCases tests edge cases for refresh threshold.
func TestCodexToken_RefreshThresholdEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		expireTime    string
		threshold     time.Duration
		wantNeedRefresh bool
	}{
		{
			name:            "zero threshold - not expired",
			expireTime:      time.Now().Add(time.Hour).Format(time.RFC3339),
			threshold:       0,
			wantNeedRefresh: false,
		},
		{
			name:            "zero threshold - expired",
			expireTime:      time.Now().Add(-time.Hour).Format(time.RFC3339),
			threshold:       0,
			wantNeedRefresh: true,
		},
		{
			name:            "very large threshold",
			expireTime:      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			threshold:       48 * time.Hour,
			wantNeedRefresh: true,
		},
		{
			name:            "exactly at threshold boundary",
			expireTime:      time.Now().Add(5 * time.Minute).Format(time.RFC3339),
			threshold:       5 * time.Minute,
			wantNeedRefresh: true,
		},
		{
			name:            "just before threshold boundary",
			expireTime:      time.Now().Add(5*time.Minute + time.Second).Format(time.RFC3339),
			threshold:       5 * time.Minute,
			wantNeedRefresh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needsRefresh := tokenNeedsRefresh(tt.expireTime, tt.threshold)
			if needsRefresh != tt.wantNeedRefresh {
				t.Errorf("tokenNeedsRefresh(%q, %v) = %v, want %v",
					tt.expireTime, tt.threshold, needsRefresh, tt.wantNeedRefresh)
			}
		})
	}
}
