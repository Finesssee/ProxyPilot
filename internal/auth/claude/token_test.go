package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper function to parse token expiry time
func parseExpireTime(expire string) (time.Time, error) {
	return time.Parse(time.RFC3339, expire)
}

// Helper function to check if token is expired
func isTokenExpired(expire string) bool {
	expireTime, err := parseExpireTime(expire)
	if err != nil {
		return true // Treat parse errors as expired
	}
	return time.Now().After(expireTime)
}

// Helper function to check if token needs refresh within lead time
func tokenNeedsRefresh(expire string, leadTime time.Duration) bool {
	expireTime, err := parseExpireTime(expire)
	if err != nil {
		return true // Treat parse errors as needing refresh
	}
	return time.Now().Add(leadTime).After(expireTime)
}

func TestToken_Parse_ValidToken(t *testing.T) {
	tests := []struct {
		name      string
		tokenJSON string
		wantEmail string
		wantType  string
		wantErr   bool
	}{
		{
			name: "valid token with all fields",
			tokenJSON: `{
				"id_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test",
				"access_token": "sk-ant-test-access-token",
				"refresh_token": "rt-test-refresh-token",
				"last_refresh": "2025-01-15T10:00:00Z",
				"email": "user@example.com",
				"type": "claude",
				"expired": "2025-01-15T11:00:00Z"
			}`,
			wantEmail: "user@example.com",
			wantType:  "claude",
			wantErr:   false,
		},
		{
			name: "valid token with minimal fields",
			tokenJSON: `{
				"access_token": "sk-ant-minimal",
				"refresh_token": "rt-minimal",
				"email": "minimal@test.com",
				"type": "claude",
				"expired": "2025-12-31T23:59:59Z"
			}`,
			wantEmail: "minimal@test.com",
			wantType:  "claude",
			wantErr:   false,
		},
		{
			name: "token with empty strings",
			tokenJSON: `{
				"access_token": "",
				"refresh_token": "",
				"email": "",
				"type": "claude",
				"expired": ""
			}`,
			wantEmail: "",
			wantType:  "claude",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var token ClaudeTokenStorage
			err := json.Unmarshal([]byte(tt.tokenJSON), &token)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if token.Email != tt.wantEmail {
					t.Errorf("Email = %v, want %v", token.Email, tt.wantEmail)
				}
				if token.Type != tt.wantType {
					t.Errorf("Type = %v, want %v", token.Type, tt.wantType)
				}
			}
		})
	}
}

func TestToken_Parse_ExpiredToken(t *testing.T) {
	tests := []struct {
		name      string
		tokenJSON string
		wantErr   bool
	}{
		{
			name: "token with past expiration date",
			tokenJSON: `{
				"access_token": "expired-token",
				"refresh_token": "rt-expired",
				"email": "expired@test.com",
				"type": "claude",
				"expired": "2020-01-01T00:00:00Z"
			}`,
			wantErr: false,
		},
		{
			name: "token with invalid expiration format",
			tokenJSON: `{
				"access_token": "invalid-expire",
				"refresh_token": "rt-invalid",
				"email": "invalid@test.com",
				"type": "claude",
				"expired": "not-a-valid-date"
			}`,
			wantErr: false, // JSON parsing succeeds, date validation is separate
		},
		{
			name: "token with Unix timestamp expiration (wrong format)",
			tokenJSON: `{
				"access_token": "unix-token",
				"refresh_token": "rt-unix",
				"email": "unix@test.com",
				"type": "claude",
				"expired": "1704067200"
			}`,
			wantErr: false, // JSON parsing succeeds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var token ClaudeTokenStorage
			err := json.Unmarshal([]byte(tt.tokenJSON), &token)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify the token can be parsed even if expired
			if !tt.wantErr && token.AccessToken == "" {
				t.Error("AccessToken should not be empty after successful parse")
			}
		})
	}
}

func TestToken_IsExpired_FreshToken(t *testing.T) {
	tests := []struct {
		name        string
		expireTime  time.Time
		wantExpired bool
	}{
		{
			name:        "token expiring in 1 hour",
			expireTime:  time.Now().Add(1 * time.Hour),
			wantExpired: false,
		},
		{
			name:        "token expiring in 24 hours",
			expireTime:  time.Now().Add(24 * time.Hour),
			wantExpired: false,
		},
		{
			name:        "token expiring in 1 minute",
			expireTime:  time.Now().Add(1 * time.Minute),
			wantExpired: false,
		},
		{
			name:        "token expiring in 1 second",
			expireTime:  time.Now().Add(1 * time.Second),
			wantExpired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expireStr := tt.expireTime.Format(time.RFC3339)
			expired := isTokenExpired(expireStr)

			if expired != tt.wantExpired {
				t.Errorf("isTokenExpired() = %v, want %v", expired, tt.wantExpired)
			}
		})
	}
}

