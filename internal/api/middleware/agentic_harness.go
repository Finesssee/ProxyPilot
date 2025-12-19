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
	// Initializer prompt: forces the agent to set up the environment.
	harnessInitializerPrompt = `
<harness_mode>INITIALIZER</harness_mode>
You are the **Initializer Agent** in a Long-Running Agent Harness.
Your ONLY goal is to set up the project environment for future Coding Agents.

**CRITICAL INSTRUCTIONS**:
1.  **Analyze the User's Request**: Understand what feature or app needs to be built.
2.  **Create 'feature_list.json'**:
    -   Write a JSON file listing ALL features/requirements as separate items.
    -   Mark them all as "passes": false.
    -   Format:
        Request: "Build a todo app"
        File Content:
        [
          { "category": "core", "description": "User can add a todo", "passes": false },
          { "category": "core", "description": "User can delete a todo", "passes": false }
        ]
3.  **Create 'claude-progress.txt'**:
    -   Create this file with a header "## Progress Log".
    -   Add an initial entry: "- [Initializer] Environment setup started."
4.  **Create 'init.sh' (Optional)**:
    -   If a dev server is needed, create a script to start it (e.g., 'npm run dev').
5.  **Initial Commit**:
    -   Initialize git if not present.
    -   Commit these foundational files.

**DO NOT** start implementing features yourself. Your job is ONLY to scaffold the harness.
`

	// Coding prompt: forces the agent to work incrementally.
	harnessCodingPrompt = `
<harness_mode>CODING</harness_mode>
You are a **Coding Agent** in a Long-Running Agent Harness.
Your goal is to make **incremental progress** on the project.

**WORKFLOW**:
1.  **Read Context**:
    -   Read 'claude-progress.txt' to see what was done last.
    -   Read 'feature_list.json' to see what is missing.
2.  **Pick ONE Task**:
    -   Choose the highest-priority failing feature from 'feature_list.json'.
    -   Announce your plan.
3.  **Implement & Verify**:
    -   Write code for that ONE feature.
    -   Verify it works (run tests or browser check).
4.  **Update State**:
    -   Update 'feature_list.json': set "passes": true for the completed feature.
    -   Append to 'claude-progress.txt': "- [Coding] Implemented <feature>."
5.  **Commit**:
    -   Git commit your changes.

**RESTRICTIONS**:
-   Do NOT try to finish the whole project in one turn.
-   Do NOT leave the code in a broken state.
-   Always update the harness files ('feature_list.json', 'claude-progress.txt') before stopping.
`
)

// AgenticHarnessMiddleware injects system prompts to guide long-running agents.
func AgenticHarnessMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		if req == nil || req.Method != http.MethodPost {
			c.Next()
			return
		}

		// 1. Check eligibility
		ua := strings.ToLower(req.Header.Get("User-Agent"))
		isAgenticCLI := strings.Contains(ua, "claude-cli") ||
			strings.Contains(ua, "codex") ||
			strings.Contains(ua, "droid") ||
			req.Header.Get("X-ProxyPilot-Harness") == "true"

		if !isAgenticCLI {
			c.Next()
			return
		}

		// 2. Read Body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			c.Next()
			return
		}
		// restore body for next handler if we bail
		req.Body = io.NopCloser(bytes.NewReader(body))

		// 3. Detect State
		// We look for evidence that the harness is already active (features/progress files).
		// We also check conversation length.
		state := detectHarnessState(body)

		// 4. Inject Prompt
		var promptToInject string
		switch state {
		case "INITIALIZER":
			promptToInject = harnessInitializerPrompt
		case "CODING":
			promptToInject = harnessCodingPrompt
		default:
			// "PASSIVE" - do nothing, let the user drive.
			c.Next()
			return
		}

		newBody := injectSystemPrompt(body, promptToInject)

		// 5. Update Request
		if len(newBody) != len(body) {
			req.Body = io.NopCloser(bytes.NewReader(newBody))
			req.ContentLength = int64(len(newBody))
			req.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
			c.Header("X-ProxyPilot-Harness-Mode", state)
		}

		c.Next()
	}
}

func detectHarnessState(body []byte) string {
	// Simple heuristic:
	// If the conversation contains references to "claude-progress.txt" or "feature_list.json",
	// it's likely already in the harness flow -> CODING.
	// If it's very short (just the user prompt) and lacks those, -> INITIALIZER.

	raw := string(body)
	if strings.Contains(raw, "claude-progress.txt") || strings.Contains(raw, "feature_list.json") {
		return "CODING"
	}

	// Check conversation depth
	// For OAI/Claude, we look at "messages" array length.
	msgs := gjson.GetBytes(body, "messages")
	if msgs.Exists() && msgs.IsArray() {
		count := len(msgs.Array())
		// Usually: System + User = 2 messages for a fresh start.
		// If it's small, and we haven't seen progress files, assume we want to initialize.
		// We use < 5 to be safe (System, User, maybe a tool use in between).
		if count < 5 {
			return "INITIALIZER"
		}
	}

	// If it's a long conversation but no harness files mentioned,
	// maybe the user didn't want the harness, or it's a legacy chat.
	// We'll default to PASSIVE to avoid annoying the user.
	return "PASSIVE"
}

