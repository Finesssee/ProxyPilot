package management

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestImportIFlowCredential_NoConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{cfg: nil}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.ImportIFlowCredential(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "config unavailable")
}

func TestImportIFlowCredential_NoAuthDir(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{cfg: &config.Config{}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.ImportIFlowCredential(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "auth directory not configured")
}

func TestImportIFlowCredential_NoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{cfg: &config.Config{AuthDir: "/tmp"}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("POST", "/import", nil)
	c.Request = req

	h.ImportIFlowCredential(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file required")
}

func TestImportIFlowCredential_EmptyFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{cfg: &config.Config{AuthDir: "/tmp"}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte(""))
	writer.Close()

	req, _ := http.NewRequest("POST", "/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req

	h.ImportIFlowCredential(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "no valid cookies found")
}

func TestParseCookieLines(t *testing.T) {
	content := `
# Comment
cookie1

cookie2
`
	reader := bytes.NewReader([]byte(content))
	cookies, err := parseCookieLines(reader)

	assert.NoError(t, err)
	assert.Len(t, cookies, 2)
	assert.Equal(t, "cookie1", cookies[0])
	assert.Equal(t, "cookie2", cookies[1])
}
