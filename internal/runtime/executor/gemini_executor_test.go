package executor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

func TestGeminiExecutor_Identifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{
			name: "returns gemini identifier",
			want: "gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewGeminiExecutor(nil)
			if got := e.Identifier(); got != tt.want {
				t.Errorf("Identifier() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGeminiExecutor_geminiCreds_APIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		auth       *cliproxyauth.Auth
		wantAPIKey string
		wantBearer string
	}{
		{
			name:       "nil auth returns empty credentials",
			auth:       nil,
			wantAPIKey: "",
			wantBearer: "",
		},
		{
			name: "auth with api_key in attributes",
			auth: &cliproxyauth.Auth{
				ID: "test-1",
				Attributes: map[string]string{
					"api_key": "test-api-key-123",
				},
			},
			wantAPIKey: "test-api-key-123",
			wantBearer: "",
		},
		{
			name: "auth with empty api_key",
			auth: &cliproxyauth.Auth{
				ID: "test-2",
				Attributes: map[string]string{
					"api_key": "",
				},
			},
			wantAPIKey: "",
			wantBearer: "",
		},
		{
			name: "auth with nil attributes",
			auth: &cliproxyauth.Auth{
				ID:         "test-3",
				Attributes: nil,
			},
			wantAPIKey: "",
			wantBearer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAPIKey, gotBearer := geminiCreds(tt.auth)
			if gotAPIKey != tt.wantAPIKey {
				t.Errorf("geminiCreds() apiKey = %v, want %v", gotAPIKey, tt.wantAPIKey)
			}
			if gotBearer != tt.wantBearer {
				t.Errorf("geminiCreds() bearer = %v, want %v", gotBearer, tt.wantBearer)
			}
		})
	}
}

func TestGeminiExecutor_geminiCreds_OAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		auth       *cliproxyauth.Auth
		wantAPIKey string
		wantBearer string
	}{
		{
			name: "auth with access_token in metadata",
			auth: &cliproxyauth.Auth{
				ID: "oauth-1",
				Metadata: map[string]any{
					"access_token": "bearer-token-abc",
				},
			},
			wantAPIKey: "",
			wantBearer: "bearer-token-abc",
		},
		{
			name: "auth with nested token.access_token in metadata",
			auth: &cliproxyauth.Auth{
				ID: "oauth-2",
				Metadata: map[string]any{
					"token": map[string]any{
						"access_token": "nested-bearer-token-xyz",
					},
				},
			},
			wantAPIKey: "",
			wantBearer: "nested-bearer-token-xyz",
		},
		{
			name: "auth with both api_key and access_token",
			auth: &cliproxyauth.Auth{
				ID: "mixed-1",
				Attributes: map[string]string{
					"api_key": "api-key-value",
				},
				Metadata: map[string]any{
					"access_token": "oauth-token-value",
				},
			},
			wantAPIKey: "api-key-value",
			wantBearer: "oauth-token-value",
		},
		{
			name: "auth with nil metadata",
			auth: &cliproxyauth.Auth{
				ID:       "oauth-3",
				Metadata: nil,
			},
			wantAPIKey: "",
			wantBearer: "",
		},
		{
			name: "auth with empty access_token",
			auth: &cliproxyauth.Auth{
				ID: "oauth-4",
				Metadata: map[string]any{
					"access_token": "",
				},
			},
			wantAPIKey: "",
			wantBearer: "",
		},
		{
			name: "nested token with empty access_token",
			auth: &cliproxyauth.Auth{
				ID: "oauth-5",
				Metadata: map[string]any{
					"token": map[string]any{
						"access_token": "",
					},
				},
			},
			wantAPIKey: "",
			wantBearer: "",
		},
		{
			name: "nested token nil map",
			auth: &cliproxyauth.Auth{
				ID: "oauth-6",
				Metadata: map[string]any{
					"token": nil,
				},
			},
			wantAPIKey: "",
			wantBearer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAPIKey, gotBearer := geminiCreds(tt.auth)
			if gotAPIKey != tt.wantAPIKey {
				t.Errorf("geminiCreds() apiKey = %v, want %v", gotAPIKey, tt.wantAPIKey)
			}
			if gotBearer != tt.wantBearer {
				t.Errorf("geminiCreds() bearer = %v, want %v", gotBearer, tt.wantBearer)
			}
		})
	}
}

