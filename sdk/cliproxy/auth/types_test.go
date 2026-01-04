package auth

import (
	"testing"
	"time"
)

func TestAuth_Clone_DeepCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		auth *Auth
	}{
		{
			name: "nil auth",
			auth: nil,
		},
		{
			name: "empty auth",
			auth: &Auth{},
		},
		{
			name: "auth with attributes",
			auth: &Auth{
				ID:       "test-id",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "secret-key",
					"region":  "us-west",
				},
			},
		},
		{
			name: "auth with metadata",
			auth: &Auth{
				ID:       "oauth-id",
				Provider: "gemini-cli",
				Metadata: map[string]any{
					"email":      "user@example.com",
					"project_id": "my-project",
				},
			},
		},
		{
			name: "auth with model states",
			auth: &Auth{
				ID:       "model-state-id",
				Provider: "claude",
				ModelStates: map[string]*ModelState{
					"claude-4-opus": {
						Status:      StatusActive,
						Unavailable: false,
					},
					"claude-4-sonnet": {
						Status:        StatusError,
						StatusMessage: "rate limited",
						Unavailable:   true,
						LastError: &Error{
							Code:       "rate_limit",
							Message:    "Too many requests",
							Retryable:  true,
							HTTPStatus: 429,
						},
					},
				},
			},
		},
		{
			name: "auth with all fields",
			auth: &Auth{
				ID:       "full-auth",
				Provider: "gemini",
				Prefix:   "teamA",
				Label:    "Primary Account",
				Attributes: map[string]string{
					"api_key": "key-123",
				},
				Metadata: map[string]any{
					"email": "team@example.com",
				},
				ModelStates: map[string]*ModelState{
					"gemini-pro": {
						Status: StatusActive,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clone := tc.auth.Clone()

			if tc.auth == nil {
				if clone != nil {
					t.Fatalf("Clone() = %v, want nil", clone)
				}
				return
			}

			if clone == nil {
				t.Fatalf("Clone() = nil, want non-nil")
			}

			// Verify clone is a different pointer
			if clone == tc.auth {
				t.Fatalf("Clone() returned same pointer")
			}

			// Verify basic fields are copied
			if clone.ID != tc.auth.ID {
				t.Errorf("Clone().ID = %q, want %q", clone.ID, tc.auth.ID)
			}
			if clone.Provider != tc.auth.Provider {
				t.Errorf("Clone().Provider = %q, want %q", clone.Provider, tc.auth.Provider)
			}

			// Verify Attributes map is a deep copy
			if len(tc.auth.Attributes) > 0 {
				if len(clone.Attributes) != len(tc.auth.Attributes) {
					t.Fatalf("Clone().Attributes length = %d, want %d", len(clone.Attributes), len(tc.auth.Attributes))
				}
				// Mutate original and verify clone is unaffected
				originalKey := ""
				for k := range tc.auth.Attributes {
					originalKey = k
					break
				}
				originalValue := tc.auth.Attributes[originalKey]
				tc.auth.Attributes[originalKey] = "mutated"
				if clone.Attributes[originalKey] != originalValue {
					t.Errorf("Clone().Attributes was mutated when original changed")
				}
				tc.auth.Attributes[originalKey] = originalValue // restore
			}

			// Verify Metadata map is a deep copy
			if len(tc.auth.Metadata) > 0 {
				if len(clone.Metadata) != len(tc.auth.Metadata) {
					t.Fatalf("Clone().Metadata length = %d, want %d", len(clone.Metadata), len(tc.auth.Metadata))
				}
			}

			// Verify ModelStates map is a deep copy
			if len(tc.auth.ModelStates) > 0 {
				if len(clone.ModelStates) != len(tc.auth.ModelStates) {
					t.Fatalf("Clone().ModelStates length = %d, want %d", len(clone.ModelStates), len(tc.auth.ModelStates))
				}
				for model, state := range tc.auth.ModelStates {
					cloneState := clone.ModelStates[model]
					if cloneState == nil {
						t.Fatalf("Clone().ModelStates[%q] = nil", model)
					}
					if cloneState == state {
						t.Errorf("Clone().ModelStates[%q] is same pointer as original", model)
					}
				}
			}
		})
	}
}

