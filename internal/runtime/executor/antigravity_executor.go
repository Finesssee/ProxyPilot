// Package executor provides runtime execution capabilities for various AI service providers.
// This file implements the Antigravity executor that proxies requests to the antigravity
// upstream using OAuth credentials.
package executor

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/agentdebug"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	antigravityBaseURLDaily        = "https://daily-cloudcode-pa.googleapis.com"
	antigravitySandboxBaseURLDaily = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	antigravityBaseURLProd         = "https://cloudcode-pa.googleapis.com"
	antigravityCountTokensPath     = "/v1internal:countTokens"
	antigravityStreamPath          = "/v1internal:streamGenerateContent"
	antigravityGeneratePath        = "/v1internal:generateContent"
	antigravityModelsPath          = "/v1internal:fetchAvailableModels"
	antigravityClientID            = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityClientSecret        = "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"
	defaultAntigravityAgent        = "antigravity/1.11.5 windows/amd64"
	antigravityAuthType            = "antigravity"
	refreshSkew                    = 3000 * time.Second
	antigravityXGoogAPIClient      = "gl-node/22.17.0"
	antigravityClientMetadata      = "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI"
	systemInstruction              = "You are Antigravity, a powerful agentic AI coding assistant designed by the Google Deepmind team working on Advanced Agentic Coding.You are pair programming with a USER to solve their coding task. The task may require creating a new codebase, modifying or debugging an existing codebase, or simply answering a question.**Absolute paths only****Proactiveness**"
)

var (
	randSource      = rand.New(rand.NewSource(time.Now().UnixNano()))
	randSourceMutex sync.Mutex
)

// AntigravityExecutor proxies requests to the antigravity upstream.
type AntigravityExecutor struct {
	cfg *config.Config
}

// NewAntigravityExecutor creates a new Antigravity executor instance.
//
// Parameters:
//   - cfg: The application configuration
//
// Returns:
//   - *AntigravityExecutor: A new Antigravity executor instance
func NewAntigravityExecutor(cfg *config.Config) *AntigravityExecutor {
	return &AntigravityExecutor{cfg: cfg}
}

// Identifier returns the executor identifier.
func (e *AntigravityExecutor) Identifier() string { return antigravityAuthType }

// PrepareRequest injects Antigravity credentials into the outgoing HTTP request.
func (e *AntigravityExecutor) PrepareRequest(req *http.Request, auth *cliproxyauth.Auth) error {
	if req == nil {
		return nil
	}
	token, _, errToken := e.ensureAccessToken(req.Context(), auth)
	if errToken != nil {
		return errToken
	}
	if strings.TrimSpace(token) == "" {
		return statusErr{code: http.StatusUnauthorized, msg: "missing access token"}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// HttpRequest injects Antigravity credentials into the request and executes it.
func (e *AntigravityExecutor) HttpRequest(ctx context.Context, auth *cliproxyauth.Auth, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("antigravity executor: request is nil")
	}
	if ctx == nil {
		ctx = req.Context()
	}
	httpReq := req.WithContext(ctx)
	if err := e.PrepareRequest(httpReq, auth); err != nil {
		return nil, err
	}
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	return httpClient.Do(httpReq)
}

// Execute performs a non-streaming request to the Antigravity API.
func (e *AntigravityExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (resp cliproxyexecutor.Response, err error) {
	isClaude := strings.Contains(strings.ToLower(req.Model), "claude")
	if isClaude || strings.Contains(req.Model, "gemini-3-pro") {
		return e.executeClaudeNonStream(ctx, auth, req, opts)
	}

	token, updatedAuth, errToken := e.ensureAccessToken(ctx, auth)
	if errToken != nil {
		return resp, errToken
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	from := opts.SourceFormat
	to := sdktranslator.FromString("antigravity")
	switchProject := e.cfg != nil && e.cfg.QuotaExceeded.SwitchProject
	switchPreviewModel := e.cfg != nil && e.cfg.QuotaExceeded.SwitchPreviewModel

	projects := projectIDCandidatesFromAuth(auth)
	if !switchProject && len(projects) > 1 {
		projects = projects[:1]
	}
	if len(projects) == 0 {
		projects = []string{""}
	}

	models := []string{req.Model}
	if switchPreviewModel {
		models = append(models, quotaPreviewFallbackOrder(req.Model)...)
	}
	{
		seen := make(map[string]struct{}, len(models))
		uniq := make([]string, 0, len(models))
		for _, m := range models {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			uniq = append(uniq, m)
		}
		models = uniq
	}

	baseURLs := antigravityBaseURLFallbackOrder(auth)
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)

	var lastStatus int
	var lastBody []byte
	var lastErr error

	for midx, attemptModel := range models {
		translated := sdktranslator.TranslateRequest(from, to, attemptModel, bytes.Clone(req.Payload), false)
		translated = applyThinkingMetadataCLI(translated, req.Metadata, attemptModel)
		translated = util.ApplyGemini3ThinkingLevelFromMetadataCLI(attemptModel, req.Metadata, translated)
		translated = util.ApplyDefaultThinkingIfNeededCLI(attemptModel, translated)
		translated = normalizeAntigravityThinking(attemptModel, translated)
		translated = applyPayloadConfigWithRoot(e.cfg, attemptModel, "antigravity", "request", translated, nil)

		for bidx, baseURL := range baseURLs {
			for pidx, projectID := range projects {
				httpReq, errReq := e.buildRequest(ctx, auth, token, attemptModel, projectID, translated, false, opts.Alt, baseURL)
				if errReq != nil {
					err = errReq
					return resp, err
				}

				httpResp, errDo := httpClient.Do(httpReq)
				if errDo != nil {
					recordAPIResponseError(ctx, e.cfg, errDo)
					lastStatus = 0
					lastBody = nil
					lastErr = errDo
					if bidx+1 < len(baseURLs) {
						log.Debugf("antigravity executor: request error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[bidx+1])
						continue
					}
					err = errDo
					return resp, err
				}

				recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())
				bodyBytes, errRead := io.ReadAll(httpResp.Body)
				if errClose := httpResp.Body.Close(); errClose != nil {
					log.Errorf("antigravity executor: close response body error: %v", errClose)
				}
				if errRead != nil {
					recordAPIResponseError(ctx, e.cfg, errRead)
					err = errRead
					return resp, err
				}
				appendAPIResponseChunk(ctx, e.cfg, bodyBytes)

				if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
					log.Debugf("antigravity executor: upstream error status: %d, body: %s", httpResp.StatusCode, summarizeErrorBody(httpResp.Header.Get("Content-Type"), bodyBytes))
					lastStatus = httpResp.StatusCode
					lastBody = append([]byte(nil), bodyBytes...)
					lastErr = nil
					if httpResp.StatusCode == http.StatusTooManyRequests {
						if pidx+1 < len(projects) {
							log.Debugf("antigravity executor: rate limited on project %s, retrying with next project: %s", projectID, projects[pidx+1])
							continue
						}
						if bidx+1 < len(baseURLs) {
							log.Debugf("antigravity executor: rate limited on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[bidx+1])
							continue
						}
						if midx+1 < len(models) {
							log.Debugf("antigravity executor: rate limited, retrying with fallback model: %s", models[midx+1])
							continue
						}
					}
					retryAfter := antigravityRetryAfter(httpResp.StatusCode, httpResp.Header, bodyBytes)
					err = statusErr{code: httpResp.StatusCode, msg: formatErrorMessage(bodyBytes, auth), retryAfter: retryAfter}
					return resp, err
				}

				reporter.publish(ctx, parseAntigravityUsage(bodyBytes))
				var param any
				converted := sdktranslator.TranslateNonStream(ctx, to, from, attemptModel, bytes.Clone(opts.OriginalRequest), translated, bodyBytes, &param)
				resp = cliproxyexecutor.Response{Payload: []byte(converted)}
				// #region agent log
				agentdebug.Log(
					"H1",
					"internal/runtime/executor/antigravity_executor.go:Execute:nonstream_done",
					"nonstream_payload_sizes",
					map[string]any{
						"model":           attemptModel,
						"reqPayloadBytes": len(req.Payload),
						"translatedBytes": len(translated),
						"respBytes":       len(resp.Payload),
					},
				)
				// #endregion agent log
				reporter.ensurePublished(ctx)
				return resp, nil
			}
		}
	}

	switch {
	case lastStatus != 0:
		err = statusErr{code: lastStatus, msg: formatErrorMessage(lastBody, auth)}
	case lastErr != nil:
		err = lastErr
	default:
		err = statusErr{code: http.StatusServiceUnavailable, msg: "antigravity executor: no base url available"}
	}
	return resp, err
}

// executeClaudeNonStream performs a claude non-streaming request to the Antigravity API.
func (e *AntigravityExecutor) executeClaudeNonStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (resp cliproxyexecutor.Response, err error) {
	token, updatedAuth, errToken := e.ensureAccessToken(ctx, auth)
	if errToken != nil {
		return resp, errToken
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	from := opts.SourceFormat
	to := sdktranslator.FromString("antigravity")
	translated := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)
	translated = applyThinkingMetadataCLI(translated, req.Metadata, req.Model)
	translated = util.ApplyGemini3ThinkingLevelFromMetadataCLI(req.Model, req.Metadata, translated)
	translated = util.ApplyDefaultThinkingIfNeededCLI(req.Model, translated)
	translated = normalizeAntigravityThinking(req.Model, translated)
	translated = applyPayloadConfigWithRoot(e.cfg, req.Model, "antigravity", "request", translated, nil)

	switchProject := e.cfg != nil && e.cfg.QuotaExceeded.SwitchProject
	switchPreviewModel := e.cfg != nil && e.cfg.QuotaExceeded.SwitchPreviewModel

	projects := projectIDCandidatesFromAuth(auth)
	if !switchProject && len(projects) > 1 {
		projects = projects[:1]
	}
	if len(projects) == 0 {
		projects = []string{""}
	}

	models := []string{req.Model}
	if switchPreviewModel {
		models = append(models, quotaPreviewFallbackOrder(req.Model)...)
	}
	{
		seen := make(map[string]struct{}, len(models))
		uniq := make([]string, 0, len(models))
		for _, m := range models {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			uniq = append(uniq, m)
		}
		models = uniq
	}

	baseURLs := antigravityBaseURLFallbackOrder(auth)
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)

	var lastStatus int
	var lastBody []byte
	var lastErr error

	projectID := ""
	if projects := projectIDCandidatesFromAuth(auth); len(projects) > 0 {
		projectID = projects[0]
	}

	for idx, baseURL := range baseURLs {
		httpReq, errReq := e.buildRequest(ctx, auth, token, req.Model, projectID, translated, true, opts.Alt, baseURL)
		if errReq != nil {
			err = errReq
			return resp, err
		}

		httpResp, errDo := httpClient.Do(httpReq)
		if errDo != nil {
			recordAPIResponseError(ctx, e.cfg, errDo)
			lastStatus = 0
			lastBody = nil
			lastErr = errDo
			if idx+1 < len(baseURLs) {
				log.Debugf("antigravity executor: request error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
				continue
			}
			err = errDo
			return resp, err
		}
		recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())
		if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
			bodyBytes, errRead := io.ReadAll(httpResp.Body)
			if errClose := httpResp.Body.Close(); errClose != nil {
				log.Errorf("antigravity executor: close response body error: %v", errClose)
			}
			if errRead != nil {
				recordAPIResponseError(ctx, e.cfg, errRead)
				lastStatus = 0
				lastBody = nil
				lastErr = errRead
				if idx+1 < len(baseURLs) {
					log.Debugf("antigravity executor: read error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
					continue
				}
				err = errRead
				return resp, err
			}
			appendAPIResponseChunk(ctx, e.cfg, bodyBytes)
			lastStatus = httpResp.StatusCode
			lastBody = append([]byte(nil), bodyBytes...)
			lastErr = nil
			if httpResp.StatusCode == http.StatusTooManyRequests && idx+1 < len(baseURLs) {
				log.Debugf("antigravity executor: rate limited on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
				continue
			}
			retryAfter := antigravityRetryAfter(httpResp.StatusCode, httpResp.Header, bodyBytes)
			err = statusErr{code: httpResp.StatusCode, msg: string(bodyBytes), retryAfter: retryAfter}
			return resp, err
		}

		out := make(chan cliproxyexecutor.StreamChunk)
		go func(resp *http.Response) {
			defer close(out)
			defer func() {
				if errClose := resp.Body.Close(); errClose != nil {
					log.Errorf("antigravity executor: close response body error: %v", errClose)
				}
			}()
			scanner := bufio.NewScanner(resp.Body)
			scanner.Buffer(nil, streamScannerBuffer)
			for scanner.Scan() {
				line := scanner.Bytes()
				appendAPIResponseChunk(ctx, e.cfg, line)

				// Filter usage metadata for all models
				// Only retain usage statistics in the terminal chunk
				line = FilterSSEUsageMetadata(line)

				payload := jsonPayload(line)
				if payload == nil {
					continue
				}

				if detail, ok := parseAntigravityStreamUsage(payload); ok {
					reporter.publish(ctx, detail)
				}

				out <- cliproxyexecutor.StreamChunk{Payload: payload}
			}
			if errScan := scanner.Err(); errScan != nil {
				recordAPIResponseError(ctx, e.cfg, errScan)
				reporter.publishFailure(ctx)
				out <- cliproxyexecutor.StreamChunk{Err: errScan}
			} else {
				reporter.ensurePublished(ctx)
			}
		}(httpResp)

		var buffer bytes.Buffer
		for chunk := range out {
			if chunk.Err != nil {
				return resp, chunk.Err
			}
			if len(chunk.Payload) > 0 {
				_, _ = buffer.Write(chunk.Payload)
				_, _ = buffer.Write([]byte("\n"))
			}
		}
		resp = cliproxyexecutor.Response{Payload: e.convertStreamToNonStream(buffer.Bytes())}

		reporter.publish(ctx, parseAntigravityUsage(resp.Payload))
		var param any
		converted := sdktranslator.TranslateNonStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), translated, resp.Payload, &param)
		resp = cliproxyexecutor.Response{Payload: []byte(converted)}
		reporter.ensurePublished(ctx)

		return resp, nil
	}

	switch {
	case lastStatus != 0:
		err = statusErr{code: lastStatus, msg: string(lastBody)}
	case lastErr != nil:
		err = lastErr
	default:
		err = statusErr{code: http.StatusServiceUnavailable, msg: "antigravity executor: no base url available"}
	}
	return resp, err
}

