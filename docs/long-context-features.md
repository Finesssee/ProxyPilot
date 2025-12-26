# Long-Context Features in ProxyPilot

ProxyPilot implements two research-backed features for handling long-running AI agent sessions:

1. **Context Compression** - Based on Factory.ai research
2. **Agentic Harness** - Based on Anthropic's "Building Effective Agents" blog

Both are enabled by default and can be toggled via environment variables.

---

## Sources

| Feature | Source | Link |
|---------|--------|------|
| Context Compression | Factory.ai Research | [Context Compression for Long-Running Agents](https://www.factory.ai/news/context-compression) |
| Agentic Harness | Anthropic Blog | [Building Effective Agents](https://www.anthropic.com/research/building-effective-agents) |

---

## 1. Context Compression (Factory.ai)

### What Factory.ai Found

Factory.ai researched how to handle long coding sessions where conversation context exceeds model limits. Their key findings:

- **Naive truncation fails** - Cutting old messages loses critical context
- **Structured summarization works** - Preserving key information in a specific format maintains continuity
- **Anchored approach is best** - Merging new info into existing summaries beats creating fresh ones

### How ProxyPilot Implements It

When conversation exceeds 75% of the model's context window:

1. **Trigger**: Token count check detects threshold exceeded
2. **Summarize**: LLM generates structured summary of older messages
3. **Inject**: Summary replaces compressed messages in subsequent requests

### Structured Summary Format

```
## Session Intent
[High-level goal of the conversation]

## File Modifications
- path/to/file1.go: Added function X, modified Y
- path/to/file2.ts: Refactored component Z

## Decisions Made
- Chose approach A over B because...
- Confirmed that X should use pattern Y

## Next Steps
1. Implement feature X
2. Add tests for Y

## Technical Details
- Key code snippets or patterns
- Important variable names or function signatures
```

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CLIPROXY_TOKEN_AWARE_ENABLED` | `true` | Enable token-based compression |
| `CLIPROXY_LLM_SUMMARY_ENABLED` | `true` | Use LLM for summaries (vs regex fallback) |
| `CLIPROXY_COMPRESSION_THRESHOLD` | `0.75` | Trigger at this % of context |

### Disable

```bash
export CLIPROXY_LLM_SUMMARY_ENABLED=false
```

---

## 2. Agentic Harness (Anthropic)

### What Anthropic Found

Anthropic's research on long-running agents identified key failure modes:

- Agents try to do everything at once and fail
- Agents don't track state between turns
- Agents skip verification ("the code looks correct")
- Agents lose context of what was already done

Their solution: **Structured harness with explicit state files and disciplined workflow.**

### How ProxyPilot Implements It

ProxyPilot injects prompts based on conversation state:

#### INITIALIZER Mode (First Request)

Injected when agentic client sends first request:

```
You are the Initializer Agent. Your ONLY goal is to set up the project environment.

1. Create 'feature_list.json' - List ALL features with:
   - id, category, description, steps, passes: false, priority

2. Create 'claude-progress.txt' - Session log with:
   - [Initializer] Environment setup started

3. Create 'init.sh' - Dev server startup script

DO NOT implement features. Only scaffold the harness files.
```

#### CODING Mode (After Harness Files Exist)

Injected when `feature_list.json` or `claude-progress.txt` detected:

```
You are a Coding Agent. Follow this workflow:

STEP 1: GET BEARINGS (MANDATORY)
- pwd, git log, cat claude-progress.txt, cat feature_list.json

STEP 2: SANITY CHECK
- Verify existing features work
- Fix regressions before new work

STEP 3: PICK ONE TASK
- Find highest-priority feature where passes: false
- Announce: "I will implement: [description]"

STEP 4: IMPLEMENT & VERIFY
- Write code for ONE feature only
- Run tests to verify it works
- Do NOT mark complete without actual testing

STEP 5: UPDATE STATE
- Set passes: true in feature_list.json
- Append to claude-progress.txt

STEP 6: COMMIT
- Git commit with feature ID in message
```

#### PASSIVE Mode

For non-agentic clients (browsers, curl), no prompts injected.

### Client Detection

Harness activates for these User-Agents:
- `claude-cli`
- `codex`
- `droid`

Or when header `X-ProxyPilot-Harness: true` is set.

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CLIPROXY_HARNESS_ENABLED` | `true` | Enable agentic harness |
| `CLIPROXY_MEMORY_DIR` | `~/.proxypilot/memory` | Session storage location |

### Disable

```bash
export CLIPROXY_HARNESS_ENABLED=false
```

---

## Implementation Details

### Files Modified

| File | Change |
|------|--------|
| `internal/memory/summarizer_executor.go` | Added `CoreManagerExecutor` interface and `ManagerAuthAdapter` |
| `internal/api/middleware/codex_prompt_budget.go` | Added `InitSummarizerWithAuthManager()` |
| `sdk/cliproxy/service.go` | Added `coreManagerWrapper`, wired up compression |
| `internal/api/server.go` | Registered `AgenticHarnessMiddleware()` |

### Files Created

| File | Purpose |
|------|---------|
| `internal/memory/summarizer_executor_test.go` | 12 unit tests for adapter |
| `internal/api/features_integration_test.go` | 3 integration tests |

### Architecture

```
Request Flow (Context Compression):

Client Request
      │
      ▼
┌─────────────────────┐
│ PromptBudget        │
│ Middleware          │
└─────────────────────┘
      │
      ▼
┌─────────────────────┐
│ Token Count Check   │──── < 75% ────▶ Pass through
└─────────────────────┘
      │
      │ > 75%
      ▼
┌─────────────────────┐
│ PipelineSummarizer  │
│ Executor            │
└─────────────────────┘
      │
      ▼
┌─────────────────────┐
│ ManagerAuthAdapter  │
└─────────────────────┘
      │
      ▼
┌─────────────────────┐
│ coreManagerWrapper  │
└─────────────────────┘
      │
      ▼
┌─────────────────────┐
│ Auth Manager        │
│ (claude/codex/      │
│  gemini)            │
└─────────────────────┘
```

```
Request Flow (Agentic Harness):

Client Request
      │
      ▼
┌─────────────────────┐
│ Check User-Agent    │──── Not agentic ────▶ Pass through
└─────────────────────┘
      │
      │ Agentic client
      ▼
┌─────────────────────┐
│ Detect Harness      │
│ State               │
└─────────────────────┘
      │
      ├── No harness files + short convo ──▶ INITIALIZER prompt
      │
      ├── Harness files exist ──────────────▶ CODING prompt
      │
      └── Long convo, no files ─────────────▶ PASSIVE (no injection)
```

---

## What Was NOT Implemented

During implementation, we removed features that were added by a previous session but **not from the original sources**:

| Removed | Reason |
|---------|--------|
| Puppeteer MCP detection | Not in Factory.ai or Anthropic sources |
| Browser automation reminders | Made up, not from research |

The current implementation is pure to the original Factory.ai and Anthropic research.

---

## Testing

```bash
# Run all tests
go test ./...

# Run specific feature tests
go test ./internal/memory/... -v                    # Compression adapter
go test ./internal/api/middleware/... -v -run Harness  # Harness middleware
go test ./internal/api/... -v -run Integration      # Integration tests
```

### Test Coverage

| Test File | Tests | Coverage |
|-----------|-------|----------|
| `summarizer_executor_test.go` | 12 | Adapter, pipeline, edge cases |
| `agentic_harness_test.go` | 10 | All modes, injection, session storage |
| `features_integration_test.go` | 3 | Combined feature validation |

---

## Quick Reference

### Enable/Disable Features

```bash
# Disable context compression
export CLIPROXY_LLM_SUMMARY_ENABLED=false

# Disable agentic harness
export CLIPROXY_HARNESS_ENABLED=false

# Disable both
export CLIPROXY_TOKEN_AWARE_ENABLED=false
export CLIPROXY_HARNESS_ENABLED=false
```

### Key Integration Points

```go
// Context Compression - sdk/cliproxy/service.go:516
middleware.InitSummarizerWithAuthManager(wrapper, providers)

// Agentic Harness - internal/api/server.go:224
engine.Use(middleware.AgenticHarnessMiddleware())
```

---

## Changelog

### 2025-12-27

- **Implemented**: Factory.ai-style context compression with LLM summarization
- **Implemented**: Anthropic-style agentic harness (INITIALIZER/CODING modes)
- **Added**: `ManagerAuthAdapter` to bridge auth manager types
- **Added**: `coreManagerWrapper` to adapt concrete types to interfaces
- **Added**: `InitSummarizerWithAuthManager()` for initialization
- **Added**: Session-based harness file storage
- **Added**: Comprehensive test coverage (25+ tests)
- **Removed**: Puppeteer MCP detection (not from original sources)
