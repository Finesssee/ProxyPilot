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
	rawJSON = tightenToolSchemas(rawJSON, true)

	// Truncate input array if too long to prevent "Prompt is too long" errors
	modelName := gjson.GetBytes(rawJSON, "model").String()
	rawJSON = truncateResponsesInput(rawJSON, modelName)

	rawJSON = maybeCompactFactoryInput(c, rawJSON)
	rawJSON = maybeInjectFactoryInstructions(c, rawJSON)
	rawJSON = maybeInjectFactoryTools(c, rawJSON)

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
	resp = sanitizeToolCallArguments(resp, rawJSON, true)
	resp = convertToolCallTagsToResponsesFunctionCalls(resp)

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

	// Codex CLI: keep true streaming for normal turns, but for compaction/checkpoints and huge
	// requests prefer non-stream upstream + synthesized SSE, with a single self-healing retry.
	if isCodexCLIUserAgent(c.GetHeader("User-Agent")) {
		if reason := codexSynthReason(rawJSON); reason != "" {
			nonStreamReq := forceResponsesNonStreaming(rawJSON)
			nonStreamReq = tightenToolSchemas(nonStreamReq, true)

			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("X-CLIProxyAPI-Synthesized-Stream", "true")
			c.Header("X-CLIProxyAPI-Synth-Reason", reason)

			exec := func(m string, req []byte) ([]byte, *interfaces.ErrorMessage) {
				return h.ExecuteWithAuthManager(cliCtx, h.HandlerType(), m, req, "")
			}
			resp, retried, _, errMsg := codexNonStreamWithSingleRetry(modelName, nonStreamReq, exec)
			if retried {
				c.Header("X-CLIProxyAPI-Retry", "1")
			} else {
				c.Header("X-CLIProxyAPI-Retry", "0")
			}
			if errMsg != nil {
				h.writeSyntheticResponsesErrorSSE(c, flusher, errMsg)
				cliCancel(errMsg.Error)
				return
			}

			resp = sanitizeToolCallArguments(resp, nonStreamReq, true)
			resp = convertToolCallTagsToResponsesFunctionCalls(resp)
			resp = ensureResponsesOutputText(resp)
			if codexIsSilentMaxTokens(resp) {
				resp = setResponseOutputText(resp, codexSilentMaxTokensFallback)
			}

			if isLikelySSE(resp) {
				h.writeSSEBody(c, flusher, resp)
			} else {
				h.writeSyntheticResponsesSSE(c, flusher, resp)
			}
			cliCancel(nil)
			return
		}
	}

	if strings.Contains(ua, "factory-cli") || isStainless {
		nonStreamReq := forceResponsesNonStreaming(rawJSON)
		nonStreamReq = tightenToolSchemas(nonStreamReq, true)
		resp, errMsg := h.ExecuteWithAuthManager(cliCtx, h.HandlerType(), modelName, nonStreamReq, "")
		if errMsg != nil {
			// Factory/Stainless clients request stream:true and may not surface non-2xx JSON bodies well.
			// Emit a synthetic SSE response with an assistant-facing error message to avoid "no body".
			h.writeSyntheticResponsesErrorSSE(c, flusher, errMsg)
			cliCancel(errMsg.Error)
			return
		}
		resp = sanitizeToolCallArguments(resp, nonStreamReq, true)
		resp = convertToolCallTagsToResponsesFunctionCalls(resp)
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		if isLikelySSE(resp) {
			h.writeSSEBody(c, flusher, resp)
		} else {
			h.writeSyntheticResponsesSSE(c, flusher, resp)
		}
		cliCancel(nil)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	dataChan, errChan := h.ExecuteStreamWithAuthManager(cliCtx, h.HandlerType(), modelName, rawJSON, "")
	h.forwardResponsesStream(c, flusher, func(err error) { cliCancel(err) }, dataChan, errChan)
	return
}

