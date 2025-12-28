package translator

import (
	"fmt"

	"github.com/tidwall/gjson"
)

// Schema validation errors.
var (
	ErrInvalidSchema     = fmt.Errorf("invalid schema")
	ErrMissingField      = fmt.Errorf("missing required field")
	ErrInvalidFieldType  = fmt.Errorf("invalid field type")
	ErrEmptyPayload      = fmt.Errorf("empty payload")
	ErrInvalidJSON       = fmt.Errorf("invalid JSON")
)

// ValidationResult contains the result of a schema validation.
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// AddError adds an error to the validation result and marks it as invalid.
func (v *ValidationResult) AddError(err string) {
	v.Valid = false
	v.Errors = append(v.Errors, err)
}

// AddWarning adds a warning to the validation result without affecting validity.
func (v *ValidationResult) AddWarning(warn string) {
	v.Warnings = append(v.Warnings, warn)
}

// Error returns a combined error if the validation failed.
func (v *ValidationResult) Error() error {
	if v.Valid {
		return nil
	}
	if len(v.Errors) == 0 {
		return ErrInvalidSchema
	}
	return fmt.Errorf("%w: %v", ErrInvalidSchema, v.Errors)
}

// NewValidationResult creates a new validation result starting as valid.
func NewValidationResult() *ValidationResult {
	return &ValidationResult{Valid: true}
}

// validateJSON checks if the payload is valid JSON and not empty.
func validateJSON(payload []byte) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	if !gjson.ValidBytes(payload) {
		return ErrInvalidJSON
	}
	return nil
}

// checkRequiredField checks if a field exists and optionally its type.
func checkRequiredField(parsed gjson.Result, field string, expectedType ...gjson.Type) (bool, string) {
	value := parsed.Get(field)
	if !value.Exists() {
		return false, fmt.Sprintf("missing required field: %s", field)
	}
	if len(expectedType) > 0 && value.Type != expectedType[0] {
		return false, fmt.Sprintf("field %s has wrong type: expected %v, got %v", field, expectedType[0], value.Type)
	}
	return true, ""
}

// checkOptionalField checks if an optional field, if present, has the correct type.
func checkOptionalField(parsed gjson.Result, field string, expectedType gjson.Type) (bool, string) {
	value := parsed.Get(field)
	if !value.Exists() {
		return true, "" // Optional field not present is OK
	}
	if value.Type != expectedType {
		return false, fmt.Sprintf("field %s has wrong type: expected %v, got %v", field, expectedType, value.Type)
	}
	return true, ""
}

// ValidateGeminiSchema validates that a payload conforms to the Gemini API schema.
// Required fields: contents (array)
// Optional fields: model, generationConfig, safetySettings, tools, systemInstruction
func ValidateGeminiSchema(payload []byte) error {
	if err := validateJSON(payload); err != nil {
		return err
	}

	result := NewValidationResult()
	parsed := gjson.ParseBytes(payload)

	// Check required field: contents
	if ok, errMsg := checkRequiredField(parsed, "contents", gjson.JSON); !ok {
		result.AddError(errMsg)
	} else {
		// Verify contents is an array
		contents := parsed.Get("contents")
		if !contents.IsArray() {
			result.AddError("contents must be an array")
		} else if len(contents.Array()) == 0 {
			result.AddWarning("contents array is empty")
		} else {
			// Validate each content item has required 'parts' field
			for i, content := range contents.Array() {
				if !content.Get("parts").Exists() {
					result.AddError(fmt.Sprintf("contents[%d] missing required field: parts", i))
				}
			}
		}
	}

	// Check optional fields with correct types
	if ok, errMsg := checkOptionalField(parsed, "model", gjson.String); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "generationConfig", gjson.JSON); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "safetySettings", gjson.JSON); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "tools", gjson.JSON); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "systemInstruction", gjson.JSON); !ok {
		result.AddError(errMsg)
	}

	return result.Error()
}

