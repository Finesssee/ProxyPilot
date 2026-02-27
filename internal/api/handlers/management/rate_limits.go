package management

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitEntry represents the rate limit status for a single credential.
type RateLimitEntry struct {
	AuthID        string                `json:"auth_id"`
	Provider      string                `json:"provider"`
	Label         string                `json:"label,omitempty"`
	Email         string                `json:"email,omitempty"`
	QuotaExceeded bool                  `json:"quota_exceeded"`
	QuotaReason   string                `json:"quota_reason,omitempty"`
	RecoverAt     time.Time             `json:"recover_at,omitempty"`
	RecoverIn     string                `json:"recover_in,omitempty"`
	BackoffLevel  int                   `json:"backoff_level,omitempty"`
	ModelLimits   []ModelRateLimitEntry `json:"model_limits,omitempty"`
}

// ModelRateLimitEntry represents the rate limit status for a specific model.
type ModelRateLimitEntry struct {
	Model         string    `json:"model"`
	QuotaExceeded bool      `json:"quota_exceeded"`
	QuotaReason   string    `json:"quota_reason,omitempty"`
	RecoverAt     time.Time `json:"recover_at,omitempty"`
	RecoverIn     string    `json:"recover_in,omitempty"`
	BackoffLevel  int       `json:"backoff_level,omitempty"`
}

// RateLimitsResponse contains overall rate limit status.
type RateLimitsResponse struct {
	TotalCredentials int              `json:"total_credentials"`
	CoolingDown      int              `json:"cooling_down"`
	Available        int              `json:"available"`
	Credentials      []RateLimitEntry `json:"credentials"`
}

// GetRateLimits returns the rate limit status for all credentials.
// GET /v0/management/rate-limits
func (h *Handler) GetRateLimits(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusOK, RateLimitsResponse{})
		return
	}

	auths := h.authManager.List()
	now := time.Now()

	resp := RateLimitsResponse{
		TotalCredentials: len(auths),
		Credentials:      make([]RateLimitEntry, 0, len(auths)),
	}

	for _, auth := range auths {
		entry := RateLimitEntry{
			AuthID:        auth.ID,
			Provider:      auth.Provider,
			Label:         auth.Label,
			QuotaExceeded: auth.Quota.Exceeded,
			QuotaReason:   auth.Quota.Reason,
			BackoffLevel:  auth.Quota.BackoffLevel,
		}

		// Extract email from metadata
		if auth.Metadata != nil {
			if email, ok := auth.Metadata["email"].(string); ok {
				entry.Email = email
			}
		}

		// Calculate recover time
		if !auth.Quota.NextRecoverAt.IsZero() {
			entry.RecoverAt = auth.Quota.NextRecoverAt
			if auth.Quota.NextRecoverAt.After(now) {
				entry.RecoverIn = auth.Quota.NextRecoverAt.Sub(now).Round(time.Second).String()
			}
		}

		// Per-model rate limits
		if len(auth.ModelStates) > 0 {
			entry.ModelLimits = make([]ModelRateLimitEntry, 0, len(auth.ModelStates))
			for model, state := range auth.ModelStates {
				if state.Quota.Exceeded || !state.Quota.NextRecoverAt.IsZero() {
					modelEntry := ModelRateLimitEntry{
						Model:         model,
						QuotaExceeded: state.Quota.Exceeded,
						QuotaReason:   state.Quota.Reason,
						BackoffLevel:  state.Quota.BackoffLevel,
					}
					if !state.Quota.NextRecoverAt.IsZero() {
						modelEntry.RecoverAt = state.Quota.NextRecoverAt
						if state.Quota.NextRecoverAt.After(now) {
							modelEntry.RecoverIn = state.Quota.NextRecoverAt.Sub(now).Round(time.Second).String()
						}
					}
					entry.ModelLimits = append(entry.ModelLimits, modelEntry)
				}
			}
		}

		// Count cooling down vs available
		isCooling := auth.Quota.Exceeded || (auth.Quota.NextRecoverAt.After(now))
		if isCooling {
			resp.CoolingDown++
		} else if !auth.Disabled && !auth.Unavailable {
			resp.Available++
		}

		resp.Credentials = append(resp.Credentials, entry)
	}

	c.JSON(http.StatusOK, resp)
}

// GetRateLimitsSummary returns a brief summary of rate limit status.
// GET /v0/management/rate-limits/summary
func (h *Handler) GetRateLimitsSummary(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"total":        0,
			"available":    0,
			"cooling_down": 0,
			"disabled":     0,
		})
		return
	}

	auths := h.authManager.List()
	now := time.Now()

	var available, coolingDown, disabled int
	var nextRecovery time.Time

	for _, auth := range auths {
		if auth.Disabled {
			disabled++
			continue
		}

		isCooling := auth.Quota.Exceeded || (auth.Quota.NextRecoverAt.After(now))
		if isCooling {
			coolingDown++
			if nextRecovery.IsZero() || auth.Quota.NextRecoverAt.Before(nextRecovery) {
				nextRecovery = auth.Quota.NextRecoverAt
			}
		} else if !auth.Unavailable {
			available++
		}
	}

	resp := gin.H{
		"total":        len(auths),
		"available":    available,
		"cooling_down": coolingDown,
		"disabled":     disabled,
	}

	if !nextRecovery.IsZero() && nextRecovery.After(now) {
		resp["next_recovery_at"] = nextRecovery
		resp["next_recovery_in"] = nextRecovery.Sub(now).Round(time.Second).String()
	}

	c.JSON(http.StatusOK, resp)
}
