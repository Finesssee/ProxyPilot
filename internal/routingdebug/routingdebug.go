package routingdebug

import (
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// Entry captures a single routed request for local debugging.
// It is intentionally small and stored only in memory.
type Entry struct {
	Timestamp          time.Time `json:"timestamp"`
	Method             string    `json:"method"`
	Path               string    `json:"path"`
	Client             string    `json:"client"`
	UserAgent          string    `json:"user_agent"`
	RequestedModel     string    `json:"requested_model"`
	ResolvedModel      string    `json:"resolved_model"`
	ProviderCandidates []string  `json:"provider_candidates"`
	SelectedProvider   string    `json:"selected_provider"`
	SelectedAuthID     string    `json:"selected_auth_id"`
	SelectedLabel      string    `json:"selected_label"`
	SelectedAccount    string    `json:"selected_account"`
}

const maxEntries = 64

var (
	mu      sync.RWMutex
	entries []Entry
)

// RecordFromContext appends a routing entry based on the Gin context and selection trace.
// It is safe to call even when ctx or trace are nil.
func RecordFromContext(c *gin.Context, requestedModel, resolvedModel string, providers []string, trace *coreauth.SelectionTrace) {
	if c == nil {
		return
	}

	clientIP := strings.TrimSpace(c.ClientIP())
	ua := c.GetHeader("User-Agent")
	method := ""
	path := ""
	if c.Request != nil {
		method = c.Request.Method
		path = c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
	}

	provider, authID, label, masked := trace.Snapshot()

	e := Entry{
		Timestamp:          time.Now().UTC(),
		Method:             method,
		Path:               path,
		Client:             clientIP,
		UserAgent:          ua,
		RequestedModel:     strings.TrimSpace(requestedModel),
		ResolvedModel:      strings.TrimSpace(resolvedModel),
		ProviderCandidates: append([]string(nil), providers...),
		SelectedProvider:   strings.TrimSpace(provider),
		SelectedAuthID:     strings.TrimSpace(authID),
		SelectedLabel:      strings.TrimSpace(label),
		SelectedAccount:    strings.TrimSpace(masked),
	}

	mu.Lock()
	defer mu.Unlock()

	// Append and trim to maxEntries (simple ring-buffer-like behaviour).
	entries = append(entries, e)
	if len(entries) > maxEntries {
		offset := len(entries) - maxEntries
		entries = append([]Entry(nil), entries[offset:]...)
	}
}

// Snapshot returns a copy of the recent routing entries, newest first.
func Snapshot() []Entry {
	mu.RLock()
	defer mu.RUnlock()

	if len(entries) == 0 {
		return nil
	}

	out := make([]Entry, len(entries))
	copy(out, entries)

	// Reverse in-place so that callers get newest-first ordering.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
