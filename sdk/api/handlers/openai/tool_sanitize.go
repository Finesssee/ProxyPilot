package openai

import (
	"encoding/json"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func tightenToolSchemas(raw []byte, isResponses bool) []byte {
	tools := gjson.GetBytes(raw, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return raw
	}

	out := raw
	for i, t := range tools.Array() {
		if t.Get("type").String() != "function" {
			continue
		}
		var params gjson.Result
		if isResponses {
			params = t.Get("parameters")
		} else {
			params = t.Get("function.parameters")
		}
		if !params.Exists() {
			continue
		}
		if params.Get("type").String() != "object" {
			continue
		}
		if params.Get("additionalProperties").Exists() {
			continue
		}
		path := ""
		if isResponses {
			path = "tools." + itoa(i) + ".parameters.additionalProperties"
		} else {
			path = "tools." + itoa(i) + ".function.parameters.additionalProperties"
		}
		updated, err := sjson.SetBytes(out, path, false)
		if err == nil {
			out = updated
		}
	}
	return out
}

func sanitizeToolCallArguments(resp []byte, req []byte, isResponses bool) []byte {
	allowed := allowedToolArgs(req, isResponses)
	if len(allowed) == 0 {
		return resp
	}

	if isResponses {
		return sanitizeResponsesToolArgs(resp, allowed)
	}
	return sanitizeChatToolArgs(resp, allowed)
}

func allowedToolArgs(req []byte, isResponses bool) map[string]map[string]struct{} {
	tools := gjson.GetBytes(req, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}
	out := make(map[string]map[string]struct{})
	for _, t := range tools.Array() {
		if t.Get("type").String() != "function" {
			continue
		}
		name := ""
		var props gjson.Result
		if isResponses {
			name = t.Get("name").String()
			props = t.Get("parameters.properties")
		} else {
			name = t.Get("function.name").String()
			props = t.Get("function.parameters.properties")
		}
		if name == "" || !props.Exists() || !props.IsObject() {
			continue
		}
		keys := make(map[string]struct{})
		props.ForEach(func(k, _ gjson.Result) bool {
			if k.String() != "" {
				keys[k.String()] = struct{}{}
			}
			return true
		})
		if len(keys) > 0 {
			out[name] = keys
		}
	}
	return out
}

func sanitizeChatToolArgs(resp []byte, allowed map[string]map[string]struct{}) []byte {
	out := resp
	choices := gjson.GetBytes(out, "choices")
	if !choices.Exists() || !choices.IsArray() {
		return out
	}
	for ci := range choices.Array() {
		toolCalls := gjson.GetBytes(out, "choices."+itoa(ci)+".message.tool_calls")
		if !toolCalls.Exists() || !toolCalls.IsArray() {
			continue
		}
		for ti := range toolCalls.Array() {
			name := gjson.GetBytes(out, "choices."+itoa(ci)+".message.tool_calls."+itoa(ti)+".function.name").String()
			allowedKeys, ok := allowed[name]
			if !ok || len(allowedKeys) == 0 {
				continue
			}
			argsStr := gjson.GetBytes(out, "choices."+itoa(ci)+".message.tool_calls."+itoa(ti)+".function.arguments").String()
			newArgs, ok := filterArgs(argsStr, allowedKeys)
			if !ok {
				continue
			}
			updated, err := sjson.SetBytes(out, "choices."+itoa(ci)+".message.tool_calls."+itoa(ti)+".function.arguments", newArgs)
			if err == nil {
				out = updated
			}
		}
	}
	return out
}

func sanitizeResponsesToolArgs(resp []byte, allowed map[string]map[string]struct{}) []byte {
	out := resp
	output := gjson.GetBytes(out, "output")
	if !output.Exists() || !output.IsArray() {
		return out
	}
	for oi, item := range output.Array() {
		if item.Get("type").String() != "function_call" {
			continue
		}
		name := item.Get("name").String()
		allowedKeys, ok := allowed[name]
		if !ok || len(allowedKeys) == 0 {
			continue
		}
		argsStr := item.Get("arguments").String()
		newArgs, ok := filterArgs(argsStr, allowedKeys)
		if !ok {
			continue
		}
		updated, err := sjson.SetBytes(out, "output."+itoa(oi)+".arguments", newArgs)
		if err == nil {
			out = updated
		}
	}
	return out
}

func filterArgs(args string, allowed map[string]struct{}) (string, bool) {
	if args == "" {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return "", false
	}
	changed := false
	for k := range m {
		if _, ok := allowed[k]; !ok {
			delete(m, k)
			changed = true
		}
	}
	if !changed {
		return args, true
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [32]byte
	pos := len(buf)
	n := i
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
