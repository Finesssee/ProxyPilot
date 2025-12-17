package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

// RoundRobinSelector provides a simple provider scoped round-robin selection strategy.
type RoundRobinSelector struct {
	mu      sync.Mutex
	cursors map[string]int
}

type blockReason int

const (
	blockReasonNone blockReason = iota
	blockReasonCooldown
	blockReasonDisabled
	blockReasonOther
)

type modelCooldownError struct {
	model    string
	resetIn  time.Duration
	provider string
}

func newModelCooldownError(model, provider string, resetIn time.Duration) *modelCooldownError {
	if resetIn < 0 {
		resetIn = 0
	}
	return &modelCooldownError{
		model:    model,
		provider: provider,
		resetIn:  resetIn,
	}
}

func (e *modelCooldownError) Error() string {
	modelName := e.model
	if modelName == "" {
		modelName = "requested model"
	}
	message := fmt.Sprintf("All credentials for model %s are cooling down", modelName)
	if e.provider != "" {
		message = fmt.Sprintf("%s via provider %s", message, e.provider)
	}
	resetSeconds := int(math.Ceil(e.resetIn.Seconds()))
	if resetSeconds < 0 {
		resetSeconds = 0
	}
	displayDuration := e.resetIn
	if displayDuration > 0 && displayDuration < time.Second {
		displayDuration = time.Second
	} else {
		displayDuration = displayDuration.Round(time.Second)
	}
	errorBody := map[string]any{
		"code":          "model_cooldown",
		"message":       message,
		"model":         e.model,
		"reset_time":    displayDuration.String(),
		"reset_seconds": resetSeconds,
	}
	if e.provider != "" {
		errorBody["provider"] = e.provider
	}
	payload := map[string]any{"error": errorBody}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"error":{"code":"model_cooldown","message":"%s"}}`, message)
	}
	return string(data)
}

func (e *modelCooldownError) StatusCode() int {
	return http.StatusTooManyRequests
}

func (e *modelCooldownError) Headers() http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	resetSeconds := int(math.Ceil(e.resetIn.Seconds()))
	if resetSeconds < 0 {
		resetSeconds = 0
	}
	headers.Set("Retry-After", strconv.Itoa(resetSeconds))
	return headers
}

type authBlockedError struct {
	model        string
	provider     string
	resetIn      time.Duration
	cooldown     int
	disabled     int
	other        int
	lastStatuses map[int]int
}

func (e *authBlockedError) Error() string {
	modelName := e.model
	if modelName == "" {
		modelName = "requested model"
	}
	message := fmt.Sprintf("All credentials for model %s are temporarily unavailable", modelName)
	if e.provider != "" {
		message = fmt.Sprintf("%s via provider %s", message, e.provider)
	}
	resetSeconds := int(math.Ceil(e.resetIn.Seconds()))
	if resetSeconds < 0 {
		resetSeconds = 0
	}
	displayDuration := e.resetIn
	if displayDuration > 0 && displayDuration < time.Second {
		displayDuration = time.Second
	} else {
		displayDuration = displayDuration.Round(time.Second)
	}

	errorBody := map[string]any{
		"code":          "auth_unavailable",
		"message":       message,
		"model":         e.model,
		"reset_time":    displayDuration.String(),
		"reset_seconds": resetSeconds,
		"blocked": map[string]any{
			"cooldown": e.cooldown,
			"disabled": e.disabled,
			"other":    e.other,
		},
	}
	if e.provider != "" {
		errorBody["provider"] = e.provider
	}
	if len(e.lastStatuses) > 0 {
		statuses := make(map[string]int, len(e.lastStatuses))
		for k, v := range e.lastStatuses {
			if k <= 0 || v <= 0 {
				continue
			}
			statuses[strconv.Itoa(k)] = v
		}
		if len(statuses) > 0 {
			errorBody["last_http_statuses"] = statuses
		}
	}
	payload := map[string]any{"error": errorBody}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"error":{"code":"auth_unavailable","message":"%s"}}`, message)
	}
	return string(data)
}

func (e *authBlockedError) StatusCode() int {
	// When credentials are temporarily blocked (non-quota reasons), report 503 with Retry-After.
	return http.StatusServiceUnavailable
}

