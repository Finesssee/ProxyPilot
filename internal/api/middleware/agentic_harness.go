package middleware

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	harnessEnabledOnce sync.Once
	harnessEnabled     bool
)

// isHarnessEnabled checks if the agentic harness is enabled via environment variable.
// Defaults to true if CLIPROXY_HARNESS_ENABLED is not set or is "true"/"1".
func isHarnessEnabled() bool {
	harnessEnabledOnce.Do(func() {
		val := os.Getenv("CLIPROXY_HARNESS_ENABLED")
		if val == "" {
			harnessEnabled = true
			return
		}
		val = strings.ToLower(strings.TrimSpace(val))
		harnessEnabled = val == "true" || val == "1" || val == "yes"
	})
	return harnessEnabled
}

const (
	// Initializer prompt: forces the agent to set up the environment.
	harnessInitializerPrompt = `
<harness_mode>INITIALIZER</harness_mode>
You are the **Initializer Agent** in a Long-Running Agent Harness.
Your ONLY goal is to set up the project environment for future Coding Agents.

## CRITICAL: YOU DO NOT IMPLEMENT FEATURES
Your job is ONLY to scaffold the harness files. Do NOT write any application code.
Do NOT create components, pages, or business logic. Only create the harness metadata files.

## INSTRUCTIONS

### 1. Analyze the User's Request
Understand what feature or app needs to be built. Break it down into discrete, testable features.

### 2. Create 'feature_list.json'
Write a JSON file listing ALL features/requirements as separate items.

**Required Format:**
{
  "features": [
    {
      "id": "feat-001",
      "category": "core",
      "description": "User can add a todo item",
      "steps": [
        "Open the application",
        "Enter text in the input field",
        "Press Enter or click Add button",
        "Verify new item appears in the list"
      ],
      "passes": false,
      "priority": 1
    },
    {
      "id": "feat-002",
      "category": "core",
      "description": "User can delete a todo item",
      "steps": [
        "Open the application with existing items",
        "Click the delete button on an item",
        "Verify item is removed from the list"
      ],
      "passes": false,
      "priority": 2
    }
  ]
}

**Field Descriptions:**
- id: Unique identifier (feat-XXX format)
- category: "core", "enhancement", "ui", "integration", etc.
- description: Clear, testable user-facing behavior
- steps: Exact steps to verify this feature works
- passes: Always false initially
- priority: Lower number = higher priority (implement first)

### 3. Create 'claude-progress.txt'
Create this file with structured session logging:

## Progress Log

### Session: YYYY-MM-DD HH:MM
- [Initializer] Environment setup started.
- [Initializer] Created feature_list.json with N features.
- [Initializer] Created init.sh for development server.

### 4. Create 'init.sh' (REQUIRED for web apps)
For ANY web application, you MUST create init.sh:
#!/bin/bash
# Start the development server
npm install
npm run dev &
sleep 5
echo "Dev server started at http://localhost:3000"

Make it executable and ensure it handles common setup tasks.

### 5. Initial Commit
- Initialize git if not present.
- Commit these foundational files with message: "chore: initialize agent harness"

## REMINDER
You are ONLY scaffolding. The Coding Agent will implement features in future turns.
Do NOT write application code. Do NOT create UI components. ONLY create the harness files.
`

	// Coding prompt: forces the agent to work incrementally.
	harnessCodingPrompt = `
<harness_mode>CODING</harness_mode>
You are a **Coding Agent** in a Long-Running Agent Harness.
Your goal is to make **incremental progress** on the project.

## STEP 1: GET BEARINGS (MANDATORY - DO THIS FIRST)
Before ANY coding, you MUST execute these commands in order:

1. pwd                              # Know where you are
2. git log --oneline -5             # See recent changes
3. cat claude-progress.txt          # Read what was done before
4. cat feature_list.json            # See all features and their status
5. ./init.sh                        # Start dev server if it exists

Do NOT skip this step. Do NOT assume you know the project state.

## STEP 2: SANITY CHECK (MANDATORY)
Before implementing anything new:
- Verify existing features still work
- Check for any regressions from previous changes
- If something is broken, FIX IT FIRST before new work

## STEP 3: PICK ONE TASK
- Find the highest-priority feature where "passes": false
- Announce: "I will implement: [feature description]"
- Do NOT pick multiple features

## STEP 4: IMPLEMENT & VERIFY
- Write code for that ONE feature only
- You MUST verify it works before marking complete
- Verification methods (in order of preference):
  1. **Run tests**: Execute unit/integration test suites
  2. **Build check**: Ensure the project compiles without errors
  3. **Manual verification**: Test the feature manually if automated tests unavailable

**VERIFICATION IS NOT OPTIONAL.** You must ACTUALLY TEST the feature.
Do NOT mark a feature as passing based on "the code looks correct."

## STEP 5: UPDATE STATE
After VERIFIED implementation:

1. Update feature_list.json:
   - Set "passes": true ONLY for features you have ACTUALLY VERIFIED

2. Append to claude-progress.txt:
   - [Coding] Implemented <feature id>: <description>. Verified via <method>.

   Example:
   - [Coding] Implemented feat-003: User can mark todo complete. Verified via unit tests.

## STEP 6: COMMIT
- Git commit with descriptive message
- Include feature ID in commit message

## RESTRICTIONS
- Do NOT try to finish the whole project in one turn
- Do NOT leave the code in a broken state
- Do NOT mark features as "passes": true without ACTUAL verification
- Do NOT edit feature_list.json except to toggle "passes" after verified testing
- ALWAYS update harness files before stopping
`
)