func (e *AntigravityExecutor) convertStreamToNonStream(stream []byte) []byte {
	responseTemplate := ""
	var traceID string
	var finishReason string
	var modelVersion string
	var responseID string
	var role string
	var usageRaw string
	parts := make([]map[string]interface{}, 0)
	var pendingKind string
	var pendingText strings.Builder
	var pendingThoughtSig string

	flushPending := func() {
		if pendingKind == "" {
			return
		}
		text := pendingText.String()
		switch pendingKind {
		case "text":
			if strings.TrimSpace(text) == "" {
				pendingKind = ""
				pendingText.Reset()
				pendingThoughtSig = ""
				return
			}
			parts = append(parts, map[string]interface{}{"text": text})
		case "thought":
			if strings.TrimSpace(text) == "" && pendingThoughtSig == "" {
				pendingKind = ""
				pendingText.Reset()
				pendingThoughtSig = ""
				return
			}
			part := map[string]interface{}{"thought": true}
			part["text"] = text
			if pendingThoughtSig != "" {
				part["thoughtSignature"] = pendingThoughtSig
			}
			parts = append(parts, part)
		}
		pendingKind = ""
		pendingText.Reset()
		pendingThoughtSig = ""
	}

	normalizePart := func(partResult gjson.Result) map[string]interface{} {
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(partResult.Raw), &m)
		if m == nil {
			m = map[string]interface{}{}
		}
		sig := partResult.Get("thoughtSignature").String()
		if sig == "" {
			sig = partResult.Get("thought_signature").String()
		}
		if sig != "" {
			m["thoughtSignature"] = sig
			delete(m, "thought_signature")
		}
		if inlineData, ok := m["inline_data"]; ok {
			m["inlineData"] = inlineData
			delete(m, "inline_data")
		}
		return m
	}

	for _, line := range bytes.Split(stream, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || !gjson.ValidBytes(trimmed) {
			continue
		}

		root := gjson.ParseBytes(trimmed)
		responseNode := root.Get("response")
		if !responseNode.Exists() {
			if root.Get("candidates").Exists() {
				responseNode = root
			} else {
				continue
			}
		}
		responseTemplate = responseNode.Raw

		if traceResult := root.Get("traceId"); traceResult.Exists() && traceResult.String() != "" {
			traceID = traceResult.String()
		}

		if roleResult := responseNode.Get("candidates.0.content.role"); roleResult.Exists() {
			role = roleResult.String()
		}

		if finishResult := responseNode.Get("candidates.0.finishReason"); finishResult.Exists() && finishResult.String() != "" {
			finishReason = finishResult.String()
		}

		if modelResult := responseNode.Get("modelVersion"); modelResult.Exists() && modelResult.String() != "" {
			modelVersion = modelResult.String()
		}
		if responseIDResult := responseNode.Get("responseId"); responseIDResult.Exists() && responseIDResult.String() != "" {
			responseID = responseIDResult.String()
		}
		if usageResult := responseNode.Get("usageMetadata"); usageResult.Exists() {
			usageRaw = usageResult.Raw
		} else if usageResult := root.Get("usageMetadata"); usageResult.Exists() {
			usageRaw = usageResult.Raw
		}

		if partsResult := responseNode.Get("candidates.0.content.parts"); partsResult.IsArray() {
			for _, part := range partsResult.Array() {
				hasFunctionCall := part.Get("functionCall").Exists()
				hasInlineData := part.Get("inlineData").Exists() || part.Get("inline_data").Exists()
				sig := part.Get("thoughtSignature").String()
				if sig == "" {
					sig = part.Get("thought_signature").String()
				}
				text := part.Get("text").String()
				thought := part.Get("thought").Bool()

				if hasFunctionCall || hasInlineData {
					flushPending()
					parts = append(parts, normalizePart(part))
					continue
				}

				if thought || part.Get("text").Exists() {
					kind := "text"
					if thought {
						kind = "thought"
					}
					if pendingKind != "" && pendingKind != kind {
						flushPending()
					}
					pendingKind = kind
					pendingText.WriteString(text)
					if kind == "thought" && sig != "" {
						pendingThoughtSig = sig
					}
					continue
				}

				flushPending()
				parts = append(parts, normalizePart(part))
			}
		}
	}
	flushPending()

	if responseTemplate == "" {
		responseTemplate = `{"candidates":[{"content":{"role":"model","parts":[]}}]}`
	}

	partsJSON, _ := json.Marshal(parts)
	responseTemplate, _ = sjson.SetRaw(responseTemplate, "candidates.0.content.parts", string(partsJSON))
	if role != "" {
		responseTemplate, _ = sjson.Set(responseTemplate, "candidates.0.content.role", role)
	}
	if finishReason != "" {
		responseTemplate, _ = sjson.Set(responseTemplate, "candidates.0.finishReason", finishReason)
	}
	if modelVersion != "" {
		responseTemplate, _ = sjson.Set(responseTemplate, "modelVersion", modelVersion)
	}
	if responseID != "" {
		responseTemplate, _ = sjson.Set(responseTemplate, "responseId", responseID)
	}
	if usageRaw != "" {
		responseTemplate, _ = sjson.SetRaw(responseTemplate, "usageMetadata", usageRaw)
	} else if !gjson.Get(responseTemplate, "usageMetadata").Exists() {
		responseTemplate, _ = sjson.Set(responseTemplate, "usageMetadata.promptTokenCount", 0)
		responseTemplate, _ = sjson.Set(responseTemplate, "usageMetadata.candidatesTokenCount", 0)
		responseTemplate, _ = sjson.Set(responseTemplate, "usageMetadata.totalTokenCount", 0)
	}

	output := `{"response":{},"traceId":""}`
	output, _ = sjson.SetRaw(output, "response", responseTemplate)
	if traceID != "" {
		output, _ = sjson.Set(output, "traceId", traceID)
	}
	return []byte(output)
}

