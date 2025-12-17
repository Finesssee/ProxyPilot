package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	// codexHardReadLimit is a safety ceiling to avoid unbounded memory reads.
	codexHardReadLimit = 10 * 1024 * 1024
	// codexMaxBodyBytes is a best-effort budget to keep Codex CLI requests under common model limits.
	codexMaxBodyBytes = 350 * 1024
)

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

		ua := req.Header.Get("User-Agent")
		if !strings.Contains(strings.ToLower(ua), "openai codex") {
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
		if originalLen <= codexMaxBodyBytes {
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
			trimmed = trimOpenAIChatCompletions(trimmed, codexMaxBodyBytes)
		case strings.HasSuffix(path, "/v1/responses"):
			trimmed = trimOpenAIResponses(trimmed, codexMaxBodyBytes)
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
	for keep >= 1 {
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

		out := setJSONArrayBytes(body, "messages", newMsgs)
		if len(out) <= maxBytes {
			return out
		}

		// If still too large, reduce kept messages and also tighten per-message text limit.
		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
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
	for keep >= 1 {
		outBody := body
		if inst := root.Get("instructions"); inst.Exists() && inst.Type == gjson.String {
			s := inst.String()
			if len(s) > perTextLimit {
				outBody, _ = sjson.SetBytes(outBody, "instructions", s[:perTextLimit]+"\n...[truncated]...")
			}
		}

		newItems := make([]string, 0, keep)
		kept := 0
		for i := len(arr) - 1; i >= 0 && kept < keep; i-- {
			newItems = append(newItems, truncateMessageContent(arr[i].Raw, perTextLimit))
			kept++
		}
		reverseStrings(newItems)

		out := setJSONArrayBytes(outBody, "input", newItems)
		if len(out) <= maxBytes {
			return out
		}

		keep = keep / 2
		if perTextLimit > 5_000 {
			perTextLimit = perTextLimit / 2
		}
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
