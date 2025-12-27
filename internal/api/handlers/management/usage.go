package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

// GetUsageStatistics returns the in-memory request statistics snapshot.
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}

	usageStats := usage.ComputeUsageStats(snapshot)

	c.JSON(http.StatusOK, gin.H{
		"usage":           usageStats,
		"raw_snapshot":    snapshot,
		"failed_requests": snapshot.FailureCount,
	})
}