// ExecuteStream performs a streaming request to the Antigravity API.
func (e *AntigravityExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (stream <-chan cliproxyexecutor.StreamChunk, err error) {
	ctx = context.WithValue(ctx, "alt", "")

	token, updatedAuth, errToken := e.ensureAccessToken(ctx, auth)
	if errToken != nil {
		return nil, errToken
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	from := opts.SourceFormat
	to := sdktranslator.FromString("antigravity")
	translated := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)

	translated = applyThinkingMetadataCLI(translated, req.Metadata, req.Model)
	translated = util.ApplyGemini3ThinkingLevelFromMetadataCLI(req.Model, req.Metadata, translated)
	translated = util.ApplyDefaultThinkingIfNeededCLI(req.Model, translated)
	translated = normalizeAntigravityThinking(req.Model, translated)
	translated = applyPayloadConfigWithRoot(e.cfg, req.Model, "antigravity", "request", translated, nil)

	switchProject := e.cfg != nil && e.cfg.QuotaExceeded.SwitchProject
	switchPreviewModel := e.cfg != nil && e.cfg.QuotaExceeded.SwitchPreviewModel

	projects := projectIDCandidatesFromAuth(auth)
	if !switchProject && len(projects) > 1 {
		projects = projects[:1]
	}
	if len(projects) == 0 {
		projects = []string{""}
	}

	models := []string{req.Model}
	if switchPreviewModel {
		models = append(models, quotaPreviewFallbackOrder(req.Model)...)
	}
	{
		seen := make(map[string]struct{}, len(models))
		uniq := make([]string, 0, len(models))
		for _, m := range models {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			uniq = append(uniq, m)
		}
		models = uniq
	}

	baseURLs := antigravityBaseURLFallbackOrder(auth)
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)

	// #region agent log
	agentdebug.Log(
		"H2",
		"internal/runtime/executor/antigravity_executor.go:ExecuteStream:entry",
		"stream_request_sizes",
		map[string]any{
			"model":           req.Model,
			"reqPayloadBytes": len(req.Payload),
			"translatedBytes": len(translated),
			"baseURLCount":    len(baseURLs),
		},
	)
	// #endregion agent log

	var lastStatus int
	var lastBody []byte
	var lastErr error

	for midx, attemptModel := range models {
		attemptTranslated := sdktranslator.TranslateRequest(from, to, attemptModel, bytes.Clone(req.Payload), true)
		attemptTranslated = applyThinkingMetadataCLI(attemptTranslated, req.Metadata, attemptModel)
		attemptTranslated = util.ApplyGemini3ThinkingLevelFromMetadataCLI(attemptModel, req.Metadata, attemptTranslated)
		attemptTranslated = util.ApplyDefaultThinkingIfNeededCLI(attemptModel, attemptTranslated)
		attemptTranslated = normalizeAntigravityThinking(attemptModel, attemptTranslated)
		attemptTranslated = applyPayloadConfigWithRoot(e.cfg, attemptModel, "antigravity", "request", attemptTranslated, nil)

		for bidx, baseURL := range baseURLs {
			for pidx, projectID := range projects {
				httpReq, errReq := e.buildRequest(ctx, auth, token, attemptModel, projectID, attemptTranslated, true, opts.Alt, baseURL)
				if errReq != nil {
					err = errReq
					return nil, err
				}

				httpResp, errDo := httpClient.Do(httpReq)
				if errDo != nil {
					recordAPIResponseError(ctx, e.cfg, errDo)
					lastStatus = 0
					lastBody = nil
					lastErr = errDo
					if bidx+1 < len(baseURLs) {
						log.Debugf("antigravity executor: request error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[bidx+1])
						continue
					}
					err = errDo
					return nil, err
				}
				recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())
				if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
					bodyBytes, errRead := io.ReadAll(httpResp.Body)
					if errClose := httpResp.Body.Close(); errClose != nil {
						log.Errorf("antigravity executor: close response body error: %v", errClose)
					}
					if errRead != nil {
						recordAPIResponseError(ctx, e.cfg, errRead)
						lastStatus = 0
						lastBody = nil
						lastErr = errRead
						if bidx+1 < len(baseURLs) {
							log.Debugf("antigravity executor: read error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[bidx+1])
							continue
						}
						err = errRead
						return nil, err
					}
					appendAPIResponseChunk(ctx, e.cfg, bodyBytes)
					lastStatus = httpResp.StatusCode
					lastBody = append([]byte(nil), bodyBytes...)
					lastErr = nil
					if httpResp.StatusCode == http.StatusTooManyRequests {
						if pidx+1 < len(projects) {
							log.Debugf("antigravity executor: rate limited on project %s, retrying with next project: %s", projectID, projects[pidx+1])
							continue
						}
						if bidx+1 < len(baseURLs) {
							log.Debugf("antigravity executor: rate limited on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[bidx+1])
							continue
						}
						if midx+1 < len(models) {
							log.Debugf("antigravity executor: rate limited, retrying with fallback model: %s", models[midx+1])
							continue
						}
					}
					retryAfter := antigravityRetryAfter(httpResp.StatusCode, httpResp.Header, bodyBytes)
					err = statusErr{code: httpResp.StatusCode, msg: formatErrorMessage(bodyBytes, auth), retryAfter: retryAfter}
					return nil, err
				}

				out := make(chan cliproxyexecutor.StreamChunk)
				stream = out
				go func(resp *http.Response, model string, translated []byte) {
					defer close(out)
					defer func() {
						if errClose := resp.Body.Close(); errClose != nil {
							log.Errorf("antigravity executor: close response body error: %v", errClose)
						}
					}()
					scanner := bufio.NewScanner(resp.Body)
					scanner.Buffer(nil, streamScannerBuffer)
					var param any
					scanLines := 0
					outChunks := 0
					outBytes := 0
					maxChunksPerLine := 0
					maxPayloadBytes := 0
					for scanner.Scan() {
						line := scanner.Bytes()
						appendAPIResponseChunk(ctx, e.cfg, line)
						scanLines++

						// Filter usage metadata for all models
						// Only retain usage statistics in the terminal chunk
						line = FilterSSEUsageMetadata(line)

						payload := jsonPayload(line)
						if payload == nil {
							continue
						}
						if n := len(payload); n > maxPayloadBytes {
							maxPayloadBytes = n
						}

						if detail, ok := parseAntigravityStreamUsage(payload); ok {
							reporter.publish(ctx, detail)
						}

						chunks := sdktranslator.TranslateStream(ctx, to, from, model, bytes.Clone(opts.OriginalRequest), translated, bytes.Clone(payload), &param)
						if len(chunks) > maxChunksPerLine {
							maxChunksPerLine = len(chunks)
						}
						for i := range chunks {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunks[i])}
							outChunks++
							outBytes += len(chunks[i])
						}
					}
					tail := sdktranslator.TranslateStream(ctx, to, from, model, bytes.Clone(opts.OriginalRequest), translated, []byte("[DONE]"), &param)
					for i := range tail {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(tail[i])}
						outChunks++
						outBytes += len(tail[i])
					}
					if errScan := scanner.Err(); errScan != nil {
						recordAPIResponseError(ctx, e.cfg, errScan)
						reporter.publishFailure(ctx)
						// #region agent log
						agentdebug.Log(
							"H5",
							"internal/runtime/executor/antigravity_executor.go:ExecuteStream:scanner_err",
							"stream_scanner_error",
							map[string]any{
								"model":     model,
								"scanLines": scanLines,
								"error":     errScan.Error(),
							},
						)
						// #endregion agent log
						out <- cliproxyexecutor.StreamChunk{Err: errScan}
					} else {
						reporter.ensurePublished(ctx)
					}
					// #region agent log
					agentdebug.Log(
						"H2",
						"internal/runtime/executor/antigravity_executor.go:ExecuteStream:done",
						"stream_totals",
						map[string]any{
							"model":            model,
							"scanLines":        scanLines,
							"outChunks":        outChunks,
							"outBytes":         outBytes,
							"maxChunksPerLine": maxChunksPerLine,
							"maxPayloadBytes":  maxPayloadBytes,
						},
					)
					// #endregion agent log
				}(httpResp, attemptModel, attemptTranslated)
				return stream, nil
			}
		}
	}

	switch {
	case lastStatus != 0:
		err = statusErr{code: lastStatus, msg: formatErrorMessage(lastBody, auth)}
	case lastErr != nil:
		err = lastErr
	default:
		err = statusErr{code: http.StatusServiceUnavailable, msg: "antigravity executor: no base url available"}
	}
	return nil, err
}

