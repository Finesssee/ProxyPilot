package openai

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteSSEBody_StripsDoneSentinel_ForFactoryCLI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "factory-cli/0.36.5")

	body := []byte("event: response.completed\n" +
		"data: {\"type\":\"response.completed\"}\n\n" +
		"data: [DONE]\n\n")

	var h OpenAIResponsesAPIHandler
	h.writeSSEBody(c, nopFlusher{}, body)

	out := w.Body.String()
	if strings.Contains(out, "[DONE]") {
		t.Fatalf("expected [DONE] to be stripped for factory-cli, got: %q", out)
	}
	if !strings.Contains(out, "response.completed") {
		t.Fatalf("expected body to contain response content, got: %q", out)
	}
}

func TestWriteSSEBody_KeepsDoneSentinel_ForNonFactory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "some-client/1.0")

	body := []byte("event: response.completed\n" +
		"data: {\"type\":\"response.completed\"}\n\n" +
		"data: [DONE]\n\n")

	var h OpenAIResponsesAPIHandler
	h.writeSSEBody(c, nopFlusher{}, body)

	out := w.Body.String()
	if !strings.Contains(out, "[DONE]") {
		t.Fatalf("expected [DONE] to remain for non-factory clients, got: %q", out)
	}
}
