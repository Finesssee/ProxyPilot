package claude

import (
	"encoding/json"
	"testing"
	"time"
)

// TestClaudeAuthBundle_Parse tests parsing of ClaudeAuthBundle JSON.
func TestClaudeAuthBundle_Parse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		wantErr     bool
		checkFields func(t *testing.T, bundle *ClaudeAuthBundle)
	}{
		{
			name: "parse complete auth bundle",
			jsonData: `{
				"api_key": "sk-ant-api-key-12345",
				"token_data": {
					"access_token": "access_token_value",
					"refresh_token": "refresh_token_value",
					"email": "user@example.com",
					"expired": "2025-01-15T12:00:00Z"
				},
				"last_refresh": "2025-01-15T10:00:00Z"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, bundle *ClaudeAuthBundle) {
				if bundle.APIKey != "sk-ant-api-key-12345" {
					t.Errorf("APIKey = %v, want sk-ant-api-key-12345", bundle.APIKey)
				}
				if bundle.TokenData.AccessToken != "access_token_value" {
					t.Errorf("TokenData.AccessToken = %v, want access_token_value", bundle.TokenData.AccessToken)
				}
				if bundle.TokenData.Email != "user@example.com" {
					t.Errorf("TokenData.Email = %v, want user@example.com", bundle.TokenData.Email)
				}
				if bundle.LastRefresh != "2025-01-15T10:00:00Z" {
					t.Errorf("LastRefresh = %v, want 2025-01-15T10:00:00Z", bundle.LastRefresh)
				}
			},
		},
		{
			name: "parse bundle without api key",
			jsonData: `{
				"token_data": {
					"access_token": "token_only",
					"refresh_token": "refresh_only"
				},
				"last_refresh": "2025-01-15T10:00:00Z"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, bundle *ClaudeAuthBundle) {
				if bundle.APIKey != "" {
					t.Errorf("APIKey should be empty, got %v", bundle.APIKey)
				}
				if bundle.TokenData.AccessToken != "token_only" {
					t.Errorf("TokenData.AccessToken = %v, want token_only", bundle.TokenData.AccessToken)
				}
			},
		},
		{
			name: "parse bundle with empty token data",
			jsonData: `{
				"api_key": "sk-ant-key",
				"token_data": {},
				"last_refresh": ""
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, bundle *ClaudeAuthBundle) {
				if bundle.APIKey != "sk-ant-key" {
					t.Errorf("APIKey = %v, want sk-ant-key", bundle.APIKey)
				}
				if bundle.TokenData.AccessToken != "" {
					t.Errorf("TokenData.AccessToken should be empty")
				}
			},
		},
		{
			name:     "parse empty JSON object",
			jsonData: `{}`,
			wantErr:  false,
			checkFields: func(t *testing.T, bundle *ClaudeAuthBundle) {
				if bundle.APIKey != "" {
					t.Errorf("APIKey should be empty for empty JSON")
				}
			},
		},
		{
			name:     "invalid JSON",
			jsonData: `{"invalid": json}`,
			wantErr:  true,
			checkFields: func(t *testing.T, bundle *ClaudeAuthBundle) {
				// Not called on error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bundle ClaudeAuthBundle
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

// TestClaudeTokenData_Parse tests parsing of ClaudeTokenData JSON.
func TestClaudeTokenData_Parse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		wantErr     bool
		checkFields func(t *testing.T, td *ClaudeTokenData)
	}{
		{
			name: "parse complete token data",
			jsonData: `{
				"access_token": "access_token_here",
				"refresh_token": "refresh_token_here",
				"email": "user@example.com",
				"expired": "2025-01-15T12:00:00Z"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, td *ClaudeTokenData) {
				if td.AccessToken != "access_token_here" {
					t.Errorf("AccessToken = %v, want access_token_here", td.AccessToken)
				}
				if td.RefreshToken != "refresh_token_here" {
					t.Errorf("RefreshToken = %v, want refresh_token_here", td.RefreshToken)
				}
				if td.Email != "user@example.com" {
					t.Errorf("Email = %v, want user@example.com", td.Email)
				}
			},
		},
		{
			name: "parse minimal token data",
			jsonData: `{
				"access_token": "only_access"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, td *ClaudeTokenData) {
				if td.AccessToken != "only_access" {
					t.Errorf("AccessToken = %v, want only_access", td.AccessToken)
				}
				if td.RefreshToken != "" {
					t.Errorf("RefreshToken should be empty")
				}
			},
		},
		{
			name: "parse token with special characters",
			jsonData: `{
				"access_token": "token-with_special.chars/and+more==",
				"refresh_token": "refresh/token+value=",
				"email": "user+test@sub.example.com"
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, td *ClaudeTokenData) {
				if td.AccessToken != "token-with_special.chars/and+more==" {
					t.Errorf("AccessToken mismatch with special chars")
				}
				if td.Email != "user+test@sub.example.com" {
					t.Errorf("Email mismatch with special chars")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var td ClaudeTokenData
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

// TestPKCECodes_Parse tests parsing of PKCE codes.
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
		{
			name: "parse empty PKCE codes",
			jsonData: `{
				"code_verifier": "",
				"code_challenge": ""
			}`,
			wantErr: false,
			checkFields: func(t *testing.T, codes *PKCECodes) {
				if codes.CodeVerifier != "" {
					t.Errorf("CodeVerifier should be empty")
				}
				if codes.CodeChallenge != "" {
					t.Errorf("CodeChallenge should be empty")
				}
			},
		},
		{
			name:     "parse minimal JSON",
			jsonData: `{}`,
			wantErr:  false,
			checkFields: func(t *testing.T, codes *PKCECodes) {
				if codes.CodeVerifier != "" || codes.CodeChallenge != "" {
					t.Errorf("Empty JSON should result in empty codes")
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

// TestClaudeToken_IsValid tests token validation logic.
func TestClaudeToken_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		token     ClaudeTokenStorage
		wantValid bool
	}{
		{
			name: "valid token with all fields",
			token: ClaudeTokenStorage{
				AccessToken:  "valid-access-token",
				RefreshToken: "valid-refresh-token",
				Email:        "user@example.com",
				Expire:       time.Now().Add(time.Hour).Format(time.RFC3339),
			},
			wantValid: true,
		},
		{
			name: "token with empty access token",
			token: ClaudeTokenStorage{
				AccessToken:  "",
				RefreshToken: "valid-refresh-token",
			},
			wantValid: false,
		},
		{
			name: "token with only access token",
			token: ClaudeTokenStorage{
				AccessToken: "access-only",
			},
			wantValid: true,
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

// TestClaudeTokenStorage_Serialize tests JSON serialization round-trip.
func TestClaudeTokenStorage_Serialize(t *testing.T) {
	original := ClaudeTokenStorage{
		IDToken:      "test-id-token",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		LastRefresh:  "2025-01-15T10:00:00Z",
		Email:        "test@example.com",
		Type:         "claude",
		Expire:       "2025-01-15T11:00:00Z",
	}

	// Serialize to JSON
	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Failed to marshal token: %v", err)
	}

	// Deserialize back
	var restored ClaudeTokenStorage
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
