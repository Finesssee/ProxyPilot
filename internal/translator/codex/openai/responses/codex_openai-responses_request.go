package responses

import (
	"bytes"

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

	return rawJSON
}
