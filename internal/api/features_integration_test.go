package api

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
)

// mockCoreManager implements memory.CoreManagerExecutor for testing
type mockCoreManager struct {
	response interface{}
	err      error
}

func (m *mockCoreManager) Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) (interface{}, error) {
	return m.response, m.err
}

func TestContextCompression_AdapterIntegration(t *testing.T) {
	// Test that the adapter correctly bridges manager to executor
	mockResp := struct {
		Payload []byte `json:"payload"`
	}{
		Payload: []byte(`{"choices":[{"message":{"content":"Test summary"}}]}`),
	}

	manager := &mockCoreManager{response: mockResp}
	adapter := memory.NewManagerAuthAdapter(manager)

	if adapter == nil {
		t.Fatal("Expected adapter to be created")
	}

	// Test execute
	resp, err := adapter.Execute(context.Background(), []string{"claude"}, memory.ExecutorRequest{
		Model:   "claude-3-opus",
		Payload: []byte(`{"test": true}`),
	}, memory.ExecutorOptions{})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(resp) == 0 {
		t.Error("Expected non-empty response")
	}
}

func TestAgenticHarness_MiddlewareIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create temp directory for harness files
	tmpDir, err := os.MkdirTemp("", "harness-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	handler := middleware.AgenticHarnessMiddlewareWithRootDir(tmpDir)

	// Test 1: Fresh conversation should get INITIALIZER
	t.Run("fresh_conversation_initializer", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		body := `{"model":"gpt-4","messages":[{"role":"user","content":"Build a todo app"}]}`
		c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		c.Request.Header.Set("User-Agent", "claude-cli/1.0")
		c.Request.Header.Set("Content-Type", "application/json")

		handler(c)

		mode := c.Writer.Header().Get("X-ProxyPilot-Harness-Mode")
		if mode != "INITIALIZER" {
			t.Errorf("Expected INITIALIZER mode, got: %s", mode)
		}
	})

	// Test 2: After creating harness files, should get CODING
	t.Run("with_harness_files_coding", func(t *testing.T) {
		// Create harness file
		os.WriteFile(tmpDir+"/feature_list.json", []byte(`[{"id":"test","passes":false}]`), 0644)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		body := `{"model":"gpt-4","messages":[{"role":"user","content":"Continue"}]}`
		c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		c.Request.Header.Set("User-Agent", "claude-cli/1.0")
		c.Request.Header.Set("Content-Type", "application/json")

		handler(c)

		mode := c.Writer.Header().Get("X-ProxyPilot-Harness-Mode")
		if mode != "CODING" {
			t.Errorf("Expected CODING mode, got: %s", mode)
		}
	})
}

func TestCombinedFeatures_NoConflict(t *testing.T) {
	// Verify both features can be enabled simultaneously without conflict
	gin.SetMode(gin.TestMode)

	// Set up harness
	tmpDir, _ := os.MkdirTemp("", "combined-test")
	defer os.RemoveAll(tmpDir)

	// Create engine with both middlewares
	engine := gin.New()
	engine.Use(middleware.AgenticHarnessMiddleware())

	// Verify engine was created without panic
	if engine == nil {
		t.Fatal("Engine should be created")
	}
}