// Refresh refreshes the authentication credentials using the refresh token.
func (e *AntigravityExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return auth, nil
	}
	updated, errRefresh := e.refreshToken(ctx, auth.Clone())
	if errRefresh != nil {
		return nil, errRefresh
	}
	return updated, nil
}

// CountTokens counts tokens for the given request using the Antigravity API.
func (e *AntigravityExecutor) CountTokens(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	token, updatedAuth, errToken := e.ensureAccessToken(ctx, auth)
	if errToken != nil {
		return cliproxyexecutor.Response{}, errToken
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}
	if strings.TrimSpace(token) == "" {
		return cliproxyexecutor.Response{}, statusErr{code: http.StatusUnauthorized, msg: "missing access token"}
	}

	from := opts.SourceFormat
	to := sdktranslator.FromString("antigravity")
	respCtx := context.WithValue(ctx, "alt", opts.Alt)

	baseURLs := antigravityBaseURLFallbackOrder(auth)
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)

	var authID, authLabel, authType, authValue string
	if auth != nil {
		authID = auth.ID
		authLabel = auth.Label
		authType, authValue = auth.AccountInfo()
	}

	var lastStatus int
	var lastBody []byte
	var lastErr error

	for idx, baseURL := range baseURLs {
		payload := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), false)
		payload = applyThinkingMetadataCLI(payload, req.Metadata, req.Model)
		payload = util.ApplyDefaultThinkingIfNeededCLI(req.Model, payload)
		payload = normalizeAntigravityThinking(req.Model, payload)
		payload = deleteJSONField(payload, "project")
		payload = deleteJSONField(payload, "model")
		payload = deleteJSONField(payload, "request.safetySettings")

		base := strings.TrimSuffix(baseURL, "/")
		if base == "" {
			base = buildBaseURL(auth)
		}

		var requestURL strings.Builder
		requestURL.WriteString(base)
		requestURL.WriteString(antigravityCountTokensPath)
		if opts.Alt != "" {
			requestURL.WriteString("?$alt=")
			requestURL.WriteString(url.QueryEscape(opts.Alt))
		}

		httpReq, errReq := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(payload))
		if errReq != nil {
			return cliproxyexecutor.Response{}, errReq
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token)
		httpReq.Header.Set("User-Agent", resolveUserAgent(auth))
		httpReq.Header.Set("X-Goog-Api-Client", antigravityXGoogAPIClient)
		httpReq.Header.Set("Client-Metadata", antigravityClientMetadata)
		httpReq.Header.Set("Accept", "application/json")
		if host := resolveHost(base); host != "" {
			httpReq.Host = host
		}

		recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
			URL:       requestURL.String(),
			Method:    http.MethodPost,
			Headers:   httpReq.Header.Clone(),
			Body:      payload,
			Provider:  e.Identifier(),
			AuthID:    authID,
			AuthLabel: authLabel,
			AuthType:  authType,
			AuthValue: authValue,
		})

		httpResp, errDo := httpClient.Do(httpReq)
		if errDo != nil {
			recordAPIResponseError(ctx, e.cfg, errDo)
			lastStatus = 0
			lastBody = nil
			lastErr = errDo
			if idx+1 < len(baseURLs) {
				log.Debugf("antigravity executor: request error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
				continue
			}
			return cliproxyexecutor.Response{}, errDo
		}

		recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())
		bodyBytes, errRead := io.ReadAll(httpResp.Body)
		if errClose := httpResp.Body.Close(); errClose != nil {
			log.Errorf("antigravity executor: close response body error: %v", errClose)
		}
		if errRead != nil {
			recordAPIResponseError(ctx, e.cfg, errRead)
			return cliproxyexecutor.Response{}, errRead
		}
		appendAPIResponseChunk(ctx, e.cfg, bodyBytes)

		if httpResp.StatusCode >= http.StatusOK && httpResp.StatusCode < http.StatusMultipleChoices {
			count := gjson.GetBytes(bodyBytes, "totalTokens").Int()
			translated := sdktranslator.TranslateTokenCount(respCtx, to, from, count, bodyBytes)
			return cliproxyexecutor.Response{Payload: []byte(translated)}, nil
		}

		lastStatus = httpResp.StatusCode
		lastBody = append([]byte(nil), bodyBytes...)
		lastErr = nil
		if httpResp.StatusCode == http.StatusTooManyRequests && idx+1 < len(baseURLs) {
			log.Debugf("antigravity executor: rate limited on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
			continue
		}
		retryAfter := antigravityRetryAfter(httpResp.StatusCode, httpResp.Header, bodyBytes)
		return cliproxyexecutor.Response{}, statusErr{code: httpResp.StatusCode, msg: string(bodyBytes), retryAfter: retryAfter}
	}

	switch {
	case lastStatus != 0:
		return cliproxyexecutor.Response{}, statusErr{code: lastStatus, msg: string(lastBody)}
	case lastErr != nil:
		return cliproxyexecutor.Response{}, lastErr
	default:
		return cliproxyexecutor.Response{}, statusErr{code: http.StatusServiceUnavailable, msg: "antigravity executor: no base url available"}
	}
}

// Embed performs an embedding request (not supported for Antigravity).
func (e *AntigravityExecutor) Embed(context.Context, *cliproxyauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "embeddings not supported"}
}

