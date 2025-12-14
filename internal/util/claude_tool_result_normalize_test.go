package util

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestNormalizeClaudeToolResults_InsertsToolResultMessageImmediatelyAfterToolUse(t *testing.T) {
	in := []byte(`{
  "model":"gemini-claude-opus-4-5-thinking",
  "max_tokens":64,
  "messages":[
    {"role":"user","content":"hi"},
    {"role":"assistant","content":[{"type":"tool_use","id":"call_a","name":"a","input":{}}]},
    {"role":"user","content":"(oops user text before tool_result)"},
    {"role":"user","content":[{"type":"tool_result","tool_use_id":"call_a","content":"outA"}]},
    {"role":"user","content":"continue"}
  ]
}`)

	out := NormalizeClaudeToolResults(in)

	if got := gjson.GetBytes(out, "messages.#").Int(); got != 5 {
		t.Fatalf("expected 5 messages after normalization, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.2.role").String() != "user" {
		t.Fatalf("expected messages.2.role=user body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.2.content.0.type").String() != "tool_result" {
		t.Fatalf("expected messages.2.content[0].type=tool_result body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.2.content.0.tool_use_id").String() != "call_a" {
		t.Fatalf("expected messages.2.content[0].tool_use_id=call_a body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.3.content").String() != "(oops user text before tool_result)" {
		t.Fatalf("expected messages.3.content to retain user text body=%s", string(out))
	}
}
