package common

import (
	"sync"
	"time"
)

// SignatureCache provides a thread-safe cache for storing and retrieving
// thought signatures associated with tool use IDs. This is necessary because
// some clients (like Claude Code) may not persist the signature in their history,
// but the Gemini API requires it for valid tool execution with Thinking models.
type SignatureCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	signature string
	expiresAt time.Time
}

// GlobalSignatureCache is the singleton instance of the signature cache.
var GlobalSignatureCache = &SignatureCache{
	items: make(map[string]cacheItem),
}

// Add stores a signature for a given ID in the cache.
// The signature is stored with an expiration time (default 1 hour).
func (c *SignatureCache) Add(id, signature string) {
	if id == "" || signature == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Lazy cleanup: occasionally clean up expired items
	// This is a simple probabilistic approach to avoid a dedicated goroutine
	if len(c.items)%100 == 0 {
		c.cleanup()
	}

	c.items[id] = cacheItem{
		signature: signature,
		expiresAt: time.Now().Add(1 * time.Hour),
	}
}

// Get retrieves a signature for a given ID from the cache.
// Returns an empty string if the ID is not found or has expired.
func (c *SignatureCache) Get(id string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[id]
	if !ok {
		return ""
	}

	if time.Now().After(item.expiresAt) {
		return ""
	}

	return item.signature
}

// cleanup removes expired items from the cache.
// Must be called with the lock held.
func (c *SignatureCache) cleanup() {
	now := time.Now()
	for id, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, id)
		}
	}
}
