package responses

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func ConvertOpenAIResponsesRequestToCodex(modelName string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := bytes.Clone(inputRawJSON)

	rawJSON, _ = sjson.SetBytes(rawJSON, "stream", true)
	rawJSON, _ = sjson.SetBytes(rawJSON, "store", false)
	rawJSON, _ = sjson.SetBytes(rawJSON, "parallel_tool_calls", true)
	rawJSON, _ = sjson.SetBytes(rawJSON, "include", []string{"reasoning.encrypted_content"})
	// Codex Responses rejects token limit fields, so strip them out before forwarding.
	rawJSON, _ = sjson.DeleteBytes(rawJSON, "max_output_tokens")
	rawJSON, _ = sjson.DeleteBytes(rawJSON, "max_completion_tokens")
	rawJSON, _ = sjson.DeleteBytes(rawJSON, "temperature")
	rawJSON, _ = sjson.DeleteBytes(rawJSON, "top_p")
	rawJSON, _ = sjson.DeleteBytes(rawJSON, "service_tier")

	originalInstructions := ""
	originalInstructionsText := ""
	originalInstructionsResult := gjson.GetBytes(rawJSON, "instructions")
	if originalInstructionsResult.Exists() {
		originalInstructions = originalInstructionsResult.Raw
		originalInstructionsText = originalInstructionsResult.String()
	}

	hasOfficialInstructions, instructions := misc.CodexInstructionsForModel(modelName, originalInstructionsResult.String())

	inputResult := gjson.GetBytes(rawJSON, "input")
	var inputResults []gjson.Result
	if inputResult.Exists() {
		if inputResult.IsArray() {
			inputResults = inputResult.Array()
		} else if inputResult.Type == gjson.String {
			newInput := `[{"type":"message","role":"user","content":[{"type":"input_text","text":""}]}]`
			newInput, _ = sjson.SetRaw(newInput, "0.content.0.text", inputResult.Raw)
			inputResults = gjson.Parse(newInput).Array()
		}
	} else {
		inputResults = []gjson.Result{}
	}

	extractedSystemInstructions := false
	if originalInstructions == "" && len(inputResults) > 0 {
		for _, item := range inputResults {
			if strings.EqualFold(item.Get("role").String(), "system") {
				var builder strings.Builder
				if content := item.Get("content"); content.Exists() && content.IsArray() {
					content.ForEach(func(_, contentItem gjson.Result) bool {
						text := contentItem.Get("text").String()
						if builder.Len() > 0 && text != "" {
							builder.WriteByte('\n')
						}
						builder.WriteString(text)
						return true
					})
				}
				originalInstructionsText = builder.String()
				originalInstructions = strconv.Quote(originalInstructionsText)
				extractedSystemInstructions = true
				break
			}
		}
	}

	if hasOfficialInstructions {
		return rawJSON
	}

	// If the caller already provided instructions (or a system message we extracted),
	// keep them as-is to avoid duplicating large prompts.
	if strings.TrimSpace(originalInstructionsText) != "" {
		if extractedSystemInstructions && len(inputResults) > 0 {
			// Remove system messages from input since they're now represented via "instructions".
			newInput := "[]"
			for _, item := range inputResults {
				if strings.EqualFold(item.Get("role").String(), "system") {
					continue
				}
				newInput, _ = sjson.SetRaw(newInput, "-1", item.Raw)
			}
			rawJSON, _ = sjson.SetRawBytes(rawJSON, "input", []byte(newInput))
		}
		rawJSON, _ = sjson.SetBytes(rawJSON, "instructions", originalInstructionsText)
		return rawJSON
	}

	// Otherwise, inject standard Codex instructions.
	rawJSON, _ = sjson.SetBytes(rawJSON, "instructions", instructions)
	return rawJSON
}