// AgenticHarnessMiddleware injects system prompts to guide long-running agents.
func AgenticHarnessMiddleware() gin.HandlerFunc {
	return AgenticHarnessMiddlewareWithRootDir("")
}

// AgenticHarnessMiddlewareWithRootDir injects system prompts and checks harness files under rootDir.
// If rootDir is empty, the current working directory is used.
func AgenticHarnessMiddlewareWithRootDir(rootDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if harness is enabled via environment variable
		if !isHarnessEnabled() {
			c.Next()
			return
		}

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
		// We also check conversation length and session-based storage.
		state := detectHarnessState(body, c, rootDir)

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
		if !bytes.Equal(newBody, body) {
			req.Body = io.NopCloser(bytes.NewReader(newBody))
			req.ContentLength = int64(len(newBody))
			req.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
			c.Header("X-ProxyPilot-Harness-Mode", state)
		}

		c.Next()
	}
}

// extractSessionKeyFromContext extracts session key from request headers.
func extractSessionKeyFromContext(c *gin.Context) string {
	if v := c.GetHeader("X-CLIProxyAPI-Session"); v != "" {
		return v
	}
	if v := c.GetHeader("X-Session-Id"); v != "" {
		return v
	}
	return ""
}

// extractSessionKeyFromBody extracts session key from request body JSON fields.
func extractSessionKeyFromBody(body []byte) string {
	if v := gjson.GetBytes(body, "prompt_cache_key"); v.Exists() && v.Type == gjson.String {
		return v.String()
	}
	if v := gjson.GetBytes(body, "metadata.session_id"); v.Exists() && v.Type == gjson.String {
		return v.String()
	}
	if v := gjson.GetBytes(body, "session_id"); v.Exists() && v.Type == gjson.String {
		return v.String()
	}
	return ""
}

// memoryBaseDirForHarness returns the base directory for memory/session storage.
func memoryBaseDirForHarness() string {
	if v := os.Getenv("CLIPROXY_MEMORY_DIR"); v != "" {
		return v
	}
	// Try default locations
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".proxypilot", "memory")
	}
	return filepath.Join(".", ".proxypilot", "memory")
}

// harnessFileVariations lists possible file name variations for harness files.
var harnessProgressFileVariations = []string{
	"claude-progress.txt",
	"claude-progress",
	"progress.txt",
}

var harnessFeatureFileVariations = []string{
	"feature_list.json",
	"feature_list",
	"features.json",
}

// checkHarnessFilesExist checks if any harness files exist in the given directory.
func checkHarnessFilesExist(dir string) bool {
	for _, f := range harnessProgressFileVariations {
		if fileExists(dir, f) {
			return true
		}
	}
	for _, f := range harnessFeatureFileVariations {
		if fileExists(dir, f) {
			return true
		}
	}
	return false
}

// projectIndicators are files/dirs that indicate an established codebase.
// If any of these exist, we skip the INITIALIZER harness to avoid polluting real projects.
var projectIndicators = []string{
	".git",
	"package.json",
	"go.mod",
	"Cargo.toml",
	"pyproject.toml",
	"requirements.txt",
	"pom.xml",
	"build.gradle",
	"Makefile",
	"CMakeLists.txt",
	".sln",
	"composer.json",
	"Gemfile",
	"mix.exs",
	"pubspec.yaml",
	"deno.json",
	"bun.lockb",
}

