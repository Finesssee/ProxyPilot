package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

// translationStats holds atomic counters for translation statistics.
type translationStats struct {
	totalCount  atomic.Int64
	failedCount atomic.Int64
}

// performanceMetrics holds timing data for translation performance.
type performanceMetrics struct {
	mu           sync.RWMutex
	totalTime    time.Duration
	requestCount int64
}

// LazyRequestLoader is a function that returns a RequestTransform when called.
// Used for expensive translator initialization that should be deferred.
type LazyRequestLoader func() RequestTransform

// LazyResponseLoader is a function that returns a ResponseTransform when called.
// Used for expensive translator initialization that should be deferred.
type LazyResponseLoader func() ResponseTransform

// lazyRequestEntry holds a loader function and tracks loading state.
type lazyRequestEntry struct {
	loader   LazyRequestLoader
	loaded   RequestTransform
	isLoaded bool
	loadMu   sync.Mutex
}

// lazyResponseEntry holds a loader function and tracks loading state.
type lazyResponseEntry struct {
	loader   LazyResponseLoader
	loaded   ResponseTransform
	isLoaded bool
	loadMu   sync.Mutex
}

// Registry manages translation functions across schemas.
type Registry struct {
	mu        sync.RWMutex
	requests  map[Format]map[Format]RequestTransform
	responses map[Format]map[Format]ResponseTransform

	// Lazy loading support
	lazyRequests  map[Format]map[Format]*lazyRequestEntry
	lazyResponses map[Format]map[Format]*lazyResponseEntry
	loadedCount   atomic.Int64
	lazyCount     atomic.Int64

	// Observability features
	debugMode   atomic.Bool
	dryRunMode  atomic.Bool
	statsMu     sync.RWMutex
	stats       map[string]*translationStats // key: "from->to"
	perfMu      sync.RWMutex
	perfMetrics map[string]*performanceMetrics // key: "from->to"
}

// NewRegistry constructs an empty translator registry.
func NewRegistry() *Registry {
	return &Registry{
		requests:      make(map[Format]map[Format]RequestTransform),
		responses:    make(map[Format]map[Format]ResponseTransform),
		lazyRequests:  make(map[Format]map[Format]*lazyRequestEntry),
		lazyResponses: make(map[Format]map[Format]*lazyResponseEntry),
		stats:        make(map[string]*translationStats),
		perfMetrics:  make(map[string]*performanceMetrics),
	}
}

// pairKey generates a consistent key for a (from, to) translation pair.
func pairKey(from, to Format) string {
	return fmt.Sprintf("%s->%s", from, to)
}

// truncatePayload truncates a byte slice to the specified max length for logging.
func truncatePayload(data []byte, maxLen int) string {
	if len(data) <= maxLen {
		return string(data)
	}
	return string(data[:maxLen]) + "...[truncated]"
}

// SetDebugMode enables or disables debug mode for the registry.
// When enabled, translation methods will log before/after payloads (truncated to 500 chars).
func (r *Registry) SetDebugMode(enabled bool) {
	r.debugMode.Store(enabled)
	if enabled {
		log.Info("translator: debug mode enabled - payloads will be logged")
	} else {
		log.Info("translator: debug mode disabled")
	}
}

// IsDebugMode returns the current debug mode state.
func (r *Registry) IsDebugMode() bool {
	return r.debugMode.Load()
}

// LogRegisteredTranslators logs all registered translators at INFO level with a count summary,
// and DEBUG level details for each registered pair.
func (r *Registry) LogRegisteredTranslators() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	requestCount := 0
	responseCount := 0
	var requestPairs []string
	var responsePairs []string

	// Count and collect request translators
	for from, targets := range r.requests {
		for to := range targets {
			requestCount++
			requestPairs = append(requestPairs, pairKey(from, to))
		}
	}

	// Count and collect response translators
	for from, targets := range r.responses {
		for to := range targets {
			responseCount++
			responsePairs = append(responsePairs, pairKey(from, to))
		}
	}

	// Sort for consistent output
	sort.Strings(requestPairs)
	sort.Strings(responsePairs)

	log.Infof("translator: registered %d request translator(s) and %d response translator(s)",
		requestCount, responseCount)

	// Log details at DEBUG level
	for _, pair := range requestPairs {
		log.Debugf("translator: request translator registered: %s", pair)
	}
	for _, pair := range responsePairs {
		log.Debugf("translator: response translator registered: %s", pair)
	}
}

