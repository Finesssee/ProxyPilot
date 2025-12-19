// Package executor provides runtime execution capabilities for various AI service providers.
package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/sjson"
)

const (
	// copilotEndpoint is the base URL for the GitHub Copilot API.
	copilotEndpoint = "https://api.githubcopilot.com"
)

// CopilotExecutor is a stateless executor for GitHub Copilot.
type CopilotExecutor struct {
	cfg *config.Config
}

// NewCopilotExecutor creates a new Copilot executor instance.
func NewCopilotExecutor(cfg *config.Config) *CopilotExecutor {
	return &CopilotExecutor{cfg: cfg}
}

// Identifier returns the executor identifier.
func (e *CopilotExecutor) Identifier() string { return "copilot" }

// PrepareRequest prepares the HTTP request for execution.
func (e *CopilotExecutor) PrepareRequest(_ *http.Request, _ *cliproxyauth.Auth) error { return nil }

// Execute performs a non-streaming request to the Copilot API.
func (e *CopilotExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (resp cliproxyexecutor.Response, err error) {
	bearer := copilotToken(auth)

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	upstreamModel := util.ResolveOriginalModel(req.Model, req.Metadata)

	// Translate request to OpenAI format (Copilot uses OpenAI-compatible API)
	from := opts.SourceFormat
	to := sdktranslator.FromString("openai")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), false)
	body = applyPayloadConfig(e.cfg, req.Model, body)
	body, _ = sjson.SetBytes(body, "model", upstreamModel)
	body, _ = sjson.DeleteBytes(body, "stream")

	url := fmt.Sprintf("%s/chat/completions", copilotEndpoint)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+bearer)
	}
	// Required header for Copilot
	httpReq.Header.Set("Editor-Version", "vscode/1.85.1")
	httpReq.Header.Set("Editor-Plugin-Version", "copilot-chat/0.12.1")
	httpReq.Header.Set("User-Agent", "GitHubCopilotChat/0.12.1")

	applyCopilotHeaders(httpReq, auth)

	var authID, authLabel, authType, authValue string
	if auth != nil {
		authID = auth.ID
		authLabel = auth.Label
		authType, authValue = auth.AccountInfo()
	}
	recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
		URL:       url,
		Method:    http.MethodPost,
		Headers:   httpReq.Header.Clone(),
		Body:      body,
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
			log.Errorf("copilot executor: close response body error: %v", errClose)
		}
	}()
	recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		b, _ := io.ReadAll(httpResp.Body)
		appendAPIResponseChunk(ctx, e.cfg, b)
		log.Debugf("request error, error status: %d, error body: %s", httpResp.StatusCode, summarizeErrorBody(httpResp.Header.Get("Content-Type"), b))
		err = statusErr{code: httpResp.StatusCode, msg: string(b)}
		return resp, err
	}

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		recordAPIResponseError(ctx, e.cfg, err)
		return resp, err
	}
	appendAPIResponseChunk(ctx, e.cfg, data)
	reporter.publish(ctx, parseOpenAIUsage(data))

	var param any
	out := sdktranslator.TranslateNonStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), body, data, &param)
	resp = cliproxyexecutor.Response{Payload: []byte(out)}
	return resp, nil
}

// ExecuteStream performs a streaming request to the Copilot API.
func (e *CopilotExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (stream <-chan cliproxyexecutor.StreamChunk, err error) {
	bearer := copilotToken(auth)

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	upstreamModel := util.ResolveOriginalModel(req.Model, req.Metadata)

	from := opts.SourceFormat
	to := sdktranslator.FromString("openai")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)
	body = applyPayloadConfig(e.cfg, req.Model, body)
	body, _ = sjson.SetBytes(body, "model", upstreamModel)
	body, _ = sjson.SetBytes(body, "stream", true)

	url := fmt.Sprintf("%s/chat/completions", copilotEndpoint)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+bearer)
	}
	// Required header for Copilot
	httpReq.Header.Set("Editor-Version", "vscode/1.85.1")
	httpReq.Header.Set("Editor-Plugin-Version", "copilot-chat/0.12.1")
	httpReq.Header.Set("User-Agent", "GitHubCopilotChat/0.12.1")

	applyCopilotHeaders(httpReq, auth)

	var authID, authLabel, authType, authValue string
	if auth != nil {
		authID = auth.ID
		authLabel = auth.Label
		authType, authValue = auth.AccountInfo()
	}
	recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
		URL:       url,
		Method:    http.MethodPost,
		Headers:   httpReq.Header.Clone(),
		Body:      body,
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
			log.Errorf("copilot executor: close response body error: %v", errClose)
		}
		err = statusErr{code: httpResp.StatusCode, msg: string(b)}
		return nil, err
	}

	out := make(chan cliproxyexecutor.StreamChunk)
	stream = out
	go func() {
		defer close(out)
		defer func() {
			if errClose := httpResp.Body.Close(); errClose != nil {
				log.Errorf("copilot executor: close response body error: %v", errClose)
			}
		}()
		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(nil, streamScannerBuffer)
		var param any
		for scanner.Scan() {
			line := scanner.Bytes()
			appendAPIResponseChunk(ctx, e.cfg, line)

			// Handle standard SSE format
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}

			payload := bytes.TrimPrefix(line, []byte("data: "))
			if string(payload) == "[DONE]" {
				break
			}

			// Copilot specific: might send empty data or keep-alive
			if len(bytes.TrimSpace(payload)) == 0 {
				continue
			}

			if detail, ok := parseOpenAIStreamUsage(payload); ok {
				reporter.publish(ctx, detail)
			}

			lines := sdktranslator.TranslateStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), body, bytes.Clone(payload), &param)
			for i := range lines {
				out <- cliproxyexecutor.StreamChunk{Payload: []byte(lines[i])}
			}
		}

		lines := sdktranslator.TranslateStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), body, bytes.Clone([]byte("[DONE]")), &param)
		for i := range lines {
			out <- cliproxyexecutor.StreamChunk{Payload: []byte(lines[i])}
		}

		if errScan := scanner.Err(); errScan != nil {
			recordAPIResponseError(ctx, e.cfg, errScan)
			reporter.publishFailure(ctx)
			out <- cliproxyexecutor.StreamChunk{Err: errScan}
		}
	}()
	return stream, nil
}

func (e *CopilotExecutor) CountTokens(context.Context, *cliproxyauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "count tokens not supported"}
}

// Embed performs an embedding request (not supported for Copilot).
func (e *CopilotExecutor) Embed(context.Context, *cliproxyauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "embeddings not supported"}
}

// Refresh refreshes the authentication credentials.
func (e *CopilotExecutor) Refresh(_ context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	// Token refresh logic should be handled by the auth manager or before execution if needed.
	// Since Copilot tokens expire, we might need to check and refresh here if the architecture allows.
	// For now, return as is. The auth layer usually handles the refresh loop if using the standard provider flow.
	return auth, nil
}

func copilotToken(a *cliproxyauth.Auth) string {
	if a == nil {
		return ""
	}
	if a.Metadata != nil {
		if v, ok := a.Metadata["access_token"].(string); ok && v != "" {
			return v
		}
		if token, ok := a.Metadata["token"].(map[string]any); ok && token != nil {
			if v, ok2 := token["access_token"].(string); ok2 && v != "" {
				return v
			}
		}
	}
	return ""
}

func applyCopilotHeaders(req *http.Request, auth *cliproxyauth.Auth) {
	var attrs map[string]string
	if auth != nil {
		attrs = auth.Attributes
	}
	util.ApplyCustomHeadersFromAttrs(req, attrs)
}
