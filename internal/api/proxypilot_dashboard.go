package api

import (
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
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

	// Dashboard assets are not available - return a placeholder message
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
	// Assets not available
	c.AbortWithStatus(http.StatusNotFound)
}

func (s *Server) serveProxyPilotViteIcon(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	// Asset not available
	c.AbortWithStatus(http.StatusNotFound)
}

func (s *Server) serveProxyPilotLogo(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	// Asset not available
	c.AbortWithStatus(http.StatusNotFound)
}

func isLocalClient(c *gin.Context) bool {
	clientIP := c.ClientIP()
	return clientIP == "127.0.0.1" || clientIP == "::1"
}

func escapeAttr(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), `"`, "")
}
