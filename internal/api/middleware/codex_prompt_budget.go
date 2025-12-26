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
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/embeddings"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
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

	specModePrompt = `SPEC MODE (do not code yet).
1) Produce a complete, reviewable specification (requirements, acceptance criteria, architecture, and file-level plan).
2) Wait for explicit approval before editing code.
3) If clarification is needed, ask now before writing code.`
)

var (
	codexMaxBodyBytesOnce sync.Once
	codexMaxBodyBytes     int

	memOnce  sync.Once
	memStore memory.Store

	embedOnce   sync.Once
	embedClient *embeddings.OllamaClient

	embedQueueOnce sync.Once
	embedQueue     *semanticEmbedQueue

	pruneMu   sync.Mutex
	lastPrune time.Time

	limiterMu        sync.Mutex
	memoryLimiters   = map[string]*rateLimiter{}
	semanticLimiters = map[string]*rateLimiter{}
)

type rateLimiter struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func (r *rateLimiter) Allow(ratePerSec float64, burst float64) bool {
	if ratePerSec <= 0 {
		return true
	}
	if burst <= 0 {
		burst = ratePerSec
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if r.last.IsZero() {
		r.last = now
		r.tokens = burst
	}
	elapsed := now.Sub(r.last).Seconds()
	if elapsed > 0 {
		r.tokens += elapsed * ratePerSec
		if r.tokens > burst {
			r.tokens = burst
		}
		r.last = now
	}
	if r.tokens >= 1 {
		r.tokens -= 1
		return true
	}
	return false
}

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

		// Initialize summarizer with default config (uses regex fallback until executor is set)
		if agenticLLMSummaryEnabled() {
			if fs, ok := memStore.(*memory.FileStore); ok {
				config := memory.DefaultSummarizerConfig()
				// Start with NoOp executor - will fall back to regex
				// Call SetSummarizerExecutor() later with a real executor to enable LLM summarization
				summarizer := memory.NewSummarizer(config, memory.NewNoOpSummarizerExecutor())
				fs.SetSummarizer(summarizer)
			}
		}
	})
	return memStore
}

// SetSummarizerExecutor configures the LLM executor for context summarization.
// Call this from server initialization with the auth manager to enable full LLM summarization.
// Example: middleware.SetSummarizerExecutor(memory.NewPipelineSummarizerExecutor(authManager, providers))
func SetSummarizerExecutor(executor memory.SummarizerExecutor) {
	store := agenticMemoryStore()
	if store == nil {
		return
	}
	fs, ok := store.(*memory.FileStore)
	if !ok || fs == nil {
		return
	}
	summarizer := fs.GetSummarizer()
	if summarizer == nil {
		config := memory.DefaultSummarizerConfig()
		summarizer = memory.NewSummarizer(config, executor)
		fs.SetSummarizer(summarizer)
	} else {
		// Replace the existing summarizer with one that has the real executor
		config := memory.DefaultSummarizerConfig()
		fs.SetSummarizer(memory.NewSummarizer(config, executor))
	}
}

// InitSummarizerWithAuthManager configures LLM-based summarization using the core auth manager.
// Call this after the auth manager is initialized to enable Factory.ai-style context compression.
func InitSummarizerWithAuthManager(manager memory.CoreManagerExecutor, providers []string) {
	if manager == nil {
		return
	}
	adapter := memory.NewManagerAuthAdapter(manager)
	executor := memory.NewPipelineSummarizerExecutor(adapter, providers)
	SetSummarizerExecutor(executor)
}

func agenticSemanticEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_ENABLED")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
	}
	return true
}

func agenticSemanticModel() string {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MODEL")); v != "" {
		return v
	}
	return "embeddinggemma"
}

func agenticSemanticBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_BASE_URL")); v != "" {
		return v
	}
	return "http://127.0.0.1:11434"
}

func agenticSemanticMaxSnips() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MAX_SNIPS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 4
}

func agenticSemanticMaxChars() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MAX_CHARS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 3000
}

func agenticSemanticClient() *embeddings.OllamaClient {
	embedOnce.Do(func() {
		embedClient = &embeddings.OllamaClient{
			BaseURL: agenticSemanticBaseURL(),
			Model:   agenticSemanticModel(),
			Client:  &http.Client{Timeout: 8 * time.Second},
		}
	})
	return embedClient
}