func TestAuth_EnsureIndex_GeneratesUniqueIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		auth         *Auth
		wantNonEmpty bool
	}{
		{
			name:         "nil auth",
			auth:         nil,
			wantNonEmpty: false,
		},
		{
			name:         "empty auth with no identifiers",
			auth:         &Auth{},
			wantNonEmpty: false,
		},
		{
			name: "auth with filename",
			auth: &Auth{
				FileName: "/path/to/auth.json",
			},
			wantNonEmpty: true,
		},
		{
			name: "auth with api_key",
			auth: &Auth{
				Attributes: map[string]string{
					"api_key": "sk-1234567890",
				},
			},
			wantNonEmpty: true,
		},
		{
			name: "auth with ID only",
			auth: &Auth{
				ID: "unique-auth-id",
			},
			wantNonEmpty: true,
		},
		{
			name: "auth with filename takes priority",
			auth: &Auth{
				ID:       "some-id",
				FileName: "/path/to/file.json",
				Attributes: map[string]string{
					"api_key": "sk-key",
				},
			},
			wantNonEmpty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			index := tc.auth.EnsureIndex()

			if tc.wantNonEmpty {
				if index == "" {
					t.Fatalf("EnsureIndex() = empty, want non-empty")
				}
				// Verify index is a hex string (16 chars from 8 bytes)
				if len(index) != 16 {
					t.Errorf("EnsureIndex() length = %d, want 16", len(index))
				}
			} else {
				if index != "" {
					t.Fatalf("EnsureIndex() = %q, want empty", index)
				}
			}
		})
	}

	// Verify different inputs produce different indexes
	auth1 := &Auth{FileName: "/path/a.json"}
	auth2 := &Auth{FileName: "/path/b.json"}
	auth3 := &Auth{Attributes: map[string]string{"api_key": "key-1"}}
	auth4 := &Auth{Attributes: map[string]string{"api_key": "key-2"}}

	idx1 := auth1.EnsureIndex()
	idx2 := auth2.EnsureIndex()
	idx3 := auth3.EnsureIndex()
	idx4 := auth4.EnsureIndex()

	if idx1 == idx2 {
		t.Errorf("Different filenames should produce different indexes")
	}
	if idx3 == idx4 {
		t.Errorf("Different api_keys should produce different indexes")
	}
	if idx1 == idx3 {
		t.Errorf("Filename and api_key should produce different indexes")
	}
}

func TestAuth_EnsureIndex_IdempotentWhenAssigned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		auth *Auth
	}{
		{
			name: "auth with filename",
			auth: &Auth{
				FileName: "/path/to/auth.json",
			},
		},
		{
			name: "auth with api_key",
			auth: &Auth{
				Attributes: map[string]string{
					"api_key": "sk-test-key",
				},
			},
		},
		{
			name: "auth with ID",
			auth: &Auth{
				ID: "stable-id",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// First call generates and assigns index
			first := tc.auth.EnsureIndex()
			if first == "" {
				t.Fatalf("EnsureIndex() first call = empty")
			}

			// Subsequent calls should return the same value
			second := tc.auth.EnsureIndex()
			if second != first {
				t.Errorf("EnsureIndex() second call = %q, want %q", second, first)
			}

			third := tc.auth.EnsureIndex()
			if third != first {
				t.Errorf("EnsureIndex() third call = %q, want %q", third, first)
			}

			// Verify the Index field is set
			if tc.auth.Index != first {
				t.Errorf("auth.Index = %q, want %q", tc.auth.Index, first)
			}
		})
	}
}

