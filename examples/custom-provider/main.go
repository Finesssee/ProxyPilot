// Package main demonstrates how to create a custom AI provider executor
// and integrate it with the CLI Proxy API server. This example shows how to:
// - Create a custom executor that implements the Executor interface
// - Register custom translators for request/response transformation
// - Integrate the custom provider with the SDK server
// - Register custom models in the model registry
//
// This example uses a simple echo service (httpbin.org) as the upstream API
// for demonstration purposes. In a real implementation, you would replace
// this with your actual AI service provider.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	clipexec "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/logging"
	sdktr "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
	"github.com/tiktoken-go/tokenizer"
)

const (
	// providerKey is the identifier for our custom provider.
	providerKey = "myprov"

	// fOpenAI represents the OpenAI chat format.
	fOpenAI = sdktr.Format("openai.chat")

	// fMyProv represents our custom provider's chat format.
	fMyProv = sdktr.Format("myprov.chat")
)

// init registers trivial translators for demonstration purposes.
// In a real implementation, you would implement proper request/response
// transformation logic between OpenAI format and your provider's format.
func init() {
	sdktr.Register(fOpenAI, fMyProv,
		func(model string, raw []byte, stream bool) []byte { return raw },
		sdktr.ResponseTransform{
			Stream: func(ctx context.Context, model string, originalReq, translatedReq, raw []byte, param *any) []string {
				return []string{string(raw)}
			},
			NonStream: func(ctx context.Context, model string, originalReq, translatedReq, raw []byte, param *any) string {
				return string(raw)
			},
		},
	)
}

// ============================================================================
// Token Counting Utilities
// ============================================================================
//
// This section demonstrates how to implement token counting for a custom provider.
// Token counting is important for:
//   - Quota management and billing estimation
//   - Ensuring requests don't exceed model context limits
//   - Client-side optimization and request batching
//
// There are several approaches you can use:
//
// 1. SIMPLE ESTIMATION (shown in estimateTokens):
//    - Fast and requires no external dependencies beyond string operations
//    - Uses character count / 4 as a rough approximation
//    - Good for quick estimates, less accurate for complex text
//
// 2. TIKTOKEN (shown in countWithTiktoken):
//    - Uses OpenAI's tiktoken library for accurate BPE token counting
//    - Best for OpenAI-compatible models (GPT-4, GPT-3.5, etc.)
//    - Very accurate but requires the tiktoken-go package
//
// 3. UPSTREAM API (not shown):
//    - Call your provider's token counting API endpoint
//    - Most accurate but adds latency and API costs
//    - Implement this if your provider offers a /count_tokens endpoint
//
// Choose the approach that best fits your accuracy needs and performance requirements.

var (
	// tokenizerCache stores tokenizer instances to avoid repeated initialization.
	// The sync.Map is safe for concurrent access.
	tokenizerCache sync.Map
)

// getOrCreateTokenizer returns a cached tokenizer codec for the given model.
// This improves performance by avoiding repeated tokenizer initialization.
//
// Customize this function to map your provider's model names to appropriate
// tokenizer encodings. The tiktoken-go library provides encodings for:
//   - O200kBase: GPT-4o, GPT-5, and newer models (default fallback)
//   - Cl100kBase: GPT-4, GPT-3.5-turbo
//   - P50kBase: text-davinci-003, Codex models
func getOrCreateTokenizer(model string) (tokenizer.Codec, error) {
	// Check cache first
	if cached, ok := tokenizerCache.Load(model); ok {
		return cached.(tokenizer.Codec), nil
	}

	// Select encoding based on model prefix
	// Customize this mapping for your provider's model naming convention
	var enc tokenizer.Codec
	var err error

	sanitized := strings.ToLower(strings.TrimSpace(model))
	switch {
	// Example: Map your custom models to appropriate encodings
	case strings.HasPrefix(sanitized, "myprov-gpt4"):
		enc, err = tokenizer.ForModel(tokenizer.GPT4)
	case strings.HasPrefix(sanitized, "myprov-gpt3"):
		enc, err = tokenizer.ForModel(tokenizer.GPT35Turbo)
	case strings.HasPrefix(sanitized, "gpt-4o"), strings.HasPrefix(sanitized, "gpt-4.1"):
		enc, err = tokenizer.ForModel(tokenizer.GPT4o)
	case strings.HasPrefix(sanitized, "gpt-4"):
		enc, err = tokenizer.ForModel(tokenizer.GPT4)
	case strings.HasPrefix(sanitized, "gpt-3.5"):
		enc, err = tokenizer.ForModel(tokenizer.GPT35Turbo)
	default:
		// Default to O200kBase - the most recent encoding for modern models
		enc, err = tokenizer.Get(tokenizer.O200kBase)
	}

	if err != nil {
		return nil, err
	}

	// Store in cache (use LoadOrStore to handle race conditions)
	actual, _ := tokenizerCache.LoadOrStore(model, enc)
	return actual.(tokenizer.Codec), nil
}

