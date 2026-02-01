package executor

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	kiroclaude "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/claude"
	kirocommon "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/common"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
)

// EventStreamError represents an Event Stream processing error
type EventStreamError struct {
	Type    string // "fatal", "malformed"
	Message string
	Cause   error
}

func (e *EventStreamError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("event stream %s: %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("event stream %s: %s", e.Type, e.Message)
}

// eventStreamMessage represents a parsed AWS Event Stream message
type eventStreamMessage struct {
	EventType string // Event type from headers (e.g., "assistantResponseEvent")
	Payload   []byte // JSON payload of the message
}

// parseEventStream parses AWS Event Stream binary format.
// Extracts text content, tool uses, and stop_reason from the response.
// Supports embedded [Called ...] tool calls and input buffering for toolUseEvent.
// Returns: content, toolUses, usageInfo, stopReason, error
func (e *KiroExecutor) parseEventStream(body io.Reader) (string, []kiroclaude.KiroToolUse, usage.Detail, string, error) {
	var content strings.Builder
	var toolUses []kiroclaude.KiroToolUse
	var usageInfo usage.Detail
	var stopReason string // Extracted from upstream response
	reader := bufio.NewReader(body)

	// Tool use state tracking for input buffering and deduplication
	processedIDs := make(map[string]bool)
	var currentToolUse *kiroclaude.ToolUseState

	// Upstream usage tracking - Kiro API returns credit usage and context percentage
	var upstreamContextPercentage float64 // Context usage percentage from upstream (e.g., 78.56)

	for {
		msg, eventErr := e.readEventStreamMessage(reader)
		if eventErr != nil {
			log.Errorf("kiro: parseEventStream error: %v", eventErr)
			return content.String(), toolUses, usageInfo, stopReason, eventErr
		}
		if msg == nil {
			// Normal end of stream (EOF)
			break
		}

		eventType := msg.EventType
		payload := msg.Payload
		if len(payload) == 0 {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Debugf("kiro: skipping malformed event: %v", err)
			continue
		}

		// Check for error/exception events in the payload (Kiro API may return errors with HTTP 200)
		// These can appear as top-level fields or nested within the event
		if errType, hasErrType := event["_type"].(string); hasErrType {
			// AWS-style error: {"_type": "com.amazon.aws.codewhisperer#ValidationException", "message": "..."}
			errMsg := ""
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			}
			log.Errorf("kiro: received AWS error in event stream: type=%s, message=%s", errType, errMsg)
			return "", nil, usageInfo, stopReason, fmt.Errorf("kiro API error: %s - %s", errType, errMsg)
		}
		if errType, hasErrType := event["type"].(string); hasErrType && (errType == "error" || errType == "exception") {
			// Generic error event
			errMsg := ""
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			} else if errObj, ok := event["error"].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
			}
			log.Errorf("kiro: received error event in stream: type=%s, message=%s", errType, errMsg)
			return "", nil, usageInfo, stopReason, fmt.Errorf("kiro API error: %s", errMsg)
		}

		// Extract stop_reason from various event formats
		// Kiro/Amazon Q API may include stop_reason in different locations
		if sr := kirocommon.GetString(event, "stop_reason"); sr != "" {
			stopReason = sr
			log.Debugf("kiro: parseEventStream found stop_reason (top-level): %s", stopReason)
		}
		if sr := kirocommon.GetString(event, "stopReason"); sr != "" {
			stopReason = sr
			log.Debugf("kiro: parseEventStream found stopReason (top-level): %s", stopReason)
		}

		// Handle different event types
		switch eventType {
		case "followupPromptEvent":
			// Filter out followupPrompt events - these are UI suggestions, not content
			log.Debugf("kiro: parseEventStream ignoring followupPrompt event")
			continue

		case "assistantResponseEvent":
			if assistantResp, ok := event["assistantResponseEvent"].(map[string]interface{}); ok {
				if contentText, ok := assistantResp["content"].(string); ok {
					content.WriteString(contentText)
				}
				// Extract stop_reason from assistantResponseEvent
				if sr := kirocommon.GetString(assistantResp, "stop_reason"); sr != "" {
					stopReason = sr
					log.Debugf("kiro: parseEventStream found stop_reason in assistantResponseEvent: %s", stopReason)
				}
				if sr := kirocommon.GetString(assistantResp, "stopReason"); sr != "" {
					stopReason = sr
					log.Debugf("kiro: parseEventStream found stopReason in assistantResponseEvent: %s", stopReason)
				}
				// Extract tool uses from response
				if toolUsesRaw, ok := assistantResp["toolUses"].([]interface{}); ok {
					for _, tuRaw := range toolUsesRaw {
						if tu, ok := tuRaw.(map[string]interface{}); ok {
							toolUseID := kirocommon.GetStringValue(tu, "toolUseId")
							// Check for duplicate
							if processedIDs[toolUseID] {
								log.Debugf("kiro: skipping duplicate tool use from assistantResponse: %s", toolUseID)
								continue
							}
							processedIDs[toolUseID] = true

							toolUse := kiroclaude.KiroToolUse{
								ToolUseID: toolUseID,
								Name:      kirocommon.GetStringValue(tu, "name"),
							}
							if input, ok := tu["input"].(map[string]interface{}); ok {
								toolUse.Input = input
							}
							toolUses = append(toolUses, toolUse)
						}
					}
				}
			}
			// Also try direct format
			if contentText, ok := event["content"].(string); ok {
				content.WriteString(contentText)
			}
			// Direct tool uses
			if toolUsesRaw, ok := event["toolUses"].([]interface{}); ok {
				for _, tuRaw := range toolUsesRaw {
					if tu, ok := tuRaw.(map[string]interface{}); ok {
						toolUseID := kirocommon.GetStringValue(tu, "toolUseId")
						// Check for duplicate
						if processedIDs[toolUseID] {
							log.Debugf("kiro: skipping duplicate direct tool use: %s", toolUseID)
							continue
						}
						processedIDs[toolUseID] = true

						toolUse := kiroclaude.KiroToolUse{
							ToolUseID: toolUseID,
							Name:      kirocommon.GetStringValue(tu, "name"),
						}
						if input, ok := tu["input"].(map[string]interface{}); ok {
							toolUse.Input = input
						}
						toolUses = append(toolUses, toolUse)
					}
				}
			}

		case "toolUseEvent":
			// Handle dedicated tool use events with input buffering
			completedToolUses, newState := kiroclaude.ProcessToolUseEvent(event, currentToolUse, processedIDs)
			currentToolUse = newState
			toolUses = append(toolUses, completedToolUses...)

		case "supplementaryWebLinksEvent":
			if inputTokens, ok := event["inputTokens"].(float64); ok {
				usageInfo.InputTokens = int64(inputTokens)
			}
			if outputTokens, ok := event["outputTokens"].(float64); ok {
				usageInfo.OutputTokens = int64(outputTokens)
			}

		case "messageStopEvent", "message_stop":
			// Handle message stop events which may contain stop_reason
			if sr := kirocommon.GetString(event, "stop_reason"); sr != "" {
				stopReason = sr
				log.Debugf("kiro: parseEventStream found stop_reason in messageStopEvent: %s", stopReason)
			}
			if sr := kirocommon.GetString(event, "stopReason"); sr != "" {
				stopReason = sr
				log.Debugf("kiro: parseEventStream found stopReason in messageStopEvent: %s", stopReason)
			}

		case "messageMetadataEvent", "metadataEvent":
			// Handle message metadata events which contain token counts
			// Official format: { tokenUsage: { outputTokens, totalTokens, uncachedInputTokens, cacheReadInputTokens, cacheWriteInputTokens, contextUsagePercentage } }
			var metadata map[string]interface{}
			if m, ok := event["messageMetadataEvent"].(map[string]interface{}); ok {
				metadata = m
			} else if m, ok := event["metadataEvent"].(map[string]interface{}); ok {
				metadata = m
			} else {
				metadata = event // event itself might be the metadata
			}

			// Check for nested tokenUsage object (official format)
			if tokenUsage, ok := metadata["tokenUsage"].(map[string]interface{}); ok {
				// outputTokens - precise output token count
				if outputTokens, ok := tokenUsage["outputTokens"].(float64); ok {
					usageInfo.OutputTokens = int64(outputTokens)
					log.Infof("kiro: parseEventStream found precise outputTokens in tokenUsage: %d", usageInfo.OutputTokens)
				}
				// totalTokens - precise total token count
				if totalTokens, ok := tokenUsage["totalTokens"].(float64); ok {
					usageInfo.TotalTokens = int64(totalTokens)
					log.Infof("kiro: parseEventStream found precise totalTokens in tokenUsage: %d", usageInfo.TotalTokens)
				}
				// uncachedInputTokens - input tokens not from cache
				if uncachedInputTokens, ok := tokenUsage["uncachedInputTokens"].(float64); ok {
					usageInfo.InputTokens = int64(uncachedInputTokens)
					log.Infof("kiro: parseEventStream found uncachedInputTokens in tokenUsage: %d", usageInfo.InputTokens)
				}
				// cacheReadInputTokens - tokens read from cache
				if cacheReadTokens, ok := tokenUsage["cacheReadInputTokens"].(float64); ok {
					// Add to input tokens if we have uncached tokens, otherwise use as input
					if usageInfo.InputTokens > 0 {
						usageInfo.InputTokens += int64(cacheReadTokens)
					} else {
						usageInfo.InputTokens = int64(cacheReadTokens)
					}
					log.Debugf("kiro: parseEventStream found cacheReadInputTokens in tokenUsage: %d", int64(cacheReadTokens))
				}
				// contextUsagePercentage - can be used as fallback for input token estimation
				if ctxPct, ok := tokenUsage["contextUsagePercentage"].(float64); ok {
					upstreamContextPercentage = ctxPct
					log.Debugf("kiro: parseEventStream found contextUsagePercentage in tokenUsage: %.2f%%", ctxPct)
				}
			}

			// Fallback: check for direct fields in metadata (legacy format)
			if usageInfo.InputTokens == 0 {
				if inputTokens, ok := metadata["inputTokens"].(float64); ok {
					usageInfo.InputTokens = int64(inputTokens)
					log.Debugf("kiro: parseEventStream found inputTokens in messageMetadataEvent: %d", usageInfo.InputTokens)
				}
			}
			if usageInfo.OutputTokens == 0 {
				if outputTokens, ok := metadata["outputTokens"].(float64); ok {
					usageInfo.OutputTokens = int64(outputTokens)
					log.Debugf("kiro: parseEventStream found outputTokens in messageMetadataEvent: %d", usageInfo.OutputTokens)
				}
			}
			if usageInfo.TotalTokens == 0 {
				if totalTokens, ok := metadata["totalTokens"].(float64); ok {
					usageInfo.TotalTokens = int64(totalTokens)
					log.Debugf("kiro: parseEventStream found totalTokens in messageMetadataEvent: %d", usageInfo.TotalTokens)
				}
			}

		case "usageEvent", "usage":
			// Handle dedicated usage events
			if inputTokens, ok := event["inputTokens"].(float64); ok {
				usageInfo.InputTokens = int64(inputTokens)
				log.Debugf("kiro: parseEventStream found inputTokens in usageEvent: %d", usageInfo.InputTokens)
			}
			if outputTokens, ok := event["outputTokens"].(float64); ok {
				usageInfo.OutputTokens = int64(outputTokens)
				log.Debugf("kiro: parseEventStream found outputTokens in usageEvent: %d", usageInfo.OutputTokens)
			}
			if totalTokens, ok := event["totalTokens"].(float64); ok {
				usageInfo.TotalTokens = int64(totalTokens)
				log.Debugf("kiro: parseEventStream found totalTokens in usageEvent: %d", usageInfo.TotalTokens)
			}
			// Also check nested usage object
			if usageObj, ok := event["usage"].(map[string]interface{}); ok {
				if inputTokens, ok := usageObj["input_tokens"].(float64); ok {
					usageInfo.InputTokens = int64(inputTokens)
				} else if inputTokens, ok := usageObj["prompt_tokens"].(float64); ok {
					usageInfo.InputTokens = int64(inputTokens)
				}
				if outputTokens, ok := usageObj["output_tokens"].(float64); ok {
					usageInfo.OutputTokens = int64(outputTokens)
				} else if outputTokens, ok := usageObj["completion_tokens"].(float64); ok {
					usageInfo.OutputTokens = int64(outputTokens)
				}
				if totalTokens, ok := usageObj["total_tokens"].(float64); ok {
					usageInfo.TotalTokens = int64(totalTokens)
				}
				log.Debugf("kiro: parseEventStream found usage object: input=%d, output=%d, total=%d",
					usageInfo.InputTokens, usageInfo.OutputTokens, usageInfo.TotalTokens)
			}

		case "metricsEvent":
			// Handle metrics events which may contain usage data
			if metrics, ok := event["metricsEvent"].(map[string]interface{}); ok {
				if inputTokens, ok := metrics["inputTokens"].(float64); ok {
					usageInfo.InputTokens = int64(inputTokens)
				}
				if outputTokens, ok := metrics["outputTokens"].(float64); ok {
					usageInfo.OutputTokens = int64(outputTokens)
				}
				log.Debugf("kiro: parseEventStream found metricsEvent: input=%d, output=%d",
					usageInfo.InputTokens, usageInfo.OutputTokens)
			}

		case "meteringEvent":
			// Handle metering events from Kiro API (usage billing information)
			// Official format: { unit: string, unitPlural: string, usage: number }
			if metering, ok := event["meteringEvent"].(map[string]interface{}); ok {
				unit := ""
				if u, ok := metering["unit"].(string); ok {
					unit = u
				}
				usageVal := 0.0
				if u, ok := metering["usage"].(float64); ok {
					usageVal = u
				}
				log.Infof("kiro: parseEventStream received meteringEvent: usage=%.2f %s", usageVal, unit)
				// Store metering info for potential billing/statistics purposes
				// Note: This is separate from token counts - it's AWS billing units
			} else {
				// Try direct fields
				unit := ""
				if u, ok := event["unit"].(string); ok {
					unit = u
				}
				usageVal := 0.0
				if u, ok := event["usage"].(float64); ok {
					usageVal = u
				}
				if unit != "" || usageVal > 0 {
					log.Infof("kiro: parseEventStream received meteringEvent (direct): usage=%.2f %s", usageVal, unit)
				}
			}

		case "error", "exception", "internalServerException", "invalidStateEvent":
			// Handle error events from Kiro API stream
			errMsg := ""
			errType := eventType

			// Try to extract error message from various formats
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			} else if errObj, ok := event[eventType].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
				if t, ok := errObj["type"].(string); ok {
					errType = t
				}
			} else if errObj, ok := event["error"].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
				if t, ok := errObj["type"].(string); ok {
					errType = t
				}
			}

			// Check for specific error reasons
			if reason, ok := event["reason"].(string); ok {
				errMsg = fmt.Sprintf("%s (reason: %s)", errMsg, reason)
			}

			log.Errorf("kiro: parseEventStream received error event: type=%s, message=%s", errType, errMsg)

			// For invalidStateEvent, we may want to continue processing other events
			if eventType == "invalidStateEvent" {
				log.Warnf("kiro: invalidStateEvent received, continuing stream processing")
				continue
			}

			// For other errors, return the error
			if errMsg != "" {
				return "", nil, usageInfo, stopReason, fmt.Errorf("kiro API error (%s): %s", errType, errMsg)
			}

		default:
			// Check for contextUsagePercentage in any event
			if ctxPct, ok := event["contextUsagePercentage"].(float64); ok {
				upstreamContextPercentage = ctxPct
				log.Debugf("kiro: parseEventStream received context usage: %.2f%%", upstreamContextPercentage)
			}
			// Log unknown event types for debugging (to discover new event formats)
			log.Debugf("kiro: parseEventStream unknown event type: %s, payload: %s", eventType, string(payload))
		}

		// Check for direct token fields in any event (fallback)
		if usageInfo.InputTokens == 0 {
			if inputTokens, ok := event["inputTokens"].(float64); ok {
				usageInfo.InputTokens = int64(inputTokens)
				log.Debugf("kiro: parseEventStream found direct inputTokens: %d", usageInfo.InputTokens)
			}
		}
		if usageInfo.OutputTokens == 0 {
			if outputTokens, ok := event["outputTokens"].(float64); ok {
				usageInfo.OutputTokens = int64(outputTokens)
				log.Debugf("kiro: parseEventStream found direct outputTokens: %d", usageInfo.OutputTokens)
			}
		}

		// Check for usage object in any event (OpenAI format)
		if usageInfo.InputTokens == 0 || usageInfo.OutputTokens == 0 {
			if usageObj, ok := event["usage"].(map[string]interface{}); ok {
				if usageInfo.InputTokens == 0 {
					if inputTokens, ok := usageObj["input_tokens"].(float64); ok {
						usageInfo.InputTokens = int64(inputTokens)
					} else if inputTokens, ok := usageObj["prompt_tokens"].(float64); ok {
						usageInfo.InputTokens = int64(inputTokens)
					}
				}
				if usageInfo.OutputTokens == 0 {
					if outputTokens, ok := usageObj["output_tokens"].(float64); ok {
						usageInfo.OutputTokens = int64(outputTokens)
					} else if outputTokens, ok := usageObj["completion_tokens"].(float64); ok {
						usageInfo.OutputTokens = int64(outputTokens)
					}
				}
				if usageInfo.TotalTokens == 0 {
					if totalTokens, ok := usageObj["total_tokens"].(float64); ok {
						usageInfo.TotalTokens = int64(totalTokens)
					}
				}
				log.Debugf("kiro: parseEventStream found usage object (fallback): input=%d, output=%d, total=%d",
					usageInfo.InputTokens, usageInfo.OutputTokens, usageInfo.TotalTokens)
			}
		}

		// Also check nested supplementaryWebLinksEvent
		if usageEvent, ok := event["supplementaryWebLinksEvent"].(map[string]interface{}); ok {
			if inputTokens, ok := usageEvent["inputTokens"].(float64); ok {
				usageInfo.InputTokens = int64(inputTokens)
			}
			if outputTokens, ok := usageEvent["outputTokens"].(float64); ok {
				usageInfo.OutputTokens = int64(outputTokens)
			}
		}
	}

	// Parse embedded tool calls from content (e.g., [Called tool_name with args: {...}])
	contentStr := content.String()
	cleanedContent, embeddedToolUses := kiroclaude.ParseEmbeddedToolCalls(contentStr, processedIDs)
	toolUses = append(toolUses, embeddedToolUses...)

	// Deduplicate all tool uses
	toolUses = kiroclaude.DeduplicateToolUses(toolUses)

	// Apply fallback logic for stop_reason if not provided by upstream
	// Priority: upstream stopReason > tool_use detection > end_turn default
	if stopReason == "" {
		if len(toolUses) > 0 {
			stopReason = "tool_use"
			log.Debugf("kiro: parseEventStream using fallback stop_reason: tool_use (detected %d tool uses)", len(toolUses))
		} else {
			stopReason = "end_turn"
			log.Debugf("kiro: parseEventStream using fallback stop_reason: end_turn")
		}
	}

	// Log warning if response was truncated due to max_tokens
	if stopReason == "max_tokens" {
		log.Warnf("kiro: response truncated due to max_tokens limit")
	}

	// Use contextUsagePercentage to calculate more accurate input tokens
	// Kiro model has 200k max context, contextUsagePercentage represents the percentage used
	// Formula: input_tokens = contextUsagePercentage * 200000 / 100
	if upstreamContextPercentage > 0 {
		calculatedInputTokens := int64(upstreamContextPercentage * 200000 / 100)
		if calculatedInputTokens > 0 {
			localEstimate := usageInfo.InputTokens
			usageInfo.InputTokens = calculatedInputTokens
			usageInfo.TotalTokens = usageInfo.InputTokens + usageInfo.OutputTokens
			log.Infof("kiro: parseEventStream using contextUsagePercentage (%.2f%%) to calculate input tokens: %d (local estimate was: %d)",
				upstreamContextPercentage, calculatedInputTokens, localEstimate)
		}
	}

	return cleanedContent, toolUses, usageInfo, stopReason, nil
}

