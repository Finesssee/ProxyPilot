package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAgenticHarnessMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootDir := t.TempDir()

	tests := []struct {
		name           string
		userAgent      string
		body           string
		expectMode     string // "INITIALIZER", "CODING", or "" (passive)
		expectInjected bool
	}{
		{
			name:           "Non-Agentic Client",
			userAgent:      "curl/7.64.1",
			body:           `{"messages": [{"role": "user", "content": "hi"}]}`,
			expectMode:     "",
			expectInjected: false,
		},
		{
			name:           "Agentic Client - Fresh Conversation (Initializer)",
			userAgent:      "claude-cli/0.0.1",
			body:           `{"messages": [{"role": "system", "content": "sys"}, {"role": "user", "content": "create todo app"}]}`,
			expectMode:     "INITIALIZER", // Only 2 messages
			expectInjected: true,
		},
		{
			name:           "Agentic Client - Short Conversation but Manual Override (Initializer)",
			userAgent:      "codex-cli",
			body:           `{"messages": [{"role": "user", "content": "setup environment"}]}`,
			expectMode:     "INITIALIZER",
			expectInjected: true,
		},
		{
			name:           "Agentic Client - With Progress File (Coding)",
			userAgent:      "droid-cli",
			body:           `{"messages": [{"role": "user", "content": "I checked claude-progress.txt and..."}]}`,
			expectMode:     "CODING",
			expectInjected: true,
		},
		{
			name:           "Agentic Client - With Feature List (Coding)",
			userAgent:      "claude-cli",
			body:           `{"messages": [{"role": "user", "content": "feature_list.json shows 2 features"}]}`,
			expectMode:     "CODING",
			expectInjected: true,
		},
		{
			name:           "Agentic Client - Long Conversation/Passive",
			userAgent:      "claude-cli",
			body:           `{"messages": [{"role":"u","content":"1"},{"role":"a","content":"2"},{"role":"u","content":"3"},{"role":"a","content":"4"},{"role":"u","content":"5"}]}`, // 5 messages
			expectMode:     "",                                                                                                                                                       // Default passive because length >= 5 and no files mentioned
			expectInjected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(tt.body))
			req.Header.Set("User-Agent", tt.userAgent)
			req.Header.Set("Content-Type", "application/json")
			c.Request = req

			// Chain: Harness -> Verification Handler
			handler := AgenticHarnessMiddlewareWithRootDir(rootDir)

			// Execute middleware
			handler(c)

			// Check Header
			actualMode := c.Writer.Header().Get("X-ProxyPilot-Harness-Mode")
			assert.Equal(t, tt.expectMode, actualMode)

			// Check Body Injection
			// Since handler replaces request body, we read it back.
			newBodyBytes, _ := io.ReadAll(c.Request.Body)
			newBody := string(newBodyBytes)

			if tt.expectInjected {
				if tt.expectMode == "INITIALIZER" {
					assert.Contains(t, newBody, "Initializer Agent")
					assert.Contains(t, newBody, "feature_list.json")
				}
				if tt.expectMode == "CODING" {
					assert.Contains(t, newBody, "Coding Agent")
					assert.Contains(t, newBody, "incremental progress")
				}
			} else {
				assert.NotContains(t, newBody, "Initializer Agent")
				assert.NotContains(t, newBody, "Coding Agent")
			}
		})
	}
}

func TestAgenticHarnessMiddleware_FilePresenceForcesCoding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(rootDir, "feature_list.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatalf("write feature_list.json: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"messages": [{"role": "user", "content": "setup environment"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("User-Agent", "claude-cli")
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler := AgenticHarnessMiddlewareWithRootDir(rootDir)
	handler(c)

	assert.Equal(t, "CODING", c.Writer.Header().Get("X-ProxyPilot-Harness-Mode"))
	newBodyBytes, _ := io.ReadAll(c.Request.Body)
	assert.Contains(t, string(newBodyBytes), "Coding Agent")
}

func TestAgenticHarnessMiddleware_ResponsesAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootDir := t.TempDir()

	tests := []struct {
		name           string
		body           string
		expectContains string
	}{
		{
			name:           "Inject into instructions",
			body:           `{"instructions":"Follow the user","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`,
			expectContains: "instructions",
		},
		{
			name:           "Inject into input string",
			body:           `{"input":"hello world"}`,
			expectContains: "hello world",
		},
		{
			name:           "Inject into input array",
			body:           `{"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`,
			expectContains: "SYSTEM NOTE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(tt.body))
			req.Header.Set("User-Agent", "codex_cli_rs/0.75.0")
			req.Header.Set("Content-Type", "application/json")
			c.Request = req

			handler := AgenticHarnessMiddlewareWithRootDir(rootDir)
			handler(c)

			assert.Equal(t, "INITIALIZER", c.Writer.Header().Get("X-ProxyPilot-Harness-Mode"))
			newBodyBytes, _ := io.ReadAll(c.Request.Body)
			newBody := string(newBodyBytes)
			assert.Contains(t, newBody, "Initializer Agent")
			assert.Contains(t, newBody, tt.expectContains)
		})
	}
}

func TestAgenticHarnessMiddleware_SessionStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create temp memory directory
	tmpDir, err := os.MkdirTemp("", "harness-session-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	os.Setenv("CLIPROXY_MEMORY_DIR", tmpDir)
	defer os.Unsetenv("CLIPROXY_MEMORY_DIR")

	handler := AgenticHarnessMiddleware()

	// First request with session header
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("User-Agent", "claude-cli/1.0")
	c.Request.Header.Set("X-CLIProxyAPI-Session", "test-session-123")
	c.Request.Header.Set("Content-Type", "application/json")

	handler(c)

	// Verify session directory was considered
	if c.Writer.Header().Get("X-ProxyPilot-Harness-Mode") == "" {
		t.Error("Expected harness mode header to be set")
	}
}