// getOrCreateStats returns the stats for a pair, creating if necessary.
func (r *Registry) getOrCreateStats(key string) *translationStats {
	r.statsMu.RLock()
	s, ok := r.stats[key]
	r.statsMu.RUnlock()
	if ok {
		return s
	}

	r.statsMu.Lock()
	defer r.statsMu.Unlock()
	// Double-check after acquiring write lock
	if s, ok = r.stats[key]; ok {
		return s
	}
	s = &translationStats{}
	r.stats[key] = s
	return s
}

// getOrCreatePerfMetrics returns the performance metrics for a pair, creating if necessary.
func (r *Registry) getOrCreatePerfMetrics(key string) *performanceMetrics {
	r.perfMu.RLock()
	p, ok := r.perfMetrics[key]
	r.perfMu.RUnlock()
	if ok {
		return p
	}

	r.perfMu.Lock()
	defer r.perfMu.Unlock()
	// Double-check after acquiring write lock
	if p, ok = r.perfMetrics[key]; ok {
		return p
	}
	p = &performanceMetrics{}
	r.perfMetrics[key] = p
	return p
}

// recordTranslation records a successful translation.
func (r *Registry) recordTranslation(from, to Format, duration time.Duration) {
	key := pairKey(from, to)
	stats := r.getOrCreateStats(key)
	stats.totalCount.Add(1)

	perf := r.getOrCreatePerfMetrics(key)
	perf.mu.Lock()
	perf.totalTime += duration
	perf.requestCount++
	perf.mu.Unlock()
}

// recordFailedTranslation records a failed translation (no translator found).
func (r *Registry) recordFailedTranslation(from, to Format) {
	key := pairKey(from, to)
	stats := r.getOrCreateStats(key)
	stats.failedCount.Add(1)
}

// GetTranslationStats returns a map of translation statistics.
// Keys are in the format "from->to:total" and "from->to:failed".
func (r *Registry) GetTranslationStats() map[string]int64 {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()

	result := make(map[string]int64)
	for key, s := range r.stats {
		result[key+":total"] = s.totalCount.Load()
		result[key+":failed"] = s.failedCount.Load()
	}
	return result
}

// GetPerformanceMetrics returns the average translation latency per (from, to) pair.
func (r *Registry) GetPerformanceMetrics() map[string]time.Duration {
	r.perfMu.RLock()
	defer r.perfMu.RUnlock()

	result := make(map[string]time.Duration)
	for key, p := range r.perfMetrics {
		p.mu.RLock()
		if p.requestCount > 0 {
			result[key] = p.totalTime / time.Duration(p.requestCount)
		} else {
			result[key] = 0
		}
		p.mu.RUnlock()
	}
	return result
}

// DiffPayloads compares two JSON payloads and returns a human-readable diff summary
// showing fields added, removed, or changed.
func DiffPayloads(before, after []byte) string {
	var beforeMap, afterMap map[string]interface{}

	if err := json.Unmarshal(before, &beforeMap); err != nil {
		return fmt.Sprintf("error parsing 'before' payload: %v", err)
	}
	if err := json.Unmarshal(after, &afterMap); err != nil {
		return fmt.Sprintf("error parsing 'after' payload: %v", err)
	}

	var added, removed, changed []string

	// Find removed and changed fields
	for key, beforeVal := range beforeMap {
		afterVal, exists := afterMap[key]
		if !exists {
			removed = append(removed, key)
		} else if !jsonValuesEqual(beforeVal, afterVal) {
			changed = append(changed, key)
		}
	}

	// Find added fields
	for key := range afterMap {
		if _, exists := beforeMap[key]; !exists {
			added = append(added, key)
		}
	}

	// Sort for consistent output
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)

	var parts []string
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("added: [%s]", strings.Join(added, ", ")))
	}
	if len(removed) > 0 {
		parts = append(parts, fmt.Sprintf("removed: [%s]", strings.Join(removed, ", ")))
	}
	if len(changed) > 0 {
		parts = append(parts, fmt.Sprintf("changed: [%s]", strings.Join(changed, ", ")))
	}

	if len(parts) == 0 {
		return "no changes detected"
	}
	return strings.Join(parts, "; ")
}

