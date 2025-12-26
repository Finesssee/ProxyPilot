package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
// message content from an OpenAI-compatible response format.
func extractAssistantContent(payload []byte) (string, error) {
	if len(payload) == 0 {
		return "", errors.New("empty response payload")
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	content := response.Choices[0].Message.Content
	if content == "" {
		return "", errors.New("empty content in response")
	}

	return content, nil
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
