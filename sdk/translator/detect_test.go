package translator

import (
	"testing"
)

func TestDetectFormat_OpenAI(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected Format
	}{
		{
			name: "OpenAI with GPT model",
			payload: []byte(`{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`),
			expected: FormatOpenAI,
		},
		{
			name: "OpenAI with o1 model",
			payload: []byte(`{
				"model": "o1-preview",
				"messages": [{"role": "user", "content": "Hello"}]
			}`),
			expected: FormatOpenAI,
		},
		{
			name: "OpenAI with presence_penalty",
			payload: []byte(`{
				"model": "custom-model",
				"messages": [{"role": "user", "content": "Hello"}],
				"presence_penalty": 0.5
			}`),
			expected: FormatOpenAI,
		},
		{
			name: "OpenAI with n parameter",
			payload: []byte(`{
				"model": "custom-model",
				"messages": [{"role": "user", "content": "Hello"}],
				"n": 2
			}`),
			expected: FormatOpenAI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat(tt.payload)
			if result != tt.expected {
				t.Errorf("DetectFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFormat_Claude(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected Format
	}{
		{
			name: "Claude with anthropic_version",
			payload: []byte(`{
				"model": "claude-3",
				"messages": [{"role": "user", "content": "Hello"}],
				"anthropic_version": "2023-06-01"
			}`),
			expected: FormatClaude,
		},
		{
			name: "Claude model name",
			payload: []byte(`{
				"model": "claude-3-opus",
				"messages": [{"role": "user", "content": "Hello"}]
			}`),
			expected: FormatClaude,
		},
		{
			name: "Claude with tool_use content",
			payload: []byte(`{
				"model": "some-model",
				"messages": [{"role": "assistant", "content": [{"type": "tool_use", "id": "123"}]}]
			}`),
			expected: FormatClaude,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat(tt.payload)
			if result != tt.expected {
				t.Errorf("DetectFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFormat_Gemini(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected Format
	}{
		{
			name: "Gemini with contents and generationConfig",
			payload: []byte(`{
				"contents": [{"parts": [{"text": "Hello"}]}],
				"generationConfig": {"temperature": 0.7}
			}`),
			expected: FormatGemini,
		},
		{
			name: "Gemini with contents and parts",
			payload: []byte(`{
				"contents": [{"parts": [{"text": "Hello"}], "role": "user"}]
			}`),
			expected: FormatGemini,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat(tt.payload)
			if result != tt.expected {
				t.Errorf("DetectFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFormat_Antigravity(t *testing.T) {
	payload := []byte(`{
		"request": {
			"contents": [{"parts": [{"text": "Hello"}]}]
		}
	}`)

	result := DetectFormat(payload)
	if result != FormatAntigravity {
		t.Errorf("DetectFormat() = %v, want %v", result, FormatAntigravity)
	}
}

func TestDetectFormat_OpenAIResponse(t *testing.T) {
	payload := []byte(`{
		"input": "Hello",
		"instructions": "Be helpful"
	}`)

	result := DetectFormat(payload)
	if result != FormatOpenAIResponse {
		t.Errorf("DetectFormat() = %v, want %v", result, FormatOpenAIResponse)
	}
}

func TestDetectFormat_Empty(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"nil payload", nil},
		{"empty payload", []byte{}},
		{"non-object JSON", []byte(`["array"]`)},
		{"invalid JSON", []byte(`{invalid}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat(tt.payload)
			if result != "" {
				t.Errorf("DetectFormat() = %v, want empty string", result)
			}
		})
	}
}

func TestDetectFormatDetailed_Confidence(t *testing.T) {
	tests := []struct {
		name           string
		payload        []byte
		expectedFormat Format
		minConfidence  float64
	}{
		{
			name: "Antigravity - highest confidence",
			payload: []byte(`{
				"request": {"contents": []}
			}`),
			expectedFormat: FormatAntigravity,
			minConfidence:  1.0,
		},
		{
			name: "Claude with anthropic_version - high confidence",
			payload: []byte(`{
				"model": "claude",
				"messages": [],
				"anthropic_version": "2023-06-01"
			}`),
			expectedFormat: FormatClaude,
			minConfidence:  1.0,
		},
		{
			name: "Gemini with generationConfig - high confidence",
			payload: []byte(`{
				"contents": [],
				"generationConfig": {}
			}`),
			expectedFormat: FormatGemini,
			minConfidence:  0.9,
		},
		{
			name: "Generic messages+model - lower confidence",
			payload: []byte(`{
				"model": "unknown-model",
				"messages": []
			}`),
			expectedFormat: FormatOpenAI,
			minConfidence:  0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormatDetailed(tt.payload)
			if result.Format != tt.expectedFormat {
				t.Errorf("Format = %v, want %v", result.Format, tt.expectedFormat)
			}
			if result.Confidence < tt.minConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.minConfidence)
			}
			if result.Reason == "" {
				t.Error("Reason should not be empty")
			}
		})
	}
}

func TestDetectFormatDetailed_EmptyPayload(t *testing.T) {
	result := DetectFormatDetailed([]byte{})
	if result.Format != "" {
		t.Errorf("Format = %v, want empty", result.Format)
	}
	if result.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", result.Confidence)
	}
	if result.Reason != "empty payload" {
		t.Errorf("Reason = %v, want 'empty payload'", result.Reason)
	}
}

func TestDetectFormatDetailed_NonObject(t *testing.T) {
	result := DetectFormatDetailed([]byte(`["array"]`))
	if result.Format != "" {
		t.Errorf("Format = %v, want empty", result.Format)
	}
	if result.Reason != "not a JSON object" {
		t.Errorf("Reason = %v, want 'not a JSON object'", result.Reason)
	}
}

func TestMustDetectFormat_Success(t *testing.T) {
	payload := []byte(`{"model": "gpt-4", "messages": []}`)

	// Should not panic
	result := MustDetectFormat(payload)
	if result != FormatOpenAI {
		t.Errorf("MustDetectFormat() = %v, want %v", result, FormatOpenAI)
	}
}

func TestMustDetectFormat_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustDetectFormat should panic on undetectable format")
		}
	}()

	MustDetectFormat([]byte(`{"unknown": "structure"}`))
}

func TestIsKnownFormat(t *testing.T) {
	knownFormats := []Format{
		FormatOpenAI,
		FormatOpenAIResponse,
		FormatClaude,
		FormatGemini,
		FormatGeminiCLI,
		FormatCodex,
		FormatAntigravity,
		FormatKiro,
	}

	for _, f := range knownFormats {
		if !IsKnownFormat(f) {
			t.Errorf("IsKnownFormat(%v) = false, want true", f)
		}
	}

	unknownFormats := []Format{
		"unknown",
		"custom-format",
		"",
	}

	for _, f := range unknownFormats {
		if IsKnownFormat(f) {
			t.Errorf("IsKnownFormat(%v) = true, want false", f)
		}
	}
}
