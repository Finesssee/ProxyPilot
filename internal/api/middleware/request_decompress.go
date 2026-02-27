package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequestDecompressionMiddleware transparently decompresses gzipped request bodies.
//
// Some clients (notably Factory Droid / OpenAI Node SDK) send requests with
// Content-Encoding: gzip. net/http does not automatically decode request bodies,
// so handlers that expect JSON will otherwise see compressed bytes and fail with
// confusing 400/no-body errors.
func RequestDecompressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		enc := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Encoding")))
		if enc == "" || !strings.Contains(enc, "gzip") {
			c.Next()
			return
		}

		// Defensive cap against gzip bombs. Real-world CLI requests are far below this.
		const maxDecompressedBytes = 128 << 20 // 128MiB

		gzr, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "invalid gzip request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		defer gzr.Close()

		decoded, err := io.ReadAll(io.LimitReader(gzr, maxDecompressedBytes+1))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "failed to decompress gzip request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		if int64(len(decoded)) > maxDecompressedBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": gin.H{
					"message": "decompressed request body too large",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(decoded))
		c.Request.ContentLength = int64(len(decoded))
		c.Request.Header.Del("Content-Encoding")
		c.Next()
	}
}