// readEventStreamMessage reads and validates a single AWS Event Stream message.
// Returns the parsed message or a structured error for different failure modes.
// This function implements boundary protection and detailed error classification.
//
// AWS Event Stream binary format:
// - Prelude (12 bytes): total_length (4) + headers_length (4) + prelude_crc (4)
// - Headers (variable): header entries
// - Payload (variable): JSON data
// - Message CRC (4 bytes): CRC32C of entire message (not validated, just skipped)
func (e *KiroExecutor) readEventStreamMessage(reader *bufio.Reader) (*eventStreamMessage, *EventStreamError) {
	// Read prelude (first 12 bytes: total_len + headers_len + prelude_crc)
	prelude := make([]byte, 12)
	_, err := io.ReadFull(reader, prelude)
	if err == io.EOF {
		return nil, nil // Normal end of stream
	}
	if err != nil {
		return nil, &EventStreamError{
			Type:    ErrStreamFatal,
			Message: "failed to read prelude",
			Cause:   err,
		}
	}

	totalLength := binary.BigEndian.Uint32(prelude[0:4])
	headersLength := binary.BigEndian.Uint32(prelude[4:8])
	// Note: prelude[8:12] is prelude_crc - we read it but don't validate (no CRC check per requirements)

	// Boundary check: minimum frame size
	if totalLength < minEventStreamFrameSize {
		return nil, &EventStreamError{
			Type:    ErrStreamMalformed,
			Message: fmt.Sprintf("invalid message length: %d (minimum is %d)", totalLength, minEventStreamFrameSize),
		}
	}

	// Boundary check: maximum message size
	if totalLength > maxEventStreamMsgSize {
		return nil, &EventStreamError{
			Type:    ErrStreamMalformed,
			Message: fmt.Sprintf("message too large: %d bytes (maximum is %d)", totalLength, maxEventStreamMsgSize),
		}
	}

	// Boundary check: headers length within message bounds
	// Message structure: prelude(12) + headers(headersLength) + payload + message_crc(4)
	// So: headersLength must be <= totalLength - 16 (12 for prelude + 4 for message_crc)
	if headersLength > totalLength-16 {
		return nil, &EventStreamError{
			Type:    ErrStreamMalformed,
			Message: fmt.Sprintf("headers length %d exceeds message bounds (total: %d)", headersLength, totalLength),
		}
	}

	// Read the rest of the message (total - 12 bytes already read)
	remaining := make([]byte, totalLength-12)
	_, err = io.ReadFull(reader, remaining)
	if err != nil {
		return nil, &EventStreamError{
			Type:    ErrStreamFatal,
			Message: "failed to read message body",
			Cause:   err,
		}
	}

	// Extract event type from headers
	// Headers start at beginning of 'remaining', length is headersLength
	var eventType string
	if headersLength > 0 && headersLength <= uint32(len(remaining)) {
		eventType = e.extractEventTypeFromBytes(remaining[:headersLength])
	}

	// Calculate payload boundaries
	// Payload starts after headers, ends before message_crc (last 4 bytes)
	payloadStart := headersLength
	payloadEnd := uint32(len(remaining)) - 4 // Skip message_crc at end

	// Validate payload boundaries
	if payloadStart >= payloadEnd {
		// No payload, return empty message
		return &eventStreamMessage{
			EventType: eventType,
			Payload:   nil,
		}, nil
	}

	payload := remaining[payloadStart:payloadEnd]

	return &eventStreamMessage{
		EventType: eventType,
		Payload:   payload,
	}, nil
}