// FetchAntigravityModels retrieves available models using the supplied auth.
func FetchAntigravityModels(ctx context.Context, auth *cliproxyauth.Auth, cfg *config.Config) []*registry.ModelInfo {
	exec := &AntigravityExecutor{cfg: cfg}
	token, updatedAuth, errToken := exec.ensureAccessToken(ctx, auth)
	if errToken != nil || token == "" {
		return nil
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}

	baseURLs := antigravityBaseURLFallbackOrder(auth)
	httpClient := newProxyAwareHTTPClient(ctx, cfg, auth, 0)

	for idx, baseURL := range baseURLs {
		modelsURL := baseURL + antigravityModelsPath
		httpReq, errReq := http.NewRequestWithContext(ctx, http.MethodPost, modelsURL, bytes.NewReader([]byte(`{}`)))
		if errReq != nil {
			return nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token)
		httpReq.Header.Set("User-Agent", resolveUserAgent(auth))
		httpReq.Header.Set("X-Goog-Api-Client", antigravityXGoogAPIClient)
		httpReq.Header.Set("Client-Metadata", antigravityClientMetadata)
		if host := resolveHost(baseURL); host != "" {
			httpReq.Host = host
		}

		httpResp, errDo := httpClient.Do(httpReq)
		if errDo != nil {
			if idx+1 < len(baseURLs) {
				log.Debugf("antigravity executor: models request error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
				continue
			}
			return nil
		}

		bodyBytes, errRead := io.ReadAll(httpResp.Body)
		if errClose := httpResp.Body.Close(); errClose != nil {
			log.Errorf("antigravity executor: close response body error: %v", errClose)
		}
		if errRead != nil {
			if idx+1 < len(baseURLs) {
				log.Debugf("antigravity executor: models read error on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
				continue
			}
			return nil
		}
		if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
			if httpResp.StatusCode == http.StatusTooManyRequests && idx+1 < len(baseURLs) {
				log.Debugf("antigravity executor: models request rate limited on base url %s, retrying with fallback base url: %s", baseURL, baseURLs[idx+1])
				continue
			}
			return nil
		}

		result := gjson.GetBytes(bodyBytes, "models")
		if !result.Exists() {
			return nil
		}

		now := time.Now().Unix()
		modelConfig := registry.GetAntigravityModelConfig()
		limits := antigravityBuildStaticLimitsIndex()
		models := make([]*registry.ModelInfo, 0, len(result.Map()))
		appendModel := func(id string) {
			id = strings.TrimSpace(id)
			if id == "" {
				return
			}
			// Skip certain models that shouldn't be listed
			switch id {
			case "chat_20706", "chat_23310", "gemini-2.5-flash-thinking", "gemini-3-pro-low", "gemini-2.5-pro":
				return
			}
			cfg := modelConfig[id]
			modelName := id
			modelInfo := &registry.ModelInfo{
				ID:          id,
				Name:        modelName,
				Description: id,
				DisplayName: id,
				Version:     id,
				Object:      "model",
				Created:     now,
				OwnedBy:     antigravityAuthType,
				Type:        antigravityAuthType,
			}
			// Copy known limits from our static Gemini model list (best-effort).
			// This improves /v1/models output (context_length/max_completion_tokens) for antigravity models.
			if lim, ok := limits[id]; ok {
				if modelInfo.InputTokenLimit == 0 && lim.InputTokenLimit > 0 {
					modelInfo.InputTokenLimit = lim.InputTokenLimit
				}
				if modelInfo.OutputTokenLimit == 0 && lim.OutputTokenLimit > 0 {
					modelInfo.OutputTokenLimit = lim.OutputTokenLimit
				}
				if modelInfo.ContextLength == 0 && lim.ContextLength > 0 {
					modelInfo.ContextLength = lim.ContextLength
				}
				if modelInfo.MaxCompletionTokens == 0 && lim.MaxCompletionTokens > 0 {
					modelInfo.MaxCompletionTokens = lim.MaxCompletionTokens
				}
			}
			// Look up Thinking support from static config using ID
			if cfg != nil {
				if cfg.Thinking != nil {
					modelInfo.Thinking = cfg.Thinking
				}
				if cfg.MaxCompletionTokens > 0 {
					modelInfo.MaxCompletionTokens = cfg.MaxCompletionTokens
				}
			} else {
				// Try lookup with aliased name for limits (e.g. gemini-3-pro-high -> gemini-3-pro-preview)
				alias := alias2ModelName(id)
				if lim, ok := limits[alias]; ok {
					if modelInfo.InputTokenLimit == 0 && lim.InputTokenLimit > 0 {
						modelInfo.InputTokenLimit = lim.InputTokenLimit
					}
					if modelInfo.OutputTokenLimit == 0 && lim.OutputTokenLimit > 0 {
						modelInfo.OutputTokenLimit = lim.OutputTokenLimit
					}
					if modelInfo.ContextLength == 0 && lim.ContextLength > 0 {
						modelInfo.ContextLength = lim.ContextLength
					}
					if modelInfo.MaxCompletionTokens == 0 && lim.MaxCompletionTokens > 0 {
						modelInfo.MaxCompletionTokens = lim.MaxCompletionTokens
					}
				}
			}
			models = append(models, modelInfo)
		}
		for id := range result.Map() {
			appendModel(id)
		}

		// Also append static models from config (e.g. antigravity-claude-*)
		for id := range modelConfig {
			if !strings.HasPrefix(id, "antigravity-") {
				continue
			}
			appendModel(id)
		}

		return models
	}
	return nil
}

type antigravityStaticLimit struct {
	InputTokenLimit     int
	OutputTokenLimit    int
	ContextLength       int
	MaxCompletionTokens int
}

func antigravityBuildStaticLimitsIndex() map[string]antigravityStaticLimit {
	index := make(map[string]antigravityStaticLimit)
	add := func(models []*registry.ModelInfo) {
		for _, m := range models {
			if m == nil || strings.TrimSpace(m.ID) == "" {
				continue
			}
			id := strings.TrimSpace(m.ID)
			cur := index[id]
			if cur.InputTokenLimit == 0 && m.InputTokenLimit > 0 {
				cur.InputTokenLimit = m.InputTokenLimit
			}
			if cur.OutputTokenLimit == 0 && m.OutputTokenLimit > 0 {
				cur.OutputTokenLimit = m.OutputTokenLimit
			}
			if cur.ContextLength == 0 && m.ContextLength > 0 {
				cur.ContextLength = m.ContextLength
			}
			if cur.MaxCompletionTokens == 0 && m.MaxCompletionTokens > 0 {
				cur.MaxCompletionTokens = m.MaxCompletionTokens
			}
			// Fallback: if only Gemini-style limits exist, treat them as context/output.
			if cur.ContextLength == 0 && cur.InputTokenLimit > 0 {
				cur.ContextLength = cur.InputTokenLimit
			}
			if cur.MaxCompletionTokens == 0 && cur.OutputTokenLimit > 0 {
				cur.MaxCompletionTokens = cur.OutputTokenLimit
			}
			index[id] = cur
		}
	}

	add(registry.GetGeminiModels())
	add(registry.GetGeminiVertexModels())
	add(registry.GetGeminiCLIModels())
	add(registry.GetAIStudioModels())
	add(registry.GetClaudeModels())
	add(registry.GetQwenModels())

	// Add explicit limits for antigravity-claude models.
	// These models route to Claude via Antigravity but use aliases that don't match GetClaudeModels() IDs.
	// Claude 3.5 Sonnet and Claude 3 Opus both have 200k context but we use a more conservative
	// limit to account for prompt overhead and ensure successful API calls.
	claudeAntigravityLimit := antigravityStaticLimit{
		ContextLength:       180000, // Conservative (200k actual but leave room for overhead)
		MaxCompletionTokens: 64000,
		InputTokenLimit:     180000,
		OutputTokenLimit:    64000,
	}
	index["antigravity-claude-sonnet-4-5"] = claudeAntigravityLimit
	index["antigravity-claude-sonnet-4-5-thinking"] = claudeAntigravityLimit
	index["antigravity-claude-opus-4-5-thinking"] = claudeAntigravityLimit
	// Also add the underlying claude model IDs that alias2ModelName() converts to
	index["claude-3-5-sonnet"] = claudeAntigravityLimit
	index["claude-3-opus"] = claudeAntigravityLimit
	// Add the user-facing model names (without prefix) that get passed to truncateConversation
	index["claude-opus-4-5-thinking"] = claudeAntigravityLimit
	index["claude-sonnet-4-5-thinking"] = claudeAntigravityLimit
	index["claude-sonnet-4-5"] = claudeAntigravityLimit

	return index
}

func (e *AntigravityExecutor) ensureAccessToken(ctx context.Context, auth *cliproxyauth.Auth) (string, *cliproxyauth.Auth, error) {
	if auth == nil {
		return "", nil, statusErr{code: http.StatusUnauthorized, msg: "missing auth"}
	}
	accessToken := metaStringValue(auth.Metadata, "access_token")
	expiry := tokenExpiry(auth.Metadata)
	if accessToken != "" && expiry.After(time.Now().Add(refreshSkew)) {
		return accessToken, nil, nil
	}
	updated, errRefresh := e.refreshToken(ctx, auth.Clone())
	if errRefresh != nil {
		return "", nil, errRefresh
	}
	return metaStringValue(updated.Metadata, "access_token"), updated, nil
}

func (e *AntigravityExecutor) refreshToken(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing auth"}
	}
	refreshToken := metaStringValue(auth.Metadata, "refresh_token")
	if refreshToken == "" {
		return auth, statusErr{code: http.StatusUnauthorized, msg: "missing refresh token"}
	}

	form := url.Values{}
	form.Set("client_id", antigravityClientID)
	form.Set("client_secret", antigravityClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	httpReq, errReq := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if errReq != nil {
		return auth, errReq
	}
	httpReq.Header.Set("Host", "oauth2.googleapis.com")
	httpReq.Header.Set("User-Agent", defaultAntigravityAgent)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	httpResp, errDo := httpClient.Do(httpReq)
	if errDo != nil {
		return auth, errDo
	}
	defer func() {
		if errClose := httpResp.Body.Close(); errClose != nil {
			log.Errorf("antigravity executor: close response body error: %v", errClose)
		}
	}()

	bodyBytes, errRead := io.ReadAll(httpResp.Body)
	if errRead != nil {
		return auth, errRead
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return auth, statusErr{code: httpResp.StatusCode, msg: string(bodyBytes)}
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if errUnmarshal := json.Unmarshal(bodyBytes, &tokenResp); errUnmarshal != nil {
		return auth, errUnmarshal
	}

	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["access_token"] = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		auth.Metadata["refresh_token"] = tokenResp.RefreshToken
	}
	auth.Metadata["expires_in"] = tokenResp.ExpiresIn
	now := time.Now()
	auth.Metadata["timestamp"] = now.UnixMilli()
	auth.Metadata["expired"] = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	auth.Metadata["type"] = antigravityAuthType

	// Ensure project_id is set for antigravity auth
	if errProject := e.ensureAntigravityProjectID(ctx, auth, tokenResp.AccessToken); errProject != nil {
		log.Warnf("antigravity executor: ensure project id failed: %v", errProject)
	}

	return auth, nil
}

// ensureAntigravityProjectID fetches and sets the project_id in auth metadata if missing.
func (e *AntigravityExecutor) ensureAntigravityProjectID(ctx context.Context, auth *cliproxyauth.Auth, accessToken string) error {
	if auth == nil {
		return nil
	}
	if auth.Metadata != nil && auth.Metadata["project_id"] != nil {
		return nil
	}
	token := strings.TrimSpace(accessToken)
	if token == "" {
		token = metaStringValue(auth.Metadata, "access_token")
	}
	if token == "" {
		return nil
	}
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	projectID, errFetch := sdkAuth.FetchAntigravityProjectID(ctx, token, httpClient)
	if errFetch != nil {
		return errFetch
	}
	if strings.TrimSpace(projectID) == "" {
		return nil
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["project_id"] = strings.TrimSpace(projectID)
	return nil
}

func (e *AntigravityExecutor) buildRequest(ctx context.Context, auth *cliproxyauth.Auth, token, modelName, projectID string, payload []byte, stream bool, alt, baseURL string) (*http.Request, error) {
	if token == "" {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing access token"}
	}

	base := strings.TrimSuffix(baseURL, "/")
	if base == "" {
		base = buildBaseURL(auth)
	}
	path := antigravityGeneratePath
	if stream {
		path = antigravityStreamPath
	}
	var requestURL strings.Builder
	requestURL.WriteString(base)
	requestURL.WriteString(path)
	if stream {
		if alt != "" {
			requestURL.WriteString("?$alt=")
			requestURL.WriteString(url.QueryEscape(alt))
		} else {
			requestURL.WriteString("?alt=sse")
		}
	} else if alt != "" {
		requestURL.WriteString("?$alt=")
		requestURL.WriteString(url.QueryEscape(alt))
	}

	projectID = strings.TrimSpace(projectID)
	payload = geminiToAntigravity(modelName, payload, projectID, stream)
	payload, _ = sjson.SetBytes(payload, "model", alias2ModelName(modelName))

	if strings.Contains(modelName, "claude") {
		strJSON := string(payload)
		paths := make([]string, 0)
		util.Walk(gjson.ParseBytes(payload), "", "parametersJsonSchema", &paths)
		for _, p := range paths {
			strJSON, _ = util.RenameKey(strJSON, p, p[:len(p)-len("parametersJsonSchema")]+"parameters")
		}

		// Use the centralized schema cleaner to handle unsupported keywords,
		// const->enum conversion, and flattening of types/anyOf.
		strJSON = util.CleanJSONSchemaForAntigravity(strJSON)

		// Normalize nullable types like ["string", "null"] -> "string"
		// Vertex AI doesn't accept array-style nullable type definitions
		strJSON = util.NormalizeNullableTypes(strJSON)

		payload = []byte(strJSON)
	}

	if strings.Contains(modelName, "claude") || strings.Contains(modelName, "gemini-3-pro-high") {
		systemInstructionPartsResult := gjson.GetBytes(payload, "request.systemInstruction.parts")
		payload, _ = sjson.SetBytes(payload, "request.systemInstruction.role", "user")
		payload, _ = sjson.SetBytes(payload, "request.systemInstruction.parts.0.text", systemInstruction)
		payload, _ = sjson.SetBytes(payload, "request.systemInstruction.parts.1.text", fmt.Sprintf("Please ignore following [ignore]%s[/ignore]", systemInstruction))

		if systemInstructionPartsResult.Exists() && systemInstructionPartsResult.IsArray() {
			for _, partResult := range systemInstructionPartsResult.Array() {
				payload, _ = sjson.SetRawBytes(payload, "request.systemInstruction.parts.-1", []byte(partResult.Raw))
			}
		}
	}

	httpReq, errReq := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(payload))
	if errReq != nil {
		return nil, errReq
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", resolveUserAgent(auth))
	httpReq.Header.Set("X-Goog-Api-Client", antigravityXGoogAPIClient)
	httpReq.Header.Set("Client-Metadata", antigravityClientMetadata)
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}
	if host := resolveHost(base); host != "" {
		httpReq.Host = host
	}

	var authID, authLabel, authType, authValue string
	if auth != nil {
		authID = auth.ID
		authLabel = auth.Label
		authType, authValue = auth.AccountInfo()
	}
	recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
		URL:       requestURL.String(),
		Method:    http.MethodPost,
		Headers:   httpReq.Header.Clone(),
		Body:      payload,
		Provider:  e.Identifier(),
		AuthID:    authID,
		AuthLabel: authLabel,
		AuthType:  authType,
		AuthValue: authValue,
	})

	return httpReq, nil
}

