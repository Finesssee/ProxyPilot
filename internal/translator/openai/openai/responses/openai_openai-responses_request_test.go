package responses

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertOpenAIResponsesRequestToOpenAIChatCompletions_PreservesDeveloperRole(t *testing.T) {
	in := []byte(`{
  "model":"gpt-5",
  "input":[
    {
      "type":"message",
      "role":"developer",
      "content":[{"type":"input_text","text":"stay scoped"}]
    },
    {
      "type":"message",
      "role":"user",
      "content":[{"type":"input_text","text":"hi"}]
    }
  ]
}`)

	out := ConvertOpenAIResponsesRequestToOpenAIChatCompletions("gpt-5", in, false)

	if got := gjson.GetBytes(out, "messages.0.role").String(); got != "developer" {
		t.Fatalf("expected first message role to remain developer, got %q body=%s", got, string(out))
	}
	if got := gjson.GetBytes(out, "messages.1.role").String(); got != "user" {
		t.Fatalf("expected second message role to remain user, got %q body=%s", got, string(out))
	}
}
