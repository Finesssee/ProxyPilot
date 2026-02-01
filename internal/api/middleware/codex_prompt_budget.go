package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
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
	return CodexPromptBudgetMiddlewareWithRootDir("")
}

// CodexPromptBudgetMiddlewareWithRootDir trims oversized requests and injects scaffold state.
// rootDir is used to locate AGENTS.md. When empty, no AGENTS.md is loaded from disk.
func CodexPromptBudgetMiddlewareWithRootDir(rootDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		if req == nil {
			c.Next()
			return
		}

		ua := strings.ToLower(req.Header.Get("User-Agent"))
		isStainless := req.Header.Get("X-Stainless-Lang") != "" || req.Header.Get("X-Stainless-Package-Version") != ""
		mustKeepTools := strings.Contains(ua, "factory-cli") || strings.Contains(ua, "droid") || strings.Contains(ua, "claude-cli") || isStainless
		isAgenticCLI := strings.Contains(ua, "openai codex") || strings.Contains(ua, "factory-cli") || strings.Contains(ua, "warp") || strings.Contains(ua, "droid") || strings.Contains(ua, "claude-cli") || isStainless
		if !isAgenticCLI {
			c.Next()
			return
		}

		if strings.TrimSpace(req.Header.Get("X-CLIProxyAPI-Internal")) != "" {
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
		if err != nil {
			// On read error, return 400 rather than passing empty body downstream
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if len(body) == 0 {
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

		// Token-aware compression: analyze token budget before byte-based check
		tokenAnalysis := analyzeTokenBudget(body)
		maxBytes := tokenAnalysis.TargetMaxBytes

		// If token analysis didn't trigger, fall back to byte-based model limit
		if !tokenAnalysis.ShouldTrim {
			maxBytes = agenticMaxBodyBytesForModel(body)
		}

		// Add diagnostic headers for localhost debugging
		if c != nil {
			ip := c.ClientIP()
			if ip == "127.0.0.1" || ip == "::1" {
				if tokenAnalysis.Model != "" {
					c.Header("X-ProxyPilot-Model", tokenAnalysis.Model)
					c.Header("X-ProxyPilot-Context-Window", strconv.Itoa(tokenAnalysis.ContextWindow))
					c.Header("X-ProxyPilot-Current-Tokens", strconv.FormatInt(tokenAnalysis.CurrentTokens, 10))
					if tokenAnalysis.ShouldTrim {
						c.Header("X-ProxyPilot-Token-Triggered", "true")
						c.Header("X-ProxyPilot-Target-Tokens", strconv.FormatInt(tokenAnalysis.TargetTokens, 10))
					}
				}
			}
		}

		// Session-scoped state (pinned + anchor + TODO + spec) is injected as append-only
		// scaffolding when enabled. This preserves prompt-cache friendliness.
		if agenticScaffoldEnabled() {
			session := extractAgenticSessionKey(req, body)
			body = agenticMaybeUpsertAndInjectPackedState(req, session, body, maxBytes, rootDir)
			originalLen = len(body)
			// Recompute token analysis after scaffold injection since body size changed
			tokenAnalysis = analyzeTokenBudget(body)
			if tokenAnalysis.ShouldTrim {
				maxBytes = tokenAnalysis.TargetMaxBytes
			}
		}

		// Proactive compression: trim if over token threshold OR over byte limit
		needsTrim := tokenAnalysis.ShouldTrim || originalLen > maxBytes
		if !needsTrim {
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
			agenticStoreAndInjectMemory(c, req, session, res, maxBytes)
			trimmed = res.Body
		case strings.HasSuffix(path, "/v1/responses"):
			res := trimOpenAIResponsesWithMemory(trimmed, maxBytes, mustKeepTools)
			trimmed = res.Body
			agenticStoreAndInjectMemory(c, req, session, res, maxBytes)
			trimmed = res.Body
		case strings.HasSuffix(path, "/v1/messages"):
			// Claude Messages API uses similar structure to chat completions
			res := trimClaudeMessagesWithMemory(trimmed, maxBytes, mustKeepTools)
			trimmed = res.Body
			agenticStoreAndInjectMemory(c, req, session, res, maxBytes)
			trimmed = res.Body
		default:
			// Not a known payload shape; keep as-is.
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

func agenticMaybeUpsertAndInjectPackedState(req *http.Request, session string, body []byte, maxBytes int, rootDir string) []byte {
	if req == nil || session == "" || len(body) == 0 {
		return body
	}

	// Always strip internal headers to prevent forwarding upstream, regardless of memory store availability
	todoHeader := strings.TrimSpace(req.Header.Get("X-CLIProxyAPI-Todo"))
	req.Header.Del("X-CLIProxyAPI-Todo")

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
	if todoHeader != "" {
		_ = fs.WriteTodo(session, todoHeader, 8000)
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
	if agents := readAgentsMarkdown(rootDir); strings.TrimSpace(agents) != "" {
		pinned = mergePinned(pinned, agents)
	}
	if agenticAnchorAppendOnly() {
		if pending := strings.TrimSpace(fs.ReadPendingAnchor(session, 4000)); pending != "" {
			block := buildAnchorBlock(pending)
			if out, ok := appendSystemBlock(shape, body, block, maxBytes); ok {
				body = out
				_ = fs.ClearPendingAnchor(session)
			}
		}
	}

	anchor := ""
	if !agenticAnchorAppendOnly() {
		anchor = fs.ReadSummary(session, 2500)
	}
	mem := ""
	if agenticSemanticEnabled() && !fs.IsSemanticDisabled(session) {
		ns := semanticNamespace(req, body, session)
		query := semanticQueryText(shape, body)
		query = strings.TrimSpace(query)
		if query != "" {
			if maxChars := agenticSemanticQueryMaxChars(); maxChars > 0 && len(query) > maxChars {
				query = query[:maxChars] + "\n...[truncated]..."
			}
		}
		if query != "" && allowSemanticWrite(session) {
			client := agenticSemanticClient()
			if client != nil {
				vecs, err := client.Embed(context.Background(), []string{query})
				if err == nil && len(vecs) > 0 && len(vecs[0]) > 0 {
					if snips, err := fs.SearchSemanticWithText(ns, vecs[0], query, agenticSemanticMaxChars(), agenticSemanticMaxSnips()); err == nil {
						mem = semanticBlockFromSnips(snips)
					}
					_ = fs.AppendSemantic(ns, []memory.SemanticRecord{
						{
							Role:    "user",
							Text:    query,
							Vec:     vecs[0],
							Source:  "query",
							Session: session,
							Repo:    ns,
						},
					})
				}
			}
		}
	}
	spec := ""
	if agenticSpecModeEnabled(req, body) && !agenticSpecApproved(body) {
		spec = specModePrompt
	}
	block := buildPackedState(pinned, anchor, todo, mem, spec)
	if strings.TrimSpace(block) == "" {
		return body
	}
	if agenticScaffoldAppendOnly() {
		return appendScaffoldState(shape, body, block, maxBytes)
	}
	return prependToLastUserText(shape, body, block, maxBytes)
}

func detectShapeFromPath(path string) string {
	switch {
	case strings.HasSuffix(path, "/v1/chat/completions"):
		return "chat"
	case strings.HasSuffix(path, "/v1/responses"):
		return "responses"
	case strings.HasSuffix(path, "/v1/messages"):
		return "claude"
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
	case "claude":
		arr := gjson.GetBytes(body, "messages").Array()
		return extractLastUserTextFromClaude(arr)
	default:
		return ""
	}
}

func buildPackedState(pinned string, anchor string, todo string, mem string, spec string) string {
	pinned = strings.TrimSpace(pinned)
	anchor = strings.TrimSpace(anchor)
	todo = strings.TrimSpace(todo)
	mem = strings.TrimSpace(mem)
	spec = strings.TrimSpace(spec)
	if pinned == "" && anchor == "" && todo == "" && mem == "" && spec == "" {
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
	if mem != "" {
		b.WriteString("<memory>\n")
		b.WriteString(mem)
		b.WriteString("\n</memory>\n")
	}
	if spec != "" {
		b.WriteString("<spec>\n")
		b.WriteString(spec)
		b.WriteString("\n</spec>\n")
	}
	b.WriteString("</proxypilot_state>\n\n")
	return b.String()
}

func buildAnchorBlock(anchor string) string {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("<proxypilot_anchor>\n")
	b.WriteString(anchor)
	b.WriteString("\n</proxypilot_anchor>\n\n")
	return b.String()
}

func agenticSpecModeEnabled(req *http.Request, body []byte) bool {
	if req == nil {
		return false
	}
	if v := strings.TrimSpace(req.Header.Get("X-CLIProxyAPI-Spec-Mode")); v != "" {
		return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "on") || strings.EqualFold(v, "yes")
	}
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SPEC_MODE")); v != "" {
		return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "on") || strings.EqualFold(v, "yes")
	}
	if gjson.GetBytes(body, "spec_mode").Bool() {
		return true
	}
	return false
}

func agenticSpecApproved(body []byte) bool {
	raw := strings.ToLower(string(body))
	return strings.Contains(raw, "spec approved") ||
		strings.Contains(raw, "<spec_approved>") ||
		strings.Contains(raw, "spec_approved") ||
		strings.Contains(raw, "approved spec")
}

func readAgentsMarkdown(rootDir string) string {
	if strings.TrimSpace(rootDir) == "" {
		return ""
	}
	path := filepath.Join(rootDir, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	out := strings.TrimSpace(string(data))
	if out == "" {
		return ""
	}
	if len(out) > 6000 {
		out = out[:6000] + "\n...[truncated]..."
	}
	return out
}

func mergePinned(pinned string, agents string) string {
	pinned = strings.TrimSpace(pinned)
	agents = strings.TrimSpace(agents)
	if agents == "" {
		return pinned
	}
	if pinned == "" {
		return agents
	}
	if strings.Contains(pinned, agents) {
		return pinned
	}
	return pinned + "\n\n" + agents
}

func appendScaffoldState(shape string, body []byte, block string, maxBytes int) []byte {
	out, _ := appendSystemBlock(shape, body, block, maxBytes)
	return out
}

func appendSystemBlock(shape string, body []byte, block string, maxBytes int) ([]byte, bool) {
	block = strings.TrimSpace(block)
	if block == "" {
		return body, false
	}
	limit := maxBytes - len(body) - 512
	if maxBytes > 0 && limit <= 0 {
		return body, false
	}
	if maxBytes > 0 && len(block) > limit {
		block = block[:limit] + "\n...[truncated]..."
	}

	switch shape {
	case "responses":
		input := gjson.GetBytes(body, "input")
		if input.Exists() && input.IsArray() {
			entry := map[string]any{
				"role": "system",
				"content": []map[string]string{
					{"type": "input_text", "text": block},
				},
			}
			if out, err := sjson.SetBytes(body, "input.-1", entry); err == nil {
				return out, true
			}
		}
		out := injectMemoryIntoBody("responses", body, block, maxBytes)
		return out, out != nil && !bytes.Equal(out, body)
	case "chat":
		msgs := gjson.GetBytes(body, "messages")
		if msgs.Exists() && msgs.IsArray() {
			entry := map[string]string{
				"role":    "system",
				"content": block,
			}
			if out, err := sjson.SetBytes(body, "messages.-1", entry); err == nil {
				return out, true
			}
		}
		out := injectMemoryIntoBody("chat", body, block, maxBytes)
		return out, out != nil && !bytes.Equal(out, body)
	case "claude":
		// Claude Messages API prefers top-level "system"; append there as fallback.
		if sys := gjson.GetBytes(body, "system"); sys.Exists() && sys.Type == gjson.String {
			merged := strings.TrimSpace(sys.String()) + "\n\n" + block
			if out, err := sjson.SetBytes(body, "system", merged); err == nil {
				return out, true
			}
		}
		out := injectMemoryIntoBody("claude", body, block, maxBytes)
		return out, out != nil && !bytes.Equal(out, body)
	default:
		return body, false
	}
}

func semanticNamespace(req *http.Request, body []byte, session string) string {
	if req != nil {
		for _, h := range []string{"X-CLIProxyAPI-Repo", "X-Repo-Path", "X-Workspace-Root", "X-Project-Root"} {
			if v := strings.TrimSpace(req.Header.Get(h)); v != "" {
				return v
			}
		}
	}
	if v := strings.TrimSpace(gjson.GetBytes(body, "metadata.repo").String()); v != "" {
		return v
	}
	if v := strings.TrimSpace(gjson.GetBytes(body, "metadata.repo_path").String()); v != "" {
		return v
	}
	if v := strings.TrimSpace(gjson.GetBytes(body, "metadata.workspace_root").String()); v != "" {
		return v
	}
	if v := strings.TrimSpace(gjson.GetBytes(body, "repo").String()); v != "" {
		return v
	}
	if v := strings.TrimSpace(gjson.GetBytes(body, "workspace_root").String()); v != "" {
		return v
	}
	return session
}

func semanticQueryText(shape string, body []byte) string {
	return extractLastUserIntent(shape, body)
}

func semanticBlockFromSnips(snips []string) string {
	if len(snips) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Relevant prior context (semantic):\n")
	for i := range snips {
		b.WriteString("\n---\n")
		b.WriteString(snips[i])
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func collectSemanticTexts(dropped []memory.Event, maxItems int) ([]string, []string) {
	if maxItems <= 0 {
		maxItems = 12
	}
	out := make([]string, 0, maxItems)
	roles := make([]string, 0, maxItems)
	for i := len(dropped) - 1; i >= 0; i-- {
		if len(out) >= maxItems {
			break
		}
		txt := strings.TrimSpace(dropped[i].Text)
		if txt == "" {
			continue
		}
		if len(txt) > 800 {
			txt = txt[:800] + "\n...[truncated]..."
		}
		out = append(out, txt)
		roles = append(roles, dropped[i].Role)
	}
	return out, roles
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
	case "claude":
		// Claude Messages API uses messages array with content blocks
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
				// Find first text block
				parts := content.Array()
				for j := 0; j < len(parts); j++ {
					if parts[j].Get("type").String() != "text" {
						continue
					}
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
	case "claude":
		// Claude Messages API uses messages array with content blocks
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
				// Find first text block
				parts := content.Array()
				for j := 0; j < len(parts); j++ {
					if parts[j].Get("type").String() != "text" {
						continue
					}
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

func agenticStoreAndInjectMemory(c *gin.Context, req *http.Request, session string, res *trimWithMemoryResult, maxBytes int) {
	if req == nil || res == nil {
		return
	}
	if session == "" {
		return
	}

	// Set session header for diagnostics (only for localhost)
	if c != nil {
		ip := c.ClientIP()
		if ip == "127.0.0.1" || ip == "::1" {
			c.Header("X-ProxyPilot-Session", session)
			c.Header("X-ProxyPilot-Request-Shape", res.Shape)
		}
	}

	store := agenticMemoryStore()
	if store == nil {
		return
	}

	if len(res.Dropped) > 0 {
		stored := false
		if allowMemoryWrite(session) {
			_ = store.Append(session, res.Dropped)
			stored = true
		} else if c != nil {
			ip := c.ClientIP()
			if ip == "127.0.0.1" || ip == "::1" {
				c.Header("X-ProxyPilot-Memory-Limited", "true")
			}
		}
		// Indicate memory was stored
		if stored && c != nil {
			ip := c.ClientIP()
			if ip == "127.0.0.1" || ip == "::1" {
				c.Header("X-ProxyPilot-Memory-Stored", strconv.Itoa(len(res.Dropped)))
			}
		}
	}

	// Update anchored summary and pinned context (best-effort).
	if fs, ok := store.(*memory.FileStore); ok {
		pinned := extractPinnedContext(req, res.Shape, res.Body)
		if pinned != "" {
			_ = fs.WritePinned(session, pinned, 8000)
		}
		if len(res.Dropped) > 0 {
			if agenticLLMSummaryEnabled() {
				model := gjson.GetBytes(res.Body, "model").String()
				ctx := c.Request.Context()
				_ = agenticUpdateAnchoredSummaryWithLLM(ctx, model, fs, session, res.Dropped, pinned, res.Query)
			} else {
				_ = agenticUpdateAnchoredSummary(fs, session, res.Dropped, pinned, res.Query)
			}
		}
		if agenticSemanticEnabled() && len(res.Dropped) > 0 && !fs.IsSemanticDisabled(session) {
			ns := semanticNamespace(req, res.Body, session)
			if allowSemanticWrite(session) {
				texts, roles := collectSemanticTexts(res.Dropped, 12)
				if len(texts) > 0 {
					enqueueSemanticEmbeds(fs, ns, session, texts, roles, "dropped")
				}
			} else if c != nil {
				ip := c.ClientIP()
				if ip == "127.0.0.1" || ip == "::1" {
					c.Header("X-ProxyPilot-Semantic-Limited", "true")
				}
			}
		}
	}

	// Only inject retrieval when we actually trimmed (otherwise it just spends tokens).
	// Also avoid injecting if tools were forcibly disabled by the client.
	if strings.TrimSpace(res.Query) == "" {
		agenticMaybePruneMemory()
		return
	}

	maxSnips := 8
	maxChars := 6000
	snips, err := store.Search(session, res.Query, maxChars, maxSnips)
	if err != nil || len(snips) == 0 {
		return
	}

	// Indicate memory was retrieved and injected
	if c != nil {
		ip := c.ClientIP()
		if ip == "127.0.0.1" || ip == "::1" {
			c.Header("X-ProxyPilot-Memory-Retrieved", strconv.Itoa(len(snips)))
		}
	}

	memBlock := buildMemoryBlock(snips)
	res.Body = appendToLastUserText(res.Shape, res.Body, memBlock, maxBytes)

	agenticMaybePruneMemory()
}

func agenticUpdateAnchoredSummary(fs *memory.FileStore, session string, dropped []memory.Event, pinned string, latestIntent string) error {
	if fs == nil || session == "" {
		return nil
	}
	prev := fs.ReadSummary(session, agenticAnchorSummaryMaxChars())
	next := memory.BuildAnchoredSummary(prev, dropped, latestIntent)
	if strings.TrimSpace(next) == "" {
		return nil
	}
	if agenticAnchorAppendOnly() {
		return fs.SetAnchorSummary(session, next, agenticAnchorSummaryMaxChars())
	}
	return fs.WriteSummary(session, next, agenticAnchorSummaryMaxChars())
}

// agenticUpdateAnchoredSummaryWithLLM updates the anchored summary using LLM.
func agenticUpdateAnchoredSummaryWithLLM(ctx context.Context, model string, fs *memory.FileStore, session string, dropped []memory.Event, pinned string, latestIntent string) error {
	if fs == nil || session == "" {
		return nil
	}
	summarizer := fs.GetSummarizer()
	if summarizer == nil {
		// Fall back to regex-based
		return agenticUpdateAnchoredSummary(fs, session, dropped, pinned, latestIntent)
	}

	prev := fs.ReadSummary(session, agenticAnchorSummaryMaxChars())
	next := memory.BuildAnchoredSummaryWithLLM(ctx, model, prev, dropped, latestIntent, summarizer)
	if strings.TrimSpace(next) == "" {
		return nil
	}
	if agenticAnchorAppendOnly() {
		return fs.SetAnchorSummary(session, next, agenticAnchorSummaryMaxChars())
	}
	return fs.WriteSummary(session, next, agenticAnchorSummaryMaxChars())
}

func agenticMaybePruneMemory() {
	maxAge := agenticMemoryMaxAgeDays()
	maxSessions := agenticMemoryMaxSessions()
	maxBytes := agenticMemoryMaxBytesPerSession()
	maxNamespaces := agenticSemanticMaxNamespaces()
	maxBytesNamespace := agenticSemanticMaxBytesPerNamespace()
	if maxAge <= 0 && maxSessions <= 0 && maxBytes <= 0 && maxNamespaces <= 0 && maxBytesNamespace <= 0 {
		return
	}
	pruneMu.Lock()
	if time.Since(lastPrune) < 10*time.Minute {
		pruneMu.Unlock()
		return
	}
	lastPrune = time.Now()
	pruneMu.Unlock()

	store := agenticMemoryStore()
	fs, ok := store.(*memory.FileStore)
	if !ok || fs == nil {
		return
	}
	_, _ = fs.PruneSessions(maxAge, maxSessions, maxBytes)
	_, _ = fs.PruneSemantic(maxAge, maxNamespaces, maxBytesNamespace)
}

func extractPinnedContext(req *http.Request, shape string, body []byte) string {
	// Pinned is intended to capture durable "always-on" state: system instructions / policies.
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
	case "claude":
		// Claude Messages API uses "system" field at root level for system prompt
		if v := gjson.GetBytes(body, "system"); v.Exists() && v.Type == gjson.String {
			s := strings.TrimSpace(v.String())
			if len(s) > 6000 {
				s = s[:6000] + "\n...[truncated]..."
			}
			return s
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

func agenticSemanticQueue() *semanticEmbedQueue {
	embedQueueOnce.Do(func() {
		store := agenticMemoryStore()
		fs, _ := store.(*memory.FileStore)
		embedQueue = &semanticEmbedQueue{
			ch: make(chan semanticEmbedTask, 64),
			fs: fs,
		}
		go embedQueue.run()
	})
	return embedQueue
}

func (q *semanticEmbedQueue) run() {
	for task := range q.ch {
		if q == nil || q.fs == nil {
			continue
		}
		if len(task.texts) == 0 {
			continue
		}
		client := agenticSemanticClient()
		if client == nil {
			continue
		}
		vecs, err := client.Embed(context.Background(), task.texts)
		if err != nil || len(vecs) != len(task.texts) {
			memory.IncSemanticFailed(len(task.texts))
			time.Sleep(2 * time.Second)
			continue
		}
		records := make([]memory.SemanticRecord, 0, len(task.texts))
		for i := range task.texts {
			if len(vecs[i]) == 0 {
				continue
			}
			role := ""
			if i < len(task.roles) {
				role = task.roles[i]
			}
			records = append(records, memory.SemanticRecord{
				Role:    role,
				Text:    task.texts[i],
				Vec:     vecs[i],
				Source:  task.source,
				Session: task.session,
				Repo:    task.namespace,
			})
		}
		if len(records) > 0 {
			_ = q.fs.AppendSemantic(task.namespace, records)
			memory.IncSemanticProcessed(len(records))
		}
	}
}

func enqueueSemanticEmbeds(fs *memory.FileStore, namespace string, session string, texts []string, roles []string, source string) {
	if fs == nil || namespace == "" || len(texts) == 0 {
		return
	}
	q := agenticSemanticQueue()
	if q == nil {
		return
	}
	if q.fs == nil {
		q.fs = fs
	}
	task := semanticEmbedTask{namespace: namespace, session: session, texts: texts, roles: roles, source: source}
	memory.IncSemanticQueued(len(texts))
	select {
	case q.ch <- task:
	default:
		// Drop if the queue is full to avoid backpressure.
		memory.IncSemanticDropped(len(texts))
	}
}
