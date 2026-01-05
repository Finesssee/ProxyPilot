package handlers

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/cache"
)

func TestExtractClaudeSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		rawJSON  string
		expected string
	}{
		{
			name:     "string system prompt",
			rawJSON:  `{"model":"claude-3","system":"You are a helpful assistant.","messages":[]}`,
			expected: "You are a helpful assistant.",
		},
		{
			name:     "array system prompt",
			rawJSON:  `{"model":"claude-3","system":[{"type":"text","text":"First part."},{"type":"text","text":"Second part."}],"messages":[]}`,
			expected: "First part.\nSecond part.",
		},
		{
			name:     "empty system",
			rawJSON:  `{"model":"claude-3","messages":[]}`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSystemPrompt("claude", []byte(tt.rawJSON))
			if result != tt.expected {
				t.Errorf("extractSystemPrompt() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractOpenAISystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		rawJSON  string
		expected string
	}{
		{
			name:     "system role in messages",
			rawJSON:  `{"model":"gpt-4","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Hello"}]}`,
			expected: "You are helpful.",
		},
		{
			name:     "multiple system messages",
			rawJSON:  `{"model":"gpt-4","messages":[{"role":"system","content":"First."},{"role":"system","content":"Second."},{"role":"user","content":"Hello"}]}`,
			expected: "First.\nSecond.",
		},
		{
			name:     "no system message",
			rawJSON:  `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`,
			expected: "",
		},
		{
			name:     "instructions field (Responses API)",
			rawJSON:  `{"model":"gpt-4","instructions":"You are a code assistant.","input":"Write code"}`,
			expected: "You are a code assistant.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSystemPrompt("openai", []byte(tt.rawJSON))
			if result != tt.expected {
				t.Errorf("extractSystemPrompt() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractGeminiSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		rawJSON  string
		expected string
	}{
		{
			name:     "systemInstruction with parts",
			rawJSON:  `{"model":"gemini-pro","systemInstruction":{"parts":[{"text":"You are Gemini."}]}}`,
			expected: "You are Gemini.",
		},
		{
			name:     "systemInstruction string",
			rawJSON:  `{"model":"gemini-pro","systemInstruction":"Simple system prompt"}`,
			expected: "Simple system prompt",
		},
		{
			name:     "system_instruction snake_case",
			rawJSON:  `{"model":"gemini-pro","system_instruction":{"parts":[{"text":"Snake case works."}]}}`,
			expected: "Snake case works.",
		},
		{
			name:     "no system instruction",
			rawJSON:  `{"model":"gemini-pro","contents":[]}`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSystemPrompt("gemini", []byte(tt.rawJSON))
			if result != tt.expected {
				t.Errorf("extractSystemPrompt() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractAndTrackSystemPrompt_CacheDisabled(t *testing.T) {
	// Default cache is disabled, so we should get DISABLED status
	rawJSON := []byte(`{"model":"claude-3","system":"Test prompt","messages":[]}`)

	status, hash := ExtractAndTrackSystemPrompt("claude", rawJSON, "anthropic")

	// When cache is disabled, status should be DISABLED
	if status != PromptCacheDisabled {
		t.Errorf("ExtractAndTrackSystemPrompt() status = %v, want %v", status, PromptCacheDisabled)
	}

	// Hash should still be computed
	expectedHash := cache.HashPrompt("Test prompt")
	if hash != expectedHash {
		t.Errorf("ExtractAndTrackSystemPrompt() hash = %v, want %v", hash, expectedHash)
	}
}

func TestExtractAndTrackSystemPrompt_EmptyPrompt(t *testing.T) {
	rawJSON := []byte(`{"model":"claude-3","messages":[]}`)

	status, hash := ExtractAndTrackSystemPrompt("claude", rawJSON, "anthropic")

	if status != PromptCacheDisabled {
		t.Errorf("ExtractAndTrackSystemPrompt() status = %v, want %v", status, PromptCacheDisabled)
	}

	if hash != "" {
		t.Errorf("ExtractAndTrackSystemPrompt() hash = %v, want empty", hash)
	}
}

func TestExtractAndTrackSystemPrompt_CacheEnabled(t *testing.T) {
	// Enable cache for this test
	cfg := cache.PromptCacheConfig{
		Enabled: true,
		MaxSize: 100,
		TTL:     10 * 60 * 1000000000, // 10 minutes in nanoseconds
	}
	testCache := cache.NewPromptCache(cfg)

	// First request should be a MISS
	rawJSON := []byte(`{"model":"claude-3","system":"Unique test prompt for cache test","messages":[]}`)

	// Manually extract and track with the test cache
	prompt := extractSystemPrompt("claude", rawJSON)
	_, isNew := testCache.CacheSystemPrompt(prompt, "anthropic")

	if !isNew {
		t.Error("First cache call should return isNew=true")
	}

	// Second request with same prompt should be a HIT
	_, isNew2 := testCache.CacheSystemPrompt(prompt, "anthropic")
	if isNew2 {
		t.Error("Second cache call should return isNew=false (cache hit)")
	}
}

func TestFallbackExtraction(t *testing.T) {
	// Test that unknown handler types fall back to trying all formats
	tests := []struct {
		name        string
		handlerType string
		rawJSON     string
		expected    string
	}{
		{
			name:        "unknown type finds Claude format",
			handlerType: "unknown",
			rawJSON:     `{"system":"Claude style"}`,
			expected:    "Claude style",
		},
		{
			name:        "unknown type finds OpenAI format",
			handlerType: "custom",
			rawJSON:     `{"messages":[{"role":"system","content":"OpenAI style"}]}`,
			expected:    "OpenAI style",
		},
		{
			name:        "unknown type finds Gemini format",
			handlerType: "other",
			rawJSON:     `{"systemInstruction":{"parts":[{"text":"Gemini style"}]}}`,
			expected:    "Gemini style",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSystemPrompt(tt.handlerType, []byte(tt.rawJSON))
			if result != tt.expected {
				t.Errorf("extractSystemPrompt() = %q, want %q", result, tt.expected)
			}
		})
	}
}
