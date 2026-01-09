package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

// SummarizerExecutor defines the interface for executing summarization requests
// against the LLM pipeline.
type SummarizerExecutor interface {
	// Summarize sends a summarization prompt to the specified model and returns
	// the assistant's response content.
	Summarize(ctx context.Context, model string, prompt string) (string, error)
}

// AuthExecutor defines the minimal interface required to execute requests
// through the auth manager pipeline. This abstraction avoids import cycles
// with the auth package.
type AuthExecutor interface {
	// Execute routes a request through the configured providers and returns
	// the response payload.
	Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) ([]byte, error)
}

// ExecutorRequest mirrors the executor.Request structure to avoid direct imports.
type ExecutorRequest struct {
	Model    string
	Payload  []byte
	Metadata map[string]any
}

// ExecutorOptions mirrors the executor.Options structure to avoid direct imports.
type ExecutorOptions struct {
	Stream  bool
	Headers http.Header
}

// PipelineSummarizerExecutor implements SummarizerExecutor by routing requests
// through the existing auth manager pipeline.
type PipelineSummarizerExecutor struct {
	authManager AuthExecutor
	providers   []string
}

// NewPipelineSummarizerExecutor creates a new PipelineSummarizerExecutor with
// the given auth manager and provider list.
func NewPipelineSummarizerExecutor(authManager AuthExecutor, providers []string) *PipelineSummarizerExecutor {
	if providers == nil {
		providers = []string{}
	}
	return &PipelineSummarizerExecutor{
		authManager: authManager,
		providers:   providers,
	}
}

// Summarize builds an OpenAI-compatible chat completion request and executes it
// through the pipeline, extracting the assistant response content.
func (p *PipelineSummarizerExecutor) Summarize(ctx context.Context, model string, prompt string) (string, error) {
	if p.authManager == nil {
		return "", errors.New("summarizer executor: auth manager not configured")
	}
	if model == "" {
		return "", errors.New("summarizer executor: model not specified")
	}
	if prompt == "" {
		return "", errors.New("summarizer executor: prompt is empty")
	}

	payload := buildSummarizationPayload(model, prompt)

	req := ExecutorRequest{
		Model:   model,
		Payload: payload,
		Metadata: map[string]any{
			"internal": true,
		},
	}

	opts := ExecutorOptions{
		Stream: false,
		Headers: http.Header{
			"X-CLIProxyAPI-Internal": []string{"summarization"},
			"Content-Type":           []string{"application/json"},
		},
	}

	responsePayload, err := p.authManager.Execute(ctx, p.providers, req, opts)
	if err != nil {
		return "", fmt.Errorf("summarizer executor: execution failed: %w", err)
	}

	content, err := extractAssistantContent(responsePayload)
	if err != nil {
		return "", fmt.Errorf("summarizer executor: failed to extract response: %w", err)
	}

	return content, nil
}

// buildSummarizationPayload creates an OpenAI-compatible chat completion request payload.
func buildSummarizationPayload(model, prompt string) []byte {
	systemMessage := "You are a context compression assistant. Your task is to summarize conversation history while preserving key information, decisions, and context that would be useful for continuing the conversation. Be concise but comprehensive."

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemMessage,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"max_tokens":  2000,
		"temperature": 0.3,
	}

	data, _ := json.Marshal(payload)
	return data
}

// extractAssistantContent parses the response payload and extracts the assistant's
// message content from multiple API response formats (OpenAI, Claude, Gemini).
func extractAssistantContent(payload []byte) (string, error) {
	if len(payload) == 0 {
		return "", errors.New("empty response payload")
	}

	// Try OpenAI format first (most common after translation)
	if content := tryOpenAIFormat(payload); content != "" {
		return content, nil
	}

	// Try Claude/Anthropic format
	if content := tryClaudeFormat(payload); content != "" {
		return content, nil
	}

	// Try Gemini format
	if content := tryGeminiFormat(payload); content != "" {
		return content, nil
	}

	// Try raw text extraction as last resort
	if content := tryRawTextExtraction(payload); content != "" {
		return content, nil
	}

	return "", errors.New("failed to extract content from response")
}

