// Package handlers provides core API handler functionality for the CLI Proxy API server.
// It includes common types, client management, load balancing, and error handling
// shared across all API endpoint handlers (OpenAI, Claude, Gemini).
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/routingdebug"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
	"golang.org/x/net/context"
)

// ErrorResponse represents a standard error response format for the API.
// It contains a single ErrorDetail field.
type ErrorResponse struct {
	// Error contains detailed information about the error that occurred.
	Error ErrorDetail `json:"error"`
}

// ErrorDetail provides specific information about an error that occurred.
// It includes a human-readable message, an error type, and an optional error code.
type ErrorDetail struct {
	// Message is a human-readable message providing more details about the error.
	Message string `json:"message"`

	// Type is the category of error that occurred (e.g., "invalid_request_error").
	Type string `json:"type"`

	// Code is a short code identifying the error, if applicable.
	Code string `json:"code,omitempty"`
}

// BaseAPIHandler contains the handlers for API endpoints.
// It holds a pool of clients to interact with the backend service and manages
// load balancing, client selection, and configuration.
type BaseAPIHandler struct {
	// AuthManager manages auth lifecycle and execution in the new architecture.
	AuthManager *coreauth.Manager

	// Cfg holds the current application configuration.
	Cfg *config.SDKConfig

	// OpenAICompatProviders is a list of provider names for OpenAI compatibility.
	OpenAICompatProviders []string
}

func (h *BaseAPIHandler) setProxyPilotRoutingHeaders(ctx context.Context, requestedModel, normalizedModel string, providers []string, trace *coreauth.SelectionTrace) {
	ginVal := ctx.Value("gin")
	c, ok := ginVal.(*gin.Context)
	if !ok || c == nil {
		return
	}

	// Only expose routing debug info to localhost clients (safe for Droid/Codex/etc).
	ip := strings.TrimSpace(c.ClientIP())
	if ip != "*********" && ip != "::1" && !strings.EqualFold(ip, "localhost") {
		return
	}

	trimmedRequested := strings.TrimSpace(requestedModel)
	trimmedNormalized := strings.TrimSpace(normalizedModel)
	if trimmedRequested != "" {
		c.Header("X-ProxyPilot-Requested-Model", trimmedRequested)
	}
	if trimmedNormalized != "" {
		c.Header("X-ProxyPilot-Resolved-Model", trimmedNormalized)
	}
	if len(providers) > 0 {
		c.Header("X-ProxyPilot-Provider-Candidates", strings.Join(providers, ","))
	}

	var provider string
	if trace != nil {
		provider, _, _, _ = trace.Snapshot()
		trimmedProvider := strings.TrimSpace(provider)
		if trimmedProvider != "" {
			c.Header("X-ProxyPilot-Resolved-Provider", trimmedProvider)
		}
	}

	// Record a recent routing entry for management/debug APIs.
	routingdebug.RecordFromContext(c, trimmedRequested, trimmedNormalized, providers, trace)
}

// NewBaseAPIHandlers creates a new API handlers instance.
// It takes a slice of clients and configuration as input.
//
// Parameters:
//   - cliClients: A slice of AI service clients
//   - cfg: The application configuration
//
// Returns:
//   - *BaseAPIHandler: A new API handlers instance
func NewBaseAPIHandlers(cfg *config.SDKConfig, authManager *coreauth.Manager, openAICompatProviders []string) *BaseAPIHandler {
	return &BaseAPIHandler{
		Cfg:                   cfg,
		AuthManager:           authManager,
		OpenAICompatProviders: openAICompatProviders,
	}
}

// UpdateClients updates the handlers' client list and configuration.
// This method is called when the configuration or authentication tokens change.
//
// Parameters:
//   - clients: The new slice of AI service clients
//   - cfg: The new application configuration
func (h *BaseAPIHandler) UpdateClients(cfg *config.SDKConfig) { h.Cfg = cfg }