// ValidateClaudeSchema validates that a payload conforms to the Claude/Anthropic API schema.
// Required fields: model, messages (array) or prompt (for legacy)
// Optional fields: max_tokens, temperature, top_p, top_k, stream, system, tools
func ValidateClaudeSchema(payload []byte) error {
	if err := validateJSON(payload); err != nil {
		return err
	}

	result := NewValidationResult()
	parsed := gjson.ParseBytes(payload)

	// Check required field: model
	if ok, errMsg := checkRequiredField(parsed, "model", gjson.String); !ok {
		result.AddError(errMsg)
	}

	// Check messages or prompt (at least one required)
	hasMessages := parsed.Get("messages").Exists()
	hasPrompt := parsed.Get("prompt").Exists()

	if !hasMessages && !hasPrompt {
		result.AddError("missing required field: messages or prompt")
	}

	if hasMessages {
		messages := parsed.Get("messages")
		if !messages.IsArray() {
			result.AddError("messages must be an array")
		} else if len(messages.Array()) == 0 {
			result.AddWarning("messages array is empty")
		} else {
			// Validate each message has required fields
			for i, msg := range messages.Array() {
				if !msg.Get("role").Exists() {
					result.AddError(fmt.Sprintf("messages[%d] missing required field: role", i))
				}
				if !msg.Get("content").Exists() {
					result.AddError(fmt.Sprintf("messages[%d] missing required field: content", i))
				}
			}
		}
	}

	// Check max_tokens - required for Claude API
	if ok, errMsg := checkRequiredField(parsed, "max_tokens", gjson.Number); !ok {
		// max_tokens might be named differently, check alternatives
		if !parsed.Get("max_tokens_to_sample").Exists() {
			result.AddWarning(errMsg + " (may cause API error)")
		}
	}

	// Check optional fields with correct types
	if ok, errMsg := checkOptionalField(parsed, "temperature", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "top_p", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "top_k", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "stream", gjson.True); !ok {
		// stream can also be false
		if parsed.Get("stream").Type != gjson.False {
			result.AddError(errMsg)
		}
	}
	if ok, errMsg := checkOptionalField(parsed, "system", gjson.String); !ok {
		// system can also be an array for Claude
		if parsed.Get("system").Type != gjson.JSON {
			result.AddError(errMsg)
		}
	}
	if ok, errMsg := checkOptionalField(parsed, "tools", gjson.JSON); !ok {
		result.AddError(errMsg)
	}

	return result.Error()
}

// ValidateOpenAISchema validates that a payload conforms to the OpenAI API schema.
// Required fields: model, messages (array)
// Optional fields: max_tokens, temperature, top_p, n, stream, stop, presence_penalty, frequency_penalty, tools, tool_choice
func ValidateOpenAISchema(payload []byte) error {
	if err := validateJSON(payload); err != nil {
		return err
	}

	result := NewValidationResult()
	parsed := gjson.ParseBytes(payload)

	// Check required field: model
	if ok, errMsg := checkRequiredField(parsed, "model", gjson.String); !ok {
		result.AddError(errMsg)
	}

	// Check required field: messages
	if ok, errMsg := checkRequiredField(parsed, "messages", gjson.JSON); !ok {
		result.AddError(errMsg)
	} else {
		messages := parsed.Get("messages")
		if !messages.IsArray() {
			result.AddError("messages must be an array")
		} else if len(messages.Array()) == 0 {
			result.AddWarning("messages array is empty")
		} else {
			// Validate each message has required fields
			for i, msg := range messages.Array() {
				if !msg.Get("role").Exists() {
					result.AddError(fmt.Sprintf("messages[%d] missing required field: role", i))
				}
				// content can be null for assistant messages with tool_calls
				if !msg.Get("content").Exists() && !msg.Get("tool_calls").Exists() {
					result.AddWarning(fmt.Sprintf("messages[%d] has neither content nor tool_calls", i))
				}
			}
		}
	}

	// Check optional fields with correct types
	if ok, errMsg := checkOptionalField(parsed, "max_tokens", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "temperature", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "top_p", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "n", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "presence_penalty", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "frequency_penalty", gjson.Number); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "tools", gjson.JSON); !ok {
		result.AddError(errMsg)
	}

	return result.Error()
}

