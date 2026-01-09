package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type noopFlusher struct{}

func (noopFlusher) Flush() {}

func benchmarkForwardStream(b *testing.B, chunkSize, chunks int) {
	gin.SetMode(gin.ReleaseMode)
	payload := bytes.Repeat([]byte("a"), chunkSize)
	h := &BaseAPIHandler{Cfg: &sdkconfig.SDKConfig{}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		data := make(chan []byte, chunks)
		for j := 0; j < chunks; j++ {
			data <- payload
		}
		close(data)

		errs := make(chan *interfaces.ErrorMessage)
		h.ForwardStream(c, noopFlusher{}, func(error) {}, data, errs, StreamForwardOptions{
			WriteChunk: func(chunk []byte) {
				_, _ = c.Writer.Write(chunk)
			},
		})
	}
}

func BenchmarkForwardStreamSmall(b *testing.B) {
	benchmarkForwardStream(b, 256, 128)
}

func BenchmarkForwardStreamLarge(b *testing.B) {
	benchmarkForwardStream(b, 4096, 256)
}
