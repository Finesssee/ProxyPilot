package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNewResponseCache tests cache initialization.
func TestNewResponseCache(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled:       true,
		MaxSize:       100,
		TTL:           5 * time.Minute,
		ExcludeModels: []string{"*-thinking"},
	}
	cache := NewResponseCache(cfg)

	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.disabled {
		t.Error("expected cache to be enabled")
	}
	if !cache.IsEnabled() {
		t.Error("IsEnabled() should return true")
	}
}

// TestBasicGetSet tests basic cache get/set operations.
func TestBasicGetSet(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	payload := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	response := []byte(`{"choices":[{"message":{"content":"hi"}}]}`)
	contentType := "application/json"
	statusCode := 200

	// Test miss
	result := cache.Get(model, payload)
	if result != nil {
		t.Error("expected nil for cache miss")
	}

	// Set
	cache.Set(model, payload, response, contentType, statusCode)

	// Test hit
	result = cache.Get(model, payload)
	if result == nil {
		t.Fatal("expected cache hit")
	}
	if string(result.Response) != string(response) {
		t.Errorf("response mismatch: got %s, want %s", result.Response, response)
	}
	if result.ContentType != contentType {
		t.Errorf("content type mismatch: got %s, want %s", result.ContentType, contentType)
	}
	if result.StatusCode != statusCode {
		t.Errorf("status code mismatch: got %d, want %d", result.StatusCode, statusCode)
	}
	if result.Model != model {
		t.Errorf("model mismatch: got %s, want %s", result.Model, model)
	}
}

// TestTTLExpiration tests that entries expire after TTL.
func TestTTLExpiration(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     50 * time.Millisecond, // Very short TTL for testing
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	payload := []byte(`{"test":"ttl"}`)
	response := []byte(`{"result":"ok"}`)

	cache.Set(model, payload, response, "application/json", 200)

	// Should hit immediately
	result := cache.Get(model, payload)
	if result == nil {
		t.Fatal("expected cache hit before TTL")
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Should miss after TTL
	result = cache.Get(model, payload)
	if result != nil {
		t.Error("expected cache miss after TTL expiration")
	}

	// Check eviction stats
	stats := cache.GetStats()
	if stats.Evictions == 0 {
		t.Error("expected at least one eviction due to TTL")
	}
}

// TestLRUEviction tests LRU eviction when at max capacity.
func TestLRUEviction(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	// Fill cache to capacity
	for i := 0; i < 3; i++ {
		model := "gpt-4"
		payload := []byte(fmt.Sprintf(`{"msg":%d}`, i))
		response := []byte(fmt.Sprintf(`{"resp":%d}`, i))
		cache.Set(model, payload, response, "application/json", 200)
	}

	// All 3 should be present
	for i := 0; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"msg":%d}`, i))
		if cache.Get("gpt-4", payload) == nil {
			t.Errorf("expected entry %d to be in cache", i)
		}
	}

	// Add one more, should evict the oldest (index 0)
	cache.Set("gpt-4", []byte(`{"msg":3}`), []byte(`{"resp":3}`), "application/json", 200)

	// Entry 0 should be evicted (oldest)
	if cache.Get("gpt-4", []byte(`{"msg":0}`)) != nil {
		t.Error("expected oldest entry (0) to be evicted")
	}

	// Entry 3 should exist
	if cache.Get("gpt-4", []byte(`{"msg":3}`)) == nil {
		t.Error("expected new entry (3) to be in cache")
	}

	stats := cache.GetStats()
	if stats.Evictions == 0 {
		t.Error("expected eviction count > 0")
	}
}

// TestLRUAccessOrder tests that accessing an entry moves it to the end.
func TestLRUAccessOrder(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 3,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	// Fill cache: 0, 1, 2
	for i := 0; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"msg":%d}`, i))
		response := []byte(fmt.Sprintf(`{"resp":%d}`, i))
		cache.Set("gpt-4", payload, response, "application/json", 200)
	}

	// Access entry 0 to move it to end
	cache.Get("gpt-4", []byte(`{"msg":0}`))

	// Add entry 3, should evict entry 1 (now oldest)
	cache.Set("gpt-4", []byte(`{"msg":3}`), []byte(`{"resp":3}`), "application/json", 200)

	// Entry 0 should still exist (was accessed, moved to end)
	if cache.Get("gpt-4", []byte(`{"msg":0}`)) == nil {
		t.Error("expected recently accessed entry (0) to still be in cache")
	}

	// Entry 1 should be evicted (was oldest after 0 was accessed)
	if cache.Get("gpt-4", []byte(`{"msg":1}`)) != nil {
		t.Error("expected entry 1 to be evicted")
	}
}

