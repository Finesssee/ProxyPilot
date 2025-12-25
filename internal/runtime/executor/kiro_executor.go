// Package executor provides runtime execution capabilities for various AI service providers.
package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	kiroauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	kiroEndpoint    = "https://q.us-east-1.amazonaws.com"
	kiroContentType = "application/x-amz-json-1.0"
	kiroTarget      = "AmazonCodeWhispererStreamingService.GenerateAssistantResponse"
)

// KiroExecutor is an executor for Kiro (Amazon Q).
// Kiro uses AWS Event Stream binary format instead of standard SSE.
type KiroExecutor struct {
	cfg       *config.Config
	refreshMu sync.Mutex
}

// NewKiroExecutor creates a new Kiro executor instance.
func NewKiroExecutor(cfg *config.Config) *KiroExecutor {
	return &KiroExecutor{cfg: cfg}
}

// Identifier returns the executor identifier.
func (e *KiroExecutor) Identifier() string { return "kiro" }

// PrepareRequest prepares the HTTP request for execution.
func (e *KiroExecutor) PrepareRequest(_ *http.Request, _ *cliproxyauth.Auth) error { return nil }

// Execute performs a non-streaming request to the Kiro API.
func (e *KiroExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (resp cliproxyexecutor.Response, err error) {
	// Ensure token is valid before making the request
	token, updatedAuth, err := e.ensureValidToken(ctx, auth)
	if err != nil {
		return resp, err
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	upstreamModel := util.ResolveOriginalModel(req.Model, req.Metadata)

	// Translate request to Claude format, then wrap in Kiro payload structure
	from := opts.SourceFormat
	to := sdktranslator.FromString("claude")
	claudeBody := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), false)
	claudeBody = applyPayloadConfig(e.cfg, req.Model, claudeBody)
	claudeBody, _ = sjson.SetBytes(claudeBody, "model", upstreamModel)
	claudeBody, _ = sjson.DeleteBytes(claudeBody, "stream")

	// Try AI_EDITOR origin first, fallback to CLI on quota exceeded
	origins := []string{"AI_EDITOR", "CLI"}
	var lastErr error
	var lastStatusCode int

	for _, origin := range origins {
		// Wrap Claude-format payload in Kiro structure
		kiroBody, err := buildKiroPayload(claudeBody, auth, origin)
		if err != nil {
			return resp, err
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, kiroEndpoint, bytes.NewReader(kiroBody))
		if err != nil {
			return resp, err
		}
		applyKiroHeaders(httpReq, auth, token)

		var authID, authLabel, authType, authValue string
		if auth != nil {
			authID = auth.ID
			authLabel = auth.Label
			authType, authValue = auth.AccountInfo()
		}
		recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
			URL:       kiroEndpoint,
			Method:    http.MethodPost,
			Headers:   httpReq.Header.Clone(),
			Body:      kiroBody,
			Provider:  e.Identifier(),
			AuthID:    authID,
			AuthLabel: authLabel,
			AuthType:  authType,
			AuthValue: authValue,
		})

		httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
		httpResp, err := httpClient.Do(httpReq)
		if err != nil {
			recordAPIResponseError(ctx, e.cfg, err)
			return resp, err
		}
		defer func() {
			if errClose := httpResp.Body.Close(); errClose != nil {
				log.Errorf("kiro executor: close response body error: %v", errClose)
			}
		}()
		recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())

		// Handle error responses
		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			b, _ := io.ReadAll(httpResp.Body)
			appendAPIResponseChunk(ctx, e.cfg, b)
			log.Debugf("request error, error status: %d, error body: %s", httpResp.StatusCode, summarizeErrorBody(httpResp.Header.Get("Content-Type"), b))

			lastStatusCode = httpResp.StatusCode
			lastErr = statusErr{code: httpResp.StatusCode, msg: string(b)}

			// On 401/403, try refreshing token and retry once
			if httpResp.StatusCode == http.StatusUnauthorized || httpResp.StatusCode == http.StatusForbidden {
				refreshedAuth, refreshErr := e.Refresh(ctx, auth)
				if refreshErr == nil && refreshedAuth != nil {
					auth = refreshedAuth
					token = kiroToken(auth)
					log.Debugf("kiro executor: token refreshed, retrying request")
					continue
				}
			}

			// On 429 with AI_EDITOR, try CLI origin
			if httpResp.StatusCode == http.StatusTooManyRequests && origin == "AI_EDITOR" {
				log.Debugf("kiro executor: quota exceeded with AI_EDITOR origin, retrying with CLI origin")
				continue
			}

			return resp, lastErr
		}

		// Read and parse AWS Event Stream binary format
		data, err := io.ReadAll(httpResp.Body)
		if err != nil {
			recordAPIResponseError(ctx, e.cfg, err)
			return resp, err
		}
		appendAPIResponseChunk(ctx, e.cfg, data)

		// Parse AWS Event Stream and extract content
		var fullContent bytes.Buffer
		var usageData []byte
		events, err := parseAWSEventStream(data)
		if err != nil {
			return resp, fmt.Errorf("failed to parse AWS event stream: %w", err)
		}

		for _, event := range events {
			eventType := event.EventType
			payload := event.Payload

			switch eventType {
			case "assistantResponseEvent":
				// Extract content from the event
				content := gjson.GetBytes(payload, "content").String()
				fullContent.WriteString(content)
			case "toolUseEvent":
				// Handle tool calls if needed
				log.Debugf("kiro: received tool use event")
			case "supplementaryWebLinksEvent":
				// Store usage info for later
				usageData = payload
			}
		}

		// Parse usage if available
		if len(usageData) > 0 {
			// TODO: Add Kiro-specific usage parsing if format is known
			// reporter.publish(ctx, parseKiroUsage(usageData))
		}

		// Build Claude-format response and translate back
		claudeResp := map[string]any{
			"id":      "kiro-response",
			"type":    "message",
			"role":    "assistant",
			"content": []map[string]string{{"type": "text", "text": fullContent.String()}},
			"model":   upstreamModel,
		}
		claudeRespJSON, _ := sjson.SetBytes(nil, "", claudeResp)

		var param any
		out := sdktranslator.TranslateNonStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), claudeBody, claudeRespJSON, &param)
		resp = cliproxyexecutor.Response{Payload: []byte(out)}
		return resp, nil
	}

	// If we get here, all retries failed
	if lastErr != nil {
		return resp, lastErr
	}
	return resp, statusErr{code: lastStatusCode, msg: "all retry attempts failed"}
}

