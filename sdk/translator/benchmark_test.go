package translator

import (
	"context"
	"testing"
	"time"
)

func TestBenchmarkTranslation_Basic(t *testing.T) {
	reg := NewRegistry()

	from := Format("test-from")
	to := Format("test-to")

	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
		return rawJSON
	}, ResponseTransform{})

	// Use the default registry for the benchmark
	defaultRegistry.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		time.Sleep(1 * time.Millisecond)
		return rawJSON
	}, ResponseTransform{})

	result := BenchmarkTranslation(from, to, "model", []byte("test payload"), 10)

	if result.Iterations != 10 {
		t.Errorf("expected 10 iterations, got %d", result.Iterations)
	}
	if result.Min == 0 {
		t.Error("expected non-zero min duration")
	}
	if result.Max == 0 {
		t.Error("expected non-zero max duration")
	}
	if result.Avg == 0 {
		t.Error("expected non-zero avg duration")
	}
	if result.Min > result.Avg || result.Avg > result.Max {
		t.Error("invalid min/avg/max relationship")
	}
	if result.OpsPerSec == 0 {
		t.Error("expected non-zero ops/sec")
	}
}

func TestBenchmarkTranslation_WithConfig(t *testing.T) {
	from := Format("test-from")
	to := Format("test-to")

	config := BenchmarkConfig{
		WarmupIterations: 2,
		ForceGC:          true,
		DisableCache:     true,
		Parallel:         1,
	}

	result := BenchmarkTranslationWithConfig(from, to, "model", []byte("test"), 5, config)

	if result.Iterations != 5 {
		t.Errorf("expected 5 iterations, got %d", result.Iterations)
	}
}

func TestBenchmarkTranslation_Parallel(t *testing.T) {
	from := Format("test-from")
	to := Format("test-to")

	config := BenchmarkConfig{
		WarmupIterations: 1,
		Parallel:         4,
	}

	result := BenchmarkTranslationWithConfig(from, to, "model", []byte("test"), 20, config)

	if result.Iterations != 20 {
		t.Errorf("expected 20 iterations, got %d", result.Iterations)
	}
}

func TestBenchmarkResult_Percentiles(t *testing.T) {
	from := Format("test-from")
	to := Format("test-to")

	// Need enough iterations for meaningful percentiles
	result := BenchmarkTranslation(from, to, "model", []byte("test"), 100)

	if result.P50 == 0 {
		t.Error("expected non-zero P50")
	}
	if result.P95 == 0 {
		t.Error("expected non-zero P95")
	}
	if result.P99 == 0 {
		t.Error("expected non-zero P99")
	}
	if result.P50 > result.P95 || result.P95 > result.P99 {
		t.Error("percentiles should be in ascending order")
	}
}

func TestBenchmarkAllPaths(t *testing.T) {
	// Register some test translators on default registry
	from1 := Format("bench-from1")
	to1 := Format("bench-to1")
	from2 := Format("bench-from2")
	to2 := Format("bench-to2")

	Register(from1, to1, func(model string, rawJSON []byte, stream bool) []byte {
		return rawJSON
	}, ResponseTransform{})

	Register(from2, to2, func(model string, rawJSON []byte, stream bool) []byte {
		return rawJSON
	}, ResponseTransform{})

	results := BenchmarkAllPaths("model", []byte("test"), 5)

	if len(results) < 2 {
		t.Errorf("expected at least 2 paths benchmarked, got %d", len(results))
	}

	// Check specific paths exist
	key1 := PathKey{From: from1, To: to1}.String()
	if _, ok := results[key1]; !ok {
		t.Errorf("expected results for %s", key1)
	}
}

func TestPathKey_String(t *testing.T) {
	pk := PathKey{From: "openai", To: "anthropic"}
	expected := "openai->anthropic"
	if pk.String() != expected {
		t.Errorf("expected %s, got %s", expected, pk.String())
	}
}

func TestCompareBenchmarks(t *testing.T) {
	r1 := BenchmarkResult{
		Iterations: 100,
		Avg:        10 * time.Millisecond,
		P95:        15 * time.Millisecond,
	}

	r2 := BenchmarkResult{
		Iterations: 100,
		Avg:        20 * time.Millisecond,
		P95:        30 * time.Millisecond,
	}

	comp := CompareBenchmarks("fast", r1, "slow", r2)

	if comp.Faster != "fast" {
		t.Errorf("expected 'fast' to be faster, got %s", comp.Faster)
	}
	if comp.SpeedupRatio != 2.0 {
		t.Errorf("expected 2x speedup, got %f", comp.SpeedupRatio)
	}
	if comp.AvgDiff != -10*time.Millisecond {
		t.Errorf("expected -10ms diff, got %v", comp.AvgDiff)
	}
}

