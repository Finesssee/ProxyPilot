// Package openai provides HTTP handlers for OpenAIResponses API endpoints.
// This package implements the OpenAIResponses-compatible API interface, including model listing
// and chat completion functionality. It supports both streaming and non-streaming responses,
// and manages a pool of clients to interact with backend services.
// The handlers translate OpenAIResponses API requests to the appropriate backend format and
// convert responses back to OpenAIResponses-compatible format.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OpenAIResponsesAPIHandler contains the handlers for OpenAIResponses API endpoints.
// It holds a pool of clients to interact with the backend service.
type OpenAIResponsesAPIHandler struct {
	*handlers.BaseAPIHandler
}

// NewOpenAIResponsesAPIHandler creates a new OpenAIResponses API handlers instance.
// It takes an BaseAPIHandler instance as input and returns an OpenAIResponsesAPIHandler.
//
// Parameters:
//   - apiHandlers: The base API handlers instance
//
// Returns:
//   - *OpenAIResponsesAPIHandler: A new OpenAIResponses API handlers instance
func NewOpenAIResponsesAPIHandler(apiHandlers *handlers.BaseAPIHandler) *OpenAIResponsesAPIHandler {
	return &OpenAIResponsesAPIHandler{
		BaseAPIHandler: apiHandlers,
	}
}

// HandlerType returns the identifier for this handler implementation.
func (h *OpenAIResponsesAPIHandler) HandlerType() string {
	return OpenaiResponse
}

// Models returns the OpenAIResponses-compatible model metadata supported by this handler.
func (h *OpenAIResponsesAPIHandler) Models() []map[string]any {
	// Get dynamic models from the global registry
	modelRegistry := registry.GetGlobalRegistry()
	return modelRegistry.GetAvailableModels("openai")
}

// OpenAIResponsesModels handles the /v1/models endpoint.
// It returns a list of available AI models with their capabilities
// and specifications in OpenAIResponses-compatible format.
func (h *OpenAIResponsesAPIHandler) OpenAIResponsesModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   h.Models(),
	})
}

// Responses handles the /v1/responses endpoint.
// It determines whether the request is for a streaming or non-streaming response
// and calls the appropriate handler based on the model provider.
//
// Parameters:
//   - c: The Gin context containing the HTTP request and response
func (h *OpenAIResponsesAPIHandler) Responses(c *gin.Context) {
	rawJSON, err := c.GetRawData()
	// If data retrieval fails, return a 400 Bad Request error.
	if err != nil {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: fmt.Sprintf("Invalid request: %v", err),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	rawJSON = util.NormalizeOpenAIResponsesToolOrder(rawJSON)

	// Check if the client requested a streaming response.
	streamResult := gjson.GetBytes(rawJSON, "stream")
	wantsStream := streamResult.Type == gjson.True
	if wantsStream {
		ua := strings.ToLower(c.GetHeader("User-Agent"))
		isStainless := c.GetHeader("X-Stainless-Lang") != "" || c.GetHeader("X-Stainless-Package-Version") != ""
		// For strict JSON clients, respect Accept: application/json by switching to non-streaming.
		// Factory's droid CLI (Stainless) requests stream:true but still expects SSE even with Accept: application/json.
		if !(strings.Contains(ua, "factory-cli") || isStainless) {
			accept := strings.ToLower(c.GetHeader("Accept"))
			if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/event-stream") {
				wantsStream = false
			}
		}
	}

	if wantsStream {
		h.handleStreamingResponse(c, rawJSON)
	} else {
		h.handleNonStreamingResponse(c, rawJSON)
	}

}

// handleNonStreamingResponse handles non-streaming chat completion responses
// for Gemini models. It selects a client from the pool, sends the request, and
// aggregates the response before sending it back to the client in OpenAIResponses format.
//
// Parameters:
//   - c: The Gin context containing the HTTP request and response
//   - rawJSON: The raw JSON bytes of the OpenAIResponses-compatible request
func (h *OpenAIResponsesAPIHandler) handleNonStreamingResponse(c *gin.Context, rawJSON []byte) {
	c.Header("Content-Type", "application/json")

	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	defer func() {
		cliCancel()
	}()

	resp, errMsg := h.ExecuteWithAuthManager(cliCtx, h.HandlerType(), modelName, rawJSON, "")
	if errMsg != nil {
		h.WriteErrorResponse(c, errMsg)
		return
	}

	// Some clients assume output[0] is a message. Ensure messages come first for known agentic CLIs.
	resp = normalizeResponsesOutputOrder(c, resp)
	// Ensure `output_text` is present for clients that render it directly.
	resp = ensureResponsesOutputText(resp)

	_, _ = c.Writer.Write(resp)
	return

	// no legacy fallback

}

