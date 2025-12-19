package openai

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// Embeddings handles the OpenAI-compatible /v1/embeddings endpoint.
//
// Many IDE clients (including Cursor) expect an embeddings endpoint even when using
// chat models. CLIProxyAPI may not have a native embeddings backend for all providers,
// so this handler returns deterministic synthetic embeddings to keep clients functional.
func (h *OpenAIAPIHandler) Embeddings(c *gin.Context) {
	rawJSON, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: fmt.Sprintf("Invalid request: %v", err),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	root := gjson.ParseBytes(rawJSON)
	model := strings.TrimSpace(root.Get("model").String())
	if model == "" {
		model = "text-embedding-3-small"
	}

	dims := int(root.Get("dimensions").Int())
	if dims <= 0 {
		dims = defaultEmbeddingDimensions(model)
	}

	inputs, ok := parseEmbeddingInputs(root.Get("input"))
	if !ok || len(inputs) == 0 {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Invalid request: input must be a non-empty string or array of strings",
				Type:    "invalid_request_error",
			},
		})
		return
	}

	alt := h.GetAlt(c)
	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	defer cliCancel()

	resp, errMsg := h.ExecuteEmbedWithAuthManager(cliCtx, h.HandlerType(), model, rawJSON, alt)
	if errMsg == nil && len(resp) > 0 {
		c.Header("Content-Type", "application/json")
		_, _ = c.Writer.Write(resp)
		return
	}

	// Fallback to synthetic embeddings if real execution fails or is not supported by the provider.
	// This keeps IDE features working even when a native embeddings backend is not available.
	log.Debugf("Embeddings execution failed or not supported, falling back to synthetic: %v", errMsg)

	data := make([]map[string]any, 0, len(inputs))
	promptTokens := 0
	for i, input := range inputs {
		promptTokens += approximateTokens(input)
		data = append(data, map[string]any{
			"object":    "embedding",
			"index":     i,
			"embedding": syntheticEmbedding(model, input, dims),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
		"model":  model,
		"usage": gin.H{
			"prompt_tokens": promptTokens,
			"total_tokens":  promptTokens,
		},
	})
}

func defaultEmbeddingDimensions(model string) int {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "text-embedding-3-large":
		return 3072
	case "text-embedding-3-small":
		return 1536
	default:
		// Keep a widely compatible default.
		return 1536
	}
}

func parseEmbeddingInputs(value gjson.Result) ([]string, bool) {
	switch value.Type {
	case gjson.String:
		return []string{value.String()}, true
	case gjson.JSON:
		// gjson.JSON can represent arrays/objects. We only accept arrays of strings.
		if !value.IsArray() {
			return nil, false
		}
		out := make([]string, 0, len(value.Array()))
		ok := true
		value.ForEach(func(_, v gjson.Result) bool {
			if v.Type != gjson.String {
				ok = false
				return false
			}
			out = append(out, v.String())
			return true
		})
		return out, ok
	default:
		return nil, false
	}
}

func approximateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	// Rough approximation: 1 token ~ 4 chars for English-like text.
	// This is only used for the "usage" object to satisfy clients.
	return (len([]rune(trimmed)) + 3) / 4
}

func syntheticEmbedding(model, input string, dims int) []float64 {
	if dims <= 0 {
		dims = 1536
	}

	out := make([]float64, dims)

	seed := []byte(model + "\n" + input)
	var buf [32]byte
	buf = sha256.Sum256(seed)

	for i := 0; i < dims; i++ {
		// Rehash every 8 floats to cheaply expand entropy.
		if i%8 == 0 && i != 0 {
			buf = sha256.Sum256(buf[:])
		}
		u := binary.LittleEndian.Uint32(buf[(i%8)*4 : (i%8+1)*4])
		// Map uint32 to [-1, 1].
		out[i] = (float64(u)/4294967295.0)*2.0 - 1.0
	}

	return out
}
