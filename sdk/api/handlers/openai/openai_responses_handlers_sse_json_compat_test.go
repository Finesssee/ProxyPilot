package openai

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteSSEBody_AllDataLinesAreJSON_ForFactoryCLI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "factory-cli/0.36.5")

	// Simulate an SSE body that includes the non-JSON [DONE] sentinel.
	body := []byte("event: response.completed\n" +
		"data: {\"type\":\"response.completed\"}\n\n" +
		"data: [DONE]\n\n")

	var h OpenAIResponsesAPIHandler
	h.writeSSEBody(c, nopFlusher{}, body)

	for _, line := range strings.Split(w.Body.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			t.Fatalf("expected factory-cli data line to be JSON, got %q: %v", payload, err)
		}
	}
}

func TestWriteSSEBody_AllowsDoneSentinel_ForNonFactoryClients(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.72.0")

	body := []byte("event: response.completed\n" +
		"data: {\"type\":\"response.completed\"}\n\n")

	var h OpenAIResponsesAPIHandler
	h.writeSSEBody(c, nopFlusher{}, body)

	out := w.Body.String()
	if !strings.Contains(out, "response.completed") {
		t.Fatalf("expected response content, got: %q", out)
	}
	// For non-factory clients we emit the sentinel when missing.
	if !strings.Contains(out, "[DONE]") {
		t.Fatalf("expected [DONE] to be present for non-factory clients, got: %q", out)
	}
}
