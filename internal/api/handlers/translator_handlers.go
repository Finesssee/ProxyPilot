// Package handlers provides HTTP handlers for the API server.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

// TranslatorHandler provides HTTP handlers for translation-related endpoints.
type TranslatorHandler struct{}

// NewTranslatorHandler creates a new translator handler.
func NewTranslatorHandler() *TranslatorHandler {
	return &TranslatorHandler{}
}

// TranslationsMatrixResponse represents the response for the translations matrix endpoint.
type TranslationsMatrixResponse struct {
	Matrix  map[string][]string         `json:"matrix"`
	Formats []string                    `json:"formats"`
	Total   int                         `json:"total_translations"`
	Details []translator.TranslationInfo `json:"details,omitempty"`
}

// GetTranslationsMatrix returns the full compatibility matrix of supported translations.
// GET /v1/translations
func (h *TranslatorHandler) GetTranslationsMatrix(c *gin.Context) {
	matrix := translator.GetCompatibilityMatrix()
	formats := translator.GetSupportedFormats()
	translations := translator.GetAllTranslations()

	// Convert formats to strings
	formatStrings := make([]string, len(formats))
	for i, f := range formats {
		formatStrings[i] = f.String()
	}

	// Count total translations
	total := 0
	for _, targets := range matrix {
		total += len(targets)
	}

	// Check if detailed info is requested
	includeDetails := c.Query("details") == "true"

	response := TranslationsMatrixResponse{
		Matrix:  matrix,
		Formats: formatStrings,
		Total:   total,
	}

	if includeDetails {
		response.Details = translations
	}

	c.JSON(http.StatusOK, response)
}

// CheckTranslationResponse represents the response for checking a specific translation.
type CheckTranslationResponse struct {
	Supported    bool                      `json:"supported"`
	Fallback     bool                      `json:"fallback"`
	From         string                    `json:"from"`
	To           string                    `json:"to"`
	Info         *translator.TranslationInfo `json:"info,omitempty"`
	Alternatives []string                  `json:"alternatives,omitempty"`
}

// CheckTranslation checks if a specific translation path is supported.
// GET /v1/translations/check?from=X&to=Y
func (h *TranslatorHandler) CheckTranslation(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")

	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing required query parameters",
			"details": "both 'from' and 'to' query parameters are required",
		})
		return
	}

	fromFormat := translator.FromString(from)
	toFormat := translator.FromString(to)

	supported := translator.IsTranslationSupported(fromFormat, toFormat)
	info := translator.GetTranslationInfo(fromFormat, toFormat)

	response := CheckTranslationResponse{
		Supported: supported,
		Fallback:  false,
		From:      from,
		To:        to,
	}

	// Include detailed info if translation exists
	if supported {
		response.Info = info
	} else {
		// Check for potential fallback paths or alternatives
		matrix := translator.GetCompatibilityMatrix()
		if targets, exists := matrix[from]; exists {
			response.Alternatives = targets
		}

		// Mark as fallback if the request would pass through unchanged
		// (same format translation is always supported as a no-op)
		if from == to {
			response.Fallback = true
			response.Supported = true
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetTranslationDocs returns markdown documentation for all translations.
// GET /v1/translations/docs
func (h *TranslatorHandler) GetTranslationDocs(c *gin.Context) {
	format := c.Query("format")

	switch format {
	case "mermaid":
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, translator.GenerateMermaidDiagram())
	case "summary":
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, translator.GenerateTranslationSummary())
	default:
		// Default to markdown
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.String(http.StatusOK, translator.GenerateMarkdownDocs())
	}
}

// ScoreTranslationRequest represents the request body for scoring a translation.
type ScoreTranslationRequest struct {
	From   string `json:"from" binding:"required"`
	To     string `json:"to" binding:"required"`
	Before string `json:"before" binding:"required"`
	After  string `json:"after" binding:"required"`
}

// ScoreTranslation analyzes the quality of a translation.
// POST /v1/translations/score
func (h *TranslatorHandler) ScoreTranslation(c *gin.Context) {
	var req ScoreTranslationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	fromFormat := translator.FromString(req.From)
	toFormat := translator.FromString(req.To)

	report := translator.ScoreTranslation(
		fromFormat,
		toFormat,
		[]byte(req.Before),
		[]byte(req.After),
	)

	c.JSON(http.StatusOK, report)
}

// CompareStructuresRequest represents the request body for comparing JSON structures.
type CompareStructuresRequest struct {
	Before string `json:"before" binding:"required"`
	After  string `json:"after" binding:"required"`
}

// CompareStructures provides detailed comparison of two JSON structures.
// POST /v1/translations/compare
func (h *TranslatorHandler) CompareStructures(c *gin.Context) {
	var req CompareStructuresRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	comparison, err := translator.CompareJSONStructures(
		[]byte(req.Before),
		[]byte(req.After),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "failed to compare structures",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, comparison)
}