func TestGeminiExecutor_buildURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		auth    *cliproxyauth.Auth
		wantURL string
	}{
		{
			name:    "nil auth uses default endpoint",
			auth:    nil,
			wantURL: glEndpoint,
		},
		{
			name: "auth with nil attributes uses default",
			auth: &cliproxyauth.Auth{
				ID:         "test-1",
				Attributes: nil,
			},
			wantURL: glEndpoint,
		},
		{
			name: "auth with empty base_url uses default",
			auth: &cliproxyauth.Auth{
				ID: "test-2",
				Attributes: map[string]string{
					"base_url": "",
				},
			},
			wantURL: glEndpoint,
		},
		{
			name: "auth with whitespace base_url uses default",
			auth: &cliproxyauth.Auth{
				ID: "test-3",
				Attributes: map[string]string{
					"base_url": "   ",
				},
			},
			wantURL: glEndpoint,
		},
		{
			name: "auth with custom base_url",
			auth: &cliproxyauth.Auth{
				ID: "test-4",
				Attributes: map[string]string{
					"base_url": "https://custom.googleapis.com",
				},
			},
			wantURL: "https://custom.googleapis.com",
		},
		{
			name: "auth with trailing slash in base_url",
			auth: &cliproxyauth.Auth{
				ID: "test-5",
				Attributes: map[string]string{
					"base_url": "https://custom.googleapis.com/",
				},
			},
			wantURL: "https://custom.googleapis.com",
		},
		{
			name: "auth with multiple trailing slashes",
			auth: &cliproxyauth.Auth{
				ID: "test-6",
				Attributes: map[string]string{
					"base_url": "https://custom.googleapis.com///",
				},
			},
			wantURL: "https://custom.googleapis.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveGeminiBaseURL(tt.auth)
			if got != tt.wantURL {
				t.Errorf("resolveGeminiBaseURL() = %v, want %v", got, tt.wantURL)
			}
		})
	}
}

func TestGeminiExecutor_applyGeminiHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		auth        *cliproxyauth.Auth
		wantHeaders map[string]string
	}{
		{
			name:        "nil auth sets no custom headers",
			auth:        nil,
			wantHeaders: map[string]string{},
		},
		{
			name: "auth with nil attributes sets no custom headers",
			auth: &cliproxyauth.Auth{
				ID:         "test-1",
				Attributes: nil,
			},
			wantHeaders: map[string]string{},
		},
		{
			name: "auth with empty attributes sets no custom headers",
			auth: &cliproxyauth.Auth{
				ID:         "test-2",
				Attributes: map[string]string{},
			},
			wantHeaders: map[string]string{},
		},
		{
			name: "auth with header: prefix attribute sets custom header",
			auth: &cliproxyauth.Auth{
				ID: "test-3",
				Attributes: map[string]string{
					"header:X-Custom-Header": "custom-value",
				},
			},
			wantHeaders: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		},
		{
			name: "auth with multiple header: prefix attributes",
			auth: &cliproxyauth.Auth{
				ID: "test-4",
				Attributes: map[string]string{
					"header:X-First":  "value-1",
					"header:X-Second": "value-2",
				},
			},
			wantHeaders: map[string]string{
				"X-First":  "value-1",
				"X-Second": "value-2",
			},
		},
		{
			name: "auth with mixed header and non-header attributes",
			auth: &cliproxyauth.Auth{
				ID: "test-5",
				Attributes: map[string]string{
					"api_key":                "some-key",
					"header:X-Custom-Header": "custom-value",
					"base_url":               "https://example.com",
				},
			},
			wantHeaders: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "https://example.com/test", nil)
			applyGeminiHeaders(req, tt.auth)

			for key, wantValue := range tt.wantHeaders {
				gotValue := req.Header.Get(key)
				if gotValue != wantValue {
					t.Errorf("Header %q = %v, want %v", key, gotValue, wantValue)
				}
			}
		})
	}
}

