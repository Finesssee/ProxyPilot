package management

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

func (h *Handler) GetSemanticHealth(c *gin.Context) {
	enabled := semanticEnabled()
	baseURL := semanticBaseURL()
	model := semanticModel()
	status := "disabled"
	version := ""
	errMsg := ""

	if enabled {
		status = "unreachable"
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/version", nil)
		resp, err := client.Do(req)
		if err != nil {
			errMsg = err.Error()
		} else {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				status = "ok"
				var out map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&out); err == nil {
					if v, ok := out["version"].(string); ok {
						version = v
					}
				}
			} else {
				errMsg = resp.Status
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": enabled,
		"baseURL": baseURL,
		"model":   model,
		"status":  status,
		"version": version,
		"error":   errMsg,
	})
}

func (h *Handler) ListSemanticNamespaces(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"namespaces": []any{}})
		return
	}
	semanticDir := filepath.Join(base, "semantic")
	entries, err := os.ReadDir(semanticDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"namespaces": []any{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		key := e.Name()
		nsPath := filepath.Join(semanticDir, key, "namespace.txt")
		label := ""
		if data, err := os.ReadFile(nsPath); err == nil {
			label = strings.TrimSpace(string(data))
		}
		out = append(out, gin.H{"key": key, "label": label})
	}

	c.JSON(http.StatusOK, gin.H{"namespaces": out})
}

func (h *Handler) GetSemanticItems(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}
	key := strings.TrimSpace(c.Query("namespace"))
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing namespace"})
		return
	}
	limit := 50
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	store := memory.NewFileStore(base)
	items, err := store.ReadSemanticTail(key, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"ts":      it.TS.Format(time.RFC3339),
			"role":    it.Role,
			"text":    it.Text,
			"source":  it.Source,
			"session": it.Session,
			"repo":    it.Repo,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func memoryBaseDir() string {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_DIR")); v != "" {
		return v
	}
	if w := util.WritablePath(); w != "" {
		return filepath.Join(w, ".proxypilot", "memory")
	}
	return filepath.Join(".proxypilot", "memory")
}

func semanticEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_ENABLED")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
	}
	return true
}

func semanticModel() string {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_MODEL")); v != "" {
		return v
	}
	return "embeddinggemma"
}

func semanticBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_SEMANTIC_BASE_URL")); v != "" {
		return v
	}
	return "http://127.0.0.1:11434"
}
