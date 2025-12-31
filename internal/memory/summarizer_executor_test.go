package memory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockCoreManagerExecutor is a mock implementation of CoreManagerExecutor for testing.
type mockCoreManagerExecutor struct {
	response interface{}
	err      error
}

func (m *mockCoreManagerExecutor) Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) (interface{}, error) {
	return m.response, m.err
}

// responseWithPayload is a test struct that has a Payload field for testing extraction.
type responseWithPayload struct {
	Payload []byte `json:"payload"`
	Status  int    `json:"status"`
}

// responseWithGetPayload implements the GetPayload interface.
type responseWithGetPayload struct {
	data []byte
}

func (r *responseWithGetPayload) GetPayload() []byte {
	return r.data
}

// TestManagerAuthAdapter_NilManager tests that Execute returns an error when manager is nil.
func TestManagerAuthAdapter_NilManager(t *testing.T) {
	adapter := &ManagerAuthAdapter{manager: nil}

	ctx := context.Background()
	providers := []string{"test-provider"}

	result, err := adapter.Execute(ctx, providers, nil, nil)

	if err == nil {
		t.Fatal("Expected error when manager is nil, got nil")
	}

	expectedErr := "manager auth adapter: manager not configured"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

// TestManagerAuthAdapter_NilResponse tests that Execute returns an error when manager returns nil response.
func TestManagerAuthAdapter_NilResponse(t *testing.T) {
	mock := &mockCoreManagerExecutor{
		response: nil,
		err:      nil,
	}
	adapter := NewManagerAuthAdapter(mock)

	ctx := context.Background()
	providers := []string{"test-provider"}

	result, err := adapter.Execute(ctx, providers, nil, nil)

	if err == nil {
		t.Fatal("Expected error when response is nil, got nil")
	}

	expectedErr := "manager auth adapter: nil response from manager"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

// TestManagerAuthAdapter_BytesResponse tests that Execute returns bytes directly when response is []byte.
func TestManagerAuthAdapter_BytesResponse(t *testing.T) {
	expectedBytes := []byte(`{"choices":[{"message":{"content":"test response"}}]}`)
	mock := &mockCoreManagerExecutor{
		response: expectedBytes,
		err:      nil,
	}
	adapter := NewManagerAuthAdapter(mock)

	ctx := context.Background()
	providers := []string{"test-provider"}

	result, err := adapter.Execute(ctx, providers, nil, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(result) != string(expectedBytes) {
		t.Errorf("Expected result '%s', got '%s'", string(expectedBytes), string(result))
	}
}

// TestManagerAuthAdapter_StructWithPayload tests that Execute extracts Payload field from struct response.
func TestManagerAuthAdapter_StructWithPayload(t *testing.T) {
	expectedPayload := []byte(`{"choices":[{"message":{"content":"extracted payload"}}]}`)
	mock := &mockCoreManagerExecutor{
		response: responseWithPayload{
			Payload: expectedPayload,
			Status:  200,
		},
		err: nil,
	}
	adapter := NewManagerAuthAdapter(mock)

	ctx := context.Background()
	providers := []string{"test-provider"}

	result, err := adapter.Execute(ctx, providers, nil, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(result) != string(expectedPayload) {
		t.Errorf("Expected result '%s', got '%s'", string(expectedPayload), string(result))
	}
}

// TestManagerAuthAdapter_GetPayloadInterface tests that Execute uses GetPayload() method when available.
func TestManagerAuthAdapter_GetPayloadInterface(t *testing.T) {
	expectedPayload := []byte(`{"choices":[{"message":{"content":"from GetPayload"}}]}`)
	mock := &mockCoreManagerExecutor{
		response: &responseWithGetPayload{data: expectedPayload},
		err:      nil,
	}
	adapter := NewManagerAuthAdapter(mock)

	ctx := context.Background()
	providers := []string{"test-provider"}

	result, err := adapter.Execute(ctx, providers, nil, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(result) != string(expectedPayload) {
		t.Errorf("Expected result '%s', got '%s'", string(expectedPayload), string(result))
	}
}

// TestManagerAuthAdapter_ManagerError tests that Execute propagates manager errors.
func TestManagerAuthAdapter_ManagerError(t *testing.T) {
	expectedErr := errors.New("upstream provider error")
	mock := &mockCoreManagerExecutor{
		response: nil,
		err:      expectedErr,
	}
	adapter := NewManagerAuthAdapter(mock)

	ctx := context.Background()
	providers := []string{"test-provider"}

	result, err := adapter.Execute(ctx, providers, nil, nil)

	if err == nil {
		t.Fatal("Expected error to be propagated, got nil")
	}

	if err != expectedErr {
		t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

// TestManagerAuthAdapter_SerializedFallback tests fallback to JSON serialization for unknown types without Payload.
func TestManagerAuthAdapter_SerializedFallback(t *testing.T) {
	unknownResponse := map[string]interface{}{
		"data":   "some data",
		"status": "ok",
	}
	mock := &mockCoreManagerExecutor{
		response: unknownResponse,
		err:      nil,
	}
	adapter := NewManagerAuthAdapter(mock)

	ctx := context.Background()
	providers := []string{"test-provider"}

	result, err := adapter.Execute(ctx, providers, nil, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Result should be JSON serialized version of the response
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Result should be valid JSON: %v", err)
	}

	if parsed["data"] != "some data" {
		t.Errorf("Expected data='some data', got %v", parsed["data"])
	}

	if parsed["status"] != "ok" {
		t.Errorf("Expected status='ok', got %v", parsed["status"])
	}
}

// mockAuthExecutor is a mock for testing PipelineSummarizerExecutor.
type mockAuthExecutor struct {
	response []byte
	err      error
	called   bool
	lastReq  interface{}
	lastOpts interface{}
}

func (m *mockAuthExecutor) Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) ([]byte, error) {
	m.called = true
	m.lastReq = req
	m.lastOpts = opts
	return m.response, m.err
}

// mockAuthExecutorWithCallback allows dynamic behavior via a callback function.
type mockAuthExecutorWithCallback struct {
	callback func(ctx context.Context, providers []string, req interface{}, opts interface{}) ([]byte, error)
}

func (m *mockAuthExecutorWithCallback) Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) ([]byte, error) {
	return m.callback(ctx, providers, req, opts)
}

// TestPipelineSummarizerExecutor_WithAdapter tests the full pipeline with the adapter integration.
func TestPipelineSummarizerExecutor_WithAdapter(t *testing.T) {
	// Create a valid OpenAI-style response
	responseContent := "This is a summarized version of the conversation."
	openAIResponse := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"message": map[string]interface{}{
					"content": responseContent,
				},
			},
		},
	}
	responseBytes, _ := json.Marshal(openAIResponse)

	// Create mock core manager that returns a struct with Payload
	mockManager := &mockCoreManagerExecutor{
		response: responseWithPayload{
			Payload: responseBytes,
			Status:  200,
		},
		err: nil,
	}

	// Create adapter wrapping the mock manager
	adapter := NewManagerAuthAdapter(mockManager)

	// Create pipeline executor with the adapter
	executor := NewPipelineSummarizerExecutor(adapter, []string{"openai"})

	ctx := context.Background()
	model := "gpt-4"
	prompt := "Please summarize the following conversation..."

	result, err := executor.Summarize(ctx, model, prompt)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != responseContent {
		t.Errorf("Expected result '%s', got '%s'", responseContent, result)
	}
}

// TestPipelineSummarizerExecutor_WithAdapter_BytesResponse tests pipeline with direct bytes response.
func TestPipelineSummarizerExecutor_WithAdapter_BytesResponse(t *testing.T) {
	responseContent := "Summary from bytes response."
	openAIResponse := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"message": map[string]interface{}{
					"content": responseContent,
				},
			},
		},
	}
	responseBytes, _ := json.Marshal(openAIResponse)

	// Mock manager returns bytes directly
	mockManager := &mockCoreManagerExecutor{
		response: responseBytes,
		err:      nil,
	}

	adapter := NewManagerAuthAdapter(mockManager)
	executor := NewPipelineSummarizerExecutor(adapter, []string{"anthropic"})

	ctx := context.Background()
	result, err := executor.Summarize(ctx, "claude-3-opus", "Summarize this...")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != responseContent {
		t.Errorf("Expected result '%s', got '%s'", responseContent, result)
	}
}

