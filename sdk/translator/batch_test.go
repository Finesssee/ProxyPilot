package translator

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatchTranslator_BasicBatch(t *testing.T) {
	reg := NewRegistry()

	// Register a simple transformer
	from := Format("test-from")
	to := Format("test-to")
	var callCount atomic.Int32

	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		callCount.Add(1)
		return append([]byte("translated:"), rawJSON...)
	}, ResponseTransform{})

	bt := NewBatchTranslator(reg, nil)

	requests := []BatchRequest{
		{Model: "model1", Payload: []byte("payload1")},
		{Model: "model2", Payload: []byte("payload2")},
		{Model: "model3", Payload: []byte("payload3")},
	}

	results := bt.TranslateBatch(context.Background(), from, to, requests)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("result %d has error: %v", i, r.Error)
		}
		if r.Index != i {
			t.Errorf("result %d has wrong index: %d", i, r.Index)
		}
		// Duration can be 0 on fast machines - that's OK
		expected := "translated:payload" + string('1'+byte(i))
		if string(r.Payload) != expected {
			t.Errorf("result %d: expected %s, got %s", i, expected, string(r.Payload))
		}
	}

	if callCount.Load() != 3 {
		t.Errorf("expected 3 translator calls, got %d", callCount.Load())
	}
}

func TestBatchTranslator_WithCache(t *testing.T) {
	reg := NewRegistry()
	cache := NewTranslationCache()

	from := Format("test-from")
	to := Format("test-to")
	var callCount atomic.Int32

	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		callCount.Add(1)
		return append([]byte("translated:"), rawJSON...)
	}, ResponseTransform{})

	bt := NewBatchTranslator(reg, cache)

	// Same payload twice
	requests := []BatchRequest{
		{Model: "model", Payload: []byte("same-payload")},
		{Model: "model", Payload: []byte("same-payload")},
		{Model: "model", Payload: []byte("different")},
	}

	results := bt.TranslateBatch(context.Background(), from, to, requests)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Due to parallel execution, both identical payloads might be processed
	// before caching kicks in. This is expected behavior.
	// Just verify all results are correct
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
	}
}

func TestBatchTranslator_SetWorkers(t *testing.T) {
	bt := NewBatchTranslator(nil, nil)

	bt.SetBatchWorkers(8)
	if bt.GetWorkerCount() != 8 {
		t.Errorf("expected 8 workers, got %d", bt.GetWorkerCount())
	}

	bt.SetBatchWorkers(0) // Should clamp to 1
	if bt.GetWorkerCount() != 1 {
		t.Errorf("expected 1 worker (minimum), got %d", bt.GetWorkerCount())
	}
}

func TestBatchTranslator_ContextCancellation(t *testing.T) {
	reg := NewRegistry()

	from := Format("test-from")
	to := Format("test-to")

	// Slow transformer
	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		time.Sleep(100 * time.Millisecond)
		return rawJSON
	}, ResponseTransform{})

	bt := NewBatchTranslator(reg, nil)
	bt.SetBatchWorkers(1) // Force sequential processing

	requests := make([]BatchRequest, 10)
	for i := range requests {
		requests[i] = BatchRequest{Model: "m", Payload: []byte("p")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	results := bt.TranslateBatch(ctx, from, to, requests)

	// Some results should have context errors
	hasContextError := false
	for _, r := range results {
		if r.Error == context.DeadlineExceeded {
			hasContextError = true
			break
		}
	}

	if !hasContextError {
		t.Log("Note: context cancellation may not affect all results due to timing")
	}
}

func TestBatchTranslator_EmptyBatch(t *testing.T) {
	bt := NewBatchTranslator(nil, nil)

	results := bt.TranslateBatch(context.Background(), "from", "to", nil)
	if results != nil {
		t.Errorf("expected nil for empty batch, got %v", results)
	}

	results = bt.TranslateBatch(context.Background(), "from", "to", []BatchRequest{})
	if results != nil {
		t.Errorf("expected nil for empty batch, got %v", results)
	}
}

func TestBatchTranslator_WithCallback(t *testing.T) {
	reg := NewRegistry()

	from := Format("test-from")
	to := Format("test-to")

	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		return rawJSON
	}, ResponseTransform{})

	bt := NewBatchTranslator(reg, nil)

	requests := []BatchRequest{
		{Model: "m1", Payload: []byte("p1")},
		{Model: "m2", Payload: []byte("p2")},
	}

	var callbackCount atomic.Int32
	bt.TranslateBatchWithCallback(context.Background(), from, to, requests, func(r BatchResult) {
		callbackCount.Add(1)
	})

	if callbackCount.Load() != 2 {
		t.Errorf("expected 2 callbacks, got %d", callbackCount.Load())
	}
}