func skipEventStreamHeaderValue(headers []byte, offset int, valueType byte) (int, bool) {
	switch valueType {
	case 0, 1: // bool true / bool false
		return offset, true
	case 2: // byte
		if offset+1 > len(headers) {
			return offset, false
		}
		return offset + 1, true
	case 3: // short
		if offset+2 > len(headers) {
			return offset, false
		}
		return offset + 2, true
	case 4: // int
		if offset+4 > len(headers) {
			return offset, false
		}
		return offset + 4, true
	case 5: // long
		if offset+8 > len(headers) {
			return offset, false
		}
		return offset + 8, true
	case 6: // byte array (2-byte length + data)
		if offset+2 > len(headers) {
			return offset, false
		}
		valueLen := int(binary.BigEndian.Uint16(headers[offset : offset+2]))
		offset += 2
		if offset+valueLen > len(headers) {
			return offset, false
		}
		return offset + valueLen, true
	case 8: // timestamp
		if offset+8 > len(headers) {
			return offset, false
		}
		return offset + 8, true
	case 9: // uuid
		if offset+16 > len(headers) {
			return offset, false
		}
		return offset + 16, true
	default:
		return offset, false
	}
}

// extractEventTypeFromBytes extracts the event type from raw header bytes (without prelude CRC prefix)
func (e *KiroExecutor) extractEventTypeFromBytes(headers []byte) string {
	offset := 0
	for offset < len(headers) {
		nameLen := int(headers[offset])
		offset++
		if offset+nameLen > len(headers) {
			break
		}
		name := string(headers[offset : offset+nameLen])
		offset += nameLen

		if offset >= len(headers) {
			break
		}
		valueType := headers[offset]
		offset++

		if valueType == 7 { // String type
			if offset+2 > len(headers) {
				break
			}
			valueLen := int(binary.BigEndian.Uint16(headers[offset : offset+2]))
			offset += 2
			if offset+valueLen > len(headers) {
				break
			}
			value := string(headers[offset : offset+valueLen])
			offset += valueLen

			if name == ":event-type" {
				return value
			}
			continue
		}

		nextOffset, ok := skipEventStreamHeaderValue(headers, offset, valueType)
		if !ok {
			break
		}
		offset = nextOffset
	}
	return ""
}