func TestToken_IsExpired_ExpiredToken(t *testing.T) {
	tests := []struct {
		name        string
		expireTime  time.Time
		wantExpired bool
	}{
		{
			name:        "token expired 1 hour ago",
			expireTime:  time.Now().Add(-1 * time.Hour),
			wantExpired: true,
		},
		{
			name:        "token expired 24 hours ago",
			expireTime:  time.Now().Add(-24 * time.Hour),
			wantExpired: true,
		},
		{
			name:        "token expired 1 second ago",
			expireTime:  time.Now().Add(-1 * time.Second),
			wantExpired: true,
		},
		{
			name:        "token expired in 2020",
			expireTime:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			wantExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expireStr := tt.expireTime.Format(time.RFC3339)
			expired := isTokenExpired(expireStr)

			if expired != tt.wantExpired {
				t.Errorf("isTokenExpired() = %v, want %v", expired, tt.wantExpired)
			}
		})
	}
}

func TestToken_NeedsRefresh_WithinLeadTime(t *testing.T) {
	tests := []struct {
		name        string
		expireTime  time.Time
		leadTime    time.Duration
		wantRefresh bool
	}{
		{
			name:        "token expiring in 5 minutes with 10 minute lead time",
			expireTime:  time.Now().Add(5 * time.Minute),
			leadTime:    10 * time.Minute,
			wantRefresh: true,
		},
		{
			name:        "token expiring in 30 seconds with 1 minute lead time",
			expireTime:  time.Now().Add(30 * time.Second),
			leadTime:    1 * time.Minute,
			wantRefresh: true,
		},
		{
			name:        "token expiring in 4 minutes with 5 minute lead time",
			expireTime:  time.Now().Add(4 * time.Minute),
			leadTime:    5 * time.Minute,
			wantRefresh: true,
		},
		{
			name:        "already expired token with any lead time",
			expireTime:  time.Now().Add(-1 * time.Minute),
			leadTime:    5 * time.Minute,
			wantRefresh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expireStr := tt.expireTime.Format(time.RFC3339)
			needsRefresh := tokenNeedsRefresh(expireStr, tt.leadTime)

			if needsRefresh != tt.wantRefresh {
				t.Errorf("tokenNeedsRefresh() = %v, want %v", needsRefresh, tt.wantRefresh)
			}
		})
	}
}

func TestToken_NeedsRefresh_NotYet(t *testing.T) {
	tests := []struct {
		name        string
		expireTime  time.Time
		leadTime    time.Duration
		wantRefresh bool
	}{
		{
			name:        "token expiring in 1 hour with 5 minute lead time",
			expireTime:  time.Now().Add(1 * time.Hour),
			leadTime:    5 * time.Minute,
			wantRefresh: false,
		},
		{
			name:        "token expiring in 10 minutes with 5 minute lead time",
			expireTime:  time.Now().Add(10 * time.Minute),
			leadTime:    5 * time.Minute,
			wantRefresh: false,
		},
		{
			name:        "token expiring in 24 hours with 1 hour lead time",
			expireTime:  time.Now().Add(24 * time.Hour),
			leadTime:    1 * time.Hour,
			wantRefresh: false,
		},
		{
			name:        "token expiring in 6 minutes with 5 minute lead time",
			expireTime:  time.Now().Add(6 * time.Minute),
			leadTime:    5 * time.Minute,
			wantRefresh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expireStr := tt.expireTime.Format(time.RFC3339)
			needsRefresh := tokenNeedsRefresh(expireStr, tt.leadTime)

			if needsRefresh != tt.wantRefresh {
				t.Errorf("tokenNeedsRefresh() = %v, want %v", needsRefresh, tt.wantRefresh)
			}
		})
	}
}

func TestToken_SaveTokenToFile(t *testing.T) {
	tests := []struct {
		name    string
		token   ClaudeTokenStorage
		wantErr bool
	}{
		{
			name: "save valid token",
			token: ClaudeTokenStorage{
				IDToken:      "test-id-token",
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				LastRefresh:  time.Now().Format(time.RFC3339),
				Email:        "test@example.com",
				Type:         "claude",
				Expire:       time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			},
			wantErr: false,
		},
		{
			name: "save token with empty fields",
			token: ClaudeTokenStorage{
				AccessToken:  "minimal-token",
				RefreshToken: "minimal-refresh",
				Type:         "claude",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test-token.json")

			err := tt.token.SaveTokenToFile(filePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("SaveTokenToFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file was created
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Error("Token file was not created")
					return
				}

				// Verify file contents can be parsed back
				data, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("Failed to read token file: %v", err)
					return
				}

				var savedToken ClaudeTokenStorage
				if err := json.Unmarshal(data, &savedToken); err != nil {
					t.Errorf("Failed to parse saved token: %v", err)
					return
				}

				// Verify key fields match
				if savedToken.AccessToken != tt.token.AccessToken {
					t.Errorf("AccessToken = %v, want %v", savedToken.AccessToken, tt.token.AccessToken)
				}
				if savedToken.Type != "claude" {
					t.Errorf("Type = %v, want claude", savedToken.Type)
				}
			}
		})
	}
}

func TestToken_SaveTokenToFile_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "structure", "token.json")

	token := ClaudeTokenStorage{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		Type:         "claude",
	}

	err := token.SaveTokenToFile(nestedPath)
	if err != nil {
		t.Errorf("SaveTokenToFile() should create nested directories, got error: %v", err)
		return
	}

	// Verify file exists
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Token file was not created in nested directory")
	}
}
