package management

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type authResetCooldownRequest struct {
	AuthID   string `json:"auth_id"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ResetAuthCooldown clears quota/cooldown flags so the next request can probe availability again.
// This is useful when an upstream 429 is suspected to be transient or misattributed.
func (h *Handler) ResetAuthCooldown(c *gin.Context) {
	var req authResetCooldownRequest
	_ = c.ShouldBindJSON(&req)
	req.AuthID = strings.TrimSpace(req.AuthID)
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.Model = strings.TrimSpace(req.Model)

	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth manager unavailable"})
		return
	}

	updated := 0
	now := time.Now()

	resetAuth := func(a *coreauth.Auth) {
		if a == nil {
			return
		}
		a.Unavailable = false
		a.NextRetryAfter = time.Time{}
		a.Quota = coreauth.QuotaState{}
		a.LastError = nil
		a.StatusMessage = ""
		a.UpdatedAt = now

		if len(a.ModelStates) == 0 {
			return
		}
		for modelID, st := range a.ModelStates {
			if req.Model != "" && modelID != req.Model {
				continue
			}
			if st == nil {
				continue
			}
			st.Unavailable = false
			st.NextRetryAfter = time.Time{}
			st.Quota = coreauth.QuotaState{}
			st.LastError = nil
			st.StatusMessage = ""
			st.Status = coreauth.StatusActive
			st.UpdatedAt = now

			registry.GetGlobalRegistry().ClearModelQuotaExceeded(a.ID, modelID)
			registry.GetGlobalRegistry().ResumeClientModel(a.ID, modelID)
		}
	}

	if req.AuthID != "" {
		if a, ok := h.authManager.GetByID(req.AuthID); ok && a != nil {
			resetAuth(a)
			_, _ = h.authManager.Update(c.Request.Context(), a)
			updated = 1
		}
		c.JSON(http.StatusOK, gin.H{"updated": updated})
		return
	}

	for _, a := range h.authManager.List() {
		if a == nil {
			continue
		}
		if req.Provider != "" && strings.ToLower(strings.TrimSpace(a.Provider)) != req.Provider {
			continue
		}
		resetAuth(a)
		_, _ = h.authManager.Update(c.Request.Context(), a)
		updated++
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