func tokenExpiry(metadata map[string]any) time.Time {
	if metadata == nil {
		return time.Time{}
	}
	if expStr, ok := metadata["expired"].(string); ok {
		expStr = strings.TrimSpace(expStr)
		if expStr != "" {
			if parsed, errParse := time.Parse(time.RFC3339, expStr); errParse == nil {
				return parsed
			}
		}
	}
	expiresIn, hasExpires := int64Value(metadata["expires_in"])
	tsMs, hasTimestamp := int64Value(metadata["timestamp"])
	if hasExpires && hasTimestamp {
		return time.Unix(0, tsMs*int64(time.Millisecond)).Add(time.Duration(expiresIn) * time.Second)
	}
	return time.Time{}
}

// truncateConversation estimates token count and truncates history if it exceeds context limit.
// It also persists dropped messages to memory if a session ID is available.
func truncateConversation(payload []byte, modelID string, metadata map[string]any) []byte {
	// 1. Get Context Length from static limits index (same as FetchAntigravityModels uses)
	limit := 0
	limits := antigravityBuildStaticLimitsIndex()

	if lim, ok := limits[modelID]; ok {
		if lim.ContextLength > 0 {
			limit = lim.ContextLength
		} else if lim.InputTokenLimit > 0 {
			limit = lim.InputTokenLimit
		}
	}

	if limit <= 0 {
		// Try alias
		alias := alias2ModelName(modelID)
		if lim, ok := limits[alias]; ok {
			if lim.ContextLength > 0 {
				limit = lim.ContextLength
			} else if lim.InputTokenLimit > 0 {
				limit = lim.InputTokenLimit
			}
		}
	}

	// Default fallback if unknown (safe conservative)
	if limit == 0 {
		// If we still don't know the limit, we default to 128k tokens.
		// This is a safe upper bound for most modern models (Claude 3.5, Gemini 1.5, GPT-4o).
		// If the model is smaller (e.g. 8k, 32k), we might still overshoot, but this prevents
		// sending multi-megabyte infinite prompts that are guaranteed to fail.
		limit = 128000
		log.Warnf("Unknown context limit for model %s; defaulting to %d tokens", modelID, limit)
	}

	// Apply a 0.7 safety factor to the limit to account for system overhead,
	// tool definitions, and potential token counting differences.
	// We use 0.7 (instead of 0.9) to be more aggressive with truncation.
	limit = int(float64(limit) * 0.7)

	// 2. Estimate Tokens (Rough char count)
	// OpenAI generic rule of thumb: 1 token ~= 4 chars.
	// Gemini/Antigravity can be different, but strict safety margin is better.
	// Claude models tend to tokenize more aggressively, so use 3.0 chars/token for them.
	fullJSON := string(payload)
	charsPerToken := 3.5
	if strings.Contains(strings.ToLower(modelID), "claude") {
		charsPerToken = 3.0 // More conservative for Claude
	}
	estimatedTokens := int(float64(len(fullJSON)) / charsPerToken)

	log.Infof("truncateConversation: model=%s, limit=%d, payloadLen=%d, estimatedTokens=%d", modelID, limit, len(fullJSON), estimatedTokens)

	if estimatedTokens <= limit {
		return payload
	}

	// 3. Truncate
	// We need to parse "contents" and remove old messages.
	// Typically, we keep system instruction (which is separate in Gemini)
	// and the last few messages.
	// Antigravity payload structure: { "request": { "contents": [ { "role": "...", "parts": [...] } ] } }

	log.Infof("Truncating conversation for model %s: Est. %d tokens > Limit %d", modelID, estimatedTokens, limit)

	contents := gjson.GetBytes(payload, "request.contents")
	log.Infof("truncateConversation: request.contents exists=%v, isArray=%v", contents.Exists(), contents.IsArray())
	if !contents.IsArray() {
		// Fallback: check root level "contents" for other formats
		contents = gjson.GetBytes(payload, "contents")
		log.Infof("truncateConversation: fallback contents exists=%v, isArray=%v", contents.Exists(), contents.IsArray())
		if !contents.IsArray() {
			log.Warnf("truncateConversation: no contents array found, cannot truncate")
			return payload
		}
	}

	arr := contents.Array()
	if len(arr) <= 2 {
		// Too few messages to truncate meaningful history without breaking context
		return payload
	}

	// Strategy: Keep last N messages that fit.
	// We iterate backwards, summing estimated tokens until we hit ~Limit * 0.9 (safety buffer).

	keepIdx := 0
	currentChars := 0
	// Base overhead
	currentChars += len(fullJSON) - len(contents.Raw)

	// Always keep the last message (User prompt)
	lastMsg := arr[len(arr)-1]
	currentChars += len(lastMsg.Raw)

	// Iterate backwards from second to last
	for i := len(arr) - 2; i >= 0; i-- {
		msg := arr[i]
		msgLen := len(msg.Raw)

		// If adding this message exceeds limit, stop here
		// (We use a slightly tighter limit for safety)
		newTotal := currentChars + msgLen
		newEstTokens := int(float64(newTotal) / 3.5)

		if newEstTokens > limit {
			// Stop, this message and all before it (up to 0) must go.
			// But wait, if this is a tool_response, we MUST keep its corresponding tool_call.
			// And if it's a tool_call, we generally want to keep it if we keep the response.
			// For simplicity in this rough pass: Just cut.
			// Improving: Ensure we don't split tool pairs?
			// Gemini is strict about function_call <-> function_response ordering.
			// If we cut a function_call, we must cut its response (which is later, so we would have processed it).
			// If we cut a function_response, we must cut the call (which is earlier).

			// Actually, if we are iterating backwards:
			// "User" -> "Model" -> "Tool" -> "Model"
			// If we decide to keep "Tool", we must keep "Model" (call).

			// Simple heuristics:
			// If we drop a message at index i, we assume everything < i is also dropped.
			// We just need to find the split point.
			keepIdx = i + 1
			break
		}
		currentChars += msgLen
	}

	// Perform the cut and persist dropped memory
	if keepIdx > 0 {
		// Collect dropped messages for memory
		droppedEvents := make([]memory.Event, 0, keepIdx)
		for i := 0; i < keepIdx; i++ {
			msg := arr[i]
			role := msg.Get("role").String()
			// Extract text content (skip thinking parts to avoid leaking internal reasoning)
			var textBuilder strings.Builder
			parts := msg.Get("parts")
			if parts.IsArray() {
				for _, p := range parts.Array() {
					// Skip thinking/thought parts - these are internal reasoning
					// and should not be captured into memory/summary
					if p.Get("thought").Bool() {
						continue
					}
					if t := p.Get("text").String(); t != "" {
						textBuilder.WriteString(t)
						textBuilder.WriteString("\n")
					}
				}
			}
			text := strings.TrimSpace(textBuilder.String())
			if text != "" {
				droppedEvents = append(droppedEvents, memory.Event{
					TS:   time.Now(),
					Kind: "message",
					Role: role,
					Type: "text", // simplify for now
					Text: text,
				})
			}
		}

		if len(droppedEvents) > 0 {
			sessionID := extractSessionID(metadata)
			if sessionID != "" {
				if store := getMemoryStore(); store != nil {
					log.Debugf("Persisting %d dropped messages to memory for session %s", len(droppedEvents), sessionID)
					if err := store.Append(sessionID, droppedEvents); err != nil {
						log.Warnf("Failed to append dropped messages to memory: %v", err)
					}

					// Build anchored summary (Factory-style structured summarization)
					if fs, ok := store.(*memory.FileStore); ok {
						latestIntent := extractLatestUserIntent(droppedEvents)
						if err := fs.UpsertAnchoredSummary(sessionID, droppedEvents, "", latestIntent); err != nil {
							log.Warnf("Failed to upsert anchored summary: %v", err)
						} else {
							log.Infof("Updated anchored summary for session %s with %d dropped messages", sessionID, len(droppedEvents))
						}
					}
				}
			}
		}

		// Construct new contents array
		// We have to be careful with sjson/gjson on large arrays.
		// Rebuilding the array is safer.
		var newContents []string
		for i := keepIdx; i < len(arr); i++ {
			newContents = append(newContents, arr[i].Raw)
		}
		// Join them
		newJsonStr := "[" + strings.Join(newContents, ",") + "]"

		// Set the truncated contents back - try request.contents first (Antigravity format)
		updated, err := sjson.SetRawBytes(payload, "request.contents", []byte(newJsonStr))
		if err != nil {
			// Fallback to root contents
			updated, err = sjson.SetRawBytes(payload, "contents", []byte(newJsonStr))
			if err != nil {
				log.Errorf("Failed to truncate payload: %v", err)
				return payload
			}
		}
		return updated
	}

	return payload
}

