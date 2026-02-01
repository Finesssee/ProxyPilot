package middleware

import (
	"strconv"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// tokenAwareCompressionResult contains the result of token-aware analysis
type tokenAwareCompressionResult struct {
	ShouldTrim     bool   // Whether trimming is needed
	CurrentTokens  int64  // Estimated current token count
	ContextWindow  int    // Model's context window
	TargetTokens   int64  // Target token count after trimming
	TargetMaxBytes int    // Approximate bytes to achieve target tokens
	Model          string // The model name
}

// analyzeTokenBudget checks if the request exceeds the token threshold and calculates trimming targets.
func analyzeTokenBudget(body []byte) *tokenAwareCompressionResult {
	result := &tokenAwareCompressionResult{
		ShouldTrim:     false,
		TargetMaxBytes: agenticMaxBodyBytes(),
	}

	if !agenticTokenAwareEnabled() {
		return result
	}

	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return result
	}
	result.Model = model

	contextWindow := getModelContextWindow(model)
	result.ContextWindow = contextWindow

	currentTokens, err := executor.EstimateRequestTokens(model, body)
	if err != nil {
		currentTokens = int64(len(body) / 4)
	}
	result.CurrentTokens = currentTokens

	reserveTokens := agenticReserveTokens()
	availableContext := contextWindow - reserveTokens
	if availableContext < 1000 {
		availableContext = contextWindow / 2
	}

	threshold := agenticCompressionThreshold()
	maxInputTokens := int64(float64(availableContext) * threshold)

	if currentTokens <= maxInputTokens {
		result.ShouldTrim = false
		return result
	}

	targetRatio := threshold * 0.9
	targetTokens := int64(float64(availableContext) * targetRatio)
	result.TargetTokens = targetTokens
	result.ShouldTrim = true

	targetBytes := int(targetTokens * 4)

	minBytes := 32 * 1024
	if targetBytes < minBytes {
		targetBytes = minBytes
	}

	maxBytes := agenticMaxBodyBytes()
	if targetBytes > maxBytes {
		targetBytes = maxBytes
	}

	result.TargetMaxBytes = targetBytes
	return result
}

// getTokenAwareMaxBytes returns the maximum body size based on token analysis.
func getTokenAwareMaxBytes(body []byte) int {
	if !agenticTokenAwareEnabled() {
		return agenticMaxBodyBytesForModel(body)
	}

	result := analyzeTokenBudget(body)
	if result.ShouldTrim {
		return result.TargetMaxBytes
	}

	return agenticMaxBodyBytesForModel(body)
}

func agenticMaxBodyBytesForModel(body []byte) int {
	maxBytes := agenticMaxBodyBytes()
	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return maxBytes
	}

	info := registry.GetGlobalRegistry().GetModelInfo(model, "")
	if info == nil || info.ContextLength <= 0 {
		return maxBytes
	}

	estimated := info.ContextLength * 4
	const minBytes = 32 * 1024
	if estimated < minBytes {
		estimated = minBytes
	}
	if estimated < maxBytes {
		return estimated
	}
	return maxBytes
}

// trimWithMemoryResult holds the result of trimming with memory extraction.
type trimWithMemoryResult struct {
	Body    []byte
	Query   string
	Dropped []memory.Event
	Shape   string // "chat", "responses", or "claude"
}

