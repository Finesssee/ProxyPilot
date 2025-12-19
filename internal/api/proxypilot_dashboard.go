package api

import (
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	ppassets "github.com/router-for-me/CLIProxyAPI/v6/cmd/proxypilotui/assets"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
)

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
}

func (s *Server) serveProxyPilotDashboard(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	index, err := fsReadFile("index.html")
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	html := string(index)
	key := strings.TrimSpace(getManagementKey())
	if key != "" && s.managementRoutesEnabled.Load() {
		meta := `<meta name="pp-mgmt-key" content="` + escapeAttr(key) + `">`
		html = strings.Replace(html, "</head>", meta+"</head>", 1)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func (s *Server) serveProxyPilotAsset(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	fp := strings.TrimPrefix(c.Param("filepath"), "/")
	if fp == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	name := path.Clean("assets/" + fp)
	data, err := fsReadFile(name)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	writeAssetResponse(c, name, data)
}

func (s *Server) serveProxyPilotViteIcon(c *gin.Context) {
	if !isLocalClient(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	data, err := fsReadFile("vite.svg")
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	writeAssetResponse(c, "vite.svg", data)
}

func writeAssetResponse(c *gin.Context, name string, data []byte) {
	ext := strings.TrimPrefix(path.Ext(name), ".")
	if mt := misc.MimeTypes[strings.ToLower(ext)]; mt != "" {
		c.Header("Content-Type", mt)
	} else {
		c.Header("Content-Type", "application/octet-stream")
	}
	c.Header("Cache-Control", "public, max-age=600")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(data)
}

func fsReadFile(name string) ([]byte, error) {
	f, err := ppassets.FS.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func isLocalClient(c *gin.Context) bool {
	clientIP := c.ClientIP()
	return clientIP == "127.0.0.1" || clientIP == "::1"
}

func getManagementKey() string {
	return strings.TrimSpace(os.Getenv("MANAGEMENT_PASSWORD"))
}

func escapeAttr(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), `"`, "")
}