func (e *authBlockedError) Headers() http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	resetSeconds := int(math.Ceil(e.resetIn.Seconds()))
	if resetSeconds < 0 {
		resetSeconds = 0
	}
	if resetSeconds > 0 {
		headers.Set("Retry-After", strconv.Itoa(resetSeconds))
	}
	return headers
}

var antigravityPrimaryEmailOverride atomic.Value // string

// SetAntigravityPrimaryEmail configures the strict fallback primary account for antigravity.
// When non-empty, it takes precedence over the CLIPROXY_ANTIGRAVITY_PRIMARY_EMAIL env var.
func SetAntigravityPrimaryEmail(email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		antigravityPrimaryEmailOverride.Store("")
		return
	}
	antigravityPrimaryEmailOverride.Store(email)
}

func antigravityPrimaryEmail() string {
	if v, ok := antigravityPrimaryEmailOverride.Load().(string); ok {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			return v
		}
	}
	// Optional override: CLIPROXY_ANTIGRAVITY_PRIMARY_EMAIL=primary@example.com
	return strings.ToLower(strings.TrimSpace(os.Getenv("CLIPROXY_ANTIGRAVITY_PRIMARY_EMAIL")))
}

func authEmail(auth *Auth) string {
	if auth == nil || auth.Metadata == nil {
		return ""
	}
	if v, ok := auth.Metadata["email"].(string); ok {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return ""
}

func authProjectID(auth *Auth) string {
	if auth == nil || auth.Metadata == nil {
		return ""
	}
	if v, ok := auth.Metadata["project_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func isPrimaryAntigravityAuth(auth *Auth, primaryEmail string) bool {
	if auth == nil {
		return false
	}
	// Explicit primary flag wins.
	if auth.Metadata != nil {
		if v, ok := auth.Metadata["primary"].(bool); ok && v {
			return true
		}
	}
	if primaryEmail == "" {
		return false
	}
	// Match by email (preferred) or by label (fallback).
	if authEmail(auth) == primaryEmail {
		return true
	}
	if strings.ToLower(strings.TrimSpace(auth.Label)) == primaryEmail {
		return true
	}
	return false
}

// Pick selects the next available auth for the provider in a round-robin manner.
func (s *RoundRobinSelector) Pick(ctx context.Context, provider, model string, opts cliproxyexecutor.Options, auths []*Auth) (*Auth, error) {
	_ = ctx
	_ = opts
	if len(auths) == 0 {
		return nil, &Error{Code: "auth_not_found", Message: "no auth candidates"}
	}
	if s.cursors == nil {
		s.cursors = make(map[string]int)
	}
	available := make([]*Auth, 0, len(auths))
	now := time.Now()
	cooldownCount := 0
	disabledCount := 0
	otherCount := 0
	var earliestCooldown time.Time
	var earliestBlocked time.Time
	statusCounts := make(map[int]int)
	for i := 0; i < len(auths); i++ {
		candidate := auths[i]
		blocked, reason, next := isAuthBlockedForModel(candidate, model, now)
		if !blocked {
			available = append(available, candidate)
			continue
		}
		switch reason {
		case blockReasonCooldown:
			cooldownCount++
		case blockReasonDisabled:
			disabledCount++
		default:
			otherCount++
		}
		if candidate != nil && model != "" && candidate.ModelStates != nil {
			if st := candidate.ModelStates[model]; st != nil && st.LastError != nil && st.LastError.HTTPStatus > 0 {
				statusCounts[st.LastError.HTTPStatus]++
			}
		}
		if reason == blockReasonCooldown {
			if !next.IsZero() && (earliestCooldown.IsZero() || next.Before(earliestCooldown)) {
				earliestCooldown = next
			}
		}
		if !next.IsZero() && (earliestBlocked.IsZero() || next.Before(earliestBlocked)) {
			earliestBlocked = next
		}
	}
	if len(available) == 0 {
		if cooldownCount == len(auths) && !earliestCooldown.IsZero() {
			resetIn := earliestCooldown.Sub(now)
			if resetIn < 0 {
				resetIn = 0
			}
			return nil, newModelCooldownError(model, provider, resetIn)
		}
		if !earliestBlocked.IsZero() {
			resetIn := earliestBlocked.Sub(now)
			if resetIn < 0 {
				resetIn = 0
			}
			return nil, &authBlockedError{
				model:        model,
				provider:     provider,
				resetIn:      resetIn,
				cooldown:     cooldownCount,
				disabled:     disabledCount,
				other:        otherCount,
				lastStatuses: statusCounts,
			}
		}
		return nil, &Error{Code: "auth_unavailable", Message: "no auth available", HTTPStatus: http.StatusServiceUnavailable}
	}

	// Strict fallback for Antigravity: prefer a primary auth when it is available.
	// Motivation: keep one account as "primary usage" and only use the backup when primary is blocked.
	if strings.EqualFold(provider, "antigravity") && len(available) > 1 {
		primaryEmail := antigravityPrimaryEmail()
		var candidates []*Auth

		// 1) Explicit primary flag in auth metadata.
		for _, a := range available {
			if a == nil {
				continue
			}
			if a.Metadata != nil {
				if v, ok := a.Metadata["primary"].(bool); ok && v {
					candidates = append(candidates, a)
				}
			}
		}

		// 2) Match primary by email/label via env override.
		if len(candidates) == 0 && primaryEmail != "" {
			for _, a := range available {
				if isPrimaryAntigravityAuth(a, primaryEmail) {
					candidates = append(candidates, a)
				}
			}
		}

		// 3) Heuristic: if only one auth has a project_id, treat it as primary.
		if len(candidates) == 0 && primaryEmail == "" {
			var hasProjectID []*Auth
			for _, a := range available {
				if authProjectID(a) != "" {
					hasProjectID = append(hasProjectID, a)
				}
			}
			if len(hasProjectID) == 1 {
				candidates = hasProjectID
			}
		}

		// Enforce "strict" fallback by picking a single deterministic primary.
		if len(candidates) > 0 {
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
			available = candidates[:1]
		}
	}

	// Make round-robin deterministic even if caller's candidate order is unstable.
	if len(available) > 1 {
		sort.Slice(available, func(i, j int) bool { return available[i].ID < available[j].ID })
	}
	key := provider + ":" + model
	s.mu.Lock()
	index := s.cursors[key]

	if index >= 2_147_483_640 {
		index = 0
	}

	s.cursors[key] = index + 1
	s.mu.Unlock()
	// log.Debugf("available: %d, index: %d, key: %d", len(available), index, index%len(available))
	return available[index%len(available)], nil
}

func isAuthBlockedForModel(auth *Auth, model string, now time.Time) (bool, blockReason, time.Time) {
	if auth == nil {
		return true, blockReasonOther, time.Time{}
	}
	if auth.Disabled || auth.Status == StatusDisabled {
		return true, blockReasonDisabled, time.Time{}
	}
	if model != "" {
		if len(auth.ModelStates) > 0 {
			if state, ok := auth.ModelStates[model]; ok && state != nil {
				if state.Status == StatusDisabled {
					return true, blockReasonDisabled, time.Time{}
				}
				if state.Unavailable {
					if state.NextRetryAfter.IsZero() {
						return false, blockReasonNone, time.Time{}
					}
					if state.NextRetryAfter.After(now) {
						next := state.NextRetryAfter
						if !state.Quota.NextRecoverAt.IsZero() && state.Quota.NextRecoverAt.After(now) {
							next = state.Quota.NextRecoverAt
						}
						if next.Before(now) {
							next = now
						}
						if state.Quota.Exceeded {
							return true, blockReasonCooldown, next
						}
						return true, blockReasonOther, next
					}
				}
				return false, blockReasonNone, time.Time{}
			}
		}
		return false, blockReasonNone, time.Time{}
	}
	if auth.Unavailable && auth.NextRetryAfter.After(now) {
		next := auth.NextRetryAfter
		if !auth.Quota.NextRecoverAt.IsZero() && auth.Quota.NextRecoverAt.After(now) {
			next = auth.Quota.NextRecoverAt
		}
		if next.Before(now) {
			next = now
		}
		if auth.Quota.Exceeded {
			return true, blockReasonCooldown, next
		}
		return true, blockReasonOther, next
	}
	return false, blockReasonNone, time.Time{}
}
