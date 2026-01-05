package cache

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
	log "github.com/sirupsen/logrus"
)

// DefaultEvictionInterval is the default interval for periodic cache eviction.
const DefaultEvictionInterval = 1 * time.Minute

// ResponseCacheConfig defines configuration for the response cache.
type ResponseCacheConfig struct {
	// Enabled controls whether response caching is active.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// MaxSize is the maximum number of cached responses.
	MaxSize int `yaml:"max-size" json:"max-size"`
	// TTL is how long responses are cached (e.g., "5m", "1h").
	TTL time.Duration `yaml:"ttl" json:"ttl"`
	// ExcludeModels is a list of model patterns to exclude from caching.
	ExcludeModels []string `yaml:"exclude-models" json:"exclude-models"`
	// PersistFile is the optional file path to persist cache across restarts.
	PersistFile string `yaml:"persist-file" json:"persist-file"`
}

// DefaultResponseCacheConfig returns sensible defaults.
func DefaultResponseCacheConfig() ResponseCacheConfig {
	return ResponseCacheConfig{
		Enabled: false, // Opt-in by default
		MaxSize: 1000,
		TTL:     5 * time.Minute,
	}
}

// CachedResponse stores a cached API response with metadata.
type CachedResponse struct {
	// Response is the cached response body.
	Response []byte
	// ContentType is the response content type.
	ContentType string
	// StatusCode is the HTTP status code.
	StatusCode int
	// Model is the model that generated this response.
	Model string
	// CreatedAt is when the response was cached.
	CreatedAt time.Time
	// HitCount tracks how many times this cache entry was hit.
	HitCount int
}

// ResponseCache provides LRU-based caching for API responses.
// This caches complete responses at the proxy layer to reduce upstream API calls.
type ResponseCache struct {
	mu       sync.RWMutex
	entries  map[string]*CachedResponse
	order    []string // LRU order tracking
	config   ResponseCacheConfig
	stats    ResponseCacheStats
	disabled bool // runtime toggle
}

// ResponseCacheStats tracks cache performance metrics.
type ResponseCacheStats struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	Size       int
	TotalSaved int64 // Approximate tokens saved
}

// NewResponseCache creates a new response cache with the given config.
func NewResponseCache(cfg ResponseCacheConfig) *ResponseCache {
	return &ResponseCache{
		entries:  make(map[string]*CachedResponse),
		order:    make([]string, 0, cfg.MaxSize),
		config:   cfg,
		disabled: !cfg.Enabled,
	}
}

// defaultResponseCache is the package-level cache instance.
var (
	defaultResponseCache     *ResponseCache
	defaultResponseCacheOnce sync.Once
)

// GetDefaultResponseCache returns the package-level response cache.
func GetDefaultResponseCache() *ResponseCache {
	defaultResponseCacheOnce.Do(func() {
		defaultResponseCache = NewResponseCache(DefaultResponseCacheConfig())
	})
	return defaultResponseCache
}

// InitDefaultResponseCache initializes the default cache with custom config.
func InitDefaultResponseCache(cfg ResponseCacheConfig) {
	defaultResponseCacheOnce.Do(func() {
		defaultResponseCache = NewResponseCache(cfg)
	})
	// If already initialized, update config
	if defaultResponseCache != nil {
		defaultResponseCache.UpdateConfig(cfg)
	}
}