// TestModelExclusionPatterns tests wildcard exclusion patterns.
func TestModelExclusionPatterns(t *testing.T) {
	tests := []struct {
		name           string
		excludeModels  []string
		model          string
		shouldBeCached bool
	}{
		{
			name:           "exact match excluded",
			excludeModels:  []string{"gpt-4"},
			model:          "gpt-4",
			shouldBeCached: false,
		},
		{
			name:           "exact match not in list",
			excludeModels:  []string{"gpt-4"},
			model:          "gpt-3.5-turbo",
			shouldBeCached: true,
		},
		{
			name:           "suffix wildcard *-thinking",
			excludeModels:  []string{"*-thinking"},
			model:          "claude-3-opus-thinking",
			shouldBeCached: false,
		},
		{
			name:           "suffix wildcard no match",
			excludeModels:  []string{"*-thinking"},
			model:          "claude-3-opus",
			shouldBeCached: true,
		},
		{
			name:           "prefix wildcard claude-*",
			excludeModels:  []string{"claude-*"},
			model:          "claude-3-sonnet",
			shouldBeCached: false,
		},
		{
			name:           "prefix wildcard no match",
			excludeModels:  []string{"claude-*"},
			model:          "gpt-4",
			shouldBeCached: true,
		},
		{
			name:           "global wildcard *",
			excludeModels:  []string{"*"},
			model:          "any-model",
			shouldBeCached: false,
		},
		{
			name:           "multiple patterns",
			excludeModels:  []string{"*-thinking", "o1-*"},
			model:          "o1-preview",
			shouldBeCached: false,
		},
		{
			name:           "multiple patterns no match",
			excludeModels:  []string{"*-thinking", "o1-*"},
			model:          "gpt-4",
			shouldBeCached: true,
		},
		{
			name:           "empty exclusion list",
			excludeModels:  []string{},
			model:          "gpt-4",
			shouldBeCached: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ResponseCacheConfig{
				Enabled:       true,
				MaxSize:       10,
				TTL:           5 * time.Minute,
				ExcludeModels: tt.excludeModels,
			}
			cache := NewResponseCache(cfg)

			payload := []byte(`{"test":"exclusion"}`)
			response := []byte(`{"result":"ok"}`)

			cache.Set(tt.model, payload, response, "application/json", 200)

			result := cache.Get(tt.model, payload)
			if tt.shouldBeCached && result == nil {
				t.Errorf("expected model %s to be cached", tt.model)
			}
			if !tt.shouldBeCached && result != nil {
				t.Errorf("expected model %s to be excluded from cache", tt.model)
			}
		})
	}
}

// TestMatchPattern tests the wildcard pattern matching function directly.
func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		model   string
		want    bool
	}{
		{"*", "anything", true},
		{"*", "", true},
		{"gpt-4", "gpt-4", true},
		{"gpt-4", "gpt-4-turbo", false},
		{"*-thinking", "claude-thinking", true},
		{"*-thinking", "claude-3-opus-thinking", true},
		{"*-thinking", "claude-3-opus", false},
		{"claude-*", "claude-3", true},
		{"claude-*", "claude-3-sonnet", true},
		{"claude-*", "gpt-4", false},
		{"*-preview", "o1-preview", true},
		{"*-preview", "o1-mini", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.pattern, tt.model), func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.model)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.model, got, tt.want)
			}
		})
	}
}

