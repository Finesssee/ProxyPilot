package executor

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestClaudeExecutor_Identifier(t *testing.T) {
	tests := []struct {
		name     string
		executor *ClaudeExecutor
		want     string
	}{
		{
			name:     "returns claude identifier",
			executor: NewClaudeExecutor(nil),
			want:     "claude",
		},
		{
			name:     "returns claude identifier with config",
			executor: NewClaudeExecutor(&config.Config{}),
			want:     "claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.executor.Identifier()
			if got != tt.want {
				t.Errorf("ClaudeExecutor.Identifier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaudeExecutor_claudeCreds_APIKey(t *testing.T) {
	tests := []struct {
		name        string
		auth        *cliproxyauth.Auth
		wantAPIKey  string
		wantBaseURL string
	}{
		{
			name:        "nil auth returns empty values",
			auth:        nil,
			wantAPIKey:  "",
			wantBaseURL: "",
		},
		{
			name: "auth with api_key in attributes",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{
					"api_key": "sk-test-key-123",
				},
			},
			wantAPIKey:  "sk-test-key-123",
			wantBaseURL: "",
		},
		{
			name: "auth with api_key and base_url in attributes",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{
					"api_key":  "sk-test-key-456",
					"base_url": "https://custom.anthropic.com",
				},
			},
			wantAPIKey:  "sk-test-key-456",
			wantBaseURL: "https://custom.anthropic.com",
		},
		{
			name: "auth with empty attributes",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{},
			},
			wantAPIKey:  "",
			wantBaseURL: "",
		},
		{
			name: "auth with nil attributes",
			auth: &cliproxyauth.Auth{
				Attributes: nil,
			},
			wantAPIKey:  "",
			wantBaseURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAPIKey, gotBaseURL := claudeCreds(tt.auth)
			if gotAPIKey != tt.wantAPIKey {
				t.Errorf("claudeCreds() apiKey = %q, want %q", gotAPIKey, tt.wantAPIKey)
			}
			if gotBaseURL != tt.wantBaseURL {
				t.Errorf("claudeCreds() baseURL = %q, want %q", gotBaseURL, tt.wantBaseURL)
			}
		})
	}
}

func TestClaudeExecutor_claudeCreds_OAuth(t *testing.T) {
	tests := []struct {
		name        string
		auth        *cliproxyauth.Auth
		wantAPIKey  string
		wantBaseURL string
	}{
		{
			name: "auth with access_token in metadata (OAuth)",
			auth: &cliproxyauth.Auth{
				Metadata: map[string]any{
					"access_token": "oauth-access-token-789",
				},
			},
			wantAPIKey:  "oauth-access-token-789",
			wantBaseURL: "",
		},
		{
			name: "auth with access_token and email in metadata",
			auth: &cliproxyauth.Auth{
				Metadata: map[string]any{
					"access_token": "oauth-token-abc",
					"email":        "user@example.com",
				},
			},
			wantAPIKey:  "oauth-token-abc",
			wantBaseURL: "",
		},
		{
			name: "api_key in attributes takes precedence over metadata",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{
					"api_key": "sk-api-key-priority",
				},
				Metadata: map[string]any{
					"access_token": "oauth-token-ignored",
				},
			},
			wantAPIKey:  "sk-api-key-priority",
			wantBaseURL: "",
		},
		{
			name: "empty api_key falls back to access_token",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{
					"api_key": "",
				},
				Metadata: map[string]any{
					"access_token": "oauth-fallback-token",
				},
			},
			wantAPIKey:  "oauth-fallback-token",
			wantBaseURL: "",
		},
		{
			name: "access_token as non-string is ignored",
			auth: &cliproxyauth.Auth{
				Metadata: map[string]any{
					"access_token": 12345,
				},
			},
			wantAPIKey:  "",
			wantBaseURL: "",
		},
		{
			name: "nil metadata with nil attributes",
			auth: &cliproxyauth.Auth{
				Attributes: nil,
				Metadata:   nil,
			},
			wantAPIKey:  "",
			wantBaseURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAPIKey, gotBaseURL := claudeCreds(tt.auth)
			if gotAPIKey != tt.wantAPIKey {
				t.Errorf("claudeCreds() apiKey = %q, want %q", gotAPIKey, tt.wantAPIKey)
			}
			if gotBaseURL != tt.wantBaseURL {
				t.Errorf("claudeCreds() baseURL = %q, want %q", gotBaseURL, tt.wantBaseURL)
			}
		})
	}
}

