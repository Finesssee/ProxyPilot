package translator

import (
	"context"
	"sync"
)

// Registry manages translation functions across schemas.
type Registry struct {
	mu        sync.RWMutex
	requests  map[Format]map[Format]RequestTransform
	responses map[Format]map[Format]ResponseTransform

	// Lazy loading support
	lazyRequests  map[Format]map[Format]*lazyRequestEntry
	lazyResponses map[Format]map[Format]*lazyResponseEntry
}

type lazyRequestEntry struct {
	loader func() RequestTransform
	loaded bool
}

type lazyResponseEntry struct {
	loader func() ResponseTransform
	loaded bool
}

// LazyLoadStats contains statistics about lazy loaded translators.
type LazyLoadStats struct {
	TotalLazy   int
	TotalLoaded int
}

// NewRegistry constructs an empty translator registry.
func NewRegistry() *Registry {
	return &Registry{
		requests:      make(map[Format]map[Format]RequestTransform),
		responses:     make(map[Format]map[Format]ResponseTransform),
		lazyRequests:  make(map[Format]map[Format]*lazyRequestEntry),
		lazyResponses: make(map[Format]map[Format]*lazyResponseEntry),
	}
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

// TranslateRequest converts a payload between schemas, returning the original payload
// if no translator is registered. It also loads lazy transforms on-demand.
func (r *Registry) TranslateRequest(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	// First try with read lock
	r.mu.RLock()
	if byTarget, ok := r.requests[from]; ok {
		if fn, isOk := byTarget[to]; isOk && fn != nil {
			r.mu.RUnlock()
			return fn(model, rawJSON, stream)
		}
	}

	// Check if there's a lazy transform to load
	var needsLoad bool
	if byTarget, ok := r.lazyRequests[from]; ok {
		if entry, isOk := byTarget[to]; isOk && !entry.loaded {
			needsLoad = true
		}
	}
	r.mu.RUnlock()

	// Load lazy transform if needed
	if needsLoad {
		r.mu.Lock()
		// Double-check after acquiring write lock
		if byTarget, ok := r.lazyRequests[from]; ok {
			if entry, isOk := byTarget[to]; isOk && !entry.loaded && entry.loader != nil {
				fn := entry.loader()
				entry.loaded = true
				if fn != nil {
					if _, ok := r.requests[from]; !ok {
						r.requests[from] = make(map[Format]RequestTransform)
					}
					r.requests[from][to] = fn
				}
			}
		}
		r.mu.Unlock()

		// Try again with the loaded transform
		r.mu.RLock()
		if byTarget, ok := r.requests[from]; ok {
			if fn, isOk := byTarget[to]; isOk && fn != nil {
				r.mu.RUnlock()
				return fn(model, rawJSON, stream)
			}
		}
		r.mu.RUnlock()
	}

	return rawJSON
}

// HasResponseTransformer indicates whether a response translator (eager or lazy) exists.
func (r *Registry) HasResponseTransformer(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check eager transforms first
	if byTarget, ok := r.responses[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	// Check lazy transforms
	if byTarget, ok := r.lazyResponses[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	return false
}

// TranslateStream applies the registered streaming response translator.
// It also loads lazy response transforms on-demand.
func (r *Registry) TranslateStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	// First try with read lock
	r.mu.RLock()
	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.Stream != nil {
			r.mu.RUnlock()
			return fn.Stream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
		}
	}

	// Check if there's a lazy transform to load
	var needsLoad bool
	if byTarget, ok := r.lazyResponses[to]; ok {
		if entry, isOk := byTarget[from]; isOk && !entry.loaded {
			needsLoad = true
		}
	}
	r.mu.RUnlock()

	// Load lazy transform if needed
	if needsLoad {
		r.mu.Lock()
		// Double-check after acquiring write lock
		if byTarget, ok := r.lazyResponses[to]; ok {
			if entry, isOk := byTarget[from]; isOk && !entry.loaded && entry.loader != nil {
				fn := entry.loader()
				entry.loaded = true
				if _, ok := r.responses[to]; !ok {
					r.responses[to] = make(map[Format]ResponseTransform)
				}
				r.responses[to][from] = fn
			}
		}
		r.mu.Unlock()

		// Try again with the loaded transform
		r.mu.RLock()
		if byTarget, ok := r.responses[to]; ok {
			if fn, isOk := byTarget[from]; isOk && fn.Stream != nil {
				r.mu.RUnlock()
				return fn.Stream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
			}
		}
		r.mu.RUnlock()
	}

	return []string{string(rawJSON)}
}

// TranslateNonStream applies the registered non-stream response translator.
// It also loads lazy response transforms on-demand.
func (r *Registry) TranslateNonStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	// First try with read lock
	r.mu.RLock()
	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.NonStream != nil {
			r.mu.RUnlock()
			return fn.NonStream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
		}
	}

	// Check if there's a lazy transform to load
	var needsLoad bool
	if byTarget, ok := r.lazyResponses[to]; ok {
		if entry, isOk := byTarget[from]; isOk && !entry.loaded {
			needsLoad = true
		}
	}
	r.mu.RUnlock()

	// Load lazy transform if needed
	if needsLoad {
		r.mu.Lock()
		// Double-check after acquiring write lock
		if byTarget, ok := r.lazyResponses[to]; ok {
			if entry, isOk := byTarget[from]; isOk && !entry.loaded && entry.loader != nil {
				fn := entry.loader()
				entry.loaded = true
				if _, ok := r.responses[to]; !ok {
					r.responses[to] = make(map[Format]ResponseTransform)
				}
				r.responses[to][from] = fn
			}
		}
		r.mu.Unlock()

		// Try again with the loaded transform
		r.mu.RLock()
		if byTarget, ok := r.responses[to]; ok {
			if fn, isOk := byTarget[from]; isOk && fn.NonStream != nil {
				r.mu.RUnlock()
				return fn.NonStream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
			}
		}
		r.mu.RUnlock()
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

// RegisterLazy stores a lazy-loaded request transform between two formats.
func (r *Registry) RegisterLazy(from, to Format, loader func() RequestTransform) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.lazyRequests[from]; !ok {
		r.lazyRequests[from] = make(map[Format]*lazyRequestEntry)
	}
	r.lazyRequests[from][to] = &lazyRequestEntry{loader: loader}
}

