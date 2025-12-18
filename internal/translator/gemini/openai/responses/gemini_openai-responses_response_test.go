package responses

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConvertGeminiResponseToOpenAIResponses_EmitsFallbackTextForCodexOnMaxTokensNoOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.73.0 (Windows)")

	ctx := context.WithValue(context.Background(), "gin", c)

	var param any
	raw := []byte(`{"responseId":"req_test","candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"MAX_TOKENS"}]}`)
	chunks := ConvertGeminiResponseToOpenAIResponses(ctx, "gemini-claude-sonnet-4-5-thinking", nil, nil, raw, &param)
	joined := strings.Join(chunks, "\n")
	if !strings.Contains(joined, "response.output_text.delta") {
		t.Fatalf("expected output_text delta event, got=%s", joined)
	}
	if !strings.Contains(joined, "No visible output was produced") {
		t.Fatalf("expected fallback message, got=%s", joined)
	}
}

func TestConvertGeminiResponseToOpenAIResponsesNonStream_MapsMaxTokensToIncomplete(t *testing.T) {
	var param any
	raw := []byte(`{"responseId":"req_test","candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"MAX_TOKENS"}]}`)
	resp := ConvertGeminiResponseToOpenAIResponsesNonStream(context.Background(), "gemini-claude-sonnet-4-5-thinking", nil, nil, raw, &param)
	if !strings.Contains(resp, `"status":"incomplete"`) {
		t.Fatalf("expected status incomplete, got=%s", resp)
	}
	if !strings.Contains(resp, `"incomplete_details"`) || !strings.Contains(resp, "max_output_tokens") {
		t.Fatalf("expected incomplete_details max_output_tokens, got=%s", resp)
	}
}
