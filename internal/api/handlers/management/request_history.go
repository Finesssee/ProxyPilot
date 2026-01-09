package management

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
)

// GetRequestHistory returns the persistent request history with optional filtering.
// GET /v0/management/request-history
func (h *Handler) GetRequestHistory(c *gin.Context) {
	history := middleware.GetRequestHistory()
	if history == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request history disabled"})
		return
	}

	// Parse filter parameters
	filter := &middleware.RequestHistoryFilter{}

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if t, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filter.StartDate = &t
		}
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if t, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filter.EndDate = &t
		}
	}

	filter.Model = c.Query("model")
	filter.Provider = c.Query("provider")

	if statusMinStr := c.Query("status_min"); statusMinStr != "" {
		if v, err := strconv.Atoi(statusMinStr); err == nil {
			filter.StatusMin = v
		}
	}

	if statusMaxStr := c.Query("status_max"); statusMaxStr != "" {
		if v, err := strconv.Atoi(statusMaxStr); err == nil {
			filter.StatusMax = v
		}
	}

	if errorsOnly := c.Query("errors_only"); errorsOnly == "true" {
		filter.ErrorsOnly = true
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = v
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = v
		}
	}

	entries := history.GetEntries(filter)
	total := history.Count()

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"total":   total,
		"count":   len(entries),
		"filter":  filter,
	})
}

// GetRequestHistoryStats returns aggregated statistics from request history.
// GET /v0/management/request-history/stats
func (h *Handler) GetRequestHistoryStats(c *gin.Context) {
	history := middleware.GetRequestHistory()
	if history == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request history disabled"})
		return
	}
	stats := history.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// ClearRequestHistory clears all entries from persistent request history.
// DELETE /v0/management/request-history
func (h *Handler) ClearRequestHistory(c *gin.Context) {
	history := middleware.GetRequestHistory()
	if history == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request history disabled"})
		return
	}
	history.Clear()

	c.JSON(http.StatusOK, gin.H{
		"message": "Request history cleared",
	})
}

// ExportRequestHistory exports the full request history for backup.
// GET /v0/management/request-history/export
func (h *Handler) ExportRequestHistory(c *gin.Context) {
	history := middleware.GetRequestHistory()
	if history == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request history disabled"})
		return
	}
	snapshot := history.Export()

	c.JSON(http.StatusOK, snapshot)
}

// ImportRequestHistory imports request history from a backup.
// POST /v0/management/request-history/import
func (h *Handler) ImportRequestHistory(c *gin.Context) {
	history := middleware.GetRequestHistory()
	if history == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request history disabled"})
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var snapshot middleware.RequestHistorySnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	added, skipped := history.Import(snapshot)

	c.JSON(http.StatusOK, gin.H{
		"added":   added,
		"skipped": skipped,
		"total":   history.Count(),
	})
}

// SaveRequestHistory forces a save of request history to disk.
// POST /v0/management/request-history/save
func (h *Handler) SaveRequestHistory(c *gin.Context) {
	history := middleware.GetRequestHistory()
	if history == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request history disabled"})
		return
	}
	if err := history.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Request history saved",
	})
}