// findRealThinkingEndTag finds the real </thinking> end tag, skipping false positives.
// Returns -1 if no real end tag is found.
//
// Real </thinking> tags from Kiro API have specific characteristics:
// - Usually preceded by newline (.\n</thinking>)
// - Usually followed by newline (\n\n)
// - Not inside code blocks or inline code
//
// False positives (discussion text) have characteristics:
// - In the middle of a sentence
// - Preceded by discussion words like "tag", "returns"
// - Inside code blocks or inline code
//
// Parameters:
// - content: the content to search in
// - alreadyInCodeBlock: whether we're already inside a code block from previous chunks
// - alreadyInInlineCode: whether we're already inside inline code from previous chunks
func findRealThinkingEndTag(content string, alreadyInCodeBlock, alreadyInInlineCode bool) int {
	searchStart := 0
	for {
		endIdx := strings.Index(content[searchStart:], kirocommon.ThinkingEndTag)
		if endIdx < 0 {
			return -1
		}
		endIdx += searchStart // Adjust to absolute position

		textBeforeEnd := content[:endIdx]
		textAfterEnd := content[endIdx+len(kirocommon.ThinkingEndTag):]

		// Check 1: Is it inside inline code?
		// Count backticks in current content and add state from previous chunks
		backtickCount := strings.Count(textBeforeEnd, "`")
		effectiveInInlineCode := alreadyInInlineCode
		if backtickCount%2 == 1 {
			effectiveInInlineCode = !effectiveInInlineCode
		}
		if effectiveInInlineCode {
			log.Debugf("kiro: found </thinking> inside inline code at pos %d, skipping", endIdx)
			searchStart = endIdx + len(kirocommon.ThinkingEndTag)
			continue
		}

		// Check 2: Is it inside a code block?
		// Count fences in current content and add state from previous chunks
		fenceCount := strings.Count(textBeforeEnd, "```")
		altFenceCount := strings.Count(textBeforeEnd, "~~~")
		effectiveInCodeBlock := alreadyInCodeBlock
		if fenceCount%2 == 1 || altFenceCount%2 == 1 {
			effectiveInCodeBlock = !effectiveInCodeBlock
		}
		if effectiveInCodeBlock {
			log.Debugf("kiro: found </thinking> inside code block at pos %d, skipping", endIdx)
			searchStart = endIdx + len(kirocommon.ThinkingEndTag)
			continue
		}

		// Check 3: Real </thinking> tags are usually preceded by newline or at start
		// and followed by newline or at end. Check the format.
		charBeforeTag := byte(0)
		if endIdx > 0 {
			charBeforeTag = content[endIdx-1]
		}
		charAfterTag := byte(0)
		if len(textAfterEnd) > 0 {
			charAfterTag = textAfterEnd[0]
		}

		// Real end tag format: preceded by newline OR end of sentence (. ! ?)
		// and followed by newline OR end of content
		isPrecededByNewlineOrSentenceEnd := charBeforeTag == '\n' || charBeforeTag == '.' ||
			charBeforeTag == '!' || charBeforeTag == '?' || charBeforeTag == 0
		isFollowedByNewlineOrEnd := charAfterTag == '\n' || charAfterTag == 0

		// If the tag has proper formatting (newline before/after), it's likely real
		if isPrecededByNewlineOrSentenceEnd && isFollowedByNewlineOrEnd {
			log.Debugf("kiro: found properly formatted </thinking> at pos %d", endIdx)
			return endIdx
		}

		// Check 4: Is the tag preceded by discussion keywords on the same line?
		lastNewlineIdx := strings.LastIndex(textBeforeEnd, "\n")
		lineBeforeTag := textBeforeEnd
		if lastNewlineIdx >= 0 {
			lineBeforeTag = textBeforeEnd[lastNewlineIdx+1:]
		}
		lineBeforeTagLower := strings.ToLower(lineBeforeTag)

		// Discussion patterns - if found, this is likely discussion text
		discussionPatterns := []string{
			"tag", "return", "output", "contain", "use", "parse", "emit", "convert", "generate",
			"<thinking>",    // discussing both tags together
			"`</thinking>`", // explicitly in inline code
		}
		isDiscussion := false
		for _, pattern := range discussionPatterns {
			if strings.Contains(lineBeforeTagLower, pattern) {
				isDiscussion = true
				break
			}
		}
		if isDiscussion {
			log.Debugf("kiro: found </thinking> after discussion text at pos %d, skipping", endIdx)
			searchStart = endIdx + len(kirocommon.ThinkingEndTag)
			continue
		}

		// Check 5: Is there text immediately after on the same line?
		// Real end tags don't have text immediately after on the same line
		if len(textAfterEnd) > 0 && charAfterTag != '\n' && charAfterTag != 0 {
			// Find the next newline
			nextNewline := strings.Index(textAfterEnd, "\n")
			var textOnSameLine string
			if nextNewline >= 0 {
				textOnSameLine = textAfterEnd[:nextNewline]
			} else {
				textOnSameLine = textAfterEnd
			}
			// If there's non-whitespace text on the same line after the tag, it's discussion
			if strings.TrimSpace(textOnSameLine) != "" {
				log.Debugf("kiro: found </thinking> with text after on same line at pos %d, skipping", endIdx)
				searchStart = endIdx + len(kirocommon.ThinkingEndTag)
				continue
			}
		}

		// Check 6: Is there another <thinking> tag after this </thinking>?
		if strings.Contains(textAfterEnd, kirocommon.ThinkingStartTag) {
			nextStartIdx := strings.Index(textAfterEnd, kirocommon.ThinkingStartTag)
			textBeforeNextStart := textAfterEnd[:nextStartIdx]
			nextBacktickCount := strings.Count(textBeforeNextStart, "`")
			nextFenceCount := strings.Count(textBeforeNextStart, "```")
			nextAltFenceCount := strings.Count(textBeforeNextStart, "~~~")

			// If the next <thinking> is NOT in code, then this </thinking> is discussion text
			if nextBacktickCount%2 == 0 && nextFenceCount%2 == 0 && nextAltFenceCount%2 == 0 {
				log.Debugf("kiro: found </thinking> followed by <thinking> at pos %d, likely discussion text, skipping", endIdx)
				searchStart = endIdx + len(kirocommon.ThinkingEndTag)
				continue
			}
		}

		// This looks like a real end tag
		return endIdx
	}
}