// jsonValuesEqual compares two JSON values for equality.
func jsonValuesEqual(a, b interface{}) bool {
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// Register stores request/response transforms between two formats.
func (r *Registry) Register(from, to Format, request RequestTransform, response ResponseTransform) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.requests[from]; !ok {
		r.requests[from] = make(map[Format]RequestTransform)
	}
	if request != nil {
		r.requests[from][to] = request
	}

	if _, ok := r.responses[from]; !ok {
		r.responses[from] = make(map[Format]ResponseTransform)
	}
	r.responses[from][to] = response
}

// RegisterLazy registers a lazy-loaded request transformer.
// The loader function is called on first use, which is useful for expensive initialization.
func (r *Registry) RegisterLazy(from, to Format, loader LazyRequestLoader) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.lazyRequests[from]; !ok {
		r.lazyRequests[from] = make(map[Format]*lazyRequestEntry)
	}
	r.lazyRequests[from][to] = &lazyRequestEntry{
		loader: loader,
	}
	r.lazyCount.Add(1)
}

// RegisterLazyResponse registers a lazy-loaded response transformer.
// The loader function is called on first use.
func (r *Registry) RegisterLazyResponse(from, to Format, loader LazyResponseLoader) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.lazyResponses[from]; !ok {
		r.lazyResponses[from] = make(map[Format]*lazyResponseEntry)
	}
	r.lazyResponses[from][to] = &lazyResponseEntry{
		loader: loader,
	}
	r.lazyCount.Add(1)
}

// RegisterLazyBoth registers both request and response transformers as lazy-loaded.
func (r *Registry) RegisterLazyBoth(from, to Format, reqLoader LazyRequestLoader, respLoader LazyResponseLoader) {
	r.RegisterLazy(from, to, reqLoader)
	r.RegisterLazyResponse(from, to, respLoader)
}

// loadLazyRequest loads a lazy request transformer if not already loaded.
func (r *Registry) loadLazyRequest(from, to Format) RequestTransform {
	r.mu.RLock()
	entry, exists := r.lazyRequests[from][to]
	r.mu.RUnlock()

	if !exists || entry == nil {
		return nil
	}

	entry.loadMu.Lock()
	defer entry.loadMu.Unlock()

	if entry.isLoaded {
		return entry.loaded
	}

	if entry.loader != nil {
		entry.loaded = entry.loader()
		entry.isLoaded = true
		r.loadedCount.Add(1)
		log.Debugf("translator: lazy-loaded request transformer for %s -> %s", from, to)
	}

	return entry.loaded
}

// loadLazyResponse loads a lazy response transformer if not already loaded.
func (r *Registry) loadLazyResponse(from, to Format) *ResponseTransform {
	r.mu.RLock()
	entry, exists := r.lazyResponses[from][to]
	r.mu.RUnlock()

	if !exists || entry == nil {
		return nil
	}

	entry.loadMu.Lock()
	defer entry.loadMu.Unlock()

	if entry.isLoaded {
		return &entry.loaded
	}

	if entry.loader != nil {
		entry.loaded = entry.loader()
		entry.isLoaded = true
		r.loadedCount.Add(1)
		log.Debugf("translator: lazy-loaded response transformer for %s -> %s", from, to)
	}

	return &entry.loaded
}

// GetLazyLoadStats returns statistics about lazy loading.
func (r *Registry) GetLazyLoadStats() LazyLoadStats {
	return LazyLoadStats{
		TotalLazy:   r.lazyCount.Load(),
		TotalLoaded: r.loadedCount.Load(),
	}
}

// LazyLoadStats contains statistics about lazy loading.
type LazyLoadStats struct {
	TotalLazy   int64 `json:"total_lazy"`
	TotalLoaded int64 `json:"total_loaded"`
}

// IsLazyLoaded checks if a lazy transformer has been loaded.
func (r *Registry) IsLazyLoaded(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.lazyRequests[from]; ok {
		if entry, exists := byTarget[to]; exists {
			entry.loadMu.Lock()
			defer entry.loadMu.Unlock()
			return entry.isLoaded
		}
	}
	return false
}

// ForceLoadLazy forces loading of all lazy transformers.
func (r *Registry) ForceLoadLazy() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for from, targets := range r.lazyRequests {
		for to := range targets {
			r.loadLazyRequest(from, to)
		}
	}
	for from, targets := range r.lazyResponses {
		for to := range targets {
			r.loadLazyResponse(from, to)
		}
	}
}

