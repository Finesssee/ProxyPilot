package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
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

	memOnce  sync.Once
	memStore memory.Store
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

func agenticMemoryStore() memory.Store {
	memOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_ENABLED")); v != "" {
			if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
				memStore = nil
				return
			}
		}

		base := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_DIR"))
		if base == "" {
			if w := util.WritablePath(); w != "" {
				base = filepath.Join(w, ".proxypilot", "memory")
			} else {
				base = filepath.Join(".proxypilot", "memory")
			}
		}
		memStore = memory.NewFileStore(base)
	})
	return memStore
}

func agenticTodoEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_TODO_ENABLED")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
	}
	return true
}

func agenticTodoMaxChars() int {
	v := strings.TrimSpace(os.Getenv("CLIPROXY_TODO_MAX_CHARS"))
	if v == "" {
		return 4000
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 4000
	}
	if n < 512 {
		return 512
	}
	if n > 20_000 {
		return 20_000
	}
	return n
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
		mustKeepTools := strings.Contains(ua, "factory-cli") || strings.Contains(ua, "droid") || isStainless
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

		// Session-scoped state (pinned + anchor + TODO) is packed into the *last user message*
		// so we don't mutate instructions/system blocks. This is more prompt-cache friendly
		// and survives trimming (we prepend state so head-truncation keeps it).
		if agenticTodoEnabled() {
			session := extractAgenticSessionKey(req, body)
			body = agenticMaybeUpsertAndInjectPackedState(req, session, body, maxBytes)
			originalLen = len(body)
		}

		if originalLen <= maxBytes {
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(originalLen)
			req.Header.Set("Content-Length", strconv.Itoa(originalLen))
			c.Next()
			return
		}

		path := req.URL.Path
		trimmed := body
		session := extractAgenticSessionKey(req, body)
		switch {
		case strings.HasSuffix(path, "/v1/chat/completions"):
			res := trimOpenAIChatCompletionsWithMemory(trimmed, maxBytes, mustKeepTools)
			trimmed = res.Body
			agenticStoreAndInjectMemory(req, session, res, maxBytes)
			trimmed = res.Body
		case strings.HasSuffix(path, "/v1/responses"):
			res := trimOpenAIResponsesWithMemory(trimmed, maxBytes, mustKeepTools)
			trimmed = res.Body
			agenticStoreAndInjectMemory(req, session, res, maxBytes)
			trimmed = res.Body
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

func agenticMaybeUpsertAndInjectPackedState(req *http.Request, session string, body []byte, maxBytes int) []byte {
	if req == nil || session == "" || len(body) == 0 {
		return body
	}
	store := agenticMemoryStore()
	if store == nil {
		return body
	}
	fs, ok := store.(*memory.FileStore)
	if !ok {
		return body
	}

	// Allow external controllers (ProxyPilot UI) to set TODO via header.
	// Keep it small and redacted; no auth is stored.
	if hdr := strings.TrimSpace(req.Header.Get("X-CLIProxyAPI-Todo")); hdr != "" {
		_ = fs.WriteTodo(session, hdr, 8000)
		// Avoid forwarding this header upstream.
		req.Header.Del("X-CLIProxyAPI-Todo")
	}

	// Upgrade pinned context: capture coding guidelines / AGENTS.md content when present in the payload.
	if pinned := extractCodingGuidelinesFromBody(body); strings.TrimSpace(pinned) != "" {
		_ = fs.WritePinned(session, pinned, 8000)
	}

	todo := fs.ReadTodo(session, agenticTodoMaxChars())
	if strings.TrimSpace(todo) == "" {
		// Seed a minimal TODO from the last user intent if we have nothing yet.
		shape := detectShapeFromPath(req.URL.Path)
		seed := extractLastUserIntent(shape, body)
		if strings.TrimSpace(seed) != "" {
			seedTodo := "# TODO\n\n- " + strings.TrimSpace(seed) + "\n"
			_ = fs.WriteTodo(session, seedTodo, 8000)
			todo = fs.ReadTodo(session, agenticTodoMaxChars())
		}
	}
	shape := detectShapeFromPath(req.URL.Path)
	pinned := fs.ReadPinned(session, 6000)
	anchor := fs.ReadSummary(session, 2500)
	block := buildPackedState(pinned, anchor, todo)
	if strings.TrimSpace(block) == "" {
		return body
	}
	// Inject TODO as pinned instructions/system, not as memory (so it’s always present).
	return prependToLastUserText(shape, body, block, maxBytes)
}

func detectShapeFromPath(path string) string {
	switch {
	case strings.HasSuffix(path, "/v1/chat/completions"):
		return "chat"
	case strings.HasSuffix(path, "/v1/responses"):
		return "responses"
	default:
		return ""
	}
}

func extractLastUserIntent(shape string, body []byte) string {
	switch shape {
	case "responses":
		arr := gjson.GetBytes(body, "input").Array()
		return extractLastUserTextFromResponses(arr)
	case "chat":
		arr := gjson.GetBytes(body, "messages").Array()
		return extractLastUserTextFromChat(arr)
	default:
		return ""
	}
}

func buildPackedState(pinned string, anchor string, todo string) string {
	pinned = strings.TrimSpace(pinned)
	anchor = strings.TrimSpace(anchor)
	todo = strings.TrimSpace(todo)
	if pinned == "" && anchor == "" && todo == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("<proxypilot_state>\n")
	if pinned != "" {
		b.WriteString("<pinned>\n")
		b.WriteString(pinned)
		b.WriteString("\n</pinned>\n")
	}
	if anchor != "" {
		b.WriteString("<anchor>\n")
		b.WriteString(anchor)
		b.WriteString("\n</anchor>\n")
	}
	if todo != "" {
		b.WriteString("<todo>\n")
		b.WriteString(todo)
		b.WriteString("\n</todo>\n")
	}
	b.WriteString("</proxypilot_state>\n\n")
	return b.String()
}

func extractCodingGuidelinesFromBody(body []byte) string {
	// Best-effort extraction for agentic CLIs that embed <coding_guidelines>...</coding_guidelines>
	// (commonly from AGENTS.md) into the request history.
	if len(body) == 0 {
		return ""
	}
	const maxScan = 350_000
	raw := string(body)
	if len(raw) > maxScan {
		raw = raw[:maxScan]
	}
	start := strings.Index(raw, "<coding_guidelines>")
	if start < 0 {
		// PowerShell ConvertTo-Json escapes '<' and '>' into \\u003c/\\u003e.
		start = strings.Index(raw, "\\u003ccoding_guidelines\\u003e")
	}
	if start < 0 {
		return ""
	}
	end := strings.Index(raw[start:], "</coding_guidelines>")
	if end < 0 {
		end = strings.Index(raw[start:], "\\u003c/coding_guidelines\\u003e")
	}
	if end < 0 {
		return ""
	}
	// Keep the closing tag if we can find it.
	endAbs := start + end
	if strings.HasPrefix(raw[start+end:], "</coding_guidelines>") {
		endAbs += len("</coding_guidelines>")
	} else if strings.HasPrefix(raw[start+end:], "\\u003c/coding_guidelines\\u003e") {
		endAbs += len("\\u003c/coding_guidelines\\u003e")
	}
	out := strings.TrimSpace(raw[start:endAbs])
	out = strings.ReplaceAll(out, "\\u003c", "<")
	out = strings.ReplaceAll(out, "\\u003e", ">")
	out = strings.ReplaceAll(out, "\\u003C", "<")
	out = strings.ReplaceAll(out, "\\u003E", ">")
	out = strings.ReplaceAll(out, "\\n", "\n")
	out = strings.ReplaceAll(out, "\\r", "\r")
	if len(out) > 8000 {
		out = out[:8000] + "\n...[truncated]..."
	}
	return out
}

func prependToLastUserText(shape string, body []byte, prefix string, maxBytes int) []byte {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return body
	}
	limit := maxBytes - len(body) - 512
	if maxBytes > 0 && limit <= 0 {
		return body
	}
	if maxBytes > 0 && len(prefix) > limit {
		prefix = prefix[:limit] + "\n...[truncated]..."
	}
	prefix = prefix + "\n"

	switch shape {
	case "responses":
		input := gjson.GetBytes(body, "input")
		if !input.Exists() || !input.IsArray() {
			return body
		}
		arr := input.Array()
		for i := len(arr) - 1; i >= 0; i-- {
			if !strings.EqualFold(arr[i].Get("role").String(), "user") {
				continue
			}
			content := arr[i].Get("content")
			if !content.Exists() || !content.IsArray() {
				continue
			}
			parts := content.Array()
			for j := 0; j < len(parts); j++ {
				t := parts[j].Get("type").String()
				if t == "" && parts[j].Get("text").Exists() {
					t = "input_text"
				}
				if t != "input_text" {
					continue
				}
				old := parts[j].Get("text").String()
				newText := prefix + old
				out, err := sjson.SetBytes(body, "input."+strconv.Itoa(i)+".content."+strconv.Itoa(j)+".text", newText)
				if err == nil {
					return out
				}
				return body
			}
		}
		return body

	case "chat":
		msgs := gjson.GetBytes(body, "messages")
		if !msgs.Exists() || !msgs.IsArray() {
			return body
		}
		arr := msgs.Array()
		for i := len(arr) - 1; i >= 0; i-- {
			if !strings.EqualFold(arr[i].Get("role").String(), "user") {
				continue
			}
			content := arr[i].Get("content")
			switch {
			case content.Type == gjson.String:
				old := content.String()
				newText := prefix + old
				out, err := sjson.SetBytes(body, "messages."+strconv.Itoa(i)+".content", newText)
				if err == nil {
					return out
				}
				return body
			case content.IsArray():
				parts := content.Array()
				for j := 0; j < len(parts); j++ {
					txt := parts[j].Get("text")
					if !txt.Exists() || txt.Type != gjson.String {
						continue
					}
					old := txt.String()
					newText := prefix + old
					out, err := sjson.SetBytes(body, "messages."+strconv.Itoa(i)+".content."+strconv.Itoa(j)+".text", newText)
					if err == nil {
						return out
					}
					return body
				}
				return body
			default:
				return body
			}
		}
		return body
	default:
		return body
	}
}

func appendToLastUserText(shape string, body []byte, suffix string, maxBytes int) []byte {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return body
	}
	limit := maxBytes - len(body) - 512
	if maxBytes > 0 && limit <= 0 {
		return body
	}
	if maxBytes > 0 && len(suffix) > limit {
		suffix = suffix[:limit] + "\n...[truncated]..."
	}
	suffix = "\n" + suffix

	switch shape {
	case "responses":
		input := gjson.GetBytes(body, "input")
		if !input.Exists() || !input.IsArray() {
			return body
		}
		arr := input.Array()
		for i := len(arr) - 1; i >= 0; i-- {
			if !strings.EqualFold(arr[i].Get("role").String(), "user") {
				continue
			}
			content := arr[i].Get("content")
			if !content.Exists() || !content.IsArray() {
				continue
			}
			parts := content.Array()
			for j := 0; j < len(parts); j++ {
				t := parts[j].Get("type").String()
				if t == "" && parts[j].Get("text").Exists() {
					t = "input_text"
				}
				if t != "input_text" {
					continue
				}
				old := parts[j].Get("text").String()
				newText := old + suffix
				out, err := sjson.SetBytes(body, "input."+strconv.Itoa(i)+".content."+strconv.Itoa(j)+".text", newText)
				if err == nil {
					return out
				}
				return body
			}
		}
		return body

	case "chat":
		msgs := gjson.GetBytes(body, "messages")
		if !msgs.Exists() || !msgs.IsArray() {
			return body
		}
		arr := msgs.Array()
		for i := len(arr) - 1; i >= 0; i-- {
			if !strings.EqualFold(arr[i].Get("role").String(), "user") {
				continue
			}
			content := arr[i].Get("content")
			switch {
			case content.Type == gjson.String:
				old := content.String()
				newText := old + suffix
				out, err := sjson.SetBytes(body, "messages."+strconv.Itoa(i)+".content", newText)
				if err == nil {
					return out
				}
				return body
			case content.IsArray():
				parts := content.Array()
				for j := 0; j < len(parts); j++ {
					txt := parts[j].Get("text")
					if !txt.Exists() || txt.Type != gjson.String {
						continue
					}
					old := txt.String()
					newText := old + suffix
					out, err := sjson.SetBytes(body, "messages."+strconv.Itoa(i)+".content."+strconv.Itoa(j)+".text", newText)
					if err == nil {
						return out
					}
					return body
				}
				return body
			default:
				return body
			}
		}
		return body
	default:
		return body
	}
}

