package test

import (
	"context"
	"strings"
	"testing"

	geminiresponses "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/openai/responses"
	"github.com/tidwall/gjson"
)

func TestGeminiResponsesStream_DoneEventsContainFullText(t *testing.T) {
	var state any
	ctx := context.Background()

	chunk1 := []byte(`{
  "responseId":"resp_1",
  "candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]}}]
}`)
	evs1 := geminiresponses.ConvertGeminiResponseToOpenAIResponses(ctx, "gemini-claude-sonnet-4-5-thinking", nil, nil, chunk1, &state)
	if len(evs1) == 0 {
		t.Fatalf("expected initial chunk to emit events")
	}

	chunk2 := []byte(`{
  "responseId":"resp_1",
  "candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[]}}]
}`)
	evs2 := geminiresponses.ConvertGeminiResponseToOpenAIResponses(ctx, "gemini-claude-sonnet-4-5-thinking", nil, nil, chunk2, &state)

	var itemDoneData string
	for _, ev := range evs2 {
		if strings.Contains(ev, "event: response.output_item.done") {
			parts := strings.SplitN(ev, "data: ", 2)
			if len(parts) == 2 {
				itemDoneData = parts[1]
				break
			}
		}
	}
	if itemDoneData == "" {
		t.Fatalf("expected finish chunk to emit response.output_item.done events, got: %v", evs2)
	}
	got := gjson.Get(itemDoneData, "item.content.0.text").String()
	if got != "Hello" {
		t.Fatalf("expected output_item.done content text to be %q, got %q (data=%s)", "Hello", got, itemDoneData)
	}
}
