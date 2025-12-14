package util

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIResponsesToolOrder_PullsOutputsImmediatelyAfterCalls(t *testing.T) {
	in := []byte(`{
  "model":"x",
  "input":[
    {"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
    {"type":"function_call","call_id":"call_a","name":"a","arguments":"{}"},
    {"type":"function_call","call_id":"call_b","name":"b","arguments":"{}"},
    {"type":"message","role":"user","content":[{"type":"input_text","text":"(interleaved)"}]},
    {"type":"function_call_output","call_id":"call_b","output":"outB"},
    {"type":"function_call_output","call_id":"call_a","output":"outA"},
    {"type":"message","role":"user","content":[{"type":"input_text","text":"after"}]}
  ]
}`)

	out := NormalizeOpenAIResponsesToolOrder(in)

	if got := gjson.GetBytes(out, "input.#").Int(); got != 7 {
		t.Fatalf("expected 7 items, got %d body=%s", got, string(out))
	}
	// Calls stay in place.
	if gjson.GetBytes(out, "input.1.type").String() != "function_call" || gjson.GetBytes(out, "input.2.type").String() != "function_call" {
		t.Fatalf("expected function_call items at 1 and 2 body=%s", string(out))
	}
	// Outputs are pulled right after calls, preserving relative order among pulled outputs.
	if gjson.GetBytes(out, "input.3.type").String() != "function_call_output" || gjson.GetBytes(out, "input.4.type").String() != "function_call_output" {
		t.Fatalf("expected function_call_output items at 3 and 4 body=%s", string(out))
	}
	if gjson.GetBytes(out, "input.3.call_id").String() != "call_b" || gjson.GetBytes(out, "input.4.call_id").String() != "call_a" {
		t.Fatalf("unexpected output ordering body=%s", string(out))
	}
	// Interleaved message is pushed after the outputs.
	if gjson.GetBytes(out, "input.5.type").String() != "message" {
		t.Fatalf("expected interleaved message at 5 body=%s", string(out))
	}
}

func TestNormalizeOpenAIResponsesToolOrder_DropsOrphanCallsWithoutOutputs(t *testing.T) {
	in := []byte(`{
  "model":"x",
  "input":[
    {"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
    {"type":"function_call","call_id":"call_a","name":"a","arguments":"{}"},
    {"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
  ]
}`)

	out := NormalizeOpenAIResponsesToolOrder(in)
	if got := gjson.GetBytes(out, "input.#").Int(); got != 2 {
		t.Fatalf("expected orphan function_call to be dropped, got %d items body=%s", got, string(out))
	}
	if gjson.GetBytes(out, "input.0.type").String() != "message" || gjson.GetBytes(out, "input.1.type").String() != "message" {
		t.Fatalf("expected only message items to remain body=%s", string(out))
	}
}