// streamToChannel converts AWS Event Stream to channel-based streaming.
// Supports tool calling - emits tool_use content blocks when tools are used.
// Includes embedded [Called ...] tool call parsing and input buffering for toolUseEvent.
// Implements duplicate content filtering using lastContentEvent detection (based on AIClient-2-API).
// Extracts stop_reason from upstream events when available.
// thinkingEnabled controls whether <thinking> tags are parsed - only parse when request enabled thinking.
func (e *KiroExecutor) streamToChannel(ctx context.Context, body io.Reader, out chan<- cliproxyexecutor.StreamChunk, targetFormat sdktranslator.Format, model string, originalReq, claudeBody []byte, reporter *usageReporter, thinkingEnabled bool) {
	reader := bufio.NewReaderSize(body, 20*1024*1024) // 20MB buffer to match other providers
	var totalUsage usage.Detail
	var hasToolUses bool          // Track if any tool uses were emitted
	var upstreamStopReason string // Track stop_reason from upstream events

	// Tool use state tracking for input buffering and deduplication
	processedIDs := make(map[string]bool)
	var currentToolUse *kiroclaude.ToolUseState

	// Streaming token calculation - accumulate content for real-time token counting
	var accumulatedContent strings.Builder
	accumulatedContent.Grow(4096) // Pre-allocate 4KB capacity to reduce reallocations

	// Real-time usage estimation state
	var lastUsageUpdateLen int           // Last accumulated content length when usage was sent
	var lastUsageUpdateTime = time.Now() // Last time usage update was sent
	var lastReportedOutputTokens int64   // Last reported output token count

	// Upstream usage tracking - Kiro API returns credit usage and context percentage
	var upstreamCreditUsage float64       // Credit usage from upstream (e.g., 1.458)
	var upstreamContextPercentage float64 // Context usage percentage from upstream (e.g., 78.56)
	var hasUpstreamUsage bool             // Whether we received usage from upstream

	// Translator param for maintaining tool call state across streaming events
	var translatorParam any

	// Thinking mode state tracking
	inThinkBlock := false
	isThinkingBlockOpen := false
	thinkingBlockIndex := -1
	var accumulatedThinkingContent strings.Builder

	// Buffer for handling partial tag matches at chunk boundaries
	var pendingContent strings.Builder

	// Pre-calculate input tokens from request if possible
	if enc, err := getTokenizer(model); err == nil {
		var inputTokens int64
		var countMethod string

		// Try Claude format first (Kiro uses Claude API format)
		if inp, err := countClaudeChatTokens(enc, claudeBody); err == nil && inp > 0 {
			inputTokens = inp
			countMethod = "claude"
		} else if inp, err := countOpenAIChatTokens(enc, originalReq); err == nil && inp > 0 {
			// Fallback to OpenAI format (for OpenAI-compatible requests)
			inputTokens = inp
			countMethod = "openai"
		} else {
			// Final fallback: estimate from raw request size
			inputTokens = int64(len(claudeBody) / 4)
			if inputTokens == 0 && len(claudeBody) > 0 {
				inputTokens = 1
			}
			countMethod = "estimate"
		}

		totalUsage.InputTokens = inputTokens
		log.Debugf("kiro: streamToChannel pre-calculated input tokens: %d (method: %s, claude body: %d bytes, original req: %d bytes)",
			totalUsage.InputTokens, countMethod, len(claudeBody), len(originalReq))
	}

	contentBlockIndex := -1
	messageStartSent := false
	isTextBlockOpen := false
	var outputLen int

	// Ensure usage is published even on early return
	defer func() {
		reporter.publish(ctx, totalUsage)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, eventErr := e.readEventStreamMessage(reader)
		if eventErr != nil {
			log.Errorf("kiro: streamToChannel error: %v", eventErr)
			out <- cliproxyexecutor.StreamChunk{Err: eventErr}
			return
		}
		if msg == nil {
			// Normal end of stream (EOF)
			// Flush any incomplete tool use before ending stream
			if currentToolUse != nil && !processedIDs[currentToolUse.ToolUseID] {
				log.Warnf("kiro: flushing incomplete tool use at EOF: %s (ID: %s)", currentToolUse.Name, currentToolUse.ToolUseID)
				fullInput := currentToolUse.InputBuffer.String()
				repairedJSON := kiroclaude.RepairJSON(fullInput)
				var finalInput map[string]interface{}
				if err := json.Unmarshal([]byte(repairedJSON), &finalInput); err != nil {
					log.Warnf("kiro: failed to parse incomplete tool input at EOF: %v", err)
					finalInput = make(map[string]interface{})
				}

				processedIDs[currentToolUse.ToolUseID] = true
				contentBlockIndex++

				// Send tool_use content block
				blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(contentBlockIndex, "tool_use", currentToolUse.ToolUseID, currentToolUse.Name)
				sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				// Send tool input as delta
				inputBytes, _ := json.Marshal(finalInput)
				inputDelta := kiroclaude.BuildClaudeInputJsonDeltaEvent(string(inputBytes), contentBlockIndex)
				sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, inputDelta, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				// Close block
				blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
				sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				hasToolUses = true
				currentToolUse = nil
			}
			break
		}

		eventType := msg.EventType
		payload := msg.Payload
		if len(payload) == 0 {
			continue
		}
		appendAPIResponseChunk(ctx, e.cfg, payload)

		var event map[string]interface{}
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Warnf("kiro: failed to unmarshal event payload: %v, raw: %s", err, string(payload))
			continue
		}

		// Check for error/exception events in the payload
		if errType, hasErrType := event["_type"].(string); hasErrType {
			errMsg := ""
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			}
			log.Errorf("kiro: received AWS error in stream: type=%s, message=%s", errType, errMsg)
			out <- cliproxyexecutor.StreamChunk{Err: fmt.Errorf("kiro API error: %s - %s", errType, errMsg)}
			return
		}
		if errType, hasErrType := event["type"].(string); hasErrType && (errType == "error" || errType == "exception") {
			errMsg := ""
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			} else if errObj, ok := event["error"].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
			}
			log.Errorf("kiro: received error event in stream: type=%s, message=%s", errType, errMsg)
			out <- cliproxyexecutor.StreamChunk{Err: fmt.Errorf("kiro API error: %s", errMsg)}
			return
		}

		// Extract stop_reason from various event formats (streaming)
		if sr := kirocommon.GetString(event, "stop_reason"); sr != "" {
			upstreamStopReason = sr
			log.Debugf("kiro: streamToChannel found stop_reason (top-level): %s", upstreamStopReason)
		}
		if sr := kirocommon.GetString(event, "stopReason"); sr != "" {
			upstreamStopReason = sr
			log.Debugf("kiro: streamToChannel found stopReason (top-level): %s", upstreamStopReason)
		}

		// Send message_start on first event
		if !messageStartSent {
			msgStart := kiroclaude.BuildClaudeMessageStartEvent(model, totalUsage.InputTokens)
			sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, msgStart, &translatorParam)
			for _, chunk := range sseData {
				if chunk != "" {
					out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
				}
			}
			messageStartSent = true
		}

		// Process based on event type - delegate to handleStreamEvent for main logic
		e.handleStreamEvent(ctx, eventType, event, payload, &streamState{
			out:                        out,
			targetFormat:               targetFormat,
			model:                      model,
			originalReq:                originalReq,
			claudeBody:                 claudeBody,
			translatorParam:            &translatorParam,
			contentBlockIndex:          &contentBlockIndex,
			isTextBlockOpen:            &isTextBlockOpen,
			isThinkingBlockOpen:        &isThinkingBlockOpen,
			thinkingBlockIndex:         &thinkingBlockIndex,
			inThinkBlock:               &inThinkBlock,
			pendingContent:             &pendingContent,
			accumulatedContent:         &accumulatedContent,
			accumulatedThinkingContent: &accumulatedThinkingContent,
			totalUsage:                 &totalUsage,
			hasUpstreamUsage:           &hasUpstreamUsage,
			upstreamCreditUsage:        &upstreamCreditUsage,
			upstreamContextPercentage:  &upstreamContextPercentage,
			upstreamStopReason:         &upstreamStopReason,
			lastUsageUpdateLen:         &lastUsageUpdateLen,
			lastUsageUpdateTime:        &lastUsageUpdateTime,
			lastReportedOutputTokens:   &lastReportedOutputTokens,
			outputLen:                  &outputLen,
			processedIDs:               processedIDs,
			currentToolUse:             &currentToolUse,
			hasToolUses:                &hasToolUses,
		})
	}

	// Close content block if open
	if isTextBlockOpen && contentBlockIndex >= 0 {
		blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
		sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
		for _, chunk := range sseData {
			if chunk != "" {
				out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
			}
		}
	}

	// Calculate output tokens from accumulated content
	if totalUsage.OutputTokens == 0 && accumulatedContent.Len() > 0 {
		if enc, err := getTokenizer(model); err == nil {
			if tokenCount, countErr := enc.Count(accumulatedContent.String()); countErr == nil {
				totalUsage.OutputTokens = int64(tokenCount)
				log.Debugf("kiro: streamToChannel calculated output tokens using tiktoken: %d", totalUsage.OutputTokens)
			} else {
				totalUsage.OutputTokens = int64(accumulatedContent.Len() / 4)
				if totalUsage.OutputTokens == 0 {
					totalUsage.OutputTokens = 1
				}
			}
		} else {
			totalUsage.OutputTokens = int64(accumulatedContent.Len() / 4)
			if totalUsage.OutputTokens == 0 {
				totalUsage.OutputTokens = 1
			}
		}
	} else if totalUsage.OutputTokens == 0 && outputLen > 0 {
		totalUsage.OutputTokens = int64(outputLen / 4)
		if totalUsage.OutputTokens == 0 {
			totalUsage.OutputTokens = 1
		}
	}

	// Use contextUsagePercentage for more accurate input tokens
	if upstreamContextPercentage > 0 {
		calculatedInputTokens := int64(upstreamContextPercentage * 200000 / 100)
		if calculatedInputTokens > 0 {
			localEstimate := totalUsage.InputTokens
			totalUsage.InputTokens = calculatedInputTokens
			log.Debugf("kiro: using contextUsagePercentage (%.2f%%) to calculate input tokens: %d (local estimate was: %d)",
				upstreamContextPercentage, calculatedInputTokens, localEstimate)
		}
	}

	totalUsage.TotalTokens = totalUsage.InputTokens + totalUsage.OutputTokens

	// Log upstream usage information if received
	if hasUpstreamUsage {
		log.Debugf("kiro: upstream usage - credits: %.4f, context: %.2f%%, final tokens - input: %d, output: %d, total: %d",
			upstreamCreditUsage, upstreamContextPercentage,
			totalUsage.InputTokens, totalUsage.OutputTokens, totalUsage.TotalTokens)
	}

	// Determine stop reason
	stopReason := upstreamStopReason
	if stopReason == "" {
		if hasToolUses {
			stopReason = "tool_use"
			log.Debugf("kiro: streamToChannel using fallback stop_reason: tool_use")
		} else {
			stopReason = "end_turn"
			log.Debugf("kiro: streamToChannel using fallback stop_reason: end_turn")
		}
	}

	if stopReason == "max_tokens" {
		log.Warnf("kiro: response truncated due to max_tokens limit (streamToChannel)")
	}

	// Send message_delta event
	msgDelta := kiroclaude.BuildClaudeMessageDeltaEvent(stopReason, totalUsage)
	sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, msgDelta, &translatorParam)
	for _, chunk := range sseData {
		if chunk != "" {
			out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
		}
	}

	// Send message_stop event
	msgStop := kiroclaude.BuildClaudeMessageStopOnlyEvent()
	sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, msgStop, &translatorParam)
	for _, chunk := range sseData {
		if chunk != "" {
			out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
		}
	}
}