// trimOpenAIChatCompletions trims an OpenAI Chat Completions payload by shortening the messages array.
func trimOpenAIChatCompletions(body []byte, maxBytes int, mustKeepTools bool) []byte {
	root := gjson.ParseBytes(body)
	msgs := root.Get("messages")
	if !msgs.IsArray() {
		return body
	}
	arr := msgs.Array()
	if len(arr) == 0 {
		return body
	}

	firstSystem := gjson.Result{}
	for i := 0; i < len(arr); i++ {
		if strings.EqualFold(arr[i].Get("role").String(), "system") {
			firstSystem = arr[i]
			break
		}
	}

	isToolResultMsg := func(m gjson.Result) bool {
		role := strings.ToLower(strings.TrimSpace(m.Get("role").String()))
		return role == "tool" || role == "function"
	}
	assistantHasToolCall := func(m gjson.Result) bool {
		if !strings.EqualFold(m.Get("role").String(), "assistant") {
			return false
		}
		if tc := m.Get("tool_calls"); tc.Exists() && tc.IsArray() && len(tc.Array()) > 0 {
			return true
		}
		if fc := m.Get("function_call"); fc.Exists() {
			return true
		}
		return false
	}

	keep := 20
	perTextLimit := 20_000
	dropTools := false
	minKeep := agenticMinKeepMessages()
	if keep < minKeep {
		keep = minKeep
	}
	for keep >= minKeep {
		outBody := body
		if dropTools && !mustKeepTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", "none")
		}

		newMsgs := make([]string, 0, keep+1)
		if firstSystem.Exists() {
			newMsgs = append(newMsgs, truncateMessageContent(firstSystem.Raw, perTextLimit))
		}

		required := make(map[int]struct{}, 8)
		tailKept := 0
		for i := len(arr) - 1; i >= 0 && (tailKept < keep || len(required) > 0); i-- {
			if strings.EqualFold(arr[i].Get("role").String(), "system") {
				continue
			}

			_, req := required[i]
			if !req && tailKept >= keep {
				continue
			}

			newMsgs = append(newMsgs, truncateMessageContent(arr[i].Raw, perTextLimit))
			if !req {
				tailKept++
			} else {
				delete(required, i)
			}

			if isToolResultMsg(arr[i]) {
				prev := i - 1
				if prev >= 0 && assistantHasToolCall(arr[prev]) {
					required[prev] = struct{}{}
				}
			}
		}

		if firstSystem.Exists() {
			reverseStrings(newMsgs[1:])
		} else {
			reverseStrings(newMsgs)
		}

		out := setJSONArrayBytes(outBody, "messages", newMsgs)
		if len(out) <= maxBytes {
			return out
		}

		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
		dropTools = true
	}

	return body
}

// trimOpenAIResponses trims an OpenAI Responses payload by shortening the input array.
func trimOpenAIResponses(body []byte, maxBytes int, mustKeepTools bool) []byte {
	root := gjson.ParseBytes(body)
	input := root.Get("input")
	if !input.Exists() || !input.IsArray() {
		return body
	}
	arr := input.Array()
	if len(arr) == 0 {
		return body
	}

	keep := 30
	perTextLimit := 20_000
	dropTools := false
	minKeep := agenticMinKeepMessages()
	if keep < minKeep {
		keep = minKeep
	}
	for keep >= minKeep {
		outBody := body
		if dropTools && !mustKeepTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", "none")
		}
		if inst := root.Get("instructions"); inst.Exists() && inst.Type == gjson.String {
			s := inst.String()
			instructionsLimit := perTextLimit
			if instructionsLimit > 2048 {
				instructionsLimit = 2048
			}
			if len(s) > instructionsLimit {
				outBody, _ = sjson.SetBytes(outBody, "instructions", s[:instructionsLimit]+"\n...[truncated]...")
			}
		}

		callByID := make(map[string]string, 16)
		for i := 0; i < len(arr); i++ {
			item := arr[i]
			t := item.Get("type").String()
			if t == "" && item.Get("role").String() != "" {
				t = "message"
			}
			if t != "function_call" {
				continue
			}
			callID := item.Get("call_id").String()
			if callID == "" {
				continue
			}
			if _, ok := callByID[callID]; !ok {
				callByID[callID] = item.Raw
			}
		}

		needCall := make(map[string]struct{}, 8)
		newItems := make([]string, 0, keep+8)
		kept := 0
		for i := len(arr) - 1; i >= 0 && kept < keep; i-- {
			item := arr[i]
			t := item.Get("type").String()
			if t == "" && item.Get("role").String() != "" {
				t = "message"
			}

			if dropTools && !mustKeepTools && (t == "function_call" || t == "function_call_output") {
				continue
			}

			if t == "function_call_output" {
				callID := item.Get("call_id").String()
				if callID != "" {
					needCall[callID] = struct{}{}
				}
			}
			if t == "function_call" {
				callID := item.Get("call_id").String()
				if callID != "" {
					delete(needCall, callID)
				}
			}

			newItems = append(newItems, truncateMessageContent(item.Raw, perTextLimit))
			kept++
		}
		reverseStrings(newItems)

		if !dropTools && len(needCall) > 0 {
			prefix := make([]string, 0, len(needCall))
			for callID := range needCall {
				if raw, ok := callByID[callID]; ok {
					prefix = append(prefix, raw)
				}
			}
			if len(prefix) > 0 {
				ordered := make([]string, 0, len(prefix))
				for i := 0; i < len(arr); i++ {
					item := arr[i]
					if item.Get("type").String() != "function_call" {
						continue
					}
					callID := item.Get("call_id").String()
					if callID == "" {
						continue
					}
					if _, ok := needCall[callID]; ok {
						if raw, ok2 := callByID[callID]; ok2 {
							ordered = append(ordered, raw)
						}
					}
				}
				newItems = append(ordered, newItems...)
			}
		}

		out := setJSONArrayBytes(outBody, "input", newItems)
		if len(out) <= maxBytes {
			return out
		}

		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
		if !mustKeepTools {
			dropTools = true
		}
	}

	return body
}