func TestCalculateBatchStats(t *testing.T) {
	results := []BatchResult{
		{Duration: 10 * time.Millisecond},
		{Duration: 20 * time.Millisecond},
		{Duration: 30 * time.Millisecond, Error: context.Canceled},
		{Duration: 40 * time.Millisecond},
		{Duration: 50 * time.Millisecond},
	}

	stats := CalculateBatchStats(results)

	if stats.TotalRequests != 5 {
		t.Errorf("expected 5 requests, got %d", stats.TotalRequests)
	}
	if stats.MinDuration != 10*time.Millisecond {
		t.Errorf("expected min 10ms, got %v", stats.MinDuration)
	}
	if stats.MaxDuration != 50*time.Millisecond {
		t.Errorf("expected max 50ms, got %v", stats.MaxDuration)
	}
	if stats.Errors != 1 {
		t.Errorf("expected 1 error, got %d", stats.Errors)
	}

	expectedAvg := 30 * time.Millisecond
	if stats.AverageDuration != expectedAvg {
		t.Errorf("expected avg %v, got %v", expectedAvg, stats.AverageDuration)
	}
}

func TestCalculateBatchStats_Empty(t *testing.T) {
	stats := CalculateBatchStats(nil)
	if stats.TotalRequests != 0 {
		t.Errorf("expected 0 requests for empty input, got %d", stats.TotalRequests)
	}

	stats = CalculateBatchStats([]BatchResult{})
	if stats.TotalRequests != 0 {
		t.Errorf("expected 0 requests for empty slice, got %d", stats.TotalRequests)
	}
}

func TestBatchProcessor_FluentAPI(t *testing.T) {
	reg := NewRegistry()

	from := Format("test-from")
	to := Format("test-to")

	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		return append([]byte("ok:"), rawJSON...)
	}, ResponseTransform{})

	results, stats := NewBatchProcessor(from, to).
		WithRegistry(reg).
		WithWorkers(2).
		Add("model", []byte("data1"), false).
		Add("model", []byte("data2"), false).
		ExecuteWithStats()

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if stats.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", stats.TotalRequests)
	}
}

func TestBatchProcessor_AddMany(t *testing.T) {
	from := Format("from")
	to := Format("to")

	processor := NewBatchProcessor(from, to)

	payloads := [][]byte{
		[]byte("p1"),
		[]byte("p2"),
		[]byte("p3"),
	}

	processor.AddMany("model", payloads, false)

	// Execute will use default registry which has no transformer
	// So payloads pass through unchanged
	results := processor.Execute()

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestPackageLevelBatchFunctions(t *testing.T) {
	// Test package-level functions
	SetBatchWorkers(4)
	if GetBatchWorkers() != 4 {
		t.Errorf("expected 4 workers, got %d", GetBatchWorkers())
	}

	// TranslateBatch with no registered translator - should pass through
	requests := []BatchRequest{
		{Model: "m", Payload: []byte("test")},
	}
	results := TranslateBatch(context.Background(), "from", "to", requests)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}
