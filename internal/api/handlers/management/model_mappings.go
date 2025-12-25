package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// ModelMappingEntry represents a global model mapping for the API
type ModelMappingEntry struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Provider string `json:"provider,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// GetModelMappings returns the current global model mappings
func (h *Handler) GetModelMappings(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusOK, gin.H{"mappings": []any{}})
		return
	}

	mappings := make([]ModelMappingEntry, 0, len(h.cfg.GlobalModelMappings))
	for _, m := range h.cfg.GlobalModelMappings {
		mappings = append(mappings, ModelMappingEntry{
			From:     m.From,
			To:       m.To,
			Provider: m.Provider,
			Enabled:  m.IsEnabled(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"mappings": mappings})
}

// SetModelMappings replaces all global model mappings
func (h *Handler) SetModelMappings(c *gin.Context) {
	var req struct {
		Mappings []ModelMappingEntry `json:"mappings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "config not loaded"})
		return
	}

	// Convert to config type
	newMappings := make([]config.GlobalModelMapping, 0, len(req.Mappings))
	for _, m := range req.Mappings {
		enabled := m.Enabled
		newMappings = append(newMappings, config.GlobalModelMapping{
			From:     m.From,
			To:       m.To,
			Provider: m.Provider,
			Enabled:  &enabled,
		})
	}

	h.cfg.GlobalModelMappings = newMappings

	// Try to save config if configPath is available
	// Note: This requires the config path to be stored somewhere accessible

	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": len(newMappings)})
}

// TestModelMapping tests a model name against global mappings
func (h *Handler) TestModelMapping(c *gin.Context) {
	model := c.Query("model")
	provider := c.Query("provider")

	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing model parameter"})
		return
	}

	if h.cfg == nil {
		c.JSON(http.StatusOK, gin.H{"model": model, "mapped_to": model, "matched": false})
		return
	}

	mappedTo := h.cfg.LookupGlobalModelMapping(model, provider)
	if mappedTo == "" {
		c.JSON(http.StatusOK, gin.H{"model": model, "mapped_to": model, "matched": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"model": model, "mapped_to": mappedTo, "matched": true})
}