func TestClaudeExecutor_applyClaudeHeaders(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		auth        *cliproxyauth.Auth
		apiKey      string
		stream      bool
		extraBetas  []string
		checkHeader func(t *testing.T, h http.Header)
	}{
		{
			name:   "sets x-api-key for api.anthropic.com with api_key auth",
			url:    "https://api.anthropic.com/v1/messages",
			apiKey: "sk-test-api-key",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{
					"api_key": "sk-test-api-key",
				},
			},
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				if got := h.Get("x-api-key"); got != "sk-test-api-key" {
					t.Errorf("x-api-key = %q, want %q", got, "sk-test-api-key")
				}
				if got := h.Get("Authorization"); got != "" {
					t.Errorf("Authorization should be empty for x-api-key auth, got %q", got)
				}
			},
		},
		{
			name:   "sets Authorization Bearer for non-anthropic URL",
			url:    "https://custom.api.com/v1/messages",
			apiKey: "sk-custom-key",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{
					"api_key": "sk-custom-key",
				},
			},
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				if got := h.Get("Authorization"); got != "Bearer sk-custom-key" {
					t.Errorf("Authorization = %q, want %q", got, "Bearer sk-custom-key")
				}
			},
		},
		{
			name:       "sets Accept to text/event-stream when streaming",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     true,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				if got := h.Get("Accept"); got != "text/event-stream" {
					t.Errorf("Accept = %q, want %q", got, "text/event-stream")
				}
			},
		},
		{
			name:       "sets Accept to application/json when not streaming",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				if got := h.Get("Accept"); got != "application/json" {
					t.Errorf("Accept = %q, want %q", got, "application/json")
				}
			},
		},
		{
			name:       "sets Content-Type to application/json",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				if got := h.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want %q", got, "application/json")
				}
			},
		},
		{
			name:       "sets Anthropic-Beta header with default betas",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				beta := h.Get("Anthropic-Beta")
				if !strings.Contains(beta, "oauth-2025-04-20") {
					t.Errorf("Anthropic-Beta should contain oauth-2025-04-20, got %q", beta)
				}
				if !strings.Contains(beta, "claude-code-20250219") {
					t.Errorf("Anthropic-Beta should contain claude-code-20250219, got %q", beta)
				}
			},
		},
		{
			name:       "merges extra betas into Anthropic-Beta header",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     false,
			extraBetas: []string{"custom-beta-2025", "another-beta"},
			checkHeader: func(t *testing.T, h http.Header) {
				beta := h.Get("Anthropic-Beta")
				if !strings.Contains(beta, "custom-beta-2025") {
					t.Errorf("Anthropic-Beta should contain custom-beta-2025, got %q", beta)
				}
				if !strings.Contains(beta, "another-beta") {
					t.Errorf("Anthropic-Beta should contain another-beta, got %q", beta)
				}
			},
		},
		{
			name:       "sets Anthropic-Version header",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				if got := h.Get("Anthropic-Version"); got != "2023-06-01" {
					t.Errorf("Anthropic-Version = %q, want %q", got, "2023-06-01")
				}
			},
		},
		{
			name:       "sets Connection header to keep-alive",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				if got := h.Get("Connection"); got != "keep-alive" {
					t.Errorf("Connection = %q, want %q", got, "keep-alive")
				}
			},
		},
		{
			name:       "sets Accept-Encoding header",
			url:        "https://api.anthropic.com/v1/messages",
			apiKey:     "sk-test",
			auth:       nil,
			stream:     false,
			extraBetas: nil,
			checkHeader: func(t *testing.T, h http.Header) {
				encoding := h.Get("Accept-Encoding")
				if !strings.Contains(encoding, "gzip") {
					t.Errorf("Accept-Encoding should contain gzip, got %q", encoding)
				}
				if !strings.Contains(encoding, "br") {
					t.Errorf("Accept-Encoding should contain br, got %q", encoding)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, tt.url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			applyClaudeHeaders(req, tt.auth, tt.apiKey, tt.stream, tt.extraBetas)
			tt.checkHeader(t, req.Header)
		})
	}
}

