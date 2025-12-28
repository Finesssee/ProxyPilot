package translator

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"
)

// CacheStats contains statistics about cache performance.
type CacheStats struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
	Size   int   `json:"size"`
}

// cacheEntry represents a cached translation result with expiration.
type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// TranslationCache provides LRU-based caching for translation results.
type TranslationCache struct {
	mu       sync.RWMutex
	enabled  atomic.Bool
	maxSize  int
	ttl      time.Duration
	cache    map[string]*cacheEntry
	order    []string // LRU order: oldest first
	hits     atomic.Int64
	misses   atomic.Int64
}

// NewTranslationCache creates a new translation cache with default settings.
func NewTranslationCache() *TranslationCache {
	tc := &TranslationCache{
		maxSize: 1000,
		ttl:     5 * time.Minute,
		cache:   make(map[string]*cacheEntry),
		order:   make([]string, 0),
	}
	tc.enabled.Store(true)
	return tc
}

// SetCacheEnabled enables or disables the cache.
func (tc *TranslationCache) SetCacheEnabled(enabled bool) {
	tc.enabled.Store(enabled)
}

// IsEnabled returns whether the cache is enabled.
func (tc *TranslationCache) IsEnabled() bool {
	return tc.enabled.Load()
}

// SetCacheMaxSize sets the maximum number of entries in the cache.
func (tc *TranslationCache) SetCacheMaxSize(size int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.maxSize = size
	tc.evictExcess()
}

// SetCacheTTL sets the time-to-live for cache entries.
func (tc *TranslationCache) SetCacheTTL(ttl time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.ttl = ttl
}

// GetCacheStats returns current cache statistics.
func (tc *TranslationCache) GetCacheStats() CacheStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return CacheStats{
		Hits:   tc.hits.Load(),
		Misses: tc.misses.Load(),
		Size:   len(tc.cache),
	}
}

// Clear removes all entries from the cache and resets statistics.
func (tc *TranslationCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cache = make(map[string]*cacheEntry)
	tc.order = make([]string, 0)
	tc.hits.Store(0)
	tc.misses.Store(0)
}

// generateKey creates a cache key from translation parameters.
func (tc *TranslationCache) generateKey(from, to Format, model string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(from.String()))
	h.Write([]byte(to.String()))
	h.Write([]byte(model))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// Get retrieves a cached translation result.
func (tc *TranslationCache) Get(from, to Format, model string, payload []byte) ([]byte, bool) {
	if !tc.enabled.Load() {
		return nil, false
	}

	key := tc.generateKey(from, to, model, payload)

	tc.mu.Lock()
	defer tc.mu.Unlock()

	entry, exists := tc.cache[key]
	if !exists {
		tc.misses.Add(1)
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		tc.removeKey(key)
		tc.misses.Add(1)
		return nil, false
	}

	// Move to end of LRU order (most recently used)
	tc.moveToEnd(key)
	tc.hits.Add(1)

	// Return a copy to prevent mutation
	result := make([]byte, len(entry.value))
	copy(result, entry.value)
	return result, true
}

// Set stores a translation result in the cache.
func (tc *TranslationCache) Set(from, to Format, model string, payload, result []byte) {
	if !tc.enabled.Load() {
		return
	}

	key := tc.generateKey(from, to, model, payload)

	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Check if key already exists
	if _, exists := tc.cache[key]; exists {
		tc.moveToEnd(key)
	} else {
		tc.order = append(tc.order, key)
	}

	// Store a copy to prevent mutation
	valueCopy := make([]byte, len(result))
	copy(valueCopy, result)

	tc.cache[key] = &cacheEntry{
		value:     valueCopy,
		expiresAt: time.Now().Add(tc.ttl),
	}

	tc.evictExcess()
}

// moveToEnd moves a key to the end of the LRU order.
func (tc *TranslationCache) moveToEnd(key string) {
	for i, k := range tc.order {
		if k == key {
			tc.order = append(tc.order[:i], tc.order[i+1:]...)
			tc.order = append(tc.order, key)
			break
		}
	}
}

// removeKey removes a key from cache and order.
func (tc *TranslationCache) removeKey(key string) {
	delete(tc.cache, key)
	for i, k := range tc.order {
		if k == key {
			tc.order = append(tc.order[:i], tc.order[i+1:]...)
			break
		}
	}
}

// evictExcess removes oldest entries if cache exceeds max size.
func (tc *TranslationCache) evictExcess() {
	for len(tc.cache) > tc.maxSize && len(tc.order) > 0 {
		oldest := tc.order[0]
		tc.order = tc.order[1:]
		delete(tc.cache, oldest)
	}
}

// evictExpired removes all expired entries.
func (tc *TranslationCache) evictExpired() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	now := time.Now()
	var expiredKeys []string
	for key, entry := range tc.cache {
		if now.After(entry.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	for _, key := range expiredKeys {
		tc.removeKey(key)
	}
}

// defaultCache is the package-level cache instance.
var defaultCache = NewTranslationCache()

// DefaultCache returns the package-level translation cache.
func DefaultCache() *TranslationCache {
	return defaultCache
}

// SetCacheEnabled enables or disables the default cache.
func SetCacheEnabled(enabled bool) {
	defaultCache.SetCacheEnabled(enabled)
}

// SetCacheMaxSize sets the maximum size of the default cache.
func SetCacheMaxSize(size int) {
	defaultCache.SetCacheMaxSize(size)
}

// SetCacheTTL sets the TTL for the default cache.
func SetCacheTTL(ttl time.Duration) {
	defaultCache.SetCacheTTL(ttl)
}

// GetCacheStats returns statistics for the default cache.
func GetCacheStats() CacheStats {
	return defaultCache.GetCacheStats()
}

// ClearCache clears the default cache.
func ClearCache() {
	defaultCache.Clear()
}

// CachedRegistry wraps a Registry with caching support.
type CachedRegistry struct {
	*Registry
	cache *TranslationCache
}

// NewCachedRegistry creates a registry with integrated caching.
func NewCachedRegistry(cache *TranslationCache) *CachedRegistry {
	if cache == nil {
		cache = defaultCache
	}
	return &CachedRegistry{
		Registry: NewRegistry(),
		cache:    cache,
	}
}

// TranslateRequest translates with caching support.
func (cr *CachedRegistry) TranslateRequest(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	// Try cache first
	if result, found := cr.cache.Get(from, to, model, rawJSON); found {
		return result
	}

	// Perform translation
	result := cr.Registry.TranslateRequest(from, to, model, rawJSON, stream)

	// Cache the result
	cr.cache.Set(from, to, model, rawJSON, result)

	return result
}

// GetCache returns the underlying cache.
func (cr *CachedRegistry) GetCache() *TranslationCache {
	return cr.cache
}
