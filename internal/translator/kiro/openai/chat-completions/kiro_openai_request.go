// Package chat_completions provides request translation functionality for OpenAI to Kiro API compatibility.
// It converts OpenAI Chat Completions API requests to Claude format which Kiro uses internally.
// The actual wrapping in Kiro payload structure is handled by the executor.
package chat_completions

import (
	"bytes"

	claudechatcompletions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/claude/openai/chat-completions"
)

// ConvertOpenAIRequestToKiro converts an OpenAI Chat Completions API request to Kiro-compatible format.
// Since Kiro uses Claude format internally, this delegates to the OpenAI->Claude converter.
// The executor will then wrap the Claude-format request in Kiro's payload structure.
//
// Parameters:
//   - modelName: The name of the model to use for the request
//   - rawJSON: The raw JSON request data from the OpenAI API
//   - stream: A boolean indicating if the request is for a streaming response
//
// Returns:
//   - []byte: The request data in Claude API format (ready for Kiro executor to wrap)
func ConvertOpenAIRequestToKiro(modelName string, rawJSON []byte, stream bool) []byte {
	// Convert OpenAI to Claude format first
	// The executor will handle wrapping this in Kiro's payload structure
	return claudechatcompletions.ConvertOpenAIRequestToClaude(modelName, bytes.Clone(rawJSON), stream)
}
