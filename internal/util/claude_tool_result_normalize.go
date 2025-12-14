package util

import (
	"encoding/json"
	"strings"
)

// NormalizeClaudeToolResults ensures Claude Messages-style payloads satisfy the constraint that
// tool_result blocks for an assistant tool_use turn appear in the immediately following message.
//
// This is primarily a resilience feature for clients that may insert user text messages between
// tool_use and tool_result (or send tool_result blocks later). When possible, it moves matching
// tool_result blocks into a tool_result-only user message directly after the corresponding assistant.
func NormalizeClaudeToolResults(body []byte) []byte {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}
	rawMsgs, ok := root["messages"].([]any)
	if !ok || len(rawMsgs) == 0 {
		return body
	}

	changed := false

	for i := 0; i < len(rawMsgs); i++ {
		msg, ok := rawMsgs[i].(map[string]any)
		if !ok {
			continue
		}
		if getClaudeRole(msg) != "assistant" {
			continue
		}

		toolUseIDs := collectClaudeToolUseIDs(msg)
		if len(toolUseIDs) == 0 {
			continue
		}

		changedBeforeTurn := changed

		insertAt := i + 1
		if insertAt > len(rawMsgs) {
			insertAt = len(rawMsgs)
		}

		var toolResultMsg map[string]any
		insertedThisTurn := false
		if insertAt < len(rawMsgs) {
			if candidate, ok := rawMsgs[insertAt].(map[string]any); ok && isClaudeUserToolResultOnly(candidate) {
				toolResultMsg = candidate
			}
		}
		if toolResultMsg == nil {
			toolResultMsg = map[string]any{"role": "user", "content": []any{}}
			rawMsgs = append(rawMsgs, nil)
			copy(rawMsgs[insertAt+1:], rawMsgs[insertAt:])
			rawMsgs[insertAt] = toolResultMsg
			insertedThisTurn = true
			changed = true
			i++
		}

		j := insertAt + 1
		for j < len(rawMsgs) {
			userMsg, ok := rawMsgs[j].(map[string]any)
			if !ok || getClaudeRole(userMsg) != "user" {
				j++
				continue
			}

			moved, kept, okParts := splitClaudeToolResults(userMsg, toolUseIDs)
			if !okParts || len(moved) == 0 {
				j++
				continue
			}

			appendClaudeToolResults(toolResultMsg, moved)
			if len(kept) == 0 {
				rawMsgs = append(rawMsgs[:j], rawMsgs[j+1:]...)
				changed = true
				continue
			}
			userMsg["content"] = kept
			rawMsgs[j] = userMsg
			changed = true
			j++
		}

		if insertedThisTurn {
			if content, ok := toolResultMsg["content"].([]any); ok && len(content) == 0 {
				if insertAt < len(rawMsgs) {
					rawMsgs = append(rawMsgs[:insertAt], rawMsgs[insertAt+1:]...)
					changed = changedBeforeTurn
					i--
				}
			}
		}
	}

	if !changed {
		return body
	}

	root["messages"] = rawMsgs
	updated, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return updated
}

func getClaudeRole(msg map[string]any) string {
	if msg == nil {
		return ""
	}
	if v, ok := msg["role"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func collectClaudeToolUseIDs(msg map[string]any) map[string]struct{} {
	out := make(map[string]struct{})
	if msg == nil {
		return out
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return out
	}
	for _, partAny := range content {
		part, ok := partAny.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := part["type"].(string); typ != "tool_use" {
			continue
		}
		if id, _ := part["id"].(string); strings.TrimSpace(id) != "" {
			out[strings.TrimSpace(id)] = struct{}{}
		}
	}
	return out
}

func isClaudeUserToolResultOnly(msg map[string]any) bool {
	if getClaudeRole(msg) != "user" {
		return false
	}
	content, ok := msg["content"].([]any)
	if !ok || len(content) == 0 {
		return false
	}
	for _, partAny := range content {
		part, ok := partAny.(map[string]any)
		if !ok {
			return false
		}
		if typ, _ := part["type"].(string); typ != "tool_result" {
			return false
		}
	}
	return true
}

func splitClaudeToolResults(msg map[string]any, ids map[string]struct{}) (moved []any, kept []any, ok bool) {
	content, ok := msg["content"].([]any)
	if !ok {
		return nil, nil, false
	}
	for _, partAny := range content {
		part, okPart := partAny.(map[string]any)
		if !okPart {
			kept = append(kept, partAny)
			continue
		}
		typ, _ := part["type"].(string)
		if typ != "tool_result" {
			kept = append(kept, partAny)
			continue
		}
		id, _ := part["tool_use_id"].(string)
		id = strings.TrimSpace(id)
		if id != "" {
			if _, match := ids[id]; match {
				moved = append(moved, partAny)
				continue
			}
		}
		kept = append(kept, partAny)
	}
	return moved, kept, true
}

func appendClaudeToolResults(msg map[string]any, parts []any) {
	if msg == nil || len(parts) == 0 {
		return
	}
	content, ok := msg["content"].([]any)
	if !ok {
		content = []any{}
	}
	content = append(content, parts...)
	msg["content"] = content
}
