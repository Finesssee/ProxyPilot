package translator

import (
	"sync"
)

// Middleware is a function that can transform payloads before or after translation.
type Middleware func(from, to Format, model string, payload []byte) []byte

// MiddlewareRegistry manages pre and post translation middleware.
type MiddlewareRegistry struct {
	mu             sync.RWMutex
	preMiddleware  []Middleware
	postMiddleware []Middleware
}

// NewMiddlewareRegistry creates a new MiddlewareRegistry.
func NewMiddlewareRegistry() *MiddlewareRegistry {
	return &MiddlewareRegistry{
		preMiddleware:  make([]Middleware, 0),
		postMiddleware: make([]Middleware, 0),
	}
}

// RegisterPre adds a middleware to be executed before translation.
// Middleware are executed in the order they are registered.
func (mr *MiddlewareRegistry) RegisterPre(mw Middleware) {
	if mw == nil {
		return
	}
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.preMiddleware = append(mr.preMiddleware, mw)
}

// RegisterPost adds a middleware to be executed after translation.
// Middleware are executed in the order they are registered.
func (mr *MiddlewareRegistry) RegisterPost(mw Middleware) {
	if mw == nil {
		return
	}
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.postMiddleware = append(mr.postMiddleware, mw)
}

// ApplyPre applies all pre-translation middleware in order.
func (mr *MiddlewareRegistry) ApplyPre(from, to Format, model string, payload []byte) []byte {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	result := payload
	for _, mw := range mr.preMiddleware {
		result = mw(from, to, model, result)
	}
	return result
}

// ApplyPost applies all post-translation middleware in order.
func (mr *MiddlewareRegistry) ApplyPost(from, to Format, model string, payload []byte) []byte {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	result := payload
	for _, mw := range mr.postMiddleware {
		result = mw(from, to, model, result)
	}
	return result
}

// ClearPre removes all pre-translation middleware.
func (mr *MiddlewareRegistry) ClearPre() {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.preMiddleware = make([]Middleware, 0)
}

// ClearPost removes all post-translation middleware.
func (mr *MiddlewareRegistry) ClearPost() {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.postMiddleware = make([]Middleware, 0)
}

// Clear removes all middleware.
func (mr *MiddlewareRegistry) Clear() {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.preMiddleware = make([]Middleware, 0)
	mr.postMiddleware = make([]Middleware, 0)
}

// Clone creates a deep copy of the MiddlewareRegistry.
func (mr *MiddlewareRegistry) Clone() *MiddlewareRegistry {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	newMR := NewMiddlewareRegistry()
	newMR.preMiddleware = make([]Middleware, len(mr.preMiddleware))
	copy(newMR.preMiddleware, mr.preMiddleware)
	newMR.postMiddleware = make([]Middleware, len(mr.postMiddleware))
	copy(newMR.postMiddleware, mr.postMiddleware)
	return newMR
}

// PreCount returns the number of registered pre-middleware.
func (mr *MiddlewareRegistry) PreCount() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return len(mr.preMiddleware)
}

// PostCount returns the number of registered post-middleware.
func (mr *MiddlewareRegistry) PostCount() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return len(mr.postMiddleware)
}

// defaultMiddlewareRegistry is the package-level middleware registry.
var defaultMiddlewareRegistry = NewMiddlewareRegistry()

// DefaultMiddlewareRegistry returns the package-level middleware registry.
func DefaultMiddlewareRegistry() *MiddlewareRegistry {
	return defaultMiddlewareRegistry
}

// RegisterPreMiddleware registers a pre-translation middleware in the default registry.
func RegisterPreMiddleware(mw Middleware) {
	defaultMiddlewareRegistry.RegisterPre(mw)
}

// RegisterPostMiddleware registers a post-translation middleware in the default registry.
func RegisterPostMiddleware(mw Middleware) {
	defaultMiddlewareRegistry.RegisterPost(mw)
}

// ApplyPreMiddleware applies all pre-translation middleware from the default registry.
func ApplyPreMiddleware(from, to Format, model string, payload []byte) []byte {
	return defaultMiddlewareRegistry.ApplyPre(from, to, model, payload)
}

// ApplyPostMiddleware applies all post-translation middleware from the default registry.
func ApplyPostMiddleware(from, to Format, model string, payload []byte) []byte {
	return defaultMiddlewareRegistry.ApplyPost(from, to, model, payload)
}

// ClearMiddleware clears all middleware from the default registry.
func ClearMiddleware() {
	defaultMiddlewareRegistry.Clear()
}

// TranslateRequestWithMiddleware translates a request with pre and post middleware applied.
func (r *Registry) TranslateRequestWithMiddleware(mr *MiddlewareRegistry, from, to Format, model string, rawJSON []byte, stream bool) []byte {
	if mr == nil {
		mr = defaultMiddlewareRegistry
	}

	// Apply pre-middleware
	payload := mr.ApplyPre(from, to, model, rawJSON)

	// Perform translation
	result := r.TranslateRequest(from, to, model, payload, stream)

	// Apply post-middleware
	return mr.ApplyPost(from, to, model, result)
}

// TranslateRequestWithMiddleware is a helper on the default registry.
func TranslateRequestWithMiddleware(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	return defaultRegistry.TranslateRequestWithMiddleware(defaultMiddlewareRegistry, from, to, model, rawJSON, stream)
}

// ConditionalMiddleware creates a middleware that only runs when a condition is met.
func ConditionalMiddleware(condition func(from, to Format, model string) bool, mw Middleware) Middleware {
	return func(from, to Format, model string, payload []byte) []byte {
		if condition(from, to, model) {
			return mw(from, to, model, payload)
		}
		return payload
	}
}

// ChainMiddleware chains multiple middleware into one.
func ChainMiddleware(middlewares ...Middleware) Middleware {
	return func(from, to Format, model string, payload []byte) []byte {
		result := payload
		for _, mw := range middlewares {
			if mw != nil {
				result = mw(from, to, model, result)
			}
		}
		return result
	}
}