// TestCacheDisabled tests behavior when cache is disabled.
func TestCacheDisabled(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: false,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	payload := []byte(`{"test":"disabled"}`)
	response := []byte(`{"result":"ok"}`)

	// Set should be no-op
	cache.Set(model, payload, response, "application/json", 200)

	// Get should always return nil
	result := cache.Get(model, payload)
	if result != nil {
		t.Error("expected nil when cache is disabled")
	}

	// Stats should show no entries
	stats := cache.GetStats()
	if stats.Size != 0 {
		t.Errorf("expected size 0, got %d", stats.Size)
	}
}

// TestSetEnabledToggle tests runtime enable/disable toggle.
func TestSetEnabledToggle(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	payload := []byte(`{"test":"toggle"}`)
	response := []byte(`{"result":"ok"}`)

	// Cache works when enabled
	cache.Set(model, payload, response, "application/json", 200)
	if cache.Get(model, payload) == nil {
		t.Error("expected cache hit when enabled")
	}

	// Disable cache
	cache.SetEnabled(false)
	if cache.IsEnabled() {
		t.Error("expected IsEnabled() to return false after disabling")
	}

	// Get should return nil
	if cache.Get(model, payload) != nil {
		t.Error("expected nil after disabling cache")
	}

	// Set should be no-op
	payload2 := []byte(`{"test":"toggle2"}`)
	cache.Set(model, payload2, response, "application/json", 200)

	// Re-enable
	cache.SetEnabled(true)
	if !cache.IsEnabled() {
		t.Error("expected IsEnabled() to return true after enabling")
	}

	// Original entry should still be there
	if cache.Get(model, payload) == nil {
		t.Error("expected original entry to still exist after re-enabling")
	}

	// New entry should not be there (was set while disabled)
	if cache.Get(model, payload2) != nil {
		t.Error("expected entry set while disabled to not exist")
	}
}

// TestStatsTracking tests hit/miss/eviction stats.
func TestStatsTracking(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 2,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	response := []byte(`{"result":"ok"}`)

	// Miss
	cache.Get(model, []byte(`{"miss":1}`))
	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}

	// Set and hit
	cache.Set(model, []byte(`{"hit":1}`), response, "application/json", 200)
	cache.Get(model, []byte(`{"hit":1}`))
	stats = cache.GetStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}

	// Multiple hits on same entry
	cache.Get(model, []byte(`{"hit":1}`))
	cache.Get(model, []byte(`{"hit":1}`))
	stats = cache.GetStats()
	if stats.Hits != 3 {
		t.Errorf("expected 3 hits, got %d", stats.Hits)
	}

	// Fill to capacity and trigger eviction
	cache.Set(model, []byte(`{"hit":2}`), response, "application/json", 200)
	cache.Set(model, []byte(`{"hit":3}`), response, "application/json", 200) // Evicts first entry
	stats = cache.GetStats()
	if stats.Evictions == 0 {
		t.Error("expected eviction count > 0")
	}
	if stats.Size != 2 {
		t.Errorf("expected size 2, got %d", stats.Size)
	}
}