type trimWithMemoryResult struct {
	Body    []byte
	Query   string
	Dropped []memory.Event
	Shape   string // "chat" or "responses"
}

func agenticStoreAndInjectMemory(req *http.Request, session string, res *trimWithMemoryResult, maxBytes int) {
	if req == nil || res == nil {
		return
	}
	if session == "" {
		return
	}
	store := agenticMemoryStore()
	if store == nil {
		return
	}

	if len(res.Dropped) > 0 {
		_ = store.Append(session, res.Dropped)
	}

	// Update anchored summary and pinned context (best-effort).
	if fs, ok := store.(*memory.FileStore); ok {
		pinned := extractPinnedContext(req, res.Shape, res.Body)
		_ = fs.UpsertAnchoredSummary(session, res.Dropped, pinned, res.Query)
	}

	// Only inject retrieval when we actually trimmed (otherwise it just spends tokens).
	// Also avoid injecting if tools were forcibly disabled by the client.
	if strings.TrimSpace(res.Query) == "" {
		return
	}

	maxSnips := 8
	maxChars := 6000
	snips, err := store.Search(session, res.Query, maxChars, maxSnips)
	if err != nil || len(snips) == 0 {
		return
	}

	memBlock := buildMemoryBlock(snips)
	res.Body = appendToLastUserText(res.Shape, res.Body, memBlock, maxBytes)
}