// GetAlt extracts the 'alt' parameter from the request query string.
// It checks both 'alt' and '$alt' parameters and returns the appropriate value.
//
// Parameters:
//   - c: The Gin context containing the HTTP request
//
// Returns:
//   - string: The alt parameter value, or empty string if it's "sse"
func (h *BaseAPIHandler) GetAlt(c *gin.Context) string {
	var alt string
	var hasAlt bool
	alt, hasAlt = c.GetQuery("alt")
	if !hasAlt {
		alt, _ = c.GetQuery("$alt")
	}
	if alt == "sse" {
		return ""
	}
	return alt
}

// GetContextWithCancel creates a new context with cancellation capabilities.
// It embeds the Gin context and the API handler into the new context for later use.
// The returned cancel function also handles logging the API response if request logging is enabled.
//
// Parameters:
//   - handler: The API handler associated with the request.
//   - c: The Gin context of the current request.
//   - ctx: The parent context.
//
// Returns:
//   - context.Context: The new context with cancellation and embedded values.
//   - APIHandlerCancelFunc: A function to cancel the context and log the response.
func (h *BaseAPIHandler) GetContextWithCancel(handler interfaces.APIHandler, c *gin.Context, ctx context.Context) (context.Context, APIHandlerCancelFunc) {
	newCtx, cancel := context.WithCancel(ctx)
	newCtx = context.WithValue(newCtx, "gin", c)
	newCtx = context.WithValue(newCtx, "handler", handler)
	return newCtx, func(params ...interface{}) {
		if h != nil && h.Cfg != nil && h.Cfg.RequestLog && len(params) == 1 {
			var payload []byte
			switch data := params[0].(type) {
			case []byte:
				payload = data
			case error:
				if data != nil {
					payload = []byte(data.Error())
				}
			case string:
				payload = []byte(data)
			}
			if len(payload) > 0 {
				if existing, exists := c.Get("API_RESPONSE"); exists {
					if existingBytes, ok := existing.([]byte); ok && len(existingBytes) > 0 {
						trimmedPayload := bytes.TrimSpace(payload)
						if len(trimmedPayload) > 0 && bytes.Contains(existingBytes, trimmedPayload) {
							cancel()
							return
						}
					}
				}
				appendAPIResponse(c, payload)
			}
		}

		cancel()
	}
}

// appendAPIResponse preserves any previously captured API response and appends new data.
func appendAPIResponse(c *gin.Context, data []byte) {
	if c == nil || len(data) == 0 {
		return
	}

	if existing, exists := c.Get("API_RESPONSE"); exists {
		if existingBytes, ok := existing.([]byte); ok && len(existingBytes) > 0 {
			combined := make([]byte, 0, len(existingBytes)+len(data)+1)
			combined = append(combined, existingBytes...)
			if existingBytes[len(existingBytes)-1] != '\n' {
				combined = append(combined, '\n')
			}
			combined = append(combined, data...)
			c.Set("API_RESPONSE", combined)
			return
		}
	}

	c.Set("API_RESPONSE", bytes.Clone(data))
}

// ExecuteWithAuthManager executes a non-streaming request via the core auth manager.
// This path is the only supported execution route.
func (h *BaseAPIHandler) ExecuteWithAuthManager(ctx context.Context, handlerType, modelName string, rawJSON []byte, alt string) ([]byte, *interfaces.ErrorMessage) {
	providers, normalizedModel, metadata, errMsg := h.getRequestDetails(modelName)
	if errMsg != nil {
		return nil, errMsg
	}
	trace := &coreauth.SelectionTrace{}
	ctx = coreauth.WithSelectionTrace(ctx, trace)
	req := coreexecutor.Request{
		Model:   normalizedModel,
		Payload: cloneBytes(rawJSON),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		req.Metadata = cloned
	} else {
		req.Metadata = make(map[string]any)
	}
	h.enrichMetadataFromContext(ctx, req.Metadata, rawJSON)

	opts := coreexecutor.Options{
		Stream:          false,
		Alt:             alt,
		OriginalRequest: cloneBytes(rawJSON),
		SourceFormat:    sdktranslator.FromString(handlerType),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		opts.Metadata = cloned
	}
	resp, err := h.AuthManager.Execute(ctx, providers, req, opts)
	if err != nil {
		status := http.StatusInternalServerError
		if se, ok := err.(interface{ StatusCode() int }); ok && se != nil {
			if code := se.StatusCode(); code > 0 {
				status = code
			}
		}
		var addon http.Header
		if he, ok := err.(interface{ Headers() http.Header }); ok && he != nil {
			if hdr := he.Headers(); hdr != nil {
				addon = hdr.Clone()
			}
		}
		h.maybeSetSelectionHeaders(ctx, trace)
		h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
		return nil, &interfaces.ErrorMessage{StatusCode: status, Error: err, Addon: addon}
	}
	h.maybeSetSelectionHeaders(ctx, trace)
	h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
	return cloneBytes(resp.Payload), nil
}

