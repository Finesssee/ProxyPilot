package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetAuthUsage returns usage statistics for a specific auth entry.
func (h *Handler) GetAuthUsage(c *gin.Context) {
	authID := strings.TrimSpace(c.Param("id"))
	if authID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing auth id"})
		return
	}
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth manager unavailable"})
		return
	}
	auth, ok := h.authManager.GetByID(authID)
	if !ok || auth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total_input_tokens":  auth.Usage.TotalInputTokens,
		"total_output_tokens": auth.Usage.TotalOutputTokens,
		"request_count":       auth.Usage.RequestCount,
		"daily_input_tokens":  auth.Usage.DailyInputTokens,
		"daily_output_tokens": auth.Usage.DailyOutputTokens,
		"daily_request_count": auth.Usage.DailyRequestCount,
	})
}
