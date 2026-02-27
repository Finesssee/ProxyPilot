package translator

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"time"
)

// BenchmarkResult contains timing statistics from a benchmark run.
type BenchmarkResult struct {
	Iterations int           `json:"iterations"`
	Min        time.Duration `json:"min"`
	Max        time.Duration `json:"max"`
	Avg        time.Duration `json:"avg"`
	P50        time.Duration `json:"p50"`
	P95        time.Duration `json:"p95"`
	P99        time.Duration `json:"p99"`
	Total      time.Duration `json:"total"`
	OpsPerSec  float64       `json:"ops_per_sec"`
	Errors     int           `json:"errors"`
}

// BenchmarkConfig configures benchmark behavior.
type BenchmarkConfig struct {
	// WarmupIterations runs before measurement starts
	WarmupIterations int
	// ForceGC runs garbage collection between iterations
	ForceGC bool
	// DisableCache disables caching during benchmark
	DisableCache bool
	// Parallel runs iterations in parallel with this many goroutines
	Parallel int
}

// DefaultBenchmarkConfig returns sensible defaults.
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		WarmupIterations: 10,
		ForceGC:          false,
		DisableCache:     true,
		Parallel:         1,
	}
}

// BenchmarkTranslation measures translation performance.
func BenchmarkTranslation(from, to Format, model string, payload []byte, iterations int) BenchmarkResult {
	return BenchmarkTranslationWithConfig(from, to, model, payload, iterations, DefaultBenchmarkConfig())
}

// BenchmarkTranslationWithConfig measures translation with custom configuration.
func BenchmarkTranslationWithConfig(from, to Format, model string, payload []byte, iterations int, config BenchmarkConfig) BenchmarkResult {
	if iterations < 1 {
		iterations = 1
	}

	// Save and restore cache state if disabling
	if config.DisableCache {
		wasEnabled := defaultCache.IsEnabled()
		defaultCache.SetCacheEnabled(false)
		defer defaultCache.SetCacheEnabled(wasEnabled)
	}

	// Warmup
	for i := 0; i < config.WarmupIterations; i++ {
		_ = TranslateRequest(from, to, model, payload, false)
	}

	if config.ForceGC {
		runtime.GC()
	}

	durations := make([]time.Duration, iterations)
	errors := 0

	if config.Parallel > 1 {
		durations, errors = runParallelBenchmark(from, to, model, payload, iterations, config.Parallel)
	} else {
		for i := 0; i < iterations; i++ {
			start := time.Now()
			result := TranslateRequest(from, to, model, payload, false)
			durations[i] = time.Since(start)
			if result == nil {
				errors++
			}
		}
	}

	return calculateBenchmarkResult(durations, errors)
}

// runParallelBenchmark runs iterations across multiple goroutines.
func runParallelBenchmark(from, to Format, model string, payload []byte, iterations, parallel int) ([]time.Duration, int) {
	durations := make([]time.Duration, iterations)
	errCount := make(chan int, parallel)

	iterPerWorker := iterations / parallel
	remainder := iterations % parallel

	done := make(chan struct{})
	idx := 0

	for w := 0; w < parallel; w++ {
		count := iterPerWorker
		if w < remainder {
			count++
		}
		startIdx := idx
		idx += count

		go func(start, cnt int) {
			localErrors := 0
			for i := 0; i < cnt; i++ {
				s := time.Now()
				result := TranslateRequest(from, to, model, payload, false)
				durations[start+i] = time.Since(s)
				if result == nil {
					localErrors++
				}
			}
			errCount <- localErrors
			done <- struct{}{}
		}(startIdx, count)
	}

	totalErrors := 0
	for i := 0; i < parallel; i++ {
		<-done
		totalErrors += <-errCount
	}

	return durations, totalErrors
}

// calculateBenchmarkResult computes statistics from durations.
func calculateBenchmarkResult(durations []time.Duration, errors int) BenchmarkResult {
	if len(durations) == 0 {
		return BenchmarkResult{}
	}

	// Sort for percentile calculation
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	n := len(durations)
	result := BenchmarkResult{
		Iterations: n,
		Min:        sorted[0],
		Max:        sorted[n-1],
		Avg:        total / time.Duration(n),
		P50:        sorted[n*50/100],
		P95:        sorted[n*95/100],
		P99:        sorted[n*99/100],
		Total:      total,
		Errors:     errors,
	}

	if total > 0 {
		result.OpsPerSec = float64(n) / total.Seconds()
	}

	return result
}

