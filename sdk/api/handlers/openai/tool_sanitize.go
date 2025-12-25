package openai

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func tightenToolSchemas(raw []byte, isResponses bool) []byte {
	tools := gjson.GetBytes(raw, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return raw
	}

	out := raw
	for i, t := range tools.Array() {
		if t.Get("type").String() != "function" {
			continue
		}
		var params gjson.Result
		if isResponses {
			params = t.Get("parameters")
		} else {
			params = t.Get("function.parameters")
		}
		if !params.Exists() {
			continue
		}
		if params.Get("type").String() != "object" {
			continue
		}
		if params.Get("additionalProperties").Exists() {
			continue
		}
		path := ""
		if isResponses {
			path = "tools." + itoa(i) + ".parameters.additionalProperties"
		} else {
			path = "tools." + itoa(i) + ".function.parameters.additionalProperties"
		}
		updated, err := sjson.SetBytes(out, path, false)
		if err == nil {
			out = updated
		}
	}

	// Normalize nullable types like ["string", "null"] -> "string"
	// Vertex AI / Antigravity doesn't accept array-style nullable type definitions
	out = []byte(normalizeNullableTypes(string(out)))

	return out
}

func sanitizeToolCallArguments(resp []byte, req []byte, isResponses bool) []byte {
	allowed := allowedToolArgs(req, isResponses)
	if len(allowed) == 0 {
		return resp
	}

	if isResponses {
		return sanitizeResponsesToolArgs(resp, allowed)
	}
	return sanitizeChatToolArgs(resp, allowed)
}

func allowedToolArgs(req []byte, isResponses bool) map[string]map[string]struct{} {
	tools := gjson.GetBytes(req, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}
	out := make(map[string]map[string]struct{})
	for _, t := range tools.Array() {
		if t.Get("type").String() != "function" {
			continue
		}
		name := ""
		var props gjson.Result
		if isResponses {
			name = t.Get("name").String()
			props = t.Get("parameters.properties")
		} else {
			name = t.Get("function.name").String()
			props = t.Get("function.parameters.properties")
		}
		if name == "" || !props.Exists() || !props.IsObject() {
			continue
		}
		keys := make(map[string]struct{})
		props.ForEach(func(k, _ gjson.Result) bool {
			if k.String() != "" {
				keys[k.String()] = struct{}{}
			}
			return true
		})
		if len(keys) > 0 {
			out[name] = keys
		}
	}
	return out
}

func sanitizeChatToolArgs(resp []byte, allowed map[string]map[string]struct{}) []byte {
	out := resp
	choices := gjson.GetBytes(out, "choices")
	if !choices.Exists() || !choices.IsArray() {
		return out
	}
	for ci := range choices.Array() {
		toolCalls := gjson.GetBytes(out, "choices."+itoa(ci)+".message.tool_calls")
		if !toolCalls.Exists() || !toolCalls.IsArray() {
			continue
		}
		for ti := range toolCalls.Array() {
			name := gjson.GetBytes(out, "choices."+itoa(ci)+".message.tool_calls."+itoa(ti)+".function.name").String()
			allowedKeys, ok := allowed[name]
			if !ok || len(allowedKeys) == 0 {
				continue
			}
			argsStr := gjson.GetBytes(out, "choices."+itoa(ci)+".message.tool_calls."+itoa(ti)+".function.arguments").String()
			newArgs, ok := filterArgs(argsStr, allowedKeys)
			if !ok {
				continue
			}
			updated, err := sjson.SetBytes(out, "choices."+itoa(ci)+".message.tool_calls."+itoa(ti)+".function.arguments", newArgs)
			if err == nil {
				out = updated
			}
		}
	}
	return out
}

func sanitizeResponsesToolArgs(resp []byte, allowed map[string]map[string]struct{}) []byte {
	out := resp
	output := gjson.GetBytes(out, "output")
	if !output.Exists() || !output.IsArray() {
		return out
	}
	for oi, item := range output.Array() {
		if item.Get("type").String() != "function_call" {
			continue
		}
		name := item.Get("name").String()
		allowedKeys, ok := allowed[name]
		if !ok || len(allowedKeys) == 0 {
			continue
		}
		argsStr := item.Get("arguments").String()
		newArgs, ok := filterArgs(argsStr, allowedKeys)
		if !ok {
			continue
		}
		updated, err := sjson.SetBytes(out, "output."+itoa(oi)+".arguments", newArgs)
		if err == nil {
			out = updated
		}
	}
	return out
}