// generateKey creates a cache key from request parameters.
// Key is based on: model + messages hash + relevant parameters.
func (rc *ResponseCache) generateKey(model string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// Get retrieves a cached response.
// Returns nil if not found, expired, or cache is disabled.
func (rc *ResponseCache) Get(model string, payload []byte) *CachedResponse {
	if rc.disabled || !rc.config.Enabled {
		return nil
	}

	key := rc.generateKey(model, payload)

	rc.mu.RLock()
	entry, exists := rc.entries[key]
	rc.mu.RUnlock()

	if !exists {
		rc.mu.Lock()
		rc.stats.Misses++
		rc.mu.Unlock()
		middleware.RecordResponseCacheMiss()
		return nil
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > rc.config.TTL {
		rc.mu.Lock()
		delete(rc.entries, key)
		rc.removeFromOrder(key)
		rc.stats.Evictions++
		rc.stats.Misses++
		middleware.SetResponseCacheSize(len(rc.entries))
		rc.mu.Unlock()
		middleware.RecordResponseCacheMiss()
		return nil
	}

	rc.mu.Lock()
	entry.HitCount++
	rc.stats.Hits++
	rc.moveToEnd(key)
	rc.mu.Unlock()
	middleware.RecordResponseCacheHit()

	log.Debugf("response cache HIT for model %s (hits: %d)", model, entry.HitCount)
	return entry
}

// Set stores a response in the cache.
func (rc *ResponseCache) Set(model string, payload []byte, response []byte, contentType string, statusCode int) {
	if rc.disabled || !rc.config.Enabled {
		return
	}

	// Only cache successful responses
	if statusCode < 200 || statusCode >= 300 {
		return
	}

	// Don't cache empty responses
	if len(response) == 0 {
		return
	}

	// Check if model is excluded
	if rc.isModelExcluded(model) {
		return
	}

	key := rc.generateKey(model, payload)

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Evict if at capacity
	for len(rc.entries) >= rc.config.MaxSize && len(rc.order) > 0 {
		oldest := rc.order[0]
		delete(rc.entries, oldest)
		rc.order = rc.order[1:]
		rc.stats.Evictions++
	}

	// Store new entry
	rc.entries[key] = &CachedResponse{
		Response:    response,
		ContentType: contentType,
		StatusCode:  statusCode,
		Model:       model,
		CreatedAt:   time.Now(),
		HitCount:    0,
	}
	rc.order = append(rc.order, key)
	rc.stats.Size = len(rc.entries)
	middleware.SetResponseCacheSize(len(rc.entries))

	log.Debugf("response cache SET for model %s (size: %d/%d)", model, len(rc.entries), rc.config.MaxSize)
}

// isModelExcluded checks if a model should be excluded from caching.
func (rc *ResponseCache) isModelExcluded(model string) bool {
	for _, pattern := range rc.config.ExcludeModels {
		if matchPattern(pattern, model) {
			return true
		}
	}
	return false
}

// matchPattern performs simple wildcard matching.
func matchPattern(pattern, model string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == model {
		return true
	}
	// Simple prefix/suffix matching
	if len(pattern) > 1 && pattern[0] == '*' {
		return len(model) >= len(pattern)-1 && model[len(model)-(len(pattern)-1):] == pattern[1:]
	}
	if len(pattern) > 1 && pattern[len(pattern)-1] == '*' {
		return len(model) >= len(pattern)-1 && model[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return false
}

// moveToEnd moves a key to the end of the LRU order.
func (rc *ResponseCache) moveToEnd(key string) {
	for i, k := range rc.order {
		if k == key {
			rc.order = append(rc.order[:i], rc.order[i+1:]...)
			rc.order = append(rc.order, key)
			return
		}
	}
}

// removeFromOrder removes a key from the LRU order.
func (rc *ResponseCache) removeFromOrder(key string) {
	for i, k := range rc.order {
		if k == key {
			rc.order = append(rc.order[:i], rc.order[i+1:]...)
			return
		}
	}
}

// Clear removes all entries from the cache.
func (rc *ResponseCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries = make(map[string]*CachedResponse)
	rc.order = make([]string, 0, rc.config.MaxSize)
	rc.stats.Size = 0
	middleware.SetResponseCacheSize(0)
}

// GetStats returns current cache statistics.
func (rc *ResponseCache) GetStats() ResponseCacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	rc.stats.Size = len(rc.entries)
	return rc.stats
}

// SetEnabled enables or disables the cache at runtime.
func (rc *ResponseCache) SetEnabled(enabled bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.disabled = !enabled
	rc.config.Enabled = enabled
	if !enabled {
		log.Info("response cache disabled")
	} else {
		log.Info("response cache enabled")
	}
}

// IsEnabled returns whether the cache is enabled.
func (rc *ResponseCache) IsEnabled() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.config.Enabled && !rc.disabled
}

// UpdateConfig updates the cache configuration.
func (rc *ResponseCache) UpdateConfig(cfg ResponseCacheConfig) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.config = cfg
	rc.disabled = !cfg.Enabled
	// Evict if new max size is smaller
	for len(rc.entries) > cfg.MaxSize && len(rc.order) > 0 {
		oldest := rc.order[0]
		delete(rc.entries, oldest)
		rc.order = rc.order[1:]
		rc.stats.Evictions++
	}
	rc.stats.Size = len(rc.entries)
}

// EvictExpired removes all expired entries from the cache.
func (rc *ResponseCache) EvictExpired() int {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	evicted := 0
	now := time.Now()
	newOrder := make([]string, 0, len(rc.order))

	for _, key := range rc.order {
		entry, exists := rc.entries[key]
		if !exists {
			continue
		}
		if now.Sub(entry.CreatedAt) > rc.config.TTL {
			delete(rc.entries, key)
			evicted++
			rc.stats.Evictions++
		} else {
			newOrder = append(newOrder, key)
		}
	}

	rc.order = newOrder
	rc.stats.Size = len(rc.entries)
	middleware.SetResponseCacheSize(len(rc.entries))
	return evicted
}

// StartPeriodicEviction starts a background goroutine that periodically evicts expired entries.
// The goroutine stops when the context is cancelled.
func (rc *ResponseCache) StartPeriodicEviction(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultEvictionInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if evicted := rc.EvictExpired(); evicted > 0 {
					log.Debugf("response cache: evicted %d expired entries", evicted)
				}
			}
		}
	}()
}

