package util

import (
	"encoding/json"
	"strings"
)

// NormalizeOpenAIResponsesToolOrder reorders OpenAI Responses API `input` items so that any
// `function_call_output` items for a run of `function_call` items are pulled immediately after
// that run. This helps downstream providers that require tool outputs to immediately follow tool calls.
func NormalizeOpenAIResponsesToolOrder(body []byte) []byte {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}

	rawInput, ok := root["input"].([]any)
	if !ok || len(rawInput) == 0 {
		return body
	}

	skip := make([]bool, len(rawInput))
	out := make([]any, 0, len(rawInput))
	changed := false

	itemType := func(item any) string {
		obj, ok := item.(map[string]any)
		if !ok {
			return ""
		}
		if t, ok := obj["type"].(string); ok && strings.TrimSpace(t) != "" {
			return strings.TrimSpace(t)
		}
		if r, ok := obj["role"].(string); ok && strings.TrimSpace(r) != "" {
			return "message"
		}
		return ""
	}

	callIDOf := func(item any) string {
		obj, ok := item.(map[string]any)
		if !ok {
			return ""
		}
		if v, ok := obj["call_id"].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}

	for i := 0; i < len(rawInput); i++ {
		if skip[i] {
			continue
		}

		if itemType(rawInput[i]) != "function_call" {
			out = append(out, rawInput[i])
			skip[i] = true
			continue
		}

		callItems := make([]any, 0, 4)
		callIDs := make(map[string]struct{})
		callIDByIdx := make(map[int]string)
		j := i
		for j < len(rawInput) {
			if skip[j] {
				j++
				continue
			}
			if itemType(rawInput[j]) != "function_call" {
				break
			}
			callItems = append(callItems, rawInput[j])
			skip[j] = true
			if id := callIDOf(rawInput[j]); id != "" {
				callIDs[id] = struct{}{}
				callIDByIdx[len(callItems)-1] = id
			}
			j++
		}

		foundOutputs := make(map[string]struct{})
		pulledOutputs := make([]any, 0, 4)
		k := j
		for k < len(rawInput) {
			if skip[k] {
				k++
				continue
			}
			if itemType(rawInput[k]) == "function_call_output" {
				if id := callIDOf(rawInput[k]); id != "" {
					if _, ok := callIDs[id]; ok {
						pulledOutputs = append(pulledOutputs, rawInput[k])
						skip[k] = true
						changed = true
						foundOutputs[id] = struct{}{}
					}
				}
			}
			k++
		}

		// Emit only function_call items that have a matching output somewhere later in the request.
		// This prevents downstream Claude-style validation failures when clients accidentally send
		// orphan tool calls without including their tool outputs.
		for idx, item := range callItems {
			id, hasID := callIDByIdx[idx]
			if hasID {
				if _, ok := foundOutputs[id]; !ok {
					changed = true
					continue
				}
			}
			out = append(out, item)
		}
		out = append(out, pulledOutputs...)

		i = j - 1
	}

	if !changed {
		return body
	}

	root["input"] = out
	updated, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return updated
}