func (h *OpenAIResponsesAPIHandler) writeSyntheticResponsesErrorSSE(c *gin.Context, flusher http.Flusher, errMsg *interfaces.ErrorMessage) {
	// Always emit SSE with HTTP 200 so strict streaming clients can display the message.
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	msg := "Request failed."
	statusCode := 0
	if errMsg != nil {
		statusCode = errMsg.StatusCode
		if errMsg.Error != nil && errMsg.Error.Error() != "" {
			raw := errMsg.Error.Error()
			// Try to extract a user-facing message from common upstream error shapes.
			if gjson.Valid(raw) {
				if v := gjson.Get(raw, "error.message"); v.Exists() && v.String() != "" {
					msg = v.String()
				} else if v := gjson.Get(raw, "error.error.message"); v.Exists() && v.String() != "" {
					msg = v.String()
				} else if v := gjson.Get(raw, "message"); v.Exists() && v.String() != "" {
					msg = v.String()
				} else {
					msg = raw
				}
			} else {
				msg = raw
			}
		}
	}
	if statusCode > 0 {
		msg = fmt.Sprintf("[HTTP %d] %s", statusCode, strings.TrimSpace(msg))
	}

	// Keep the message short; some clients stuff huge upstream JSON error blobs.
	const maxChars = 1800
	if len(msg) > maxChars {
		msg = msg[:maxChars] + "\n...[truncated]..."
	}

	resp := fmt.Sprintf(`{"id":"resp_error_%d","output_text":%q}`, time.Now().UnixNano(), msg)
	h.writeSyntheticResponsesSSE(c, flusher, []byte(resp))
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
				// If we haven't started streaming, prefer a JSON error payload (SSE errors are often dropped by clients).
				if !c.Writer.Written() {
					c.Header("Content-Type", "application/json")
					c.Writer.Header().Del("Cache-Control")
					c.Writer.Header().Del("Connection")
				}
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

func stripDoneSentinel(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	out := bytes.ReplaceAll(body, []byte("\r\ndata: [DONE]\r\n\r\n"), []byte("\r\n\r\n"))
	out = bytes.ReplaceAll(out, []byte("data: [DONE]\r\n\r\n"), []byte(""))
	out = bytes.ReplaceAll(out, []byte("\ndata: [DONE]\n\n"), []byte("\n\n"))
	out = bytes.ReplaceAll(out, []byte("data: [DONE]\n\n"), []byte(""))
	return out
}

func (h *OpenAIResponsesAPIHandler) writeSSEBody(c *gin.Context, flusher http.Flusher, body []byte) {
	if !shouldEmitDoneSentinel(c) {
		body = stripDoneSentinel(body)
	}
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

	nextSeq := 0
	emitRaw := func(event string, data string) {
		_, _ = c.Writer.Write([]byte("event: " + event + "\n"))
		_, _ = c.Writer.Write([]byte("data: " + data + "\n\n"))
	}
	seq := func() int { nextSeq++; return nextSeq }

	emitJSON := func(event string, payload any) {
		b, err := json.Marshal(payload)
		if err != nil {
			return
		}
		emitRaw(event, string(b))
	}

	emitJSON("response.created", map[string]any{
		"type":            "response.created",
		"sequence_number": seq(),
		"response": map[string]any{
			"id":         id,
			"object":     "response",
			"status":     "in_progress",
			"background": false,
			"error":      nil,
		},
	})
	emitJSON("response.in_progress", map[string]any{
		"type":            "response.in_progress",
		"sequence_number": seq(),
		"response": map[string]any{
			"id":     id,
			"object": "response",
			"status": "in_progress",
		},
	})

	output := gjson.GetBytes(resp, "output")
	if output.Exists() && output.IsArray() && len(output.Array()) > 0 {
		for i, item := range output.Array() {
			itemVal := item.Value()

			// Ensure a stable id/status for streaming-style events.
			itemID := item.Get("id").String()
			if itemID == "" {
				itemID = fmt.Sprintf("item_%s_%d", id, i)
				if updated, err := sjson.SetBytes([]byte(item.Raw), "id", itemID); err == nil {
					itemVal = gjson.ParseBytes(updated).Value()
				}
			}

			emitJSON("response.output_item.added", map[string]any{
				"type":            "response.output_item.added",
				"sequence_number": seq(),
				"output_index":    i,
				"item":            itemVal,
			})

			if item.Get("type").String() == "message" {
				content := item.Get("content")
				if content.Exists() && content.IsArray() {
					for contentIndex, part := range content.Array() {
						if part.Get("type").String() != "output_text" {
							continue
						}
						text := part.Get("text").String()
						if text == "" {
							continue
						}

						emitJSON("response.content_part.added", map[string]any{
							"type":            "response.content_part.added",
							"sequence_number": seq(),
							"item_id":         itemID,
							"output_index":    i,
							"content_index":   contentIndex,
							"part": map[string]any{
								"type":        "output_text",
								"annotations": []any{},
								"logprobs":    []any{},
								"text":        "",
							},
						})
						emitJSON("response.output_text.delta", map[string]any{
							"type":            "response.output_text.delta",
							"sequence_number": seq(),
							"item_id":         itemID,
							"output_index":    i,
							"content_index":   contentIndex,
							"delta":           text,
							"logprobs":        []any{},
						})
						emitJSON("response.output_text.done", map[string]any{
							"type":            "response.output_text.done",
							"sequence_number": seq(),
							"item_id":         itemID,
							"output_index":    i,
							"content_index":   contentIndex,
							"text":            text,
							"logprobs":        []any{},
						})
					}
				}
			} else if item.Get("type").String() == "function_call" || item.Get("type").String() == "tool_call" {
				// Some clients rely on argument deltas to trigger tool execution even when upstream is non-streaming.
				args := item.Get("arguments").String()
				if args == "" {
					args = item.Get("input").String()
				}
				if args != "" {
					emitJSON("response.function_call_arguments.delta", map[string]any{
						"type":            "response.function_call_arguments.delta",
						"sequence_number": seq(),
						"item_id":         itemID,
						"output_index":    i,
						"delta":           args,
					})
					emitJSON("response.function_call_arguments.done", map[string]any{
						"type":            "response.function_call_arguments.done",
						"sequence_number": seq(),
						"item_id":         itemID,
						"output_index":    i,
						"arguments":       args,
					})
				}
			}

			emitJSON("response.output_item.done", map[string]any{
				"type":            "response.output_item.done",
				"sequence_number": seq(),
				"output_index":    i,
				"item":            item.Value(),
			})
		}
	} else {
		// Fallback for older response shapes: synthesize a single message item from output_text.
		text := gjson.GetBytes(resp, "output_text").String()
		if text == "" {
			text = gjson.GetBytes(resp, `output.#(type=="message").content.#(type=="output_text").text`).String()
		}
		if text != "" {
			msgID := "msg_" + id + "_0"
			emitJSON("response.output_item.added", map[string]any{
				"type":            "response.output_item.added",
				"sequence_number": seq(),
				"output_index":    0,
				"item": map[string]any{
					"id":      msgID,
					"type":    "message",
					"status":  "in_progress",
					"content": []any{},
					"role":    "assistant",
				},
			})
			emitJSON("response.content_part.added", map[string]any{
				"type":            "response.content_part.added",
				"sequence_number": seq(),
				"item_id":         msgID,
				"output_index":    0,
				"content_index":   0,
				"part": map[string]any{
					"type":        "output_text",
					"annotations": []any{},
					"logprobs":    []any{},
					"text":        "",
				},
			})
			emitJSON("response.output_text.delta", map[string]any{
				"type":            "response.output_text.delta",
				"sequence_number": seq(),
				"item_id":         msgID,
				"output_index":    0,
				"content_index":   0,
				"delta":           text,
				"logprobs":        []any{},
			})
			emitJSON("response.output_text.done", map[string]any{
				"type":            "response.output_text.done",
				"sequence_number": seq(),
				"item_id":         msgID,
				"output_index":    0,
				"content_index":   0,
				"text":            text,
				"logprobs":        []any{},
			})
			emitJSON("response.output_item.done", map[string]any{
				"type":            "response.output_item.done",
				"sequence_number": seq(),
				"output_index":    0,
				"item": map[string]any{
					"id":     msgID,
					"type":   "message",
					"status": "completed",
					"content": []any{
						map[string]any{"type": "output_text", "annotations": []any{}, "logprobs": []any{}, "text": text},
					},
					"role": "assistant",
				},
			})
		}
	}

	emitJSON("response.completed", map[string]any{
		"type":            "response.completed",
		"sequence_number": seq(),
		"response": map[string]any{
			"id":     id,
			"object": "response",
			"status": "completed",
			"error":  nil,
		},
	})
	if shouldEmitDoneSentinel(c) {
		_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	}
	flusher.Flush()
}
