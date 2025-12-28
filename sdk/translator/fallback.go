package translator

import (
	"sync"
)

// fallbackChain represents a chain of intermediate formats for translation.
type fallbackChain struct {
	via []Format
}

// FallbackRegistry manages fallback chains for translations.
type FallbackRegistry struct {
	mu     sync.RWMutex
	chains map[Format]map[Format]fallbackChain
}

// NewFallbackRegistry creates a new FallbackRegistry.
func NewFallbackRegistry() *FallbackRegistry {
	return &FallbackRegistry{
		chains: make(map[Format]map[Format]fallbackChain),
	}
}

// RegisterChain registers a fallback chain: from -> to via intermediate formats.
// For example, RegisterChain(A, B, []Format{X, Y}) means try A->X->Y->B.
func (fr *FallbackRegistry) RegisterChain(from, to Format, via []Format) {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if _, ok := fr.chains[from]; !ok {
		fr.chains[from] = make(map[Format]fallbackChain)
	}
	fr.chains[from][to] = fallbackChain{via: via}
}

// GetChain returns the registered fallback chain for from -> to.
// Returns nil if no chain is registered.
func (fr *FallbackRegistry) GetChain(from, to Format) []Format {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	if byTarget, ok := fr.chains[from]; ok {
		if chain, isOk := byTarget[to]; isOk {
			// Return a copy to prevent mutation
			result := make([]Format, len(chain.via))
			copy(result, chain.via)
			return result
		}
	}
	return nil
}

// UnregisterChain removes a fallback chain.
func (fr *FallbackRegistry) UnregisterChain(from, to Format) {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if byTarget, ok := fr.chains[from]; ok {
		delete(byTarget, to)
		if len(byTarget) == 0 {
			delete(fr.chains, from)
		}
	}
}

// Clone creates a deep copy of the FallbackRegistry.
func (fr *FallbackRegistry) Clone() *FallbackRegistry {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	newFR := NewFallbackRegistry()
	for from, targets := range fr.chains {
		newFR.chains[from] = make(map[Format]fallbackChain)
		for to, chain := range targets {
			viaCopy := make([]Format, len(chain.via))
			copy(viaCopy, chain.via)
			newFR.chains[from][to] = fallbackChain{via: viaCopy}
		}
	}
	return newFR
}

// defaultFallbackRegistry is the package-level fallback registry.
var defaultFallbackRegistry = NewFallbackRegistry()

// DefaultFallbackRegistry returns the package-level fallback registry.
func DefaultFallbackRegistry() *FallbackRegistry {
	return defaultFallbackRegistry
}

// RegisterFallbackChain registers a fallback chain in the default registry.
func RegisterFallbackChain(from, to Format, via []Format) {
	defaultFallbackRegistry.RegisterChain(from, to, via)
}

// GetFallbackChain returns the fallback chain from the default registry.
func GetFallbackChain(from, to Format) []Format {
	return defaultFallbackRegistry.GetChain(from, to)
}

// UnregisterFallbackChain removes a fallback chain from the default registry.
func UnregisterFallbackChain(from, to Format) {
	defaultFallbackRegistry.UnregisterChain(from, to)
}

// buildFullPath builds the complete translation path including via formats.
// For A->B via [X, Y], returns [A, X, Y, B].
func buildFullPath(from, to Format, via []Format) []Format {
	path := make([]Format, 0, len(via)+2)
	path = append(path, from)
	path = append(path, via...)
	path = append(path, to)
	return path
}

// TranslateRequestViaChain translates a request through a chain of formats.
func (r *Registry) TranslateRequestViaChain(path []Format, model string, rawJSON []byte, stream bool) []byte {
	if len(path) < 2 {
		return rawJSON
	}

	current := rawJSON
	for i := 0; i < len(path)-1; i++ {
		current = r.TranslateRequest(path[i], path[i+1], model, current, stream)
	}
	return current
}

// TranslateRequestViaChain is a helper on the default registry.
func TranslateRequestViaChain(path []Format, model string, rawJSON []byte, stream bool) []byte {
	return defaultRegistry.TranslateRequestViaChain(path, model, rawJSON, stream)
}