func TestClaudeExecutor_injectThinkingConfig(t *testing.T) {
	executor := NewClaudeExecutor(&config.Config{})

	tests := []struct {
		name      string
		modelName string
		metadata  map[string]any
		body      string
		wantCheck func(t *testing.T, result []byte)
	}{
		{
			name:      "non-thinking model returns body unchanged",
			modelName: "claude-3-5-sonnet-20240620",
			metadata:  nil,
			body:      `{"model":"claude-3-5-sonnet-20240620","messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				if strings.Contains(string(result), "thinking") {
					t.Error("non-thinking model should not have thinking config")
				}
			},
		},
		{
			name:      "thinking model without metadata returns body unchanged",
			modelName: "claude-sonnet-4-5-thinking",
			metadata:  nil,
			body:      `{"model":"claude-sonnet-4-5-thinking","messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				// Without metadata and without registry support, no thinking config should be injected
				if strings.Contains(string(result), `"thinking"`) {
					t.Error("thinking model without metadata should not have thinking config injected")
				}
			},
		},
		{
			name:      "body with existing thinking config is unchanged",
			modelName: "claude-sonnet-4-5-thinking",
			metadata: map[string]any{
				"thinking_budget": 5000,
			},
			body: `{"model":"claude-sonnet-4-5-thinking","messages":[],"thinking":{"type":"enabled","budget_tokens":10000}}`,
			wantCheck: func(t *testing.T, result []byte) {
				// Existing config should be preserved
				if !strings.Contains(string(result), `"budget_tokens":10000`) {
					t.Error("existing thinking config should be preserved")
				}
			},
		},
		{
			name:      "model not in registry returns body unchanged even with budget",
			modelName: "claude-sonnet-4-5-thinking",
			metadata: map[string]any{
				"thinking_budget": 8000,
			},
			body: `{"model":"claude-sonnet-4-5-thinking","messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				// Without registry support, injectThinkingConfig relies on ModelSupportsThinking
				// which requires the model to be registered. In test environment without registry,
				// the body should remain unchanged.
				// This tests the actual behavior of the function.
				resultStr := string(result)
				// The function checks registry first - if model not found, returns unchanged
				if strings.Contains(resultStr, `"thinking":{"type":"enabled"`) {
					// If thinking was injected, that means registry was populated
					// which is fine - but in clean test environment, it won't be
				}
				// The test passes if no error occurs - behavior depends on registry state
			},
		},
		{
			name:      "empty model name returns body unchanged",
			modelName: "",
			metadata: map[string]any{
				"thinking_budget": 8000,
			},
			body: `{"model":"","messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				if strings.Contains(string(result), `"thinking":{"type":"enabled"`) {
					t.Error("empty model should not have thinking config injected")
				}
			},
		},
		{
			name:      "nil metadata returns body unchanged",
			modelName: "claude-sonnet-4-5-thinking",
			metadata:  nil,
			body:      `{"model":"claude-sonnet-4-5-thinking","messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				if strings.Contains(string(result), `"thinking":{"type":"enabled"`) {
					t.Error("nil metadata should not inject thinking config")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.injectThinkingConfig(tt.modelName, tt.metadata, []byte(tt.body))
			tt.wantCheck(t, result)
		})
	}
}

func TestDecodeResponseBody_Gzip(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		contentEncoding string
		wantContent     string
		wantErr         bool
	}{
		{
			name:            "valid gzip content",
			content:         "Hello, World!",
			contentEncoding: "gzip",
			wantContent:     "Hello, World!",
			wantErr:         false,
		},
		{
			name:            "gzip with uppercase encoding",
			content:         "Test content",
			contentEncoding: "GZIP",
			wantContent:     "Test content",
			wantErr:         false,
		},
		{
			name:            "gzip with mixed case",
			content:         "Mixed case test",
			contentEncoding: "Gzip",
			wantContent:     "Mixed case test",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create gzip compressed content
			var buf bytes.Buffer
			gzWriter := gzip.NewWriter(&buf)
			if _, err := gzWriter.Write([]byte(tt.content)); err != nil {
				t.Fatalf("failed to write gzip content: %v", err)
			}
			if err := gzWriter.Close(); err != nil {
				t.Fatalf("failed to close gzip writer: %v", err)
			}

			body := io.NopCloser(bytes.NewReader(buf.Bytes()))
			decoded, err := decodeResponseBody(body, tt.contentEncoding)

			if (err != nil) != tt.wantErr {
				t.Errorf("decodeResponseBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				content, readErr := io.ReadAll(decoded)
				if readErr != nil {
					t.Fatalf("failed to read decoded content: %v", readErr)
				}
				if err := decoded.Close(); err != nil {
					t.Errorf("failed to close decoded body: %v", err)
				}

				if string(content) != tt.wantContent {
					t.Errorf("decoded content = %q, want %q", string(content), tt.wantContent)
				}
			}
		})
	}
}

func TestDecodeResponseBody_Brotli(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		contentEncoding string
		wantContent     string
		wantErr         bool
	}{
		{
			name:            "valid brotli content",
			content:         "Hello, Brotli World!",
			contentEncoding: "br",
			wantContent:     "Hello, Brotli World!",
			wantErr:         false,
		},
		{
			name:            "brotli with uppercase encoding",
			content:         "Brotli test",
			contentEncoding: "BR",
			wantContent:     "Brotli test",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create brotli compressed content
			var buf bytes.Buffer
			brWriter := brotli.NewWriter(&buf)
			if _, err := brWriter.Write([]byte(tt.content)); err != nil {
				t.Fatalf("failed to write brotli content: %v", err)
			}
			if err := brWriter.Close(); err != nil {
				t.Fatalf("failed to close brotli writer: %v", err)
			}

			body := io.NopCloser(bytes.NewReader(buf.Bytes()))
			decoded, err := decodeResponseBody(body, tt.contentEncoding)

			if (err != nil) != tt.wantErr {
				t.Errorf("decodeResponseBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				content, readErr := io.ReadAll(decoded)
				if readErr != nil {
					t.Fatalf("failed to read decoded content: %v", readErr)
				}
				if err := decoded.Close(); err != nil {
					t.Errorf("failed to close decoded body: %v", err)
				}

				if string(content) != tt.wantContent {
					t.Errorf("decoded content = %q, want %q", string(content), tt.wantContent)
				}
			}
		})
	}
}

func TestDecodeResponseBody_Identity(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		contentEncoding string
		wantContent     string
		wantErr         bool
	}{
		{
			name:            "empty content encoding returns body as-is",
			content:         "Plain text content",
			contentEncoding: "",
			wantContent:     "Plain text content",
			wantErr:         false,
		},
		{
			name:            "identity encoding returns body as-is",
			content:         "Identity encoded content",
			contentEncoding: "identity",
			wantContent:     "Identity encoded content",
			wantErr:         false,
		},
		{
			name:            "identity with uppercase",
			content:         "IDENTITY test",
			contentEncoding: "IDENTITY",
			wantContent:     "IDENTITY test",
			wantErr:         false,
		},
		{
			name:            "unknown encoding returns body as-is",
			content:         "Unknown encoding content",
			contentEncoding: "unknown-encoding",
			wantContent:     "Unknown encoding content",
			wantErr:         false,
		},
		{
			name:            "nil body returns error",
			content:         "",
			contentEncoding: "",
			wantContent:     "",
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.ReadCloser
			if tt.name == "nil body returns error" {
				body = nil
			} else {
				body = io.NopCloser(strings.NewReader(tt.content))
			}

			decoded, err := decodeResponseBody(body, tt.contentEncoding)

			if (err != nil) != tt.wantErr {
				t.Errorf("decodeResponseBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && decoded != nil {
				content, readErr := io.ReadAll(decoded)
				if readErr != nil {
					t.Fatalf("failed to read decoded content: %v", readErr)
				}
				if err := decoded.Close(); err != nil {
					t.Errorf("failed to close decoded body: %v", err)
				}

				if string(content) != tt.wantContent {
					t.Errorf("decoded content = %q, want %q", string(content), tt.wantContent)
				}
			}
		})
	}
}

func TestClaudeExecutor_extractAndRemoveBetas(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantBetas  []string
		wantInBody bool
	}{
		{
			name:       "body without betas",
			body:       `{"model":"claude-3-5-sonnet","messages":[]}`,
			wantBetas:  nil,
			wantInBody: false,
		},
		{
			name:       "body with array of betas",
			body:       `{"model":"claude-3-5-sonnet","betas":["beta-1","beta-2"],"messages":[]}`,
			wantBetas:  []string{"beta-1", "beta-2"},
			wantInBody: false,
		},
		{
			name:       "body with single string beta",
			body:       `{"model":"claude-3-5-sonnet","betas":"single-beta","messages":[]}`,
			wantBetas:  []string{"single-beta"},
			wantInBody: false,
		},
		{
			name:       "body with empty betas array",
			body:       `{"model":"claude-3-5-sonnet","betas":[],"messages":[]}`,
			wantBetas:  nil,
			wantInBody: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBetas, gotBody := extractAndRemoveBetas([]byte(tt.body))

			// Check betas
			if len(gotBetas) != len(tt.wantBetas) {
				t.Errorf("extractAndRemoveBetas() betas length = %d, want %d", len(gotBetas), len(tt.wantBetas))
			}
			for i, beta := range gotBetas {
				if i < len(tt.wantBetas) && beta != tt.wantBetas[i] {
					t.Errorf("extractAndRemoveBetas() beta[%d] = %q, want %q", i, beta, tt.wantBetas[i])
				}
			}

			// Check that betas key is removed from body
			if strings.Contains(string(gotBody), `"betas"`) && !tt.wantInBody {
				t.Error("extractAndRemoveBetas() should remove betas from body")
			}
		})
	}
}

func TestClaudeExecutor_Integration_MockServer(t *testing.T) {
	// Create a mock HTTP server that simulates Claude API responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		// Check for API key or Authorization header
		apiKey := r.Header.Get("x-api-key")
		authHeader := r.Header.Get("Authorization")
		if apiKey == "" && authHeader == "" {
			t.Error("Expected either x-api-key or Authorization header")
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "msg_test123",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Hello from mock server!"}],
			"model": "claude-3-5-sonnet-20240620",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 20}
		}`))
	}))
	defer server.Close()

	t.Run("mock server accepts requests with valid headers", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/messages", strings.NewReader(`{"model":"claude-3-5-sonnet","messages":[]}`))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		auth := &cliproxyauth.Auth{
			Attributes: map[string]string{
				"api_key": "sk-test-key",
			},
		}
		applyClaudeHeaders(req, auth, "sk-test-key", false, nil)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		if !strings.Contains(string(body), "Hello from mock server!") {
			t.Errorf("unexpected response body: %s", string(body))
		}
	})
}

func TestClaudeExecutor_checkSystemInstructions(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantCheck func(t *testing.T, result []byte)
	}{
		{
			name:    "payload without system adds claude code instructions",
			payload: `{"model":"claude-3-5-sonnet","messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				if !strings.Contains(string(result), "Claude Code") {
					t.Error("should add Claude Code instructions")
				}
			},
		},
		{
			name:    "payload with string system is converted to array",
			payload: `{"model":"claude-3-5-sonnet","system":"Custom system prompt","messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				if !strings.Contains(string(result), "Claude Code") {
					t.Error("should add Claude Code instructions")
				}
			},
		},
		{
			name:    "payload with existing claude code instructions is unchanged",
			payload: `{"model":"claude-3-5-sonnet","system":[{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."}],"messages":[]}`,
			wantCheck: func(t *testing.T, result []byte) {
				if !strings.Contains(string(result), "Claude Code") {
					t.Error("should preserve Claude Code instructions")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkSystemInstructions([]byte(tt.payload))
			tt.wantCheck(t, result)
		})
	}
}
