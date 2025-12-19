// Package logging provides Gin middleware for HTTP request logging and panic recovery.
// It integrates Gin web framework with logrus for structured logging of HTTP requests,
// responses, and error handling with panic recovery capabilities.
package logging

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	log "github.com/sirupsen/logrus"
)

const skipGinLogKey = "__gin_skip_request_logging__"

// GinLogrusLogger returns a Gin middleware handler that logs HTTP requests and responses
// using logrus. It captures request details including method, path, status code, latency,
// client IP, and any error messages, and attaches structured fields for downstream analysis.
//
// Returns:
//   - gin.HandlerFunc: A middleware handler for request logging
func GinLogrusLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := util.MaskSensitiveQuery(c.Request.URL.RawQuery)

		// Derive or generate a request ID and propagate it via response headers.
		requestID := c.Request.Header.Get("X-Request-Id")
		if strings.TrimSpace(requestID) == "" {
			requestID = uuid.NewString()
		}
		c.Writer.Header().Set("X-Request-Id", requestID)

		c.Next()

		if shouldSkipGinRequestLogging(c) {
			return
		}

		if raw != "" {
			path = path + "?" + raw
		}

		latency := time.Since(start)
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		} else {
			latency = latency.Truncate(time.Millisecond)
		}

		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		userAgent := c.Request.UserAgent()
		// Basic client classification to help distinguish agentic CLIs and IDEs.
		clientType := "generic"
		uaLower := strings.ToLower(userAgent)
		if strings.Contains(uaLower, "factory-cli") || strings.Contains(uaLower, "droid") {
			clientType = "factory-cli"
		} else if strings.Contains(uaLower, "openai codex") {
			clientType = "codex-cli"
		} else if strings.Contains(uaLower, "warp") {
			clientType = "warp-cli"
		} else if strings.Contains(uaLower, "cursor") {
			clientType = "cursor-ide"
		}

		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		timestamp := time.Now().Format("2006/01/02 - 15:04:05")
		logLine := fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s \"%s\"", timestamp, statusCode, latency, clientIP, method, path)
		if errorMessage != "" {
			logLine = logLine + " | " + errorMessage
		}

		fields := log.Fields{
			"status":      statusCode,
			"latency_ms":  latency.Milliseconds(),
			"client_ip":   clientIP,
			"method":      method,
			"path":        path,
			"request_id":  requestID,
			"client_type": clientType,
		}
		// Avoid logging very long user-agents verbatim, but keep a shortened hint.
		if userAgent != "" {
			ua := userAgent
			if len(ua) > 180 {
				ua = ua[:180] + "..."
			}
			fields["user_agent"] = ua
		}

		entry := log.WithFields(fields)
		switch {
		case statusCode >= http.StatusInternalServerError:
			entry.Error(logLine)
		case statusCode >= http.StatusBadRequest:
			entry.Warn(logLine)
		default:
			entry.Info(logLine)
		}
	}
}

// GinLogrusRecovery returns a Gin middleware handler that recovers from panics and logs
// them using logrus. When a panic occurs, it captures the panic value, stack trace,
// and request path, then returns a 500 Internal Server Error response to the client.
//
// Returns:
//   - gin.HandlerFunc: A middleware handler for panic recovery
func GinLogrusRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		log.WithFields(log.Fields{
			"panic": recovered,
			"stack": string(debug.Stack()),
			"path":  c.Request.URL.Path,
		}).Error("recovered from panic")

		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

// SkipGinRequestLogging marks the provided Gin context so that GinLogrusLogger
// will skip emitting a log line for the associated request.
func SkipGinRequestLogging(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(skipGinLogKey, true)
}

func shouldSkipGinRequestLogging(c *gin.Context) bool {
	if c == nil {
		return false
	}
	val, exists := c.Get(skipGinLogKey)
	if !exists {
		return false
	}
	flag, ok := val.(bool)
	return ok && flag
}