// PathKey represents a translation path for benchmarking.
type PathKey struct {
	From Format
	To   Format
}

// String returns a string representation of the path.
func (pk PathKey) String() string {
	return fmt.Sprintf("%s->%s", pk.From, pk.To)
}

// BenchmarkAllPaths benchmarks all registered translation paths.
func BenchmarkAllPaths(model string, payload []byte, iterations int) map[string]BenchmarkResult {
	return BenchmarkAllPathsWithConfig(model, payload, iterations, DefaultBenchmarkConfig())
}

// BenchmarkAllPathsWithConfig benchmarks all paths with custom configuration.
func BenchmarkAllPathsWithConfig(model string, payload []byte, iterations int, config BenchmarkConfig) map[string]BenchmarkResult {
	results := make(map[string]BenchmarkResult)

	defaultRegistry.mu.RLock()
	paths := make([]PathKey, 0)
	for from, targets := range defaultRegistry.requests {
		for to := range targets {
			paths = append(paths, PathKey{From: from, To: to})
		}
	}
	defaultRegistry.mu.RUnlock()

	for _, path := range paths {
		key := path.String()
		results[key] = BenchmarkTranslationWithConfig(path.From, path.To, model, payload, iterations, config)
	}

	return results
}

// BenchmarkRegistry benchmarks a specific registry instance.
func BenchmarkRegistry(r *Registry, from, to Format, model string, payload []byte, iterations int) BenchmarkResult {
	if iterations < 1 {
		iterations = 1
	}

	config := DefaultBenchmarkConfig()

	// Warmup
	for i := 0; i < config.WarmupIterations; i++ {
		_ = r.TranslateRequest(from, to, model, payload, false)
	}

	durations := make([]time.Duration, iterations)
	errors := 0

	for i := 0; i < iterations; i++ {
		start := time.Now()
		result := r.TranslateRequest(from, to, model, payload, false)
		durations[i] = time.Since(start)
		if result == nil {
			errors++
		}
	}

	return calculateBenchmarkResult(durations, errors)
}

// CompareBenchmarks compares two benchmark results.
type BenchmarkComparison struct {
	Name1        string        `json:"name1"`
	Name2        string        `json:"name2"`
	AvgDiff      time.Duration `json:"avg_diff"`
	AvgDiffPct   float64       `json:"avg_diff_pct"`
	P95Diff      time.Duration `json:"p95_diff"`
	P95DiffPct   float64       `json:"p95_diff_pct"`
	Faster       string        `json:"faster"`
	SpeedupRatio float64       `json:"speedup_ratio"`
}

// CompareBenchmarks compares two benchmark results.
func CompareBenchmarks(name1 string, result1 BenchmarkResult, name2 string, result2 BenchmarkResult) BenchmarkComparison {
	comp := BenchmarkComparison{
		Name1:   name1,
		Name2:   name2,
		AvgDiff: result1.Avg - result2.Avg,
		P95Diff: result1.P95 - result2.P95,
	}

	if result2.Avg > 0 {
		comp.AvgDiffPct = float64(comp.AvgDiff) / float64(result2.Avg) * 100
	}
	if result2.P95 > 0 {
		comp.P95DiffPct = float64(comp.P95Diff) / float64(result2.P95) * 100
	}

	if result1.Avg < result2.Avg {
		comp.Faster = name1
		if result1.Avg > 0 {
			comp.SpeedupRatio = float64(result2.Avg) / float64(result1.Avg)
		}
	} else {
		comp.Faster = name2
		if result2.Avg > 0 {
			comp.SpeedupRatio = float64(result1.Avg) / float64(result2.Avg)
		}
	}

	return comp
}

// BenchmarkSuite provides a comprehensive benchmarking API.
type BenchmarkSuite struct {
	registry *Registry
	config   BenchmarkConfig
	results  map[string]BenchmarkResult
}

// NewBenchmarkSuite creates a new benchmark suite.
func NewBenchmarkSuite(registry *Registry) *BenchmarkSuite {
	if registry == nil {
		registry = defaultRegistry
	}
	return &BenchmarkSuite{
		registry: registry,
		config:   DefaultBenchmarkConfig(),
		results:  make(map[string]BenchmarkResult),
	}
}