// TestUpdateConfig tests configuration updates.
func TestUpdateConfig(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 5,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	// Fill cache
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"item":%d}`, i))
		cache.Set("gpt-4", payload, []byte(`{"ok":true}`), "application/json", 200)
	}

	stats := cache.GetStats()
	if stats.Size != 5 {
		t.Errorf("expected size 5, got %d", stats.Size)
	}

	// Reduce max size, should evict oldest entries
	newCfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 2,
		TTL:     5 * time.Minute,
	}
	cache.UpdateConfig(newCfg)

	stats = cache.GetStats()
	if stats.Size != 2 {
		t.Errorf("expected size 2 after config update, got %d", stats.Size)
	}

	// Oldest entries (0, 1, 2) should be evicted
	for i := 0; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"item":%d}`, i))
		if cache.Get("gpt-4", payload) != nil {
			t.Errorf("expected entry %d to be evicted", i)
		}
	}

	// Newest entries (3, 4) should remain
	for i := 3; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"item":%d}`, i))
		if cache.Get("gpt-4", payload) == nil {
			t.Errorf("expected entry %d to still exist", i)
		}
	}
}

// TestUpdateConfigDisable tests disabling cache via config update.
func TestUpdateConfigDisable(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	cache.Set("gpt-4", []byte(`{"test":1}`), []byte(`{"ok":true}`), "application/json", 200)

	// Disable via config
	newCfg := ResponseCacheConfig{
		Enabled: false,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache.UpdateConfig(newCfg)

	if cache.IsEnabled() {
		t.Error("expected cache to be disabled after config update")
	}

	// Get should return nil
	if cache.Get("gpt-4", []byte(`{"test":1}`)) != nil {
		t.Error("expected nil after disabling cache")
	}
}

// TestEvictExpired tests the EvictExpired function.
func TestEvictExpired(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     50 * time.Millisecond,
	}
	cache := NewResponseCache(cfg)

	// Add entries
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"item":%d}`, i))
		cache.Set("gpt-4", payload, []byte(`{"ok":true}`), "application/json", 200)
	}

	stats := cache.GetStats()
	if stats.Size != 5 {
		t.Errorf("expected size 5, got %d", stats.Size)
	}

	// Wait for TTL
	time.Sleep(60 * time.Millisecond)

	// Evict expired
	evicted := cache.EvictExpired()
	if evicted != 5 {
		t.Errorf("expected 5 evicted, got %d", evicted)
	}

	stats = cache.GetStats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after eviction, got %d", stats.Size)
	}
}

