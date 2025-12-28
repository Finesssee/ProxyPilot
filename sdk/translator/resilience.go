package translator

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Unregister removes a translator pair from the registry.
func (r *Registry) Unregister(from, to Format) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if byTarget, ok := r.requests[from]; ok {
		delete(byTarget, to)
		if len(byTarget) == 0 {
			delete(r.requests, from)
		}
	}

	if byTarget, ok := r.responses[from]; ok {
		delete(byTarget, to)
		if len(byTarget) == 0 {
			delete(r.responses, from)
		}
	}

	// Also remove lazy loaders
	if byTarget, ok := r.lazyRequests[from]; ok {
		delete(byTarget, to)
		if len(byTarget) == 0 {
			delete(r.lazyRequests, from)
		}
	}

	if byTarget, ok := r.lazyResponses[from]; ok {
		delete(byTarget, to)
		if len(byTarget) == 0 {
			delete(r.lazyResponses, from)
		}
	}
}

// Unregister removes a translator pair from the default registry.
func Unregister(from, to Format) {
	defaultRegistry.Unregister(from, to)
}

// Clone creates a deep copy of the Registry.
// Note: Transform functions are shared (not deep copied) as they are typically stateless.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	newReg := NewRegistry()

	// Copy requests
	for from, targets := range r.requests {
		newReg.requests[from] = make(map[Format]RequestTransform)
		for to, fn := range targets {
			newReg.requests[from][to] = fn
		}
	}

	// Copy responses
	for from, targets := range r.responses {
		newReg.responses[from] = make(map[Format]ResponseTransform)
		for to, fn := range targets {
			newReg.responses[from][to] = fn
		}
	}

	// Copy lazy requests
	for from, targets := range r.lazyRequests {
		newReg.lazyRequests[from] = make(map[Format]*lazyRequestEntry)
		for to, entry := range targets {
			entry.loadMu.Lock()
			newReg.lazyRequests[from][to] = &lazyRequestEntry{
				loader:   entry.loader,
				loaded:   entry.loaded,
				isLoaded: entry.isLoaded,
			}
			entry.loadMu.Unlock()
		}
	}

	// Copy lazy responses
	for from, targets := range r.lazyResponses {
		newReg.lazyResponses[from] = make(map[Format]*lazyResponseEntry)
		for to, entry := range targets {
			entry.loadMu.Lock()
			newReg.lazyResponses[from][to] = &lazyResponseEntry{
				loader:   entry.loader,
				loaded:   entry.loaded,
				isLoaded: entry.isLoaded,
			}
			entry.loadMu.Unlock()
		}
	}

	// Copy debug/dry-run settings
	newReg.debugMode.Store(r.debugMode.Load())
	newReg.dryRunMode.Store(r.dryRunMode.Load())
	newReg.loadedCount.Store(r.loadedCount.Load())
	newReg.lazyCount.Store(r.lazyCount.Load())

	return newReg
}

// CloneRegistry creates a deep copy of the default registry.
func CloneRegistry() *Registry {
	return defaultRegistry.Clone()
}

// registryPtr holds a pointer to the current default registry for atomic swap.
var registryPtr atomic.Pointer[Registry]

func init() {
	registryPtr.Store(defaultRegistry)
}

// ReplaceRegistry atomically swaps the default registry with a new one.
// Returns the old registry.
func ReplaceRegistry(newRegistry *Registry) *Registry {
	if newRegistry == nil {
		return nil
	}
	old := registryPtr.Swap(newRegistry)
	defaultRegistry = newRegistry
	return old
}

// HasRequestTransformer indicates whether a request translator exists.
// Alias for HasRequestTranslator for consistency with HasResponseTransformer.
func (r *Registry) HasRequestTransformer(from, to Format) bool {
	return r.HasRequestTranslator(from, to)
}

// HasRequestTransformer inspects the default registry.
func HasRequestTransformer(from, to Format) bool {
	return defaultRegistry.HasRequestTransformer(from, to)
}

// TranslationError provides detailed information about a failed translation attempt.
type TranslationError struct {
	From           Format
	To             Format
	AttemptedPaths [][]Format
	DetectedFormat Format
	Cause          error
}

func (e *TranslationError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("translation failed from %s to %s", e.From, e.To))
	if e.DetectedFormat != "" {
		sb.WriteString(fmt.Sprintf(" (detected: %s)", e.DetectedFormat))
	}
	if len(e.AttemptedPaths) > 0 {
		sb.WriteString("; attempted paths: ")
		for i, path := range e.AttemptedPaths {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("[")
			for j, f := range path {
				if j > 0 {
					sb.WriteString("->")
				}
				sb.WriteString(string(f))
			}
			sb.WriteString("]")
		}
	}
	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf("; cause: %v", e.Cause))
	}
	return sb.String()
}

