package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetIntegrationsStatus returns the list of detected tools and their configuration status.
func (h *Handler) GetIntegrationsStatus(c *gin.Context) {
	if h.integrationManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "integration manager not initialized"})
		return
	}
	statuses := h.integrationManager.ListStatus()
	c.JSON(http.StatusOK, gin.H{"integrations": statuses})
}

// PostIntegrationConfigure triggers the configuration logic for a specific tool.
func (h *Handler) PostIntegrationConfigure(c *gin.Context) {
	if h.integrationManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "integration manager not initialized"})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing integration id"})
		return
	}

	if err := h.integrationManager.Configure(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "configured", "id": id})
}