func TestGeminiExecutor_Execute_APIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		auth           *cliproxyauth.Auth
		request        cliproxyexecutor.Request
		opts           cliproxyexecutor.Options
		wantErr        bool
		wantErrStatus  int
		checkResponse  func(t *testing.T, resp cliproxyexecutor.Response)
	}{
		{
			name: "successful API key request",
			serverResponse: `{
				"candidates": [{
					"content": {
						"parts": [{"text": "Hello from Gemini!"}],
						"role": "model"
					},
					"finishReason": "STOP"
				}],
				"usageMetadata": {
					"promptTokenCount": 10,
					"candidatesTokenCount": 5,
					"totalTokenCount": 15
				}
			}`,
			serverStatus: http.StatusOK,
			auth: &cliproxyauth.Auth{
				ID:       "api-key-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "test-api-key",
				},
			},
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Hello"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				SourceFormat:    sdktranslator.FormatGemini,
				OriginalRequest: []byte(`{"contents":[{"parts":[{"text":"Hello"}],"role":"user"}]}`),
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp cliproxyexecutor.Response) {
				if len(resp.Payload) == 0 {
					t.Error("Expected non-empty response payload")
				}
			},
		},
		{
			name:           "server returns 401 unauthorized",
			serverResponse: `{"error": {"code": 401, "message": "API key not valid"}}`,
			serverStatus:   http.StatusUnauthorized,
			auth: &cliproxyauth.Auth{
				ID:       "invalid-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "invalid-api-key",
				},
			},
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Hello"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				SourceFormat: sdktranslator.FormatGemini,
			},
			wantErr:       true,
			wantErrStatus: http.StatusUnauthorized,
		},
		{
			name:           "server returns 429 rate limited",
			serverResponse: `{"error": {"code": 429, "message": "Rate limit exceeded"}}`,
			serverStatus:   http.StatusTooManyRequests,
			auth: &cliproxyauth.Auth{
				ID:       "rate-limited-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "rate-limited-key",
				},
			},
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Hello"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				SourceFormat: sdktranslator.FormatGemini,
			},
			wantErr:       true,
			wantErrStatus: http.StatusTooManyRequests,
		},
		{
			name:           "server returns 500 internal error",
			serverResponse: `{"error": {"code": 500, "message": "Internal server error"}}`,
			serverStatus:   http.StatusInternalServerError,
			auth: &cliproxyauth.Auth{
				ID:       "server-error-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "test-key",
				},
			},
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Hello"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				SourceFormat: sdktranslator.FormatGemini,
			},
			wantErr:       true,
			wantErrStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Verify Content-Type header
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}

				// Verify API key header if present
				if tt.auth != nil && tt.auth.Attributes != nil {
					if apiKey := tt.auth.Attributes["api_key"]; apiKey != "" {
						gotAPIKey := r.Header.Get("x-goog-api-key")
						if gotAPIKey != apiKey {
							t.Errorf("Expected x-goog-api-key %s, got %s", apiKey, gotAPIKey)
						}
					}
				}

				// Verify URL contains generateContent action
				if !strings.Contains(r.URL.Path, "generateContent") {
					t.Errorf("Expected URL to contain generateContent, got %s", r.URL.Path)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.serverStatus)
				_, _ = w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			// Update auth to use test server URL
			if tt.auth != nil {
				if tt.auth.Attributes == nil {
					tt.auth.Attributes = make(map[string]string)
				}
				tt.auth.Attributes["base_url"] = server.URL
			}

			cfg := &config.Config{}
			executor := NewGeminiExecutor(cfg)

			ctx := context.Background()
			resp, err := executor.Execute(ctx, tt.auth, tt.request, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Execute() expected error, got nil")
					return
				}
				if statusErr, ok := err.(cliproxyexecutor.StatusError); ok {
					if statusErr.StatusCode() != tt.wantErrStatus {
						t.Errorf("Execute() error status = %v, want %v", statusErr.StatusCode(), tt.wantErrStatus)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
				return
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestGeminiExecutor_ExecuteStream(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverChunks   []string
		serverStatus   int
		auth           *cliproxyauth.Auth
		request        cliproxyexecutor.Request
		opts           cliproxyexecutor.Options
		wantErr        bool
		wantErrStatus  int
		wantChunkCount int
	}{
		{
			name: "successful streaming request",
			serverChunks: []string{
				`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}`,
				`data: {"candidates":[{"content":{"parts":[{"text":" World"}],"role":"model"}}]}`,
				`data: {"candidates":[{"content":{"parts":[{"text":"!"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`,
			},
			serverStatus: http.StatusOK,
			auth: &cliproxyauth.Auth{
				ID:       "stream-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "stream-api-key",
				},
			},
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Say hello"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				Stream:          true,
				SourceFormat:    sdktranslator.FormatGemini,
				OriginalRequest: []byte(`{"contents":[{"parts":[{"text":"Say hello"}],"role":"user"}]}`),
			},
			wantErr:        false,
			wantChunkCount: 3,
		},
		{
			name:         "streaming request with 401 error",
			serverChunks: []string{},
			serverStatus: http.StatusUnauthorized,
			auth: &cliproxyauth.Auth{
				ID:       "invalid-stream-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "invalid-stream-key",
				},
			},
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Say hello"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				Stream:       true,
				SourceFormat: sdktranslator.FormatGemini,
			},
			wantErr:       true,
			wantErrStatus: http.StatusUnauthorized,
		},
		{
			name: "streaming with OAuth bearer token",
			serverChunks: []string{
				`data: {"candidates":[{"content":{"parts":[{"text":"OAuth response"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`,
			},
			serverStatus: http.StatusOK,
			auth: &cliproxyauth.Auth{
				ID:       "oauth-stream-auth",
				Provider: "gemini",
				Metadata: map[string]any{
					"access_token": "oauth-bearer-token",
				},
			},
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"OAuth test"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				Stream:          true,
				SourceFormat:    sdktranslator.FormatGemini,
				OriginalRequest: []byte(`{"contents":[{"parts":[{"text":"OAuth test"}],"role":"user"}]}`),
			},
			wantErr:        false,
			wantChunkCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Verify Content-Type header
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}

				// Verify auth header
				if tt.auth != nil {
					if tt.auth.Attributes != nil && tt.auth.Attributes["api_key"] != "" {
						gotAPIKey := r.Header.Get("x-goog-api-key")
						if gotAPIKey != tt.auth.Attributes["api_key"] {
							t.Errorf("Expected x-goog-api-key %s, got %s", tt.auth.Attributes["api_key"], gotAPIKey)
						}
					} else if tt.auth.Metadata != nil {
						if accessToken, ok := tt.auth.Metadata["access_token"].(string); ok && accessToken != "" {
							authHeader := r.Header.Get("Authorization")
							expectedAuth := "Bearer " + accessToken
							if authHeader != expectedAuth {
								t.Errorf("Expected Authorization %s, got %s", expectedAuth, authHeader)
							}
						}
					}
				}

				// Verify URL contains streamGenerateContent
				if !strings.Contains(r.URL.Path, "streamGenerateContent") {
					t.Errorf("Expected URL to contain streamGenerateContent, got %s", r.URL.Path)
				}

				// Handle error status codes
				if tt.serverStatus != http.StatusOK {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.serverStatus)
					_, _ = w.Write([]byte(`{"error": {"code": ` + http.StatusText(tt.serverStatus) + `}}`))
					return
				}

				// Set headers for SSE
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)

				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Error("Expected ResponseWriter to be a Flusher")
					return
				}

				for _, chunk := range tt.serverChunks {
					_, _ = w.Write([]byte(chunk + "\n"))
					flusher.Flush()
				}
			}))
			defer server.Close()

			// Update auth to use test server URL
			if tt.auth != nil {
				if tt.auth.Attributes == nil {
					tt.auth.Attributes = make(map[string]string)
				}
				tt.auth.Attributes["base_url"] = server.URL
			}

			cfg := &config.Config{}
			executor := NewGeminiExecutor(cfg)

			ctx := context.Background()
			stream, err := executor.ExecuteStream(ctx, tt.auth, tt.request, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExecuteStream() expected error, got nil")
					return
				}
				if statusErr, ok := err.(cliproxyexecutor.StatusError); ok {
					if statusErr.StatusCode() != tt.wantErrStatus {
						t.Errorf("ExecuteStream() error status = %v, want %v", statusErr.StatusCode(), tt.wantErrStatus)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteStream() unexpected error: %v", err)
				return
			}

			if stream == nil {
				t.Error("ExecuteStream() returned nil stream")
				return
			}

			// Consume stream chunks
			chunkCount := 0
			for chunk := range stream.Chunks {
				if chunk.Err != nil {
					t.Errorf("Received error chunk: %v", chunk.Err)
					continue
				}
				chunkCount++
			}

			// Note: The actual chunk count may differ from server chunks due to translation
			// We just verify we received some chunks for successful requests
			if chunkCount == 0 && tt.wantChunkCount > 0 {
				t.Errorf("Expected to receive chunks, got 0")
			}
		})
	}
}

