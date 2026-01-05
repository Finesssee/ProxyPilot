package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cache"
)

// CacheStatsResponse represents the response for cache statistics.
type CacheStatsResponse struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Evictions  int64 `json:"evictions"`
	Size       int   `json:"size"`
	TotalSaved int64 `json:"total_saved"`
	Enabled    bool  `json:"enabled"`
}

// GetCacheStats returns cache statistics.
// GET /v0/management/cache/stats
func (h *Handler) GetCacheStats(c *gin.Context) {
	responseCache := cache.GetDefaultResponseCache()
	if responseCache == nil {
		c.JSON(http.StatusOK, CacheStatsResponse{Enabled: false})
		return
	}

	stats := responseCache.GetStats()
	c.JSON(http.StatusOK, CacheStatsResponse{
		Hits:       stats.Hits,
		Misses:     stats.Misses,
		Evictions:  stats.Evictions,
		Size:       stats.Size,
		TotalSaved: stats.TotalSaved,
		Enabled:    responseCache.IsEnabled(),
	})
}

// ClearCache clears all cache entries.
// POST /v0/management/cache/clear
func (h *Handler) ClearCache(c *gin.Context) {
	responseCache := cache.GetDefaultResponseCache()
	if responseCache == nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "cache not initialized"})
		return
	}

	responseCache.Clear()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "cache cleared"})
}

// CacheEnabledRequest is the request body for enabling/disabling cache.
type CacheEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// SetCacheEnabled enables or disables the cache at runtime.
// PUT /v0/management/cache/enabled
func (h *Handler) SetCacheEnabled(c *gin.Context) {
	var req CacheEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	responseCache := cache.GetDefaultResponseCache()
	if responseCache == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache not initialized"})
		return
	}

	responseCache.SetEnabled(req.Enabled)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "enabled": req.Enabled})
}

// PromptCacheStatsResponse represents the response for prompt cache statistics.
type PromptCacheStatsResponse struct {
	Hits                 int64            `json:"hits"`
	Misses               int64            `json:"misses"`
	Evictions            int64            `json:"evictions"`
	Size                 int              `json:"size"`
	UniquePrompts        int64            `json:"unique_prompts"`
	TotalRequests        int64            `json:"total_requests"`
	EstimatedTokensSaved int64            `json:"estimated_tokens_saved"`
	TopProviders         map[string]int64 `json:"top_providers,omitempty"`
	Enabled              bool             `json:"enabled"`
}

// GetPromptCacheStats returns prompt cache statistics.
// GET /v0/management/prompt-cache/stats
func (h *Handler) GetPromptCacheStats(c *gin.Context) {
	promptCache := cache.GetDefaultPromptCache()
	if promptCache == nil {
		c.JSON(http.StatusOK, PromptCacheStatsResponse{Enabled: false})
		return
	}

	stats := promptCache.GetStats()
	c.JSON(http.StatusOK, PromptCacheStatsResponse{
		Hits:                 stats.Hits,
		Misses:               stats.Misses,
		Evictions:            stats.Evictions,
		Size:                 stats.Size,
		UniquePrompts:        stats.UniquePrompts,
		TotalRequests:        stats.TotalRequests,
		EstimatedTokensSaved: stats.EstimatedTokensSaved,
		TopProviders:         stats.TopProviders,
		Enabled:              promptCache.IsEnabled(),
	})
}

// ClearPromptCache clears all prompt cache entries.
// POST /v0/management/prompt-cache/clear
func (h *Handler) ClearPromptCache(c *gin.Context) {
	promptCache := cache.GetDefaultPromptCache()
	if promptCache == nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "prompt cache not initialized"})
		return
	}

	promptCache.Clear()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "prompt cache cleared"})
}

// SetPromptCacheEnabled enables or disables the prompt cache at runtime.
// PUT /v0/management/prompt-cache/enabled
func (h *Handler) SetPromptCacheEnabled(c *gin.Context) {
	var req CacheEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	promptCache := cache.GetDefaultPromptCache()
	if promptCache == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prompt cache not initialized"})
		return
	}

	promptCache.SetEnabled(req.Enabled)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "enabled": req.Enabled})
}

// TopPromptEntry represents a frequently used prompt.
type TopPromptEntry struct {
	Hash          string         `json:"hash"`
	TokenEstimate int            `json:"token_estimate"`
	HitCount      int64          `json:"hit_count"`
	Providers     map[string]int `json:"providers,omitempty"`
	PromptPreview string         `json:"prompt_preview"`
}

// GetTopPrompts returns the most frequently hit prompts.
// GET /v0/management/prompt-cache/top
func (h *Handler) GetTopPrompts(c *gin.Context) {
	promptCache := cache.GetDefaultPromptCache()
	if promptCache == nil {
		c.JSON(http.StatusOK, gin.H{"prompts": []TopPromptEntry{}})
		return
	}

	top := promptCache.GetTopPrompts(10)
	entries := make([]TopPromptEntry, 0, len(top))
	for _, p := range top {
		preview := p.Prompt
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		entries = append(entries, TopPromptEntry{
			Hash:          p.Hash,
			TokenEstimate: p.TokenEstimate,
			HitCount:      p.HitCount,
			Providers:     p.Providers,
			PromptPreview: preview,
		})
	}
	c.JSON(http.StatusOK, gin.H{"prompts": entries})
}

// WarmPromptCacheRequest is the request body for warming the prompt cache.
type WarmPromptCacheRequest struct {
	Prompts []cache.WarmPromptEntry `json:"prompts"`
}

// WarmPromptCacheResponse is the response for the warm operation.
type WarmPromptCacheResponse struct {
	Status  string `json:"status"`
	Total   int    `json:"total"`
	Added   int    `json:"added"`
	Skipped int    `json:"skipped"`
	Errors  int    `json:"errors"`
}

// WarmPromptCache pre-populates the prompt cache with known system prompts.
// POST /v0/management/prompt-cache/warm
func (h *Handler) WarmPromptCache(c *gin.Context) {
	var req WarmPromptCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if len(req.Prompts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no prompts provided"})
		return
	}

	promptCache := cache.GetDefaultPromptCache()
	if promptCache == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prompt cache not initialized"})
		return
	}

	result := promptCache.WarmCache(req.Prompts)
	c.JSON(http.StatusOK, WarmPromptCacheResponse{
		Status:  "ok",
		Total:   result.Total,
		Added:   result.Added,
		Skipped: result.Skipped,
		Errors:  result.Errors,
	})
}
