// Package responses provides response translation functionality for Kiro to OpenAI Responses API compatibility.
// It converts Claude-format responses (which Kiro uses internally) to OpenAI Responses API format.
// The executor handles AWS Event Stream parsing and extracts Claude-format events, which this translator converts to OpenAI Responses format.
package responses

import (
	"context"

	clauderesponses "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/claude/openai/responses"
)

// ConvertKiroResponseToOpenAIResponses converts Kiro streaming response format to OpenAI Responses API format.
// The Kiro executor parses AWS Event Stream and extracts Claude-format SSE events,
// which this function then converts to OpenAI Responses streaming format.
//
// Parameters:
//   - ctx: The context for the request
//   - modelName: The name of the model being used for the response
//   - originalRequestRawJSON: The original request JSON before any translation
//   - requestRawJSON: The translated request JSON sent to the upstream (Claude format)
//   - rawJSON: The Claude-format SSE event data extracted from Kiro's AWS Event Stream
//   - param: A pointer to a parameter object for maintaining state between calls
//
// Returns:
//   - []string: A slice of strings, each containing an OpenAI Responses-compatible JSON response chunk
func ConvertKiroResponseToOpenAIResponses(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	// The executor has already extracted Claude-format events from AWS Event Stream
	// Now convert those Claude events to OpenAI Responses format
	return clauderesponses.ConvertClaudeResponseToOpenAIResponses(ctx, modelName, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// ConvertKiroResponseToOpenAIResponsesNonStream converts a non-streaming Kiro response to a non-streaming OpenAI Responses response.
// The Kiro executor parses AWS Event Stream and builds a Claude-format response,
// which this function then converts to OpenAI Responses format.
//
// Parameters:
//   - ctx: The context for the request
//   - modelName: The name of the model being used for the response
//   - originalRequestRawJSON: The original request JSON before any translation
//   - requestRawJSON: The translated request JSON sent to the upstream (Claude format)
//   - rawJSON: The Claude-format response data built from Kiro's AWS Event Stream
//   - param: A pointer to a parameter object for the conversion
//
// Returns:
//   - string: An OpenAI Responses-compatible JSON response
func ConvertKiroResponseToOpenAIResponsesNonStream(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	// The executor has already built a Claude-format response from AWS Event Stream
	// Now convert that Claude response to OpenAI Responses format
	return clauderesponses.ConvertClaudeResponseToOpenAIResponsesNonStream(ctx, modelName, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}
