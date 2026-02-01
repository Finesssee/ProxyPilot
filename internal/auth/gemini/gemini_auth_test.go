package gemini

import (
	"encoding/json"
	"testing"
)

// getTokenAccessToken extracts access_token from the Token field (which is type any).
func getTokenAccessToken(token any) string {
	if token == nil {
		return ""
	}
	switch t := token.(type) {
	case map[string]any:
		if v, ok := t["access_token"]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	case map[string]string:
		return t["access_token"]
	}
	return ""
}

// TestGeminiTokenStorage_Parse tests parsing of GeminiTokenStorage JSON.
func TestGeminiTokenStorage_Parse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		wantErr     bool
		checkFields func(t *testing.T, ts *GeminiTokenStorage)
	}{
		{
			name: "parse complete token storage",
			jsonData: `{
				"token": {
					"access_token": "ya29.test-token",
					"refresh_token": "1//test-refresh"
				},
				"project_id": "my-project-123",
				"email": "user@example.com",
				"auto": true,
				"checked": true,
				"type": "gemini"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, ts *GeminiTokenStorage) {
				if getTokenAccessToken(ts.Token) != "ya29.test-token" {
					t.Errorf("Token[access_token] mismatch, got %v", getTokenAccessToken(ts.Token))
				}
				if ts.ProjectID != "my-project-123" {
					t.Errorf("ProjectID = %v, want my-project-123", ts.ProjectID)
				}
				if ts.Email != "user@example.com" {
					t.Errorf("Email = %v, want user@example.com", ts.Email)
				}
				if !ts.Auto {
					t.Errorf("Auto = false, want true")
				}
				if !ts.Checked {
					t.Errorf("Checked = false, want true")
				}
			},
		},
		{
			name: "parse minimal token storage",
			jsonData: `{
				"token": {"access_token": "minimal-token"},
				"project_id": "project",
				"email": "test@test.com"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, ts *GeminiTokenStorage) {
				if getTokenAccessToken(ts.Token) != "minimal-token" {
					t.Errorf("Token[access_token] = %v, want minimal-token", getTokenAccessToken(ts.Token))
				}
				if ts.Auto {
					t.Errorf("Auto should be false by default")
				}
				if ts.Checked {
					t.Errorf("Checked should be false by default")
				}
			},
		},
		{
			name: "parse token with multiple projects (comma-separated)",
			jsonData: `{
				"token": {"access_token": "multi-project-token"},
				"project_id": "project1,project2,project3",
				"email": "multi@example.com"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, ts *GeminiTokenStorage) {
				if ts.ProjectID != "project1,project2,project3" {
					t.Errorf("ProjectID = %v, want project1,project2,project3", ts.ProjectID)
				}
			},
		},
		{
			name: "parse token with all projects",
			jsonData: `{
				"token": {"access_token": "all-projects-token"},
				"project_id": "all",
				"email": "all@example.com"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, ts *GeminiTokenStorage) {
				if ts.ProjectID != "all" {
					t.Errorf("ProjectID = %v, want all", ts.ProjectID)
				}
			},
		},
		{
			name:     "parse empty JSON object",
			jsonData: `{}`,
			wantErr:  false,
			checkFields: func(t *testing.T, ts *GeminiTokenStorage) {
				// Token is of type any, so we need to type assert to check if it's empty
				if ts.Token != nil {
					if tokenMap, ok := ts.Token.(map[string]any); ok && len(tokenMap) > 0 {
						t.Errorf("Token should be nil or empty for empty JSON")
					}
				}
				if ts.ProjectID != "" {
					t.Errorf("ProjectID should be empty")
				}
			},
		},
		{
			name:     "invalid JSON",
			jsonData: `{"invalid": json}`,
			wantErr:  true,
			checkFields: func(t *testing.T, ts *GeminiTokenStorage) {
				// Not called on error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts GeminiTokenStorage
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

// TestGeminiTokenStorage_Serialize tests JSON serialization round-trip.
func TestGeminiTokenStorage_Serialize(t *testing.T) {
	original := GeminiTokenStorage{
		Token: map[string]any{
			"access_token":  "ya29.test-access-token",
			"refresh_token": "1//test-refresh-token",
			"token_type":    "Bearer",
		},
		ProjectID: "test-project-456",
		Email:     "serialize@example.com",
		Auto:      true,
		Checked:   true,
		Type:      "gemini",
	}

	// Serialize to JSON
	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Failed to marshal token: %v", err)
	}

	// Deserialize back
	var restored GeminiTokenStorage
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal token: %v", err)
	}

	// Verify all fields match
	if getTokenAccessToken(restored.Token) != "ya29.test-access-token" {
		t.Errorf("Token[access_token] mismatch")
	}
	if restored.ProjectID != original.ProjectID {
		t.Errorf("ProjectID = %v, want %v", restored.ProjectID, original.ProjectID)
	}
	if restored.Email != original.Email {
		t.Errorf("Email = %v, want %v", restored.Email, original.Email)
	}
	if restored.Auto != original.Auto {
		t.Errorf("Auto = %v, want %v", restored.Auto, original.Auto)
	}
	if restored.Checked != original.Checked {
		t.Errorf("Checked = %v, want %v", restored.Checked, original.Checked)
	}
	if restored.Type != original.Type {
		t.Errorf("Type = %v, want %v", restored.Type, original.Type)
	}
}

// TestGeminiToken_IsValid tests token validation logic.
func TestGeminiToken_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		token     GeminiTokenStorage
		wantValid bool
	}{
		{
			name: "valid token with access_token",
			token: GeminiTokenStorage{
				Token: map[string]any{
					"access_token": "valid-token",
				},
				ProjectID: "project",
				Email:     "user@example.com",
			},
			wantValid: true,
		},
		{
			name: "token with empty token map",
			token: GeminiTokenStorage{
				Token:     map[string]any{},
				ProjectID: "project",
				Email:     "user@example.com",
			},
			wantValid: false,
		},
		{
			name: "token with nil token",
			token: GeminiTokenStorage{
				Token:     nil,
				ProjectID: "project",
				Email:     "user@example.com",
			},
			wantValid: false,
		},
		{
			name: "token with empty access_token value",
			token: GeminiTokenStorage{
				Token: map[string]any{
					"access_token": "",
				},
				ProjectID: "project",
				Email:     "user@example.com",
			},
			wantValid: false,
		},
		{
			name: "token with only refresh_token (no access_token)",
			token: GeminiTokenStorage{
				Token: map[string]any{
					"refresh_token": "has-refresh",
				},
				ProjectID: "project",
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := getTokenAccessToken(tt.token.Token) != ""
			if isValid != tt.wantValid {
				t.Errorf("Token validity = %v, want %v", isValid, tt.wantValid)
			}
		})
	}
}

