package test

import (
	"testing"

	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"

	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestOpenAIToClaude_ToolResultsAsSeparateMessages(t *testing.T) {
	// The translator produces separate messages for each tool result.
	// Grouping is handled later by NormalizeClaudeToolResults in the executor.
	raw := []byte(`{
  "model":"gpt-5.2",
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
		"claude-sonnet-4-5-20250929",
		raw,
		false,
	)

	// Translator produces 5 messages: user, assistant (with 2 tool_use), user (tool_result a), user (tool_result b), user (done)
	if got := gjson.GetBytes(out, "messages.#").Int(); got != 5 {
		t.Fatalf("expected 5 messages, got %d body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "messages.1.role").String() != "assistant" {
		t.Fatalf("expected messages.1.role=assistant body=%s", string(out))
	}
	if got := gjson.GetBytes(out, "messages.1.content.#").Int(); got != 2 {
		t.Fatalf("expected 2 tool_use blocks, got %d body=%s", got, string(out))
	}
	// Tool results are in separate user messages
	if gjson.GetBytes(out, "messages.2.role").String() != "user" {
		t.Fatalf("expected messages.2.role=user body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.2.content.0.tool_use_id").String() != "call_a" {
		t.Fatalf("expected messages.2 tool_use_id=call_a body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.3.role").String() != "user" {
		t.Fatalf("expected messages.3.role=user body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.3.content.0.tool_use_id").String() != "call_b" {
		t.Fatalf("expected messages.3 tool_use_id=call_b body=%s", string(out))
	}
}

func TestOpenAIResponsesToClaude_CallOutputsAsSeparateMessages(t *testing.T) {
	// The translator produces separate messages for each function_call and function_call_output.
	// Grouping is handled later by NormalizeClaudeToolResults in the executor.
	raw := []byte(`{
  "model":"gpt-5.2",
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
		"claude-sonnet-4-5-20250929",
		raw,
		false,
	)

	// Translator produces 6 messages: user (hi), assistant (tool_use a), assistant (tool_use b),
	// user (tool_result a), user (tool_result b), user (done)
	if got := gjson.GetBytes(out, "messages.#").Int(); got != 6 {
		t.Fatalf("expected 6 messages, got %d body=%s", got, string(out))
	}
	// First message is user "hi"
	if gjson.GetBytes(out, "messages.0.role").String() != "user" {
		t.Fatalf("expected messages.0.role=user body=%s", string(out))
	}
	// Function calls become separate assistant messages with tool_use
	if gjson.GetBytes(out, "messages.1.role").String() != "assistant" {
		t.Fatalf("expected messages.1.role=assistant body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.1.content.0.type").String() != "tool_use" {
		t.Fatalf("expected messages.1.content.0.type=tool_use body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.2.role").String() != "assistant" {
		t.Fatalf("expected messages.2.role=assistant body=%s", string(out))
	}
	// Function call outputs become separate user messages with tool_result
	if gjson.GetBytes(out, "messages.3.role").String() != "user" {
		t.Fatalf("expected messages.3.role=user body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.3.content.0.tool_use_id").String() != "call_a" {
		t.Fatalf("expected messages.3 tool_use_id=call_a body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.4.role").String() != "user" {
		t.Fatalf("expected messages.4.role=user body=%s", string(out))
	}
	if gjson.GetBytes(out, "messages.4.content.0.tool_use_id").String() != "call_b" {
		t.Fatalf("expected messages.4 tool_use_id=call_b body=%s", string(out))
	}
}
