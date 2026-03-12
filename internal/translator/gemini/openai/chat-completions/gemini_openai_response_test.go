package chat_completions

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertGeminiResponseToOpenAI_MapsCandidateFinishReasons(t *testing.T) {
	raw := []byte(`{
  "responseId":"resp_test",
  "modelVersion":"gemini-test",
  "candidates":[
    {"index":0,"content":{"role":"model","parts":[{"text":"a"}]},"finishReason":"MAX_TOKENS"},
    {"index":1,"content":{"role":"model","parts":[{"text":"b"}]},"finishReason":"SAFETY"}
  ]
}`)

	var param any
	chunks := ConvertGeminiResponseToOpenAI(context.Background(), "gemini-test", nil, nil, raw, &param)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	first := gjson.Parse(chunks[0])
	second := gjson.Parse(chunks[1])

	if got := first.Get("choices.0.finish_reason").String(); got != "length" {
		t.Fatalf("expected first finish_reason length, got %q chunk=%s", got, chunks[0])
	}
	if got := first.Get("choices.0.native_finish_reason").String(); got != "max_tokens" {
		t.Fatalf("expected first native_finish_reason max_tokens, got %q chunk=%s", got, chunks[0])
	}
	if got := second.Get("choices.0.finish_reason").String(); got != "content_filter" {
		t.Fatalf("expected second finish_reason content_filter, got %q chunk=%s", got, chunks[1])
	}
	if got := second.Get("choices.0.native_finish_reason").String(); got != "safety" {
		t.Fatalf("expected second native_finish_reason safety, got %q chunk=%s", got, chunks[1])
	}
}

func TestConvertGeminiResponseToOpenAINonStream_MapsToolAndSafetyFinishReasons(t *testing.T) {
	raw := []byte(`{
  "responseId":"resp_test",
  "candidates":[
    {
      "index":0,
      "content":{"role":"model","parts":[{"functionCall":{"name":"Read","args":{"path":"/tmp/x"}}}]},
      "finishReason":"MALFORMED_FUNCTION_CALL"
    },
    {
      "index":1,
      "content":{"role":"model","parts":[{"text":"blocked"}]},
      "finishReason":"BLOCKLIST"
    }
  ]
}`)

	out := ConvertGeminiResponseToOpenAINonStream(context.Background(), "gemini-test", nil, nil, raw, nil)

	if got := gjson.Get(out, "choices.0.finish_reason").String(); got != "tool_calls" {
		t.Fatalf("expected candidate 0 finish_reason tool_calls, got %q body=%s", got, out)
	}
	if got := gjson.Get(out, "choices.0.native_finish_reason").String(); got != "malformed_function_call" {
		t.Fatalf("expected candidate 0 native_finish_reason malformed_function_call, got %q body=%s", got, out)
	}
	if got := gjson.Get(out, "choices.1.finish_reason").String(); got != "content_filter" {
		t.Fatalf("expected candidate 1 finish_reason content_filter, got %q body=%s", got, out)
	}
	if got := gjson.Get(out, "choices.1.native_finish_reason").String(); got != "blocklist" {
		t.Fatalf("expected candidate 1 native_finish_reason blocklist, got %q body=%s", got, out)
	}
}
