// Package middleware provides HTTP middleware components for the CLI Proxy API server.
// This file contains Prometheus metrics middleware for observability.
package middleware

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// httpRequestsTotal counts the total number of HTTP requests processed.
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxypilot_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDurationSeconds tracks the duration of HTTP requests.
	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxypilot_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// httpRequestSizeBytes tracks the size of HTTP request bodies.
	httpRequestSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxypilot_http_request_size_bytes",
			Help:    "Size of HTTP request bodies in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to 10GB
		},
		[]string{"method", "path"},
	)

	// httpResponseSizeBytes tracks the size of HTTP response bodies.
	httpResponseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxypilot_http_response_size_bytes",
			Help:    "Size of HTTP response bodies in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to 10GB
		},
		[]string{"method", "path"},
	)

	// activeConnections tracks the number of currently active connections.
	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "proxypilot_active_connections",
			Help: "Number of currently active HTTP connections",
		},
	)

	// activeConnectionsCount provides atomic access to the connection count.
	activeConnectionsCount int64

	// apiRequestsByProvider counts requests by AI provider.
	apiRequestsByProvider = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxypilot_api_requests_by_provider_total",
			Help: "Total API requests grouped by AI provider",
		},
		[]string{"provider", "model"},
	)

	// apiRequestErrors counts API request errors by type.
	apiRequestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxypilot_api_request_errors_total",
			Help: "Total number of API request errors",
		},
		[]string{"error_type", "provider"},
	)

	// tokenUsage tracks token usage for AI API calls.
	tokenUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxypilot_token_usage_total",
			Help: "Total tokens used in API requests",
		},
		[]string{"provider", "model", "type"}, // type: input or output
	)

	// Response cache metrics
	responseCacheHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "proxypilot_response_cache_hits_total",
			Help: "Total number of response cache hits",
		},
	)
	responseCacheMissesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "proxypilot_response_cache_misses_total",
			Help: "Total number of response cache misses",
		},
	)
	responseCacheSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "proxypilot_response_cache_size",
			Help: "Current number of entries in the response cache",
		},
	)

	// Prompt cache metrics
	promptCacheHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "proxypilot_prompt_cache_hits_total",
			Help: "Total number of prompt cache hits",
		},
	)
	promptCacheMissesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "proxypilot_prompt_cache_misses_total",
			Help: "Total number of prompt cache misses",
		},
	)
	promptCacheSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "proxypilot_prompt_cache_size",
			Help: "Current number of entries in the prompt cache",
		},
	)
	promptCacheTokensSavedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "proxypilot_prompt_cache_tokens_saved_total",
			Help: "Estimated total tokens saved by prompt cache hits",
		},
	)

	// metricsRegistered ensures metrics are only registered once.
	metricsRegistered atomic.Bool
	metricsEnabled    atomic.Bool
)

// SetMetricsEnabled toggles Prometheus metrics collection.
func SetMetricsEnabled(enabled bool) {
	metricsEnabled.Store(enabled)
}

// IsMetricsEnabled reports whether metrics are enabled.
func IsMetricsEnabled() bool {
	return metricsEnabled.Load()
}

// RegisterMetrics registers all Prometheus metrics.
// It is safe to call multiple times; metrics will only be registered once.
func RegisterMetrics() {
	if !metricsRegistered.CompareAndSwap(false, true) {
		return
	}

	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDurationSeconds,
		httpRequestSizeBytes,
		httpResponseSizeBytes,
		activeConnections,
		apiRequestsByProvider,
		apiRequestErrors,
		tokenUsage,
		responseCacheHitsTotal,
		responseCacheMissesTotal,
		responseCacheSize,
		promptCacheHitsTotal,
		promptCacheMissesTotal,
		promptCacheSize,
		promptCacheTokensSavedTotal,
	)
}

// PrometheusMiddleware returns a Gin middleware that collects Prometheus metrics
// for HTTP requests including request count, duration, and active connections.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsMetricsEnabled() {
			c.Next()
			return
		}
		// Ensure metrics are registered
		RegisterMetrics()

		// Skip metrics endpoint to avoid self-referential metrics
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// Track active connections
		atomic.AddInt64(&activeConnectionsCount, 1)
		activeConnections.Inc()
		defer func() {
			atomic.AddInt64(&activeConnectionsCount, -1)
			activeConnections.Dec()
		}()

		// Normalize path for metrics to avoid high cardinality
		path := normalizePath(c.Request.URL.Path)
		method := c.Request.Method

		// Track request size
		if c.Request.ContentLength > 0 {
			httpRequestSizeBytes.WithLabelValues(method, path).Observe(float64(c.Request.ContentLength))
		}

		// Record start time
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get status code
		status := strconv.Itoa(c.Writer.Status())

		// Record metrics
		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDurationSeconds.WithLabelValues(method, path).Observe(duration)

		// Track response size
		responseSize := c.Writer.Size()
		if responseSize > 0 {
			httpResponseSizeBytes.WithLabelValues(method, path).Observe(float64(responseSize))
		}

		// Track provider-specific metrics if available
		if provider, exists := c.Get("provider"); exists {
			if providerStr, ok := provider.(string); ok {
				model := ""
				if m, exists := c.Get("model"); exists {
					if modelStr, ok := m.(string); ok {
						model = modelStr
					}
				}
				apiRequestsByProvider.WithLabelValues(providerStr, model).Inc()

				// Track token usage if available
				if inputTokens, exists := c.Get("input_tokens"); exists {
					if tokens, ok := inputTokens.(int); ok && tokens > 0 {
						tokenUsage.WithLabelValues(providerStr, model, "input").Add(float64(tokens))
					} else if tokens, ok := inputTokens.(int64); ok && tokens > 0 {
						tokenUsage.WithLabelValues(providerStr, model, "input").Add(float64(tokens))
					}
				}
				if outputTokens, exists := c.Get("output_tokens"); exists {
					if tokens, ok := outputTokens.(int); ok && tokens > 0 {
						tokenUsage.WithLabelValues(providerStr, model, "output").Add(float64(tokens))
					} else if tokens, ok := outputTokens.(int64); ok && tokens > 0 {
						tokenUsage.WithLabelValues(providerStr, model, "output").Add(float64(tokens))
					}
				}
			}
		}

		// Track errors if present
		if c.Writer.Status() >= 400 {
			errorType := "client_error"
			if c.Writer.Status() >= 500 {
				errorType = "server_error"
			}
			provider := "unknown"
			if p, exists := c.Get("provider"); exists {
				if providerStr, ok := p.(string); ok {
					provider = providerStr
				}
			}
			apiRequestErrors.WithLabelValues(errorType, provider).Inc()
		}
	}
}

