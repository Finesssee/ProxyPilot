package auth

import (
	"context"
	"strings"
	"sync"
)

type selectionTraceKey struct{}

// SelectionTrace captures which provider/auth was selected for a request.
// It is intended for local debugging and should not be exposed to untrusted clients.
type SelectionTrace struct {
	mu sync.Mutex

	Provider string
	AuthID   string
	Label    string
	Email    string
}

func (t *SelectionTrace) Snapshot() (provider, authID, label, maskedAccount string) {
	if t == nil {
		return "", "", "", ""
	}
	t.mu.Lock()
	provider = strings.TrimSpace(t.Provider)
	authID = strings.TrimSpace(t.AuthID)
	label = strings.TrimSpace(t.Label)
	email := strings.TrimSpace(t.Email)
	t.mu.Unlock()

	maskedAccount = label
	if email != "" {
		at := strings.IndexByte(email, '@')
		if at > 1 && at < len(email)-1 {
			user := email[:at]
			domain := email[at+1:]
			maskedUser := user[:1] + "***"
			if len(user) >= 2 {
				maskedUser = user[:2] + "***"
			}
			maskedAccount = maskedUser + "@" + domain
		} else {
			maskedAccount = email
		}
	}
	return provider, authID, label, maskedAccount
}

func WithSelectionTrace(ctx context.Context, trace *SelectionTrace) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if trace == nil {
		return ctx
	}
	return context.WithValue(ctx, selectionTraceKey{}, trace)
}

func getSelectionTrace(ctx context.Context) *SelectionTrace {
	if ctx == nil {
		return nil
	}
	trace, _ := ctx.Value(selectionTraceKey{}).(*SelectionTrace)
	return trace
}

func recordSelection(ctx context.Context, provider string, a *Auth) {
	trace := getSelectionTrace(ctx)
	if trace == nil {
		return
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()

	trace.Provider = strings.TrimSpace(provider)
	if a != nil {
		trace.AuthID = a.ID
		trace.Label = strings.TrimSpace(a.Label)
		if a.Metadata != nil {
			if v, ok := a.Metadata["email"].(string); ok {
				trace.Email = strings.TrimSpace(v)
			}
		}
	}
}

func (t *SelectionTrace) MaskedAccount() string {
	if t == nil {
		return ""
	}
	_, _, _, masked := t.Snapshot()
	return masked
}
