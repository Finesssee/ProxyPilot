package test

import (
	"testing"

	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"

	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestOpenAIToClaude_GroupsToolResultsInSingleMessage(t *testing.T) {
	raw := []byte(`{
  "model":"gpt-5",
  "messages":[
    {"role":"user","content":"hi"},
    {"role":"assistant","tool_calls":[
      {"id":"call_a","type":"function","function":{"name":"a","arguments":"{}"}},
      {"id":"call_b","type":"function","function":{"name":"b","arguments":"{}"}}
    ]},
    {"role":"tool","tool_call_id":"call_a","content":"outA"},
    {"role":"tool","tool_call_id":"call_b","content":"outB"},
    {"role":"user","content":"done"}
  ]
}`)

	out := sdktranslator.TranslateRequest(
		sdktranslator.FromString("openai"),
		sdktranslator.FromString("claude"),
		"claude-3-7-sonnet-20250219",
		raw,
		false,
	)

	if got := gjson.GetBytes(out, "messages.#").Int(); got != 4 {
		t.Fatalf("expected 4 messages, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.1.role").String() != "assistant" {
		t.Fatalf("expected messages.1.role=assistant body=%s", string(out))
	}
	if got := gjson.GetBytes(out, "messages.1.content.#").Int(); got != 2 {
		t.Fatalf("expected 2 tool_use blocks, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.2.role").String() != "user" {
		t.Fatalf("expected messages.2.role=user body=%s", string(out))
	}
	if got := gjson.GetBytes(out, "messages.2.content.#").Int(); got != 2 {
		t.Fatalf("expected 2 tool_result blocks, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.2.content.0.tool_use_id").String() != "call_a" {
		t.Fatalf("expected first tool_use_id=call_a body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.2.content.1.tool_use_id").String() != "call_b" {
		t.Fatalf("expected second tool_use_id=call_b body=%s", string(out))
	}
}

func TestOpenAIResponsesToClaude_GroupsCallOutputsInSingleMessage(t *testing.T) {
	raw := []byte(`{
  "model":"gpt-5",
  "input":[
    {"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
    {"type":"function_call","call_id":"call_a","name":"a","arguments":"{}"},
    {"type":"function_call","call_id":"call_b","name":"b","arguments":"{}"},
    {"type":"function_call_output","call_id":"call_a","output":"outA"},
    {"type":"function_call_output","call_id":"call_b","output":"outB"},
    {"type":"message","role":"user","content":[{"type":"input_text","text":"done"}]}
  ]
}`)

	out := sdktranslator.TranslateRequest(
		sdktranslator.FromString("openai-response"),
		sdktranslator.FromString("claude"),
		"claude-3-7-sonnet-20250219",
		raw,
		false,
	)

	if got := gjson.GetBytes(out, "messages.#").Int(); got != 4 {
		t.Fatalf("expected 4 messages, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.1.role").String() != "assistant" {
		t.Fatalf("expected messages.1.role=assistant body=%s", string(out))
	}
	if got := gjson.GetBytes(out, "messages.1.content.#").Int(); got != 2 {
		t.Fatalf("expected 2 tool_use blocks, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.2.role").String() != "user" {
		t.Fatalf("expected messages.2.role=user body=%s", string(out))
	}
	if got := gjson.GetBytes(out, "messages.2.content.#").Int(); got != 2 {
		t.Fatalf("expected 2 tool_result blocks, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.2.content.0.tool_use_id").String() != "call_a" {
		t.Fatalf("expected first tool_use_id=call_a body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.2.content.1.tool_use_id").String() != "call_b" {
		t.Fatalf("expected second tool_use_id=call_b body=%s", string(out))
	}
}
