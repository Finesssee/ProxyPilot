package openai

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
)

func TestCodexDetThinkingModel(t *testing.T) {
	if got := codexDetThinkingModel("gemini-claude-sonnet-4-5-thinking"); got != "gemini-claude-sonnet-4-5" {
		t.Fatalf("unexpected model det-thinking: %q", got)
	}
	if got := codexDetThinkingModel("gemini-3-flash"); got != "gemini-3-flash" {
		t.Fatalf("unexpected model change: %q", got)
	}
}

func TestCodexIsSilentMaxTokens(t *testing.T) {
	resp := []byte(`{"id":"resp_1","output_text":"","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[]}`)
	if !codexIsSilentMaxTokens(resp) {
		t.Fatalf("expected silent MAX_TOKENS detection")
	}
}

func TestCodexIsSilentMaxTokens_IgnoresToolCalls(t *testing.T) {
	resp := []byte(`{"id":"resp_1","output_text":"","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"function_call","name":"x","arguments":"{}"}]}`)
	if codexIsSilentMaxTokens(resp) {
		t.Fatalf("expected tool calls to suppress silent detection")
	}
}

func TestCodexNonStreamWithSingleRetry_DetThinking(t *testing.T) {
	calls := 0
	var seenModels []string
	exec := func(model string, _ []byte) ([]byte, *interfaces.ErrorMessage) {
		calls++
		seenModels = append(seenModels, model)
		if calls == 1 {
			return []byte(`{"id":"resp_1","output_text":"","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[]}`), nil
		}
		return []byte(`{"id":"resp_1","output_text":"ok","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}],"role":"assistant"}]}`), nil
	}

	resp, retried, usedModel, errMsg := codexNonStreamWithSingleRetry("gemini-claude-sonnet-4-5-thinking", []byte(`{"model":"gemini-claude-sonnet-4-5-thinking","stream":false}`), exec)
	if errMsg != nil {
		t.Fatalf("unexpected error: %v", errMsg)
	}
	if !retried {
		t.Fatalf("expected retry to be used")
	}
	if usedModel != "gemini-claude-sonnet-4-5" {
		t.Fatalf("expected det-thinking model, got=%q", usedModel)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got=%d", calls)
	}
	if len(seenModels) != 2 || seenModels[0] != "gemini-claude-sonnet-4-5-thinking" || seenModels[1] != "gemini-claude-sonnet-4-5" {
		t.Fatalf("unexpected models: %#v", seenModels)
	}
	if string(resp) == "" {
		t.Fatalf("expected response")
	}
}