// streamState holds the state for stream processing
type streamState struct {
	out                        chan<- cliproxyexecutor.StreamChunk
	targetFormat               sdktranslator.Format
	model                      string
	originalReq                []byte
	claudeBody                 []byte
	translatorParam            *any
	contentBlockIndex          *int
	isTextBlockOpen            *bool
	isThinkingBlockOpen        *bool
	thinkingBlockIndex         *int
	inThinkBlock               *bool
	pendingContent             *strings.Builder
	accumulatedContent         *strings.Builder
	accumulatedThinkingContent *strings.Builder
	totalUsage                 *usage.Detail
	hasUpstreamUsage           *bool
	upstreamCreditUsage        *float64
	upstreamContextPercentage  *float64
	upstreamStopReason         *string
	lastUsageUpdateLen         *int
	lastUsageUpdateTime        *time.Time
	lastReportedOutputTokens   *int64
	outputLen                  *int
	processedIDs               map[string]bool
	currentToolUse             **kiroclaude.ToolUseState
	hasToolUses                *bool
}

// handleStreamEvent processes a single stream event
// This is a helper to reduce the complexity of streamToChannel
func (e *KiroExecutor) handleStreamEvent(ctx context.Context, eventType string, event map[string]interface{}, payload []byte, s *streamState) {
	switch eventType {
	case "followupPromptEvent":
		log.Debugf("kiro: streamToChannel ignoring followupPrompt event")
		return

	case "messageStopEvent", "message_stop":
		if sr := kirocommon.GetString(event, "stop_reason"); sr != "" {
			*s.upstreamStopReason = sr
		}
		if sr := kirocommon.GetString(event, "stopReason"); sr != "" {
			*s.upstreamStopReason = sr
		}

	case "meteringEvent":
		if metering, ok := event["meteringEvent"].(map[string]interface{}); ok {
			if usage, ok := metering["usage"].(float64); ok {
				*s.upstreamCreditUsage = usage
				*s.hasUpstreamUsage = true
			}
		} else if usage, ok := event["usage"].(float64); ok {
			*s.upstreamCreditUsage = usage
			*s.hasUpstreamUsage = true
		}

	case "error", "exception", "internalServerException":
		errMsg := ""
		errType := eventType
		if msg, ok := event["message"].(string); ok {
			errMsg = msg
		}
		log.Errorf("kiro: streamToChannel received error event: type=%s, message=%s", errType, errMsg)
		if errMsg != "" {
			s.out <- cliproxyexecutor.StreamChunk{Err: fmt.Errorf("kiro API error (%s): %s", errType, errMsg)}
		}

	case "invalidStateEvent":
		errMsg := ""
		if msg, ok := event["message"].(string); ok {
			errMsg = msg
		}
		log.Warnf("kiro: streamToChannel received invalidStateEvent: %s, continuing", errMsg)

	case "assistantResponseEvent":
		e.handleAssistantResponseEvent(ctx, event, s)

	case "reasoningContentEvent":
		e.handleReasoningContentEvent(ctx, event, s)

	case "toolUseEvent":
		e.handleToolUseEvent(ctx, event, s)

	case "supplementaryWebLinksEvent", "messageMetadataEvent", "metadataEvent", "usageEvent", "usage", "metricsEvent":
		e.handleUsageEvent(event, eventType, s)

	default:
		// Check for usage data in unknown events
		if ctxPct, ok := event["contextUsagePercentage"].(float64); ok {
			*s.upstreamContextPercentage = ctxPct
		}
		if unit, ok := event["unit"].(string); ok && unit == "credit" {
			if usage, ok := event["usage"].(float64); ok {
				*s.upstreamCreditUsage = usage
				*s.hasUpstreamUsage = true
			}
		}
		if inputTokens, ok := event["inputTokens"].(float64); ok {
			s.totalUsage.InputTokens = int64(inputTokens)
			*s.hasUpstreamUsage = true
		}
		if outputTokens, ok := event["outputTokens"].(float64); ok {
			s.totalUsage.OutputTokens = int64(outputTokens)
			*s.hasUpstreamUsage = true
		}
		if eventType != "" {
			log.Debugf("kiro: streamToChannel unknown event type: %s", eventType)
		}
	}
}