// TestPipelineSummarizerExecutor_WithAdapter_ManagerError tests error propagation through the pipeline.
func TestPipelineSummarizerExecutor_WithAdapter_ManagerError(t *testing.T) {
	mockManager := &mockCoreManagerExecutor{
		response: nil,
		err:      errors.New("rate limit exceeded"),
	}

	adapter := NewManagerAuthAdapter(mockManager)
	executor := NewPipelineSummarizerExecutor(adapter, []string{"openai"})

	ctx := context.Background()
	result, err := executor.Summarize(ctx, "gpt-4", "Summarize this...")

	if err == nil {
		t.Fatal("Expected error to be propagated, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty result on error, got '%s'", result)
	}

	// Error should be wrapped with context
	expectedSubstring := "rate limit exceeded"
	if !containsSubstring(err.Error(), expectedSubstring) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedSubstring, err.Error())
	}
}

// TestNewManagerAuthAdapter tests the constructor.
func TestNewManagerAuthAdapter(t *testing.T) {
	mock := &mockCoreManagerExecutor{}
	adapter := NewManagerAuthAdapter(mock)

	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}

	if adapter.manager != mock {
		t.Error("Expected adapter.manager to be set to the provided mock")
	}
}

// TestNewManagerAuthAdapter_NilInput tests constructor with nil input.
func TestNewManagerAuthAdapter_NilInput(t *testing.T) {
	adapter := NewManagerAuthAdapter(nil)

	if adapter == nil {
		t.Fatal("Expected non-nil adapter even with nil input")
	}

	// The adapter should still be created, but Execute should fail
	_, err := adapter.Execute(context.Background(), nil, nil, nil)
	if err == nil {
		t.Error("Expected error when executing with nil manager")
	}
}

