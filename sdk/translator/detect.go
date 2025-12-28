package translator

import (
	"strings"

	"github.com/tidwall/gjson"
)

// DetectFormat attempts to detect the format of a JSON payload by examining its structure.
// Returns empty Format if the format cannot be determined.
func DetectFormat(payload []byte) Format {
	if len(payload) == 0 {
		return ""
	}

	// Parse the JSON once
	result := gjson.ParseBytes(payload)
	if !result.IsObject() {
		return ""
	}

	// Check for Antigravity format: has "request.contents" nested path
	if result.Get("request.contents").Exists() {
		return FormatAntigravity
	}

	// Check for Gemini format: has "contents" + "generationConfig"
	if result.Get("contents").Exists() && result.Get("generationConfig").Exists() {
		return FormatGemini
	}

	// Check for Gemini format (alternative): has "contents" array with "parts"
	if contents := result.Get("contents"); contents.Exists() && contents.IsArray() {
		if len(contents.Array()) > 0 {
			firstContent := contents.Array()[0]
			if firstContent.Get("parts").Exists() {
				return FormatGemini
			}
		}
	}

	// Check for OpenAI Response format: has "input" + "instructions"
	if result.Get("input").Exists() && result.Get("instructions").Exists() {
		return FormatOpenAIResponse
	}

	// Check for messages-based formats (Claude, OpenAI)
	if result.Get("messages").Exists() && result.Get("model").Exists() {
		model := result.Get("model").String()
		modelLower := strings.ToLower(model)

		// Check for Claude-specific patterns
		// Claude uses: max_tokens (required), system (string or array), anthropic_version
		if result.Get("anthropic_version").Exists() {
			return FormatClaude
		}

		// Check model name patterns for Claude
		if strings.Contains(modelLower, "claude") ||
			strings.Contains(modelLower, "anthropic") {
			return FormatClaude
		}

		// Check for Claude-specific message structure
		// Claude messages can have content as array of objects with "type" field
		messages := result.Get("messages")
		if messages.IsArray() && len(messages.Array()) > 0 {
			firstMsg := messages.Array()[0]
			content := firstMsg.Get("content")
			if content.IsArray() && len(content.Array()) > 0 {
				firstContent := content.Array()[0]
				// Claude uses "type" with values like "text", "image", "tool_use", "tool_result"
				contentType := firstContent.Get("type").String()
				if contentType == "tool_use" || contentType == "tool_result" {
					return FormatClaude
				}
			}
		}

		// Check for OpenAI-specific patterns
		// OpenAI uses: n, presence_penalty, frequency_penalty, logprobs, response_format
		if result.Get("n").Exists() ||
			result.Get("presence_penalty").Exists() ||
			result.Get("frequency_penalty").Exists() ||
			result.Get("logprobs").Exists() ||
			result.Get("logit_bias").Exists() {
			return FormatOpenAI
		}

		// Check model name patterns for OpenAI
		if strings.Contains(modelLower, "gpt") ||
			strings.Contains(modelLower, "o1") ||
			strings.Contains(modelLower, "o3") ||
			strings.Contains(modelLower, "chatgpt") ||
			strings.Contains(modelLower, "text-davinci") ||
			strings.Contains(modelLower, "davinci") {
			return FormatOpenAI
		}

		// Default to OpenAI format for messages+model structure
		// as it's the most common API format
		return FormatOpenAI
	}

	// Check for Codex format: has "prompt" field (legacy completion API)
	if result.Get("prompt").Exists() && !result.Get("messages").Exists() {
		return FormatCodex
	}

	return ""
}

// DetectFormatWithConfidence returns the detected format along with a confidence score.
type FormatDetection struct {
	Format     Format
	Confidence float64 // 0.0 to 1.0
	Reason     string
}

// DetectFormatDetailed provides more detailed format detection with confidence scoring.
func DetectFormatDetailed(payload []byte) FormatDetection {
	if len(payload) == 0 {
		return FormatDetection{Format: "", Confidence: 0, Reason: "empty payload"}
	}

	result := gjson.ParseBytes(payload)
	if !result.IsObject() {
		return FormatDetection{Format: "", Confidence: 0, Reason: "not a JSON object"}
	}

	// Antigravity: highest confidence due to unique nested structure
	if result.Get("request.contents").Exists() {
		return FormatDetection{
			Format:     FormatAntigravity,
			Confidence: 1.0,
			Reason:     "has request.contents nested path",
		}
	}

	// Gemini with generationConfig: high confidence
	if result.Get("contents").Exists() && result.Get("generationConfig").Exists() {
		return FormatDetection{
			Format:     FormatGemini,
			Confidence: 0.95,
			Reason:     "has contents and generationConfig",
		}
	}

	// OpenAI Response format
	if result.Get("input").Exists() && result.Get("instructions").Exists() {
		return FormatDetection{
			Format:     FormatOpenAIResponse,
			Confidence: 0.95,
			Reason:     "has input and instructions fields",
		}
	}

	// Claude with anthropic_version
	if result.Get("messages").Exists() && result.Get("anthropic_version").Exists() {
		return FormatDetection{
			Format:     FormatClaude,
			Confidence: 1.0,
			Reason:     "has messages and anthropic_version",
		}
	}

	// Messages-based detection
	if result.Get("messages").Exists() && result.Get("model").Exists() {
		model := result.Get("model").String()
		modelLower := strings.ToLower(model)

		if strings.Contains(modelLower, "claude") {
			return FormatDetection{
				Format:     FormatClaude,
				Confidence: 0.9,
				Reason:     "model name contains 'claude'",
			}
		}

		if strings.Contains(modelLower, "gpt") || strings.Contains(modelLower, "o1") {
			return FormatDetection{
				Format:     FormatOpenAI,
				Confidence: 0.9,
				Reason:     "model name matches OpenAI pattern",
			}
		}

		// OpenAI-specific parameters
		if result.Get("n").Exists() || result.Get("presence_penalty").Exists() {
			return FormatDetection{
				Format:     FormatOpenAI,
				Confidence: 0.8,
				Reason:     "has OpenAI-specific parameters",
			}
		}

		// Default to OpenAI with lower confidence
		return FormatDetection{
			Format:     FormatOpenAI,
			Confidence: 0.5,
			Reason:     "has messages and model (defaulting to OpenAI)",
		}
	}

	// Gemini with just contents
	if result.Get("contents").Exists() {
		return FormatDetection{
			Format:     FormatGemini,
			Confidence: 0.7,
			Reason:     "has contents field",
		}
	}

	return FormatDetection{Format: "", Confidence: 0, Reason: "no matching format patterns"}
}

// MustDetectFormat detects format or panics if detection fails.
func MustDetectFormat(payload []byte) Format {
	f := DetectFormat(payload)
	if f == "" {
		panic("translator: unable to detect format from payload")
	}
	return f
}

// IsKnownFormat checks if the given format is a known/registered format.
func IsKnownFormat(f Format) bool {
	switch f {
	case FormatOpenAI, FormatOpenAIResponse, FormatClaude, FormatGemini, FormatGeminiCLI, FormatCodex, FormatAntigravity, FormatKiro:
		return true
	default:
		return false
	}
}