func extractPinnedContext(req *http.Request, shape string, body []byte) string {
	// Pinned is intended to capture durable “always-on” state: system instructions / policies.
	// For /v1/responses use instructions; for chat prefer first system message.
	switch shape {
	case "responses":
		if v := gjson.GetBytes(body, "instructions"); v.Exists() && v.Type == gjson.String {
			s := strings.TrimSpace(v.String())
			if len(s) > 6000 {
				s = s[:6000] + "\n...[truncated]..."
			}
			return s
		}
	case "chat":
		msgs := gjson.GetBytes(body, "messages")
		if msgs.Exists() && msgs.IsArray() {
			for _, m := range msgs.Array() {
				if !strings.EqualFold(m.Get("role").String(), "system") {
					continue
				}
				c := m.Get("content")
				if c.Type == gjson.String {
					s := strings.TrimSpace(c.String())
					if len(s) > 6000 {
						s = s[:6000] + "\n...[truncated]..."
					}
					return s
				}
			}
		}
	}
	// Fallback to UA to help debugging, but avoid storing auth.
	if req != nil {
		ua := strings.TrimSpace(req.Header.Get("User-Agent"))
		if ua != "" {
			return "User-Agent: " + ua
		}
	}
	return ""
}

func buildMemoryBlock(snips []string) string {
	if len(snips) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<memory>\n")
	b.WriteString("Relevant prior context (auto-retrieved):\n")
	for i := range snips {
		b.WriteString("\n---\n")
		b.WriteString(snips[i])
		b.WriteString("\n")
	}
	b.WriteString("</memory>\n")
	return b.String()
}