// ExecuteCountWithAuthManager executes a non-streaming request via the core auth manager.
// This path is the only supported execution route.
func (h *BaseAPIHandler) ExecuteCountWithAuthManager(ctx context.Context, handlerType, modelName string, rawJSON []byte, alt string) ([]byte, *interfaces.ErrorMessage) {
	providers, normalizedModel, metadata, errMsg := h.getRequestDetails(modelName)
	if errMsg != nil {
		return nil, errMsg
	}
	trace := &coreauth.SelectionTrace{}
	ctx = coreauth.WithSelectionTrace(ctx, trace)
	req := coreexecutor.Request{
		Model:   normalizedModel,
		Payload: cloneBytes(rawJSON),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		req.Metadata = cloned
	} else {
		req.Metadata = make(map[string]any)
	}
	h.enrichMetadataFromContext(ctx, req.Metadata, rawJSON)

	opts := coreexecutor.Options{
		Stream:          false,
		Alt:             alt,
		OriginalRequest: cloneBytes(rawJSON),
		SourceFormat:    sdktranslator.FromString(handlerType),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		opts.Metadata = cloned
	}
	resp, err := h.AuthManager.ExecuteCount(ctx, providers, req, opts)
	if err != nil {
		status := http.StatusInternalServerError
		if se, ok := err.(interface{ StatusCode() int }); ok && se != nil {
			if code := se.StatusCode(); code > 0 {
				status = code
			}
		}
		var addon http.Header
		if he, ok := err.(interface{ Headers() http.Header }); ok && he != nil {
			if hdr := he.Headers(); hdr != nil {
				addon = hdr.Clone()
			}
		}
		h.maybeSetSelectionHeaders(ctx, trace)
		h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
		return nil, &interfaces.ErrorMessage{StatusCode: status, Error: err, Addon: addon}
	}
	h.maybeSetSelectionHeaders(ctx, trace)
	h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
	return cloneBytes(resp.Payload), nil
}

// ExecuteEmbedWithAuthManager executes an embedding request via the core auth manager.
// This path is the only supported execution route.
func (h *BaseAPIHandler) ExecuteEmbedWithAuthManager(ctx context.Context, handlerType, modelName string, rawJSON []byte, alt string) ([]byte, *interfaces.ErrorMessage) {
	providers, normalizedModel, metadata, errMsg := h.getRequestDetails(modelName)
	if errMsg != nil {
		return nil, errMsg
	}
	trace := &coreauth.SelectionTrace{}
	ctx = coreauth.WithSelectionTrace(ctx, trace)
	req := coreexecutor.Request{
		Model:   normalizedModel,
		Payload: cloneBytes(rawJSON),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		req.Metadata = cloned
	} else {
		req.Metadata = make(map[string]any)
	}
	h.enrichMetadataFromContext(ctx, req.Metadata, rawJSON)

	opts := coreexecutor.Options{
		Stream:          false,
		Alt:             alt,
		OriginalRequest: cloneBytes(rawJSON),
		SourceFormat:    sdktranslator.FromString(handlerType),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		opts.Metadata = cloned
	}
	resp, err := h.AuthManager.ExecuteEmbed(ctx, providers, req, opts)
	if err != nil {
		status := http.StatusInternalServerError
		if se, ok := err.(interface{ StatusCode() int }); ok && se != nil {
			if code := se.StatusCode(); code > 0 {
				status = code
			}
		}
		var addon http.Header
		if he, ok := err.(interface{ Headers() http.Header }); ok && he != nil {
			if hdr := he.Headers(); hdr != nil {
				addon = hdr.Clone()
			}
		}
		h.maybeSetSelectionHeaders(ctx, trace)
		h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
		return nil, &interfaces.ErrorMessage{StatusCode: status, Error: err, Addon: addon}
	}
	h.maybeSetSelectionHeaders(ctx, trace)
	h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
	return cloneBytes(resp.Payload), nil
}

