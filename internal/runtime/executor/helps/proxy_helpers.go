package helps

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/proxyutil"
	log "github.com/sirupsen/logrus"
)

var (
	httpClientCache      = make(map[string]*http.Client)
	httpClientCacheMutex sync.RWMutex
)

// NewProxyAwareHTTPClient creates an HTTP client with proper proxy configuration priority:
// 1. Use auth.ProxyURL if configured (highest priority)
// 2. Use cfg.ProxyURL if auth proxy is not configured
// 3. Use RoundTripper from context if neither are configured
func NewProxyAwareHTTPClient(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth, timeout time.Duration) *http.Client {
	var proxyURL string
	if auth != nil {
		proxyURL = strings.TrimSpace(auth.ProxyURL)
	}
	if proxyURL == "" && cfg != nil {
		proxyURL = strings.TrimSpace(cfg.ProxyURL)
	}

	var ctxTransport http.RoundTripper
	if ctx != nil {
		if rt, ok := ctx.Value("cliproxy.roundtripper").(http.RoundTripper); ok && rt != nil {
			ctxTransport = rt
		}
	}

	// Context-bound round trippers are request-scoped and must not be served
	// from or written into the shared empty-proxy cache.
	if proxyURL == "" && ctxTransport != nil {
		httpClient := &http.Client{Transport: ctxTransport}
		if timeout > 0 {
			httpClient.Timeout = timeout
		}
		return httpClient
	}

	cacheKey := proxyURL

	httpClientCacheMutex.RLock()
	if cachedClient, ok := httpClientCache[cacheKey]; ok {
		httpClientCacheMutex.RUnlock()
		if timeout > 0 {
			return &http.Client{
				Transport: cachedClient.Transport,
				Timeout:   timeout,
			}
		}
		return cachedClient
	}
	httpClientCacheMutex.RUnlock()

	httpClient := &http.Client{}
	if timeout > 0 {
		httpClient.Timeout = timeout
	}

	if proxyURL != "" {
		transport := buildProxyTransport(proxyURL)
		if transport != nil {
			httpClient.Transport = transport
			httpClientCacheMutex.Lock()
			httpClientCache[cacheKey] = httpClient
			httpClientCacheMutex.Unlock()
			return httpClient
		}
		log.Debugf("failed to setup proxy from URL: %s, falling back to context transport", proxyURL)
	}

	if ctxTransport != nil {
		httpClient.Transport = ctxTransport
	}

	if proxyURL == "" {
		httpClientCacheMutex.Lock()
		httpClientCache[cacheKey] = httpClient
		httpClientCacheMutex.Unlock()
	}

	return httpClient
}

func buildProxyTransport(proxyURL string) *http.Transport {
	transport, _, errBuild := proxyutil.BuildHTTPTransport(proxyURL)
	if errBuild != nil {
		log.Errorf("%v", errBuild)
		return nil
	}
	return transport
}
