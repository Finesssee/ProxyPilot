package cache

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
	log "github.com/sirupsen/logrus"
)

// PromptCacheConfig defines configuration for the prompt cache.
type PromptCacheConfig struct {
	// Enabled controls whether prompt caching is active.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// MaxSize is the maximum number of cached prompts.
	MaxSize int `yaml:"max-size" json:"max-size"`
	// MaxBytes is the maximum total size (bytes) of cached prompts.
	// 0 disables size-based eviction.
	MaxBytes int64 `yaml:"max-bytes" json:"max-bytes"`
	// TTL is how long prompts are cached.
	TTL time.Duration `yaml:"ttl" json:"ttl"`
	// PersistFile is the optional file path to persist cache across restarts.
	PersistFile string `yaml:"persist-file" json:"persist-file"`
}

// DefaultPromptCacheConfig returns sensible defaults.
func DefaultPromptCacheConfig() PromptCacheConfig {
	return PromptCacheConfig{
		Enabled: false, // Opt-in by default
		MaxSize: 500,
		TTL:     30 * time.Minute,
	}
}

// CachedPrompt stores a cached system prompt with metadata.
type CachedPrompt struct {
	// Hash is the SHA256 hash of the prompt content.
	Hash string
	// Prompt is the full system prompt text.
	Prompt string
	// TokenEstimate is an estimated token count for this prompt.
	TokenEstimate int
	// CreatedAt is when the prompt was cached.
	CreatedAt time.Time
	// HitCount tracks how many times this cache entry was hit.
	HitCount int64
	// LastHit is the last time this entry was accessed.
	LastHit time.Time
	// Provider tracks which provider(s) this prompt was used with.
	Providers map[string]int
	// SizeBytes tracks the approximate size of this entry.
	SizeBytes int64
}

// PromptCache provides LRU-based caching for system prompts.
// This tracks repeated system prompts to enable synthetic caching for
// providers that don't support native prompt caching.
type PromptCache struct {
	mu         sync.RWMutex
	entries    map[string]*CachedPrompt
	order      []string // LRU order tracking
	config     PromptCacheConfig
	stats      PromptCacheStats
	totalBytes int64
	disabled   bool // runtime toggle
}

// PromptCacheStats tracks cache performance metrics.
type PromptCacheStats struct {
	// Hits is the number of cache hits.
	Hits int64 `json:"hits"`
	// Misses is the number of cache misses.
	Misses int64 `json:"misses"`
	// Evictions is the number of evicted entries.
	Evictions int64 `json:"evictions"`
	// Size is the current number of cached entries.
	Size int `json:"size"`
	// UniquePrompts is the total number of unique prompts seen.
	UniquePrompts int64 `json:"unique_prompts"`
	// TotalRequests is the total number of cache operations.
	TotalRequests int64 `json:"total_requests"`
	// EstimatedTokensSaved estimates tokens saved from cache hits.
	EstimatedTokensSaved int64 `json:"estimated_tokens_saved"`
	// TopProviders tracks hit counts by provider.
	TopProviders map[string]int64 `json:"top_providers,omitempty"`
}

// NewPromptCache creates a new prompt cache with the given config.
func NewPromptCache(cfg PromptCacheConfig) *PromptCache {
	return &PromptCache{
		entries:  make(map[string]*CachedPrompt),
		order:    make([]string, 0, cfg.MaxSize),
		config:   cfg,
		disabled: !cfg.Enabled,
		stats: PromptCacheStats{
			TopProviders: make(map[string]int64),
		},
	}
}

// defaultPromptCache is the package-level cache instance.
var (
	defaultPromptCache     *PromptCache
	defaultPromptCacheOnce sync.Once
)

// GetDefaultPromptCache returns the package-level prompt cache.
func GetDefaultPromptCache() *PromptCache {
	defaultPromptCacheOnce.Do(func() {
		defaultPromptCache = NewPromptCache(DefaultPromptCacheConfig())
	})
	return defaultPromptCache
}

