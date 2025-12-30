// Package middleware provides HTTP middleware components for the CLI Proxy API server.
// This file contains the connection tracking middleware that tracks active HTTP connections.
package middleware

import (
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// ConnectionTracker provides thread-safe tracking of active HTTP connections.
// It uses atomic operations for safe concurrent access without locks.
type ConnectionTracker struct {
	count atomic.Int64
}

// Increment atomically increases the active connection count by 1.
func (ct *ConnectionTracker) Increment() {
	ct.count.Add(1)
}

// Decrement atomically decreases the active connection count by 1.
func (ct *ConnectionTracker) Decrement() {
	ct.count.Add(-1)
}

// Count returns the current number of active connections.
func (ct *ConnectionTracker) Count() int64 {
	return ct.count.Load()
}

// ActiveConnections is the global connection tracker instance used by the server.
var ActiveConnections = &ConnectionTracker{}

// ConnectionTrackerMiddleware returns a Gin middleware that tracks active HTTP connections.
// It increments the counter when a request starts and decrements it when the response completes.
func ConnectionTrackerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ActiveConnections.Increment()
		defer ActiveConnections.Decrement()
		c.Next()
	}
}