func getMemoryStore() memory.Store {
	// Replicates logic from codex_prompt_budget.go to locate memory dir
	base := strings.TrimSpace(os.Getenv("CLIPROXY_MEMORY_DIR"))
	if base == "" {
		if w := util.WritablePath(); w != "" {
			base = filepath.Join(w, ".proxypilot", "memory")
		} else {
			base = filepath.Join(".proxypilot", "memory")
		}
	}
	return memory.NewFileStore(base)
}

// injectAnchoredSummary prepends the anchored summary to the system instruction.
// This allows the agent to recall context from truncated conversation history.
func injectAnchoredSummary(payload []byte, modelID string, metadata map[string]any) []byte {
	sessionID := extractSessionID(metadata)
	if sessionID == "" {
		return payload
	}

	store := getMemoryStore()
	if store == nil {
		return payload
	}

	fs, ok := store.(*memory.FileStore)
	if !ok {
		return payload
	}

	// Read the anchored summary (max 8k chars to avoid bloating)
	summary := fs.ReadSummary(sessionID, 8000)
	if strings.TrimSpace(summary) == "" {
		return payload
	}

	log.Debugf("Injecting anchored summary (%d chars) for session %s", len(summary), sessionID)

	// Get current system instruction
	sysInstr := gjson.GetBytes(payload, "request.systemInstruction.parts.0.text").String()

	// Check if summary is already present (avoid duplicates)
	if strings.Contains(sysInstr, "## Session Context (from previous turns)") {
		return payload
	}

	// Prepend the summary with a clear header
	contextPrefix := "## Session Context (from previous turns)\n\n" +
		"The following summarizes earlier conversation that was truncated due to context limits:\n\n" +
		summary + "\n\n---\n\n"

	newInstr := contextPrefix + sysInstr

	// Update the system instruction
	updated, err := sjson.SetBytes(payload, "request.systemInstruction.parts.0.text", newInstr)
	if err != nil {
		log.Warnf("Failed to inject anchored summary: %v", err)
		return payload
	}

	return updated
}

func extractSessionID(metadata map[string]any) string {
	if metadata == nil {
		log.Debugf("extractSessionID: metadata is nil")
		return ""
	}
	// Try standard headers often mapped to metadata
	if v, ok := metadata["X-CLIProxyAPI-Session"].(string); ok && v != "" {
		log.Debugf("extractSessionID: found X-CLIProxyAPI-Session=%s", v)
		return v
	}
	if v, ok := metadata["X-Session-Id"].(string); ok && v != "" {
		log.Debugf("extractSessionID: found X-Session-Id=%s", v)
		return v
	}
	if v, ok := metadata["session_id"].(string); ok && v != "" {
		log.Debugf("extractSessionID: found session_id=%s", v)
		return v
	}
	// Log what keys ARE present for debugging
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	log.Debugf("extractSessionID: no session found in metadata, keys present: %v", keys)
	return ""
}

// extractLatestUserIntent finds the last user message from dropped events to use as intent.
func extractLatestUserIntent(events []memory.Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		if strings.EqualFold(events[i].Role, "user") && events[i].Text != "" {
			intent := events[i].Text
			// Limit length to avoid huge intents
			if len(intent) > 1500 {
				intent = intent[:1500] + "...[truncated]..."
			}
			return intent
		}
	}
	return ""
}

func metaStringValue(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if v, ok := metadata[key]; ok {
		switch typed := v.(type) {
		case string:
			return strings.TrimSpace(typed)
		case []byte:
			return strings.TrimSpace(string(typed))
		}
	}
	return ""
}

func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		if i, errParse := typed.Int64(); errParse == nil {
			return i, true
		}
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, false
		}
		if i, errParse := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); errParse == nil {
			return i, true
		}
	}
	return 0, false
}

func buildBaseURL(auth *cliproxyauth.Auth) string {
	if baseURLs := antigravityBaseURLFallbackOrder(auth); len(baseURLs) > 0 {
		return baseURLs[0]
	}
	return antigravityBaseURLDaily
}

func resolveHost(base string) string {
	parsed, errParse := url.Parse(base)
	if errParse != nil {
		return ""
	}
	if parsed.Host != "" {
		return parsed.Host
	}
	return strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
}

func resolveUserAgent(auth *cliproxyauth.Auth) string {
	if auth != nil {
		if auth.Attributes != nil {
			if ua := strings.TrimSpace(auth.Attributes["user_agent"]); ua != "" {
				return ua
			}
		}
		if auth.Metadata != nil {
			if ua, ok := auth.Metadata["user_agent"].(string); ok && strings.TrimSpace(ua) != "" {
				return strings.TrimSpace(ua)
			}
		}
	}
	return defaultAntigravityAgent
}

func antigravityBaseURLFallbackOrder(auth *cliproxyauth.Auth) []string {
	if base := resolveCustomAntigravityBaseURL(auth); base != "" {
		return []string{base}
	}
	return []string{
		antigravitySandboxBaseURLDaily,
		antigravityBaseURLDaily,
		antigravityBaseURLProd,
	}
}

func antigravityRetryAfter(statusCode int, headers http.Header, body []byte) *time.Duration {
	if statusCode != http.StatusTooManyRequests {
		return nil
	}
	if headers != nil {
		if val := headers.Get("Retry-After"); val != "" {
			if seconds, err := strconv.Atoi(val); err == nil && seconds > 0 {
				d := time.Duration(seconds) * time.Second
				return &d
			}
			if t, err := time.Parse(time.RFC1123, val); err == nil {
				d := time.Until(t)
				if d > 0 {
					return &d
				}
			}
		}
	}
	if len(body) == 0 {
		return nil
	}
	if retryAfter, err := parseRetryDelay(body); err == nil && retryAfter != nil {
		return retryAfter
	}
	return nil
}