// WithConfig sets the benchmark configuration.
func (bs *BenchmarkSuite) WithConfig(config BenchmarkConfig) *BenchmarkSuite {
	bs.config = config
	return bs
}

// Run benchmarks a single translation path.
func (bs *BenchmarkSuite) Run(name string, from, to Format, model string, payload []byte, iterations int) *BenchmarkSuite {
	result := BenchmarkTranslationWithConfig(from, to, model, payload, iterations, bs.config)
	bs.results[name] = result
	return bs
}

// RunAll benchmarks all registered paths.
func (bs *BenchmarkSuite) RunAll(model string, payload []byte, iterations int) *BenchmarkSuite {
	allResults := BenchmarkAllPathsWithConfig(model, payload, iterations, bs.config)
	for k, v := range allResults {
		bs.results[k] = v
	}
	return bs
}

// GetResults returns all benchmark results.
func (bs *BenchmarkSuite) GetResults() map[string]BenchmarkResult {
	return bs.results
}

// GetResult returns a specific benchmark result.
func (bs *BenchmarkSuite) GetResult(name string) (BenchmarkResult, bool) {
	r, ok := bs.results[name]
	return r, ok
}

// Compare compares two named benchmarks in the suite.
func (bs *BenchmarkSuite) Compare(name1, name2 string) (BenchmarkComparison, bool) {
	r1, ok1 := bs.results[name1]
	r2, ok2 := bs.results[name2]
	if !ok1 || !ok2 {
		return BenchmarkComparison{}, false
	}
	return CompareBenchmarks(name1, r1, name2, r2), true
}

// Report generates a summary report of all benchmarks.
func (bs *BenchmarkSuite) Report() []BenchmarkReportEntry {
	entries := make([]BenchmarkReportEntry, 0, len(bs.results))
	for name, result := range bs.results {
		entries = append(entries, BenchmarkReportEntry{
			Name:   name,
			Result: result,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Result.Avg < entries[j].Result.Avg
	})
	return entries
}

// BenchmarkReportEntry is a single entry in a benchmark report.
type BenchmarkReportEntry struct {
	Name   string          `json:"name"`
	Result BenchmarkResult `json:"result"`
}

// BenchmarkWithContext benchmarks a translation with context timeout support.
func BenchmarkWithContext(ctx context.Context, from, to Format, model string, payload []byte, iterations int) (BenchmarkResult, error) {
	_ = DefaultBenchmarkConfig() // Config available for future use

	if ctx == nil {
		ctx = context.Background()
	}

	durations := make([]time.Duration, 0, iterations)
	errors := 0

	for i := 0; i < iterations; i++ {
		select {
		case <-ctx.Done():
			// Return partial results
			return calculateBenchmarkResult(durations, errors), ctx.Err()
		default:
			start := time.Now()
			result := TranslateRequest(from, to, model, payload, false)
			durations = append(durations, time.Since(start))
			if result == nil {
				errors++
			}
		}
	}

	return calculateBenchmarkResult(durations, errors), nil
}

// MemoryBenchmark measures memory allocation during translation.
type MemoryBenchmark struct {
	AllocsBefore uint64        `json:"allocs_before"`
	AllocsAfter  uint64        `json:"allocs_after"`
	AllocsDiff   uint64        `json:"allocs_diff"`
	BytesBefore  uint64        `json:"bytes_before"`
	BytesAfter   uint64        `json:"bytes_after"`
	BytesDiff    uint64        `json:"bytes_diff"`
	Duration     time.Duration `json:"duration"`
}

// BenchmarkMemory measures memory usage of a translation.
func BenchmarkMemory(from, to Format, model string, payload []byte) MemoryBenchmark {
	var memBefore, memAfter runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	start := time.Now()
	_ = TranslateRequest(from, to, model, payload, false)
	duration := time.Since(start)

	runtime.ReadMemStats(&memAfter)

	return MemoryBenchmark{
		AllocsBefore: memBefore.Mallocs,
		AllocsAfter:  memAfter.Mallocs,
		AllocsDiff:   memAfter.Mallocs - memBefore.Mallocs,
		BytesBefore:  memBefore.TotalAlloc,
		BytesAfter:   memAfter.TotalAlloc,
		BytesDiff:    memAfter.TotalAlloc - memBefore.TotalAlloc,
		Duration:     duration,
	}
}