// handleStreamingResponse handles streaming responses for Gemini models.
// It establishes a streaming connection with the backend service and forwards
// the response chunks to the client in real-time using Server-Sent Events.
//
// Parameters:
//   - c: The Gin context containing the HTTP request and response
//   - rawJSON: The raw JSON bytes of the OpenAIResponses-compatible request
func (h *OpenAIResponsesAPIHandler) handleStreamingResponse(c *gin.Context, rawJSON []byte) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Get the http.Flusher interface to manually flush the response.
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	// For Factory's droid CLI (and other Stainless clients), upstream streaming can yield
	// only lifecycle events without output deltas on very large prompts. Prefer a non-streaming
	// upstream call and synthesize an SSE response that includes output_text deltas.
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	isStainless := c.GetHeader("X-Stainless-Lang") != "" || c.GetHeader("X-Stainless-Package-Version") != ""

	// New core execution path
	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	if strings.Contains(ua, "factory-cli") || isStainless {
		nonStreamReq := forceResponsesNonStreaming(rawJSON)
		resp, errMsg := h.ExecuteWithAuthManager(cliCtx, h.HandlerType(), modelName, nonStreamReq, "")
		if errMsg != nil {
			h.WriteErrorResponse(c, errMsg)
			flusher.Flush()
			cliCancel(errMsg.Error)
			return
		}
		if isLikelySSE(resp) {
			h.writeSSEBody(c, flusher, resp)
		} else {
			h.writeSyntheticResponsesSSE(c, flusher, resp)
		}
		cliCancel(nil)
		return
	}

	dataChan, errChan := h.ExecuteStreamWithAuthManager(cliCtx, h.HandlerType(), modelName, rawJSON, "")
	h.forwardResponsesStream(c, flusher, func(err error) { cliCancel(err) }, dataChan, errChan)
	return
}

