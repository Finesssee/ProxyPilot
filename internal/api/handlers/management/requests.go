package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
)

// GetRequests returns the list of recent requests from the in-memory ring buffer.
func (h *Handler) GetRequests(c *gin.Context) {
	requests := middleware.GetRequestMonitor()
	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
	})
}