// TranslateRequest converts a payload between schemas, returning the original payload
// if no translator is registered.
func (r *Registry) TranslateRequest(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	start := time.Now()
	debugMode := r.debugMode.Load()

	if debugMode {
		log.Debugf("translator: TranslateRequest [%s->%s] BEFORE (model: %s): %s",
			from, to, model, truncatePayload(rawJSON, 500))
	}

	r.mu.RLock()
	var fn RequestTransform
	var found bool
	if byTarget, ok := r.requests[from]; ok {
		if f, isOk := byTarget[to]; isOk && f != nil {
			fn = f
			found = true
		}
	}
	r.mu.RUnlock()

	// Check lazy loaders if not found in regular registry
	if !found {
		if lazyFn := r.loadLazyRequest(from, to); lazyFn != nil {
			fn = lazyFn
			found = true
		}
	}

	if found {
		result := fn(model, rawJSON, stream)
		r.recordTranslation(from, to, time.Since(start))

		if debugMode {
			log.Debugf("translator: TranslateRequest [%s->%s] AFTER (model: %s): %s",
				from, to, model, truncatePayload(result, 500))
			log.Debugf("translator: TranslateRequest [%s->%s] DIFF: %s",
				from, to, DiffPayloads(rawJSON, result))
		}
		return result
	}

	// Warn if translation was expected but not found (from != to means translation should happen)
	if from != to && from != "" && to != "" {
		log.Warnf("translator: no request translator registered for %s -> %s (model: %s), passing through unchanged", from, to, model)
		r.recordFailedTranslation(from, to)
	}

	return rawJSON
}