// TestEvictExpiredMixed tests EvictExpired with mix of expired/fresh entries.
func TestEvictExpiredMixed(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     100 * time.Millisecond,
	}
	cache := NewResponseCache(cfg)

	// Add first batch
	for i := 0; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"old":%d}`, i))
		cache.Set("gpt-4", payload, []byte(`{"ok":true}`), "application/json", 200)
	}

	// Wait half TTL
	time.Sleep(60 * time.Millisecond)

	// Add second batch (fresh)
	for i := 0; i < 2; i++ {
		payload := []byte(fmt.Sprintf(`{"new":%d}`, i))
		cache.Set("gpt-4", payload, []byte(`{"ok":true}`), "application/json", 200)
	}

	// Wait for first batch to expire
	time.Sleep(50 * time.Millisecond)

	// Evict expired
	evicted := cache.EvictExpired()
	if evicted != 3 {
		t.Errorf("expected 3 evicted (old entries), got %d", evicted)
	}

	stats := cache.GetStats()
	if stats.Size != 2 {
		t.Errorf("expected size 2 (new entries), got %d", stats.Size)
	}

	// New entries should still exist
	for i := 0; i < 2; i++ {
		payload := []byte(fmt.Sprintf(`{"new":%d}`, i))
		if cache.Get("gpt-4", payload) == nil {
			t.Errorf("expected new entry %d to still exist", i)
		}
	}
}

// TestClear tests the Clear function.
func TestClear(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	// Add entries
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"item":%d}`, i))
		cache.Set("gpt-4", payload, []byte(`{"ok":true}`), "application/json", 200)
	}

	// Clear
	cache.Clear()

	stats := cache.GetStats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after clear, got %d", stats.Size)
	}

	// All entries should be gone
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"item":%d}`, i))
		if cache.Get("gpt-4", payload) != nil {
			t.Errorf("expected entry %d to be cleared", i)
		}
	}
}

// TestNonSuccessStatusCodes tests that non-2xx responses are not cached.
func TestNonSuccessStatusCodes(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	payload := []byte(`{"test":"status"}`)
	response := []byte(`{"error":"something wrong"}`)

	// 400 error should not be cached
	cache.Set(model, payload, response, "application/json", 400)
	if cache.Get(model, payload) != nil {
		t.Error("expected 400 response to not be cached")
	}

	// 500 error should not be cached
	cache.Set(model, payload, response, "application/json", 500)
	if cache.Get(model, payload) != nil {
		t.Error("expected 500 response to not be cached")
	}

	// 200 should be cached
	cache.Set(model, payload, response, "application/json", 200)
	if cache.Get(model, payload) == nil {
		t.Error("expected 200 response to be cached")
	}
}

// TestEmptyResponseNotCached tests that empty responses are not cached.
func TestEmptyResponseNotCached(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	payload := []byte(`{"test":"empty"}`)

	// Empty response should not be cached
	cache.Set(model, payload, []byte{}, "application/json", 200)
	if cache.Get(model, payload) != nil {
		t.Error("expected empty response to not be cached")
	}

	// Nil response should not be cached
	cache.Set(model, payload, nil, "application/json", 200)
	if cache.Get(model, payload) != nil {
		t.Error("expected nil response to not be cached")
	}
}

// TestHitCountIncrement tests that HitCount is incremented on each access.
func TestHitCountIncrement(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	model := "gpt-4"
	payload := []byte(`{"test":"hitcount"}`)
	response := []byte(`{"result":"ok"}`)

	cache.Set(model, payload, response, "application/json", 200)

	// First access
	result := cache.Get(model, payload)
	if result.HitCount != 1 {
		t.Errorf("expected hit count 1, got %d", result.HitCount)
	}

	// Second access
	result = cache.Get(model, payload)
	if result.HitCount != 2 {
		t.Errorf("expected hit count 2, got %d", result.HitCount)
	}

	// Third access
	result = cache.Get(model, payload)
	if result.HitCount != 3 {
		t.Errorf("expected hit count 3, got %d", result.HitCount)
	}
}

// TestConcurrentAccess tests thread safety with concurrent access.
func TestConcurrentAccess(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 100,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 100

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				payload := []byte(fmt.Sprintf(`{"g":%d,"op":%d}`, id, j))
				response := []byte(fmt.Sprintf(`{"r":%d}`, j))
				cache.Set("gpt-4", payload, response, "application/json", 200)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				payload := []byte(fmt.Sprintf(`{"g":%d,"op":%d}`, id, j))
				cache.Get("gpt-4", payload)
			}
		}(i)
	}

	// Concurrent stats readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.GetStats()
			}
		}()
	}

	// Concurrent IsEnabled calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.IsEnabled()
			}
		}()
	}

	wg.Wait()

	// Just verify it didn't panic and stats are consistent
	stats := cache.GetStats()
	if stats.Size < 0 || stats.Size > cfg.MaxSize {
		t.Errorf("invalid cache size: %d", stats.Size)
	}
}

// TestConcurrentReadWrite tests concurrent reads and writes to same keys.
func TestConcurrentReadWrite(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 50,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	var wg sync.WaitGroup
	model := "gpt-4"
	numGoroutines := 20
	numOperations := 50

	// Pre-populate some entries
	for i := 0; i < 10; i++ {
		payload := []byte(fmt.Sprintf(`{"key":%d}`, i))
		cache.Set(model, payload, []byte(`{"val":"initial"}`), "application/json", 200)
	}

	// Concurrent mixed operations on same keys
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := j % 10 // Use same 10 keys
				payload := []byte(fmt.Sprintf(`{"key":%d}`, key))

				if id%2 == 0 {
					// Writer
					response := []byte(fmt.Sprintf(`{"val":"updated-%d-%d"}`, id, j))
					cache.Set(model, payload, response, "application/json", 200)
				} else {
					// Reader
					cache.Get(model, payload)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is in valid state
	stats := cache.GetStats()
	if stats.Size < 0 || stats.Size > cfg.MaxSize {
		t.Errorf("invalid cache size: %d", stats.Size)
	}
}

// TestGenerateKey tests that different payloads generate different keys.
func TestGenerateKey(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     5 * time.Minute,
	}
	cache := NewResponseCache(cfg)

	// Same model, different payloads
	key1 := cache.generateKey("gpt-4", []byte(`{"a":1}`))
	key2 := cache.generateKey("gpt-4", []byte(`{"a":2}`))
	if key1 == key2 {
		t.Error("expected different keys for different payloads")
	}

	// Different models, same payload
	key3 := cache.generateKey("gpt-4", []byte(`{"a":1}`))
	key4 := cache.generateKey("gpt-3.5-turbo", []byte(`{"a":1}`))
	if key3 == key4 {
		t.Error("expected different keys for different models")
	}

	// Same model and payload should produce same key
	key5 := cache.generateKey("gpt-4", []byte(`{"a":1}`))
	if key1 != key5 {
		t.Error("expected same key for same model and payload")
	}
}

// TestDefaultResponseCacheConfig tests default config values.
func TestDefaultResponseCacheConfig(t *testing.T) {
	cfg := DefaultResponseCacheConfig()

	if cfg.Enabled {
		t.Error("expected Enabled to be false by default")
	}
	if cfg.MaxSize != 1000 {
		t.Errorf("expected MaxSize 1000, got %d", cfg.MaxSize)
	}
	if cfg.TTL != 5*time.Minute {
		t.Errorf("expected TTL 5m, got %v", cfg.TTL)
	}
	if len(cfg.ExcludeModels) != 0 {
		t.Errorf("expected empty ExcludeModels, got %v", cfg.ExcludeModels)
	}
}

// TestBoundaryStatusCodes tests boundary conditions for status codes.
func TestBoundaryStatusCodes(t *testing.T) {
	tests := []struct {
		statusCode     int
		shouldBeCached bool
	}{
		{199, false},
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{301, false},
		{400, false},
		{500, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			cfg := ResponseCacheConfig{
				Enabled: true,
				MaxSize: 10,
				TTL:     5 * time.Minute,
			}
			cache := NewResponseCache(cfg)

			payload := []byte(fmt.Sprintf(`{"status":%d}`, tt.statusCode))
			cache.Set("gpt-4", payload, []byte(`{"ok":true}`), "application/json", tt.statusCode)

			result := cache.Get("gpt-4", payload)
			if tt.shouldBeCached && result == nil {
				t.Errorf("expected status %d to be cached", tt.statusCode)
			}
			if !tt.shouldBeCached && result != nil {
				t.Errorf("expected status %d to NOT be cached", tt.statusCode)
			}
		})
	}
}

// TestUpdateExcludeModels tests updating exclusion patterns via UpdateConfig.
func TestUpdateExcludeModels(t *testing.T) {
	cfg := ResponseCacheConfig{
		Enabled:       true,
		MaxSize:       10,
		TTL:           5 * time.Minute,
		ExcludeModels: []string{},
	}
	cache := NewResponseCache(cfg)

	model := "claude-3-opus"
	payload := []byte(`{"test":"exclude"}`)
	response := []byte(`{"ok":true}`)

	// Should be cached initially
	cache.Set(model, payload, response, "application/json", 200)
	if cache.Get(model, payload) == nil {
		t.Error("expected model to be cached before exclusion")
	}

	// Update config to exclude claude-*
	newCfg := ResponseCacheConfig{
		Enabled:       true,
		MaxSize:       10,
		TTL:           5 * time.Minute,
		ExcludeModels: []string{"claude-*"},
	}
	cache.UpdateConfig(newCfg)

	// Old entry still exists (exclusion applies to new sets, not existing)
	if cache.Get(model, payload) == nil {
		t.Error("expected existing entry to still be accessible")
	}

	// New entry for same model should not be cached
	payload2 := []byte(`{"test":"exclude2"}`)
	cache.Set(model, payload2, response, "application/json", 200)
	if cache.Get(model, payload2) != nil {
		t.Error("expected new entry to be excluded after config update")
	}
}
