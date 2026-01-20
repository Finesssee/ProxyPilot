package responses

import (
	"bytes"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func ConvertOpenAIResponsesRequestToCodex(modelName string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := bytes.Clone(inputRawJSON)
	userAgent := misc.ExtractCodexUserAgent(rawJSON)
	rawJSON = misc.StripCodexUserAgent(rawJSON)

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

	originalInstructionsText := ""
	originalInstructionsResult := gjson.GetBytes(rawJSON, "instructions")
	if originalInstructionsResult.Exists() {
		originalInstructionsText = originalInstructionsResult.String()
	}

	// The chatgpt.com Codex backend requires a specific instruction prefix (Codex CLI prompt).
	// Always provide the official Codex instructions for the target model, and move any caller
	// instructions into a system message in the conversation history.
	hasOfficialInstructions, instructions := misc.CodexInstructionsForModel(modelName, originalInstructionsResult.String(), userAgent)
	rawJSON, _ = sjson.SetBytes(rawJSON, "instructions", instructions)

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

	// Preserve caller instructions by converting them into an explicit leading user message inside "input".
	// Note: Codex backend rejects role=system in input ("System messages are not allowed").
	// Only add the original instructions if we replaced them with official instructions.
	if !hasOfficialInstructions && strings.TrimSpace(originalInstructionsText) != "" {
		sys := `{"type":"message","role":"user","content":[{"type":"input_text","text":""}]}`
		sys, _ = sjson.Set(sys, "content.0.text", originalInstructionsText)

		newInput := "[]"
		newInput, _ = sjson.SetRaw(newInput, "-1", sys)
		for _, item := range inputResults {
			newInput, _ = sjson.SetRaw(newInput, "-1", item.Raw)
		}
		rawJSON, _ = sjson.SetRawBytes(rawJSON, "input", []byte(newInput))
	}
	return rawJSON
}