// HasResponseTransformer indicates whether a response translator exists.
func (r *Registry) HasResponseTransformer(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.responses[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	// Check lazy responses
	if byTarget, ok := r.lazyResponses[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	return false
}

// HasRequestTranslator indicates whether a request translator exists for the given format pair.
func (r *Registry) HasRequestTranslator(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.requests[from]; ok {
		if fn, isOk := byTarget[to]; isOk && fn != nil {
			return true
		}
	}
	// Check lazy requests
	if byTarget, ok := r.lazyRequests[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	return false
}

// MustTranslateRequest converts a payload between schemas, returning an error if no translator is registered.
// Unlike TranslateRequest, this method fails fast instead of returning the original payload.
func (r *Registry) MustTranslateRequest(from, to Format, model string, rawJSON []byte, stream bool) ([]byte, error) {
	// If formats are the same, no translation needed
	if from == to {
		return rawJSON, nil
	}

	// Check dry-run mode first
	if r.dryRunMode.Load() {
		log.Infof("translator [DRY-RUN]: would translate request from %s to %s (model: %s, stream: %v, payload size: %d bytes)",
			from, to, model, stream, len(rawJSON))
		if r.debugMode.Load() {
			log.Debugf("translator [DRY-RUN]: payload preview: %s", truncatePayload(rawJSON, 500))
		}
		return rawJSON, nil
	}

	start := time.Now()
	debugMode := r.debugMode.Load()

	if debugMode {
		log.Debugf("translator: MustTranslateRequest [%s->%s] BEFORE (model: %s): %s",
			from, to, model, truncatePayload(rawJSON, 500))
	}

	r.mu.RLock()
	var fn RequestTransform
	var found bool
	if byTarget, ok := r.requests[from]; ok {
		if f, isOk := byTarget[to]; isOk && f != nil {
			fn = f
			found = true
		}
	}
	r.mu.RUnlock()

	if found {
		result := fn(model, rawJSON, stream)
		r.recordTranslation(from, to, time.Since(start))

		if debugMode {
			log.Debugf("translator: MustTranslateRequest [%s->%s] AFTER (model: %s): %s",
				from, to, model, truncatePayload(result, 500))
			log.Debugf("translator: MustTranslateRequest [%s->%s] DIFF: %s",
				from, to, DiffPayloads(rawJSON, result))
		}

		return result, nil
	}

	r.recordFailedTranslation(from, to)
	return nil, fmt.Errorf("%w: no request translator for %s -> %s", ErrNoTranslator, from, to)
}

// ValidateTranslationPath checks if a complete translation path exists between two formats.
// It verifies both request and response translators are registered.
func (r *Registry) ValidateTranslationPath(from, to Format) error {
	if from == to {
		return nil // Same format, no translation needed
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var errors []string

	// Check request translator
	hasRequest := false
	if byTarget, ok := r.requests[from]; ok {
		if fn, isOk := byTarget[to]; isOk && fn != nil {
			hasRequest = true
		}
	}
	if !hasRequest {
		errors = append(errors, fmt.Sprintf("no request translator for %s -> %s", from, to))
	}

	// Check response translator (note: responses use reversed lookup direction)
	hasResponse := false
	if byTarget, ok := r.responses[to]; ok {
		if _, isOk := byTarget[from]; isOk {
			hasResponse = true
		}
	}
	if !hasResponse {
		errors = append(errors, fmt.Sprintf("no response translator for %s -> %s", from, to))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%w: %s", ErrNoTranslator, strings.Join(errors, "; "))
	}

	return nil
}

// SetDryRunMode enables or disables dry-run mode for the registry.
// When enabled, translation methods will log what WOULD be translated but return the original payload.
func (r *Registry) SetDryRunMode(enabled bool) {
	r.dryRunMode.Store(enabled)
	if enabled {
		log.Info("translator: dry-run mode enabled - translations will be logged but not applied")
	} else {
		log.Info("translator: dry-run mode disabled")
	}
}

// IsDryRunMode returns the current dry-run mode state.
func (r *Registry) IsDryRunMode() bool {
	return r.dryRunMode.Load()
}

// TranslateStream applies the registered streaming response translator.
func (r *Registry) TranslateStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	start := time.Now()
	debugMode := r.debugMode.Load()

	if debugMode {
		log.Debugf("translator: TranslateStream [%s->%s] BEFORE (model: %s): %s",
			from, to, model, truncatePayload(rawJSON, 500))
	}

	r.mu.RLock()
	var streamFn func(context.Context, string, []byte, []byte, []byte, *any) []string
	var found bool
	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.Stream != nil {
			streamFn = fn.Stream
			found = true
		}
	}
	r.mu.RUnlock()

	if found {
		result := streamFn(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
		r.recordTranslation(from, to, time.Since(start))

		if debugMode {
			// For stream, log first chunk only to avoid excessive logging
			if len(result) > 0 {
				log.Debugf("translator: TranslateStream [%s->%s] AFTER (model: %s, %d chunks): first chunk: %s",
					from, to, model, len(result), truncatePayload([]byte(result[0]), 500))
			}
		}
		return result
	}

	// Warn if translation was expected but not found
	if from != to && from != "" && to != "" {
		log.Warnf("translator: no stream response translator registered for %s -> %s (model: %s), passing through unchanged", from, to, model)
		r.recordFailedTranslation(from, to)
	}

	return []string{string(rawJSON)}
}

// TranslateNonStream applies the registered non-stream response translator.
func (r *Registry) TranslateNonStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	start := time.Now()
	debugMode := r.debugMode.Load()

	if debugMode {
		log.Debugf("translator: TranslateNonStream [%s->%s] BEFORE (model: %s): %s",
			from, to, model, truncatePayload(rawJSON, 500))
	}

	r.mu.RLock()
	var nonStreamFn func(context.Context, string, []byte, []byte, []byte, *any) string
	var found bool
	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.NonStream != nil {
			nonStreamFn = fn.NonStream
			found = true
		}
	}
	r.mu.RUnlock()

	if found {
		result := nonStreamFn(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
		r.recordTranslation(from, to, time.Since(start))

		if debugMode {
			log.Debugf("translator: TranslateNonStream [%s->%s] AFTER (model: %s): %s",
				from, to, model, truncatePayload([]byte(result), 500))
			log.Debugf("translator: TranslateNonStream [%s->%s] DIFF: %s",
				from, to, DiffPayloads(rawJSON, []byte(result)))
		}
		return result
	}

	// Warn if translation was expected but not found
	if from != to && from != "" && to != "" {
		log.Warnf("translator: no non-stream response translator registered for %s -> %s (model: %s), passing through unchanged", from, to, model)
		r.recordFailedTranslation(from, to)
	}

	return string(rawJSON)
}

// TranslateNonStream applies the registered non-stream response translator.
func (r *Registry) TranslateTokenCount(ctx context.Context, from, to Format, count int64, rawJSON []byte) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.TokenCount != nil {
			return fn.TokenCount(ctx, count)
		}
	}
	return string(rawJSON)
}

// ErrNoTranslator is returned when no translator is registered for a path.
var ErrNoTranslator = fmt.Errorf("no translator registered")

