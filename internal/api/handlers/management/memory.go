package management

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
)

func (h *Handler) ListMemorySessions(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"sessions": []any{}})
		return
	}
	limit := 50
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	store := memory.NewFileStore(base)
	sessions, err := store.ListSessions(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, gin.H{
			"key":                s.Key,
			"updated_at":         formatTime(s.UpdatedAt),
			"size_bytes":         s.SizeBytes,
			"events_bytes":       s.EventsBytes,
			"has_summary":        s.HasSummary,
			"has_todo":           s.HasTodo,
			"has_pinned":         s.HasPinned,
			"has_anchor_pending": s.HasAnchorPending,
		})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": out})
}

func (h *Handler) GetMemorySession(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"session": gin.H{}})
		return
	}
	session := strings.TrimSpace(c.Query("session"))
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	store := memory.NewFileStore(base)
	info, err := store.GetSessionInfo(session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := gin.H{
		"key":                info.Key,
		"updated_at":         formatTime(info.UpdatedAt),
		"size_bytes":         info.SizeBytes,
		"events_bytes":       info.EventsBytes,
		"has_summary":        info.HasSummary,
		"has_todo":           info.HasTodo,
		"has_pinned":         info.HasPinned,
		"has_anchor_pending": info.HasAnchorPending,
	}
	out["summary"] = store.ReadSummary(session, 14_000)
	out["todo"] = store.ReadTodo(session, 8_000)
	out["pinned"] = store.ReadPinned(session, 8_000)
	out["anchor_pending"] = store.ReadPendingAnchor(session, 6_000)
	c.JSON(http.StatusOK, gin.H{"session": out})
}

func (h *Handler) GetMemoryEvents(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"events": []any{}})
		return
	}
	session := strings.TrimSpace(c.Query("session"))
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	limit := 100
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	store := memory.NewFileStore(base)
	events, err := store.ReadEventTail(session, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(events))
	for _, e := range events {
		out = append(out, gin.H{
			"ts":   formatTime(e.TS),
			"kind": e.Kind,
			"role": e.Role,
			"type": e.Type,
			"text": e.Text,
		})
	}
	c.JSON(http.StatusOK, gin.H{"events": out})
}

func (h *Handler) GetMemoryAnchors(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"anchors": []any{}})
		return
	}
	session := strings.TrimSpace(c.Query("session"))
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	limit := 20
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	store := memory.NewFileStore(base)
	anchors, err := store.ReadAnchorTail(session, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(anchors))
	for _, a := range anchors {
		out = append(out, gin.H{
			"ts":      formatTime(a.TS),
			"summary": a.Summary,
		})
	}
	c.JSON(http.StatusOK, gin.H{"anchors": out})
}

type memoryUpdateRequest struct {
	Session string `json:"session"`
	Value   string `json:"value"`
}

func (h *Handler) PutMemoryTodo(c *gin.Context) {
	h.updateMemoryField(c, func(store *memory.FileStore, session string, value string) error {
		return store.WriteTodo(session, value, 8000)
	})
}

func (h *Handler) PutMemoryPinned(c *gin.Context) {
	h.updateMemoryField(c, func(store *memory.FileStore, session string, value string) error {
		return store.WritePinned(session, value, 8000)
	})
}

func (h *Handler) PutMemorySummary(c *gin.Context) {
	h.updateMemoryField(c, func(store *memory.FileStore, session string, value string) error {
		if anchorAppendOnlyEnabled() {
			return store.SetAnchorSummary(session, value, 14_000)
		}
		return store.WriteSummary(session, value, 14_000)
	})
}

func (h *Handler) DeleteMemorySession(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	session := strings.TrimSpace(c.Query("session"))
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	path := filepath.Join(base, "sessions", session)
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type memoryPruneRequest struct {
	MaxAgeDays           *int   `json:"max_age_days"`
	MaxSessions          *int   `json:"max_sessions"`
	MaxBytesPerSession   *int64 `json:"max_bytes_per_session"`
	MaxNamespaces        *int   `json:"max_namespaces"`
	MaxBytesPerNamespace *int64 `json:"max_bytes_per_namespace"`
}

func (h *Handler) PruneMemory(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	var req memoryPruneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	store := memory.NewFileStore(base)
	maxAge := 0
	maxSessions := 0
	maxBytesSession := int64(0)
	maxNamespaces := 0
	maxBytesNamespace := int64(0)
	if req.MaxAgeDays != nil {
		maxAge = *req.MaxAgeDays
	}
	if req.MaxSessions != nil {
		maxSessions = *req.MaxSessions
	}
	if req.MaxBytesPerSession != nil {
		maxBytesSession = *req.MaxBytesPerSession
	}
	if req.MaxNamespaces != nil {
		maxNamespaces = *req.MaxNamespaces
	}
	if req.MaxBytesPerNamespace != nil {
		maxBytesNamespace = *req.MaxBytesPerNamespace
	}
	sres, err := store.PruneSessions(maxAge, maxSessions, maxBytesSession)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	semres, err := store.PruneSemantic(maxAge, maxNamespaces, maxBytesNamespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := memory.PruneResult{
		SessionsRemoved:           sres.SessionsRemoved,
		SessionsTrimmed:           sres.SessionsTrimmed,
		SemanticNamespacesRemoved: semres.SemanticNamespacesRemoved,
		SemanticNamespacesTrimmed: semres.SemanticNamespacesTrimmed,
		BytesFreed:                sres.BytesFreed + semres.BytesFreed,
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "result": out})
}

func (h *Handler) ExportMemorySession(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memory not configured"})
		return
	}
	session := strings.TrimSpace(c.Query("session"))
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	dir := filepath.Join(base, "sessions", session)
	if _, err := os.Stat(dir); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	data, err := memory.ExportSessionZip(dir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename=\"proxypilot-session-"+session+".zip\"")
	c.Data(http.StatusOK, "application/zip", data)
}

func (h *Handler) ImportMemorySession(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memory not configured"})
		return
	}
	session := strings.TrimSpace(c.Query("session"))
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	replace := strings.EqualFold(strings.TrimSpace(c.Query("replace")), "true")
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
	data, err := io.ReadAll(io.LimitReader(file, 50*1024*1024))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file", "details": err.Error()})
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid zip"})
		return
	}
	dest := filepath.Join(base, "sessions", session)
	if replace {
		_ = os.RemoveAll(dest)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(f.Name)
		name = strings.TrimPrefix(name, "/")
		if strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
		_ = rc.Close()
		if err != nil {
			continue
		}
		_ = os.WriteFile(target, b, 0o644)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) updateMemoryField(c *gin.Context, fn func(store *memory.FileStore, session string, value string) error) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memory not configured"})
		return
	}
	var req memoryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Session) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	store := memory.NewFileStore(base)
	if err := fn(store, req.Session, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func anchorAppendOnlyEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("CLIPROXY_ANCHOR_APPEND_ONLY")); v != "" {
		if strings.EqualFold(v, "0") || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") || strings.EqualFold(v, "no") {
			return false
		}
	}
	return true
}
