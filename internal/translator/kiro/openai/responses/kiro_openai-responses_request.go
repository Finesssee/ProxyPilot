// Package responses provides request translation functionality for OpenAI Responses API to Kiro compatibility.
// It converts OpenAI Responses API requests to Claude format which Kiro uses internally.
// The actual wrapping in Kiro payload structure is handled by the executor.
package responses

import (
	"bytes"

	clauderesponses "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/claude/openai/responses"
)

// ConvertOpenAIResponsesRequestToKiro converts an OpenAI Responses API request to Kiro-compatible format.
// Since Kiro uses Claude format internally, this delegates to the OpenAI Responses->Claude converter.
// The executor will then wrap the Claude-format request in Kiro's payload structure.
//
// Parameters:
//   - modelName: The name of the model to use for the request
//   - rawJSON: The raw JSON request data from the OpenAI Responses API
//   - stream: A boolean indicating if the request is for a streaming response
//
// Returns:
//   - []byte: The request data in Claude API format (ready for Kiro executor to wrap)
func ConvertOpenAIResponsesRequestToKiro(modelName string, rawJSON []byte, stream bool) []byte {
	// Convert OpenAI Responses format to Claude format first
	// The executor will handle wrapping this in Kiro's payload structure
	return clauderesponses.ConvertOpenAIResponsesRequestToClaude(modelName, bytes.Clone(rawJSON), stream)
}