func TestGeminiExecutor_Execute_RequestBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		request   cliproxyexecutor.Request
		opts      cliproxyexecutor.Options
		checkBody func(t *testing.T, body []byte)
	}{
		{
			name: "request body contains model field",
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Test"}],"role":"user"}]}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				SourceFormat:    sdktranslator.FormatGemini,
				OriginalRequest: []byte(`{"contents":[{"parts":[{"text":"Test"}],"role":"user"}]}`),
			},
			checkBody: func(t *testing.T, body []byte) {
				var parsed map[string]any
				if err := json.Unmarshal(body, &parsed); err != nil {
					t.Errorf("Failed to parse request body: %v", err)
					return
				}
				if model, ok := parsed["model"].(string); !ok || model == "" {
					t.Error("Expected model field in request body")
				}
			},
		},
		{
			name: "session_id is stripped from request",
			request: cliproxyexecutor.Request{
				Model:   "gemini-2.0-flash",
				Payload: []byte(`{"contents":[{"parts":[{"text":"Test"}],"role":"user"}],"session_id":"test-session"}`),
				Format:  sdktranslator.FormatGemini,
			},
			opts: cliproxyexecutor.Options{
				SourceFormat:    sdktranslator.FormatGemini,
				OriginalRequest: []byte(`{"contents":[{"parts":[{"text":"Test"}],"role":"user"}],"session_id":"test-session"}`),
			},
			checkBody: func(t *testing.T, body []byte) {
				var parsed map[string]any
				if err := json.Unmarshal(body, &parsed); err != nil {
					t.Errorf("Failed to parse request body: %v", err)
					return
				}
				if _, ok := parsed["session_id"]; ok {
					t.Error("session_id should be stripped from request body")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody []byte

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				capturedBody, err = io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("Failed to read request body: %v", err)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"OK"}],"role":"model"},"finishReason":"STOP"}]}`))
			}))
			defer server.Close()

			auth := &cliproxyauth.Auth{
				ID:       "test-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key":  "test-key",
					"base_url": server.URL,
				},
			}

			cfg := &config.Config{}
			executor := NewGeminiExecutor(cfg)

			ctx := context.Background()
			_, err := executor.Execute(ctx, auth, tt.request, tt.opts)
			if err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
				return
			}

			if tt.checkBody != nil {
				tt.checkBody(t, capturedBody)
			}
		})
	}
}

func TestGeminiExecutor_PrepareRequest(t *testing.T) {
	t.Parallel()

	executor := NewGeminiExecutor(nil)
	req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)

	err := executor.PrepareRequest(req, nil)
	if err != nil {
		t.Errorf("PrepareRequest() unexpected error: %v", err)
	}
}

func TestGeminiExecutor_Refresh(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		auth *cliproxyauth.Auth
	}{
		{
			name: "refresh returns same auth",
			auth: &cliproxyauth.Auth{
				ID:       "test-auth",
				Provider: "gemini",
				Attributes: map[string]string{
					"api_key": "test-key",
				},
			},
		},
		{
			name: "refresh with nil auth",
			auth: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewGeminiExecutor(nil)

			ctx := context.Background()
			refreshed, err := executor.Refresh(ctx, tt.auth)

			if err != nil {
				t.Errorf("Refresh() unexpected error: %v", err)
				return
			}

			if refreshed != tt.auth {
				t.Errorf("Refresh() returned different auth, want same instance")
			}
		})
	}
}