// InitDefaultPromptCache initializes the default cache with custom config.
func InitDefaultPromptCache(cfg PromptCacheConfig) {
	defaultPromptCacheOnce.Do(func() {
		defaultPromptCache = NewPromptCache(cfg)
	})
	// If already initialized, update config
	if defaultPromptCache != nil {
		defaultPromptCache.UpdateConfig(cfg)
	}
}

// HashPrompt generates a hash for a system prompt.
func HashPrompt(prompt string) string {
	h := sha256.New()
	h.Write([]byte(prompt))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// estimateTokens provides a rough token estimate based on character count.
// Uses ~4 characters per token as a rough estimate.
func estimateTokens(prompt string) int {
	return (len(prompt) + 3) / 4
}

// CacheSystemPrompt caches a system prompt and returns its hash.
// If the prompt already exists, it increments the hit count.
// Returns (hash, isNewPrompt).
func (pc *PromptCache) CacheSystemPrompt(prompt string, provider string) (string, bool) {
	if pc.disabled || !pc.config.Enabled || prompt == "" {
		return HashPrompt(prompt), false
	}

	hash := HashPrompt(prompt)

	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.stats.TotalRequests++

	entry, exists := pc.entries[hash]
	if exists {
		// Check TTL
		if time.Since(entry.CreatedAt) > pc.config.TTL {
			pc.removeEntryLocked(hash)
			pc.stats.Evictions++
			exists = false
		}
	}

	if exists {
		entry.HitCount++
		entry.LastHit = time.Now()
		if provider != "" {
			if entry.Providers == nil {
				entry.Providers = make(map[string]int)
			}
			entry.Providers[provider]++
			pc.stats.TopProviders[provider]++
		}
		pc.stats.Hits++
		pc.stats.EstimatedTokensSaved += int64(entry.TokenEstimate)
		pc.moveToEndLocked(hash)
		middleware.RecordPromptCacheHit()
		middleware.RecordPromptCacheTokensSaved(entry.TokenEstimate)

		log.Debugf("prompt cache HIT hash=%s provider=%s hits=%d", hash[:8], provider, entry.HitCount)
		return hash, false
	}

	pc.stats.Misses++
	pc.stats.UniquePrompts++
	middleware.RecordPromptCacheMiss()

	// Evict if at capacity
	entrySize := estimatePromptEntrySize(prompt, hash)
	if pc.config.MaxBytes > 0 && entrySize > pc.config.MaxBytes {
		return hash, false
	}
	for (pc.config.MaxSize > 0 && len(pc.entries) >= pc.config.MaxSize && len(pc.order) > 0) ||
		(pc.config.MaxBytes > 0 && pc.totalBytes+entrySize > pc.config.MaxBytes && len(pc.order) > 0) {
		pc.removeEntryLocked(pc.order[0])
		pc.stats.Evictions++
	}

	// Store new entry
	tokenEst := estimateTokens(prompt)
	providers := make(map[string]int)
	if provider != "" {
		providers[provider] = 1
		pc.stats.TopProviders[provider]++
	}

	pc.entries[hash] = &CachedPrompt{
		Hash:          hash,
		Prompt:        prompt,
		TokenEstimate: tokenEst,
		CreatedAt:     time.Now(),
		HitCount:      0,
		LastHit:       time.Now(),
		Providers:     providers,
		SizeBytes:     entrySize,
	}
	pc.order = append(pc.order, hash)
	pc.totalBytes += entrySize
	pc.stats.Size = len(pc.entries)
	middleware.SetPromptCacheSize(len(pc.entries))

	log.Debugf("prompt cache SET hash=%s provider=%s tokens~%d size=%d/%d",
		hash[:8], provider, tokenEst, len(pc.entries), pc.config.MaxSize)
	return hash, true
}

// GetCachedPrompt retrieves a cached prompt by its hash.
// Returns nil if not found or expired.
func (pc *PromptCache) GetCachedPrompt(hash string) *CachedPrompt {
	if pc.disabled || !pc.config.Enabled {
		return nil
	}

	pc.mu.RLock()
	entry, exists := pc.entries[hash]
	pc.mu.RUnlock()

	if !exists {
		return nil
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > pc.config.TTL {
		pc.mu.Lock()
		pc.removeEntryLocked(hash)
		pc.stats.Evictions++
		pc.mu.Unlock()
		return nil
	}

	return entry
}

// LookupByPrompt checks if a prompt is cached and returns hit info.
// Returns (hash, hitCount, found).
func (pc *PromptCache) LookupByPrompt(prompt string) (string, int64, bool) {
	if pc.disabled || !pc.config.Enabled || prompt == "" {
		return "", 0, false
	}

	hash := HashPrompt(prompt)

	pc.mu.RLock()
	entry, exists := pc.entries[hash]
	pc.mu.RUnlock()

	if !exists {
		return hash, 0, false
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > pc.config.TTL {
		return hash, 0, false
	}

	return hash, entry.HitCount, true
}

// moveToEndLocked moves a key to the end of the LRU order.
// Caller must hold the lock.
func (pc *PromptCache) moveToEndLocked(key string) {
	for i, k := range pc.order {
		if k == key {
			pc.order = append(pc.order[:i], pc.order[i+1:]...)
			pc.order = append(pc.order, key)
			return
		}
	}
}

// removeFromOrderLocked removes a key from the LRU order.
// Caller must hold the lock.
func (pc *PromptCache) removeFromOrderLocked(key string) {
	for i, k := range pc.order {
		if k == key {
			pc.order = append(pc.order[:i], pc.order[i+1:]...)
			return
		}
	}
}

func (pc *PromptCache) removeEntryLocked(key string) {
	entry := pc.entries[key]
	if entry != nil {
		pc.totalBytes -= entry.SizeBytes
	}
	delete(pc.entries, key)
	pc.removeFromOrderLocked(key)
}

// Clear removes all entries from the cache.
func (pc *PromptCache) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.entries = make(map[string]*CachedPrompt)
	pc.order = make([]string, 0, pc.config.MaxSize)
	pc.totalBytes = 0
	pc.stats.Size = 0
	middleware.SetPromptCacheSize(0)
}

// GetStats returns current cache statistics.
func (pc *PromptCache) GetStats() PromptCacheStats {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	stats := pc.stats
	stats.Size = len(pc.entries)
	// Copy provider map to avoid race
	if len(pc.stats.TopProviders) > 0 {
		stats.TopProviders = make(map[string]int64, len(pc.stats.TopProviders))
		for k, v := range pc.stats.TopProviders {
			stats.TopProviders[k] = v
		}
	}
	return stats
}

// SetEnabled enables or disables the cache at runtime.
func (pc *PromptCache) SetEnabled(enabled bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.disabled = !enabled
	pc.config.Enabled = enabled
	if !enabled {
		log.Info("prompt cache disabled")
	} else {
		log.Info("prompt cache enabled")
	}
}

// IsEnabled returns whether the cache is enabled.
func (pc *PromptCache) IsEnabled() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.config.Enabled && !pc.disabled
}

// UpdateConfig updates the cache configuration.
func (pc *PromptCache) UpdateConfig(cfg PromptCacheConfig) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.config = cfg
	pc.disabled = !cfg.Enabled
	// Evict if new max size is smaller
	for (cfg.MaxSize > 0 && len(pc.entries) > cfg.MaxSize && len(pc.order) > 0) ||
		(cfg.MaxBytes > 0 && pc.totalBytes > cfg.MaxBytes && len(pc.order) > 0) {
		pc.removeEntryLocked(pc.order[0])
		pc.stats.Evictions++
	}
	pc.stats.Size = len(pc.entries)
}