// ExecuteStream performs a streaming request to the Kiro API.
func (e *KiroExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (stream <-chan cliproxyexecutor.StreamChunk, err error) {
	// Ensure token is valid before making the request
	token, updatedAuth, err := e.ensureValidToken(ctx, auth)
	if err != nil {
		return nil, err
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	upstreamModel := util.ResolveOriginalModel(req.Model, req.Metadata)

	// Translate request to Claude format, then wrap in Kiro payload structure
	from := opts.SourceFormat
	to := sdktranslator.FromString("claude")
	claudeBody := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)
	claudeBody = applyPayloadConfig(e.cfg, req.Model, claudeBody)
	claudeBody, _ = sjson.SetBytes(claudeBody, "model", upstreamModel)
	claudeBody, _ = sjson.SetBytes(claudeBody, "stream", true)

	// Try AI_EDITOR origin first, fallback to CLI on quota exceeded
	origins := []string{"AI_EDITOR", "CLI"}
	var lastErr error
	var lastStatusCode int

	for _, origin := range origins {
		// Wrap Claude-format payload in Kiro structure
		kiroBody, err := buildKiroPayload(claudeBody, auth, origin)
		if err != nil {
			return nil, err
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, kiroEndpoint, bytes.NewReader(kiroBody))
		if err != nil {
			return nil, err
		}
		applyKiroHeaders(httpReq, auth, token)

		var authID, authLabel, authType, authValue string
		if auth != nil {
			authID = auth.ID
			authLabel = auth.Label
			authType, authValue = auth.AccountInfo()
		}
		recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
			URL:       kiroEndpoint,
			Method:    http.MethodPost,
			Headers:   httpReq.Header.Clone(),
			Body:      kiroBody,
			Provider:  e.Identifier(),
			AuthID:    authID,
			AuthLabel: authLabel,
			AuthType:  authType,
			AuthValue: authValue,
		})

		httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
		httpResp, err := httpClient.Do(httpReq)
		if err != nil {
			recordAPIResponseError(ctx, e.cfg, err)
			return nil, err
		}

		recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())
		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			b, _ := io.ReadAll(httpResp.Body)
			appendAPIResponseChunk(ctx, e.cfg, b)
			log.Debugf("request error, error status: %d, error body: %s", httpResp.StatusCode, summarizeErrorBody(httpResp.Header.Get("Content-Type"), b))
			if errClose := httpResp.Body.Close(); errClose != nil {
				log.Errorf("kiro executor: close response body error: %v", errClose)
			}

			lastStatusCode = httpResp.StatusCode
			lastErr = statusErr{code: httpResp.StatusCode, msg: string(b)}

			// On 401/403, try refreshing token and retry once
			if httpResp.StatusCode == http.StatusUnauthorized || httpResp.StatusCode == http.StatusForbidden {
				refreshedAuth, refreshErr := e.Refresh(ctx, auth)
				if refreshErr == nil && refreshedAuth != nil {
					auth = refreshedAuth
					token = kiroToken(auth)
					log.Debugf("kiro executor: token refreshed, retrying stream request")
					continue
				}
			}

			// On 429 with AI_EDITOR, try CLI origin
			if httpResp.StatusCode == http.StatusTooManyRequests && origin == "AI_EDITOR" {
				log.Debugf("kiro executor: quota exceeded with AI_EDITOR origin, retrying stream with CLI origin")
				continue
			}

			return nil, lastErr
		}

		out := make(chan cliproxyexecutor.StreamChunk)
		stream = out
		go func() {
			defer close(out)
			defer func() {
				if errClose := httpResp.Body.Close(); errClose != nil {
					log.Errorf("kiro executor: close response body error: %v", errClose)
				}
			}()

			// AWS Event Stream parser for binary format
			reader := bufio.NewReader(httpResp.Body)
			var param any

			for {
				// Parse single AWS Event Stream message
				event, err := parseAWSEventStreamMessage(reader)
				if err != nil {
					if err == io.EOF {
						break
					}
					recordAPIResponseError(ctx, e.cfg, err)
					reporter.publishFailure(ctx)
					out <- cliproxyexecutor.StreamChunk{Err: err}
					break
				}

				appendAPIResponseChunk(ctx, e.cfg, event.Payload)

				eventType := event.EventType
				payload := event.Payload

				switch eventType {
				case "assistantResponseEvent":
					// Extract content and create Claude-format chunk
					content := gjson.GetBytes(payload, "content").String()
					if content == "" {
						continue
					}

					// Build Claude-format delta chunk
					claudeDelta := map[string]any{
						"type": "content_block_delta",
						"delta": map[string]string{
							"type": "text_delta",
							"text": content,
						},
					}
					claudeDeltaJSON, _ := sjson.SetBytes(nil, "", claudeDelta)

					chunks := sdktranslator.TranslateStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), claudeBody, claudeDeltaJSON, &param)
					for i := range chunks {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunks[i])}
					}

				case "toolUseEvent":
					// Handle tool calls if needed
					log.Debugf("kiro: received tool use event in stream")

				case "supplementaryWebLinksEvent":
					// Parse usage info
					// TODO: Add Kiro-specific usage parsing if format is known
					// if detail, ok := parseKiroUsage(payload); ok {
					// 	reporter.publish(ctx, detail)
					// }
				}
			}

			reporter.ensurePublished(ctx)
		}()
		return stream, nil
	}

	// If we get here, all retries failed
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, statusErr{code: lastStatusCode, msg: "all retry attempts failed"}
}