func TestAuth_AccountInfo_APIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		auth      *Auth
		wantType  string
		wantValue string
	}{
		{
			name:      "nil auth",
			auth:      nil,
			wantType:  "",
			wantValue: "",
		},
		{
			name:      "empty auth",
			auth:      &Auth{},
			wantType:  "",
			wantValue: "",
		},
		{
			name: "auth with api_key",
			auth: &Auth{
				Provider: "openai",
				Attributes: map[string]string{
					"api_key": "sk-1234567890abcdef",
				},
			},
			wantType:  "api_key",
			wantValue: "sk-1234567890abcdef",
		},
		{
			name: "auth with empty api_key",
			auth: &Auth{
				Provider: "openai",
				Attributes: map[string]string{
					"api_key": "",
				},
			},
			wantType:  "",
			wantValue: "",
		},
		{
			name: "auth with other attributes but no api_key",
			auth: &Auth{
				Provider: "custom",
				Attributes: map[string]string{
					"region":   "us-west",
					"endpoint": "https://api.example.com",
				},
			},
			wantType:  "",
			wantValue: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotValue := tc.auth.AccountInfo()

			if gotType != tc.wantType {
				t.Errorf("AccountInfo() type = %q, want %q", gotType, tc.wantType)
			}
			if gotValue != tc.wantValue {
				t.Errorf("AccountInfo() value = %q, want %q", gotValue, tc.wantValue)
			}
		})
	}
}

func TestAuth_AccountInfo_OAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		auth      *Auth
		wantType  string
		wantValue string
	}{
		{
			name: "gemini-cli with email and project_id",
			auth: &Auth{
				Provider: "gemini-cli",
				Metadata: map[string]any{
					"email":      "user@example.com",
					"project_id": "my-gcp-project",
				},
			},
			wantType:  "oauth",
			wantValue: "user@example.com (my-gcp-project)",
		},
		{
			name: "gemini-cli with email only",
			auth: &Auth{
				Provider: "gemini-cli",
				Metadata: map[string]any{
					"email": "user@example.com",
				},
			},
			wantType:  "oauth",
			wantValue: "user@example.com",
		},
		{
			name: "gemini-cli with empty project_id",
			auth: &Auth{
				Provider: "gemini-cli",
				Metadata: map[string]any{
					"email":      "user@example.com",
					"project_id": "  ",
				},
			},
			wantType:  "oauth",
			wantValue: "user@example.com",
		},
		{
			name: "iflow with email",
			auth: &Auth{
				Provider: "iflow",
				Metadata: map[string]any{
					"email": "flow@example.com",
				},
			},
			wantType:  "oauth",
			wantValue: "flow@example.com",
		},
		{
			name: "generic provider with email in metadata",
			auth: &Auth{
				Provider: "custom",
				Metadata: map[string]any{
					"email": "generic@example.com",
				},
			},
			wantType:  "oauth",
			wantValue: "generic@example.com",
		},
		{
			name: "oauth takes priority over api_key",
			auth: &Auth{
				Provider: "gemini-cli",
				Metadata: map[string]any{
					"email": "priority@example.com",
				},
				Attributes: map[string]string{
					"api_key": "sk-should-not-be-used",
				},
			},
			wantType:  "oauth",
			wantValue: "priority@example.com",
		},
		{
			name: "empty email falls back to api_key",
			auth: &Auth{
				Provider: "gemini",
				Metadata: map[string]any{
					"email": "  ",
				},
				Attributes: map[string]string{
					"api_key": "sk-fallback-key",
				},
			},
			wantType:  "api_key",
			wantValue: "sk-fallback-key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotValue := tc.auth.AccountInfo()

			if gotType != tc.wantType {
				t.Errorf("AccountInfo() type = %q, want %q", gotType, tc.wantType)
			}
			if gotValue != tc.wantValue {
				t.Errorf("AccountInfo() value = %q, want %q", gotValue, tc.wantValue)
			}
		})
	}
}