// EvictExpired removes all expired entries from the cache.
func (pc *PromptCache) EvictExpired() int {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	evicted := 0
	now := time.Now()
	newOrder := make([]string, 0, len(pc.order))

	for _, key := range pc.order {
		entry, exists := pc.entries[key]
		if !exists {
			continue
		}
		if now.Sub(entry.CreatedAt) > pc.config.TTL {
			pc.totalBytes -= entry.SizeBytes
			delete(pc.entries, key)
			evicted++
			pc.stats.Evictions++
		} else {
			newOrder = append(newOrder, key)
		}
	}

	pc.order = newOrder
	pc.stats.Size = len(pc.entries)
	middleware.SetPromptCacheSize(len(pc.entries))
	return evicted
}

// GetTopPrompts returns the most frequently hit prompts.
func (pc *PromptCache) GetTopPrompts(limit int) []*CachedPrompt {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Collect all entries
	entries := make([]*CachedPrompt, 0, len(pc.entries))
	for _, entry := range pc.entries {
		entries = append(entries, entry)
	}

	// Sort by hit count (simple selection for small lists)
	for i := 0; i < len(entries) && i < limit; i++ {
		maxIdx := i
		for j := i + 1; j < len(entries); j++ {
			if entries[j].HitCount > entries[maxIdx].HitCount {
				maxIdx = j
			}
		}
		entries[i], entries[maxIdx] = entries[maxIdx], entries[i]
	}

	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

// TrackSystemPrompt is a convenience function for the default cache.
func TrackSystemPrompt(prompt string, provider string) (string, bool) {
	cache := GetDefaultPromptCache()
	if cache == nil || !cache.IsEnabled() {
		return HashPrompt(prompt), false
	}
	return cache.CacheSystemPrompt(prompt, provider)
}

// IsPromptCached checks if a prompt is already cached in the default cache.
func IsPromptCached(prompt string) (string, int64, bool) {
	cache := GetDefaultPromptCache()
	if cache == nil || !cache.IsEnabled() {
		return HashPrompt(prompt), 0, false
	}
	return cache.LookupByPrompt(prompt)
}

// promptCachePersist is the serializable form of the cache for gob encoding.
type promptCachePersist struct {
	Entries map[string]*CachedPrompt
	Order   []string
	Stats   PromptCacheStats
}

// SaveToFile persists the cache to a file using gob encoding.
// If path is empty, this is a no-op. Creates parent directories if needed.
func (pc *PromptCache) SaveToFile(path string) error {
	if path == "" {
		return nil
	}

	pc.mu.RLock()
	data := promptCachePersist{
		Entries: make(map[string]*CachedPrompt, len(pc.entries)),
		Order:   append([]string(nil), pc.order...),
		Stats:   pc.stats,
	}
	// Deep copy stats.TopProviders to avoid race
	if len(pc.stats.TopProviders) > 0 {
		data.Stats.TopProviders = make(map[string]int64, len(pc.stats.TopProviders))
		for k, v := range pc.stats.TopProviders {
			data.Stats.TopProviders[k] = v
		}
	}
	for k, v := range pc.entries {
		data.Entries[k] = v
	}
	pc.mu.RUnlock()

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Write to temp file then rename for atomicity
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	enc := gob.NewEncoder(f)
	if err := enc.Encode(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	log.Debugf("prompt cache saved to %s (%d entries)", path, len(data.Entries))
	return nil
}

// LoadFromFile loads the cache from a file using gob decoding.
// If path is empty or the file doesn't exist, this is a no-op.
// Expired entries are skipped during load.
func (pc *PromptCache) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet, not an error
		}
		return err
	}
	defer func() { _ = f.Close() }()

	var data promptCachePersist
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return err
	}

	now := time.Now()
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Load entries, skipping expired ones
	loaded := 0
	for _, key := range data.Order {
		entry, exists := data.Entries[key]
		if !exists || entry == nil {
			continue
		}
		// Skip expired entries
		if now.Sub(entry.CreatedAt) > pc.config.TTL {
			continue
		}
		// Skip if over capacity
		entry.SizeBytes = ensurePromptEntrySize(entry)
		if (pc.config.MaxSize > 0 && len(pc.entries) >= pc.config.MaxSize) ||
			(pc.config.MaxBytes > 0 && pc.totalBytes+entry.SizeBytes > pc.config.MaxBytes) {
			break
		}
		pc.entries[key] = entry
		pc.order = append(pc.order, key)
		pc.totalBytes += entry.SizeBytes
		loaded++
	}
	pc.stats.Size = len(pc.entries)
	// Restore stats from persisted data
	pc.stats.Hits = data.Stats.Hits
	pc.stats.Misses = data.Stats.Misses
	pc.stats.Evictions = data.Stats.Evictions
	pc.stats.UniquePrompts = data.Stats.UniquePrompts
	pc.stats.TotalRequests = data.Stats.TotalRequests
	pc.stats.EstimatedTokensSaved = data.Stats.EstimatedTokensSaved
	if len(data.Stats.TopProviders) > 0 {
		pc.stats.TopProviders = make(map[string]int64, len(data.Stats.TopProviders))
		for k, v := range data.Stats.TopProviders {
			pc.stats.TopProviders[k] = v
		}
	}

	log.Infof("prompt cache loaded from %s (%d entries)", path, loaded)
	return nil
}