func injectSystemPrompt(body []byte, prompt string) []byte {
	// We prepend this prompt to the existing system prompt or messages.
	// This logic handles OAI /v1/chat/completions and Claude /v1/messages structure.

	// 1. Try "messages" (Chat/Claude)
	msgs := gjson.GetBytes(body, "messages")
	if msgs.Exists() && msgs.IsArray() {
		arr := msgs.Array()

		// Look for existing system message to append to
		for i, m := range arr {
			if strings.EqualFold(m.Get("role").String(), "system") {
				content := m.Get("content")
				if content.Type == gjson.String {
					old := content.String()
					newText := old + "\n\n" + prompt
					out, _ := sjson.SetBytes(body, "messages."+strconv.Itoa(i)+".content", newText)
					return out
				}
				// If array content (Claude), append text block?
				// Simpler to just Insert a new system message if complex.
			}
		}

		// If no system message found, prepend one.
		// Construct new message object
		// Note: For Claude /v1/messages, "system" is arguably a top-level field,
		// but `claude-cli` often speaks OAI dialect or we map it.
		// Let's check for top-level system field first for pure Claude Messages API.
	}

	// 2. Claude Messages API top-level "system"
	if sys := gjson.GetBytes(body, "system"); sys.Exists() {
		if sys.Type == gjson.String {
			old := sys.String()
			newText := old + "\n\n" + prompt
			out, _ := sjson.SetBytes(body, "system", newText)
			return out
		}
	}

	// 3. Fallback: Prepend a system message to "messages" array
	// Valid for OAI. For Claude, if top-level system is missing, we might use this
	// but strictly Claude Messages API wants top-level.
	// Let's assume standard OAI-like messages array is the target for now.
	// newMsg map removed as it was unused and causing lint errors
	// We proceed to check for Claude or use prependHarnessToLastUserText.

	// sjson doesn't easily "prepend" to array without reading it all.
	// But we can use sjson.Set to insert at index 0?
	// sjson path "messages.-1" appends. "messages.0" overwrites.
	// We might need to unmarshal/marshal if sjson is too tricky for insert.
	// Actually, creating a new body string is safer/easier than strict JSON manipulation for insertion.
	// But let's try sjson properly:

	// If we must insert at the front, sjson isn't great.
	// Strategy: If existing systems exist, we updated them above.
	// If NOT, we need to add one.

	// Let's try to set "system" top level again if it's empty (Anthropic specific)
	// If the request body HAS "max_tokens" (Claude) but NO "messages" (Completion), handle that?
	// Assuming Chat/Messages API.

	// If it's Claude-style body without system:
	if gjson.GetBytes(body, "messages").Exists() {
		// If checking for Claude-specifics:
		if gjson.GetBytes(body, "anthropic_version").Exists() {
			// It is Claude Messages API. Set top-level system.
			out, _ := sjson.SetBytes(body, "system", prompt)
			return out
		}

		// Otherwise assume OAI Chat. Prepend to messages.
		// "messages.0" -> insert? No, sjson overwrites.
		// We'll read the whole array, prepend in Go, write back.
		// This is expensive but safe.
		// Wait, we can cheat: Just append the prompt to the *last user message*.
		// This is what `codex_prompt_budget.go` does (`prependToLastUserText`).
		// It's robust and works for all models (context is context).
		// AND it avoids "multiple system messages" issues some models hate.

		return prependHarnessToLastUserText(body, prompt)
	}

	return body
}

// Reuse logic similar to codex_prompt_budget but simplified for just prepending to USER message.
// This ensures the model "hears" it as part of the latest command.
func prependHarnessToLastUserText(body []byte, prefix string) []byte {
	prefix = "\n\n[SYSTEM NOTE: " + strings.TrimSpace(prefix) + "]\n\n"

	msgs := gjson.GetBytes(body, "messages")
	if !msgs.Exists() || !msgs.IsArray() {
		return body
	}
	arr := msgs.Array()

	// Find last user message
	for i := len(arr) - 1; i >= 0; i-- {
		if strings.EqualFold(arr[i].Get("role").String(), "user") {
			content := arr[i].Get("content")

			// String content
			if content.Type == gjson.String {
				old := content.String()
				newText := prefix + old
				out, err := sjson.SetBytes(body, "messages."+strconv.Itoa(i)+".content", newText)
				if err == nil {
					return out
				}
			}

			// Array content (multimodal)
			if content.IsArray() {
				parts := content.Array()
				for j := 0; j < len(parts); j++ {
					// Find text part
					if parts[j].Get("type").String() == "text" || parts[j].Get("text").Exists() {
						old := parts[j].Get("text").String()
						newText := prefix + old
						out, err := sjson.SetBytes(body, "messages."+strconv.Itoa(i)+".content."+strconv.Itoa(j)+".text", newText)
						if err == nil {
							return out
						}
					}
				}
			}
			return body // Found user but failed to inject?
		}
	}
	return body
}
