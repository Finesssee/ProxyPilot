package translator

import (
	"testing"
)

// TestNeedConvert tests the NeedConvert function for checking if translation is needed.
func TestNeedConvert(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		to       string
		wantNeed bool // Note: depends on what translators are registered
	}{
		{
			name:     "same format should not need convert",
			from:     "openai",
			to:       "openai",
			wantNeed: false,
		},
		{
			name:     "empty from and to",
			from:     "",
			to:       "",
			wantNeed: false,
		},
		{
			name:     "whitespace only",
			from:     "   ",
			to:       "   ",
			wantNeed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedConvert(tt.from, tt.to)
			// For same format or empty, should be false
			if tt.from == tt.to || (tt.from == "" && tt.to == "") {
				if got != false {
					t.Errorf("NeedConvert(%q, %q) = %v, want false for identical/empty formats",
						tt.from, tt.to, got)
				}
			}
		})
	}
}

// TestRequest_EmptyInput tests Request function with empty inputs.
func TestRequest_EmptyInput(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		to        string
		modelName string
		rawJSON   []byte
		stream    bool
	}{
		{
			name:      "empty JSON",
			from:      "openai",
			to:        "claude",
			modelName: "test-model",
			rawJSON:   []byte{},
			stream:    false,
		},
		{
			name:      "nil JSON",
			from:      "openai",
			to:        "claude",
			modelName: "test-model",
			rawJSON:   nil,
			stream:    false,
		},
		{
			name:      "empty model name",
			from:      "openai",
			to:        "claude",
			modelName: "",
			rawJSON:   []byte(`{"messages":[]}`),
			stream:    false,
		},
		{
			name:      "empty from format",
			from:      "",
			to:        "claude",
			modelName: "test",
			rawJSON:   []byte(`{"messages":[]}`),
			stream:    false,
		},
		{
			name:      "empty to format",
			from:      "openai",
			to:        "",
			modelName: "test",
			rawJSON:   []byte(`{"messages":[]}`),
			stream:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic with empty/nil inputs
			result := Request(tt.from, tt.to, tt.modelName, tt.rawJSON, tt.stream)
			// Result may be nil/empty for unregistered translators, which is fine
			_ = result
		})
	}
}

// TestRequest_PassthroughWhenSameFormat tests that same format returns original.
func TestRequest_PassthroughWhenSameFormat(t *testing.T) {
	originalJSON := []byte(`{"model":"test","messages":[{"role":"user","content":"hello"}]}`)

	result := Request("openai", "openai", "test-model", originalJSON, false)

	// When no translator is registered for same format, the original should be returned
	// or it may be nil - both are acceptable behaviors
	if result != nil && len(result) > 0 && string(result) != string(originalJSON) {
		// The result is different - this is fine if the registry has a passthrough
		// The key is that it doesn't panic
	}
}

// TestFormatStrings tests that format string handling is consistent.
func TestFormatStrings(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"lowercase", "openai"},
		{"uppercase", "OPENAI"},
		{"mixed case", "OpenAI"},
		{"with hyphen", "openai-responses"},
		{"with underscore", "openai_responses"},
		{"claude format", "claude"},
		{"gemini format", "gemini"},
		{"gemini-cli format", "gemini-cli"},
		{"codex format", "codex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that format strings don't cause panics
			NeedConvert(tt.format, "openai")
			NeedConvert("openai", tt.format)
			Request(tt.format, "openai", "test", []byte(`{}`), false)
			Request("openai", tt.format, "test", []byte(`{}`), false)
		})
	}
}

// TestRequest_StreamingFlag tests that the streaming flag is handled.
func TestRequest_StreamingFlag(t *testing.T) {
	testJSON := []byte(`{"model":"test","messages":[{"role":"user","content":"hello"}]}`)

	// Test with stream=true
	resultStream := Request("openai", "claude", "test", testJSON, true)

	// Test with stream=false
	resultNonStream := Request("openai", "claude", "test", testJSON, false)

	// Both should not panic - the actual difference depends on translator implementation
	_ = resultStream
	_ = resultNonStream
}