// estimateTokens provides a simple token count estimation without external libraries.
// This is useful as a fallback when tiktoken is unavailable or for quick approximations.
//
// The algorithm uses character count / 4, which is based on the observation that
// English text averages about 4 characters per token. This is less accurate for:
//   - Code (typically more tokens per character)
//   - Non-English text (varies by language)
//   - Text with many special characters or numbers
//
// For production use, consider tiktoken for better accuracy.
func estimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Simple estimation: ~4 characters per token on average
	// This is a reasonable approximation for English text
	tokens := len(text) / 4
	if tokens == 0 && len(text) > 0 {
		tokens = 1 // At least 1 token for non-empty text
	}
	return tokens
}

// countWithTiktoken counts tokens using the tiktoken BPE tokenizer.
// This provides accurate token counts for OpenAI-compatible models.
//
// For custom providers with different tokenization schemes, you may need to:
//   - Apply an adjustment factor (e.g., multiply by 1.1 for Claude models)
//   - Use a custom tokenizer that matches your provider's implementation
//   - Call your provider's native token counting API instead
func countWithTiktoken(enc tokenizer.Codec, text string) (int, error) {
	if enc == nil {
		return 0, fmt.Errorf("tokenizer is nil")
	}
	_, tokens, err := enc.Encode(text)
	if err != nil {
		return 0, err
	}
	return len(tokens), nil
}

// extractMessagesText extracts all text content from an OpenAI-format chat request.
// This handles the standard messages array structure with role/content fields.
//
// Customize this function if your provider uses a different request format.
func extractMessagesText(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	root := gjson.ParseBytes(payload)
	var segments []string

	// Extract from messages array (OpenAI chat format)
	messages := root.Get("messages")
	if messages.Exists() && messages.IsArray() {
		messages.ForEach(func(_, msg gjson.Result) bool {
			// Extract role for context (adds ~1 token overhead per message)
			if role := msg.Get("role").String(); role != "" {
				segments = append(segments, role)
			}
			// Extract text content
			content := msg.Get("content")
			if content.IsArray() {
				// Handle array content (e.g., with images)
				content.ForEach(func(_, part gjson.Result) bool {
					if part.Get("type").String() == "text" {
						if text := part.Get("text").String(); text != "" {
							segments = append(segments, text)
						}
					}
					return true
				})
			} else if text := content.String(); text != "" {
				segments = append(segments, text)
			}
			return true
		})
	}

	// Extract from prompt field (completion format)
	if prompt := root.Get("prompt").String(); prompt != "" {
		segments = append(segments, prompt)
	}

	// Extract from input field (some APIs use this)
	if input := root.Get("input").String(); input != "" {
		segments = append(segments, input)
	}

	// Extract system prompt if present at top level
	if system := root.Get("system").String(); system != "" {
		segments = append(segments, system)
	}

	return strings.Join(segments, "\n")
}

// MyExecutor is a minimal provider implementation for demonstration purposes.
// It implements the Executor interface to handle requests to a custom AI provider.
type MyExecutor struct{}

// Identifier returns the unique identifier for this executor.
func (MyExecutor) Identifier() string { return providerKey }

// PrepareRequest optionally injects credentials to raw HTTP requests.
// This method is called before each request to allow the executor to modify
// the HTTP request with authentication headers or other necessary modifications.
//
// Parameters:
//   - req: The HTTP request to prepare
//   - a: The authentication information
//
// Returns:
//   - error: An error if request preparation fails
func (MyExecutor) PrepareRequest(req *http.Request, a *coreauth.Auth) error {
	if req == nil || a == nil {
		return nil
	}
	if a.Attributes != nil {
		if ak := strings.TrimSpace(a.Attributes["api_key"]); ak != "" {
			req.Header.Set("Authorization", "Bearer "+ak)
		}
	}
	return nil
}

