package antigravity

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetAccessToken(t *testing.T) {
	tests := []struct {
		name     string
		token    AntigravityToken
		expected string
	}{
		{
			name: "nested token",
			token: AntigravityToken{
				Token: &OAuthToken{AccessToken: "nested-token"},
			},
			expected: "nested-token",
		},
		{
			name: "legacy token",
			token: AntigravityToken{
				AccessToken: "legacy-token",
			},
			expected: "legacy-token",
		},
		{
			name: "nested takes precedence",
			token: AntigravityToken{
				Token:       &OAuthToken{AccessToken: "nested-token"},
				AccessToken: "legacy-token",
			},
			expected: "nested-token",
		},
		{
			name:     "empty token",
			token:    AntigravityToken{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.GetAccessToken()
			if result != tt.expected {
				t.Errorf("GetAccessToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetRefreshToken(t *testing.T) {
	tests := []struct {
		name     string
		token    AntigravityToken
		expected string
	}{
		{
			name: "nested token",
			token: AntigravityToken{
				Token: &OAuthToken{RefreshToken: "nested-refresh"},
			},
			expected: "nested-refresh",
		},
		{
			name: "legacy token",
			token: AntigravityToken{
				RefreshToken: "legacy-refresh",
			},
			expected: "legacy-refresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.GetRefreshToken()
			if result != tt.expected {
				t.Errorf("GetRefreshToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetExpiry(t *testing.T) {
	now := time.Now()
	futureTime := now.Add(1 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name        string
		token       AntigravityToken
		expectZero  bool
		expectAfter time.Time
	}{
		{
			name: "nested expiry",
			token: AntigravityToken{
				Token: &OAuthToken{Expiry: futureTime},
			},
			expectZero:  false,
			expectAfter: now,
		},
		{
			name: "legacy expires_at",
			token: AntigravityToken{
				ExpiresAt: futureTime,
			},
			expectZero:  false,
			expectAfter: now,
		},
		{
			name: "expires_in",
			token: AntigravityToken{
				ExpiresIn: 3600,
			},
			expectZero:  false,
			expectAfter: now,
		},
		{
			name:       "empty token",
			token:      AntigravityToken{},
			expectZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.GetExpiry()
			if tt.expectZero {
				if !result.IsZero() {
					t.Errorf("GetExpiry() = %v, want zero time", result)
				}
			} else {
				if result.IsZero() {
					t.Error("GetExpiry() returned zero time, expected non-zero")
				}
				if !result.After(tt.expectAfter) {
					t.Errorf("GetExpiry() = %v, expected after %v", result, tt.expectAfter)
				}
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name     string
		token    AntigravityToken
		expected bool
	}{
		{
			name: "expired token",
			token: AntigravityToken{
				Token: &OAuthToken{Expiry: pastTime},
			},
			expected: true,
		},
		{
			name: "valid token",
			token: AntigravityToken{
				Token: &OAuthToken{Expiry: futureTime},
			},
			expected: false,
		},
		{
			name:     "no expiry info - assume valid",
			token:    AntigravityToken{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsExpired()
			if result != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoadAntigravityTokenFromPath(t *testing.T) {
	// Create a temp file with valid token data
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "oauth_creds.json")

	validToken := `{
		"token": {
			"access_token": "test-access-token",
			"refresh_token": "test-refresh-token",
			"expiry": "2099-01-01T00:00:00Z"
		},
		"email": "test@example.com",
		"project_id": "test-project"
	}`

	if err := os.WriteFile(tokenPath, []byte(validToken), 0600); err != nil {
		t.Fatalf("Failed to write test token: %v", err)
	}

	token, err := LoadAntigravityTokenFromPath(tokenPath)
	if err != nil {
		t.Fatalf("LoadAntigravityTokenFromPath() error = %v", err)
	}

	if token.GetAccessToken() != "test-access-token" {
		t.Errorf("AccessToken = %q, want %q", token.GetAccessToken(), "test-access-token")
	}
	if token.GetRefreshToken() != "test-refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", token.GetRefreshToken(), "test-refresh-token")
	}
	if token.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", token.Email, "test@example.com")
	}
	if token.ProjectID != "test-project" {
		t.Errorf("ProjectID = %q, want %q", token.ProjectID, "test-project")
	}
}

func TestLoadAntigravityTokenFromPath_NotFound(t *testing.T) {
	_, err := LoadAntigravityTokenFromPath("/nonexistent/path/oauth_creds.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestLoadAntigravityTokenFromPath_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "oauth_creds.json")

	if err := os.WriteFile(tokenPath, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadAntigravityTokenFromPath(tokenPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
