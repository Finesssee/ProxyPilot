package middleware

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/embeddings"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
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
		if v := strings.TrimSpace(os.Getenv("CLIPROXY_AGENTIC_MAX_BODY_BYTES")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
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

		if agenticLLMSummaryEnabled() {
			if fs, ok := memStore.(*memory.FileStore); ok {
				config := memory.DefaultSummarizerConfig()
				summarizer := memory.NewSummarizer(config, memory.NewNoOpSummarizerExecutor())
				fs.SetSummarizer(summarizer)
			}
		}
	})
	return memStore
}

// SetSummarizerExecutor configures the LLM executor for context summarization.
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
		config := memory.DefaultSummarizerConfig()
		fs.SetSummarizer(memory.NewSummarizer(config, executor))
	}
}

// InitSummarizerWithAuthManager configures LLM-based summarization using the core auth manager.
func InitSummarizerWithAuthManager(manager memory.CoreManagerExecutor, providers []string) {
	if manager == nil {
		return
	}
	adapter := memory.NewManagerAuthAdapter(manager)
	baseExecutor := memory.NewPipelineSummarizerExecutor(adapter, providers)
	executor := memory.NewSummaryModelFallbackExecutor(baseExecutor)
	SetSummarizerExecutor(executor)
}

// GetSummaryModel returns the configured summary model, defaulting to gemini-3-flash.
func GetSummaryModel() string {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SUMMARY_MODEL")); v != "" {
		return v
	}
	return memory.DefaultSummaryModel
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

func agenticCompressionThreshold() float64 {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_COMPRESSION_THRESHOLD")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f < 1 {
			return f
		}
	}
	return 0.85
}

func agenticMinKeepMessages() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MIN_KEEP_MESSAGES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 25
}

func agenticLLMSummaryEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_LLM_SUMMARY_ENABLED")); v != "" {
		return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
	}
	return true
}

func getModelContextWindow(model string) int {
	info := registry.GetGlobalRegistry().GetModelInfo(model, "")
	if info != nil && info.ContextLength > 0 {
		return info.ContextLength
	}
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

func agenticTokenAwareEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_TOKEN_AWARE_ENABLED")); v != "" {
		return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
	}
	return true
}

func agenticReserveTokens() int {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_RESERVE_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8192
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