// TestCredentialFileName_EdgeCases tests edge cases for credential file naming.
func TestCredentialFileName_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		projectID     string
		includePrefix bool
		want          string
	}{
		{
			name:          "empty email",
			email:         "",
			projectID:     "my-project",
			includePrefix: false,
			want:          "-my-project.json",
		},
		{
			name:          "empty project ID",
			email:         "user@example.com",
			projectID:     "",
			includePrefix: false,
			want:          "user@example.com-.json",
		},
		{
			name:          "both empty",
			email:         "",
			projectID:     "",
			includePrefix: false,
			want:          "-.json",
		},
		{
			name:          "unicode email",
			email:         "user@例え.com",
			projectID:     "project",
			includePrefix: false,
			want:          "user@例え.com-project.json",
		},
		{
			name:          "project ID with special chars",
			email:         "user@example.com",
			projectID:     "my_project-123",
			includePrefix: true,
			want:          "gemini-user@example.com-my_project-123.json",
		},
		{
			name:          "all uppercase",
			email:         "USER@EXAMPLE.COM",
			projectID:     "MY-PROJECT",
			includePrefix: false,
			want:          "USER@EXAMPLE.COM-MY-PROJECT.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CredentialFileName(tt.email, tt.projectID, tt.includePrefix)
			if got != tt.want {
				t.Errorf("CredentialFileName(%q, %q, %v) = %q, want %q",
					tt.email, tt.projectID, tt.includePrefix, got, tt.want)
			}
		})
	}
}

// TestGeminiTokenStorage_TokenOperations tests token field operations.
func TestGeminiTokenStorage_TokenOperations(t *testing.T) {
	ts := &GeminiTokenStorage{
		Token: map[string]any{
			"access_token":  "test-access",
			"refresh_token": "test-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		},
	}

	tokenMap, ok := ts.Token.(map[string]any)
	if !ok {
		t.Fatalf("Token should be map[string]any")
	}

	if len(tokenMap) != 4 {
		t.Errorf("Token map should have 4 entries, got %d", len(tokenMap))
	}

	// Test retrieving tokens
	if tokenMap["access_token"] != "test-access" {
		t.Errorf("access_token mismatch")
	}
	if tokenMap["refresh_token"] != "test-refresh" {
		t.Errorf("refresh_token mismatch")
	}

	// Test missing key returns nil
	if tokenMap["nonexistent"] != nil {
		t.Errorf("nonexistent key should return nil")
	}

	// Test updating token
	tokenMap["access_token"] = "updated-access"
	if tokenMap["access_token"] != "updated-access" {
		t.Errorf("access_token should be updated")
	}

	// Test deleting token
	delete(tokenMap, "expires_in")
	if len(tokenMap) != 3 {
		t.Errorf("Token map should have 3 entries after delete, got %d", len(tokenMap))
	}
}

// TestGeminiTokenStorage_JSONFieldNames tests that JSON field names are correct.
func TestGeminiTokenStorage_JSONFieldNames(t *testing.T) {
	ts := GeminiTokenStorage{
		Token:     map[string]any{"access_token": "test"},
		ProjectID: "project-123",
		Email:     "test@example.com",
		Auto:      true,
		Checked:   false,
		Type:      "gemini",
	}

	data, err := json.Marshal(&ts)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(data)

	// Check expected JSON field names
	expectedFields := []string{
		`"token"`,
		`"project_id"`,
		`"email"`,
		`"auto"`,
		`"checked"`,
		`"type"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON should contain field %s, got: %s", field, jsonStr)
		}
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