// ExecuteStreamWithAuthManager executes a streaming request via the core auth manager.
// This path is the only supported execution route.
func (h *BaseAPIHandler) ExecuteStreamWithAuthManager(ctx context.Context, handlerType, modelName string, rawJSON []byte, alt string) (<-chan []byte, <-chan *interfaces.ErrorMessage) {
	providers, normalizedModel, metadata, errMsg := h.getRequestDetails(modelName)
	if errMsg != nil {
		errChan := make(chan *interfaces.ErrorMessage, 1)
		errChan <- errMsg
		close(errChan)
		return nil, errChan
	}
	trace := &coreauth.SelectionTrace{}
	ctx = coreauth.WithSelectionTrace(ctx, trace)
	req := coreexecutor.Request{
		Model:   normalizedModel,
		Payload: cloneBytes(rawJSON),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		req.Metadata = cloned
	} else {
		req.Metadata = make(map[string]any)
	}
	h.enrichMetadataFromContext(ctx, req.Metadata, rawJSON)

	opts := coreexecutor.Options{
		Stream:          true,
		Alt:             alt,
		OriginalRequest: cloneBytes(rawJSON),
		SourceFormat:    sdktranslator.FromString(handlerType),
	}
	if cloned := cloneMetadata(metadata); cloned != nil {
		opts.Metadata = cloned
	}
	chunks, err := h.AuthManager.ExecuteStream(ctx, providers, req, opts)
	if err != nil {
		errChan := make(chan *interfaces.ErrorMessage, 1)
		status := http.StatusInternalServerError
		if se, ok := err.(interface{ StatusCode() int }); ok && se != nil {
			if code := se.StatusCode(); code > 0 {
				status = code
			}
		}
		var addon http.Header
		if he, ok := err.(interface{ Headers() http.Header }); ok && he != nil {
			if hdr := he.Headers(); hdr != nil {
				addon = hdr.Clone()
			}
		}
		h.maybeSetSelectionHeaders(ctx, trace)
		h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
		errChan <- &interfaces.ErrorMessage{StatusCode: status, Error: err, Addon: addon}
		close(errChan)
		return nil, errChan
	}
	h.maybeSetSelectionHeaders(ctx, trace)
	h.setProxyPilotRoutingHeaders(ctx, modelName, normalizedModel, providers, trace)
	dataChan := make(chan []byte)
	errChan := make(chan *interfaces.ErrorMessage, 1)
	go func() {
		defer close(dataChan)
		defer close(errChan)
		for chunk := range chunks {
			if chunk.Err != nil {
				status := http.StatusInternalServerError
				if se, ok := chunk.Err.(interface{ StatusCode() int }); ok && se != nil {
					if code := se.StatusCode(); code > 0 {
						status = code
					}
				}
				var addon http.Header
				if he, ok := chunk.Err.(interface{ Headers() http.Header }); ok && he != nil {
					if hdr := he.Headers(); hdr != nil {
						addon = hdr.Clone()
					}
				}
				errChan <- &interfaces.ErrorMessage{StatusCode: status, Error: chunk.Err, Addon: addon}
				return
			}
			if len(chunk.Payload) > 0 {
				dataChan <- cloneBytes(chunk.Payload)
			}
		}
	}()
	return dataChan, errChan
}