// ValidateAntigravitySchema validates that a payload conforms to the Antigravity/Vertex AI schema.
// This is similar to Gemini but with some specific differences for Vertex AI.
// Required fields: contents (array)
// Optional fields: generationConfig, safetySettings, tools, systemInstruction
func ValidateAntigravitySchema(payload []byte) error {
	if err := validateJSON(payload); err != nil {
		return err
	}

	result := NewValidationResult()
	parsed := gjson.ParseBytes(payload)

	// Check required field: contents
	if ok, errMsg := checkRequiredField(parsed, "contents", gjson.JSON); !ok {
		result.AddError(errMsg)
	} else {
		contents := parsed.Get("contents")
		if !contents.IsArray() {
			result.AddError("contents must be an array")
		} else if len(contents.Array()) == 0 {
			result.AddWarning("contents array is empty")
		} else {
			// Validate each content item has required 'parts' field
			for i, content := range contents.Array() {
				if !content.Get("parts").Exists() {
					result.AddError(fmt.Sprintf("contents[%d] missing required field: parts", i))
				}
				// Check role field (required for Antigravity)
				if !content.Get("role").Exists() {
					result.AddWarning(fmt.Sprintf("contents[%d] missing role field", i))
				}
			}
		}
	}

	// Check optional fields with correct types
	if ok, errMsg := checkOptionalField(parsed, "generationConfig", gjson.JSON); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "safetySettings", gjson.JSON); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "tools", gjson.JSON); !ok {
		result.AddError(errMsg)
	}
	if ok, errMsg := checkOptionalField(parsed, "systemInstruction", gjson.JSON); !ok {
		result.AddError(errMsg)
	}

	// Antigravity-specific: check for cachedContent field
	if ok, errMsg := checkOptionalField(parsed, "cachedContent", gjson.String); !ok {
		result.AddError(errMsg)
	}

	return result.Error()
}

// ValidateSchema validates a payload against the specified format's schema.
func ValidateSchema(format Format, payload []byte) error {
	switch format {
	case FormatGemini, FormatGeminiCLI:
		return ValidateGeminiSchema(payload)
	case FormatClaude:
		return ValidateClaudeSchema(payload)
	case FormatOpenAI, FormatOpenAIResponse, FormatCodex:
		return ValidateOpenAISchema(payload)
	case FormatAntigravity:
		return ValidateAntigravitySchema(payload)
	default:
		return fmt.Errorf("%w: unknown format %s", ErrInvalidSchema, format)
	}
}

// RoundtripResult contains the result of a roundtrip validation.
type RoundtripResult struct {
	Preserved       bool
	OriginalModel   string
	RoundtripModel  string
	OriginalMsgCnt  int
	RoundtripMsgCnt int
	OriginalToolCnt int
	RoundtripToolCnt int
	Differences     []string
}

// TestRoundtrip translates a payload from format A to format B and back to A,
// then compares critical fields to verify data preservation.
// Returns true if the critical data is preserved, along with detailed comparison results.
func (r *Registry) TestRoundtrip(formatA Format, payload []byte) (bool, error) {
	return r.TestRoundtripTo(formatA, FormatOpenAI, payload)
}

// TestRoundtripTo translates a payload from formatA to formatB and back to formatA,
// comparing critical fields to verify data preservation.
func (r *Registry) TestRoundtripTo(formatA, formatB Format, payload []byte) (bool, error) {
	if err := validateJSON(payload); err != nil {
		return false, err
	}

	// Translate A -> B
	translatedAB := r.TranslateRequest(formatA, formatB, "", payload, false)
	if translatedAB == nil {
		return false, fmt.Errorf("translation %s -> %s failed", formatA, formatB)
	}

	// Translate B -> A
	translatedBA := r.TranslateRequest(formatB, formatA, "", translatedAB, false)
	if translatedBA == nil {
		return false, fmt.Errorf("translation %s -> %s failed", formatB, formatA)
	}

	// Compare critical fields
	result := comparePayloads(formatA, payload, translatedBA)

	return result.Preserved, nil
}

