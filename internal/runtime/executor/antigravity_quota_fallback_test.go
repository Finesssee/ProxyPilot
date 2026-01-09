package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestAntigravityExecutor_SwitchesProjectOn429(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	seenProjects := make([]string, 0, 4)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != antigravityGeneratePath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		project := gjson.GetBytes(body, "project").String()
		mu.Lock()
		seenProjects = append(seenProjects, project)
		mu.Unlock()

		if project == "p1" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":429,"message":"Resource exhausted"}}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		QuotaExceeded: config.QuotaExceeded{
			SwitchProject:      true,
			SwitchPreviewModel: false,
		},
	}
	ex := NewAntigravityExecutor(cfg)

	auth := &cliproxyauth.Auth{
		ID:       "a1",
		Provider: "antigravity",
		Status:   cliproxyauth.StatusActive,
		Metadata: map[string]any{
			"type":         "antigravity",
			"access_token": "test-token",
			"expired":      time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			"project_id":   "p1,p2",
			"base_url":     srv.URL,
		},
	}

	req := cliproxyexecutor.Request{
		Model:   "gemini-3-flash",
		Payload: []byte(`{"request":{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}}`),
	}

	resp, err := ex.Execute(context.Background(), auth, req, cliproxyexecutor.Options{
		Stream:       false,
		SourceFormat: sdktranslator.FromString("antigravity"),
	})
	if err != nil {
		t.Fatalf("Execute() err: %v", err)
	}
	if string(resp.Payload) != `{"ok":true}` {
		t.Fatalf("unexpected response payload: %s", string(resp.Payload))
	}

	mu.Lock()
	got := append([]string(nil), seenProjects...)
	mu.Unlock()
	if len(got) < 2 {
		t.Fatalf("expected at least 2 attempts, got %v", got)
	}
	if got[0] != "p1" || got[1] != "p2" {
		t.Fatalf("expected project switch p1->p2, got %v", got)
	}
}