func resolveCustomAntigravityBaseURL(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if v := strings.TrimSpace(auth.Attributes["base_url"]); v != "" {
			return strings.TrimSuffix(v, "/")
		}
	}
	if auth.Metadata != nil {
		if v, ok := auth.Metadata["base_url"].(string); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return strings.TrimSuffix(v, "/")
			}
		}
	}
	return ""
}

func geminiToAntigravity(modelName string, payload []byte, projectID string, stream bool) []byte {
	template, _ := sjson.Set(string(payload), "model", modelName)
	template, _ = sjson.Set(template, "userAgent", "antigravity")
	template, _ = sjson.Set(template, "requestType", "agent")

	// Use real project ID from auth if available, otherwise generate random (legacy fallback)
	if projectID != "" {
		template, _ = sjson.Set(template, "project", projectID)
	} else {
		template, _ = sjson.Set(template, "project", generateProjectID())
	}
	template, _ = sjson.Set(template, "requestId", generateRequestID())
	template, _ = sjson.Set(template, "request.sessionId", generateStableSessionID(payload))

	template, _ = sjson.Delete(template, "request.safetySettings")
	template, _ = sjson.Set(template, "request.toolConfig.functionCallingConfig.mode", "VALIDATED")

	if strings.Contains(modelName, "claude") {
		gjson.Get(template, "request.tools").ForEach(func(key, tool gjson.Result) bool {
			tool.Get("functionDeclarations").ForEach(func(funKey, funcDecl gjson.Result) bool {
				if funcDecl.Get("parametersJsonSchema").Exists() {
					template, _ = sjson.SetRaw(template, fmt.Sprintf("request.tools.%d.functionDeclarations.%d.parameters", key.Int(), funKey.Int()), funcDecl.Get("parametersJsonSchema").Raw)
					template, _ = sjson.Delete(template, fmt.Sprintf("request.tools.%d.functionDeclarations.%d.parameters.$schema", key.Int(), funKey.Int()))
					template, _ = sjson.Delete(template, fmt.Sprintf("request.tools.%d.functionDeclarations.%d.parametersJsonSchema", key.Int(), funKey.Int()))
				}
				return true
			})
			return true
		})
	} else {
		template, _ = sjson.Delete(template, "request.generationConfig.maxOutputTokens")
	}

	return []byte(template)
}

func generateRequestID() string {
	return "agent-" + uuid.NewString()
}

func generateSessionID() string {
	randSourceMutex.Lock()
	n := randSource.Int63n(9_000_000_000_000_000_000)
	randSourceMutex.Unlock()
	return "-" + strconv.FormatInt(n, 10)
}

func generateStableSessionID(payload []byte) string {
	contents := gjson.GetBytes(payload, "request.contents")
	if contents.IsArray() {
		for _, content := range contents.Array() {
			if content.Get("role").String() == "user" {
				text := content.Get("parts.0.text").String()
				if text != "" {
					h := sha256.Sum256([]byte(text))
					n := int64(binary.BigEndian.Uint64(h[:8])) & 0x7FFFFFFFFFFFFFFF
					return "-" + strconv.FormatInt(n, 10)
				}
			}
		}
	}
	return generateSessionID()
}

func generateProjectID() string {
	adjectives := []string{"useful", "bright", "swift", "calm", "bold"}
	nouns := []string{"fuze", "wave", "spark", "flow", "core"}
	randSourceMutex.Lock()
	adj := adjectives[randSource.Intn(len(adjectives))]
	noun := nouns[randSource.Intn(len(nouns))]
	randSourceMutex.Unlock()
	randomPart := strings.ToLower(uuid.NewString())[:5]
	return adj + "-" + noun + "-" + randomPart
}

func modelName2Alias(modelName string) string {
	switch modelName {
	case "rev19-uic3-1p":
		return "gemini-2.5-computer-use-preview-10-2025"
	case "rev19-uic3-img-1p":
		return "gemini-2.5-image-pro-preview"
	case "rev19-f1-1p":
		return "gemini-2.5-flash"
	case "rev19-f1-lite-1p":
		return "gemini-2.5-flash-lite"
	case "gemini-3-flash", "gemini-3-flash-high":
		return "gemini-3-flash-preview"
	case "gemini-3-pro-image":
		return "gemini-3-pro-image-preview"
	case "gemini-3-pro-high":
		return "gemini-3-pro-preview"
	case "gemini-3-pro-low":
		return "gemini-3-pro-low-preview"
	case "claude-sonnet-4-5":
		return "antigravity-claude-sonnet-4-5"
	case "claude-sonnet-4-5-thinking":
		return "antigravity-claude-sonnet-4-5-thinking"
	case "claude-opus-4-5-thinking":
		return "antigravity-claude-opus-4-5-thinking"
	case "chat_20706", "chat_23310", "gemini-2.5-flash-thinking", "gemini-2.5-pro":
		return ""
	default:
		return modelName
	}
}

func alias2ModelName(modelName string) string {
	switch modelName {
	case "gemini-2.5-computer-use-preview-10-2025":
		return "rev19-uic3-1p"
	case "gemini-2.5-image-pro-preview":
		return "rev19-uic3-img-1p"
	case "gemini-2.5-flash":
		return "rev19-f1-1p"
	case "gemini-2.5-flash-lite":
		return "rev19-f1-lite-1p"
	case "gemini-3-flash-preview":
		return "gemini-3-flash"
	case "gemini-3-pro-image-preview":
		return "gemini-3-pro-image"
	case "gemini-3-pro-preview":
		return "gemini-3-pro-high"
	case "gemini-3-pro-low-preview":
		return "gemini-3-pro-low"
	case "gemini-claude-sonnet-4-5":
		return "claude-sonnet-4-5"
	case "gemini-claude-sonnet-4-5-thinking":
		return "claude-sonnet-4-5-thinking"
	case "gemini-claude-opus-4-5-thinking":
		return "claude-opus-4-5-thinking"
	default:
		return modelName
	}
}

// normalizeAntigravityThinking clamps or removes thinking config based on model support.
// For Claude models, it additionally ensures thinking budget < max_tokens.
func normalizeAntigravityThinking(model string, payload []byte) []byte {
	payload = util.StripThinkingConfigIfUnsupported(model, payload)
	if !util.ModelSupportsThinking(model) {
		return payload
	}
	budget := gjson.GetBytes(payload, "request.generationConfig.thinkingConfig.thinkingBudget")
	if !budget.Exists() {
		return payload
	}
	raw := int(budget.Int())
	normalized := util.NormalizeThinkingBudget(model, raw)

	isClaude := strings.Contains(strings.ToLower(model), "claude")
	if isClaude {
		effectiveMax, setDefaultMax := antigravityEffectiveMaxTokens(model, payload)
		if effectiveMax > 0 && normalized >= effectiveMax {
			normalized = effectiveMax - 1
		}
		minBudget := antigravityMinThinkingBudget(model)
		if minBudget > 0 && normalized >= 0 && normalized < minBudget {
			// Budget is below minimum, remove thinking config entirely
			payload, _ = sjson.DeleteBytes(payload, "request.generationConfig.thinkingConfig")
			return payload
		}
		if setDefaultMax {
			if res, errSet := sjson.SetBytes(payload, "request.generationConfig.maxOutputTokens", effectiveMax); errSet == nil {
				payload = res
			}
		}
	}

	updated, err := sjson.SetBytes(payload, "request.generationConfig.thinkingConfig.thinkingBudget", normalized)
	if err != nil {
		return payload
	}
	return updated
}

// antigravityEffectiveMaxTokens returns the max tokens to cap thinking:
// prefer request-provided maxOutputTokens; otherwise fall back to model default.
// The boolean indicates whether the value came from the model default (and thus should be written back).
func antigravityEffectiveMaxTokens(model string, payload []byte) (max int, fromModel bool) {
	if maxTok := gjson.GetBytes(payload, "request.generationConfig.maxOutputTokens"); maxTok.Exists() && maxTok.Int() > 0 {
		return int(maxTok.Int()), false
	}
	if modelInfo := registry.GetGlobalRegistry().GetModelInfo(model, ""); modelInfo != nil && modelInfo.MaxCompletionTokens > 0 {
		return modelInfo.MaxCompletionTokens, true
	}
	return 0, false
}

// antigravityMinThinkingBudget returns the minimum thinking budget for a model.
// Falls back to -1 if no model info is found.
func antigravityMinThinkingBudget(model string) int {
	if modelInfo := registry.GetGlobalRegistry().GetModelInfo(model, ""); modelInfo != nil && modelInfo.Thinking != nil {
		return modelInfo.Thinking.Min
	}
	return -1
}

func formatErrorMessage(body []byte, auth *cliproxyauth.Auth) string {
	msg := strings.TrimSpace(string(body))
	if val := gjson.GetBytes(body, "error.message"); val.Exists() {
		msg = val.String()
	} else if val := gjson.GetBytes(body, "message"); val.Exists() {
		msg = val.String()
	}

	if _, acc := auth.AccountInfo(); acc != "" {
		msg += " (Account: " + acc + ")"
	}
	return "API Error: " + msg
}