// normalizePath normalizes URL paths to prevent high cardinality in metrics.
// It replaces dynamic path segments with placeholders.
func normalizePath(path string) string {
	// Define known API path patterns and their normalized forms
	switch {
	case path == "/":
		return "/"
	case path == "/healthz":
		return "/healthz"
	case path == "/metrics":
		return "/metrics"
	case path == "/v1/models" || path == "/models":
		return "/v1/models"
	case path == "/v1/chat/completions" || path == "/chat/completions":
		return "/v1/chat/completions"
	case path == "/v1/completions" || path == "/completions":
		return "/v1/completions"
	case path == "/v1/messages" || path == "/messages":
		return "/v1/messages"
	case path == "/v1/messages/count_tokens":
		return "/v1/messages/count_tokens"
	case path == "/v1/responses" || path == "/responses":
		return "/v1/responses"
	case len(path) > 8 && path[:8] == "/v1beta/":
		return "/v1beta/*"
	case len(path) > 15 && path[:15] == "/v0/management/":
		return "/v0/management/*"
	case len(path) > 11 && path[:11] == "/management":
		return "/management/*"
	default:
		// For other paths, return as-is but truncate if too long
		if len(path) > 50 {
			return path[:50] + "..."
		}
		return path
	}
}

// MetricsHandler returns the Prometheus HTTP handler for the /metrics endpoint.
func MetricsHandler() gin.HandlerFunc {
	handler := promhttp.Handler()
	return func(c *gin.Context) {
		if !IsMetricsEnabled() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		// Ensure metrics are registered before serving
		RegisterMetrics()
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// GetActiveConnections returns the current number of active connections.
func GetActiveConnections() int64 {
	return atomic.LoadInt64(&activeConnectionsCount)
}

// GetRequestMonitor returns an empty list as request monitoring is not currently required.
// This stub exists for API compatibility and can be extended if monitoring needs arise.
func GetRequestMonitor() []any {
	return []any{}
}

// RecordProviderRequest records a request to a specific AI provider.
// This can be called from handlers to track provider-specific metrics.
func RecordProviderRequest(provider, model string) {
	if !IsMetricsEnabled() {
		return
	}
	apiRequestsByProvider.WithLabelValues(provider, model).Inc()
}

// RecordTokenUsage records token usage for an AI API call.
// tokenType should be either "input" or "output".
func RecordTokenUsage(provider, model, tokenType string, tokens int) {
	if !IsMetricsEnabled() {
		return
	}
	if tokens > 0 {
		tokenUsage.WithLabelValues(provider, model, tokenType).Add(float64(tokens))
	}
}

// RecordAPIError records an API error.
// errorType should describe the type of error (e.g., "rate_limit", "auth_error", "server_error").
func RecordAPIError(errorType, provider string) {
	if !IsMetricsEnabled() {
		return
	}
	apiRequestErrors.WithLabelValues(errorType, provider).Inc()
}

// RecordResponseCacheHit increments the response cache hit counter.
func RecordResponseCacheHit() {
	if !IsMetricsEnabled() {
		return
	}
	responseCacheHitsTotal.Inc()
}

// RecordResponseCacheMiss increments the response cache miss counter.
func RecordResponseCacheMiss() {
	if !IsMetricsEnabled() {
		return
	}
	responseCacheMissesTotal.Inc()
}

// SetResponseCacheSize sets the current response cache size gauge.
func SetResponseCacheSize(size int) {
	if !IsMetricsEnabled() {
		return
	}
	responseCacheSize.Set(float64(size))
}

// RecordPromptCacheHit increments the prompt cache hit counter.
func RecordPromptCacheHit() {
	if !IsMetricsEnabled() {
		return
	}
	promptCacheHitsTotal.Inc()
}

// RecordPromptCacheMiss increments the prompt cache miss counter.
func RecordPromptCacheMiss() {
	if !IsMetricsEnabled() {
		return
	}
	promptCacheMissesTotal.Inc()
}

// SetPromptCacheSize sets the current prompt cache size gauge.
func SetPromptCacheSize(size int) {
	if !IsMetricsEnabled() {
		return
	}
	promptCacheSize.Set(float64(size))
}

// RecordPromptCacheTokensSaved adds to the total tokens saved counter.
func RecordPromptCacheTokensSaved(tokens int) {
	if !IsMetricsEnabled() {
		return
	}
	if tokens > 0 {
		promptCacheTokensSavedTotal.Add(float64(tokens))
	}
}