type semanticEmbedTask struct {
	namespace string
	session   string
	texts     []string
	roles     []string
	source    string
}

type semanticEmbedQueue struct {
	ch chan semanticEmbedTask
	fs *memory.FileStore
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

func agenticMemoryMaxAgeDays() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_MAX_AGE_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func agenticMemoryMaxSessions() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_MAX_SESSIONS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func agenticMemoryMaxBytesPerSession() int64 {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_MAX_BYTES_PER_SESSION")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

func agenticSemanticMaxNamespaces() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MAX_NAMESPACES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func agenticSemanticMaxBytesPerNamespace() int64 {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MAX_BYTES_PER_NAMESPACE")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

func agenticMemoryMaxWritesPerMin() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_MAX_WRITES_PER_MIN")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 120
}

func agenticSemanticMaxWritesPerMin() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MAX_WRITES_PER_MIN")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 120
}

func agenticSemanticQueryMaxChars() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_QUERY_MAX_CHARS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 512
}

func agenticAnchorAppendOnly() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_ANCHOR_APPEND_ONLY")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
		if strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "on") || strings.EqualFold(v, "yes") {
			return true
		}
	}
	return true
}

func agenticAnchorSummaryMaxChars() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_ANCHOR_SUMMARY_MAX_CHARS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 14_000
}

// agenticCompressionThreshold returns the context usage ratio at which to trigger LLM compression.
// Default: 0.75 (75% of context window)
func agenticCompressionThreshold() float64 {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_COMPRESSION_THRESHOLD")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f < 1 {
			return f
		}
	}
	return 0.75
}

// agenticLLMSummaryEnabled returns whether LLM-based summarization is enabled.
// Default: true
func agenticLLMSummaryEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_LLM_SUMMARY_ENABLED")); v != "" {
		return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
	}
	return true
}

// getModelContextWindow returns the context window size for a model.
func getModelContextWindow(model string) int {
	// Try registry first
	info := registry.GetGlobalRegistry().GetModelInfo(model)
	if info != nil && info.ContextLength > 0 {
		return info.ContextLength
	}
	// Fallback based on model name patterns
	lowerModel := strings.ToLower(model)
	switch {
	case strings.Contains(lowerModel, "claude-3.5"), strings.Contains(lowerModel, "claude-3-5"):
		return 200000
	case strings.Contains(lowerModel, "claude-3"):
		return 200000
	case strings.Contains(lowerModel, "claude"):
		return 100000
	case strings.Contains(lowerModel, "gpt-4-turbo"), strings.Contains(lowerModel, "gpt-4o"):
		return 128000
	case strings.Contains(lowerModel, "gpt-4"):
		return 8192
	case strings.Contains(lowerModel, "gpt-3.5"):
		return 16384
	case strings.Contains(lowerModel, "gemini"):
		return 1000000
	case strings.Contains(lowerModel, "o1"), strings.Contains(lowerModel, "o3"):
		return 200000
	default:
		return 100000
	}
}

// agenticTokenAwareEnabled returns whether token-aware compression is enabled.
// Default: true
func agenticTokenAwareEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_TOKEN_AWARE_ENABLED")); v != "" {
		return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
	}
	return true
}

// agenticReserveTokens returns the number of tokens to reserve for model output.
// Default: 8192 (reasonable output buffer)
func agenticReserveTokens() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_RESERVE_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8192
}

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
// Returns whether trimming is needed and the target byte limit.
func analyzeTokenBudget(body []byte) *tokenAwareCompressionResult {
	result := &tokenAwareCompressionResult{
		ShouldTrim:     false,
		TargetMaxBytes: agenticMaxBodyBytes(), // Default fallback
	}

	if !agenticTokenAwareEnabled() {
		return result
	}

	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return result
	}
	result.Model = model

	// Get context window for this model
	contextWindow := getModelContextWindow(model)
	result.ContextWindow = contextWindow

	// Estimate current token count
	currentTokens, err := executor.EstimateRequestTokens(model, body)
	if err != nil {
		// Fallback to byte-based heuristic: ~4 chars per token
		currentTokens = int64(len(body) / 4)
	}
	result.CurrentTokens = currentTokens

	// Reserve tokens for output
	reserveTokens := agenticReserveTokens()
	availableContext := contextWindow - reserveTokens
	if availableContext < 1000 {
		availableContext = contextWindow / 2 // At least half for input
	}

	// Check against threshold
	threshold := agenticCompressionThreshold()
	maxInputTokens := int64(float64(availableContext) * threshold)

	if currentTokens <= maxInputTokens {
		// Under threshold, no trimming needed
		result.ShouldTrim = false
		return result
	}

	// Calculate target tokens (aim for 70% of threshold to give buffer)
	targetRatio := threshold * 0.9 // Slightly under threshold
	targetTokens := int64(float64(availableContext) * targetRatio)
	result.TargetTokens = targetTokens
	result.ShouldTrim = true

	// Convert target tokens to approximate byte limit
	// Use ~4 bytes per token as heuristic, but be conservative
	targetBytes := int(targetTokens * 4)

	// Clamp to reasonable bounds
	minBytes := 32 * 1024 // 32KB minimum
	if targetBytes < minBytes {
		targetBytes = minBytes
	}

	// Don't exceed the configured max
	maxBytes := agenticMaxBodyBytes()
	if targetBytes > maxBytes {
		targetBytes = maxBytes
	}

	result.TargetMaxBytes = targetBytes
	return result
}

