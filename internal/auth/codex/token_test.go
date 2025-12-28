package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexTokenStorage_Parse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		wantErr     bool
		checkFields func(t *testing.T, ts *CodexTokenStorage)
	}{
		{
			name: "parse complete token storage",
			jsonData: `{
				"id_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.sig",
				"access_token": "access_token_value",
				"refresh_token": "refresh_token_value",
				"account_id": "acct_abc123",
				"last_refresh": "2024-01-15T10:30:00Z",
				"email": "user@example.com",
				"type": "codex",
				"expired": "2024-01-15T11:30:00Z"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, ts *CodexTokenStorage) {
				if ts.IDToken != "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.sig" {
					t.Errorf("IDToken mismatch")
				}
				if ts.AccessToken != "access_token_value" {
					t.Errorf("AccessToken = %v, want access_token_value", ts.AccessToken)
				}
				if ts.RefreshToken != "refresh_token_value" {
					t.Errorf("RefreshToken = %v, want refresh_token_value", ts.RefreshToken)
				}
				if ts.AccountID != "acct_abc123" {
					t.Errorf("AccountID = %v, want acct_abc123", ts.AccountID)
				}
				if ts.Email != "user@example.com" {
					t.Errorf("Email = %v, want user@example.com", ts.Email)
				}
				if ts.Type != "codex" {
					t.Errorf("Type = %v, want codex", ts.Type)
				}
			},
		},
		{
			name: "parse minimal token storage",
			jsonData: `{
				"access_token": "minimal_access",
				"refresh_token": "minimal_refresh",
				"type": "codex"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, ts *CodexTokenStorage) {
				if ts.AccessToken != "minimal_access" {
					t.Errorf("AccessToken = %v, want minimal_access", ts.AccessToken)
				}
				if ts.IDToken != "" {
					t.Errorf("IDToken = %v, want empty", ts.IDToken)
				}
				if ts.Email != "" {
					t.Errorf("Email = %v, want empty", ts.Email)
				}
			},
		},
		{
			name: "parse token with special characters in values",
			jsonData: `{
				"access_token": "token-with_special.chars/and+more",
				"refresh_token": "refresh/token+value=",
				"email": "user+test@sub.example.com",
				"type": "codex"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, ts *CodexTokenStorage) {
				if ts.AccessToken != "token-with_special.chars/and+more" {
					t.Errorf("AccessToken mismatch with special chars")
				}
				if ts.Email != "user+test@sub.example.com" {
					t.Errorf("Email mismatch with special chars")
				}
			},
		},
		{
			name:     "parse empty JSON object",
			jsonData: `{}`,
			wantErr:  false,
			checkFields: func(t *testing.T, ts *CodexTokenStorage) {
				if ts.Type != "" {
					t.Errorf("Type = %v, want empty for empty JSON", ts.Type)
				}
			},
		},
		{
			name:     "invalid JSON",
			jsonData: `{"invalid": json}`,
			wantErr:  true,
			checkFields: func(t *testing.T, ts *CodexTokenStorage) {
				// Not called on error
			},
		},
		{
			name:     "malformed JSON - missing closing brace",
			jsonData: `{"access_token": "test"`,
			wantErr:  true,
			checkFields: func(t *testing.T, ts *CodexTokenStorage) {
				// Not called on error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts CodexTokenStorage
			err := json.Unmarshal([]byte(tt.jsonData), &ts)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				tt.checkFields(t, &ts)
			}
		})
	}
}

func TestCodexTokenStorage_IsExpired(t *testing.T) {
	tests := []struct {
		name       string
		expireTime string
		wantResult bool
	}{
		{
			name:       "token expired 1 hour ago",
			expireTime: time.Now().Add(-time.Hour).Format(time.RFC3339),
			wantResult: true,
		},
		{
			name:       "token expired 1 minute ago",
			expireTime: time.Now().Add(-time.Minute).Format(time.RFC3339),
			wantResult: true,
		},
		{
			name:       "token expires in 1 hour",
			expireTime: time.Now().Add(time.Hour).Format(time.RFC3339),
			wantResult: false,
		},
		{
			name:       "token expires in 1 minute",
			expireTime: time.Now().Add(time.Minute).Format(time.RFC3339),
			wantResult: false,
		},
		{
			name:       "token expires now",
			expireTime: time.Now().Format(time.RFC3339),
			wantResult: true, // Tokens that expire "now" are considered expired
		},
		{
			name:       "empty expire time",
			expireTime: "",
			wantResult: true, // Treat empty as expired
		},
		{
			name:       "invalid expire time format",
			expireTime: "invalid-time",
			wantResult: true, // Treat invalid as expired for safety
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &CodexTokenStorage{
				Expire: tt.expireTime,
			}

			// Test the expiry logic
			isExpired := isTokenExpired(ts.Expire)

			if isExpired != tt.wantResult {
				t.Errorf("isTokenExpired() = %v, want %v (expire: %s)",
					isExpired, tt.wantResult, tt.expireTime)
			}
		})
	}
}

func TestCodexTokenStorage_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name         string
		expireTime   string
		refreshBefore time.Duration // How long before expiry to trigger refresh
		wantResult   bool
	}{
		{
			name:          "token expires in 30 minutes, refresh at 5 min before",
			expireTime:    time.Now().Add(30 * time.Minute).Format(time.RFC3339),
			refreshBefore: 5 * time.Minute,
			wantResult:    false,
		},
		{
			name:          "token expires in 3 minutes, refresh at 5 min before",
			expireTime:    time.Now().Add(3 * time.Minute).Format(time.RFC3339),
			refreshBefore: 5 * time.Minute,
			wantResult:    true,
		},
		{
			name:          "token expires in 1 hour, refresh at 10 min before",
			expireTime:    time.Now().Add(time.Hour).Format(time.RFC3339),
			refreshBefore: 10 * time.Minute,
			wantResult:    false,
		},
		{
			name:          "token expires in 5 minutes, refresh at 10 min before",
			expireTime:    time.Now().Add(5 * time.Minute).Format(time.RFC3339),
			refreshBefore: 10 * time.Minute,
			wantResult:    true,
		},
		{
			name:          "token already expired",
			expireTime:    time.Now().Add(-time.Hour).Format(time.RFC3339),
			refreshBefore: 5 * time.Minute,
			wantResult:    true,
		},
		{
			name:          "empty expire time",
			expireTime:    "",
			refreshBefore: 5 * time.Minute,
			wantResult:    true,
		},
		{
			name:          "expires exactly at refresh threshold",
			expireTime:    time.Now().Add(5 * time.Minute).Format(time.RFC3339),
			refreshBefore: 5 * time.Minute,
			wantResult:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &CodexTokenStorage{
				Expire: tt.expireTime,
			}

			needsRefresh := tokenNeedsRefresh(ts.Expire, tt.refreshBefore)

			if needsRefresh != tt.wantResult {
				t.Errorf("tokenNeedsRefresh() = %v, want %v (expire: %s, threshold: %v)",
					needsRefresh, tt.wantResult, tt.expireTime, tt.refreshBefore)
			}
		})
	}
}

func TestCodexTokenStorage_SaveTokenToFile(t *testing.T) {
	tests := []struct {
		name    string
		storage CodexTokenStorage
		wantErr bool
	}{
		{
			name: "save complete token storage",
			storage: CodexTokenStorage{
				IDToken:      "test_id_token",
				AccessToken:  "test_access_token",
				RefreshToken: "test_refresh_token",
				AccountID:    "test_account_id",
				LastRefresh:  time.Now().Format(time.RFC3339),
				Email:        "test@example.com",
				Type:         "codex",
				Expire:       time.Now().Add(time.Hour).Format(time.RFC3339),
			},
			wantErr: false,
		},
		{
			name: "save minimal token storage",
			storage: CodexTokenStorage{
				AccessToken:  "minimal_token",
				RefreshToken: "minimal_refresh",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for the test
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "tokens", "codex_token.json")

			err := tt.storage.SaveTokenToFile(filePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("SaveTokenToFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the file was created
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("SaveTokenToFile() file was not created at %s", filePath)
					return
				}

				// Read and verify content
				data, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("Failed to read saved file: %v", err)
					return
				}

				var loaded CodexTokenStorage
				if err := json.Unmarshal(data, &loaded); err != nil {
					t.Errorf("Failed to parse saved file: %v", err)
					return
				}

				// Verify type is always set to "codex"
				if loaded.Type != "codex" {
					t.Errorf("Type = %v, want codex", loaded.Type)
				}

				// Verify key fields match
				if loaded.AccessToken != tt.storage.AccessToken {
					t.Errorf("AccessToken = %v, want %v", loaded.AccessToken, tt.storage.AccessToken)
				}
				if loaded.RefreshToken != tt.storage.RefreshToken {
					t.Errorf("RefreshToken = %v, want %v", loaded.RefreshToken, tt.storage.RefreshToken)
				}
			}
		})
	}
}

func TestCodexTokenData_Parse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		wantErr     bool
		checkFields func(t *testing.T, td *CodexTokenData)
	}{
		{
			name: "parse complete token data",
			jsonData: `{
				"id_token": "jwt_id_token_here",
				"access_token": "access_token_here",
				"refresh_token": "refresh_token_here",
				"account_id": "acct_123",
				"email": "user@example.com",
				"expired": "2024-01-15T12:00:00Z"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, td *CodexTokenData) {
				if td.IDToken != "jwt_id_token_here" {
					t.Errorf("IDToken mismatch")
				}
				if td.AccessToken != "access_token_here" {
					t.Errorf("AccessToken mismatch")
				}
				if td.RefreshToken != "refresh_token_here" {
					t.Errorf("RefreshToken mismatch")
				}
				if td.AccountID != "acct_123" {
					t.Errorf("AccountID mismatch")
				}
				if td.Email != "user@example.com" {
					t.Errorf("Email mismatch")
				}
			},
		},
		{
			name: "parse minimal token data",
			jsonData: `{
				"access_token": "only_access"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, td *CodexTokenData) {
				if td.AccessToken != "only_access" {
					t.Errorf("AccessToken = %v, want only_access", td.AccessToken)
				}
				if td.RefreshToken != "" {
					t.Errorf("RefreshToken should be empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var td CodexTokenData
			err := json.Unmarshal([]byte(tt.jsonData), &td)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				tt.checkFields(t, &td)
			}
		})
	}
}

func TestCodexAuthBundle_Parse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		wantErr     bool
		checkFields func(t *testing.T, bundle *CodexAuthBundle)
	}{
		{
			name: "parse complete auth bundle",
			jsonData: `{
				"api_key": "sk-proj-api-key",
				"token_data": {
					"id_token": "id_token_value",
					"access_token": "access_token_value",
					"refresh_token": "refresh_token_value",
					"account_id": "acct_bundle",
					"email": "bundle@example.com",
					"expired": "2024-01-15T12:00:00Z"
				},
				"last_refresh": "2024-01-15T10:00:00Z"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, bundle *CodexAuthBundle) {
				if bundle.APIKey != "sk-proj-api-key" {
					t.Errorf("APIKey mismatch")
				}
				if bundle.TokenData.AccessToken != "access_token_value" {
					t.Errorf("TokenData.AccessToken mismatch")
				}
				if bundle.TokenData.Email != "bundle@example.com" {
					t.Errorf("TokenData.Email mismatch")
				}
				if bundle.LastRefresh != "2024-01-15T10:00:00Z" {
					t.Errorf("LastRefresh mismatch")
				}
			},
		},
		{
			name: "parse bundle without api key",
			jsonData: `{
				"token_data": {
					"access_token": "token_only"
				},
				"last_refresh": "2024-01-15T10:00:00Z"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, bundle *CodexAuthBundle) {
				if bundle.APIKey != "" {
					t.Errorf("APIKey should be empty")
				}
				if bundle.TokenData.AccessToken != "token_only" {
					t.Errorf("TokenData.AccessToken mismatch")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bundle CodexAuthBundle
			err := json.Unmarshal([]byte(tt.jsonData), &bundle)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				tt.checkFields(t, &bundle)
			}
		})
	}
}

func TestPKCECodes_Parse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		wantErr     bool
		checkFields func(t *testing.T, codes *PKCECodes)
	}{
		{
			name: "parse valid PKCE codes",
			jsonData: `{
				"code_verifier": "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
				"code_challenge": "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, codes *PKCECodes) {
				if codes.CodeVerifier != "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" {
					t.Errorf("CodeVerifier mismatch")
				}
				if codes.CodeChallenge != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
					t.Errorf("CodeChallenge mismatch")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var codes PKCECodes
			err := json.Unmarshal([]byte(tt.jsonData), &codes)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				tt.checkFields(t, &codes)
			}
		})
	}
}

// Helper functions for token expiry testing

// isTokenExpired checks if a token has expired based on its expire timestamp.
func isTokenExpired(expire string) bool {
	if expire == "" {
		return true
	}

	expireTime, err := time.Parse(time.RFC3339, expire)
	if err != nil {
		return true // Treat parse errors as expired for safety
	}

	return !expireTime.After(time.Now())
}

// tokenNeedsRefresh checks if a token should be refreshed based on its expiry
// and a threshold duration before expiry.
func tokenNeedsRefresh(expire string, threshold time.Duration) bool {
	if expire == "" {
		return true
	}

	expireTime, err := time.Parse(time.RFC3339, expire)
	if err != nil {
		return true
	}

	refreshTime := expireTime.Add(-threshold)
	return !refreshTime.After(time.Now())
}
