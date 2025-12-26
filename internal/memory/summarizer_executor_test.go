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