func trimOpenAIChatCompletionsWithMemory(body []byte, maxBytes int, mustKeepTools bool) *trimWithMemoryResult {
	root := gjson.ParseBytes(body)
	msgs := root.Get("messages")
	if !msgs.IsArray() {
		return &trimWithMemoryResult{Body: body, Shape: "chat"}
	}
	arr := msgs.Array()
	if len(arr) == 0 {
		return &trimWithMemoryResult{Body: body, Shape: "chat"}
	}

	firstSystem := gjson.Result{}
	firstSystemIndex := -1
	for i := 0; i < len(arr); i++ {
		if strings.EqualFold(arr[i].Get("role").String(), "system") {
			firstSystem = arr[i]
			firstSystemIndex = i
			break
		}
	}

	query := extractLastUserTextFromChat(arr)
	minKeep := agenticMinKeepMessages()
	keep := 20
	perTextLimit := 20_000
	dropTools := false
	if keep < minKeep {
		keep = minKeep
	}
	for keep >= minKeep {
		outBody := body
		if dropTools && !mustKeepTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", "none")
		}

		newMsgs := make([]string, 0, keep+1)
		keptIdx := make(map[int]struct{}, keep+2)
		if firstSystem.Exists() {
			newMsgs = append(newMsgs, truncateMessageContent(firstSystem.Raw, perTextLimit))
			keptIdx[firstSystemIndex] = struct{}{}
		}

		isToolResultMsg := func(m gjson.Result) bool {
			role := strings.ToLower(strings.TrimSpace(m.Get("role").String()))
			return role == "tool" || role == "function"
		}
		assistantHasToolCall := func(m gjson.Result) bool {
			if !strings.EqualFold(m.Get("role").String(), "assistant") {
				return false
			}
			if tc := m.Get("tool_calls"); tc.Exists() && tc.IsArray() && len(tc.Array()) > 0 {
				return true
			}
			if fc := m.Get("function_call"); fc.Exists() {
				return true
			}
			return false
		}

		required := make(map[int]struct{}, 8)
		tailKept := 0
		for i := len(arr) - 1; i >= 0 && (tailKept < keep || len(required) > 0); i-- {
			if strings.EqualFold(arr[i].Get("role").String(), "system") {
				continue
			}

			_, req := required[i]
			if !req && tailKept >= keep {
				continue
			}

			newMsgs = append(newMsgs, truncateMessageContent(arr[i].Raw, perTextLimit))
			keptIdx[i] = struct{}{}
			if !req {
				tailKept++
			} else {
				delete(required, i)
			}

			if isToolResultMsg(arr[i]) {
				prev := i - 1
				if prev >= 0 && assistantHasToolCall(arr[prev]) {
					required[prev] = struct{}{}
				}
			}
		}

		if firstSystem.Exists() {
			reverseStrings(newMsgs[1:])
		} else {
			reverseStrings(newMsgs)
		}

		out := setJSONArrayBytes(outBody, "messages", newMsgs)
		if len(out) <= maxBytes {
			dropped := collectDroppedChat(arr, keptIdx)
			return &trimWithMemoryResult{Body: out, Query: query, Dropped: dropped, Shape: "chat"}
		}

		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
		dropTools = true
	}
	return &trimWithMemoryResult{Body: body, Query: query, Shape: "chat"}
}

