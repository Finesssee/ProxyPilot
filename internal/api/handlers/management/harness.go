package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
)

// GetHarnessFiles returns the list of harness files for a session.
// GET /v0/management/harness/files?session=...
func (h *Handler) GetHarnessFiles(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusOK, gin.H{"files": []string{}})
		return
	}
	session := strings.TrimSpace(c.Query("session"))
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	store := memory.NewFileStore(base)
	files, err := store.ListHarnessFiles(session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if files == nil {
		files = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// GetHarnessFile returns the content of a specific harness file.
// GET /v0/management/harness/file?session=...&filename=...
func (h *Handler) GetHarnessFile(c *gin.Context) {
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
	filename := strings.TrimSpace(c.Query("filename"))
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filename"})
		return
	}
	store := memory.NewFileStore(base)
	content := store.ReadHarnessFile(session, filename, 100000)
	c.JSON(http.StatusOK, gin.H{"filename": filename, "content": content})
}

type putHarnessFileRequest struct {
	Session  string `json:"session"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// PutHarnessFile creates or updates a harness file.
// PUT /v0/management/harness/file
// Body: {"session": "...", "filename": "...", "content": "..."}
func (h *Handler) PutHarnessFile(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memory not configured"})
		return
	}
	var req putHarnessFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(req.Session) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	if strings.TrimSpace(req.Filename) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filename"})
		return
	}
	store := memory.NewFileStore(base)
	if err := store.WriteHarnessFile(req.Session, req.Filename, req.Content, 100000); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ExportHarness returns all harness files for a session as JSON.
// GET /v0/management/harness/export?session=...
func (h *Handler) ExportHarness(c *gin.Context) {
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
	store := memory.NewFileStore(base)
	files, err := store.ListHarnessFiles(session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make(map[string]string, len(files))
	for _, filename := range files {
		content := store.ReadHarnessFile(session, filename, 100000)
		result[filename] = content
	}
	c.JSON(http.StatusOK, gin.H{"session": session, "files": result})
}

type importHarnessRequest struct {
	Session string            `json:"session"`
	Files   map[string]string `json:"files"`
}

// ImportHarness imports harness files from JSON.
// POST /v0/management/harness/import
// Body: {"session": "...", "files": {"feature_list.json": "...", "claude-progress.txt": "..."}}
func (h *Handler) ImportHarness(c *gin.Context) {
	base := memoryBaseDir()
	if base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memory not configured"})
		return
	}
	var req importHarnessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(req.Session) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}
	if len(req.Files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing files"})
		return
	}
	store := memory.NewFileStore(base)
	var errors []string
	imported := 0
	for filename, content := range req.Files {
		if strings.TrimSpace(filename) == "" {
			continue
		}
		if err := store.WriteHarnessFile(req.Session, filename, content, 100000); err != nil {
			errors = append(errors, filename+": "+err.Error())
			continue
		}
		imported++
	}
	if len(errors) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":   "partial",
			"imported": imported,
			"errors":   errors,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "imported": imported})
}
