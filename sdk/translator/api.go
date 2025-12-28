package translator

import (
	"sort"
)

// GetCompatibilityMatrix returns a map of source formats to their supported target formats.
// The keys are source format strings, and values are slices of target format strings.
func (r *Registry) GetCompatibilityMatrix() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matrix := make(map[string][]string)

	// Collect all unique from -> to pairs from requests registry
	for from, targets := range r.requests {
		if _, exists := matrix[from.String()]; !exists {
			matrix[from.String()] = []string{}
		}
		for to := range targets {
			matrix[from.String()] = append(matrix[from.String()], to.String())
		}
	}

	// Also include response translators that may not have request translators
	for from, targets := range r.responses {
		if _, exists := matrix[from.String()]; !exists {
			matrix[from.String()] = []string{}
		}
		for to := range targets {
			// Check if already added from requests
			found := false
			for _, existing := range matrix[from.String()] {
				if existing == to.String() {
					found = true
					break
				}
			}
			if !found {
				matrix[from.String()] = append(matrix[from.String()], to.String())
			}
		}
	}

	// Sort targets for consistent output
	for from := range matrix {
		sort.Strings(matrix[from])
	}

	return matrix
}

// GetSupportedFormats returns all known formats that have been registered
// as either a source or target for translations.
func (r *Registry) GetSupportedFormats() []Format {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formatSet := make(map[Format]struct{})

	// Collect formats from request translators
	for from, targets := range r.requests {
		formatSet[from] = struct{}{}
		for to := range targets {
			formatSet[to] = struct{}{}
		}
	}

	// Collect formats from response translators
	for from, targets := range r.responses {
		formatSet[from] = struct{}{}
		for to := range targets {
			formatSet[to] = struct{}{}
		}
	}

	// Convert set to slice
	formats := make([]Format, 0, len(formatSet))
	for f := range formatSet {
		formats = append(formats, f)
	}

	// Sort for consistent output
	sort.Slice(formats, func(i, j int) bool {
		return formats[i].String() < formats[j].String()
	})

	return formats
}

// IsTranslationSupported checks if a translation path exists between two formats.
// It returns true if either a request or response translator is registered for the pair.
func (r *Registry) IsTranslationSupported(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check request translators
	if byTarget, ok := r.requests[from]; ok {
		if _, exists := byTarget[to]; exists {
			return true
		}
	}

	// Check response translators
	if byTarget, ok := r.responses[from]; ok {
		if _, exists := byTarget[to]; exists {
			return true
		}
	}

	return false
}

// Package-level helper functions that use the default registry

// GetCompatibilityMatrix returns the compatibility matrix from the default registry.
func GetCompatibilityMatrix() map[string][]string {
	return defaultRegistry.GetCompatibilityMatrix()
}

// GetSupportedFormats returns all supported formats from the default registry.
func GetSupportedFormats() []Format {
	return defaultRegistry.GetSupportedFormats()
}

// IsTranslationSupported checks if a translation is supported in the default registry.
func IsTranslationSupported(from, to Format) bool {
	return defaultRegistry.IsTranslationSupported(from, to)
}

// TranslationInfo contains metadata about a translation path.
type TranslationInfo struct {
	From           Format `json:"from"`
	To             Format `json:"to"`
	HasRequest     bool   `json:"has_request"`
	HasResponse    bool   `json:"has_response"`
	HasStream      bool   `json:"has_stream"`
	HasNonStream   bool   `json:"has_non_stream"`
	HasTokenCount  bool   `json:"has_token_count"`
}

// GetTranslationInfo returns detailed information about a specific translation path.
func (r *Registry) GetTranslationInfo(from, to Format) *TranslationInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := &TranslationInfo{
		From: from,
		To:   to,
	}

	// Check request translator
	if byTarget, ok := r.requests[from]; ok {
		if _, exists := byTarget[to]; exists {
			info.HasRequest = true
		}
	}

	// Check response translators
	if byTarget, ok := r.responses[from]; ok {
		if resp, exists := byTarget[to]; exists {
			info.HasResponse = true
			info.HasStream = resp.Stream != nil
			info.HasNonStream = resp.NonStream != nil
			info.HasTokenCount = resp.TokenCount != nil
		}
	}

	return info
}

// GetTranslationInfo returns translation info from the default registry.
func GetTranslationInfo(from, to Format) *TranslationInfo {
	return defaultRegistry.GetTranslationInfo(from, to)
}

// GetAllTranslations returns information about all registered translation paths.
func (r *Registry) GetAllTranslations() []TranslationInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use a map to track unique pairs
	pairs := make(map[string]*TranslationInfo)

	// Collect from request translators
	for from, targets := range r.requests {
		for to := range targets {
			key := from.String() + "->" + to.String()
			if _, exists := pairs[key]; !exists {
				pairs[key] = &TranslationInfo{
					From: from,
					To:   to,
				}
			}
			pairs[key].HasRequest = true
		}
	}

	// Collect from response translators
	for from, targets := range r.responses {
		for to, resp := range targets {
			key := from.String() + "->" + to.String()
			if _, exists := pairs[key]; !exists {
				pairs[key] = &TranslationInfo{
					From: from,
					To:   to,
				}
			}
			pairs[key].HasResponse = true
			pairs[key].HasStream = resp.Stream != nil
			pairs[key].HasNonStream = resp.NonStream != nil
			pairs[key].HasTokenCount = resp.TokenCount != nil
		}
	}

	// Convert to slice and sort
	result := make([]TranslationInfo, 0, len(pairs))
	for _, info := range pairs {
		result = append(result, *info)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].From.String() != result[j].From.String() {
			return result[i].From.String() < result[j].From.String()
		}
		return result[i].To.String() < result[j].To.String()
	})

	return result
}

// GetAllTranslations returns all translations from the default registry.
func GetAllTranslations() []TranslationInfo {
	return defaultRegistry.GetAllTranslations()
}