// tryOpenAIFormat attempts to parse OpenAI chat completion response format.
func tryOpenAIFormat(payload []byte) string {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			// Also check delta for streaming responses
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			// Text field for completions API
			Text string `json:"text"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(payload, &response); err != nil {
		return ""
	}

	if len(response.Choices) == 0 {
		return ""
	}

	choice := response.Choices[0]
	if choice.Message.Content != "" {
		return choice.Message.Content
	}
	if choice.Delta.Content != "" {
		return choice.Delta.Content
	}
	if choice.Text != "" {
		return choice.Text
	}

	return ""
}

// tryClaudeFormat attempts to parse Claude/Anthropic Messages API response format.
func tryClaudeFormat(payload []byte) string {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		// Also check completion field for legacy format
		Completion string `json:"completion"`
	}

	if err := json.Unmarshal(payload, &response); err != nil {
		return ""
	}

	// Check content blocks (modern format)
	for _, block := range response.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text
		}
	}

	// Check legacy completion field
	if response.Completion != "" {
		return response.Completion
	}

	return ""
}

// tryGeminiFormat attempts to parse Gemini API response format.
func tryGeminiFormat(payload []byte) string {
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			// Output field for some Gemini responses
			Output string `json:"output"`
		} `json:"candidates"`
		// Text field for simple responses
		Text string `json:"text"`
	}

	if err := json.Unmarshal(payload, &response); err != nil {
		return ""
	}

	// Check candidates array
	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		// Check parts
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				return part.Text
			}
		}
		// Check output field
		if candidate.Output != "" {
			return candidate.Output
		}
	}

	// Check top-level text field
	if response.Text != "" {
		return response.Text
	}

	return ""
}

// tryRawTextExtraction attempts to extract text from unknown response formats.
// It looks for common text-containing fields.
func tryRawTextExtraction(payload []byte) string {
	var generic map[string]interface{}
	if err := json.Unmarshal(payload, &generic); err != nil {
		// If not JSON, check if it's plain text
		text := strings.TrimSpace(string(payload))
		if len(text) > 0 && len(text) < 50000 && !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[") {
			return text
		}
		return ""
	}

	// Check common field names
	for _, key := range []string{"content", "text", "message", "response", "output", "result", "answer"} {
		if val, ok := generic[key]; ok {
			switch v := val.(type) {
			case string:
				if v != "" {
					return v
				}
			case map[string]interface{}:
				// Check nested content/text
				if text, ok := v["text"].(string); ok && text != "" {
					return text
				}
				if content, ok := v["content"].(string); ok && content != "" {
					return content
				}
			}
		}
	}

	return ""
}

// NoOpSummarizerExecutor is a fallback implementation that always returns an error.
// Useful for testing or when summarization is disabled.
type NoOpSummarizerExecutor struct{}

// NewNoOpSummarizerExecutor creates a new NoOpSummarizerExecutor.
func NewNoOpSummarizerExecutor() *NoOpSummarizerExecutor {
	return &NoOpSummarizerExecutor{}
}

// Summarize always returns an error indicating summarization is not available.
func (n *NoOpSummarizerExecutor) Summarize(ctx context.Context, model string, prompt string) (string, error) {
	return "", errors.New("summarization not available: no executor configured")
}

// DefaultSummaryModel is the primary model for context summarization.
const DefaultSummaryModel = "gemini-3-flash"

// FallbackSummaryModel is used when the primary model is unavailable.
const FallbackSummaryModel = "gemini-3-flash-preview"

// SummaryModelFallbackExecutor wraps a SummarizerExecutor and implements
// model fallback: tries DefaultSummaryModel first, falls back to FallbackSummaryModel
// only on "model not found/unsupported" errors.
type SummaryModelFallbackExecutor struct {
	delegate SummarizerExecutor
}

// NewSummaryModelFallbackExecutor creates an executor with model fallback logic.
func NewSummaryModelFallbackExecutor(delegate SummarizerExecutor) *SummaryModelFallbackExecutor {
	return &SummaryModelFallbackExecutor{delegate: delegate}
}

// Summarize tries the primary model, falls back to preview on model-not-found errors.
func (e *SummaryModelFallbackExecutor) Summarize(ctx context.Context, model string, prompt string) (string, error) {
	if e.delegate == nil {
		return "", errors.New("summarization not available: no delegate executor")
	}

	// Use override model if provided, otherwise use default
	primaryModel := model
	if primaryModel == "" {
		primaryModel = DefaultSummaryModel
	}

	result, err := e.delegate.Summarize(ctx, primaryModel, prompt)
	if err == nil {
		return result, nil
	}

	// Check if error indicates model not found/unsupported
	if isModelNotFoundError(err) {
		// Try fallback model
		fallbackModel := FallbackSummaryModel
		if primaryModel == FallbackSummaryModel {
			// Already tried fallback, don't loop
			return "", err
		}
		return e.delegate.Summarize(ctx, fallbackModel, prompt)
	}

	// Other errors (auth, quota, transient) - don't switch models
	return "", err
}

// isModelNotFoundError checks if the error indicates the model is unavailable.
func isModelNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "model not found") ||
		strings.Contains(msg, "model_not_found") ||
		strings.Contains(msg, "unknown model") ||
		strings.Contains(msg, "unsupported model") ||
		strings.Contains(msg, "invalid model") ||
		strings.Contains(msg, "does not exist")
}

// CoreManagerExecutor defines the minimal interface for executing requests through
// the core auth manager. This matches coreauth.Manager.Execute signature.
type CoreManagerExecutor interface {
	Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) (interface{}, error)
}

// ManagerAuthAdapter wraps a CoreManagerExecutor to implement the AuthExecutor interface.
// This adapter bridges the type differences between coreauth.Manager and the memory package.
type ManagerAuthAdapter struct {
	manager CoreManagerExecutor
}

// NewManagerAuthAdapter creates an adapter that wraps the given manager.
// The manager must implement the CoreManagerExecutor interface (compatible with coreauth.Manager).
func NewManagerAuthAdapter(manager CoreManagerExecutor) *ManagerAuthAdapter {
	return &ManagerAuthAdapter{manager: manager}
}

// Execute implements AuthExecutor by delegating to the wrapped manager and extracting
// the response payload.
func (a *ManagerAuthAdapter) Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) ([]byte, error) {
	if a.manager == nil {
		return nil, errors.New("manager auth adapter: manager not configured")
	}

	resp, err := a.manager.Execute(ctx, providers, req, opts)
	if err != nil {
		return nil, err
	}

	// Handle nil response
	if resp == nil {
		return nil, errors.New("manager auth adapter: nil response from manager")
	}

	// Extract payload from response - try common response types
	switch r := resp.(type) {
	case []byte:
		return r, nil
	case cliproxyexecutor.Response:
		return r.Payload, nil
	case *cliproxyexecutor.Response:
		if r != nil {
			return r.Payload, nil
		}
		return nil, errors.New("manager auth adapter: nil response")
	case interface{ GetPayload() []byte }:
		return r.GetPayload(), nil
	default:
		// Try to access Payload field via reflection-like approach using JSON
		// This handles cliproxyexecutor.Response which has a Payload []byte field
		if data, err := json.Marshal(resp); err == nil {
			var wrapper struct {
				Payload []byte `json:"payload"`
			}
			if json.Unmarshal(data, &wrapper) == nil && wrapper.Payload != nil {
				return wrapper.Payload, nil
			}
			// If no payload field, return the serialized response
			return data, nil
		}
		return nil, fmt.Errorf("manager auth adapter: unsupported response type %T", resp)
	}
}