// containsSubstring is a helper function to check if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestExtractAssistantContent_OpenAIFormat tests OpenAI chat completion response parsing.
func TestExtractAssistantContent_OpenAIFormat(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
		wantErr  bool
	}{
		{
			name:     "standard chat completion",
			payload:  `{"choices":[{"message":{"content":"Hello, I am a summary."}}]}`,
			expected: "Hello, I am a summary.",
			wantErr:  false,
		},
		{
			name:     "streaming delta format",
			payload:  `{"choices":[{"delta":{"content":"Streaming content here."}}]}`,
			expected: "Streaming content here.",
			wantErr:  false,
		},
		{
			name:     "legacy completions API",
			payload:  `{"choices":[{"text":"Legacy completion text."}]}`,
			expected: "Legacy completion text.",
			wantErr:  false,
		},
		{
			name:     "multiple choices returns first",
			payload:  `{"choices":[{"message":{"content":"First choice."}},{"message":{"content":"Second choice."}}]}`,
			expected: "First choice.",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractAssistantContent([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestExtractAssistantContent_ClaudeFormat tests Claude/Anthropic Messages API response parsing.
func TestExtractAssistantContent_ClaudeFormat(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
		wantErr  bool
	}{
		{
			name:     "modern content blocks",
			payload:  `{"content":[{"type":"text","text":"Claude response content."}]}`,
			expected: "Claude response content.",
			wantErr:  false,
		},
		{
			name:     "multiple content blocks",
			payload:  `{"content":[{"type":"thinking","text":"<thinking>Let me think...</thinking>"},{"type":"text","text":"Actual response."}]}`,
			expected: "Actual response.",
			wantErr:  false,
		},
		{
			name:     "legacy completion field",
			payload:  `{"completion":"Legacy Claude completion."}`,
			expected: "Legacy Claude completion.",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractAssistantContent([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestExtractAssistantContent_GeminiFormat tests Gemini API response parsing.
func TestExtractAssistantContent_GeminiFormat(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
		wantErr  bool
	}{
		{
			name:     "candidates with parts",
			payload:  `{"candidates":[{"content":{"parts":[{"text":"Gemini response."}]}}]}`,
			expected: "Gemini response.",
			wantErr:  false,
		},
		{
			name:     "multiple parts returns first",
			payload:  `{"candidates":[{"content":{"parts":[{"text":"First part."},{"text":"Second part."}]}}]}`,
			expected: "First part.",
			wantErr:  false,
		},
		{
			name:     "candidate with output field",
			payload:  `{"candidates":[{"output":"Direct output field."}]}`,
			expected: "Direct output field.",
			wantErr:  false,
		},
		{
			name:     "top-level text field",
			payload:  `{"text":"Simple text response."}`,
			expected: "Simple text response.",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractAssistantContent([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestExtractAssistantContent_GenericFormats tests fallback parsing for unknown formats.
func TestExtractAssistantContent_GenericFormats(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
		wantErr  bool
	}{
		{
			name:     "generic content field",
			payload:  `{"content":"Generic content value."}`,
			expected: "Generic content value.",
			wantErr:  false,
		},
		{
			name:     "generic text field",
			payload:  `{"text":"Generic text value."}`,
			expected: "Generic text value.",
			wantErr:  false,
		},
		{
			name:     "generic response field",
			payload:  `{"response":"Response field value."}`,
			expected: "Response field value.",
			wantErr:  false,
		},
		{
			name:     "generic output field",
			payload:  `{"output":"Output field value."}`,
			expected: "Output field value.",
			wantErr:  false,
		},
		{
			name:     "generic result field",
			payload:  `{"result":"Result field value."}`,
			expected: "Result field value.",
			wantErr:  false,
		},
		{
			name:     "nested message with text",
			payload:  `{"message":{"text":"Nested message text."}}`,
			expected: "Nested message text.",
			wantErr:  false,
		},
		{
			name:     "plain text response",
			payload:  `This is plain text, not JSON.`,
			expected: "This is plain text, not JSON.",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractAssistantContent([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestExtractAssistantContent_ErrorCases tests error handling.
func TestExtractAssistantContent_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "empty payload",
			payload: "",
		},
		{
			name:    "empty JSON object",
			payload: `{}`,
		},
		{
			name:    "empty choices array",
			payload: `{"choices":[]}`,
		},
		{
			name:    "empty content in OpenAI",
			payload: `{"choices":[{"message":{"content":""}}]}`,
		},
		{
			name:    "empty candidates array",
			payload: `{"candidates":[]}`,
		},
		{
			name:    "null values",
			payload: `{"choices":null,"candidates":null,"content":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractAssistantContent([]byte(tt.payload))
			if err == nil && result == "" {
				// Empty result is also acceptable for these cases
				return
			}
			if err == nil && result != "" {
				t.Errorf("Expected error or empty result, got '%s'", result)
			}
		})
	}
}

// TestSummaryModelFallbackExecutor tests the model fallback logic.
func TestSummaryModelFallbackExecutor(t *testing.T) {
	t.Run("uses default model when model is empty", func(t *testing.T) {
		mockAuth := &mockAuthExecutor{
			response: []byte(`{"choices":[{"message":{"content":"Response with default model."}}]}`),
			err:      nil,
		}
		base := NewPipelineSummarizerExecutor(mockAuth, []string{"test"})
		executor := NewSummaryModelFallbackExecutor(base)

		result, err := executor.Summarize(context.Background(), "", "test prompt")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != "Response with default model." {
			t.Errorf("Expected 'Response with default model.', got '%s'", result)
		}

		// Check that the request used DefaultSummaryModel
		req := mockAuth.lastReq.(ExecutorRequest)
		if req.Model != DefaultSummaryModel {
			t.Errorf("Expected model '%s', got '%s'", DefaultSummaryModel, req.Model)
		}
	})

	t.Run("falls back on model not found error", func(t *testing.T) {
		// Create a mock that fails first, then succeeds
		callCount := 0
		mockAuth := &mockAuthExecutorWithCallback{
			callback: func(ctx context.Context, providers []string, req interface{}, opts interface{}) ([]byte, error) {
				callCount++
				if callCount == 1 {
					return nil, errors.New("model not found: gemini-3-flash")
				}
				return []byte(`{"choices":[{"message":{"content":"Fallback success."}}]}`), nil
			},
		}

		base := NewPipelineSummarizerExecutor(mockAuth, []string{"test"})
		executor := NewSummaryModelFallbackExecutor(base)

		result, err := executor.Summarize(context.Background(), DefaultSummaryModel, "test prompt")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != "Fallback success." {
			t.Errorf("Expected 'Fallback success.', got '%s'", result)
		}
		if callCount != 2 {
			t.Errorf("Expected 2 calls (primary + fallback), got %d", callCount)
		}
	})

	t.Run("does not fallback on non-model errors", func(t *testing.T) {
		mockAuth := &mockAuthExecutor{
			response: nil,
			err:      errors.New("rate limit exceeded"),
		}
		base := NewPipelineSummarizerExecutor(mockAuth, []string{"test"})
		executor := NewSummaryModelFallbackExecutor(base)

		_, err := executor.Summarize(context.Background(), DefaultSummaryModel, "test prompt")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !containsSubstring(err.Error(), "rate limit") {
			t.Errorf("Expected error to contain 'rate limit', got '%s'", err.Error())
		}
	})
}

// TestIsModelNotFoundError tests the model not found error detection.
func TestIsModelNotFoundError(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected bool
	}{
		{"model not found: gemini-3-flash", true},
		{"Model_not_found", true},
		{"unknown model specified", true},
		{"unsupported model: test", true},
		{"invalid model name", true},
		{"model does not exist", true},
		{"rate limit exceeded", false},
		{"authentication failed", false},
		{"quota exceeded", false},
		{"internal server error", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = errors.New(tt.errMsg)
			}
			result := isModelNotFoundError(err)
			if result != tt.expected {
				t.Errorf("isModelNotFoundError(%q) = %v, want %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}