// isEstablishedProject checks if the directory contains common project indicators.
// Returns true if this appears to be an existing codebase (not a fresh project).
func isEstablishedProject(rootDir string) bool {
	if rootDir == "" {
		// Check current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return false
		}
		rootDir = cwd
	}

	for _, indicator := range projectIndicators {
		target := filepath.Join(rootDir, indicator)
		if info, err := os.Stat(target); err == nil {
			// Exists - could be file or directory
			_ = info
			return true
		}
	}
	return false
}

func detectHarnessState(body []byte, c *gin.Context, rootDir string) string {
	// Simple heuristic:
	// If the conversation contains references to "claude-progress.txt" or "feature_list.json",
	// it's likely already in the harness flow -> CODING.
	// If it's very short (just the user prompt) and lacks those, -> INITIALIZER.
	// BUT: If the directory is an established project, skip harness entirely -> PASSIVE.

	// First check: Is this an established codebase? If so, don't inject harness prompts.
	if isEstablishedProject(rootDir) {
		return "PASSIVE"
	}

	raw := string(body)
	if strings.Contains(raw, "claude-progress.txt") || strings.Contains(raw, "feature_list.json") {
		return "CODING"
	}

	// Try to get session key from context headers first, then from body
	sessionKey := extractSessionKeyFromContext(c)
	if sessionKey == "" {
		sessionKey = extractSessionKeyFromBody(body)
	}

	// Check session-based storage for harness files
	if sessionKey != "" {
		memoryDir := memoryBaseDirForHarness()
		sessionHarnessDir := filepath.Join(memoryDir, "sessions", sessionKey, "harness")
		if checkHarnessFilesExist(sessionHarnessDir) {
			return "CODING"
		}
	}

	// Prefer filesystem truth when available (check rootDir with file variations).
	if checkHarnessFilesExist(rootDir) {
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

	// Responses API: check "input" array length if present.
	input := gjson.GetBytes(body, "input")
	if input.Exists() && input.IsArray() {
		if len(input.Array()) < 5 {
			return "INITIALIZER"
		}
	}
	if input.Exists() && input.Type == gjson.String {
		return "INITIALIZER"
	}

	// If it's a long conversation but no harness files mentioned,
	// maybe the user didn't want the harness, or it's a legacy chat.
	// We'll default to PASSIVE to avoid annoying the user.
	return "PASSIVE"
}

func injectSystemPrompt(body []byte, prompt string) []byte {
	// We prepend this prompt to the existing system prompt or messages.
	// This logic handles OAI /v1/chat/completions, Claude /v1/messages, and OAI /v1/responses.

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

	// 2b. Responses API top-level "instructions" or "input"
	if inst := gjson.GetBytes(body, "instructions"); inst.Exists() && inst.Type == gjson.String {
		old := inst.String()
		newText := old + "\n\n" + prompt
		out, _ := sjson.SetBytes(body, "instructions", newText)
		return out
	}
	if input := gjson.GetBytes(body, "input"); input.Exists() {
		if input.Type == gjson.String {
			old := input.String()
			newText := prompt + "\n\n" + old
			out, _ := sjson.SetBytes(body, "input", newText)
			return out
		}
		if input.IsArray() {
			return prependHarnessToResponsesInput(body, prompt)
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

func prependHarnessToResponsesInput(body []byte, prefix string) []byte {
	prefix = "\n\n[SYSTEM NOTE: " + strings.TrimSpace(prefix) + "]\n\n"

	input := gjson.GetBytes(body, "input")
	if !input.Exists() || !input.IsArray() {
		return body
	}
	arr := input.Array()

	for i := len(arr) - 1; i >= 0; i-- {
		role := arr[i].Get("role").String()
		if !strings.EqualFold(role, "user") {
			continue
		}

		// String content
		content := arr[i].Get("content")
		if content.Type == gjson.String {
			old := content.String()
			newText := prefix + old
			out, err := sjson.SetBytes(body, "input."+strconv.Itoa(i)+".content", newText)
			if err == nil {
				return out
			}
		}

		// Array content (multimodal)
		if content.IsArray() {
			parts := content.Array()
			for j := 0; j < len(parts); j++ {
				if parts[j].Get("type").String() == "text" || parts[j].Get("text").Exists() {
					old := parts[j].Get("text").String()
					newText := prefix + old
					out, err := sjson.SetBytes(body, "input."+strconv.Itoa(i)+".content."+strconv.Itoa(j)+".text", newText)
					if err == nil {
						return out
					}
				}
			}
		}

		return body
	}

	return body
}

func fileExists(rootDir, path string) bool {
	target := path
	if rootDir != "" && !filepath.IsAbs(path) {
		target = filepath.Join(rootDir, path)
	}
	info, err := os.Stat(target)
	return err == nil && !info.IsDir()
}
