package translator

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestRegistryLazy_BasicLoad(t *testing.T) {
	reg := NewRegistry()

	from := Format("lazy-from")
	to := Format("lazy-to")

	var loadCount atomic.Int32

	// Register a lazy loader
	reg.RegisterLazy(from, to, func() RequestTransform {
		loadCount.Add(1)
		return func(model string, rawJSON []byte, stream bool) []byte {
			return append([]byte("lazy:"), rawJSON...)
		}
	})

	// Should not be loaded yet
	if reg.IsLazyLoaded(from, to) {
		t.Error("should not be loaded before first use")
	}
	if loadCount.Load() != 0 {
		t.Error("loader should not have been called yet")
	}

	// First translation should trigger load
	result := reg.TranslateRequest(from, to, "model", []byte("test"), false)

	if string(result) != "lazy:test" {
		t.Errorf("unexpected result: %s", string(result))
	}
	if loadCount.Load() != 1 {
		t.Errorf("expected 1 load, got %d", loadCount.Load())
	}
	if !reg.IsLazyLoaded(from, to) {
		t.Error("should be loaded after first use")
	}

	// Second translation should NOT trigger another load
	result = reg.TranslateRequest(from, to, "model", []byte("test2"), false)

	if string(result) != "lazy:test2" {
		t.Errorf("unexpected result: %s", string(result))
	}
	if loadCount.Load() != 1 {
		t.Errorf("expected still 1 load, got %d", loadCount.Load())
	}
}

func TestRegistryLazy_HasRequestTranslator(t *testing.T) {
	reg := NewRegistry()

	from := Format("has-lazy-from")
	to := Format("has-lazy-to")

	// Before registration
	if reg.HasRequestTranslator(from, to) {
		t.Error("should not have translator before registration")
	}

	// Register lazy
	reg.RegisterLazy(from, to, func() RequestTransform {
		return func(model string, rawJSON []byte, stream bool) []byte {
			return rawJSON
		}
	})

	// After registration (but before load)
	if !reg.HasRequestTranslator(from, to) {
		t.Error("should have translator after lazy registration")
	}
}

func TestRegistryLazy_ResponseLoader(t *testing.T) {
	reg := NewRegistry()

	from := Format("lazy-resp-from")
	to := Format("lazy-resp-to")

	var loadCount atomic.Int32

	reg.RegisterLazyResponse(from, to, func() ResponseTransform {
		loadCount.Add(1)
		return ResponseTransform{
			NonStream: func(ctx context.Context, model string, origReq, req, resp []byte, param *any) string {
				return "lazy-response"
			},
		}
	})

	// Should report having response transformer
	if !reg.HasResponseTransformer(from, to) {
		t.Error("should have response transformer after lazy registration")
	}

	// Load count should still be 0
	if loadCount.Load() != 0 {
		t.Error("loader should not have been called yet")
	}
}

func TestRegistryLazy_BothLoaders(t *testing.T) {
	reg := NewRegistry()

	from := Format("both-from")
	to := Format("both-to")

	var reqLoadCount, respLoadCount atomic.Int32

	reg.RegisterLazyBoth(from, to,
		func() RequestTransform {
			reqLoadCount.Add(1)
			return func(model string, rawJSON []byte, stream bool) []byte {
				return rawJSON
			}
		},
		func() ResponseTransform {
			respLoadCount.Add(1)
			return ResponseTransform{}
		},
	)

	stats := reg.GetLazyLoadStats()
	if stats.TotalLazy != 2 {
		t.Errorf("expected 2 lazy registrations, got %d", stats.TotalLazy)
	}
	if stats.TotalLoaded != 0 {
		t.Errorf("expected 0 loaded, got %d", stats.TotalLoaded)
	}

	// Trigger request load
	reg.TranslateRequest(from, to, "m", []byte("t"), false)

	stats = reg.GetLazyLoadStats()
	if stats.TotalLoaded != 1 {
		t.Errorf("expected 1 loaded after request, got %d", stats.TotalLoaded)
	}
}

