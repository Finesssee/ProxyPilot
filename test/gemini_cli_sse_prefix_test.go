package test

import (
	"context"
	"testing"

	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestGeminiCLIChatCompletionsStripsSSEDataPrefix(t *testing.T) {
	from := sdktranslator.FromString("gemini-cli")
	to := sdktranslator.FromString("openai")

	originalReq := []byte(`{"model":"gemini-3-pro-preview","stream":true}`)
	requestRaw := []byte(`{"model":"gemini-3-pro-preview","stream":true}`)
	chunk := []byte(`data: {"response":{"responseId":"r1","createTime":"2025-01-01T00:00:00Z","modelVersion":"gemini-3-pro-preview","candidates":[{"finishReason":"","content":{"parts":[{"text":"Hello"}]}}]}}`)

	var param any
	segments := sdktranslator.TranslateStream(context.Background(), from, to, "gemini-3-pro-preview", originalReq, requestRaw, chunk, &param)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}

	out := segments[0]
	if !gjson.Valid(out) {
		t.Fatalf("expected valid JSON output, got: %s", out)
	}

	content := gjson.Get(out, "choices.0.delta.content").String()
	if content != "Hello" {
		t.Fatalf("expected content %q, got %q (out=%s)", "Hello", content, out)
	}
}

