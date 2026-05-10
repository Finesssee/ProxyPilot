package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestDeleteMemorySession_RejectsTraversal(t *testing.T) {
	t.Setenv("CLIPROXY_MEMORY_DIR", t.TempDir())
	gin.SetMode(gin.TestMode)

	outsidePath := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outsidePath, []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/v0/management/memory/session?session="+url.QueryEscape("../outside"), nil)

	h := &Handler{}
	h.DeleteMemorySession(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("outside path should remain untouched: %v", err)
	}
}

func TestProcessAPIKey_DoesNotLeakSecretPrefixInSavedPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, nil)

	key := "sk-secret-prefix-1234567890"
	result := h.processAPIKey(context.Background(), "openai", key, 1)
	if !result.Success {
		t.Fatalf("processAPIKey() failed: %+v", result)
	}
	if strings.Contains(result.SavedPath, "sk-secret") {
		t.Fatalf("saved path leaked key prefix: %q", result.SavedPath)
	}
	if !strings.HasPrefix(filepath.Base(result.SavedPath), "openai-import-") {
		t.Fatalf("saved path = %q, want generated import prefix", result.SavedPath)
	}
}

func TestCommitUploadedAuthFile_InvalidDataDoesNotWriteFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	path := filepath.Join(authDir, "broken.json")
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, nil)

	err := h.commitUploadedAuthFile(context.Background(), path, []byte(`{"type":`))
	if err == nil {
		t.Fatal("commitUploadedAuthFile() unexpectedly succeeded")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("invalid upload should not leave a file behind, stat err=%v", statErr)
	}
}

func TestGetConfig_ReturnsLiveConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandlerWithoutConfigFilePath(&config.Config{
		SDKConfig: config.SDKConfig{
			ProxyURL: "http://127.0.0.1:8080",
		},
		Debug:                  true,
		UsageStatisticsEnabled: true,
	}, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/config", nil)

	h.GetConfig(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var got config.Config
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v body=%s", err, recorder.Body.String())
	}
	if !got.Debug {
		t.Fatalf("Debug = %v, want true", got.Debug)
	}
	if got.ProxyURL != "http://127.0.0.1:8080" {
		t.Fatalf("ProxyURL = %q", got.ProxyURL)
	}
	if !got.UsageStatisticsEnabled {
		t.Fatalf("UsageStatisticsEnabled = %v, want true", got.UsageStatisticsEnabled)
	}
}
