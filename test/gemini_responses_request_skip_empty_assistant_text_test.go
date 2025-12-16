package test

import (
	"testing"

	geminiresponses "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/openai/responses"
	"github.com/tidwall/gjson"
)

func TestGeminiResponsesRequest_SkipsEmptyAssistantTextMessage(t *testing.T) {
	req := []byte(`{
  "model":"gemini-claude-sonnet-4-5-thinking",
  "input":[
    {"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
    {"type":"message","role":"assistant","content":[{"type":"output_text","text":""}]},
    {"type":"function_call","call_id":"call_1","name":"shell","arguments":"{\"command\":[\"powershell.exe\",\"-Command\",\"echo hi\"]}"},
    {"type":"function_call_output","call_id":"call_1","output":"hi"},
    {"type":"message","role":"user","content":[{"type":"input_text","text":"ok"}]}
  ],
  "tools":[{"type":"function","name":"shell","description":"Runs a Powershell command","parameters":{"type":"object","properties":{"command":{"type":"array","items":{"type":"string"}}},"required":["command"]}}],
  "tool_choice":"auto"
}`)

	out := geminiresponses.ConvertOpenAIResponsesRequestToGemini("gemini-claude-sonnet-4-5-thinking", req, false)
	contents := gjson.GetBytes(out, "contents").Array()

	for _, c := range contents {
		if c.Get("role").String() != "model" {
			continue
		}
		parts := c.Get("parts").Array()
		if len(parts) == 1 && parts[0].Get("text").Exists() && parts[0].Get("text").String() == "" && !parts[0].Get("functionCall").Exists() {
			t.Fatalf("found unexpected empty model text part in contents: %s", c.Raw)
		}
	}
}
