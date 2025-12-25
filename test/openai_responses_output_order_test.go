package test

import (
	"context"
	"testing"

	openairesponses "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/openai/openai/responses"
	"github.com/tidwall/gjson"
)

func TestOpenAIResponsesNonStream_ReasoningOutputFirst(t *testing.T) {
	// When reasoning_content is present, the OpenAI Responses API puts reasoning first.
	// This matches the order in which they appear during streaming.
	chatCompletion := []byte(`{
  "id":"cmpl_1",
  "object":"chat.completion",
  "created":1700000000,
  "model":"gemini-3-pro-preview",
  "choices":[{"index":0,"message":{"role":"assistant","content":"hello","reasoning_content":"thinking..."},"finish_reason":"stop"}],
  "usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
}`)

	resp := openairesponses.ConvertOpenAIChatCompletionsResponseToOpenAIResponsesNonStream(
		context.Background(),
		"gemini-3-pro-preview",
		nil,
		[]byte(`{"model":"gemini-3-pro-preview"}`),
		chatCompletion,
		nil,
	)

	// Reasoning comes first when present
	firstType := gjson.Get(resp, "output.0.type").String()
	if firstType != "reasoning" {
		t.Fatalf("expected output[0].type to be %q, got %q (resp=%s)", "reasoning", firstType, resp)
	}

	// Message comes second
	secondType := gjson.Get(resp, "output.1.type").String()
	if secondType != "message" {
		t.Fatalf("expected output[1].type to be %q, got %q (resp=%s)", "message", secondType, resp)
	}
}

