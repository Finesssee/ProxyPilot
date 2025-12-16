// Package gemini provides request translation functionality for Gemini CLI to Gemini API compatibility.
// It handles parsing and transforming Gemini CLI API requests into Gemini API format,
// extracting model information, system instructions, message contents, and tool declarations.
// The package performs JSON data transformation to ensure compatibility
// between Gemini CLI API format and Gemini API's expected format.
package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/common"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertGeminiRequestToAntigravity parses and transforms a Gemini CLI API request into Gemini API format.
// It extracts the model name, system instruction, message contents, and tool declarations
// from the raw JSON request and returns them in the format expected by the Gemini API.
// The function performs the following transformations:
// 1. Extracts the model information from the request
// 2. Restructures the JSON to match Gemini API format
// 3. Converts system instructions to the expected format
// 4. Fixes CLI tool response format and grouping
//
// Parameters:
//   - modelName: The name of the model to use for the request (unused in current implementation)
//   - rawJSON: The raw JSON request data from the Gemini CLI API
//   - stream: A boolean indicating if the request is for a streaming response (unused in current implementation)
//
// Returns:
//   - []byte: The transformed request data in Gemini API format
func ConvertGeminiRequestToAntigravity(_ string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := bytes.Clone(inputRawJSON)
	template := ""
	template = `{"project":"","request":{},"model":""}`
	template, _ = sjson.SetRaw(template, "request", string(rawJSON))
	template, _ = sjson.Set(template, "model", gjson.Get(template, "request.model").String())
	template, _ = sjson.Delete(template, "request.model")

	template, errFixCLIToolResponse := fixCLIToolResponse(template)
	if errFixCLIToolResponse != nil {
		// Fail open: tool-call normalization is a best-effort resilience feature.
		// Returning an empty request here causes hard failures upstream.
	}

	systemInstructionResult := gjson.Get(template, "request.system_instruction")
	if systemInstructionResult.Exists() {
		template, _ = sjson.SetRaw(template, "request.systemInstruction", systemInstructionResult.Raw)
		template, _ = sjson.Delete(template, "request.system_instruction")
	}
	rawJSON = []byte(template)

	// Normalize roles in request.contents: default to valid values if missing/invalid
	contents := gjson.GetBytes(rawJSON, "request.contents")
	if contents.Exists() {
		prevRole := ""
		idx := 0
		contents.ForEach(func(_ gjson.Result, value gjson.Result) bool {
			role := value.Get("role").String()
			valid := role == "user" || role == "model"
			if role == "" || !valid {
				var newRole string
				if prevRole == "" {
					newRole = "user"
				} else if prevRole == "user" {
					newRole = "model"
				} else {
					newRole = "user"
				}
				path := fmt.Sprintf("request.contents.%d.role", idx)
				rawJSON, _ = sjson.SetBytes(rawJSON, path, newRole)
				role = newRole
			}
			prevRole = role
			idx++
			return true
		})
	}

	toolsResult := gjson.GetBytes(rawJSON, "request.tools")
	if toolsResult.Exists() && toolsResult.IsArray() {
		toolResults := toolsResult.Array()
		for i := 0; i < len(toolResults); i++ {
			functionDeclarationsResult := gjson.GetBytes(rawJSON, fmt.Sprintf("request.tools.%d.function_declarations", i))
			if functionDeclarationsResult.Exists() && functionDeclarationsResult.IsArray() {
				functionDeclarationsResults := functionDeclarationsResult.Array()
				for j := 0; j < len(functionDeclarationsResults); j++ {
					parametersResult := gjson.GetBytes(rawJSON, fmt.Sprintf("request.tools.%d.function_declarations.%d.parameters", i, j))
					if parametersResult.Exists() {
						strJson, _ := util.RenameKey(string(rawJSON), fmt.Sprintf("request.tools.%d.function_declarations.%d.parameters", i, j), fmt.Sprintf("request.tools.%d.function_declarations.%d.parametersJsonSchema", i, j))
						rawJSON = []byte(strJson)
					}
				}
			}
		}
	}

	gjson.GetBytes(rawJSON, "request.contents").ForEach(func(key, content gjson.Result) bool {
		if content.Get("role").String() == "model" {
			content.Get("parts").ForEach(func(partKey, part gjson.Result) bool {
				if part.Get("functionCall").Exists() {
					rawJSON, _ = sjson.SetBytes(rawJSON, fmt.Sprintf("request.contents.%d.parts.%d.thoughtSignature", key.Int(), partKey.Int()), "skip_thought_signature_validator")
				} else if part.Get("thoughtSignature").Exists() {
					rawJSON, _ = sjson.SetBytes(rawJSON, fmt.Sprintf("request.contents.%d.parts.%d.thoughtSignature", key.Int(), partKey.Int()), "skip_thought_signature_validator")
				}
				return true
			})
		}
		return true
	})

	return common.AttachDefaultSafetySettings(rawJSON, "request.safetySettings")
}

