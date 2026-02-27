package codex

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// createTestJWT creates a test JWT token with the given claims.
// It creates a valid 3-part JWT structure (header.payload.signature) for testing.
func createTestJWT(claims JWTClaims) string {
	header := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(claimsJSON)
	signature := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte("test-signature"))
	return header + "." + payload + "." + signature
}

func TestParseJWTToken_ValidToken(t *testing.T) {
	tests := []struct {
		name          string
		claims        JWTClaims
		wantEmail     string
		wantAccountID string
		wantIssuer    string
		wantSubject   string
	}{
		{
			name: "valid token with all fields",
			claims: JWTClaims{
				Email:         "user@example.com",
				EmailVerified: true,
				Iss:           "https://auth.openai.com/",
				Sub:           "auth0|user123",
				Exp:           int(time.Now().Add(time.Hour).Unix()),
				Iat:           int(time.Now().Unix()),
				CodexAuthInfo: CodexAuthInfo{
					ChatgptAccountID: "acct_abc123",
					ChatgptUserID:    "user_xyz789",
					ChatgptPlanType:  "plus",
					UserID:           "user-id-123",
				},
			},
			wantEmail:     "user@example.com",
			wantAccountID: "acct_abc123",
			wantIssuer:    "https://auth.openai.com/",
			wantSubject:   "auth0|user123",
		},
		{
			name: "valid token with minimal fields",
			claims: JWTClaims{
				Email: "minimal@example.com",
				Exp:   int(time.Now().Add(time.Hour).Unix()),
			},
			wantEmail:     "minimal@example.com",
			wantAccountID: "",
			wantIssuer:    "",
			wantSubject:   "",
		},
		{
			name: "valid token with organization data",
			claims: JWTClaims{
				Email: "org-user@example.com",
				Sub:   "auth0|orguser456",
				CodexAuthInfo: CodexAuthInfo{
					ChatgptAccountID: "acct_org789",
					Organizations: []Organizations{
						{
							ID:        "org-id-1",
							IsDefault: true,
							Role:      "admin",
							Title:     "My Organization",
						},
					},
				},
			},
			wantEmail:     "org-user@example.com",
			wantAccountID: "acct_org789",
			wantSubject:   "auth0|orguser456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := createTestJWT(tt.claims)

			got, err := ParseJWTToken(token)
			if err != nil {
				t.Fatalf("ParseJWTToken() unexpected error = %v", err)
			}

			if got.Email != tt.wantEmail {
				t.Errorf("ParseJWTToken() Email = %v, want %v", got.Email, tt.wantEmail)
			}

			if got.GetAccountID() != tt.wantAccountID {
				t.Errorf("ParseJWTToken() AccountID = %v, want %v", got.GetAccountID(), tt.wantAccountID)
			}

			if got.Iss != tt.wantIssuer {
				t.Errorf("ParseJWTToken() Issuer = %v, want %v", got.Iss, tt.wantIssuer)
			}

			if got.Sub != tt.wantSubject {
				t.Errorf("ParseJWTToken() Subject = %v, want %v", got.Sub, tt.wantSubject)
			}
		})
	}
}