func injectMemoryIntoBody(shape string, body []byte, memText string, maxBytes int) []byte {
	memText = strings.TrimSpace(memText)
	if memText == "" {
		return body
	}
	if maxBytes <= 0 || len(body) >= maxBytes {
		return body
	}

	// Budget the injection to fit.
	limit := maxBytes - len(body) - 512
	if limit <= 0 {
		return body
	}
	if len(memText) > limit {
		memText = memText[:limit] + "\n...[truncated]..."
	}

	out := body
	switch shape {
	case "responses":
		inst := gjson.GetBytes(out, "instructions")
		if inst.Exists() && inst.Type == gjson.String && strings.TrimSpace(inst.String()) != "" {
			merged := inst.String() + "\n\n" + memText
			if updated, err := sjson.SetBytes(out, "instructions", merged); err == nil {
				out = updated
			}
		} else {
			if updated, err := sjson.SetBytes(out, "instructions", memText); err == nil {
				out = updated
			}
		}
	case "chat":
		// Prefer to append to existing system message; otherwise prepend a new one.
		msgs := gjson.GetBytes(out, "messages")
		if !msgs.Exists() || !msgs.IsArray() {
			return out
		}
		arr := msgs.Array()
		for i := 0; i < len(arr); i++ {
			if strings.EqualFold(arr[i].Get("role").String(), "system") {
				content := arr[i].Get("content")
				if content.Type == gjson.String {
					merged := content.String() + "\n\n" + memText
					if updated, err := sjson.SetBytes(out, "messages."+strconv.Itoa(i)+".content", merged); err == nil {
						out = updated
					}
					return out
				}
			}
		}
		sys := `{"role":"system","content":""}`
		sys, _ = sjson.Set(sys, "content", memText)
		newMsgs := make([]string, 0, len(arr)+1)
		newMsgs = append(newMsgs, sys)
		for i := 0; i < len(arr); i++ {
			newMsgs = append(newMsgs, arr[i].Raw)
		}
		out = setJSONArrayBytes(out, "messages", newMsgs)
	}

	// If we still exceeded budget, drop memory (better than breaking requests).
	if len(out) > maxBytes {
		return body
	}
	return out
}

