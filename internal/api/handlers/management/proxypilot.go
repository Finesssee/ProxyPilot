package management

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
)

// GetProxyPilotLogTail returns tail lines for the launcher-managed stdout/stderr logs.
// These files are written by ProxyPilot when it spawns the server binary.
func (h *Handler) GetProxyPilotLogTail(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}

	kind := strings.ToLower(strings.TrimSpace(c.Query("file")))
	lines := 200
	if v := strings.TrimSpace(c.Query("lines")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2000 {
			lines = n
		}
	}

	var name string
	switch kind {
	case "stdout", "out":
		name = "cliproxyapi.out.log"
	case "stderr", "err":
		name = "cliproxyapi.err.log"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file (use stdout|stderr)"})
		return
	}

	dir := h.logDirectory()
	full := filepath.Join(dir, name)
	content, truncated, err := tailTextFile(full, lines, 512*1024)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"lines": []string{}, "truncated": false})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read log: %v", err)})
		return
	}
	outLines := splitLines(content)
	if len(outLines) > lines {
		outLines = outLines[len(outLines)-lines:]
		truncated = true
	}
	c.JSON(http.StatusOK, gin.H{"lines": outLines, "truncated": truncated})
}

// GetProxyPilotDiagnostics returns a single text blob suitable for copy/paste.
func (h *Handler) GetProxyPilotDiagnostics(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	lines := 120
	if v := strings.TrimSpace(c.Query("lines")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			lines = n
		}
	}

	var b strings.Builder
	b.WriteString("ProxyPilot Diagnostics\n")
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Built: version=%s commit=%s date=%s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate))
	if h.cfg != nil {
		b.WriteString(fmt.Sprintf("Port: %d\n", h.cfg.Port))
		b.WriteString(fmt.Sprintf("AuthDir: %s\n", strings.TrimSpace(h.cfg.AuthDir)))
		b.WriteString(fmt.Sprintf("RequestLog: %v\n", h.cfg.RequestLog))
		b.WriteString(fmt.Sprintf("Debug: %v\n", h.cfg.Debug))
	}
	if strings.TrimSpace(h.configFilePath) != "" {
		b.WriteString(fmt.Sprintf("ConfigPath: %s\n", h.configFilePath))
	}
	logDir := h.logDirectory()
	b.WriteString(fmt.Sprintf("LogDir: %s\n", logDir))
	b.WriteString(fmt.Sprintf("CapturedAt: %s\n", time.Now().Format(time.RFC3339)))
	b.WriteString("\n")

	addTail := func(label, fileName string) {
		full := filepath.Join(logDir, fileName)
		content, _, err := tailTextFile(full, lines, 512*1024)
		b.WriteString("=== " + label + " (" + fileName + ") ===\n")
		if err != nil {
			if os.IsNotExist(err) {
				b.WriteString("(file not found)\n\n")
				return
			}
			b.WriteString("(error reading log: " + err.Error() + ")\n\n")
			return
		}
		b.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	addTail("Launcher Stdout", "cliproxyapi.out.log")
	addTail("Launcher Stderr", "cliproxyapi.err.log")

	c.JSON(http.StatusOK, gin.H{"text": b.String()})
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

func tailTextFile(path string, wantLines int, maxBytes int64) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", false, err
	}
	size := info.Size()
	if size <= 0 {
		return "", false, nil
	}

	readBytes := maxBytes
	if readBytes <= 0 {
		readBytes = 256 * 1024
	}
	if readBytes > size {
		readBytes = size
	}

	start := size - readBytes
	if _, err := f.Seek(start, 0); err != nil {
		return "", false, err
	}
	buf := make([]byte, readBytes)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return "", false, err
	}
	buf = buf[:n]

	// Drop partial first line when we started mid-file.
	if start > 0 {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 && i+1 < len(buf) {
			buf = buf[i+1:]
		}
	}
	text := string(buf)
	// Best-effort: return enough lines; caller will truncate if needed.
	truncated := start > 0
	if wantLines <= 0 {
		return text, truncated, nil
	}
	return text, truncated, nil
}