func (h *BaseAPIHandler) maybeSetSelectionHeaders(ctx context.Context, trace *coreauth.SelectionTrace) {
	if trace == nil {
		return
	}
	ginVal := ctx.Value("gin")
	c, ok := ginVal.(*gin.Context)
	if !ok || c == nil {
		return
	}
	if c.GetHeader("X-CLIProxyAPI-Debug-Auth") == "" {
		return
	}
	ip := strings.TrimSpace(c.ClientIP())
	if ip != "127.0.0.1" && ip != "::1" && !strings.EqualFold(ip, "localhost") {
		return
	}
	provider, authID, label, masked := trace.Snapshot()

	if provider != "" {
		c.Header("X-CLIProxyAPI-Selected-Provider", provider)
	}
	if authID != "" {
		c.Header("X-CLIProxyAPI-Selected-Auth", authID)
	}
	if label != "" {
		c.Header("X-CLIProxyAPI-Selected-Label", label)
	}
	if masked != "" {
		c.Header("X-CLIProxyAPI-Selected-Account", masked)
	}
}

func (h *BaseAPIHandler) getRequestDetails(modelName string) (providers []string, normalizedModel string, metadata map[string]any, err *interfaces.ErrorMessage) {
	// Droid CLI sometimes sends a "custom:*" model id; normalize it before routing.
	modelName = util.NormalizeDroidCustomModel(modelName)

	// Resolve "auto" model to an actual available model first
	resolvedModelName := util.ResolveAutoModel(modelName)

	providerName, extractedModelName, isDynamic := h.parseDynamicModel(resolvedModelName)

	targetModelName := resolvedModelName
	if isDynamic {
		targetModelName = extractedModelName
	}

	// Normalize the model name to handle dynamic thinking suffixes before determining the provider.
	normalizedModel, metadata = normalizeModelMetadata(targetModelName)

	if isDynamic {
		providers = []string{providerName}
	} else {
		// For non-dynamic models, use the normalizedModel to get the provider name.
		providers = util.GetProviderName(normalizedModel)
		if len(providers) == 0 && metadata != nil {
			if originalRaw, ok := metadata[util.ThinkingOriginalModelMetadataKey]; ok {
				if originalModel, okStr := originalRaw.(string); okStr {
					originalModel = strings.TrimSpace(originalModel)
					if originalModel != "" && !strings.EqualFold(originalModel, normalizedModel) {
						if altProviders := util.GetProviderName(originalModel); len(altProviders) > 0 {
							providers = altProviders
							normalizedModel = originalModel
						}
					}
				}
			}
		}
	}

	if len(providers) == 0 {
		return nil, "", nil, &interfaces.ErrorMessage{StatusCode: http.StatusBadRequest, Error: fmt.Errorf("unknown provider for model %s", modelName)}
	}

	// If it's a dynamic model, the normalizedModel was already set to extractedModelName.
	// If it's a non-dynamic model, normalizedModel was set by normalizeModelMetadata.
	// So, normalizedModel is already correctly set at this point.

	return providers, normalizedModel, metadata, nil
}