func (h *OpenAIResponsesAPIHandler) forwardResponsesStream(c *gin.Context, flusher http.Flusher, cancel func(error), data <-chan []byte, errs <-chan *interfaces.ErrorMessage) {
	for {
		select {
		case <-c.Request.Context().Done():
			cancel(c.Request.Context().Err())
			return
		case chunk, ok := <-data:
			if !ok {
				_, _ = c.Writer.Write([]byte("\n"))
				flusher.Flush()
				cancel(nil)
				return
			}

			if bytes.HasPrefix(chunk, []byte("event:")) {
				_, _ = c.Writer.Write([]byte("\n"))
			}
			_, _ = c.Writer.Write(chunk)
			_, _ = c.Writer.Write([]byte("\n"))

			flusher.Flush()
		case errMsg, ok := <-errs:
			if !ok {
				continue
			}
			if errMsg != nil {
				h.WriteErrorResponse(c, errMsg)
				flusher.Flush()
			}
			var execErr error
			if errMsg != nil {
				execErr = errMsg.Error
			}
			cancel(execErr)
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func ensureResponsesOutputText(resp []byte) []byte {
	if gjson.GetBytes(resp, "output_text").Exists() {
		return resp
	}
	output := gjson.GetBytes(resp, "output")
	if !output.Exists() || !output.IsArray() {
		return resp
	}
	var b strings.Builder
	for _, item := range output.Array() {
		if item.Get("type").String() != "message" {
			continue
		}
		content := item.Get("content")
		if !content.Exists() || !content.IsArray() {
			continue
		}
		for _, part := range content.Array() {
			if part.Get("type").String() != "output_text" {
				continue
			}
			text := part.Get("text").String()
			if text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(text)
		}
	}
	if b.Len() == 0 {
		return resp
	}
	out, err := sjson.SetBytes(resp, "output_text", b.String())
	if err != nil {
		return resp
	}
	return out
}

func normalizeResponsesOutputOrder(c *gin.Context, resp []byte) []byte {
	output := gjson.GetBytes(resp, "output")
	if !output.Exists() || !output.IsArray() {
		return resp
	}

	ua := strings.ToLower(c.GetHeader("User-Agent"))
	isStainless := c.GetHeader("X-Stainless-Lang") != "" || c.GetHeader("X-Stainless-Package-Version") != ""
	if !strings.Contains(ua, "factory-cli") && !isStainless {
		return resp
	}

	var messages []any
	var others []any
	for _, item := range output.Array() {
		if item.Get("type").String() == "message" {
			messages = append(messages, item.Value())
		} else {
			others = append(others, item.Value())
		}
	}
	if len(messages) == 0 {
		return resp
	}
	merged := append(messages, others...)
	out, err := sjson.SetBytes(resp, "output", merged)
	if err != nil {
		return resp
	}
	return out
}

func isLikelySSE(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return bytes.HasPrefix(trimmed, []byte("event:")) || bytes.Contains(trimmed, []byte("\nevent:"))
}

func shouldEmitDoneSentinel(c *gin.Context) bool {
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	if strings.Contains(ua, "factory-cli") {
		return false
	}
	return true
}

func (h *OpenAIResponsesAPIHandler) writeSSEBody(c *gin.Context, flusher http.Flusher, body []byte) {
	_, _ = c.Writer.Write(body)
	if shouldEmitDoneSentinel(c) && !bytes.Contains(body, []byte("[DONE]")) {
		_, _ = c.Writer.Write([]byte("\n\ndata: [DONE]\n\n"))
	}
	flusher.Flush()
}

func forceResponsesNonStreaming(rawJSON []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(rawJSON, &obj); err != nil {
		return rawJSON
	}
	obj["stream"] = false
	delete(obj, "stream_options")
	out, err := json.Marshal(obj)
	if err != nil {
		return rawJSON
	}
	return out
}

func (h *OpenAIResponsesAPIHandler) writeSyntheticResponsesSSE(c *gin.Context, flusher http.Flusher, resp []byte) {
	id := gjson.GetBytes(resp, "id").String()
	if id == "" {
		id = gjson.GetBytes(resp, "response.id").String()
	}

	text := gjson.GetBytes(resp, `output.#(type=="message").content.0.text`).String()
	if text == "" {
		text = gjson.GetBytes(resp, `output.#(type=="message").content.#(type=="output_text").text`).String()
	}
	if text == "" {
		text = gjson.GetBytes(resp, "output_text").String()
	}

	nextSeq := 0
	emit := func(event string, data string) {
		_, _ = c.Writer.Write([]byte("event: " + event + "\n"))
		_, _ = c.Writer.Write([]byte("data: " + data + "\n\n"))
	}
	seq := func() int { nextSeq++; return nextSeq }

	emit("response.created", fmt.Sprintf("{\"type\":\"response.created\",\"sequence_number\":%d,\"response\":{\"id\":%q,\"object\":\"response\",\"status\":\"in_progress\",\"background\":false,\"error\":null}}", seq(), id))
	emit("response.in_progress", fmt.Sprintf("{\"type\":\"response.in_progress\",\"sequence_number\":%d,\"response\":{\"id\":%q,\"object\":\"response\",\"status\":\"in_progress\"}}", seq(), id))

	if text != "" {
		msgID := "msg_" + id + "_0"
		emit("response.output_item.added",
			fmt.Sprintf("{\"type\":\"response.output_item.added\",\"sequence_number\":%d,\"output_index\":0,\"item\":{\"id\":%q,\"type\":\"message\",\"status\":\"in_progress\",\"content\":[],\"role\":\"assistant\"}}",
				seq(), msgID))
		emit("response.content_part.added",
			fmt.Sprintf("{\"type\":\"response.content_part.added\",\"sequence_number\":%d,\"item_id\":%q,\"output_index\":0,\"content_index\":0,\"part\":{\"type\":\"output_text\",\"annotations\":[],\"logprobs\":[],\"text\":\"\"}}",
				seq(), msgID))
		emit("response.output_text.delta",
			fmt.Sprintf("{\"type\":\"response.output_text.delta\",\"sequence_number\":%d,\"item_id\":%q,\"output_index\":0,\"content_index\":0,\"delta\":%q,\"logprobs\":[]}",
				seq(), msgID, text))
		emit("response.output_text.done",
			fmt.Sprintf("{\"type\":\"response.output_text.done\",\"sequence_number\":%d,\"item_id\":%q,\"output_index\":0,\"content_index\":0,\"text\":%q,\"logprobs\":[]}",
				seq(), msgID, text))
		emit("response.output_item.done",
			fmt.Sprintf("{\"type\":\"response.output_item.done\",\"sequence_number\":%d,\"output_index\":0,\"item\":{\"id\":%q,\"type\":\"message\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"annotations\":[],\"logprobs\":[],\"text\":%q}],\"role\":\"assistant\"}}",
				seq(), msgID, text))
	}

	emit("response.completed", fmt.Sprintf("{\"type\":\"response.completed\",\"sequence_number\":%d,\"response\":{\"id\":%q,\"object\":\"response\",\"status\":\"completed\",\"error\":null}}", seq(), id))
	if shouldEmitDoneSentinel(c) {
		_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	}
	flusher.Flush()
}