// getTokenAwareMaxBytes returns the maximum body size based on token analysis.
// This is more accurate than the byte-based heuristic in agenticMaxBodyBytesForModel.
func getTokenAwareMaxBytes(body []byte) int {
	if !agenticTokenAwareEnabled() {
		return agenticMaxBodyBytesForModel(body)
	}

	result := analyzeTokenBudget(body)
	if result.ShouldTrim {
		return result.TargetMaxBytes
	}

	// Not over threshold, but still apply model-specific limits
	return agenticMaxBodyBytesForModel(body)
}

func allowMemoryWrite(session string) bool {
	limit := agenticMemoryMaxWritesPerMin()
	if limit <= 0 || session == "" {
		return true
	}
	limiter := getSessionLimiter(memoryLimiters, session)
	rate := float64(limit) / 60.0
	burst := float64(limit) / 6.0
	if burst < 5 {
		burst = 5
	}
	return limiter.Allow(rate, burst)
}

func allowSemanticWrite(session string) bool {
	limit := agenticSemanticMaxWritesPerMin()
	if limit <= 0 || session == "" {
		return true
	}
	limiter := getSessionLimiter(semanticLimiters, session)
	rate := float64(limit) / 60.0
	burst := float64(limit) / 6.0
	if burst < 5 {
		burst = 5
	}
	return limiter.Allow(rate, burst)
}

func getSessionLimiter(store map[string]*rateLimiter, session string) *rateLimiter {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	if limiter, ok := store[session]; ok {
		return limiter
	}
	limiter := &rateLimiter{}
	store[session] = limiter
	// best-effort cleanup for stale entries
	for key, lim := range store {
		if lim == nil {
			delete(store, key)
			continue
		}
		lim.mu.Lock()
		last := lim.last
		lim.mu.Unlock()
		if !last.IsZero() && time.Since(last) > 15*time.Minute {
			delete(store, key)
		}
	}
	return limiter
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

func agenticScaffoldEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SCAFFOLD_ENABLED")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
	}
	return true
}

func agenticScaffoldAppendOnly() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SCAFFOLD_APPEND_ONLY")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
	}
	return true
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

type trimWithMemoryResult struct {
	Body    []byte
	Query   string
	Dropped []memory.Event
	Shape   string // "chat", "responses", or "claude"
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

// trimClaudeMessagesWithMemory trims a Claude Messages API payload by shortening the messages array.
// Claude uses a similar structure to OpenAI chat but with content blocks that can include
// tool_use, tool_result, text, and other types.
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
	keep := 20
	perTextLimit := 20_000
	dropTools := false
	for keep >= 1 {
		outBody := body
		if dropTools && !mustKeepTools {
			outBody, _ = sjson.DeleteBytes(outBody, "tools")
			outBody, _ = sjson.SetBytes(outBody, "tool_choice", map[string]any{"type": "none"})
		}

		newMsgs := make([]string, 0, keep+1)
		keptIdx := make(map[int]struct{}, keep+2)

		// Claude Messages API helper functions
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

			// If this is a tool result message, ensure we keep the preceding assistant message with tool_use
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
			// Claude text blocks use type:"text" with text field
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
			// Only truncate text blocks
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