func estimatePromptEntrySize(prompt, hash string) int64 {
	return int64(len(prompt) + len(hash))
}

func ensurePromptEntrySize(entry *CachedPrompt) int64 {
	if entry == nil {
		return 0
	}
	if entry.SizeBytes > 0 {
		return entry.SizeBytes
	}
	entry.SizeBytes = estimatePromptEntrySize(entry.Prompt, entry.Hash)
	return entry.SizeBytes
}

// StartPeriodicPersistence starts a background goroutine that periodically saves the cache.
// The goroutine stops when the context is cancelled.
func (pc *PromptCache) StartPeriodicPersistence(ctx context.Context, path string, interval time.Duration) {
	if path == "" || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				// Final save on shutdown
				if err := pc.SaveToFile(path); err != nil {
					log.Warnf("failed to save prompt cache on shutdown: %v", err)
				}
				return
			case <-ticker.C:
				if err := pc.SaveToFile(path); err != nil {
					log.Warnf("failed to save prompt cache: %v", err)
				}
			}
		}
	}()
}

// WarmPromptEntry represents a prompt to pre-load into the cache.
type WarmPromptEntry struct {
	// Prompt is the system prompt text to cache.
	Prompt string `json:"prompt"`
	// Provider is the provider name (optional).
	Provider string `json:"provider"`
}

