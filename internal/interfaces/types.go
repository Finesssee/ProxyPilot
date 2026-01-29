// Package interfaces provides type aliases for backwards compatibility with translator functions.
// It defines common interface types used throughout the CLI Proxy API for request and response
// transformation operations, maintaining compatibility with the SDK translator package.
package interfaces

import (
	"time"

	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

// Backwards compatible aliases for translator function types.
type TranslateRequestFunc = sdktranslator.RequestTransform

type TranslateResponseFunc = sdktranslator.ResponseStreamTransform

type TranslateResponseNonStreamFunc = sdktranslator.ResponseNonStreamTransform

type TranslateResponse = sdktranslator.ResponseTransform

// RequestLogEntry represents a single request log entry for request history tracking.
type RequestLogEntry struct {
	// ID is a unique identifier for this entry.
	ID string `json:"id"`
	// Timestamp is when the request was made.
	Timestamp time.Time `json:"timestamp"`
	// Model is the AI model used for this request.
	Model string `json:"model"`
	// Provider is the backend provider handling the request.
	Provider string `json:"provider"`
	// Status is the HTTP status code of the response.
	Status int `json:"status"`
	// InputTokens is the number of input tokens consumed.
	InputTokens int `json:"input_tokens"`
	// OutputTokens is the number of output tokens generated.
	OutputTokens int `json:"output_tokens"`
	// Error contains any error message if the request failed.
	Error string `json:"error,omitempty"`
	// DurationMS is the request duration in milliseconds.
	DurationMS int64 `json:"duration_ms"`
}