// handleAssistantResponseEvent processes assistantResponseEvent
func (e *KiroExecutor) handleAssistantResponseEvent(ctx context.Context, event map[string]interface{}, s *streamState) {
	var contentDelta string
	var toolUses []map[string]interface{}

	if assistantResp, ok := event["assistantResponseEvent"].(map[string]interface{}); ok {
		if c, ok := assistantResp["content"].(string); ok {
			contentDelta = c
		}
		if sr := kirocommon.GetString(assistantResp, "stop_reason"); sr != "" {
			*s.upstreamStopReason = sr
		}
		if sr := kirocommon.GetString(assistantResp, "stopReason"); sr != "" {
			*s.upstreamStopReason = sr
		}
		if tus, ok := assistantResp["toolUses"].([]interface{}); ok {
			for _, tuRaw := range tus {
				if tu, ok := tuRaw.(map[string]interface{}); ok {
					toolUses = append(toolUses, tu)
				}
			}
		}
	}
	if contentDelta == "" {
		if c, ok := event["content"].(string); ok {
			contentDelta = c
		}
	}
	if tus, ok := event["toolUses"].([]interface{}); ok {
		for _, tuRaw := range tus {
			if tu, ok := tuRaw.(map[string]interface{}); ok {
				toolUses = append(toolUses, tu)
			}
		}
	}

	// Handle text content with thinking mode support
	if contentDelta != "" {
		*s.outputLen += len(contentDelta)
		s.accumulatedContent.WriteString(contentDelta)

		// Real-time usage estimation check
		shouldSendUsageUpdate := false
		if s.accumulatedContent.Len()-*s.lastUsageUpdateLen >= usageUpdateCharThreshold {
			shouldSendUsageUpdate = true
		} else if time.Since(*s.lastUsageUpdateTime) >= usageUpdateTimeInterval && s.accumulatedContent.Len() > *s.lastUsageUpdateLen {
			shouldSendUsageUpdate = true
		}

		if shouldSendUsageUpdate {
			var currentOutputTokens int64
			if enc, encErr := getTokenizer(s.model); encErr == nil {
				if tokenCount, countErr := enc.Count(s.accumulatedContent.String()); countErr == nil {
					currentOutputTokens = int64(tokenCount)
				}
			}
			if currentOutputTokens == 0 {
				currentOutputTokens = int64(s.accumulatedContent.Len() / 4)
				if currentOutputTokens == 0 {
					currentOutputTokens = 1
				}
			}

			if currentOutputTokens > *s.lastReportedOutputTokens+10 {
				pingEvent := kiroclaude.BuildClaudePingEventWithUsage(s.totalUsage.InputTokens, currentOutputTokens)
				sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, pingEvent, s.translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}
				*s.lastReportedOutputTokens = currentOutputTokens
			}

			*s.lastUsageUpdateLen = s.accumulatedContent.Len()
			*s.lastUsageUpdateTime = time.Now()
		}

		// Process thinking tags
		e.processThinkingContent(ctx, contentDelta, s)
	}

	// Handle tool uses
	for _, tu := range toolUses {
		toolUseID := kirocommon.GetString(tu, "toolUseId")
		toolName := kirocommon.GetString(tu, "name")

		if s.processedIDs[toolUseID] {
			continue
		}
		s.processedIDs[toolUseID] = true

		*s.hasToolUses = true
		e.emitToolUseBlock(ctx, tu, toolUseID, toolName, s)
	}
}

// handleReasoningContentEvent processes reasoningContentEvent
func (e *KiroExecutor) handleReasoningContentEvent(ctx context.Context, event map[string]interface{}, s *streamState) {
	var thinkingText string

	if re, ok := event["reasoningContentEvent"].(map[string]interface{}); ok {
		if text, ok := re["text"].(string); ok {
			thinkingText = text
		}
	} else {
		if text, ok := event["text"].(string); ok {
			thinkingText = text
		}
	}

	if thinkingText != "" {
		// Close text block if open
		if *s.isTextBlockOpen && *s.contentBlockIndex >= 0 {
			blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(*s.contentBlockIndex)
			sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
			for _, chunk := range sseData {
				if chunk != "" {
					s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
				}
			}
			*s.isTextBlockOpen = false
		}

		// Start thinking block if not open
		if !*s.isThinkingBlockOpen {
			*s.contentBlockIndex++
			*s.thinkingBlockIndex = *s.contentBlockIndex
			*s.isThinkingBlockOpen = true
			blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(*s.thinkingBlockIndex, "thinking", "", "")
			sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStart, s.translatorParam)
			for _, chunk := range sseData {
				if chunk != "" {
					s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
				}
			}
		}

		// Send thinking content
		thinkingEvent := kiroclaude.BuildClaudeThinkingDeltaEvent(thinkingText, *s.thinkingBlockIndex)
		sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, thinkingEvent, s.translatorParam)
		for _, chunk := range sseData {
			if chunk != "" {
				s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
			}
		}

		s.accumulatedThinkingContent.WriteString(thinkingText)
	}
}

// handleToolUseEvent processes toolUseEvent
func (e *KiroExecutor) handleToolUseEvent(ctx context.Context, event map[string]interface{}, s *streamState) {
	completedToolUses, newState := kiroclaude.ProcessToolUseEvent(event, *s.currentToolUse, s.processedIDs)
	*s.currentToolUse = newState

	for _, tu := range completedToolUses {
		*s.hasToolUses = true

		// Close text block if open
		if *s.isTextBlockOpen && *s.contentBlockIndex >= 0 {
			blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(*s.contentBlockIndex)
			sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
			for _, chunk := range sseData {
				if chunk != "" {
					s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
				}
			}
			*s.isTextBlockOpen = false
		}

		*s.contentBlockIndex++

		blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(*s.contentBlockIndex, "tool_use", tu.ToolUseID, tu.Name)
		sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStart, s.translatorParam)
		for _, chunk := range sseData {
			if chunk != "" {
				s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
			}
		}

		if tu.Input != nil {
			inputJSON, err := json.Marshal(tu.Input)
			if err == nil {
				inputDelta := kiroclaude.BuildClaudeInputJsonDeltaEvent(string(inputJSON), *s.contentBlockIndex)
				sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, inputDelta, s.translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}
			}
		}

		blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(*s.contentBlockIndex)
		sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
		for _, chunk := range sseData {
			if chunk != "" {
				s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
			}
		}
	}
}

