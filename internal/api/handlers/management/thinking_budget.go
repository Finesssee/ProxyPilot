package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// ThinkingBudgetResponse represents the thinking budget settings response.
type ThinkingBudgetResponse struct {
	Mode            string         `json:"mode"`
	CustomTokens    int            `json:"custom_tokens"`
	Enabled         bool           `json:"enabled"`
	EffectiveTokens int            `json:"effective_tokens"`
	Presets         map[string]int `json:"presets"`
}

// ThinkingBudgetRequest represents a request to update thinking budget settings.
type ThinkingBudgetRequest struct {
	Mode         string `json:"mode"`
	CustomTokens int    `json:"custom_tokens"`
	Enabled      *bool  `json:"enabled"`
}

// GetThinkingBudget returns the current thinking budget settings.
// GET /v0/management/thinking-budget
func (h *Handler) GetThinkingBudget(c *gin.Context) {
	cfg := h.cfg
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "config not available"})
		return
	}

	tb := cfg.ThinkingBudget

	response := ThinkingBudgetResponse{
		Mode:            tb.Mode,
		CustomTokens:    tb.CustomTokens,
		Enabled:         tb.IsEnabled(),
		EffectiveTokens: tb.GetThinkingBudgetTokens(),
		Presets:         config.ThinkingBudgetPresets,
	}

	// Default mode if empty
	if response.Mode == "" {
		response.Mode = "medium"
	}

	c.JSON(http.StatusOK, response)
}

// SetThinkingBudget updates the thinking budget settings.
// PUT /v0/management/thinking-budget
func (h *Handler) SetThinkingBudget(c *gin.Context) {
	cfg := h.cfg
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "config not available"})
		return
	}

	var req ThinkingBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate mode
	validModes := map[string]bool{"low": true, "medium": true, "high": true, "custom": true}
	if req.Mode != "" && !validModes[req.Mode] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode, must be one of: low, medium, high, custom"})
		return
	}

	// Validate custom tokens
	if req.Mode == "custom" && req.CustomTokens <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "custom_tokens must be positive when mode is 'custom'"})
		return
	}

	// Update config
	if req.Mode != "" {
		cfg.ThinkingBudget.Mode = req.Mode
	}
	if req.CustomTokens > 0 {
		cfg.ThinkingBudget.CustomTokens = req.CustomTokens
	}
	if req.Enabled != nil {
		cfg.ThinkingBudget.Enabled = req.Enabled
	}

	// Return updated settings
	tb := cfg.ThinkingBudget
	response := ThinkingBudgetResponse{
		Mode:            tb.Mode,
		CustomTokens:    tb.CustomTokens,
		Enabled:         tb.IsEnabled(),
		EffectiveTokens: tb.GetThinkingBudgetTokens(),
		Presets:         config.ThinkingBudgetPresets,
	}

	if response.Mode == "" {
		response.Mode = "medium"
	}

	c.JSON(http.StatusOK, response)
}