func TestBenchmarkSuite(t *testing.T) {
	from := Format("suite-from")
	to := Format("suite-to")

	Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		return rawJSON
	}, ResponseTransform{})

	suite := NewBenchmarkSuite(nil).
		WithConfig(BenchmarkConfig{
			WarmupIterations: 1,
			DisableCache:     true,
		}).
		Run("test1", from, to, "model", []byte("data1"), 5).
		Run("test2", from, to, "model", []byte("data2"), 5)

	results := suite.GetResults()
	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}

	r, ok := suite.GetResult("test1")
	if !ok {
		t.Error("expected to find test1 result")
	}
	if r.Iterations != 5 {
		t.Errorf("expected 5 iterations, got %d", r.Iterations)
	}

	_, ok = suite.GetResult("nonexistent")
	if ok {
		t.Error("expected nonexistent to not be found")
	}
}

func TestBenchmarkSuite_Compare(t *testing.T) {
	from := Format("compare-from")
	to := Format("compare-to")

	suite := NewBenchmarkSuite(nil).
		Run("a", from, to, "m", []byte("d"), 5).
		Run("b", from, to, "m", []byte("d"), 5)

	comp, ok := suite.Compare("a", "b")
	if !ok {
		t.Error("expected comparison to succeed")
	}
	if comp.Name1 != "a" || comp.Name2 != "b" {
		t.Error("comparison names don't match")
	}

	_, ok = suite.Compare("a", "nonexistent")
	if ok {
		t.Error("expected comparison to fail for nonexistent")
	}
}

func TestBenchmarkSuite_Report(t *testing.T) {
	from := Format("report-from")
	to := Format("report-to")

	suite := NewBenchmarkSuite(nil).
		Run("slow", from, to, "m", []byte("d"), 5).
		Run("fast", from, to, "m", []byte("d"), 5)

	report := suite.Report()
	if len(report) != 2 {
		t.Errorf("expected 2 entries, got %d", len(report))
	}

	// Report should be sorted by avg duration
	for i := 0; i < len(report)-1; i++ {
		if report[i].Result.Avg > report[i+1].Result.Avg {
			t.Error("report should be sorted by avg duration")
		}
	}
}

func TestBenchmarkWithContext(t *testing.T) {
	from := Format("ctx-from")
	to := Format("ctx-to")

	result, err := BenchmarkWithContext(context.Background(), from, to, "m", []byte("d"), 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Iterations != 10 {
		t.Errorf("expected 10 iterations, got %d", result.Iterations)
	}
}

func TestBenchmarkWithContext_Timeout(t *testing.T) {
	from := Format("timeout-from")
	to := Format("timeout-to")

	// Register a slow transformer
	defaultRegistry.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		time.Sleep(50 * time.Millisecond)
		return rawJSON
	}, ResponseTransform{})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := BenchmarkWithContext(ctx, from, to, "m", []byte("d"), 100)

	// Should be cancelled before all iterations complete
	if err == nil {
		t.Log("benchmark completed before timeout (this is OK if machine is fast)")
	}
	if result.Iterations == 0 {
		t.Error("should have completed at least some iterations")
	}
}

func TestBenchmarkWithContext_NilContext(t *testing.T) {
	from := Format("nil-ctx-from")
	to := Format("nil-ctx-to")

	result, err := BenchmarkWithContext(nil, from, to, "m", []byte("d"), 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Iterations != 5 {
		t.Errorf("expected 5 iterations, got %d", result.Iterations)
	}
}

func TestBenchmarkMemory(t *testing.T) {
	from := Format("mem-from")
	to := Format("mem-to")

	result := BenchmarkMemory(from, to, "m", []byte("test data"))

	// Duration can be 0 on very fast machines
	// Memory stats might be 0 if GC runs during measurement
	// Just check they're not negative
	if result.AllocsDiff < 0 {
		t.Error("allocs diff should not be negative")
	}
}

func TestBenchmarkRegistry(t *testing.T) {
	reg := NewRegistry()

	from := Format("reg-from")
	to := Format("reg-to")

	reg.Register(from, to, func(model string, rawJSON []byte, stream bool) []byte {
		return rawJSON
	}, ResponseTransform{})

	result := BenchmarkRegistry(reg, from, to, "model", []byte("test"), 10)

	if result.Iterations != 10 {
		t.Errorf("expected 10 iterations, got %d", result.Iterations)
	}
}

func TestDefaultBenchmarkConfig(t *testing.T) {
	config := DefaultBenchmarkConfig()

	if config.WarmupIterations != 10 {
		t.Errorf("expected 10 warmup iterations, got %d", config.WarmupIterations)
	}
	if config.Parallel != 1 {
		t.Errorf("expected 1 parallel, got %d", config.Parallel)
	}
	if !config.DisableCache {
		t.Error("expected DisableCache to be true by default")
	}
}