// ErrTranslationFailed is returned when translation fails.
var ErrTranslationFailed = fmt.Errorf("translation failed")

var defaultRegistry = NewRegistry()

// Default exposes the package-level registry for shared use.
func Default() *Registry {
	return defaultRegistry
}

// Register attaches transforms to the default registry.
func Register(from, to Format, request RequestTransform, response ResponseTransform) {
	defaultRegistry.Register(from, to, request, response)
}

// TranslateRequest is a helper on the default registry.
func TranslateRequest(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	return defaultRegistry.TranslateRequest(from, to, model, rawJSON, stream)
}

// HasResponseTransformer inspects the default registry.
func HasResponseTransformer(from, to Format) bool {
	return defaultRegistry.HasResponseTransformer(from, to)
}

// TranslateStream is a helper on the default registry.
func TranslateStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return defaultRegistry.TranslateStream(ctx, from, to, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// TranslateNonStream is a helper on the default registry.
func TranslateNonStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return defaultRegistry.TranslateNonStream(ctx, from, to, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// TranslateTokenCount is a helper on the default registry.
func TranslateTokenCount(ctx context.Context, from, to Format, count int64, rawJSON []byte) string {
	return defaultRegistry.TranslateTokenCount(ctx, from, to, count, rawJSON)
}

// SetDebugMode enables or disables debug mode on the default registry.
func SetDebugMode(enabled bool) {
	defaultRegistry.SetDebugMode(enabled)
}

// IsDebugMode returns the current debug mode state of the default registry.
func IsDebugMode() bool {
	return defaultRegistry.IsDebugMode()
}

// LogRegisteredTranslators logs all registered translators in the default registry.
func LogRegisteredTranslators() {
	defaultRegistry.LogRegisteredTranslators()
}

// GetTranslationStats returns translation statistics from the default registry.
func GetTranslationStats() map[string]int64 {
	return defaultRegistry.GetTranslationStats()
}

// GetPerformanceMetrics returns performance metrics from the default registry.
func GetPerformanceMetrics() map[string]time.Duration {
	return defaultRegistry.GetPerformanceMetrics()
}

// HasRequestTranslator inspects the default registry for request translators.
func HasRequestTranslator(from, to Format) bool {
	return defaultRegistry.HasRequestTranslator(from, to)
}

// MustTranslateRequest is a helper on the default registry that fails fast.
func MustTranslateRequest(from, to Format, model string, rawJSON []byte, stream bool) ([]byte, error) {
	return defaultRegistry.MustTranslateRequest(from, to, model, rawJSON, stream)
}

// ValidateTranslationPath checks if a translation path exists in the default registry.
func ValidateTranslationPath(from, to Format) error {
	return defaultRegistry.ValidateTranslationPath(from, to)
}

// SetDryRunMode enables or disables dry-run mode on the default registry.
func SetDryRunMode(enabled bool) {
	defaultRegistry.SetDryRunMode(enabled)
}

// IsDryRunMode returns the current dry-run mode state of the default registry.
func IsDryRunMode() bool {
	return defaultRegistry.IsDryRunMode()
}

// RegisterLazy registers a lazy-loaded request transformer on the default registry.
func RegisterLazy(from, to Format, loader LazyRequestLoader) {
	defaultRegistry.RegisterLazy(from, to, loader)
}

// RegisterLazyResponse registers a lazy-loaded response transformer on the default registry.
func RegisterLazyResponse(from, to Format, loader LazyResponseLoader) {
	defaultRegistry.RegisterLazyResponse(from, to, loader)
}

// RegisterLazyBoth registers both transformers as lazy-loaded on the default registry.
func RegisterLazyBoth(from, to Format, reqLoader LazyRequestLoader, respLoader LazyResponseLoader) {
	defaultRegistry.RegisterLazyBoth(from, to, reqLoader, respLoader)
}

// GetLazyLoadStats returns lazy loading statistics from the default registry.
func GetLazyLoadStats() LazyLoadStats {
	return defaultRegistry.GetLazyLoadStats()
}

// IsLazyLoaded checks if a lazy transformer has been loaded in the default registry.
func IsLazyLoaded(from, to Format) bool {
	return defaultRegistry.IsLazyLoaded(from, to)
}

// ForceLoadLazy forces loading of all lazy transformers in the default registry.
func ForceLoadLazy() {
	defaultRegistry.ForceLoadLazy()
}
