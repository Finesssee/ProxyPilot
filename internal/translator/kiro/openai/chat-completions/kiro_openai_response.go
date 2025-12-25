// Package chat_completions provides response translation functionality for Kiro to OpenAI Chat Completions API compatibility.
// It converts Claude-format responses (which Kiro uses internally) to OpenAI Chat Completions format.
// The executor handles AWS Event Stream parsing and extracts Claude-format events, which this translator converts to OpenAI format.
package chat_completions

import (
	"context"
	"fmt"

	claudechatcompletions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/claude/openai/chat-completions"
)

// ConvertKiroResponseToOpenAI converts Kiro streaming response format to OpenAI Chat Completions API format.
// The Kiro executor parses AWS Event Stream and extracts Claude-format SSE events,
// which this function then converts to OpenAI streaming format.
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
//   - []string: A slice of strings, each containing an OpenAI-compatible JSON response chunk
func ConvertKiroResponseToOpenAI(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	// The executor has already extracted Claude-format events from AWS Event Stream
	// Now convert those Claude events to OpenAI format
	return claudechatcompletions.ConvertClaudeResponseToOpenAI(ctx, modelName, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// ConvertKiroResponseToOpenAINonStream converts a non-streaming Kiro response to a non-streaming OpenAI response.
// The Kiro executor parses AWS Event Stream and builds a Claude-format response,
// which this function then converts to OpenAI format.
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
//   - string: An OpenAI-compatible JSON response
func ConvertKiroResponseToOpenAINonStream(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	// The executor has already built a Claude-format response from AWS Event Stream
	// Now convert that Claude response to OpenAI format
	return claudechatcompletions.ConvertClaudeResponseToOpenAINonStream(ctx, modelName, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// OpenAITokenCount returns the token count in OpenAI format.
func OpenAITokenCount(ctx context.Context, count int64) string {
	return fmt.Sprintf(`{"prompt_tokens":%d}`, count)
}
