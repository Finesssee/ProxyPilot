package management

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	iflowauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/iflow"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// ImportCredential handles uploading a text file with credentials for a specific provider.
// Each credential is processed and saved as a separate auth record.
func (h *Handler) ImportCredential(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider required"})
		return
	}

	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	if h.cfg.AuthDir == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth directory not configured"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file", "details": err.Error()})
		return
	}
	defer file.Close()

	ctx := c.Request.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	lines, err := parseLines(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse file", "details": err.Error()})
		return
	}

	if len(lines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid credentials found in file"})
		return
	}

	var results []importResult
	successCount := 0

	for i, raw := range lines {
		lineNum := i + 1
		var result importResult

		switch provider {
		case "iflow":
			authSvc := iflowauth.NewIFlowAuth(h.cfg)
			result = h.processIFlowCookie(ctx, authSvc, raw, lineNum)
		case "openai", "claude", "gemini":
			// Generic API Key import
			result = h.processAPIKey(ctx, provider, raw, lineNum)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider for bulk import"})
			return
		}

		if result.Success {
			successCount++
		}
		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"provider":      provider,
		"total":         len(lines),
		"success_count": successCount,
		"failed_count":  len(lines) - successCount,
		"results":       results,
	})
}

// Re-using importResult from iflow_import.go if it's exported, otherwise redefined here for clarity in new file.
// Ideally, we'd refactor iflow_import.go to share types, but for now assuming it stays as is or we modify it.
// Since we are creating a new file, let's assume we might replace iflow_import with this generic one eventually.
// For this step, I will assume we are ADDING this handler alongside existing ones.

// GenericTokenStorage is a simple map wrapper to satisfy TokenStorage interface
type GenericTokenStorage map[string]string

func (s GenericTokenStorage) SaveTokenToFile(path string) error {
	return nil
}

// processAPIKey handles generic API key import
func (h *Handler) processAPIKey(ctx context.Context, provider, key string, lineNum int) importResult {
	key = strings.TrimSpace(key)
	if key == "" {
		return importResult{Line: lineNum, Success: false, Error: "empty key"}
	}

	// Basic duplicate check could be added here if needed

	timestamp := time.Now().Unix()
	// Use a shortened key hash or prefix for filename to avoid leaking credentials in filenames
	safeName := fmt.Sprintf("%s-%s", provider, key[:min(8, len(key))])
	fileName := fmt.Sprintf("%s-%d.json", safeName, timestamp)

	record := &coreauth.Auth{
		ID:       fileName,
		Provider: provider,
		FileName: fileName,
		Storage: GenericTokenStorage{
			"api_key": key,
		},
		Metadata: map[string]any{
			"imported_at": timestamp,
			"source":      "bulk_import",
		},
		Attributes: map[string]string{
			"api_key": key,
		},
	}

	savedPath, err := h.saveTokenRecord(ctx, record)
	if err != nil {
		return importResult{Line: lineNum, Success: false, Error: "failed to save credential"}
	}

	return importResult{Line: lineNum, Success: true, SavedPath: savedPath}
}

func parseLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
