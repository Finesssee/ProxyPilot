package responses

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertOpenAIResponsesRequestToGemini_LeavesNonJSONToolOutputAsString(t *testing.T) {
	in := []byte(`{
  "model":"gemini-claude-opus-4-5-thinking",
  "input":[
    {"type":"function_call","call_id":"call_a","name":"Read","arguments":"{}"},
    {"type":"function_call_output","call_id":"call_a","output":"[1,2,3]\n\n[Process exited with code 0]"}
  ]
}`)

	out := ConvertOpenAIResponsesRequestToGemini("gemini-claude-opus-4-5-thinking", in, true)
	if !gjson.ValidBytes(out) {
		t.Fatalf("expected valid JSON output, got=%s", string(out))
	}
	got := gjson.GetBytes(out, "contents.1.parts.0.functionResponse.response.result")
	if got.Type != gjson.String {
		t.Fatalf("expected tool output to remain a string, got type=%v value=%s", got.Type, got.Raw)
	}
}

