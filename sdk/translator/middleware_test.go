package translator

import (
	"sync"
	"testing"
)

func TestMiddlewareRegistry_RegisterPre(t *testing.T) {
	mr := NewMiddlewareRegistry()

	called := false
	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte {
		called = true
		return payload
	})

	if mr.PreCount() != 1 {
		t.Errorf("PreCount() = %d, want 1", mr.PreCount())
	}

	mr.ApplyPre("from", "to", "model", []byte("test"))
	if !called {
		t.Error("Pre middleware should have been called")
	}
}

func TestMiddlewareRegistry_RegisterPost(t *testing.T) {
	mr := NewMiddlewareRegistry()

	called := false
	mr.RegisterPost(func(from, to Format, model string, payload []byte) []byte {
		called = true
		return payload
	})

	if mr.PostCount() != 1 {
		t.Errorf("PostCount() = %d, want 1", mr.PostCount())
	}

	mr.ApplyPost("from", "to", "model", []byte("test"))
	if !called {
		t.Error("Post middleware should have been called")
	}
}

func TestMiddlewareRegistry_NilMiddleware(t *testing.T) {
	mr := NewMiddlewareRegistry()

	mr.RegisterPre(nil)
	mr.RegisterPost(nil)

	if mr.PreCount() != 0 {
		t.Error("Nil middleware should not be registered")
	}
	if mr.PostCount() != 0 {
		t.Error("Nil middleware should not be registered")
	}
}

func TestMiddlewareRegistry_ExecutionOrder(t *testing.T) {
	mr := NewMiddlewareRegistry()

	var order []int

	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte {
		order = append(order, 1)
		return payload
	})
	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte {
		order = append(order, 2)
		return payload
	})
	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte {
		order = append(order, 3)
		return payload
	})

	mr.ApplyPre("from", "to", "model", []byte("test"))

	if len(order) != 3 {
		t.Fatalf("Expected 3 middleware calls, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("Middleware executed in wrong order: %v", order)
	}
}

func TestMiddlewareRegistry_PayloadTransformation(t *testing.T) {
	mr := NewMiddlewareRegistry()

	// First middleware adds prefix
	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte {
		return append([]byte("pre1:"), payload...)
	})

	// Second middleware adds another prefix
	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte {
		return append([]byte("pre2:"), payload...)
	})

	result := mr.ApplyPre("from", "to", "model", []byte("data"))

	expected := "pre2:pre1:data"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestMiddlewareRegistry_Clear(t *testing.T) {
	mr := NewMiddlewareRegistry()

	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte { return payload })
	mr.RegisterPost(func(from, to Format, model string, payload []byte) []byte { return payload })

	mr.Clear()

	if mr.PreCount() != 0 {
		t.Error("Clear should remove all pre middleware")
	}
	if mr.PostCount() != 0 {
		t.Error("Clear should remove all post middleware")
	}
}

func TestMiddlewareRegistry_ClearPre(t *testing.T) {
	mr := NewMiddlewareRegistry()

	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte { return payload })
	mr.RegisterPost(func(from, to Format, model string, payload []byte) []byte { return payload })

	mr.ClearPre()

	if mr.PreCount() != 0 {
		t.Error("ClearPre should remove all pre middleware")
	}
	if mr.PostCount() != 1 {
		t.Error("ClearPre should not affect post middleware")
	}
}

func TestMiddlewareRegistry_ClearPost(t *testing.T) {
	mr := NewMiddlewareRegistry()

	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte { return payload })
	mr.RegisterPost(func(from, to Format, model string, payload []byte) []byte { return payload })

	mr.ClearPost()

	if mr.PreCount() != 1 {
		t.Error("ClearPost should not affect pre middleware")
	}
	if mr.PostCount() != 0 {
		t.Error("ClearPost should remove all post middleware")
	}
}

