package api

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/cmd/proxypilotui/assets"
)

// ppMgmtKeyRegex matches existing pp-mgmt-key meta tags to be replaced
var ppMgmtKeyRegex = regexp.MustCompile(`<meta\s+name=["']pp-mgmt-key["']\s+content=["'][^"']*["']\s*/?\s*>`)

func (s *Server) registerProxyPilotDashboardRoutes() {
	if s == nil || s.engine == nil {
		return
	}

	s.engine.GET("/proxypilot", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/proxypilot.html")
	})
	s.engine.GET("/proxypilot.html", s.serveProxyPilotDashboard)
	s.engine.GET("/assets/*filepath", s.serveProxyPilotAsset)
	s.engine.GET("/vite.svg", s.serveProxyPilotViteIcon)
	s.engine.GET("/logo.png", s.serveProxyPilotLogo)
}

func (s *Server) serveProxyPilotDashboard(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// Read index.html from embedded assets
	htmlBytes, err := fs.ReadFile(assets.FS, "index.html")
	if err != nil {
		// Fallback placeholder if assets not available
		html := `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>ProxyPilot</title>
</head>
<body>
<h1>ProxyPilot Dashboard</h1>
<p>Dashboard UI assets are not available.</p>
</body>
</html>`
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
		return
	}

	html := string(htmlBytes)

	// Use localPassword from the server instance (set via WithLocalManagementPassword)
	// Fall back to MANAGEMENT_PASSWORD env var for legacy subprocess mode
	key := strings.TrimSpace(s.localPassword)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("MANAGEMENT_PASSWORD"))
	}
	if key != "" && s.managementRoutesEnabled.Load() {
		newMeta := `<meta name="pp-mgmt-key" content="` + escapeAttr(key) + `">`
		// Replace existing pp-mgmt-key meta tag (e.g., test placeholder) with actual key
		if ppMgmtKeyRegex.MatchString(html) {
			html = ppMgmtKeyRegex.ReplaceAllString(html, newMeta)
		} else {
			// No existing tag found, add before </head>
			html = strings.Replace(html, "</head>", newMeta+"</head>", 1)
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func (s *Server) serveProxyPilotAsset(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// Serve from embedded assets FS
	filepath := c.Param("filepath")
	assetPath := path.Join("assets", filepath)

	data, err := fs.ReadFile(assets.FS, assetPath)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// Determine content type based on extension
	contentType := "application/octet-stream"
	switch {
	case strings.HasSuffix(filepath, ".js"):
		contentType = "application/javascript"
	case strings.HasSuffix(filepath, ".css"):
		contentType = "text/css"
	case strings.HasSuffix(filepath, ".svg"):
		contentType = "image/svg+xml"
	case strings.HasSuffix(filepath, ".png"):
		contentType = "image/png"
	case strings.HasSuffix(filepath, ".jpg"), strings.HasSuffix(filepath, ".jpeg"):
		contentType = "image/jpeg"
	case strings.HasSuffix(filepath, ".woff2"):
		contentType = "font/woff2"
	case strings.HasSuffix(filepath, ".woff"):
		contentType = "font/woff"
	}

	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, data)
}

func (s *Server) serveProxyPilotViteIcon(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	data, err := fs.ReadFile(assets.FS, "vite.svg")
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "image/svg+xml")
	c.Data(http.StatusOK, "image/svg+xml", data)
}

func (s *Server) serveProxyPilotLogo(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	data, err := fs.ReadFile(assets.FS, "logo.png")
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "image/png")
	c.Data(http.StatusOK, "image/png", data)
}

func isLocalClient(c *gin.Context) bool {
	clientIP := c.ClientIP()
	return clientIP == "127.0.0.1" || clientIP == "::1"
}

func escapeAttr(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), `"`, "")
}
