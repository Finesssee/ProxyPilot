package openai

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type toolCallTag struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func convertToolCallTagsToResponsesFunctionCalls(resp []byte) []byte {
	// If upstream already produced structured calls, don't touch it.
	if gjson.GetBytes(resp, `output.#(type=="function_call")`).Exists() {
		return resp
	}
	if gjson.GetBytes(resp, `output.#(type=="tool_call")`).Exists() {
		return resp
	}

	text := gjson.GetBytes(resp, `output.#(type=="message").content.#(type=="output_text").text`).String()
	if text == "" {
		text = gjson.GetBytes(resp, "output_text").String()
	}
	name, argsJSON, ok := extractToolCallTag(text)
	if !ok || name == "" || argsJSON == "" {
		return resp
	}

	callID := "call_" + itoa(int(time.Now().UnixNano()))
	item := map[string]any{
		"type":      "function_call",
		"name":      name,
		"arguments": argsJSON,
		"call_id":   callID,
		"id":        "fc_" + callID,
		"status":    "completed",
	}

	out, err := sjson.SetBytes(resp, "output", []any{item})
	if err != nil {
		return resp
	}
	// Keep output_text minimal so tag-based tool runners don't double-trigger.
	out, _ = sjson.SetBytes(out, "output_text", "")
	return out
}

func extractToolCallTag(text string) (name string, args string, ok bool) {
	start := strings.Index(text, "<tool_call>")
	if start < 0 {
		return "", "", false
	}
	end := strings.Index(text[start:], "</tool_call>")
	if end < 0 {
		return "", "", false
	}
	inner := text[start+len("<tool_call>") : start+end]
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return "", "", false
	}
	var tc toolCallTag
	if err := json.Unmarshal([]byte(inner), &tc); err != nil {
		return "", "", false
	}
	tc.Name = strings.TrimSpace(tc.Name)
	if tc.Name == "" {
		return "", "", false
	}
	argsRaw := strings.TrimSpace(string(tc.Arguments))
	if argsRaw == "" || argsRaw == "null" {
		argsRaw = "{}"
	}
	if !gjson.Valid(argsRaw) {
		// Arguments must be JSON; fall back to empty object.
		argsRaw = "{}"
	}
	return tc.Name, argsRaw, true
}
