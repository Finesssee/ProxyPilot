// Package middleware provides HTTP middleware components for the CLI Proxy API server.
// This file contains the request logging middleware that captures comprehensive
// request and response data when enabled through configuration.
package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

var (
	requestMonitorMu sync.RWMutex
	requestMonitor   []interfaces.RequestLogEntry
	maxMonitorSize   = 100
)

func addRequestToMonitor(entry interfaces.RequestLogEntry) {
	requestMonitorMu.Lock()
	defer requestMonitorMu.Unlock()

	requestMonitor = append(requestMonitor, entry)
	if len(requestMonitor) > maxMonitorSize {
		requestMonitor = requestMonitor[1:]
	}
}

// GetRequestMonitor returns a copy of the current request monitor entries.
func GetRequestMonitor() []interfaces.RequestLogEntry {
	requestMonitorMu.RLock()
	defer requestMonitorMu.RUnlock()

	result := make([]interfaces.RequestLogEntry, len(requestMonitor))
	copy(result, requestMonitor)
	return result
}

// RequestLoggingMiddleware creates a Gin middleware that logs HTTP requests and responses.
// It captures detailed information about the request and response, including headers and body,
// and uses the provided RequestLogger to record this data. When logging is disabled in the
// logger, it still captures data so that upstream errors can be persisted.
func RequestLoggingMiddleware(logger logging.RequestLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if logger == nil {
			c.Next()
			return
		}

		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if !shouldLogRequest(path) {
			c.Next()
			return
		}

		// Capture request information
		requestInfo, err := captureRequestInfo(c)
		if err != nil {
			// Log error but continue processing
			// In a real implementation, you might want to use a proper logger here
			c.Next()
			return
		}

		// Create response writer wrapper
		wrapper := NewResponseWriterWrapper(c.Writer, logger, requestInfo)
		if !logger.IsEnabled() {
			wrapper.logOnErrorOnly = true
		}
		c.Writer = wrapper

		// Process the request
		startTime := time.Now()
		c.Next()
		latency := time.Since(startTime)

		// Finalize logging after request processing
		if err = wrapper.Finalize(c); err != nil {
			// Log error but don't interrupt the response
			// In a real implementation, you might want to use a proper logger here
		}

		// Add to monitor
		entry := interfaces.RequestLogEntry{
			ID:        logging.GetGinRequestID(c),
			Timestamp: startTime,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			LatencyMs: latency.Milliseconds(),
		}

		// Try to get Model and Provider from context
		if model, ok := c.Get("model"); ok {
			if m, ok := model.(string); ok {
				entry.Model = m
			}
		}
		if provider, ok := c.Get("provider"); ok {
			if p, ok := provider.(string); ok {
				entry.Provider = p
			}
		}

		// Try to get tokens from context
		if inputTokens, ok := c.Get("input_tokens"); ok {
			if it, ok := inputTokens.(int); ok {
				entry.InputTokens = it
			} else if it, ok := inputTokens.(int64); ok {
				entry.InputTokens = int(it)
			}
		}
		if outputTokens, ok := c.Get("output_tokens"); ok {
			if ot, ok := outputTokens.(int); ok {
				entry.OutputTokens = ot
			} else if ot, ok := outputTokens.(int64); ok {
				entry.OutputTokens = int(ot)
			}
		}

		// Try to get error from context
		if apiErr, ok := c.Get("API_RESPONSE_ERROR"); ok {
			if errs, ok := apiErr.([]*interfaces.ErrorMessage); ok && len(errs) > 0 {
				var errMsgs []string
				for _, e := range errs {
					if e != nil && e.Error != nil {
						errMsgs = append(errMsgs, e.Error.Error())
					}
				}
				entry.Error = strings.Join(errMsgs, "; ")
			}
		}

		addRequestToMonitor(entry)
	}
}

// captureRequestInfo extracts relevant information from the incoming HTTP request.
// It captures the URL, method, headers, and body. The request body is read and then
// restored so that it can be processed by subsequent handlers.
func captureRequestInfo(c *gin.Context) (*RequestInfo, error) {
	// Capture URL with sensitive query parameters masked
	maskedQuery := util.MaskSensitiveQuery(c.Request.URL.RawQuery)
	url := c.Request.URL.Path
	if maskedQuery != "" {
		url += "?" + maskedQuery
	}

	// Capture method
	method := c.Request.Method

	// Capture headers
	headers := make(map[string][]string)
	for key, values := range c.Request.Header {
		headers[key] = values
	}

	// Capture request body
	var body []byte
	if c.Request.Body != nil {
		// Read the body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return nil, err
		}

		// Restore the body for the actual request processing
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		body = bodyBytes
	}

	return &RequestInfo{
		URL:       url,
		Method:    method,
		Headers:   headers,
		Body:      body,
		RequestID: logging.GetGinRequestID(c),
	}, nil
}

// shouldLogRequest determines whether the request should be logged.
// It skips management endpoints to avoid leaking secrets but allows
// all other routes, including module-provided ones, to honor request-log.
func shouldLogRequest(path string) bool {
	if strings.HasPrefix(path, "/v0/management") || strings.HasPrefix(path, "/management") {
		return false
	}

	if strings.HasPrefix(path, "/api") {
		return strings.HasPrefix(path, "/api/provider")
	}

	return true
}