func (h *BaseAPIHandler) parseDynamicModel(modelName string) (providerName, model string, isDynamic bool) {
	var providerPart, modelPart string
	for _, sep := range []string{"://"} {
		if parts := strings.SplitN(modelName, sep, 2); len(parts) == 2 {
			providerPart = parts[0]
			modelPart = parts[1]
			break
		}
	}

	if providerPart == "" {
		return "", modelName, false
	}

	// Check if the provider is a configured openai-compatibility provider
	for _, pName := range h.OpenAICompatProviders {
		if pName == providerPart {
			return providerPart, modelPart, true
		}
	}

	return "", modelName, false
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func normalizeModelMetadata(modelName string) (string, map[string]any) {
	return util.NormalizeThinkingModel(modelName)
}

func cloneMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (h *BaseAPIHandler) enrichMetadataFromContext(ctx context.Context, metadata map[string]any, body []byte) {
	if metadata == nil {
		return
	}
	// Extract from Gin context headers
	ginVal := ctx.Value("gin")
	if c, ok := ginVal.(*gin.Context); ok && c != nil {
		if v := strings.TrimSpace(c.GetHeader("X-CLIProxyAPI-Session")); v != "" {
			metadata["X-CLIProxyAPI-Session"] = v
		}
		if v := strings.TrimSpace(c.GetHeader("X-Session-Id")); v != "" {
			metadata["X-Session-Id"] = v
		}
		if v := strings.TrimSpace(c.GetHeader("session_id")); v != "" {
			metadata["session_id"] = v
		}
	}

	// Fallback: check body if not found in headers
	if _, ok := metadata["X-CLIProxyAPI-Session"]; !ok {
		if _, ok2 := metadata["X-Session-Id"]; !ok2 {
			if _, ok3 := metadata["session_id"]; !ok3 {
				// Try extracting from body
				// Using gjson directly on the raw bytes to avoid unmarshalling overhead
				if v := gjson.GetBytes(body, "prompt_cache_key"); v.Exists() && v.String() != "" {
					metadata["session_id"] = v.String()
				} else if v := gjson.GetBytes(body, "metadata.session_id"); v.Exists() && v.String() != "" {
					metadata["session_id"] = v.String()
				} else if v := gjson.GetBytes(body, "session_id"); v.Exists() && v.String() != "" {
					metadata["session_id"] = v.String()
				}
			}
		}
	}
}

// WriteErrorResponse writes an error message to the response writer using the HTTP status embedded in the message.
func (h *BaseAPIHandler) WriteErrorResponse(c *gin.Context, msg *interfaces.ErrorMessage) {
	status := http.StatusInternalServerError
	if msg != nil && msg.StatusCode > 0 {
		status = msg.StatusCode
	}
	if msg != nil && msg.Addon != nil {
		for key, values := range msg.Addon {
			if len(values) == 0 {
				continue
			}
			c.Writer.Header().Del(key)
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
	}

	// Always return a JSON error envelope for OpenAI-compatible clients.
	// Plain-text bodies (or invalid JSON) commonly show up as "(no body)" in the OpenAI Node SDK.
	c.Header("Content-Type", "application/json")
	c.Status(status)

	message := http.StatusText(status)
	if msg != nil && msg.Error != nil {
		if s := strings.TrimSpace(msg.Error.Error()); s != "" {
			message = s
		}
	}

	errType := "invalid_request_error"
	if status >= 500 {
		errType = "server_error"
	}

	payload, _ := json.Marshal(ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    errType,
		},
	})
	if len(payload) == 0 {
		payload = []byte(`{"error":{"message":"unknown error","type":"server_error"}}`)
	}
	_, _ = c.Writer.Write(payload)
}

func (h *BaseAPIHandler) LoggingAPIResponseError(ctx context.Context, err *interfaces.ErrorMessage) {
	if h.Cfg.RequestLog {
		if ginContext, ok := ctx.Value("gin").(*gin.Context); ok {
			if apiResponseErrors, isExist := ginContext.Get("API_RESPONSE_ERROR"); isExist {
				if slicesAPIResponseError, isOk := apiResponseErrors.([]*interfaces.ErrorMessage); isOk {
					slicesAPIResponseError = append(slicesAPIResponseError, err)
					ginContext.Set("API_RESPONSE_ERROR", slicesAPIResponseError)
				}
			} else {
				// Create new response data entry
				ginContext.Set("API_RESPONSE_ERROR", []*interfaces.ErrorMessage{err})
			}
		}
	}
}

// APIHandlerCancelFunc is a function type for canceling an API handler's context.
// It can optionally accept parameters, which are used for logging the response.
type APIHandlerCancelFunc func(params ...interface{})