// TryGetCached checks the default response cache for a cached response.
// Returns (response, contentType, found). If not found, returns (nil, "", false).
func TryGetCached(model string, payload []byte) ([]byte, string, bool) {
	cache := GetDefaultResponseCache()
	if cache == nil || !cache.IsEnabled() {
		return nil, "", false
	}
	entry := cache.Get(model, payload)
	if entry == nil {
		return nil, "", false
	}
	return entry.Response, entry.ContentType, true
}

// StoreCached stores a response in the default response cache.
func StoreCached(model string, payload []byte, response []byte, contentType string, statusCode int) {
	cache := GetDefaultResponseCache()
	if cache == nil || !cache.IsEnabled() {
		return
	}
	cache.Set(model, payload, response, contentType, statusCode)
}

// responseCachePersist is the serializable form of the cache for gob encoding.
type responseCachePersist struct {
	Entries map[string]*CachedResponse
	Order   []string
	Stats   ResponseCacheStats
}

// SaveToFile persists the cache to a file using gob encoding.
// If path is empty, this is a no-op. Creates parent directories if needed.
func (rc *ResponseCache) SaveToFile(path string) error {
	if path == "" {
		return nil
	}

	rc.mu.RLock()
	data := responseCachePersist{
		Entries: make(map[string]*CachedResponse, len(rc.entries)),
		Order:   append([]string(nil), rc.order...),
		Stats:   rc.stats,
	}
	for k, v := range rc.entries {
		data.Entries[k] = v
	}
	rc.mu.RUnlock()

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

	log.Debugf("response cache saved to %s (%d entries)", path, len(data.Entries))
	return nil
}

// LoadFromFile loads the cache from a file using gob decoding.
// If path is empty or the file doesn't exist, this is a no-op.
// Expired entries are skipped during load.
func (rc *ResponseCache) LoadFromFile(path string) error {
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

	var data responseCachePersist
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return err
	}

	now := time.Now()
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Load entries, skipping expired ones
	loaded := 0
	for _, key := range data.Order {
		entry, exists := data.Entries[key]
		if !exists || entry == nil {
			continue
		}
		// Skip expired entries
		if now.Sub(entry.CreatedAt) > rc.config.TTL {
			continue
		}
		// Skip if over capacity
		if len(rc.entries) >= rc.config.MaxSize {
			break
		}
		rc.entries[key] = entry
		rc.order = append(rc.order, key)
		loaded++
	}
	rc.stats.Size = len(rc.entries)

	log.Infof("response cache loaded from %s (%d entries)", path, loaded)
	return nil
}

// StartPeriodicPersistence starts a background goroutine that periodically saves the cache.
// The goroutine stops when the context is cancelled.
func (rc *ResponseCache) StartPeriodicPersistence(ctx context.Context, path string, interval time.Duration) {
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
				if err := rc.SaveToFile(path); err != nil {
					log.Warnf("failed to save response cache on shutdown: %v", err)
				}
				return
			case <-ticker.C:
				if err := rc.SaveToFile(path); err != nil {
					log.Warnf("failed to save response cache: %v", err)
				}
			}
		}
	}()
}
