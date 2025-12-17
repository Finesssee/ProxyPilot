package openai

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func TestEmbeddings_SingleInput_DefaultDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &OpenAIAPIHandler{}
	r.POST("/v1/embeddings", h.Embeddings)

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBufferString(`{"model":"text-embedding-3-small","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.Bytes()
	if got := gjson.GetBytes(body, "data.0.embedding.#").Int(); got != 1536 {
		t.Fatalf("expected embedding length 1536, got %d", got)
	}
}

func TestEmbeddings_ArrayInput_CustomDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &OpenAIAPIHandler{}
	r.POST("/embeddings", h.Embeddings)

	req := httptest.NewRequest(http.MethodPost, "/embeddings", bytes.NewBufferString(`{"model":"text-embedding-3-large","dimensions":64,"input":["a","b"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.Bytes()
	if got := gjson.GetBytes(body, "data.#").Int(); got != 2 {
		t.Fatalf("expected 2 embeddings, got %d", got)
	}
	if got := gjson.GetBytes(body, "data.1.embedding.#").Int(); got != 64 {
		t.Fatalf("expected embedding length 64, got %d", got)
	}
}