// CountTokens counts tokens for the given request (not supported for Kiro).
func (e *KiroExecutor) CountTokens(context.Context, *cliproxyauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "count tokens not supported"}
}

// Embed performs an embedding request (not supported for Kiro).
func (e *KiroExecutor) Embed(context.Context, *cliproxyauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "embeddings not supported"}
}

// Refresh refreshes the authentication credentials.
func (e *KiroExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	log.Debugf("kiro executor: refresh called")
	if auth == nil {
		return nil, statusErr{code: http.StatusInternalServerError, msg: "kiro executor: auth is nil"}
	}

	// Extract client credentials from metadata
	var clientID, clientSecret, refreshToken string
	if auth.Metadata != nil {
		if v, ok := auth.Metadata["client_id"].(string); ok && v != "" {
			clientID = v
		}
		if v, ok := auth.Metadata["client_secret"].(string); ok && v != "" {
			clientSecret = v
		}
		if v, ok := auth.Metadata["refresh_token"].(string); ok && v != "" {
			refreshToken = v
		}
	}

	if refreshToken == "" {
		log.Debugf("kiro executor: missing refresh token for refresh, returning auth as-is")
		return auth, nil
	}

	// Build token data for refresh
	oldTokenData := &kiroauth.KiroTokenData{
		RefreshToken: refreshToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthMethod:   "builder_id", // Default to builder_id
	}
	if method, ok := auth.Metadata["auth_method"].(string); ok && method != "" {
		oldTokenData.AuthMethod = method
	}

	// Use KiroAuth to refresh tokens
	svc := kiroauth.NewKiroAuth(e.cfg)
	tokenData, err := svc.RefreshTokensWithRetry(ctx, oldTokenData, 3)
	if err != nil {
		return nil, fmt.Errorf("kiro executor: token refresh failed: %w", err)
	}

	// Update auth metadata with new tokens
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["access_token"] = tokenData.AccessToken
	if tokenData.RefreshToken != "" {
		auth.Metadata["refresh_token"] = tokenData.RefreshToken
	}
	auth.Metadata["expired"] = tokenData.ExpiresAt
	auth.Metadata["type"] = "kiro"
	auth.Metadata["last_refresh"] = time.Now().Format(time.RFC3339)

	if tokenData.Email != "" {
		auth.Metadata["email"] = tokenData.Email
	}
	if tokenData.ProfileArn != "" {
		auth.Metadata["profile_arn"] = tokenData.ProfileArn
	}

	return auth, nil
}

