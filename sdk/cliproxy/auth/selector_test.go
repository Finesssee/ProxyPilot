package auth

import (
	"context"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestRoundRobinSelector_Antigravity_StrictFallbackPrefersPrimaryFlag(t *testing.T) {
	selector := &RoundRobinSelector{}
	model := "gemini-3-pro-preview"

	primary := &Auth{
		ID:       "a-primary",
		Provider: "antigravity",
		Metadata: map[string]any{"primary": true},
	}
	backup := &Auth{
		ID:       "b-backup",
		Provider: "antigravity",
	}

	for i := 0; i < 5; i++ {
		picked, err := selector.Pick(context.Background(), "antigravity", model, cliproxyexecutor.Options{}, []*Auth{primary, backup})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if picked == nil || picked.ID != primary.ID {
			t.Fatalf("expected primary auth %q, got %#v", primary.ID, picked)
		}
	}
}

func TestRoundRobinSelector_Antigravity_StrictFallbackFallsBackWhenPrimaryBlocked(t *testing.T) {
	selector := &RoundRobinSelector{}
	model := "gemini-3-pro-preview"
	blockUntil := time.Now().Add(5 * time.Minute)

	primary := &Auth{
		ID:       "a-primary",
		Provider: "antigravity",
		Metadata: map[string]any{"primary": true},
		ModelStates: map[string]*ModelState{
			model: &ModelState{
				Unavailable:    true,
				NextRetryAfter: blockUntil,
				Quota:          QuotaState{Exceeded: true, NextRecoverAt: blockUntil},
				Status:         StatusActive,
				StatusMessage:  "rate limited",
				UpdatedAt:      time.Now(),
				LastError:      &Error{HTTPStatus: 429},
			},
		},
	}
	backup := &Auth{
		ID:       "b-backup",
		Provider: "antigravity",
	}

	picked, err := selector.Pick(context.Background(), "antigravity", model, cliproxyexecutor.Options{}, []*Auth{primary, backup})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked == nil || picked.ID != backup.ID {
		t.Fatalf("expected backup auth %q, got %#v", backup.ID, picked)
	}
}

func TestRoundRobinSelector_Antigravity_StrictFallbackUsesPrimaryEmailEnv(t *testing.T) {
	t.Setenv("CLIPROXY_ANTIGRAVITY_PRIMARY_EMAIL", "yuhh0704@gmail.com")

	selector := &RoundRobinSelector{}
	model := "gemini-3-pro-preview"

	primary := &Auth{
		ID:       "a-primary",
		Provider: "antigravity",
		Metadata: map[string]any{"email": "yuhh0704@gmail.com"},
	}
	backup := &Auth{
		ID:       "b-backup",
		Provider: "antigravity",
		Metadata: map[string]any{"email": "annq2366@gmail.com"},
	}

	picked, err := selector.Pick(context.Background(), "antigravity", model, cliproxyexecutor.Options{}, []*Auth{backup, primary})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked == nil || picked.ID != primary.ID {
		t.Fatalf("expected primary auth %q, got %#v", primary.ID, picked)
	}
}

func TestRoundRobinSelector_Antigravity_StrictFallbackUsesProjectIDHeuristic(t *testing.T) {
	selector := &RoundRobinSelector{}
	model := "gemini-3-pro-preview"

	primary := &Auth{
		ID:       "a-primary",
		Provider: "antigravity",
		Metadata: map[string]any{"project_id": "sunny-fold-7sf6z"},
	}
	backup := &Auth{
		ID:       "b-backup",
		Provider: "antigravity",
	}

	picked, err := selector.Pick(context.Background(), "antigravity", model, cliproxyexecutor.Options{}, []*Auth{backup, primary})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked == nil || picked.ID != primary.ID {
		t.Fatalf("expected primary auth %q, got %#v", primary.ID, picked)
	}
}