func TestMiddlewareRegistry_Clone(t *testing.T) {
	mr := NewMiddlewareRegistry()

	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte { return payload })
	mr.RegisterPost(func(from, to Format, model string, payload []byte) []byte { return payload })

	clone := mr.Clone()

	if clone.PreCount() != mr.PreCount() {
		t.Error("Clone should have same pre middleware count")
	}
	if clone.PostCount() != mr.PostCount() {
		t.Error("Clone should have same post middleware count")
	}

	// Modify original, clone should be unaffected
	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte { return payload })

	if clone.PreCount() == mr.PreCount() {
		t.Error("Clone should be independent of original")
	}
}

func TestMiddlewareRegistry_Concurrent(t *testing.T) {
	mr := NewMiddlewareRegistry()

	// Register some middleware
	for i := 0; i < 5; i++ {
		mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte { return payload })
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mr.ApplyPre("from", "to", "model", []byte("test"))
		}()
	}

	wg.Wait()
	// If we get here without race conditions, test passes
}

func TestConditionalMiddleware(t *testing.T) {
	callCount := 0
	mw := ConditionalMiddleware(
		func(from, to Format, model string) bool {
			return from == FormatOpenAI
		},
		func(from, to Format, model string, payload []byte) []byte {
			callCount++
			return payload
		},
	)

	// Should execute - condition met
	mw(FormatOpenAI, FormatClaude, "model", []byte("test"))
	if callCount != 1 {
		t.Error("Middleware should execute when condition is met")
	}

	// Should not execute - condition not met
	mw(FormatClaude, FormatOpenAI, "model", []byte("test"))
	if callCount != 1 {
		t.Error("Middleware should not execute when condition is not met")
	}
}

func TestChainMiddleware(t *testing.T) {
	chain := ChainMiddleware(
		func(from, to Format, model string, payload []byte) []byte {
			return append(payload, []byte("-a")...)
		},
		func(from, to Format, model string, payload []byte) []byte {
			return append(payload, []byte("-b")...)
		},
		nil, // Should handle nil gracefully
		func(from, to Format, model string, payload []byte) []byte {
			return append(payload, []byte("-c")...)
		},
	)

	result := chain("from", "to", "model", []byte("start"))
	expected := "start-a-b-c"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestTranslateRequestWithMiddleware(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return append([]byte("translated:"), data...)
	}, ResponseTransform{})

	mr := NewMiddlewareRegistry()
	mr.RegisterPre(func(from, to Format, model string, payload []byte) []byte {
		return append([]byte("pre:"), payload...)
	})
	mr.RegisterPost(func(from, to Format, model string, payload []byte) []byte {
		return append([]byte("post:"), payload...)
	})

	result := reg.TranslateRequestWithMiddleware(mr, FormatOpenAI, FormatClaude, "model", []byte("data"), false)

	expected := "post:translated:pre:data"
	if string(result) != expected {
		t.Errorf("Result = %q, want %q", string(result), expected)
	}
}

func TestTranslateRequestWithMiddleware_NilRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FormatOpenAI, FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, ResponseTransform{})

	// Should use default middleware registry when nil
	result := reg.TranslateRequestWithMiddleware(nil, FormatOpenAI, FormatClaude, "model", []byte("data"), false)
	if result == nil {
		t.Error("Should return result even with nil middleware registry")
	}
}

func TestPackageLevelMiddlewareFunctions(t *testing.T) {
	// Clear any existing middleware
	ClearMiddleware()

	called := false
	RegisterPreMiddleware(func(from, to Format, model string, payload []byte) []byte {
		called = true
		return payload
	})

	ApplyPreMiddleware("from", "to", "model", []byte("test"))
	if !called {
		t.Error("Package-level pre middleware should work")
	}

	called = false
	RegisterPostMiddleware(func(from, to Format, model string, payload []byte) []byte {
		called = true
		return payload
	})

	ApplyPostMiddleware("from", "to", "model", []byte("test"))
	if !called {
		t.Error("Package-level post middleware should work")
	}

	// Cleanup
	ClearMiddleware()
}

func TestDefaultMiddlewareRegistry(t *testing.T) {
	mr := DefaultMiddlewareRegistry()
	if mr == nil {
		t.Error("DefaultMiddlewareRegistry should not return nil")
	}
}
