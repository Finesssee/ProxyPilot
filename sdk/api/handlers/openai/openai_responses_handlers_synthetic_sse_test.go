package openai

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type nopFlusher struct{}

func (nopFlusher) Flush() {}

func TestSyntheticResponsesSSE_EmitsFunctionCallItems(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "factory-cli/0.36.3")

	resp := []byte(`{
  "id":"resp_1",
  "object":"response",
  "status":"completed",
  "output":[
    {"id":"call_1","type":"function_call","name":"TodoWrite","arguments":"{\"todos\":[{\"status\":\"in_progress\",\"content\":\"x\"}]}"}
  ]
}`)

	var h OpenAIResponsesAPIHandler
	h.writeSyntheticResponsesSSE(c, nopFlusher{}, resp)

	body := w.Body.String()
	if !strings.Contains(body, "response.output_item.added") || !strings.Contains(body, `"type":"function_call"`) {
		t.Fatalf("expected SSE to include function_call output_item.added, got: %s", body)
	}
	if !strings.Contains(body, "response.function_call_arguments.delta") || !strings.Contains(body, "TodoWrite") {
		t.Fatalf("expected SSE to include function_call_arguments delta with tool name, got: %s", body)
	}
	if !strings.Contains(body, "response.output_item.done") || !strings.Contains(body, "response.completed") {
		t.Fatalf("expected SSE to include output_item.done and response.completed, got: %s", body)
	}
}
