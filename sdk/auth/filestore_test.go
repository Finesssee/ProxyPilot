package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestFileTokenStoreSavePersistsRoutingFields(t *testing.T) {
	t.Parallel()

	store := NewFileTokenStore()
	baseDir := t.TempDir()
	store.SetBaseDir(baseDir)

	auth := &cliproxyauth.Auth{
		ID:       "gemini-test.json",
		FileName: "gemini-test.json",
		Provider: "gemini",
		Priority: 7,
		Prefix:   "team-a",
		ProxyURL: "http://127.0.0.1:8080",
		Metadata: map[string]any{
			"type":  "gemini",
			"email": "user@example.com",
		},
	}

	if _, err := store.Save(context.Background(), auth); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	items, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("List() len = %d, want 1", len(items))
	}
	got := items[0]
	if got.Priority != 7 {
		t.Fatalf("Priority = %d, want 7", got.Priority)
	}
	if got.Prefix != "team-a" {
		t.Fatalf("Prefix = %q", got.Prefix)
	}
	if got.ProxyURL != "http://127.0.0.1:8080" {
		t.Fatalf("ProxyURL = %q", got.ProxyURL)
	}
	if got.Attributes["priority"] != "7" {
		t.Fatalf("Attributes[priority] = %q", got.Attributes["priority"])
	}
}

func TestFileTokenStoreSavePersistsSecurePermissions(t *testing.T) {
	t.Parallel()

	store := NewFileTokenStore()
	baseDir := t.TempDir()
	store.SetBaseDir(baseDir)

	auth := &cliproxyauth.Auth{
		ID:       "claude-test.json",
		FileName: "claude-test.json",
		Provider: "claude",
		Metadata: map[string]any{"type": "claude"},
	}

	path, err := store.Save(context.Background(), auth)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode = %v, want %v", got, os.FileMode(0o600))
	}
	if filepath.Dir(path) != baseDir {
		t.Fatalf("path = %q, want under %q", path, baseDir)
	}
}
