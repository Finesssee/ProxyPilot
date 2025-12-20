package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
)

func (h *Handler) GetSemanticHealth(c *gin.Context) {
	enabled := semanticEnabled()
	baseURL := semanticBaseURL()
	model := semanticModel()
	status := "disabled"
	version := ""
	errMsg := ""
	modelPresent := false
	latencyMs := int64(0)

	if enabled {
		status = "unreachable"
		client := &http.Client{Timeout: 4 * time.Second}
		req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
		resp, err := client.Do(req)
		if err != nil {
			errMsg = err.Error()
		} else {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var out struct {
					Models []struct {
						Name string `json:"name"`
					} `json:"models"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&out); err == nil {
					for _, m := range out.Models {
						if strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(model)) {
							modelPresent = true
							break
						}
					}
				}
				version = resp.Status
			} else {
				errMsg = resp.Status
			}
		}

		if modelPresent {
			start := time.Now()
			payload := map[string]any{"model": model, "prompt": "ping"}
			body, _ := json.Marshal(payload)
			embedReq, _ := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/embeddings", bytes.NewReader(body))
			embedReq.Header.Set("Content-Type", "application/json")
			embedResp, err := client.Do(embedReq)
			if err != nil {
				errMsg = err.Error()
			} else {
				defer embedResp.Body.Close()
				if embedResp.StatusCode >= 200 && embedResp.StatusCode < 300 {
					status = "ok"
					latencyMs = time.Since(start).Milliseconds()
				} else {
					errMsg = embedResp.Status
				}
			}
		} else if errMsg == "" {
			status = "model_missing"
		}
	}

	queueStats := memory.GetSemanticQueueStats()
	c.JSON(http.StatusOK, gin.H{
		"enabled":       enabled,
		"baseURL":       baseURL,
		"model":         model,
		"status":        status,
		"version":       version,
		"model_present": modelPresent,
		"latency_ms":    latencyMs,
		"error":         errMsg,
		"queue":         queueStats,
	})
}

func (h *Handler) ListSemanticNamespaces(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"namespaces": []any{}})
		return
	}
	limit := 200
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	store := memory.NewFileStore(base)
	namespaces, err := store.ListSemanticNamespaces(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(namespaces))
	for _, ns := range namespaces {
		out = append(out, gin.H{
			"key":         ns.Key,
			"label":       ns.Namespace,
			"updated_at":  ns.UpdatedAt.Format(time.RFC3339),
			"size_bytes":  ns.SizeBytes,
			"items_bytes": ns.ItemsBytes,
		})
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