func TestParseJWTToken_InvalidToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr string
	}{
		{
			name:    "empty token",
			token:   "",
			wantErr: "invalid JWT token format",
		},
		{
			name:    "single part token",
			token:   "singlepart",
			wantErr: "invalid JWT token format",
		},
		{
			name:    "two part token",
			token:   "header.payload",
			wantErr: "invalid JWT token format",
		},
		{
			name:    "four part token",
			token:   "header.payload.signature.extra",
			wantErr: "invalid JWT token format",
		},
		{
			name:    "invalid base64 in payload",
			token:   "header.!!!invalid-base64!!!.signature",
			wantErr: "failed to decode JWT claims",
		},
		{
			name:    "invalid JSON in payload",
			token:   "header." + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte("not-json")) + ".signature",
			wantErr: "failed to unmarshal JWT claims",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJWTToken(tt.token)
			if err == nil {
				t.Fatalf("ParseJWTToken() expected error containing %q, got nil", tt.wantErr)
			}

			if got != nil {
				t.Errorf("ParseJWTToken() expected nil result on error, got %v", got)
			}

			if !containsSubstring(err.Error(), tt.wantErr) {
				t.Errorf("ParseJWTToken() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseJWTToken_ExtractClaims(t *testing.T) {
	tests := []struct {
		name   string
		claims JWTClaims
		checks func(t *testing.T, got *JWTClaims)
	}{
		{
			name: "extract standard claims",
			claims: JWTClaims{
				Aud:      []string{"app_test123"},
				AtHash:   "at_hash_value",
				AuthTime: 1700000000,
				Exp:      1700003600,
				Iat:      1700000000,
				Jti:      "jti_unique_id",
				Rat:      1699999900,
				Sid:      "session_id_123",
			},
			checks: func(t *testing.T, got *JWTClaims) {
				if len(got.Aud) != 1 || got.Aud[0] != "app_test123" {
					t.Errorf("Aud = %v, want [app_test123]", got.Aud)
				}
				if got.AtHash != "at_hash_value" {
					t.Errorf("AtHash = %v, want at_hash_value", got.AtHash)
				}
				if got.AuthTime != 1700000000 {
					t.Errorf("AuthTime = %v, want 1700000000", got.AuthTime)
				}
				if got.Jti != "jti_unique_id" {
					t.Errorf("Jti = %v, want jti_unique_id", got.Jti)
				}
				if got.Sid != "session_id_123" {
					t.Errorf("Sid = %v, want session_id_123", got.Sid)
				}
			},
		},
		{
			name: "extract auth provider info",
			claims: JWTClaims{
				AuthProvider: "google-oauth2",
				Email:        "google@example.com",
				CodexAuthInfo: CodexAuthInfo{
					ChatgptPlanType: "team",
					UserID:          "user-google-123",
				},
			},
			checks: func(t *testing.T, got *JWTClaims) {
				if got.AuthProvider != "google-oauth2" {
					t.Errorf("AuthProvider = %v, want google-oauth2", got.AuthProvider)
				}
				if got.CodexAuthInfo.ChatgptPlanType != "team" {
					t.Errorf("ChatgptPlanType = %v, want team", got.CodexAuthInfo.ChatgptPlanType)
				}
			},
		},
		{
			name: "extract multiple organizations",
			claims: JWTClaims{
				Email: "multi-org@example.com",
				CodexAuthInfo: CodexAuthInfo{
					Organizations: []Organizations{
						{ID: "org-1", IsDefault: true, Role: "owner", Title: "Primary Org"},
						{ID: "org-2", IsDefault: false, Role: "member", Title: "Secondary Org"},
					},
				},
			},
			checks: func(t *testing.T, got *JWTClaims) {
				orgs := got.CodexAuthInfo.Organizations
				if len(orgs) != 2 {
					t.Fatalf("Organizations count = %v, want 2", len(orgs))
				}
				if orgs[0].ID != "org-1" || !orgs[0].IsDefault {
					t.Errorf("First org = %+v, want org-1 as default", orgs[0])
				}
				if orgs[1].Role != "member" {
					t.Errorf("Second org role = %v, want member", orgs[1].Role)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := createTestJWT(tt.claims)

			got, err := ParseJWTToken(token)
			if err != nil {
				t.Fatalf("ParseJWTToken() unexpected error = %v", err)
			}

			tt.checks(t, got)
		})
	}
}

func TestParseJWTToken_ValidateExpiry(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expTime   int
		wantValid bool
	}{
		{
			name:      "token expires in 1 hour",
			expTime:   int(now.Add(time.Hour).Unix()),
			wantValid: true,
		},
		{
			name:      "token expires in 1 minute",
			expTime:   int(now.Add(time.Minute).Unix()),
			wantValid: true,
		},
		{
			name:      "token expired 1 hour ago",
			expTime:   int(now.Add(-time.Hour).Unix()),
			wantValid: false,
		},
		{
			name:      "token expired 1 minute ago",
			expTime:   int(now.Add(-time.Minute).Unix()),
			wantValid: false,
		},
		{
			name:      "token expires now",
			expTime:   int(now.Unix()),
			wantValid: false,
		},
		{
			name:      "token with zero expiry",
			expTime:   0,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := JWTClaims{
				Email: "test@example.com",
				Exp:   tt.expTime,
			}
			token := createTestJWT(claims)

			got, err := ParseJWTToken(token)
			if err != nil {
				t.Fatalf("ParseJWTToken() unexpected error = %v", err)
			}

			// Check if token is valid based on expiry
			isValid := time.Unix(int64(got.Exp), 0).After(time.Now())

			if isValid != tt.wantValid {
				t.Errorf("Token validity = %v, want %v (exp: %v, now: %v)",
					isValid, tt.wantValid, time.Unix(int64(got.Exp), 0), time.Now())
			}
		})
	}
}

func TestJWTClaims_GetUserEmail(t *testing.T) {
	tests := []struct {
		name      string
		claims    JWTClaims
		wantEmail string
	}{
		{
			name:      "standard email",
			claims:    JWTClaims{Email: "user@example.com"},
			wantEmail: "user@example.com",
		},
		{
			name:      "empty email",
			claims:    JWTClaims{Email: ""},
			wantEmail: "",
		},
		{
			name:      "email with subdomain",
			claims:    JWTClaims{Email: "user@mail.example.com"},
			wantEmail: "user@mail.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.claims.GetUserEmail(); got != tt.wantEmail {
				t.Errorf("GetUserEmail() = %v, want %v", got, tt.wantEmail)
			}
		})
	}
}

func TestJWTClaims_GetAccountID(t *testing.T) {
	tests := []struct {
		name          string
		claims        JWTClaims
		wantAccountID string
	}{
		{
			name: "with account ID",
			claims: JWTClaims{
				CodexAuthInfo: CodexAuthInfo{
					ChatgptAccountID: "acct_12345",
				},
			},
			wantAccountID: "acct_12345",
		},
		{
			name:          "empty account ID",
			claims:        JWTClaims{},
			wantAccountID: "",
		},
		{
			name: "account ID with special characters",
			claims: JWTClaims{
				CodexAuthInfo: CodexAuthInfo{
					ChatgptAccountID: "acct_abc-123_xyz",
				},
			},
			wantAccountID: "acct_abc-123_xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.claims.GetAccountID(); got != tt.wantAccountID {
				t.Errorf("GetAccountID() = %v, want %v", got, tt.wantAccountID)
			}
		})
	}
}

func TestBase64URLDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "no padding needed",
			input:   "dGVzdA",
			want:    "test",
			wantErr: false,
		},
		{
			name:    "one padding character needed",
			input:   "dGVzdDE",
			want:    "test1",
			wantErr: false,
		},
		{
			name:    "two padding characters needed",
			input:   "dGVzdDEy",
			want:    "test12",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "URL-safe characters",
			input:   "YWJjLV8",
			want:    "abc-_",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := base64URLDecode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("base64URLDecode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("base64URLDecode() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
