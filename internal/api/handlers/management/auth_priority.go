package management

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type setAuthPriorityRequest struct {
	Priority *int `json:"priority"`
}

// SetAuthPriority updates the priority field for an auth entry.
func (h *Handler) SetAuthPriority(c *gin.Context) {
	authID := strings.TrimSpace(c.Param("id"))
	if authID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing auth id"})
		return
	}

	var req setAuthPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Priority == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
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

	auth.Priority = *req.Priority
	auth.UpdatedAt = time.Now()

	updated, err := h.authManager.Update(c.Request.Context(), auth)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.buildAuthFileEntry(updated))
}