func collectDroppedChat(arr []gjson.Result, kept map[int]struct{}) []memory.Event {
	out := make([]memory.Event, 0, 32)
	for i := 0; i < len(arr); i++ {
		if _, ok := kept[i]; ok {
			continue
		}
		role := arr[i].Get("role").String()
		txt := extractTextFromChatMessage(arr[i])
		if strings.TrimSpace(txt) == "" {
			continue
		}
		out = append(out, memory.Event{Kind: "dropped_chat", Role: role, Text: txt})
	}
	return out
}

func extractLastUserTextFromChat(arr []gjson.Result) string {
	for i := len(arr) - 1; i >= 0; i-- {
		if !strings.EqualFold(arr[i].Get("role").String(), "user") {
			continue
		}
		txt := extractTextFromChatMessage(arr[i])
		if strings.TrimSpace(txt) != "" {
			return txt
		}
	}
	return ""
}

func extractTextFromChatMessage(msg gjson.Result) string {
	content := msg.Get("content")
	switch {
	case content.Type == gjson.String:
		return content.String()
	case content.IsArray():
		var b strings.Builder
		for _, it := range content.Array() {
			partType := it.Get("type").String()
			if partType == "thinking" || partType == "reasoning" {
				continue
			}
			if t := it.Get("text"); t.Exists() && t.Type == gjson.String {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(t.String())
			}
		}
		return b.String()
	default:
		return ""
	}
}

func trimOpenAIResponsesWithMemory(body []byte, maxBytes int, mustKeepTools bool) *trimWithMemoryResult {
	root := gjson.ParseBytes(body)
	input := root.Get("input")
	if !input.Exists() || !input.IsArray() {
		return &trimWithMemoryResult{Body: body, Shape: "responses"}
	}
	arr := input.Array()
	if len(arr) == 0 {
		return &trimWithMemoryResult{Body: body, Shape: "responses"}
	}

	query := extractLastUserTextFromResponses(arr)
	minKeep := agenticMinKeepMessages()
	keep := 30
	perTextLimit := 20_000
	dropTools := false
	if keep < minKeep {
		keep = minKeep
	}
	for keep >= minKeep {
		outBody := body
		if dropTools && !mustKeepTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", "none")
		}
		if inst := root.Get("instructions"); inst.Exists() && inst.Type == gjson.String {
			s := inst.String()
			instructionsLimit := perTextLimit
			if instructionsLimit > 2048 {
				instructionsLimit = 2048
			}
			if len(s) > instructionsLimit {
				outBody, _ = sjson.SetBytes(outBody, "instructions", s[:instructionsLimit]+"\n...[truncated]...")
			}
		}

		callByID := make(map[string]gjson.Result, 16)
		for i := 0; i < len(arr); i++ {
			item := arr[i]
			t := item.Get("type").String()
			if t == "" && item.Get("role").String() != "" {
				t = "message"
			}
			if t != "function_call" {
				continue
			}
			callID := item.Get("call_id").String()
			if callID == "" {
				continue
			}
			if _, ok := callByID[callID]; !ok {
				callByID[callID] = item
			}
		}

		needCall := make(map[string]struct{}, 8)
		newItems := make([]string, 0, keep+8)
		keptIdx := make(map[int]struct{}, keep+16)
		kept := 0
		for i := len(arr) - 1; i >= 0 && kept < keep; i-- {
			item := arr[i]
			t := item.Get("type").String()
			if t == "" && item.Get("role").String() != "" {
				t = "message"
			}

			if dropTools && !mustKeepTools && (t == "function_call" || t == "function_call_output") {
				continue
			}
			if t == "function_call_output" {
				callID := item.Get("call_id").String()
				if callID != "" {
					needCall[callID] = struct{}{}
				}
			}
			if t == "function_call" {
				callID := item.Get("call_id").String()
				if callID != "" {
					delete(needCall, callID)
				}
			}

			newItems = append(newItems, truncateMessageContent(item.Raw, perTextLimit))
			keptIdx[i] = struct{}{}
			kept++
		}
		reverseStrings(newItems)

		if !dropTools && len(needCall) > 0 {
			ordered := make([]string, 0, len(needCall))
			for i := 0; i < len(arr); i++ {
				item := arr[i]
				if item.Get("type").String() != "function_call" {
					continue
				}
				callID := item.Get("call_id").String()
				if callID == "" {
					continue
				}
				if _, ok := needCall[callID]; ok {
					if call, ok2 := callByID[callID]; ok2 {
						ordered = append(ordered, call.Raw)
						keptIdx[i] = struct{}{}
					}
				}
			}
			if len(ordered) > 0 {
				newItems = append(ordered, newItems...)
			}
		}

		out := setJSONArrayBytes(outBody, "input", newItems)
		if len(out) <= maxBytes {
			dropped := collectDroppedResponses(arr, keptIdx)
			return &trimWithMemoryResult{Body: out, Query: query, Dropped: dropped, Shape: "responses"}
		}

		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
		if !mustKeepTools {
			dropTools = true
		}
	}

	return &trimWithMemoryResult{Body: body, Query: query, Shape: "responses"}
}

