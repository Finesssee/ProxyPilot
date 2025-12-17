package middleware

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	// codexHardReadLimit is a safety ceiling to avoid unbounded memory reads.
	codexHardReadLimit = 10 * 1024 * 1024
	// codexMaxBodyBytesDefault is a best-effort budget to keep agentic CLI requests under common model limits.
	// It is intentionally conservative to avoid upstream "prompt too long" failures.
	codexMaxBodyBytesDefault = 200 * 1024
)

var (
	codexMaxBodyBytesOnce sync.Once
	codexMaxBodyBytes     int
)

func agenticMaxBodyBytes() int {
	codexMaxBodyBytesOnce.Do(func() {
		codexMaxBodyBytes = codexMaxBodyBytesDefault

		// Optional override (bytes). Useful when running behind very large-context models.
		// Examples:
		//   set CLIPROXY_AGENTIC_MAX_BODY_BYTES=350000
		//   export CLIPROXY_AGENTIC_MAX_BODY_BYTES=350000
		if v := strings.TrimSpace(os.Getenv("CLIPROXY_AGENTIC_MAX_BODY_BYTES")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				// Clamp to a sane range: 32KB..2MB
				if n < 32*1024 {
					n = 32 * 1024
				}
				if n > 2*1024*1024 {
					n = 2 * 1024 * 1024
				}
				codexMaxBodyBytes = n
			}
		}
	})
	return codexMaxBodyBytes
}

// CodexPromptBudgetMiddleware trims oversized OpenAI requests coming from Codex CLI.
//
// Rationale: Codex CLI can accumulate large workspace context and exceed upstream prompt limits.
// When the request is too large, we reduce the payload by:
// - keeping only the first system message (if any)
// - keeping only the last N messages/input items
// - truncating long text blocks within kept messages
//
// The middleware only activates for User-Agent containing "OpenAI Codex".
func CodexPromptBudgetMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		if req == nil {
			c.Next()
			return
		}

		ua := strings.ToLower(req.Header.Get("User-Agent"))
		isStainless := req.Header.Get("X-Stainless-Lang") != "" || req.Header.Get("X-Stainless-Package-Version") != ""
		isAgenticCLI := strings.Contains(ua, "openai codex") || strings.Contains(ua, "factory-cli") || strings.Contains(ua, "warp") || strings.Contains(ua, "droid") || isStainless
		if !isAgenticCLI {
			c.Next()
			return
		}

		if req.Method != http.MethodPost {
			c.Next()
			return
		}

		// Avoid consuming large bodies for non-JSON content.
		ct := req.Header.Get("Content-Type")
		if ct != "" && !strings.Contains(strings.ToLower(ct), "application/json") {
			c.Next()
			return
		}

		if req.Body == nil {
			c.Next()
			return
		}

		// Read body with a hard cap.
		body, err := io.ReadAll(io.LimitReader(req.Body, codexHardReadLimit+1))
		_ = req.Body.Close()
		if err != nil || len(body) == 0 {
			c.Next()
			return
		}
		if len(body) > codexHardReadLimit {
			// Too big to safely process; let upstream reject or the handler deal with it.
			req.Body = io.NopCloser(bytes.NewReader(body[:codexHardReadLimit]))
			req.ContentLength = int64(codexHardReadLimit)
			req.Header.Set("Content-Length", strconv.Itoa(codexHardReadLimit))
			c.Next()
			return
		}

		originalLen := len(body)
		maxBytes := agenticMaxBodyBytesForModel(body)
		if originalLen <= maxBytes {
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(originalLen)
			req.Header.Set("Content-Length", strconv.Itoa(originalLen))
			c.Next()
			return
		}

		path := req.URL.Path
		trimmed := body
		switch {
		case strings.HasSuffix(path, "/v1/chat/completions"):
			trimmed = trimOpenAIChatCompletions(trimmed, maxBytes)
		case strings.HasSuffix(path, "/v1/responses"):
			trimmed = trimOpenAIResponses(trimmed, maxBytes)
		default:
			// Not a known OpenAI payload shape; keep as-is.
		}

		req.Body = io.NopCloser(bytes.NewReader(trimmed))
		req.ContentLength = int64(len(trimmed))
		req.Header.Set("Content-Length", strconv.Itoa(len(trimmed)))
		if len(trimmed) < originalLen {
			req.Header.Set("X-CLIProxyAPI-Trimmed", "true")
			req.Header.Set("X-CLIProxyAPI-Original-Bytes", strconv.Itoa(originalLen))
			req.Header.Set("X-CLIProxyAPI-Trimmed-Bytes", strconv.Itoa(len(trimmed)))
		}

		c.Next()
	}
}

