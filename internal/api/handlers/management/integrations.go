package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/integrations"
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

// GetCLIAgents returns the list of detected CLI agents and IDEs.
func (h *Handler) GetCLIAgents(c *gin.Context) {
	proxyURL := h.cfg.ProxyURL
	if proxyURL == "" {
		proxyURL = "http://localhost:8080"
	}
	agents := integrations.DetectCLIAgents(proxyURL)
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// PostCLIAgentConfigure configures a CLI agent to use the proxy.
func (h *Handler) PostCLIAgentConfigure(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing agent id"})
		return
	}
	proxyURL := h.cfg.ProxyURL
	if proxyURL == "" {
		proxyURL = "http://localhost:8080"
	}
	if err := integrations.ConfigureCLIAgent(id, proxyURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "configured", "id": id})
}

// PostCLIAgentUnconfigure removes proxy configuration from a CLI agent.
func (h *Handler) PostCLIAgentUnconfigure(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing agent id"})
		return
	}
	if err := integrations.UnconfigureCLIAgent(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "unconfigured", "id": id})
}
