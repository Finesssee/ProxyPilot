package translator

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// BatchRequest represents a single request in a batch translation.
type BatchRequest struct {
	Format  Format
	Model   string
	Payload []byte
	Stream  bool
}

// BatchResult contains the result of a single batch translation.
type BatchResult struct {
	Payload  []byte        `json:"payload"`
	Error    error         `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	Index    int           `json:"index"`
}

// BatchTranslator handles parallel translation of multiple requests.
type BatchTranslator struct {
	registry   *Registry
	cache      *TranslationCache
	workers    int
	workerPool chan struct{}
	mu         sync.RWMutex
}

// NewBatchTranslator creates a new batch translator.
func NewBatchTranslator(registry *Registry, cache *TranslationCache) *BatchTranslator {
	if registry == nil {
		registry = defaultRegistry
	}
	workers := 4 // default worker count
	return &BatchTranslator{
		registry:   registry,
		cache:      cache,
		workers:    workers,
		workerPool: make(chan struct{}, workers),
	}
}

// SetBatchWorkers sets the number of concurrent workers.
func (bt *BatchTranslator) SetBatchWorkers(n int) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	if n < 1 {
		n = 1
	}
	bt.workers = n
	bt.workerPool = make(chan struct{}, n)
}

// GetWorkerCount returns the current number of workers.
func (bt *BatchTranslator) GetWorkerCount() int {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.workers
}

// TranslateBatch translates multiple requests in parallel.
func (bt *BatchTranslator) TranslateBatch(ctx context.Context, from, to Format, requests []BatchRequest) []BatchResult {
	if len(requests) == 0 {
		return nil
	}

	results := make([]BatchResult, len(requests))
	var wg sync.WaitGroup

	bt.mu.RLock()
	workerPool := bt.workerPool
	bt.mu.RUnlock()

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r BatchRequest) {
			defer wg.Done()

			// Acquire worker slot
			select {
			case workerPool <- struct{}{}:
				defer func() { <-workerPool }()
			case <-ctx.Done():
				results[idx] = BatchResult{
					Index:    idx,
					Error:    ctx.Err(),
					Duration: 0,
				}
				return
			}

			start := time.Now()

			// Try cache first if available
			var payload []byte
			var cached bool
			if bt.cache != nil {
				payload, cached = bt.cache.Get(from, to, r.Model, r.Payload)
			}

			if !cached {
				payload = bt.registry.TranslateRequest(from, to, r.Model, r.Payload, r.Stream)
				if bt.cache != nil {
					bt.cache.Set(from, to, r.Model, r.Payload, payload)
				}
			}

			results[idx] = BatchResult{
				Index:    idx,
				Payload:  payload,
				Duration: time.Since(start),
			}
		}(i, req)
	}

	wg.Wait()
	return results
}

// TranslateBatchWithCallback translates with per-result callback.
func (bt *BatchTranslator) TranslateBatchWithCallback(
	ctx context.Context,
	from, to Format,
	requests []BatchRequest,
	callback func(result BatchResult),
) {
	if len(requests) == 0 {
		return
	}

	var wg sync.WaitGroup

	bt.mu.RLock()
	workerPool := bt.workerPool
	bt.mu.RUnlock()

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r BatchRequest) {
			defer wg.Done()

			// Acquire worker slot
			select {
			case workerPool <- struct{}{}:
				defer func() { <-workerPool }()
			case <-ctx.Done():
				if callback != nil {
					callback(BatchResult{
						Index:    idx,
						Error:    ctx.Err(),
						Duration: 0,
					})
				}
				return
			}

			start := time.Now()

			var payload []byte
			var cached bool
			if bt.cache != nil {
				payload, cached = bt.cache.Get(from, to, r.Model, r.Payload)
			}

			if !cached {
				payload = bt.registry.TranslateRequest(from, to, r.Model, r.Payload, r.Stream)
				if bt.cache != nil {
					bt.cache.Set(from, to, r.Model, r.Payload, payload)
				}
			}

			result := BatchResult{
				Index:    idx,
				Payload:  payload,
				Duration: time.Since(start),
			}

			if callback != nil {
				callback(result)
			}
		}(i, req)
	}

	wg.Wait()
}

// BatchStats contains statistics about batch processing.
type BatchStats struct {
	TotalRequests   int64         `json:"total_requests"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	MinDuration     time.Duration `json:"min_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	Errors          int           `json:"errors"`
}

// CalculateBatchStats computes statistics from batch results.
func CalculateBatchStats(results []BatchResult) BatchStats {
	if len(results) == 0 {
		return BatchStats{}
	}

	stats := BatchStats{
		TotalRequests: int64(len(results)),
		MinDuration:   results[0].Duration,
		MaxDuration:   results[0].Duration,
	}

	var total time.Duration
	for _, r := range results {
		total += r.Duration
		if r.Duration < stats.MinDuration {
			stats.MinDuration = r.Duration
		}
		if r.Duration > stats.MaxDuration {
			stats.MaxDuration = r.Duration
		}
		if r.Error != nil {
			stats.Errors++
		}
	}

	stats.TotalDuration = total
	if len(results) > 0 {
		stats.AverageDuration = total / time.Duration(len(results))
	}

	return stats
}

// defaultBatchTranslator is the package-level batch translator.
var defaultBatchTranslator = NewBatchTranslator(nil, nil)
var batchWorkerCount atomic.Int32

func init() {
	batchWorkerCount.Store(4)
}

// SetBatchWorkers sets the worker count for the default batch translator.
func SetBatchWorkers(n int) {
	batchWorkerCount.Store(int32(n))
	defaultBatchTranslator.SetBatchWorkers(n)
}

// GetBatchWorkers returns the current worker count.
func GetBatchWorkers() int {
	return int(batchWorkerCount.Load())
}

// TranslateBatch translates multiple requests using the default translator.
func TranslateBatch(ctx context.Context, from, to Format, requests []BatchRequest) []BatchResult {
	return defaultBatchTranslator.TranslateBatch(ctx, from, to, requests)
}

// TranslateBatchWithCache translates multiple requests with caching.
func TranslateBatchWithCache(ctx context.Context, from, to Format, requests []BatchRequest, cache *TranslationCache) []BatchResult {
	bt := NewBatchTranslator(defaultRegistry, cache)
	bt.SetBatchWorkers(int(batchWorkerCount.Load()))
	return bt.TranslateBatch(ctx, from, to, requests)
}

// BatchProcessor provides a fluent API for batch translations.
type BatchProcessor struct {
	translator *BatchTranslator
	from       Format
	to         Format
	requests   []BatchRequest
	ctx        context.Context
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(from, to Format) *BatchProcessor {
	return &BatchProcessor{
		translator: defaultBatchTranslator,
		from:       from,
		to:         to,
		requests:   make([]BatchRequest, 0),
		ctx:        context.Background(),
	}
}

// WithContext sets the context for the batch operation.
func (bp *BatchProcessor) WithContext(ctx context.Context) *BatchProcessor {
	bp.ctx = ctx
	return bp
}

// WithRegistry sets a custom registry.
func (bp *BatchProcessor) WithRegistry(r *Registry) *BatchProcessor {
	bp.translator = NewBatchTranslator(r, nil)
	return bp
}

// WithCache sets a cache for the batch operation.
func (bp *BatchProcessor) WithCache(c *TranslationCache) *BatchProcessor {
	bp.translator = NewBatchTranslator(bp.translator.registry, c)
	return bp
}

// WithWorkers sets the number of workers.
func (bp *BatchProcessor) WithWorkers(n int) *BatchProcessor {
	bp.translator.SetBatchWorkers(n)
	return bp
}

// Add adds a request to the batch.
func (bp *BatchProcessor) Add(model string, payload []byte, stream bool) *BatchProcessor {
	bp.requests = append(bp.requests, BatchRequest{
		Format:  bp.from,
		Model:   model,
		Payload: payload,
		Stream:  stream,
	})
	return bp
}

// AddMany adds multiple payloads with the same model.
func (bp *BatchProcessor) AddMany(model string, payloads [][]byte, stream bool) *BatchProcessor {
	for _, p := range payloads {
		bp.Add(model, p, stream)
	}
	return bp
}

// Execute runs the batch translation.
func (bp *BatchProcessor) Execute() []BatchResult {
	return bp.translator.TranslateBatch(bp.ctx, bp.from, bp.to, bp.requests)
}

// ExecuteWithStats runs the batch and returns results with statistics.
func (bp *BatchProcessor) ExecuteWithStats() ([]BatchResult, BatchStats) {
	results := bp.Execute()
	stats := CalculateBatchStats(results)
	return results, stats
}