// ensureValidToken checks if the token is expired and refreshes it if needed.
// Returns the access token, potentially updated auth, and any error.
func (e *KiroExecutor) ensureValidToken(ctx context.Context, auth *cliproxyauth.Auth) (string, *cliproxyauth.Auth, error) {
	if auth == nil {
		return "", nil, statusErr{code: http.StatusUnauthorized, msg: "missing auth"}
	}

	token := kiroToken(auth)
	if token == "" {
		return "", nil, statusErr{code: http.StatusUnauthorized, msg: "missing access token"}
	}

	// Check if token is expired
	expiry := kiroTokenExpiry(auth.Metadata)
	if !expiry.IsZero() && expiry.Before(time.Now()) {
		log.Debugf("kiro executor: token expired, refreshing")
		refreshedAuth, err := e.Refresh(ctx, auth)
		if err != nil {
			return "", nil, err
		}
		return kiroToken(refreshedAuth), refreshedAuth, nil
	}

	return token, nil, nil
}

// kiroTokenExpiry extracts the token expiry time from metadata.
func kiroTokenExpiry(metadata map[string]any) time.Time {
	if metadata == nil {
		return time.Time{}
	}
	if expStr, ok := metadata["expired"].(string); ok {
		expStr = strings.TrimSpace(expStr)
		if expStr != "" {
			if parsed, err := time.Parse(time.RFC3339, expStr); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

// kiroToken extracts the access token from auth metadata.
func kiroToken(a *cliproxyauth.Auth) string {
	if a == nil {
		return ""
	}
	if a.Metadata != nil {
		if v, ok := a.Metadata["access_token"].(string); ok && v != "" {
			return v
		}
		if v, ok := a.Metadata["bearer_token"].(string); ok && v != "" {
			return v
		}
	}
	if a.Attributes != nil {
		if v := a.Attributes["api_key"]; v != "" {
			return v
		}
	}
	return ""
}

// applyKiroHeaders sets the required headers for Kiro API requests.
func applyKiroHeaders(r *http.Request, auth *cliproxyauth.Auth, token string) {
	r.Header.Set("Content-Type", kiroContentType)
	r.Header.Set("x-amz-target", kiroTarget)

	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}

	// Apply custom headers from auth attributes
	var attrs map[string]string
	if auth != nil {
		attrs = auth.Attributes
	}
	util.ApplyCustomHeadersFromAttrs(r, attrs)
}

// buildKiroPayload wraps a Claude-format request in Kiro's payload structure.
func buildKiroPayload(claudeBody []byte, auth *cliproxyauth.Auth, origin string) ([]byte, error) {
	// Extract profile ARN from auth if available
	profileArn := ""
	if auth != nil && auth.Metadata != nil {
		if v, ok := auth.Metadata["profile_arn"].(string); ok {
			profileArn = v
		}
	}

	// Default to AI_EDITOR if no origin specified
	if origin == "" {
		origin = "AI_EDITOR"
	}

	// Build Kiro payload structure
	kiroPayload := map[string]any{
		"conversationState": map[string]any{
			"currentMessage":  gjson.ParseBytes(claudeBody).Value(),
			"chatTriggerType": "MANUAL",
		},
		"source": "FeatureDev",
		"origin": origin,
	}

	if profileArn != "" {
		kiroPayload["profileArn"] = profileArn
	}

	payload, err := sjson.SetBytes(nil, "", kiroPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to build kiro payload: %w", err)
	}

	return payload, nil
}

// AWSEventStreamEvent represents a parsed AWS Event Stream message.
type AWSEventStreamEvent struct {
	EventType string
	Payload   []byte
}

// parseAWSEventStream parses a complete AWS Event Stream binary response.
func parseAWSEventStream(data []byte) ([]AWSEventStreamEvent, error) {
	var events []AWSEventStreamEvent
	reader := bytes.NewReader(data)
	bufReader := bufio.NewReader(reader)

	for {
		event, err := parseAWSEventStreamMessage(bufReader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return events, err
		}
		events = append(events, event)
	}

	return events, nil
}

// parseAWSEventStreamMessage parses a single AWS Event Stream message from a reader.
// AWS Event Stream format:
// [4 bytes: total_length (big-endian)]
// [4 bytes: headers_length (big-endian)]
// [headers section]
// [4 bytes: prelude_checksum (CRC32)]
// [payload]
// [4 bytes: message_checksum (CRC32)]
func parseAWSEventStreamMessage(reader *bufio.Reader) (AWSEventStreamEvent, error) {
	var event AWSEventStreamEvent

	// Read prelude (total_length + headers_length)
	prelude := make([]byte, 8)
	if _, err := io.ReadFull(reader, prelude); err != nil {
		return event, err
	}

	totalLength := binary.BigEndian.Uint32(prelude[0:4])
	headersLength := binary.BigEndian.Uint32(prelude[4:8])

	// Read prelude CRC
	preludeCRC := make([]byte, 4)
	if _, err := io.ReadFull(reader, preludeCRC); err != nil {
		return event, err
	}

	// Verify prelude CRC
	expectedPreludeCRC := crc32.ChecksumIEEE(prelude)
	actualPreludeCRC := binary.BigEndian.Uint32(preludeCRC)
	if expectedPreludeCRC != actualPreludeCRC {
		return event, fmt.Errorf("prelude CRC mismatch: expected %d, got %d", expectedPreludeCRC, actualPreludeCRC)
	}

	// Read headers
	headers := make([]byte, headersLength)
	if _, err := io.ReadFull(reader, headers); err != nil {
		return event, err
	}

	// Parse headers to extract event type
	eventType := parseAWSHeaders(headers)
	event.EventType = eventType

	// Calculate payload length
	// total_length includes: prelude(8) + prelude_crc(4) + headers + payload + message_crc(4)
	payloadLength := int(totalLength) - 8 - 4 - int(headersLength) - 4

	// Read payload
	if payloadLength > 0 {
		payload := make([]byte, payloadLength)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return event, err
		}
		event.Payload = payload
	}

	// Read message CRC
	messageCRC := make([]byte, 4)
	if _, err := io.ReadFull(reader, messageCRC); err != nil {
		return event, err
	}

	// Verify message CRC (includes entire message except the CRC itself)
	messageData := make([]byte, 0, int(totalLength)-4)
	messageData = append(messageData, prelude...)
	messageData = append(messageData, preludeCRC...)
	messageData = append(messageData, headers...)
	messageData = append(messageData, event.Payload...)

	expectedMessageCRC := crc32.ChecksumIEEE(messageData)
	actualMessageCRC := binary.BigEndian.Uint32(messageCRC)
	if expectedMessageCRC != actualMessageCRC {
		return event, fmt.Errorf("message CRC mismatch: expected %d, got %d", expectedMessageCRC, actualMessageCRC)
	}

	return event, nil
}

// parseAWSHeaders extracts the event type from AWS Event Stream headers.
// Headers are encoded as: [header_name_length:1][header_name][header_value_type:1][header_value_length:2][header_value]
func parseAWSHeaders(headers []byte) string {
	eventType := ""
	offset := 0

	for offset < len(headers) {
		// Read header name length
		if offset >= len(headers) {
			break
		}
		nameLen := int(headers[offset])
		offset++

		// Read header name
		if offset+nameLen > len(headers) {
			break
		}
		name := string(headers[offset : offset+nameLen])
		offset += nameLen

		// Read header value type
		if offset >= len(headers) {
			break
		}
		valueType := headers[offset]
		offset++

		// Read header value length (2 bytes big-endian for type 7 = string)
		if valueType == 7 {
			if offset+2 > len(headers) {
				break
			}
			valueLen := int(binary.BigEndian.Uint16(headers[offset : offset+2]))
			offset += 2

			// Read header value
			if offset+valueLen > len(headers) {
				break
			}
			value := string(headers[offset : offset+valueLen])
			offset += valueLen

			// Check if this is the event-type header
			if name == ":event-type" {
				eventType = value
			}
		} else {
			// Unsupported header value type, skip
			break
		}
	}

	return eventType
}