// fixCLIToolResponse normalizes Gemini CLI-style tool calling so that any model content containing
// one or more `functionCall` parts is immediately followed by a user content containing the matching
// `functionResponse` parts (matched by `id` when present).
//
// This is required for downstream Claude-style validation that enforces tool result adjacency, and
// prevents orphan tool calls that would otherwise yield 400s.
func fixCLIToolResponse(input string) (string, error) {
	// Normalize to valid UTF-8 so encoding/json can safely parse even if upstream/client
	// accidentally emits invalid bytes inside JSON strings.
	inBytes := []byte(input)
	if !utf8.Valid(inBytes) {
		inBytes = bytes.ToValidUTF8(inBytes, []byte("\uFFFD"))
	}

	var root map[string]any
	if err := json.Unmarshal(inBytes, &root); err != nil {
		return input, nil
	}
	req, _ := root["request"].(map[string]any)
	if req == nil {
		return input, nil
	}
	rawContents, _ := req["contents"].([]any)
	if rawContents == nil {
		return input, nil
	}

	type callInfo struct {
		id   string
		name string
	}

	// Collect all functionCall ids (so we only relocate matching responses).
	callIDs := make(map[string]struct{})
	for _, cAny := range rawContents {
		c, _ := cAny.(map[string]any)
		if c == nil {
			continue
		}
		parts, _ := c["parts"].([]any)
		for _, pAny := range parts {
			p, _ := pAny.(map[string]any)
			if p == nil {
				continue
			}
			fc, _ := p["functionCall"].(map[string]any)
			if fc == nil {
				continue
			}
			if id, _ := fc["id"].(string); id != "" {
				callIDs[id] = struct{}{}
			}
		}
	}

	// Index functionResponse parts by id (keep in encountered order).
	responsesByID := make(map[string][]map[string]any)
	for _, cAny := range rawContents {
		c, _ := cAny.(map[string]any)
		if c == nil {
			continue
		}
		parts, _ := c["parts"].([]any)
		for _, pAny := range parts {
			p, _ := pAny.(map[string]any)
			if p == nil {
				continue
			}
			fr, _ := p["functionResponse"].(map[string]any)
			if fr == nil {
				continue
			}
			id, _ := fr["id"].(string)
			if id == "" {
				continue
			}
			if _, ok := callIDs[id]; !ok {
				continue
			}
			responsesByID[id] = append(responsesByID[id], p)
		}
	}

	// Remove the indexed response parts from their original locations so we don't duplicate them.
	for idx := 0; idx < len(rawContents); idx++ {
		c, _ := rawContents[idx].(map[string]any)
		if c == nil {
			continue
		}
		parts, _ := c["parts"].([]any)
		if len(parts) == 0 {
			continue
		}
		kept := make([]any, 0, len(parts))
		for _, pAny := range parts {
			p, _ := pAny.(map[string]any)
			fr, _ := p["functionResponse"].(map[string]any)
			if fr != nil {
				id, _ := fr["id"].(string)
				if id != "" {
					if _, ok := callIDs[id]; ok {
						continue // relocated
					}
				}
			}
			kept = append(kept, pAny)
		}
		if len(kept) == 0 {
			// Drop content entries that were only tool responses.
			delete(c, "parts")
			rawContents[idx] = c
		} else {
			c["parts"] = kept
			rawContents[idx] = c
		}
	}

	// Build new contents, inserting a tool-response user message after each model functionCall turn.
	outContents := make([]any, 0, len(rawContents))
	for _, cAny := range rawContents {
		c, _ := cAny.(map[string]any)
		if c == nil {
			continue
		}
		partsAny, _ := c["parts"].([]any)
		if partsAny == nil {
			// Empty content (e.g. removed response-only). Skip.
			continue
		}

		outContents = append(outContents, c)

		if c["role"] != "model" {
			continue
		}

		// Extract ordered functionCall parts in this model content.
		calls := make([]callInfo, 0, 4)
		for _, pAny := range partsAny {
			p, _ := pAny.(map[string]any)
			if p == nil {
				continue
			}
			fc, _ := p["functionCall"].(map[string]any)
			if fc == nil {
				continue
			}
			id, _ := fc["id"].(string)
			name, _ := fc["name"].(string)
			if id == "" {
				continue
			}
			calls = append(calls, callInfo{id: id, name: name})
		}
		if len(calls) == 0 {
			continue
		}

		respParts := make([]any, 0, len(calls))
		for _, call := range calls {
			if bucket := responsesByID[call.id]; len(bucket) > 0 {
				respParts = append(respParts, bucket[0])
				responsesByID[call.id] = bucket[1:]
				continue
			}
			// Fallback: synthesize a placeholder result so upstream validators are satisfied.
			respParts = append(respParts, map[string]any{
				"functionResponse": map[string]any{
					"id":   call.id,
					"name": call.name,
					"response": map[string]any{
						"result": fmt.Sprintf("tool_result missing for %s", call.id),
					},
				},
			})
		}

		outContents = append(outContents, map[string]any{
			"role":  "user",
			"parts": respParts,
		})
	}

	req["contents"] = outContents
	root["request"] = req
	updated, err := json.Marshal(root)
	if err != nil {
		return input, err
	}
	return string(updated), nil
}
