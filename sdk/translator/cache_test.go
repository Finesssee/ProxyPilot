package translator

import (
	"testing"
	"time"
)

func TestTranslationCache_BasicOperations(t *testing.T) {
	cache := NewTranslationCache()

	from := Format("openai")
	to := Format("anthropic")
	model := "gpt-4"
	payload := []byte(`{"test": "data"}`)
	result := []byte(`{"translated": "data"}`)

	// Test cache miss
	_, found := cache.Get(from, to, model, payload)
	if found {
		t.Error("expected cache miss on empty cache")
	}

	stats := cache.GetCacheStats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}

	// Test cache set and hit
	cache.Set(from, to, model, payload, result)

	retrieved, found := cache.Get(from, to, model, payload)
	if !found {
		t.Error("expected cache hit after set")
	}
	if string(retrieved) != string(result) {
		t.Errorf("expected %s, got %s", string(result), string(retrieved))
	}

	stats = cache.GetCacheStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Size != 1 {
		t.Errorf("expected size 1, got %d", stats.Size)
	}
}

func TestTranslationCache_TTLExpiration(t *testing.T) {
	cache := NewTranslationCache()
	cache.SetCacheTTL(50 * time.Millisecond)

	from := Format("openai")
	to := Format("anthropic")
	model := "gpt-4"
	payload := []byte(`{"test": "data"}`)
	result := []byte(`{"translated": "data"}`)

	cache.Set(from, to, model, payload, result)

	// Should be found immediately
	_, found := cache.Get(from, to, model, payload)
	if !found {
		t.Error("expected cache hit before TTL expiration")
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Should be expired now
	_, found = cache.Get(from, to, model, payload)
	if found {
		t.Error("expected cache miss after TTL expiration")
	}
}

func TestTranslationCache_MaxSize(t *testing.T) {
	cache := NewTranslationCache()
	cache.SetCacheMaxSize(3)

	from := Format("openai")
	to := Format("anthropic")
	model := "gpt-4"

	// Add 5 entries - should only keep 3
	for i := 0; i < 5; i++ {
		payload := []byte{byte(i)}
		result := []byte{byte(i + 10)}
		cache.Set(from, to, model, payload, result)
	}

	stats := cache.GetCacheStats()
	if stats.Size != 3 {
		t.Errorf("expected size 3 after eviction, got %d", stats.Size)
	}

	// Oldest entries should be evicted (0 and 1)
	_, found := cache.Get(from, to, model, []byte{0})
	if found {
		t.Error("entry 0 should have been evicted")
	}

	_, found = cache.Get(from, to, model, []byte{1})
	if found {
		t.Error("entry 1 should have been evicted")
	}

	// Newer entries should still exist
	_, found = cache.Get(from, to, model, []byte{4})
	if !found {
		t.Error("entry 4 should still exist")
	}
}

func TestTranslationCache_DisableEnable(t *testing.T) {
	cache := NewTranslationCache()

	from := Format("openai")
	to := Format("anthropic")
	model := "gpt-4"
	payload := []byte(`{"test": "data"}`)
	result := []byte(`{"translated": "data"}`)

	// Disable cache
	cache.SetCacheEnabled(false)

	// Set should be no-op
	cache.Set(from, to, model, payload, result)

	// Get should always miss
	_, found := cache.Get(from, to, model, payload)
	if found {
		t.Error("expected cache miss when disabled")
	}

	// Enable and set
	cache.SetCacheEnabled(true)
	cache.Set(from, to, model, payload, result)

	_, found = cache.Get(from, to, model, payload)
	if !found {
		t.Error("expected cache hit after re-enabling")
	}
}

func TestTranslationCache_Clear(t *testing.T) {
	cache := NewTranslationCache()

	from := Format("openai")
	to := Format("anthropic")
	model := "gpt-4"
	payload := []byte(`{"test": "data"}`)
	result := []byte(`{"translated": "data"}`)

	cache.Set(from, to, model, payload, result)

	stats := cache.GetCacheStats()
	if stats.Size != 1 {
		t.Errorf("expected size 1, got %d", stats.Size)
	}

	cache.Clear()

	stats = cache.GetCacheStats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after clear, got %d", stats.Size)
	}
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("expected stats reset after clear")
	}
}

func TestCachedRegistry_Integration(t *testing.T) {
	cache := NewTranslationCache()
	reg := NewCachedRegistry(cache)

	from := Format("test-from")
	to := Format("test-to")

	// Register a transformer that modifies the payload
	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		return []byte(`{"modified": true}`)
	}, ResponseTransform{})

	payload := []byte(`{"original": true}`)

	// First call should execute the transformer
	result1 := reg.TranslateRequest(from, to, "model", payload, false)
	if string(result1) != `{"modified": true}` {
		t.Errorf("unexpected result: %s", string(result1))
	}

	stats := cache.GetCacheStats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}

	// Second call should hit cache
	result2 := reg.TranslateRequest(from, to, "model", payload, false)
	if string(result2) != `{"modified": true}` {
		t.Errorf("unexpected result: %s", string(result2))
	}

	stats = cache.GetCacheStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
}

func TestDefaultCacheFunctions(t *testing.T) {
	// Test package-level functions
	SetCacheEnabled(true)
	SetCacheMaxSize(100)
	SetCacheTTL(5 * time.Minute)

	stats := GetCacheStats()
	if stats.Size < 0 {
		t.Error("invalid cache size")
	}

	ClearCache()
	stats = GetCacheStats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after clear, got %d", stats.Size)
	}
}