// WarmCacheResult tracks the result of a cache warming operation.
type WarmCacheResult struct {
	// Total is the number of prompts processed.
	Total int `json:"total"`
	// Added is the number of new prompts added to cache.
	Added int `json:"added"`
	// Skipped is the number of prompts already in cache.
	Skipped int `json:"skipped"`
	// Errors is the number of prompts that failed.
	Errors int `json:"errors"`
}

// WarmCache pre-populates the cache with the given prompts.
// This is useful for loading known expensive system prompts on startup.
func (pc *PromptCache) WarmCache(prompts []WarmPromptEntry) WarmCacheResult {
	result := WarmCacheResult{Total: len(prompts)}

	for _, entry := range prompts {
		prompt := strings.TrimSpace(entry.Prompt)
		if prompt == "" {
			result.Errors++
			continue
		}

		// CacheSystemPrompt returns (hash, isNew)
		_, isNew := pc.CacheSystemPrompt(prompt, entry.Provider)
		if isNew {
			result.Added++
		} else {
			result.Skipped++
		}
	}

	log.Infof("prompt cache warmed: added=%d skipped=%d errors=%d total=%d",
		result.Added, result.Skipped, result.Errors, result.Total)
	return result
}

// LoadWarmFile reads prompts from a JSON file and warms the cache.
// Expected format: [{"prompt": "...", "provider": "claude"}, ...]
// Returns the warming result or an error if the file cannot be read.
func (pc *PromptCache) LoadWarmFile(path string) (WarmCacheResult, error) {
	if path == "" {
		return WarmCacheResult{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debugf("warm file not found: %s", path)
			return WarmCacheResult{}, nil
		}
		return WarmCacheResult{}, err
	}

	var prompts []WarmPromptEntry
	if err := json.Unmarshal(data, &prompts); err != nil {
		return WarmCacheResult{}, err
	}

	result := pc.WarmCache(prompts)
	log.Infof("prompt cache warmed from file %s: added=%d skipped=%d",
		path, result.Added, result.Skipped)
	return result, nil
}

// WarmDefaultCache warms the default cache from a file.
func WarmDefaultCache(path string) (WarmCacheResult, error) {
	cache := GetDefaultPromptCache()
	if cache == nil {
		return WarmCacheResult{}, nil
	}
	return cache.LoadWarmFile(path)
}