// TestRoundtripDetailed performs a roundtrip test and returns detailed comparison results.
func (r *Registry) TestRoundtripDetailed(formatA, formatB Format, payload []byte) (*RoundtripResult, error) {
	if err := validateJSON(payload); err != nil {
		return nil, err
	}

	// Translate A -> B
	translatedAB := r.TranslateRequest(formatA, formatB, "", payload, false)
	if translatedAB == nil {
		return nil, fmt.Errorf("translation %s -> %s failed", formatA, formatB)
	}

	// Translate B -> A
	translatedBA := r.TranslateRequest(formatB, formatA, "", translatedAB, false)
	if translatedBA == nil {
		return nil, fmt.Errorf("translation %s -> %s failed", formatB, formatA)
	}

	// Compare critical fields
	return comparePayloads(formatA, payload, translatedBA), nil
}

// comparePayloads compares two payloads of the same format and returns detailed results.
func comparePayloads(format Format, original, roundtrip []byte) *RoundtripResult {
	result := &RoundtripResult{Preserved: true}
	origParsed := gjson.ParseBytes(original)
	rtParsed := gjson.ParseBytes(roundtrip)

	switch format {
	case FormatOpenAI, FormatOpenAIResponse, FormatCodex:
		result.OriginalModel = origParsed.Get("model").String()
		result.RoundtripModel = rtParsed.Get("model").String()
		result.OriginalMsgCnt = len(origParsed.Get("messages").Array())
		result.RoundtripMsgCnt = len(rtParsed.Get("messages").Array())
		result.OriginalToolCnt = len(origParsed.Get("tools").Array())
		result.RoundtripToolCnt = len(rtParsed.Get("tools").Array())

	case FormatClaude:
		result.OriginalModel = origParsed.Get("model").String()
		result.RoundtripModel = rtParsed.Get("model").String()
		result.OriginalMsgCnt = len(origParsed.Get("messages").Array())
		result.RoundtripMsgCnt = len(rtParsed.Get("messages").Array())
		result.OriginalToolCnt = len(origParsed.Get("tools").Array())
		result.RoundtripToolCnt = len(rtParsed.Get("tools").Array())

	case FormatGemini, FormatGeminiCLI, FormatAntigravity:
		result.OriginalModel = origParsed.Get("model").String()
		result.RoundtripModel = rtParsed.Get("model").String()
		result.OriginalMsgCnt = len(origParsed.Get("contents").Array())
		result.RoundtripMsgCnt = len(rtParsed.Get("contents").Array())
		result.OriginalToolCnt = len(origParsed.Get("tools").Array())
		result.RoundtripToolCnt = len(rtParsed.Get("tools").Array())
	}

	// Check model preservation
	if result.OriginalModel != result.RoundtripModel {
		result.Preserved = false
		result.Differences = append(result.Differences,
			fmt.Sprintf("model changed: %q -> %q", result.OriginalModel, result.RoundtripModel))
	}

	// Check message/content count preservation
	if result.OriginalMsgCnt != result.RoundtripMsgCnt {
		result.Preserved = false
		result.Differences = append(result.Differences,
			fmt.Sprintf("message count changed: %d -> %d", result.OriginalMsgCnt, result.RoundtripMsgCnt))
	}

	// Check tool count preservation
	if result.OriginalToolCnt != result.RoundtripToolCnt {
		result.Preserved = false
		result.Differences = append(result.Differences,
			fmt.Sprintf("tool count changed: %d -> %d", result.OriginalToolCnt, result.RoundtripToolCnt))
	}

	return result
}

// TestRoundtrip is a helper on the default registry.
func TestRoundtrip(format Format, payload []byte) (bool, error) {
	return defaultRegistry.TestRoundtrip(format, payload)
}

// TestRoundtripTo is a helper on the default registry.
func TestRoundtripTo(formatA, formatB Format, payload []byte) (bool, error) {
	return defaultRegistry.TestRoundtripTo(formatA, formatB, payload)
}

// TestRoundtripDetailed is a helper on the default registry.
func TestRoundtripDetailed(formatA, formatB Format, payload []byte) (*RoundtripResult, error) {
	return defaultRegistry.TestRoundtripDetailed(formatA, formatB, payload)
}