func filterArgs(args string, allowed map[string]struct{}) (string, bool) {
	if args == "" {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return "", false
	}
	changed := false
	for k := range m {
		if _, ok := allowed[k]; !ok {
			delete(m, k)
			changed = true
		}
	}
	if !changed {
		return args, true
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [32]byte
	pos := len(buf)
	n := i
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// normalizeNullableTypes converts JSON schema array types like ["string", "null"]
// to just the first non-null type ("string"). Vertex AI / Antigravity doesn't
// accept array-style nullable type definitions in function parameter schemas.
func normalizeNullableTypes(jsonStr string) string {
	// Find all paths to "type" fields using a simple walk
	paths := make([]string, 0)
	walkForField(gjson.Parse(jsonStr), "", "type", &paths)

	for _, p := range paths {
		value := gjson.Get(jsonStr, p)
		if !value.IsArray() {
			continue
		}

		// Extract the first non-null type from the array
		var primaryType string
		for _, item := range value.Array() {
			typeVal := item.String()
			// Check for both lowercase and uppercase null
			if typeVal != "null" && typeVal != "NULL" {
				primaryType = typeVal
				break
			}
		}

		// If we found a primary type, replace the array with just that type
		if primaryType != "" {
			jsonStr, _ = sjson.Set(jsonStr, p, primaryType)
		}
	}

	return jsonStr
}

// walkForField recursively traverses a JSON structure to find all occurrences of a specific field.
func walkForField(value gjson.Result, path, field string, paths *[]string) {
	switch value.Type {
	case gjson.JSON:
		value.ForEach(func(key, val gjson.Result) bool {
			var childPath string
			if path == "" {
				childPath = key.String()
			} else {
				childPath = path + "." + key.String()
			}
			if key.String() == field {
				*paths = append(*paths, childPath)
			}
			walkForField(val, childPath, field, paths)
			return true
		})
	}
}

// truncateResponsesInput truncates the 'input' array in OpenAI Responses API format
// to prevent "Prompt is too long" errors. This is called early in the handler before
// translation to Antigravity format.
func truncateResponsesInput(raw []byte, model string) []byte {
	input := gjson.GetBytes(raw, "input")
	if !input.Exists() || !input.IsArray() {
		return raw
	}

	arr := input.Array()
	if len(arr) <= 2 {
		return raw
	}

	// Estimate tokens: ~3 chars per token for Claude, conservative
	payloadLen := len(raw)
	charsPerToken := 3
	estimatedTokens := payloadLen / charsPerToken

	// Use much more conservative limits to ensure we stay well within context bounds
	// Claude models have 200k context but tools/overhead consume significant space
	// Use 80k for Claude (40% of context), 60k for others to be safe
	limit := 60000
	if containsClaude(model) {
		limit = 80000
	}

	log.Printf("[truncateResponsesInput] model=%s, payloadLen=%d, estimatedTokens=%d, limit=%d, inputMsgs=%d",
		model, payloadLen, estimatedTokens, limit, len(arr))

	if estimatedTokens <= limit {
		log.Printf("[truncateResponsesInput] No truncation needed")
		return raw
	}

	log.Printf("[truncateResponsesInput] Truncation NEEDED: %d tokens > %d limit", estimatedTokens, limit)

	// Need to truncate - keep last N messages that fit

	// Calculate how much to keep
	totalInputChars := len(input.Raw)
	baseOverhead := payloadLen - totalInputChars // Non-input overhead (tools, model, etc)

	// Force keep index 0 (system prompt usually) if array has elements
	if len(arr) > 0 {
		baseOverhead += len(arr[0].Raw) // Reserve space for system prompt
	}

	// Find how many messages to keep from the end, stopping at index 1
	keepIdx := 0
	currentChars := 0

	for i := len(arr) - 1; i >= 1; i-- {
		msgLen := len(arr[i].Raw)
		newTotal := baseOverhead + currentChars + msgLen
		newEstTokens := newTotal / charsPerToken

		if newEstTokens > limit {
			keepIdx = i + 1
			break
		}
		currentChars += msgLen
	}

	// If loop finished without breaking, keepIdx stays 0.
	// We want to verify if we kept enough.
	if keepIdx <= 1 {
		// Means we can keep everything from 1..end.
		// We still need to reconstruct to include 0 if we were just keeping everything.
		// But earlier we returned execution if no truncation needed.
		// So if we are here, we MUST have needed truncation, or just marginally.
		// Whatever, reconstruction is safe.
		keepIdx = 1
	}

	log.Printf("[truncateResponsesInput] keepIdx=%d, dropping %d middle messages, keeping system+last %d",
		keepIdx, keepIdx-1, len(arr)-keepIdx)

	// Rebuild input array: arr[0] + arr[keepIdx:]
	var newInputParts []string
	if len(arr) > 0 {
		newInputParts = append(newInputParts, arr[0].Raw)
	}
	for i := keepIdx; i < len(arr); i++ {
		newInputParts = append(newInputParts, arr[i].Raw)
	}
	newInputJSON := "[" + joinStrings(newInputParts, ",") + "]"

	updated, err := sjson.SetRawBytes(raw, "input", []byte(newInputJSON))
	if err != nil {
		return raw
	}
	return updated
}

func containsClaude(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "claude")
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