// handleUsageEvent processes usage-related events
func (e *KiroExecutor) handleUsageEvent(event map[string]interface{}, eventType string, s *streamState) {
	switch eventType {
	case "supplementaryWebLinksEvent":
		if inputTokens, ok := event["inputTokens"].(float64); ok {
			s.totalUsage.InputTokens = int64(inputTokens)
		}
		if outputTokens, ok := event["outputTokens"].(float64); ok {
			s.totalUsage.OutputTokens = int64(outputTokens)
		}

	case "messageMetadataEvent", "metadataEvent":
		var metadata map[string]interface{}
		if m, ok := event["messageMetadataEvent"].(map[string]interface{}); ok {
			metadata = m
		} else if m, ok := event["metadataEvent"].(map[string]interface{}); ok {
			metadata = m
		} else {
			metadata = event
		}

		if tokenUsage, ok := metadata["tokenUsage"].(map[string]interface{}); ok {
			if outputTokens, ok := tokenUsage["outputTokens"].(float64); ok {
				s.totalUsage.OutputTokens = int64(outputTokens)
				*s.hasUpstreamUsage = true
			}
			if totalTokens, ok := tokenUsage["totalTokens"].(float64); ok {
				s.totalUsage.TotalTokens = int64(totalTokens)
			}
			if uncachedInputTokens, ok := tokenUsage["uncachedInputTokens"].(float64); ok {
				s.totalUsage.InputTokens = int64(uncachedInputTokens)
				*s.hasUpstreamUsage = true
			}
			if cacheReadTokens, ok := tokenUsage["cacheReadInputTokens"].(float64); ok {
				if s.totalUsage.InputTokens > 0 {
					s.totalUsage.InputTokens += int64(cacheReadTokens)
				} else {
					s.totalUsage.InputTokens = int64(cacheReadTokens)
				}
				*s.hasUpstreamUsage = true
			}
			if ctxPct, ok := tokenUsage["contextUsagePercentage"].(float64); ok {
				*s.upstreamContextPercentage = ctxPct
			}
		}

	case "usageEvent", "usage":
		if inputTokens, ok := event["inputTokens"].(float64); ok {
			s.totalUsage.InputTokens = int64(inputTokens)
		}
		if outputTokens, ok := event["outputTokens"].(float64); ok {
			s.totalUsage.OutputTokens = int64(outputTokens)
		}
		if totalTokens, ok := event["totalTokens"].(float64); ok {
			s.totalUsage.TotalTokens = int64(totalTokens)
		}
		if usageObj, ok := event["usage"].(map[string]interface{}); ok {
			if inputTokens, ok := usageObj["input_tokens"].(float64); ok {
				s.totalUsage.InputTokens = int64(inputTokens)
			}
			if outputTokens, ok := usageObj["output_tokens"].(float64); ok {
				s.totalUsage.OutputTokens = int64(outputTokens)
			}
		}

	case "metricsEvent":
		if metrics, ok := event["metricsEvent"].(map[string]interface{}); ok {
			if inputTokens, ok := metrics["inputTokens"].(float64); ok {
				s.totalUsage.InputTokens = int64(inputTokens)
			}
			if outputTokens, ok := metrics["outputTokens"].(float64); ok {
				s.totalUsage.OutputTokens = int64(outputTokens)
			}
		}
	}
}

// processThinkingContent processes content for thinking tags
func (e *KiroExecutor) processThinkingContent(ctx context.Context, contentDelta string, s *streamState) {
	s.pendingContent.WriteString(contentDelta)
	processContent := s.pendingContent.String()
	s.pendingContent.Reset()

	for len(processContent) > 0 {
		if *s.inThinkBlock {
			endIdx := strings.Index(processContent, kirocommon.ThinkingEndTag)
			if endIdx >= 0 {
				thinkingText := processContent[:endIdx]
				if thinkingText != "" {
					if !*s.isThinkingBlockOpen {
						*s.contentBlockIndex++
						*s.thinkingBlockIndex = *s.contentBlockIndex
						*s.isThinkingBlockOpen = true
						blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(*s.thinkingBlockIndex, "thinking", "", "")
						sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStart, s.translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
					}
					thinkingEvent := kiroclaude.BuildClaudeThinkingDeltaEvent(thinkingText, *s.thinkingBlockIndex)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, thinkingEvent, s.translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
					s.accumulatedThinkingContent.WriteString(thinkingText)
				}
				if *s.isThinkingBlockOpen {
					blockStop := kiroclaude.BuildClaudeThinkingBlockStopEvent(*s.thinkingBlockIndex)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
					*s.isThinkingBlockOpen = false
				}
				*s.inThinkBlock = false
				processContent = processContent[endIdx+len(kirocommon.ThinkingEndTag):]
			} else {
				// Check for partial tag at end
				partialMatch := false
				for i := 1; i < len(kirocommon.ThinkingEndTag) && i <= len(processContent); i++ {
					if strings.HasSuffix(processContent, kirocommon.ThinkingEndTag[:i]) {
						s.pendingContent.WriteString(processContent[len(processContent)-i:])
						processContent = processContent[:len(processContent)-i]
						partialMatch = true
						break
					}
				}
				if !partialMatch || len(processContent) > 0 {
					if processContent != "" {
						if !*s.isThinkingBlockOpen {
							*s.contentBlockIndex++
							*s.thinkingBlockIndex = *s.contentBlockIndex
							*s.isThinkingBlockOpen = true
							blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(*s.thinkingBlockIndex, "thinking", "", "")
							sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStart, s.translatorParam)
							for _, chunk := range sseData {
								if chunk != "" {
									s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
								}
							}
						}
						thinkingEvent := kiroclaude.BuildClaudeThinkingDeltaEvent(processContent, *s.thinkingBlockIndex)
						sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, thinkingEvent, s.translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
						s.accumulatedThinkingContent.WriteString(processContent)
					}
				}
				processContent = ""
			}
		} else {
			startIdx := strings.Index(processContent, kirocommon.ThinkingStartTag)
			if startIdx >= 0 {
				textBefore := processContent[:startIdx]
				if textBefore != "" {
					if *s.isThinkingBlockOpen {
						blockStop := kiroclaude.BuildClaudeThinkingBlockStopEvent(*s.thinkingBlockIndex)
						sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
						*s.isThinkingBlockOpen = false
					}
					if !*s.isTextBlockOpen {
						*s.contentBlockIndex++
						*s.isTextBlockOpen = true
						blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(*s.contentBlockIndex, "text", "", "")
						sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStart, s.translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
					}
					claudeEvent := kiroclaude.BuildClaudeStreamEvent(textBefore, *s.contentBlockIndex)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, claudeEvent, s.translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
				}
				if *s.isTextBlockOpen {
					blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(*s.contentBlockIndex)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
					*s.isTextBlockOpen = false
				}
				*s.inThinkBlock = true
				processContent = processContent[startIdx+len(kirocommon.ThinkingStartTag):]
			} else {
				// Check for partial tag at end
				partialMatch := false
				for i := 1; i < len(kirocommon.ThinkingStartTag) && i <= len(processContent); i++ {
					if strings.HasSuffix(processContent, kirocommon.ThinkingStartTag[:i]) {
						s.pendingContent.WriteString(processContent[len(processContent)-i:])
						processContent = processContent[:len(processContent)-i]
						partialMatch = true
						break
					}
				}
				if !partialMatch || len(processContent) > 0 {
					if processContent != "" {
						if !*s.isTextBlockOpen {
							*s.contentBlockIndex++
							*s.isTextBlockOpen = true
							blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(*s.contentBlockIndex, "text", "", "")
							sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStart, s.translatorParam)
							for _, chunk := range sseData {
								if chunk != "" {
									s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
								}
							}
						}
						claudeEvent := kiroclaude.BuildClaudeStreamEvent(processContent, *s.contentBlockIndex)
						sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, claudeEvent, s.translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
					}
				}
				processContent = ""
			}
		}
	}
}

// emitToolUseBlock emits a tool_use content block
func (e *KiroExecutor) emitToolUseBlock(ctx context.Context, tu map[string]interface{}, toolUseID, toolName string, s *streamState) {
	// Close text block if open
	if *s.isTextBlockOpen && *s.contentBlockIndex >= 0 {
		blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(*s.contentBlockIndex)
		sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
		for _, chunk := range sseData {
			if chunk != "" {
				s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
			}
		}
		*s.isTextBlockOpen = false
	}

	*s.contentBlockIndex++

	blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(*s.contentBlockIndex, "tool_use", toolUseID, toolName)
	sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStart, s.translatorParam)
	for _, chunk := range sseData {
		if chunk != "" {
			s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
		}
	}

	if input, ok := tu["input"].(map[string]interface{}); ok {
		inputJSON, err := json.Marshal(input)
		if err == nil {
			inputDelta := kiroclaude.BuildClaudeInputJsonDeltaEvent(string(inputJSON), *s.contentBlockIndex)
			sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, inputDelta, s.translatorParam)
			for _, chunk := range sseData {
				if chunk != "" {
					s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
				}
			}
		}
	}

	blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(*s.contentBlockIndex)
	sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), s.targetFormat, s.model, s.originalReq, s.claudeBody, blockStop, s.translatorParam)
	for _, chunk := range sseData {
		if chunk != "" {
			s.out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
		}
	}
}
