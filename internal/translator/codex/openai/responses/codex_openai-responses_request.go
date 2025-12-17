package responses

import (
	"bytes"
	"strings"

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

	originalInstructionsText := ""
	originalInstructionsResult := gjson.GetBytes(rawJSON, "instructions")
	if originalInstructionsResult.Exists() {
		originalInstructionsText = originalInstructionsResult.String()
	}

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

	// The chatgpt.com Codex backend rejects "instructions" in many cases (400: "Instructions are not valid").
	// Preserve caller instructions by converting them into an explicit system message inside "input",
	// and then delete the top-level "instructions" field.
	if strings.TrimSpace(originalInstructionsText) != "" {
		sys := `{"type":"message","role":"system","content":[{"type":"input_text","text":""}]}`
		sys, _ = sjson.Set(sys, "content.0.text", originalInstructionsText)

		newInput := "[]"
		newInput, _ = sjson.SetRaw(newInput, "-1", sys)
		for _, item := range inputResults {
			newInput, _ = sjson.SetRaw(newInput, "-1", item.Raw)
		}
		rawJSON, _ = sjson.SetRawBytes(rawJSON, "input", []byte(newInput))
	}
	rawJSON, _ = sjson.DeleteBytes(rawJSON, "instructions")

	// modelName is part of the fixed method signature
	_ = modelName
	return rawJSON
}
