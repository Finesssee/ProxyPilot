package responses

import (
	"context"
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

func TestConvertOpenAIResponsesRequestToGemini_UsesPreviousResponseCacheForFunctionOutputs(t *testing.T) {
	upstreamResponse := []byte(`{
  "responseId":"gem_prev",
  "candidates":[
    {
      "content":{
        "role":"model",
        "parts":[
          {"functionCall":{"name":"Read","args":{"path":"/tmp/x"}}}
        ]
      }
    }
  ]
}`)

	translated := ConvertGeminiResponseToOpenAIResponsesNonStream(context.Background(), "gemini-test", nil, nil, upstreamResponse, nil)
	callID := gjson.Get(translated, "output.0.call_id").String()
	responseID := gjson.Get(translated, "id").String()
	if callID == "" || responseID == "" {
		t.Fatalf("expected translated response to contain call id and response id, got body=%s", translated)
	}

	in := []byte(`{
  "model":"gemini-test",
  "previous_response_id":"` + responseID + `",
  "input":[
    {"type":"function_call_output","call_id":"` + callID + `","output":"{\"ok\":true}"}
  ]
}`)

	out := ConvertOpenAIResponsesRequestToGemini("gemini-test", in, false)
	if got := gjson.GetBytes(out, "contents.0.parts.0.functionResponse.name").String(); got != "Read" {
		t.Fatalf("expected cached function name Read, got %q body=%s", got, string(out))
	}
	if got := gjson.GetBytes(out, "contents.0.parts.0.functionResponse.response.result.ok").Bool(); !got {
		t.Fatalf("expected JSON tool output to survive conversion, got body=%s", string(out))
	}
}

func TestConvertOpenAIResponsesRequestToGemini_FailsClosedWhenFunctionNameCannotBeResolved(t *testing.T) {
	in := []byte(`{
  "model":"gemini-test",
  "previous_response_id":"resp_missing",
  "input":[
    {"type":"function_call_output","call_id":"call_missing","output":"done"}
  ]
}`)

	out := ConvertOpenAIResponsesRequestToGemini("gemini-test", in, false)
	if got := gjson.GetBytes(out, "error.type").String(); got != "translation_error" {
		t.Fatalf("expected translation_error response, got body=%s", string(out))
	}
	if got := gjson.GetBytes(out, "error.message").String(); got == "" {
		t.Fatalf("expected translation error message, got body=%s", string(out))
	}
	if got := gjson.GetBytes(out, "contents").Exists(); got {
		t.Fatalf("expected fail-closed response without Gemini contents, got body=%s", string(out))
	}
}
