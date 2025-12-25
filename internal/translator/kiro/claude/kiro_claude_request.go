// Package claude provides request translation functionality for Claude to Kiro API compatibility.
// Since Kiro uses Claude-compatible format internally, this translator acts as a pass-through.
// The actual wrapping in Kiro payload structure is handled by the executor.
package claude

import (
	"bytes"
)

// ConvertClaudeRequestToKiro converts a Claude API request to Kiro-compatible format.
// Since Kiro uses Claude format internally, this is essentially a pass-through.
// The executor will handle wrapping this in Kiro's payload structure.
//
// Parameters:
//   - modelName: The name of the model to use for the request
//   - rawJSON: The raw JSON request data from the Claude API
//   - stream: A boolean indicating if the request is for a streaming response
//
// Returns:
//   - []byte: The request data in Claude API format (ready for Kiro executor to wrap)
func ConvertClaudeRequestToKiro(modelName string, rawJSON []byte, stream bool) []byte {
	// Kiro uses Claude-compatible format internally, so we just pass through
	// The executor will handle wrapping this in Kiro's payload structure
	return bytes.Clone(rawJSON)
}