func TestRegistryLazy_ForceLoad(t *testing.T) {
	reg := NewRegistry()

	from := Format("force-from")
	to := Format("force-to")

	var loadCount atomic.Int32

	reg.RegisterLazy(from, to, func() RequestTransform {
		loadCount.Add(1)
		return func(model string, rawJSON []byte, stream bool) []byte {
			return rawJSON
		}
	})

	// Force load all lazy transformers
	reg.ForceLoadLazy()

	if loadCount.Load() != 1 {
		t.Errorf("expected 1 load after ForceLoadLazy, got %d", loadCount.Load())
	}

	// Force load again - should not trigger additional loads
	reg.ForceLoadLazy()

	if loadCount.Load() != 1 {
		t.Errorf("expected still 1 load, got %d", loadCount.Load())
	}
}

func TestRegistryLazy_PriorityOverLazy(t *testing.T) {
	reg := NewRegistry()

	from := Format("priority-from")
	to := Format("priority-to")

	var lazyLoadCount atomic.Int32

	// Register lazy first
	reg.RegisterLazy(from, to, func() RequestTransform {
		lazyLoadCount.Add(1)
		return func(model string, rawJSON []byte, stream bool) []byte {
			return []byte("lazy")
		}
	})

	// Register regular (should take priority)
	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		return []byte("regular")
	}, ResponseTransform{})

	result := reg.TranslateRequest(from, to, "m", []byte("t"), false)

	// Regular should take priority
	if string(result) != "regular" {
		t.Errorf("expected 'regular', got %s", string(result))
	}

	// Lazy loader should NOT have been called
	if lazyLoadCount.Load() != 0 {
		t.Errorf("lazy loader should not be called when regular exists, got %d calls", lazyLoadCount.Load())
	}
}

func TestRegistryLazy_ConcurrentLoad(t *testing.T) {
	reg := NewRegistry()

	from := Format("concurrent-from")
	to := Format("concurrent-to")

	var loadCount atomic.Int32

	reg.RegisterLazy(from, to, func() RequestTransform {
		loadCount.Add(1)
		return func(model string, rawJSON []byte, stream bool) []byte {
			return rawJSON
		}
	})

	// Trigger concurrent loads
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			reg.TranslateRequest(from, to, "m", []byte("t"), false)
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should only load once despite concurrent access
	if loadCount.Load() != 1 {
		t.Errorf("expected exactly 1 load with concurrent access, got %d", loadCount.Load())
	}
}

func TestPackageLevelLazy_Functions(t *testing.T) {
	from := Format("pkg-lazy-from")
	to := Format("pkg-lazy-to")

	var loadCount atomic.Int32

	RegisterLazy(from, to, func() RequestTransform {
		loadCount.Add(1)
		return func(model string, rawJSON []byte, stream bool) []byte {
			return rawJSON
		}
	})

	if IsLazyLoaded(from, to) {
		t.Error("should not be loaded yet")
	}

	stats := GetLazyLoadStats()
	if stats.TotalLazy == 0 {
		t.Error("expected at least 1 lazy registration")
	}

	// Force load
	ForceLoadLazy()

	if !IsLazyLoaded(from, to) {
		t.Error("should be loaded after ForceLoadLazy")
	}
}

func TestRegistryLazy_IsLazyLoaded_NotRegistered(t *testing.T) {
	reg := NewRegistry()

	// Check a path that was never registered
	if reg.IsLazyLoaded("never", "registered") {
		t.Error("should return false for unregistered path")
	}
}

func TestRegistryLazy_EmptyRegistry(t *testing.T) {
	reg := NewRegistry()

	stats := reg.GetLazyLoadStats()
	if stats.TotalLazy != 0 || stats.TotalLoaded != 0 {
		t.Error("empty registry should have zero lazy stats")
	}

	// ForceLoadLazy on empty should not panic
	reg.ForceLoadLazy()
}