func (e *TranslationError) Unwrap() error {
	return e.Cause
}

// TranslationResult contains the result of a translation with recovery.
type TranslationResult struct {
	Payload       []byte
	UsedPath      []Format
	Detected      Format
	Success       bool
	DirectHit     bool
	FallbackHit   bool
	AutoDetectHit bool
}

// TranslateRequestWithRecovery attempts translation with multiple recovery strategies:
// 1. Direct translation
// 2. Fallback chains
// 3. Auto-detect format and retry
func (r *Registry) TranslateRequestWithRecovery(
	from, to Format,
	model string,
	rawJSON []byte,
	stream bool,
	fallbackReg *FallbackRegistry,
) (*TranslationResult, error) {
	if fallbackReg == nil {
		fallbackReg = DefaultFallbackRegistry()
	}

	result := &TranslationResult{
		Payload: rawJSON,
	}
	var attemptedPaths [][]Format

	// Strategy 1: Try direct translation
	if r.HasRequestTransformer(from, to) {
		translated := r.TranslateRequest(from, to, model, rawJSON, stream)
		result.Payload = translated
		result.UsedPath = []Format{from, to}
		result.Success = true
		result.DirectHit = true
		return result, nil
	}
	attemptedPaths = append(attemptedPaths, []Format{from, to})

	// Strategy 2: Try fallback chains
	if chain := fallbackReg.GetChain(from, to); chain != nil {
		path := buildFullPath(from, to, chain)

		// Verify all steps in the chain exist
		chainValid := true
		for i := 0; i < len(path)-1; i++ {
			if !r.HasRequestTransformer(path[i], path[i+1]) {
				chainValid = false
				break
			}
		}

		if chainValid {
			translated := r.TranslateRequestViaChain(path, model, rawJSON, stream)
			result.Payload = translated
			result.UsedPath = path
			result.Success = true
			result.FallbackHit = true
			return result, nil
		}
		attemptedPaths = append(attemptedPaths, path)
	}

	// Strategy 3: Auto-detect format and retry
	detected := DetectFormat(rawJSON)
	result.Detected = detected

	if detected != "" && detected != from {
		// Try direct translation from detected format
		if r.HasRequestTransformer(detected, to) {
			translated := r.TranslateRequest(detected, to, model, rawJSON, stream)
			result.Payload = translated
			result.UsedPath = []Format{detected, to}
			result.Success = true
			result.AutoDetectHit = true
			return result, nil
		}
		attemptedPaths = append(attemptedPaths, []Format{detected, to})

		// Try fallback chain from detected format
		if chain := fallbackReg.GetChain(detected, to); chain != nil {
			path := buildFullPath(detected, to, chain)

			chainValid := true
			for i := 0; i < len(path)-1; i++ {
				if !r.HasRequestTransformer(path[i], path[i+1]) {
					chainValid = false
					break
				}
			}

			if chainValid {
				translated := r.TranslateRequestViaChain(path, model, rawJSON, stream)
				result.Payload = translated
				result.UsedPath = path
				result.Success = true
				result.FallbackHit = true
				result.AutoDetectHit = true
				return result, nil
			}
			attemptedPaths = append(attemptedPaths, path)
		}
	}

	// All strategies failed
	return result, &TranslationError{
		From:           from,
		To:             to,
		AttemptedPaths: attemptedPaths,
		DetectedFormat: detected,
		Cause:          ErrNoTranslator,
	}
}

// TranslateRequestWithRecovery is a helper on the default registry.
func TranslateRequestWithRecovery(from, to Format, model string, rawJSON []byte, stream bool) (*TranslationResult, error) {
	return defaultRegistry.TranslateRequestWithRecovery(from, to, model, rawJSON, stream, nil)
}

// ListRegisteredPairs returns all registered (from, to) pairs for requests.
func (r *Registry) ListRegisteredPairs() [][2]Format {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var pairs [][2]Format
	for from, targets := range r.requests {
		for to := range targets {
			pairs = append(pairs, [2]Format{from, to})
		}
	}
	// Include lazy registered pairs
	for from, targets := range r.lazyRequests {
		for to := range targets {
			pairs = append(pairs, [2]Format{from, to})
		}
	}
	return pairs
}

// ListRegisteredPairs returns all registered pairs from the default registry.
func ListRegisteredPairs() [][2]Format {
	return defaultRegistry.ListRegisteredPairs()
}

// Clear removes all registered translators from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests = make(map[Format]map[Format]RequestTransform)
	r.responses = make(map[Format]map[Format]ResponseTransform)
	r.lazyRequests = make(map[Format]map[Format]*lazyRequestEntry)
	r.lazyResponses = make(map[Format]map[Format]*lazyResponseEntry)
}

// Clear removes all registered translators from the default registry.
func Clear() {
	defaultRegistry.Clear()
}
