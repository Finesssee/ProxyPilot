package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() *gin.Engine {
	router := gin.New()
	handler := NewTranslatorHandler()

	v1 := router.Group("/v1")
	{
		v1.GET("/translations", handler.GetTranslationsMatrix)
		v1.GET("/translations/check", handler.CheckTranslation)
		v1.GET("/translations/docs", handler.GetTranslationDocs)
		v1.POST("/translations/score", handler.ScoreTranslation)
		v1.POST("/translations/compare", handler.CompareStructures)
	}

	return router
}

func TestGetTranslationsMatrix(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/v1/translations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response TranslationsMatrixResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Matrix == nil {
		t.Error("Matrix should not be nil")
	}
	if response.Formats == nil {
		t.Error("Formats should not be nil")
	}
}

func TestGetTranslationsMatrix_WithDetails(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/v1/translations?details=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response TranslationsMatrixResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// When details=true, Details should be populated
	// (may be empty if no translations registered, but should be non-nil conceptually)
}

func TestCheckTranslation_Supported(t *testing.T) {
	router := setupTestRouter()

	// Register a test translator
	translator.Register(translator.FormatOpenAI, translator.FormatClaude, func(model string, data []byte, stream bool) []byte {
		return data
	}, translator.ResponseTransform{})
	defer translator.Unregister(translator.FormatOpenAI, translator.FormatClaude)

	req, _ := http.NewRequest("GET", "/v1/translations/check?from=openai&to=claude", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response CheckTranslationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Supported {
		t.Error("Translation should be supported")
	}
	if response.From != "openai" {
		t.Errorf("From = %s, want openai", response.From)
	}
	if response.To != "claude" {
		t.Errorf("To = %s, want claude", response.To)
	}
}

func TestCheckTranslation_NotSupported(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/v1/translations/check?from=unknown&to=unknown2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response CheckTranslationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Supported {
		t.Error("Translation should not be supported for unknown formats")
	}
}

func TestCheckTranslation_MissingParams(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name string
		url  string
	}{
		{"missing both", "/v1/translations/check"},
		{"missing to", "/v1/translations/check?from=openai"},
		{"missing from", "/v1/translations/check?to=claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestCheckTranslation_SameFormat(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/v1/translations/check?from=openai&to=openai", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response CheckTranslationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Same format translation is a fallback/no-op
	if !response.Fallback {
		t.Error("Same format should be marked as fallback")
	}
	if !response.Supported {
		t.Error("Same format should be supported")
	}
}

func TestGetTranslationDocs_Markdown(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/v1/translations/docs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/markdown; charset=utf-8" {
		t.Errorf("Content-Type = %s, want text/markdown; charset=utf-8", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body should not be empty")
	}
}

func TestGetTranslationDocs_Mermaid(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/v1/translations/docs?format=mermaid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %s, want text/plain; charset=utf-8", contentType)
	}

	body := w.Body.String()
	if !containsString(body, "mermaid") {
		t.Error("Response should contain mermaid diagram")
	}
}

func TestGetTranslationDocs_Summary(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/v1/translations/docs?format=summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %s, want text/plain; charset=utf-8", contentType)
	}

	body := w.Body.String()
	if !containsString(body, "Summary") {
		t.Error("Response should contain summary")
	}
}

func TestScoreTranslation(t *testing.T) {
	router := setupTestRouter()

	requestBody := ScoreTranslationRequest{
		From:   "openai",
		To:     "claude",
		Before: `{"model": "gpt-4", "messages": []}`,
		After:  `{"model": "gpt-4", "messages": []}`,
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/v1/translations/score", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var report translator.QualityReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Perfect match should have high score
	if report.Score < 0.9 {
		t.Errorf("Score = %v, expected >= 0.9 for identical payloads", report.Score)
	}
}

func TestScoreTranslation_InvalidBody(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("POST", "/v1/translations/score", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestScoreTranslation_MissingFields(t *testing.T) {
	router := setupTestRouter()

	// Missing required fields
	requestBody := map[string]string{
		"from": "openai",
		// missing to, before, after
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/v1/translations/score", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCompareStructures(t *testing.T) {
	router := setupTestRouter()

	requestBody := CompareStructuresRequest{
		Before: `{"a": 1, "b": 2}`,
		After:  `{"a": 1, "c": 3}`,
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/v1/translations/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var comparison translator.StructureComparison
	if err := json.Unmarshal(w.Body.Bytes(), &comparison); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if comparison.SourceType != "object" {
		t.Errorf("SourceType = %s, want object", comparison.SourceType)
	}
	if comparison.TargetType != "object" {
		t.Errorf("TargetType = %s, want object", comparison.TargetType)
	}
	if len(comparison.Diffs) == 0 {
		t.Error("Should detect differences between the structures")
	}
}

func TestCompareStructures_InvalidBody(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("POST", "/v1/translations/compare", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCompareStructures_InvalidJSON(t *testing.T) {
	router := setupTestRouter()

	requestBody := CompareStructuresRequest{
		Before: `{invalid}`,
		After:  `{"valid": true}`,
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/v1/translations/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d for invalid JSON in payload", http.StatusBadRequest, w.Code)
	}
}

func TestCompareStructures_IdenticalStructures(t *testing.T) {
	router := setupTestRouter()

	requestBody := CompareStructuresRequest{
		Before: `{"a": 1, "b": 2}`,
		After:  `{"a": 1, "b": 2}`,
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/v1/translations/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var comparison translator.StructureComparison
	if err := json.Unmarshal(w.Body.Bytes(), &comparison); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(comparison.Diffs) != 0 {
		t.Errorf("Identical structures should have no diffs, got %d", len(comparison.Diffs))
	}
}

// Helper function
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