// RegisterLazyResponse stores a lazy-loaded response transform between two formats.
func (r *Registry) RegisterLazyResponse(from, to Format, loader func() ResponseTransform) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.lazyResponses[from]; !ok {
		r.lazyResponses[from] = make(map[Format]*lazyResponseEntry)
	}
	r.lazyResponses[from][to] = &lazyResponseEntry{loader: loader}
}

// RegisterLazyBoth stores both request and response lazy-loaded transforms.
func (r *Registry) RegisterLazyBoth(from, to Format, reqLoader func() RequestTransform, respLoader func() ResponseTransform) {
	r.RegisterLazy(from, to, reqLoader)
	r.RegisterLazyResponse(from, to, respLoader)
}

// IsLazyLoaded checks if a lazy-loaded transform has been loaded.
func (r *Registry) IsLazyLoaded(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.lazyRequests[from]; ok {
		if entry, isOk := byTarget[to]; isOk {
			return entry.loaded
		}
	}
	return false
}

// HasRequestTranslator checks if a request translator (eager or lazy) exists.
func (r *Registry) HasRequestTranslator(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check eager transforms first
	if byTarget, ok := r.requests[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	// Check lazy transforms
	if byTarget, ok := r.lazyRequests[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	return false
}

// GetLazyLoadStats returns statistics about lazy loaded translators.
func (r *Registry) GetLazyLoadStats() LazyLoadStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := LazyLoadStats{}
	for _, byTarget := range r.lazyRequests {
		for _, entry := range byTarget {
			stats.TotalLazy++
			if entry.loaded {
				stats.TotalLoaded++
			}
		}
	}
	for _, byTarget := range r.lazyResponses {
		for _, entry := range byTarget {
			stats.TotalLazy++
			if entry.loaded {
				stats.TotalLoaded++
			}
		}
	}
	return stats
}

// ForceLoadLazy loads all lazy-loaded transforms immediately.
func (r *Registry) ForceLoadLazy() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for from, byTarget := range r.lazyRequests {
		for to, entry := range byTarget {
			if !entry.loaded && entry.loader != nil {
				fn := entry.loader()
				entry.loaded = true
				if fn != nil {
					if _, ok := r.requests[from]; !ok {
						r.requests[from] = make(map[Format]RequestTransform)
					}
					r.requests[from][to] = fn
				}
			}
		}
	}
	for from, byTarget := range r.lazyResponses {
		for to, entry := range byTarget {
			if !entry.loaded && entry.loader != nil {
				fn := entry.loader()
				entry.loaded = true
				if _, ok := r.responses[from]; !ok {
					r.responses[from] = make(map[Format]ResponseTransform)
				}
				r.responses[from][to] = fn
			}
		}
	}
}

// RegisterLazy is a helper on the default registry.
func RegisterLazy(from, to Format, loader func() RequestTransform) {
	defaultRegistry.RegisterLazy(from, to, loader)
}

// RegisterLazyResponse is a helper on the default registry.
func RegisterLazyResponse(from, to Format, loader func() ResponseTransform) {
	defaultRegistry.RegisterLazyResponse(from, to, loader)
}

// RegisterLazyBoth is a helper on the default registry.
func RegisterLazyBoth(from, to Format, reqLoader func() RequestTransform, respLoader func() ResponseTransform) {
	defaultRegistry.RegisterLazyBoth(from, to, reqLoader, respLoader)
}

// IsLazyLoaded is a helper on the default registry.
func IsLazyLoaded(from, to Format) bool {
	return defaultRegistry.IsLazyLoaded(from, to)
}

// HasRequestTranslator is a helper on the default registry.
func HasRequestTranslator(from, to Format) bool {
	return defaultRegistry.HasRequestTranslator(from, to)
}

// GetLazyLoadStats is a helper on the default registry.
func GetLazyLoadStats() LazyLoadStats {
	return defaultRegistry.GetLazyLoadStats()
}

// ForceLoadLazy is a helper on the default registry.
func ForceLoadLazy() {
	defaultRegistry.ForceLoadLazy()
}

// Unregister removes transforms for the given from->to direction from the default registry.
func Unregister(from, to Format) {
	defaultRegistry.Unregister(from, to)
}

// Unregister removes transforms for the given from->to direction from the registry.
func (r *Registry) Unregister(from, to Format) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if byTarget, ok := r.requests[from]; ok {
		delete(byTarget, to)
	}
	if byTarget, ok := r.responses[from]; ok {
		delete(byTarget, to)
	}
}