func extractAgenticSessionKey(req *http.Request, body []byte) string {
	if req != nil {
		if v := strings.TrimSpace(req.Header.Get("X-CLIProxyAPI-Session")); v != "" {
			return v
		}
		if v := strings.TrimSpace(req.Header.Get("X-Session-Id")); v != "" {
			return v
		}
	}
	if v := gjson.GetBytes(body, "prompt_cache_key"); v.Exists() && v.Type == gjson.String && v.String() != "" {
		return v.String()
	}
	if v := gjson.GetBytes(body, "metadata.session_id"); v.Exists() && v.Type == gjson.String && v.String() != "" {
		return v.String()
	}
	if v := gjson.GetBytes(body, "session_id"); v.Exists() && v.Type == gjson.String && v.String() != "" {
		return v.String()
	}
	// Fallback: stable-ish hash of auth + UA (never store the raw values as session).
	ua := ""
	auth := ""
	if req != nil {
		ua = req.Header.Get("User-Agent")
		auth = req.Header.Get("Authorization")
	}
	sum := sha256.Sum256([]byte(auth + "|" + ua))
	return "ua_" + hex.EncodeToString(sum[:8])
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
		// OpenAI tool result message uses role:"tool". Legacy uses role:"function".
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
	for keep >= 1 {
		outBody := body
		if dropTools && !mustKeepTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", "none")
		}

		newMsgs := make([]string, 0, keep+1)
		if firstSystem.Exists() {
			newMsgs = append(newMsgs, truncateMessageContent(firstSystem.Raw, perTextLimit))
		}

		// Preserve tool call/result adjacency:
		// If we keep a tool result message, we must also keep the immediately preceding
		// assistant tool call message, otherwise downstream Claude/OpenAI validators can reject.
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
	for keep >= 1 {
		outBody := body
		if dropTools && !mustKeepTools {
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
	keep := 20
	perTextLimit := 20_000
	dropTools := false
	for keep >= 1 {
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
	keep := 30
	perTextLimit := 20_000
	dropTools := false
	for keep >= 1 {
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

		// Prepend missing function_call items required by kept outputs.
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
	// Typical Responses input item: {role:"user", content:[{type:"input_text", text:"..."}]}
	content := item.Get("content")
	if content.IsArray() {
		var b strings.Builder
		for _, part := range content.Array() {
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
	// function_call_output has output or content; capture raw-ish summary.
	if out := item.Get("output"); out.Exists() && out.Type == gjson.String {
		return out.String()
	}
	return ""
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