func TestAuth_ProxyInfo_WithProxyURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		auth *Auth
		want string
	}{
		{
			name: "nil auth",
			auth: nil,
			want: "",
		},
		{
			name: "empty proxy url",
			auth: &Auth{
				ProxyURL: "",
			},
			want: "",
		},
		{
			name: "whitespace proxy url",
			auth: &Auth{
				ProxyURL: "   ",
			},
			want: "",
		},
		{
			name: "http proxy",
			auth: &Auth{
				ProxyURL: "http://proxy.example.com:8080",
			},
			want: "via http proxy",
		},
		{
			name: "https proxy",
			auth: &Auth{
				ProxyURL: "https://secure-proxy.example.com:443",
			},
			want: "via https proxy",
		},
		{
			name: "socks5 proxy",
			auth: &Auth{
				ProxyURL: "socks5://socks-proxy.example.com:1080",
			},
			want: "via socks5 proxy",
		},
		{
			name: "proxy without scheme",
			auth: &Auth{
				ProxyURL: "proxy.example.com:8080",
			},
			want: "via proxy",
		},
		{
			name: "proxy with credentials",
			auth: &Auth{
				ProxyURL: "http://user:pass@proxy.example.com:8080",
			},
			want: "via http proxy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.auth.ProxyInfo()

			if got != tc.want {
				t.Errorf("ProxyInfo() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestQuotaState_NextRecoverAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name           string
		quota          QuotaState
		wantExceeded   bool
		wantRecoverSet bool
		wantInFuture   bool
	}{
		{
			name:           "zero quota state",
			quota:          QuotaState{},
			wantExceeded:   false,
			wantRecoverSet: false,
			wantInFuture:   false,
		},
		{
			name: "exceeded with future recover time",
			quota: QuotaState{
				Exceeded:      true,
				Reason:        "rate limit exceeded",
				NextRecoverAt: future,
				BackoffLevel:  1,
			},
			wantExceeded:   true,
			wantRecoverSet: true,
			wantInFuture:   true,
		},
		{
			name: "exceeded with past recover time",
			quota: QuotaState{
				Exceeded:      true,
				Reason:        "daily quota exceeded",
				NextRecoverAt: past,
				BackoffLevel:  2,
			},
			wantExceeded:   true,
			wantRecoverSet: true,
			wantInFuture:   false,
		},
		{
			name: "not exceeded but recover time set",
			quota: QuotaState{
				Exceeded:      false,
				NextRecoverAt: future,
			},
			wantExceeded:   false,
			wantRecoverSet: true,
			wantInFuture:   true,
		},
		{
			name: "exceeded without recover time",
			quota: QuotaState{
				Exceeded: true,
				Reason:   "unknown quota error",
			},
			wantExceeded:   true,
			wantRecoverSet: false,
			wantInFuture:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.quota.Exceeded != tc.wantExceeded {
				t.Errorf("QuotaState.Exceeded = %v, want %v", tc.quota.Exceeded, tc.wantExceeded)
			}

			recoverSet := !tc.quota.NextRecoverAt.IsZero()
			if recoverSet != tc.wantRecoverSet {
				t.Errorf("QuotaState.NextRecoverAt set = %v, want %v", recoverSet, tc.wantRecoverSet)
			}

			if tc.wantRecoverSet {
				inFuture := tc.quota.NextRecoverAt.After(now)
				if inFuture != tc.wantInFuture {
					t.Errorf("QuotaState.NextRecoverAt in future = %v, want %v", inFuture, tc.wantInFuture)
				}
			}
		})
	}
}

func TestModelState_Unavailable(t *testing.T) {
	t.Parallel()

	now := time.Now()
	future := now.Add(30 * time.Minute)

	tests := []struct {
		name        string
		state       *ModelState
		wantStatus  Status
		wantUnavail bool
		wantRetry   bool
		wantError   bool
	}{
		{
			name:        "nil model state",
			state:       nil,
			wantStatus:  "",
			wantUnavail: false,
			wantRetry:   false,
			wantError:   false,
		},
		{
			name:        "active model state",
			state:       &ModelState{Status: StatusActive},
			wantStatus:  StatusActive,
			wantUnavail: false,
			wantRetry:   false,
			wantError:   false,
		},
		{
			name: "unavailable with retry time",
			state: &ModelState{
				Status:         StatusError,
				StatusMessage:  "temporarily unavailable",
				Unavailable:    true,
				NextRetryAfter: future,
			},
			wantStatus:  StatusError,
			wantUnavail: true,
			wantRetry:   true,
			wantError:   false,
		},
		{
			name: "unavailable with error",
			state: &ModelState{
				Status:      StatusError,
				Unavailable: true,
				LastError: &Error{
					Code:       "model_overloaded",
					Message:    "Model is currently overloaded",
					Retryable:  true,
					HTTPStatus: 503,
				},
			},
			wantStatus:  StatusError,
			wantUnavail: true,
			wantRetry:   false,
			wantError:   true,
		},
		{
			name: "unavailable with quota exceeded",
			state: &ModelState{
				Status:      StatusError,
				Unavailable: true,
				Quota: QuotaState{
					Exceeded:      true,
					Reason:        "tokens per minute exceeded",
					NextRecoverAt: future,
					BackoffLevel:  3,
				},
			},
			wantStatus:  StatusError,
			wantUnavail: true,
			wantRetry:   false,
			wantError:   false,
		},
		{
			name: "full unavailable state",
			state: &ModelState{
				Status:         StatusError,
				StatusMessage:  "rate limited",
				Unavailable:    true,
				NextRetryAfter: future,
				LastError: &Error{
					Code:       "rate_limit",
					Message:    "Too many requests",
					Retryable:  true,
					HTTPStatus: 429,
				},
				Quota: QuotaState{
					Exceeded:      true,
					NextRecoverAt: future,
				},
				UpdatedAt: now,
			},
			wantStatus:  StatusError,
			wantUnavail: true,
			wantRetry:   true,
			wantError:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.state == nil {
				// For nil state, just verify we can call Clone without panic
				clone := tc.state.Clone()
				if clone != nil {
					t.Errorf("Clone() of nil = %v, want nil", clone)
				}
				return
			}

			if tc.state.Status != tc.wantStatus {
				t.Errorf("ModelState.Status = %q, want %q", tc.state.Status, tc.wantStatus)
			}

			if tc.state.Unavailable != tc.wantUnavail {
				t.Errorf("ModelState.Unavailable = %v, want %v", tc.state.Unavailable, tc.wantUnavail)
			}

			hasRetry := !tc.state.NextRetryAfter.IsZero()
			if hasRetry != tc.wantRetry {
				t.Errorf("ModelState.NextRetryAfter set = %v, want %v", hasRetry, tc.wantRetry)
			}

			hasError := tc.state.LastError != nil
			if hasError != tc.wantError {
				t.Errorf("ModelState.LastError set = %v, want %v", hasError, tc.wantError)
			}

			// Verify Clone produces independent copy
			clone := tc.state.Clone()
			if clone == nil {
				t.Fatalf("Clone() = nil, want non-nil")
			}
			if clone == tc.state {
				t.Errorf("Clone() returned same pointer")
			}
			if clone.Status != tc.state.Status {
				t.Errorf("Clone().Status = %q, want %q", clone.Status, tc.state.Status)
			}
			if clone.Unavailable != tc.state.Unavailable {
				t.Errorf("Clone().Unavailable = %v, want %v", clone.Unavailable, tc.state.Unavailable)
			}
			if tc.state.LastError != nil {
				if clone.LastError == nil {
					t.Fatalf("Clone().LastError = nil, want non-nil")
				}
				if clone.LastError == tc.state.LastError {
					t.Errorf("Clone().LastError is same pointer as original")
				}
				if clone.LastError.Code != tc.state.LastError.Code {
					t.Errorf("Clone().LastError.Code = %q, want %q", clone.LastError.Code, tc.state.LastError.Code)
				}
			}
		})
	}
}