func collectDroppedResponses(arr []gjson.Result, kept map[int]struct{}) []memory.Event {
	out := make([]memory.Event, 0, 64)
	for i := 0; i < len(arr); i++ {
		if _, ok := kept[i]; ok {
			continue
		}
		item := arr[i]
		t := item.Get("type").String()
		role := item.Get("role").String()
		txt := extractTextFromResponsesItem(item)
		if strings.TrimSpace(txt) == "" {
			continue
		}
		out = append(out, memory.Event{Kind: "dropped_responses", Type: t, Role: role, Text: txt})
	}
	return out
}

func extractLastUserTextFromResponses(arr []gjson.Result) string {
	for i := len(arr) - 1; i >= 0; i-- {
		item := arr[i]
		role := item.Get("role").String()
		if !strings.EqualFold(role, "user") {
			continue
		}
		txt := extractTextFromResponsesItem(item)
		if strings.TrimSpace(txt) != "" {
			return txt
		}
	}
	return ""
}

func extractTextFromResponsesItem(item gjson.Result) string {
	content := item.Get("content")
	if content.IsArray() {
		var b strings.Builder
		for _, part := range content.Array() {
			partType := part.Get("type").String()
			if partType == "thinking" || partType == "reasoning" {
				continue
			}
			text := part.Get("text")
			if !text.Exists() || text.Type != gjson.String {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(text.String())
		}
		return b.String()
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if t := item.Get("text"); t.Exists() && t.Type == gjson.String {
		return t.String()
	}
	if out := item.Get("output"); out.Exists() && out.Type == gjson.String {
		return out.String()
	}
	return ""
}

func trimClaudeMessagesWithMemory(body []byte, maxBytes int, mustKeepTools bool) *trimWithMemoryResult {
	root := gjson.ParseBytes(body)
	msgs := root.Get("messages")
	if !msgs.IsArray() {
		return &trimWithMemoryResult{Body: body, Shape: "claude"}
	}
	arr := msgs.Array()
	if len(arr) == 0 {
		return &trimWithMemoryResult{Body: body, Shape: "claude"}
	}

	query := extractLastUserTextFromClaude(arr)
	minKeep := agenticMinKeepMessages()
	keep := 20
	perTextLimit := 20_000
	dropTools := false
	if keep < minKeep {
		keep = minKeep
	}
	for keep >= minKeep {
		outBody := body
		if dropTools && !mustKeepTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", map[string]any{"type": "none"})
		}

		newMsgs := make([]string, 0, keep+1)
		keptIdx := make(map[int]struct{}, keep+2)

		isToolResultMsg := func(m gjson.Result) bool {
			content := m.Get("content")
			if !content.IsArray() {
				return false
			}
			for _, part := range content.Array() {
				if part.Get("type").String() == "tool_result" {
					return true
				}
			}
			return false
		}
		assistantHasToolUse := func(m gjson.Result) bool {
			if !strings.EqualFold(m.Get("role").String(), "assistant") {
				return false
			}
			content := m.Get("content")
			if !content.IsArray() {
				return false
			}
			for _, part := range content.Array() {
				if part.Get("type").String() == "tool_use" {
					return true
				}
			}
			return false
		}

		required := make(map[int]struct{}, 8)
		tailKept := 0
		for i := len(arr) - 1; i >= 0 && (tailKept < keep || len(required) > 0); i-- {
			_, req := required[i]
			if !req && tailKept >= keep {
				continue
			}

			newMsgs = append(newMsgs, truncateClaudeMessageContent(arr[i].Raw, perTextLimit))
			keptIdx[i] = struct{}{}
			if !req {
				tailKept++
			} else {
				delete(required, i)
			}

			if isToolResultMsg(arr[i]) {
				prev := i - 1
				if prev >= 0 && assistantHasToolUse(arr[prev]) {
					required[prev] = struct{}{}
				}
			}
		}

		reverseStrings(newMsgs)

		out := setJSONArrayBytes(outBody, "messages", newMsgs)
		if len(out) <= maxBytes {
			dropped := collectDroppedClaude(arr, keptIdx)
			return &trimWithMemoryResult{Body: out, Query: query, Dropped: dropped, Shape: "claude"}
		}

		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
		dropTools = true
	}
	return &trimWithMemoryResult{Body: body, Query: query, Shape: "claude"}
}

func collectDroppedClaude(arr []gjson.Result, kept map[int]struct{}) []memory.Event {
	out := make([]memory.Event, 0, 32)
	for i := 0; i < len(arr); i++ {
		if _, ok := kept[i]; ok {
			continue
		}
		role := arr[i].Get("role").String()
		txt := extractTextFromClaudeMessage(arr[i])
		if strings.TrimSpace(txt) == "" {
			continue
		}
		out = append(out, memory.Event{Kind: "dropped_claude", Role: role, Text: txt})
	}
	return out
}

func extractLastUserTextFromClaude(arr []gjson.Result) string {
	for i := len(arr) - 1; i >= 0; i-- {
		if !strings.EqualFold(arr[i].Get("role").String(), "user") {
			continue
		}
		txt := extractTextFromClaudeMessage(arr[i])
		if strings.TrimSpace(txt) != "" {
			return txt
		}
	}
	return ""
}

func extractTextFromClaudeMessage(msg gjson.Result) string {
	content := msg.Get("content")
	switch {
	case content.Type == gjson.String:
		return content.String()
	case content.IsArray():
		var b strings.Builder
		for _, part := range content.Array() {
			partType := part.Get("type").String()
			if partType == "text" {
				if t := part.Get("text"); t.Exists() && t.Type == gjson.String {
					if b.Len() > 0 {
						b.WriteString("\n")
					}
					b.WriteString(t.String())
				}
			}
		}
		return b.String()
	default:
		return ""
	}
}

func truncateMessageContent(msgRaw string, maxTextChars int) string {
	msg := msgRaw
	if maxTextChars <= 0 {
		return msg
	}

	content := gjson.Get(msg, "content")
	switch {
	case content.Type == gjson.String:
		s := content.String()
		if len(s) > maxTextChars {
			s = s[:maxTextChars] + "\n...[truncated]..."
			msg, _ = sjson.Set(msg, "content", s)
		}
		return msg
	case content.IsArray():
		items := content.Array()
		for i := 0; i < len(items); i++ {
			text := items[i].Get("text")
			if !text.Exists() || text.Type != gjson.String {
				continue
			}
			s := text.String()
			if len(s) > maxTextChars {
				s = s[:maxTextChars] + "\n...[truncated]..."
				msg, _ = sjson.Set(msg, "content."+strconv.Itoa(i)+".text", s)
			}
		}
		return msg
	default:
		return msg
	}
}

func truncateClaudeMessageContent(msgRaw string, maxTextChars int) string {
	msg := msgRaw
	if maxTextChars <= 0 {
		return msg
	}

	content := gjson.Get(msg, "content")
	switch {
	case content.Type == gjson.String:
		s := content.String()
		if len(s) > maxTextChars {
			s = s[:maxTextChars] + "\n...[truncated]..."
			msg, _ = sjson.Set(msg, "content", s)
		}
		return msg
	case content.IsArray():
		items := content.Array()
		for i := 0; i < len(items); i++ {
			partType := items[i].Get("type").String()
			if partType != "text" {
				continue
			}
			text := items[i].Get("text")
			if !text.Exists() || text.Type != gjson.String {
				continue
			}
			s := text.String()
			if len(s) > maxTextChars {
				s = s[:maxTextChars] + "\n...[truncated]..."
				msg, _ = sjson.Set(msg, "content."+strconv.Itoa(i)+".text", s)
			}
		}
		return msg
	default:
		return msg
	}
}

func setJSONArrayBytes(body []byte, key string, rawItems []string) []byte {
	out := body
	out, _ = sjson.SetRawBytes(out, key, []byte("[]"))
	for i := range rawItems {
		out, _ = sjson.SetRawBytes(out, key+".-1", []byte(rawItems[i]))
	}
	return out
}

func reverseStrings(items []string) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}
