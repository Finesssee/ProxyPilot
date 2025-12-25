// Package claude provides response translation functionality for Kiro to Claude API compatibility.
// Since Kiro uses Claude-compatible format internally and the executor extracts Claude-format events from AWS Event Stream,
// this translator acts as a pass-through for the already-formatted Claude responses.
package claude

import (
	"context"
	"fmt"
)

// ConvertKiroResponseToClaude converts Kiro streaming response format to Claude API format.
// The Kiro executor parses AWS Event Stream and extracts Claude-format SSE events,
// which this function passes through as they're already in the correct format.
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
//   - []string: A slice of strings, each containing a Claude-compatible SSE event
func ConvertKiroResponseToClaude(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	// The executor has already extracted Claude-format events from AWS Event Stream
	// Just pass through as they're already in the correct format
	return []string{string(rawJSON)}
}

// ConvertKiroResponseToClaudeNonStream converts a non-streaming Kiro response to a non-streaming Claude response.
// The Kiro executor parses AWS Event Stream and builds a Claude-format response,
// which this function passes through as it's already in the correct format.
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
//   - string: A Claude-compatible JSON response
func ConvertKiroResponseToClaudeNonStream(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	// The executor has already built a Claude-format response from AWS Event Stream
	// Just pass through as it's already in the correct format
	return string(rawJSON)
}

// ClaudeTokenCount returns the token count in Claude format.
func ClaudeTokenCount(ctx context.Context, count int64) string {
	return fmt.Sprintf(`{"input_tokens":%d}`, count)
}