func agenticMaxBodyBytesForModel(body []byte) int {
	maxBytes := agenticMaxBodyBytes()
	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return maxBytes
	}

	info := registry.GetGlobalRegistry().GetModelInfo(model)
	if info == nil || info.ContextLength <= 0 {
		return maxBytes
	}

	// Heuristic: ~4 bytes per token (UTF-8 text + JSON overhead). We only scale DOWN
	// from the global default to avoid upstream "prompt too long" for small-context models.
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

// trimOpenAIChatCompletions trims an OpenAI Chat Completions payload by shortening the messages array.
func trimOpenAIChatCompletions(body []byte, maxBytes int) []byte {
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

	keep := 20
	perTextLimit := 20_000
	dropTools := false
	for keep >= 1 {
		outBody := body
		if dropTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", "none")
		}

		newMsgs := make([]string, 0, keep+1)
		if firstSystem.Exists() {
			newMsgs = append(newMsgs, truncateMessageContent(firstSystem.Raw, perTextLimit))
		}

		kept := 0
		for i := len(arr) - 1; i >= 0 && kept < keep; i-- {
			if strings.EqualFold(arr[i].Get("role").String(), "system") {
				continue
			}
			newMsgs = append(newMsgs, truncateMessageContent(arr[i].Raw, perTextLimit))
			kept++
		}

		// Reverse tail section to restore order (system is at index 0 if present).
		if firstSystem.Exists() {
			reverseStrings(newMsgs[1:])
		} else {
			reverseStrings(newMsgs)
		}

		out := setJSONArrayBytes(outBody, "messages", newMsgs)
		if len(out) <= maxBytes {
			return out
		}

		// If still too large, reduce kept messages and also tighten per-message text limit.
		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
		dropTools = true
	}

	return body
}

// trimOpenAIResponses trims an OpenAI Responses payload by shortening the input array.
func trimOpenAIResponses(body []byte, maxBytes int) []byte {
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
	for keep >= 1 {
		outBody := body
		if dropTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", "none")
		}
		if inst := root.Get("instructions"); inst.Exists() && inst.Type == gjson.String {
			s := inst.String()
			// Instructions can be validated separately upstream; keep it much shorter than message text.
			instructionsLimit := perTextLimit
			if instructionsLimit > 2048 {
				instructionsLimit = 2048
			}
			if len(s) > instructionsLimit {
				outBody, _ = sjson.SetBytes(outBody, "instructions", s[:instructionsLimit]+"\n...[truncated]...")
			}
		}

		// Preserve tool call/result pairs:
		// - If we keep a function_call_output, we must also keep the matching function_call (same call_id),
		//   otherwise downstream Claude-style validators can reject with tool_result/tool_use mismatches.
		// - If we've decided to drop tools, remove both calls and outputs from the conversation history.
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
			// Keep the first occurrence to avoid reordering/duplication surprises.
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

			if dropTools && (t == "function_call" || t == "function_call_output") {
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

		// Prepend any missing function_call items required by kept outputs.
		// If we don't have the matching call, drop the orphan outputs later in the loop by tightening keep/perTextLimit.
		if !dropTools && len(needCall) > 0 {
			prefix := make([]string, 0, len(needCall))
			for callID := range needCall {
				if raw, ok := callByID[callID]; ok {
					prefix = append(prefix, raw)
				}
			}
			if len(prefix) > 0 {
				// Keep stable order by inserting in original array order.
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
		dropTools = true
	}

	return body
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
			// OpenAI chat: {type:"text", text:"..."} or Responses: {type:"input_text", text:"..."}
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