func buildHTTPClient(a *coreauth.Auth) *http.Client {
	if a == nil || strings.TrimSpace(a.ProxyURL) == "" {
		return http.DefaultClient
	}
	u, err := url.Parse(a.ProxyURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return http.DefaultClient
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
}

func upstreamEndpoint(a *coreauth.Auth) string {
	if a != nil && a.Attributes != nil {
		if ep := strings.TrimSpace(a.Attributes["endpoint"]); ep != "" {
			return ep
		}
	}
	// Demo echo endpoint; replace with your upstream.
	return "https://httpbin.org/post"
}

func (MyExecutor) Execute(ctx context.Context, a *coreauth.Auth, req clipexec.Request, opts clipexec.Options) (clipexec.Response, error) {
	client := buildHTTPClient(a)
	endpoint := upstreamEndpoint(a)

	httpReq, errNew := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(req.Payload))
	if errNew != nil {
		return clipexec.Response{}, errNew
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Inject credentials via PrepareRequest hook.
	_ = (MyExecutor{}).PrepareRequest(httpReq, a)

	resp, errDo := client.Do(httpReq)
	if errDo != nil {
		return clipexec.Response{}, errDo
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			// Best-effort close; log if needed in real projects.
		}
	}()
	body, _ := io.ReadAll(resp.Body)
	return clipexec.Response{Payload: body}, nil
}

// CountTokens counts tokens in the request payload.
// This implementation demonstrates a layered approach:
//  1. Try tiktoken for accurate BPE-based counting
//  2. Fall back to simple character-based estimation if tiktoken fails
//
// The response format follows the Anthropic API convention: {"count": <number>}
// Adjust the response format to match your provider's expected format.
//
// For production use, consider:
//   - Caching token counts for identical payloads
//   - Using your provider's native token counting API if available
//   - Applying model-specific adjustment factors
func (MyExecutor) CountTokens(_ context.Context, _ *coreauth.Auth, req clipexec.Request, _ clipexec.Options) (clipexec.Response, error) {
	var tokenCount int

	// Extract text content from the request payload
	text := extractMessagesText(req.Payload)

	// If no text was extracted from structured fields, use raw payload
	if text == "" && len(req.Payload) > 0 {
		text = string(req.Payload)
	}

	// Try tiktoken first for accurate counting
	enc, err := getOrCreateTokenizer(req.Model)
	if err == nil {
		// Successfully got a tokenizer, use it for accurate counting
		count, countErr := countWithTiktoken(enc, text)
		if countErr == nil {
			tokenCount = count
		} else {
			// Tiktoken encoding failed, fall back to estimation
			tokenCount = estimateTokens(text)
		}
	} else {
		// Could not get tokenizer, fall back to simple estimation
		tokenCount = estimateTokens(text)
	}

	// Return response in the expected format
	// The format follows Anthropic's count_tokens response: {"count": <number>}
	// Adjust this format to match your provider's API specification
	response := fmt.Sprintf(`{"count":%d}`, tokenCount)

	return clipexec.Response{Payload: []byte(response)}, nil
}

func (MyExecutor) Embed(context.Context, *coreauth.Auth, clipexec.Request, clipexec.Options) (clipexec.Response, error) {
	return clipexec.Response{}, errors.New("embeddings not implemented")
}

func (MyExecutor) ExecuteStream(ctx context.Context, a *coreauth.Auth, req clipexec.Request, opts clipexec.Options) (<-chan clipexec.StreamChunk, error) {
	ch := make(chan clipexec.StreamChunk, 1)
	go func() {
		defer close(ch)
		ch <- clipexec.StreamChunk{Payload: []byte("data: {\"ok\":true}\n\n")}
	}()
	return ch, nil
}

func (MyExecutor) Refresh(ctx context.Context, a *coreauth.Auth) (*coreauth.Auth, error) {
	return a, nil
}

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		panic(err)
	}

	tokenStore := sdkAuth.GetTokenStore()
	if dirSetter, ok := tokenStore.(interface{ SetBaseDir(string) }); ok {
		dirSetter.SetBaseDir(cfg.AuthDir)
	}
	core := coreauth.NewManager(tokenStore, nil, nil)
	core.RegisterExecutor(MyExecutor{})

	hooks := cliproxy.Hooks{
		OnAfterStart: func(s *cliproxy.Service) {
			// Register demo models for the custom provider so they appear in /v1/models.
			models := []*cliproxy.ModelInfo{{ID: "myprov-pro-1", Object: "model", Type: providerKey, DisplayName: "MyProv Pro 1"}}
			for _, a := range core.List() {
				if strings.EqualFold(a.Provider, providerKey) {
					cliproxy.GlobalModelRegistry().RegisterClient(a.ID, providerKey, models)
				}
			}
		},
	}

	svc, err := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath("config.yaml").
		WithCoreAuthManager(core).
		WithServerOptions(
			// Optional: add a simple middleware + custom request logger
			api.WithMiddleware(func(c *gin.Context) { c.Header("X-Example", "custom-provider"); c.Next() }),
			api.WithRequestLoggerFactory(func(cfg *config.Config, cfgPath string) logging.RequestLogger {
				return logging.NewFileRequestLogger(true, "logs", filepath.Dir(cfgPath))
			}),
		).
		WithHooks(hooks).
		Build()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
	_ = os.Stderr // keep os import used (demo only)
	_ = time.Second
}
