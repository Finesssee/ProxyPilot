// Package api provides the HTTP API server implementation for the CLI Proxy API.
// It includes the main server struct, routing setup, middleware for CORS and authentication,
// and integration with various AI API handlers (OpenAI, Claude, Gemini).
// The server supports hot-reloading of clients and configuration.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	internalHandlers "github.com/router-for-me/CLIProxyAPI/v6/internal/api/handlers"
	managementHandlers "github.com/router-for-me/CLIProxyAPI/v6/internal/api/handlers/management"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cache"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/memory"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers/claude"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers/gemini"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers/openai"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdkConfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const oauthCallbackSuccessHTML = `<html><head><meta charset="utf-8"><title>Authentication successful</title><script>setTimeout(function(){window.close();},5000);</script></head><body><h1>Authentication successful!</h1><p>You can close this window.</p><p>This window will close automatically in 5 seconds.</p></body></html>`

type serverOptionConfig struct {
	extraMiddleware      []gin.HandlerFunc
	engineConfigurator   func(*gin.Engine)
	routerConfigurator   func(*gin.Engine, *handlers.BaseAPIHandler, *config.Config)
	requestLoggerFactory func(*config.Config, string) logging.RequestLogger
	localPassword        string
	keepAliveEnabled     bool
	keepAliveTimeout     time.Duration
	keepAliveOnTimeout   func()
}

// ServerOption customises HTTP server construction.
type ServerOption func(*serverOptionConfig)

func defaultRequestLoggerFactory(cfg *config.Config, configPath string) logging.RequestLogger {
	configDir := filepath.Dir(configPath)
	if base := util.WritablePath(); base != "" {
		return logging.NewFileRequestLogger(cfg.RequestLog, filepath.Join(base, "logs"), configDir)
	}
	return logging.NewFileRequestLogger(cfg.RequestLog, "logs", configDir)
}

// WithMiddleware appends additional Gin middleware during server construction.
func WithMiddleware(mw ...gin.HandlerFunc) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.extraMiddleware = append(cfg.extraMiddleware, mw...)
	}
}

// WithEngineConfigurator allows callers to mutate the Gin engine prior to middleware setup.
func WithEngineConfigurator(fn func(*gin.Engine)) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.engineConfigurator = fn
	}
}

// WithRouterConfigurator appends a callback after default routes are registered.
func WithRouterConfigurator(fn func(*gin.Engine, *handlers.BaseAPIHandler, *config.Config)) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.routerConfigurator = fn
	}
}

// WithLocalManagementPassword stores a runtime-only management password accepted for localhost requests.
func WithLocalManagementPassword(password string) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.localPassword = password
	}
}

// WithKeepAliveEndpoint enables a keep-alive endpoint with the provided timeout and callback.
func WithKeepAliveEndpoint(timeout time.Duration, onTimeout func()) ServerOption {
	return func(cfg *serverOptionConfig) {
		if timeout <= 0 || onTimeout == nil {
			return
		}
		cfg.keepAliveEnabled = true
		cfg.keepAliveTimeout = timeout
		cfg.keepAliveOnTimeout = onTimeout
	}
}

// WithRequestLoggerFactory customises request logger creation.
func WithRequestLoggerFactory(factory func(*config.Config, string) logging.RequestLogger) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.requestLoggerFactory = factory
	}
}

// Server represents the main API server.
// It encapsulates the Gin engine, HTTP server, handlers, and configuration.
type Server struct {
	// engine is the Gin web framework engine instance.
	engine *gin.Engine

	// server is the underlying HTTP server.
	server *http.Server

	// handlers contains the API handlers for processing requests.
	handlers *handlers.BaseAPIHandler

	// cfg holds the current server configuration.
	cfg *config.Config
	// cfgHolder provides race-safe config snapshots for middleware reads.
	cfgHolder atomic.Value

	// oldConfigYaml stores a YAML snapshot of the previous configuration for change detection.
	// This prevents issues when the config object is modified in place by Management API.
	oldConfigYaml []byte

	// accessManager handles request authentication providers.
	accessManager *sdkaccess.Manager

	// requestLogger is the request logger instance for dynamic configuration updates.
	requestLogger logging.RequestLogger
	loggerToggle  func(bool)

	// configFilePath is the absolute path to the YAML config file for persistence.
	configFilePath string

	// currentPath is the absolute path to the current working directory.
	currentPath string

	// wsRoutes tracks registered websocket upgrade paths.
	wsRouteMu     sync.Mutex
	wsRoutes      map[string]struct{}
	wsAuthChanged func(bool, bool)
	wsAuthEnabled atomic.Bool

	// management handler
	mgmt *managementHandlers.Handler

	// managementRoutesRegistered tracks whether the management routes have been attached to the engine.
	managementRoutesRegistered atomic.Bool
	// managementRoutesEnabled controls whether management endpoints serve real handlers.
	managementRoutesEnabled atomic.Bool

	// envManagementSecret indicates whether MANAGEMENT_PASSWORD is configured.
	envManagementSecret bool

	localPassword string

	keepAliveEnabled   bool
	keepAliveTimeout   time.Duration
	keepAliveOnTimeout func()
	keepAliveHeartbeat chan struct{}
	keepAliveStop      chan struct{}

	// agentsV2 is the new unified agent configuration handler
	agentsV2 *managementHandlers.AgentsV2Handler
}

// NewServer creates and initializes a new API server instance.
// It sets up the Gin engine, middleware, routes, and handlers.
//
// Parameters:
//   - cfg: The server configuration
//   - authManager: core runtime auth manager
//   - accessManager: request authentication manager
//
// Returns:
//   - *Server: A new server instance
func NewServer(cfg *config.Config, authManager *auth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...ServerOption) *Server {
	optionState := &serverOptionConfig{
		requestLoggerFactory: defaultRequestLoggerFactory,
	}
	for i := range opts {
		opts[i](optionState)
	}
	// Set gin mode
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create gin engine
	engine := gin.New()
	if optionState.engineConfigurator != nil {
		optionState.engineConfigurator(engine)
	}

	metricsEnabled := cfg.IsMetricsEnabled()
	requestHistoryEnabled := cfg.IsRequestHistoryEnabled()
	agenticHarnessEnabled := cfg.IsAgenticHarnessEnabled()
	promptBudgetEnabled := cfg.IsPromptBudgetEnabled()
	if cfg.CommercialMode {
		metricsEnabled = false
		requestHistoryEnabled = false
		agenticHarnessEnabled = false
		promptBudgetEnabled = false
	}
	middleware.SetMetricsEnabled(metricsEnabled)
	middleware.SetRequestHistoryEnabled(requestHistoryEnabled)
	middleware.SetRequestHistorySampleRate(cfg.GetRequestHistorySampleRate())
	usage.SetSamplingRate(cfg.GetUsageSampleRate())

	// Add middleware
	engine.Use(logging.GinLogrusLogger())
	engine.Use(logging.GinLogrusRecovery())
	engine.Use(middleware.ConnectionTrackerMiddleware())
	engine.Use(middleware.PrometheusMiddleware())
	for _, mw := range optionState.extraMiddleware {
		engine.Use(mw)
	}

	// Add request logging middleware (positioned after recovery, before auth)
	// Resolve logs directory relative to the configuration file directory.
	var requestLogger logging.RequestLogger
	var toggle func(bool)
	if !cfg.CommercialMode {
		if optionState.requestLoggerFactory != nil {
			requestLogger = optionState.requestLoggerFactory(cfg, configFilePath)
		}
		if requestLogger != nil {
			engine.Use(middleware.RequestLoggingMiddleware(requestLogger))
			if setter, ok := requestLogger.(interface{ SetEnabled(bool) }); ok {
				toggle = setter.SetEnabled
			}
		}
	}

	// Add agentic harness middleware for long-running agents (Anthropic-style)
	// Gated by CLIPROXY_HARNESS_ENABLED env var (default: true)
	if agenticHarnessEnabled {
		engine.Use(middleware.AgenticHarnessMiddleware())
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = configFilePath
	}

	// Add context compression middleware for Claude Code / Codex CLI
	// Trims long conversations and uses LLM summarization (Factory.ai pattern)
	if promptBudgetEnabled {
		engine.Use(middleware.CodexPromptBudgetMiddlewareWithRootDir(wd))
	}

	wd, err = os.Getwd()
	if err != nil {
		wd = configFilePath
	}

	envAdminPassword, envAdminPasswordSet := os.LookupEnv("MANAGEMENT_PASSWORD")
	envAdminPassword = strings.TrimSpace(envAdminPassword)
	envManagementSecret := envAdminPasswordSet && envAdminPassword != ""

	// Create server instance
	s := &Server{
		engine:              engine,
		handlers:            handlers.NewBaseAPIHandlers(&cfg.SDKConfig, authManager),
		cfg:                 cfg,
		accessManager:       accessManager,
		requestLogger:       requestLogger,
		loggerToggle:        toggle,
		configFilePath:      configFilePath,
		currentPath:         wd,
		envManagementSecret: envManagementSecret,
		wsRoutes:            make(map[string]struct{}),
	}
	s.cfgHolder.Store(cfg)
	s.wsAuthEnabled.Store(cfg.WebsocketAuth)
	// Save initial YAML snapshot
	s.oldConfigYaml, _ = yaml.Marshal(cfg)
	s.applyAccessConfig(nil, cfg)
	cache.InitDefaultResponseCache(buildResponseCacheConfig(cfg))
	cache.InitDefaultPromptCache(buildPromptCacheConfig(cfg))
	if authManager != nil {
		authManager.SetRetryConfig(cfg.RequestRetry, time.Duration(cfg.MaxRetryInterval)*time.Second)
	}
	auth.SetQuotaCooldownDisabled(cfg.DisableCooling)

	// Initialize LLM summarizer with auth manager for Factory.ai-style context compression
	if authManager != nil {
		// Get providers for the summary model (antigravity preferred for gemini-3-flash)
		summaryProviders := util.GetProviderName("gemini-3-flash")
		middleware.InitSummarizerWithAuthManager(&authManagerAdapter{authManager}, summaryProviders)
	}

	// Initialize management handler
	s.mgmt = managementHandlers.NewHandler(cfg, configFilePath, authManager)
	if optionState.localPassword != "" {
		s.mgmt.SetLocalPassword(optionState.localPassword)
	}
	logDir := filepath.Join(s.currentPath, "logs")
	if base := util.WritablePath(); base != "" {
		logDir = filepath.Join(base, "logs")
	}
	s.mgmt.SetLogDirectory(logDir)
	s.localPassword = optionState.localPassword

	// Initialize agents v2 handler
	agentsPort := cfg.Port
	if agentsPort == 0 {
		agentsPort = 8317
	}
	if agentsV2Handler, err := managementHandlers.NewAgentsV2Handler(agentsPort); err == nil {
		s.agentsV2 = agentsV2Handler
	}

	// Setup routes
	engine.Use(corsMiddleware(s.getConfig))
	s.setupRoutes()

	// Register ProxyPilot dashboard routes (embedded UI)
	s.registerProxyPilotDashboardRoutes()

	// Apply additional router configurators from options
	if optionState.routerConfigurator != nil {
		optionState.routerConfigurator(engine, s.handlers, cfg)
	}

	// Register management routes when configuration or environment secrets are available,
	// or when a local password is configured for localhost management access.
	hasManagementSecret := cfg.RemoteManagement.SecretKey != "" || envManagementSecret || optionState.localPassword != ""
	s.managementRoutesEnabled.Store(hasManagementSecret)
	if hasManagementSecret {
		s.registerManagementRoutes()
	}

	if optionState.keepAliveEnabled {
		s.enableKeepAlive(optionState.keepAliveTimeout, optionState.keepAliveOnTimeout)
	}

	// Create HTTP server
	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: engine,
	}

	return s
}

// setupRoutes configures the API routes for the server.
// It defines the endpoints and associates them with their respective handlers.
func (s *Server) setupRoutes() {
	openaiHandlers := openai.NewOpenAIAPIHandler(s.handlers)
	geminiHandlers := gemini.NewGeminiAPIHandler(s.handlers)
	geminiCLIHandlers := gemini.NewGeminiCLIAPIHandler(s.handlers)
	claudeCodeHandlers := claude.NewClaudeCodeAPIHandler(s.handlers)
	openaiResponsesHandlers := openai.NewOpenAIResponsesAPIHandler(s.handlers)
	authMiddleware := AuthMiddleware(s.accessManager, s.allowUnauthenticated)

	// OpenAI compatible API routes
	v1 := s.engine.Group("/v1")
	v1.Use(authMiddleware)
	{
		v1.GET("/models", s.unifiedModelsHandler(openaiHandlers, claudeCodeHandlers))
		v1.POST("/chat/completions", openaiHandlers.ChatCompletions)
		v1.POST("/completions", openaiHandlers.Completions)
		v1.POST("/messages", claudeCodeHandlers.ClaudeMessages)
		v1.POST("/messages/count_tokens", claudeCodeHandlers.ClaudeCountTokens)
		v1.POST("/responses", openaiResponsesHandlers.Responses)

		// Translation API routes
		translatorHandler := internalHandlers.NewTranslatorHandler()
		v1.GET("/translations", translatorHandler.GetTranslationsMatrix)
		v1.GET("/translations/check", translatorHandler.CheckTranslation)
		v1.GET("/translations/docs", translatorHandler.GetTranslationDocs)
		v1.POST("/translations/score", translatorHandler.ScoreTranslation)
		v1.POST("/translations/compare", translatorHandler.CompareStructures)
	}

	// Gemini compatible API routes
	v1beta := s.engine.Group("/v1beta")
	v1beta.Use(authMiddleware)
	{
		v1beta.GET("/models", geminiHandlers.GeminiModels)
		v1beta.POST("/models/*action", geminiHandlers.GeminiHandler)
		v1beta.GET("/models/*action", geminiHandlers.GeminiGetHandler)
	}

	// Health check endpoint for Droid CLI and other clients
	s.engine.GET("/healthz", func(c *gin.Context) {
		cfg := s.getConfig()
		port := 0
		if cfg != nil {
			port = cfg.Port
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "port": port})
	})

	// Prometheus metrics endpoint for observability
	s.engine.GET("/metrics", middleware.MetricsHandler())

	// Root-level API routes (mirrors /v1/* for clients that dont add /v1 prefix)
	s.engine.GET("/models", authMiddleware, s.unifiedModelsHandler(openaiHandlers, claudeCodeHandlers))
	s.engine.POST("/chat/completions", authMiddleware, openaiHandlers.ChatCompletions)
	s.engine.POST("/completions", authMiddleware, openaiHandlers.Completions)
	s.engine.POST("/messages", authMiddleware, claudeCodeHandlers.ClaudeMessages)
	s.engine.POST("/responses", authMiddleware, openaiResponsesHandlers.Responses)

	// Root endpoint
	s.engine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "CLI Proxy API Server",
			"endpoints": []string{
				"POST /v1/chat/completions",
				"POST /v1/completions",
				"GET /v1/models",
			},
		})
	})

	// Event logging endpoint - handles Claude Code telemetry requests
	// Returns 200 OK to prevent 404 errors in logs
	s.engine.POST("/api/event_logging/batch", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	s.engine.POST("/v1internal:method", geminiCLIHandlers.CLIHandler)

	// OAuth callback endpoints (reuse main server port)
	// These endpoints receive provider redirects and persist
	// the short-lived code/state for the waiting goroutine.
	s.engine.GET("/anthropic/callback", func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")
		errStr := c.Query("error")
		if errStr == "" {
			errStr = c.Query("error_description")
		}
		if state != "" {
			if cfg := s.getConfig(); cfg != nil {
				_, _ = managementHandlers.WriteOAuthCallbackFileForPendingSession(cfg.AuthDir, "anthropic", state, code, errStr)
			}
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, oauthCallbackSuccessHTML)
	})

	s.engine.GET("/codex/callback", func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")
		errStr := c.Query("error")
		if errStr == "" {
			errStr = c.Query("error_description")
		}
		if state != "" {
			if cfg := s.getConfig(); cfg != nil {
				_, _ = managementHandlers.WriteOAuthCallbackFileForPendingSession(cfg.AuthDir, "codex", state, code, errStr)
			}
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, oauthCallbackSuccessHTML)
	})

	s.engine.GET("/google/callback", func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")
		errStr := c.Query("error")
		if errStr == "" {
			errStr = c.Query("error_description")
		}
		if state != "" {
			if cfg := s.getConfig(); cfg != nil {
				_, _ = managementHandlers.WriteOAuthCallbackFileForPendingSession(cfg.AuthDir, "gemini", state, code, errStr)
			}
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, oauthCallbackSuccessHTML)
	})

	s.engine.GET("/antigravity/callback", func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")
		errStr := c.Query("error")
		if errStr == "" {
			errStr = c.Query("error_description")
		}
		if state != "" {
			if cfg := s.getConfig(); cfg != nil {
				_, _ = managementHandlers.WriteOAuthCallbackFileForPendingSession(cfg.AuthDir, "antigravity", state, code, errStr)
			}
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, oauthCallbackSuccessHTML)
	})

	s.engine.GET("/kiro/callback", func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")
		errStr := c.Query("error")
		if errStr == "" {
			errStr = c.Query("error_description")
		}
		if state != "" {
			if cfg := s.getConfig(); cfg != nil {
				_, _ = managementHandlers.WriteOAuthCallbackFileForPendingSession(cfg.AuthDir, "kiro", state, code, errStr)
			}
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, oauthCallbackSuccessHTML)
	})

	// Management routes are registered lazily by registerManagementRoutes when a secret is configured.
}

// AttachWebsocketRoute registers a websocket upgrade handler on the primary Gin engine.
// The handler is served as-is without additional middleware beyond the standard stack already configured.
func (s *Server) AttachWebsocketRoute(path string, handler http.Handler) {
	if s == nil || s.engine == nil || handler == nil {
		return
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		trimmed = "/v1/ws"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	s.wsRouteMu.Lock()
	if _, exists := s.wsRoutes[trimmed]; exists {
		s.wsRouteMu.Unlock()
		return
	}
	s.wsRoutes[trimmed] = struct{}{}
	s.wsRouteMu.Unlock()

	authMiddleware := AuthMiddleware(s.accessManager, s.allowUnauthenticated)
	conditionalAuth := func(c *gin.Context) {
		if !s.wsAuthEnabled.Load() {
			c.Next()
			return
		}
		authMiddleware(c)
	}
	finalHandler := func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}

	s.engine.GET(trimmed, conditionalAuth, finalHandler)
}

func (s *Server) registerManagementRoutes() {
	if s == nil || s.engine == nil || s.mgmt == nil {
		return
	}
	if !s.managementRoutesRegistered.CompareAndSwap(false, true) {
		return
	}

	log.Info("management routes registered after secret key configuration")

	mgmt := s.engine.Group("/v0/management")
	mgmt.Use(s.managementAvailabilityMiddleware(), s.mgmt.Middleware())
	{
		mgmt.GET("/usage", s.mgmt.GetUsageStatistics)
		mgmt.GET("/usage/export", s.mgmt.ExportUsageStatistics)
		mgmt.POST("/usage/import", s.mgmt.ImportUsageStatistics)

		// Request monitoring and history routes
		mgmt.GET("/requests", s.mgmt.GetRequests)
		mgmt.GET("/request-history", s.mgmt.GetRequestHistory)
		mgmt.GET("/request-history/stats", s.mgmt.GetRequestHistoryStats)
		mgmt.DELETE("/request-history", s.mgmt.ClearRequestHistory)
		mgmt.GET("/request-history/export", s.mgmt.ExportRequestHistory)
		mgmt.POST("/request-history/import", s.mgmt.ImportRequestHistory)
		mgmt.POST("/request-history/save", s.mgmt.SaveRequestHistory)
		mgmt.GET("/config", s.mgmt.GetConfig)
		mgmt.GET("/config.yaml", s.mgmt.GetConfigYAML)
		mgmt.PUT("/config.yaml", s.mgmt.PutConfigYAML)
		mgmt.GET("/latest-version", s.mgmt.GetLatestVersion)

		mgmt.GET("/debug", s.mgmt.GetDebug)
		mgmt.PUT("/debug", s.mgmt.PutDebug)
		mgmt.PATCH("/debug", s.mgmt.PutDebug)

		mgmt.GET("/logging-to-file", s.mgmt.GetLoggingToFile)
		mgmt.PUT("/logging-to-file", s.mgmt.PutLoggingToFile)
		mgmt.PATCH("/logging-to-file", s.mgmt.PutLoggingToFile)

		mgmt.GET("/usage-statistics-enabled", s.mgmt.GetUsageStatisticsEnabled)
		mgmt.PUT("/usage-statistics-enabled", s.mgmt.PutUsageStatisticsEnabled)
		mgmt.PATCH("/usage-statistics-enabled", s.mgmt.PutUsageStatisticsEnabled)

		mgmt.GET("/proxy-url", s.mgmt.GetProxyURL)
		mgmt.PUT("/proxy-url", s.mgmt.PutProxyURL)
		mgmt.PATCH("/proxy-url", s.mgmt.PutProxyURL)
		mgmt.DELETE("/proxy-url", s.mgmt.DeleteProxyURL)

		mgmt.GET("/quota-exceeded/switch-project", s.mgmt.GetSwitchProject)
		mgmt.PUT("/quota-exceeded/switch-project", s.mgmt.PutSwitchProject)
		mgmt.PATCH("/quota-exceeded/switch-project", s.mgmt.PutSwitchProject)

		mgmt.GET("/quota-exceeded/switch-preview-model", s.mgmt.GetSwitchPreviewModel)
		mgmt.PUT("/quota-exceeded/switch-preview-model", s.mgmt.PutSwitchPreviewModel)
		mgmt.PATCH("/quota-exceeded/switch-preview-model", s.mgmt.PutSwitchPreviewModel)

		mgmt.GET("/api-keys", s.mgmt.GetAPIKeys)
		mgmt.PUT("/api-keys", s.mgmt.PutAPIKeys)
		mgmt.PATCH("/api-keys", s.mgmt.PatchAPIKeys)
		mgmt.DELETE("/api-keys", s.mgmt.DeleteAPIKeys)

		mgmt.GET("/gemini-api-key", s.mgmt.GetGeminiKeys)
		mgmt.PUT("/gemini-api-key", s.mgmt.PutGeminiKeys)
		mgmt.PATCH("/gemini-api-key", s.mgmt.PatchGeminiKey)
		mgmt.DELETE("/gemini-api-key", s.mgmt.DeleteGeminiKey)

		mgmt.GET("/logs", s.mgmt.GetLogs)
		mgmt.DELETE("/logs", s.mgmt.DeleteLogs)
		mgmt.GET("/request-error-logs", s.mgmt.GetRequestErrorLogs)
		mgmt.GET("/request-error-logs/:name", s.mgmt.DownloadRequestErrorLog)
		mgmt.GET("/request-log-by-id/:id", s.mgmt.GetRequestLogByID)
		mgmt.GET("/request-log", s.mgmt.GetRequestLog)
		mgmt.PUT("/request-log", s.mgmt.PutRequestLog)
		mgmt.PATCH("/request-log", s.mgmt.PutRequestLog)
		mgmt.GET("/ws-auth", s.mgmt.GetWebsocketAuth)
		mgmt.PUT("/ws-auth", s.mgmt.PutWebsocketAuth)
		mgmt.PATCH("/ws-auth", s.mgmt.PutWebsocketAuth)

		mgmt.GET("/request-retry", s.mgmt.GetRequestRetry)
		mgmt.PUT("/request-retry", s.mgmt.PutRequestRetry)
		mgmt.PATCH("/request-retry", s.mgmt.PutRequestRetry)
		mgmt.GET("/max-retry-interval", s.mgmt.GetMaxRetryInterval)
		mgmt.PUT("/max-retry-interval", s.mgmt.PutMaxRetryInterval)
		mgmt.PATCH("/max-retry-interval", s.mgmt.PutMaxRetryInterval)

		mgmt.GET("/claude-api-key", s.mgmt.GetClaudeKeys)
		mgmt.PUT("/claude-api-key", s.mgmt.PutClaudeKeys)
		mgmt.PATCH("/claude-api-key", s.mgmt.PatchClaudeKey)
		mgmt.DELETE("/claude-api-key", s.mgmt.DeleteClaudeKey)

		mgmt.GET("/codex-api-key", s.mgmt.GetCodexKeys)
		mgmt.PUT("/codex-api-key", s.mgmt.PutCodexKeys)
		mgmt.PATCH("/codex-api-key", s.mgmt.PatchCodexKey)
		mgmt.DELETE("/codex-api-key", s.mgmt.DeleteCodexKey)

		mgmt.GET("/openai-compatibility", s.mgmt.GetOpenAICompat)
		mgmt.PUT("/openai-compatibility", s.mgmt.PutOpenAICompat)
		mgmt.PATCH("/openai-compatibility", s.mgmt.PatchOpenAICompat)
		mgmt.DELETE("/openai-compatibility", s.mgmt.DeleteOpenAICompat)

		mgmt.GET("/oauth-excluded-models", s.mgmt.GetOAuthExcludedModels)
		mgmt.PUT("/oauth-excluded-models", s.mgmt.PutOAuthExcludedModels)
		mgmt.PATCH("/oauth-excluded-models", s.mgmt.PatchOAuthExcludedModels)
		mgmt.DELETE("/oauth-excluded-models", s.mgmt.DeleteOAuthExcludedModels)

			mgmt.GET("/auth-files", s.mgmt.ListAuthFiles)
			mgmt.GET("/auth-files/models", s.mgmt.GetAuthFileModels)
			mgmt.GET("/auth-files/download", s.mgmt.DownloadAuthFile)
			mgmt.POST("/auth-files", s.mgmt.UploadAuthFile)
			mgmt.DELETE("/auth-files", s.mgmt.DeleteAuthFile)
			mgmt.POST("/auth/reset-cooldown", s.mgmt.ResetAuthCooldown)
			mgmt.PATCH("/auth/:id/priority", s.mgmt.SetAuthPriority)
			mgmt.GET("/auth/:id/usage", s.mgmt.GetAuthUsage)
		mgmt.POST("/vertex/import", s.mgmt.ImportVertexCredential)

		mgmt.GET("/anthropic-auth-url", s.mgmt.RequestAnthropicToken)
		mgmt.GET("/codex-auth-url", s.mgmt.RequestCodexToken)
		mgmt.GET("/gemini-cli-auth-url", s.mgmt.RequestGeminiCLIToken)
		mgmt.GET("/antigravity-auth-url", s.mgmt.RequestAntigravityToken)
		mgmt.GET("/qwen-auth-url", s.mgmt.RequestQwenToken)
		mgmt.GET("/kiro-auth-url", s.mgmt.RequestKiroToken)
		mgmt.POST("/amazonq-import", s.mgmt.ImportAmazonQToken)
		mgmt.POST("/minimax-api-key", s.mgmt.SaveMiniMaxAPIKey)
		mgmt.POST("/zhipu-api-key", s.mgmt.SaveZhipuAPIKey)
		mgmt.POST("/oauth-callback", s.mgmt.PostOAuthCallback)
		mgmt.GET("/get-auth-status", s.mgmt.GetAuthStatus)

		// Harness management routes
		mgmt.GET("/harness/files", s.mgmt.GetHarnessFiles)
		mgmt.GET("/harness/file", s.mgmt.GetHarnessFile)
		mgmt.PUT("/harness/file", s.mgmt.PutHarnessFile)
		mgmt.GET("/harness/export", s.mgmt.ExportHarness)
		mgmt.POST("/harness/import", s.mgmt.ImportHarness)

		// Global model mappings routes
		mgmt.GET("/model-mappings", s.mgmt.GetModelMappings)
		mgmt.PUT("/model-mappings", s.mgmt.SetModelMappings)
		mgmt.GET("/model-mappings/test", s.mgmt.TestModelMapping)

		// Thinking budget routes
		mgmt.GET("/thinking-budget", s.mgmt.GetThinkingBudget)
		mgmt.PUT("/thinking-budget", s.mgmt.SetThinkingBudget)

		// Integration detection and setup routes
		mgmt.GET("/integrations/status", s.mgmt.GetIntegrationsStatus)
		mgmt.POST("/integrations/:id/configure", s.mgmt.PostIntegrationConfigure)
		mgmt.GET("/agents", s.mgmt.GetCLIAgents)
		mgmt.POST("/agents/:id/configure", s.mgmt.PostCLIAgentConfigure)
		mgmt.POST("/agents/:id/unconfigure", s.mgmt.PostCLIAgentUnconfigure)

		// New unified agent configuration routes (v2)
		if s.agentsV2 != nil {
			mgmt.GET("/v2/agents", s.agentsV2.GetAgents)
			mgmt.GET("/v2/agents/state", s.agentsV2.GetAgentState)
			mgmt.POST("/v2/agents/enable-all", s.agentsV2.EnableAllAgents)
			mgmt.POST("/v2/agents/disable-all", s.agentsV2.DisableAllAgents)
			mgmt.GET("/v2/agents/:id", s.agentsV2.GetAgent)
			mgmt.POST("/v2/agents/:id/enable", s.agentsV2.EnableAgent)
			mgmt.POST("/v2/agents/:id/disable", s.agentsV2.DisableAgent)
		}

		// Memory management routes
		mgmt.GET("/memory/sessions", s.mgmt.ListMemorySessions)
		mgmt.GET("/memory/session", s.mgmt.GetMemorySession)
		mgmt.GET("/memory/events", s.mgmt.GetMemoryEvents)
		mgmt.GET("/memory/anchors", s.mgmt.GetMemoryAnchors)
		mgmt.PUT("/memory/todo", s.mgmt.PutMemoryTodo)
		mgmt.PUT("/memory/pinned", s.mgmt.PutMemoryPinned)
		mgmt.PUT("/memory/summary", s.mgmt.PutMemorySummary)
		mgmt.PUT("/memory/semantic-toggle", s.mgmt.PutMemorySemanticToggle)
		mgmt.DELETE("/memory/session", s.mgmt.DeleteMemorySession)
		mgmt.POST("/memory/prune", s.mgmt.PruneMemory)
		mgmt.GET("/memory/export", s.mgmt.ExportMemorySession)
		mgmt.GET("/memory/export-all", s.mgmt.ExportAllMemory)
		mgmt.DELETE("/memory/all", s.mgmt.DeleteAllMemory)
		mgmt.POST("/memory/import", s.mgmt.ImportMemorySession)

		// Semantic memory routes
		mgmt.GET("/semantic/health", s.mgmt.GetSemanticHealth)
		mgmt.GET("/semantic/namespaces", s.mgmt.ListSemanticNamespaces)
		mgmt.GET("/semantic/items", s.mgmt.GetSemanticItems)

		// ProxyPilot diagnostics and logs routes
		mgmt.GET("/proxypilot/logs/tail", s.mgmt.GetProxyPilotLogTail)
		mgmt.GET("/proxypilot/diagnostics", s.mgmt.GetProxyPilotDiagnostics)

		mgmt.GET("/updates/check", s.mgmt.GetUpdateInfo)
		mgmt.GET("/updates/status", s.mgmt.GetUpdateStatus)
		mgmt.POST("/updates/download", s.mgmt.DownloadUpdate)
		mgmt.POST("/updates/verify", s.mgmt.VerifyUpdate)
		mgmt.POST("/updates/install", s.mgmt.InstallUpdate)

		// Cache management routes
		mgmt.GET("/cache/stats", s.mgmt.GetCacheStats)
		mgmt.POST("/cache/clear", s.mgmt.ClearCache)
		mgmt.PUT("/cache/enabled", s.mgmt.SetCacheEnabled)

		// Prompt cache management routes
		mgmt.GET("/prompt-cache/stats", s.mgmt.GetPromptCacheStats)
		mgmt.POST("/prompt-cache/clear", s.mgmt.ClearPromptCache)
		mgmt.PUT("/prompt-cache/enabled", s.mgmt.SetPromptCacheEnabled)
		mgmt.GET("/prompt-cache/top", s.mgmt.GetTopPrompts)
		mgmt.POST("/prompt-cache/warm", s.mgmt.WarmPromptCache)
	}
}

func (s *Server) managementAvailabilityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.managementRoutesEnabled.Load() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	}
}

func (s *Server) enableKeepAlive(timeout time.Duration, onTimeout func()) {
	if timeout <= 0 || onTimeout == nil {
		return
	}

	s.keepAliveEnabled = true
	s.keepAliveTimeout = timeout
	s.keepAliveOnTimeout = onTimeout
	s.keepAliveHeartbeat = make(chan struct{}, 1)
	s.keepAliveStop = make(chan struct{}, 1)

	s.engine.GET("/keep-alive", s.handleKeepAlive)

	go s.watchKeepAlive()
}

func (s *Server) handleKeepAlive(c *gin.Context) {
	if s.localPassword != "" {
		provided := strings.TrimSpace(c.GetHeader("Authorization"))
		if provided != "" {
			parts := strings.SplitN(provided, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				provided = parts[1]
			}
		}
		if provided == "" {
			provided = strings.TrimSpace(c.GetHeader("X-Local-Password"))
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.localPassword)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
			return
		}
	}

	s.signalKeepAlive()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) signalKeepAlive() {
	if !s.keepAliveEnabled {
		return
	}
	select {
	case s.keepAliveHeartbeat <- struct{}{}:
	default:
	}
}

func (s *Server) watchKeepAlive() {
	if !s.keepAliveEnabled {
		return
	}

	timer := time.NewTimer(s.keepAliveTimeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			log.Warnf("keep-alive endpoint idle for %s, shutting down", s.keepAliveTimeout)
			if s.keepAliveOnTimeout != nil {
				s.keepAliveOnTimeout()
			}
			return
		case <-s.keepAliveHeartbeat:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(s.keepAliveTimeout)
		case <-s.keepAliveStop:
			return
		}
	}
}

// unifiedModelsHandler creates a unified handler for the /v1/models endpoint
// that routes to different handlers based on the User-Agent header.
// If User-Agent starts with "claude-cli", it routes to Claude handler,
// otherwise it routes to OpenAI handler.
func (s *Server) unifiedModelsHandler(openaiHandler *openai.OpenAIAPIHandler, claudeHandler *claude.ClaudeCodeAPIHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		userAgent := c.GetHeader("User-Agent")

		// Route to Claude handler if User-Agent starts with "claude-cli"
		if strings.HasPrefix(userAgent, "claude-cli") {
			// log.Debugf("Routing /v1/models to Claude handler for User-Agent: %s", userAgent)
			claudeHandler.ClaudeModels(c)
		} else {
			// log.Debugf("Routing /v1/models to OpenAI handler for User-Agent: %s", userAgent)
			openaiHandler.OpenAIModels(c)
		}
	}
}

// Start begins listening for and serving HTTP or HTTPS requests.
// It's a blocking call and will only return on an unrecoverable error.
//
// Returns:
//   - error: An error if the server fails to start
func (s *Server) Start() error {
	if s == nil || s.server == nil {
		return fmt.Errorf("failed to start HTTP server: server not initialized")
	}

	cfg := s.getConfig()
	useTLS := cfg != nil && cfg.TLS.Enable
	if useTLS {
		cert := strings.TrimSpace(cfg.TLS.Cert)
		key := strings.TrimSpace(cfg.TLS.Key)
		if cert == "" || key == "" {
			return fmt.Errorf("failed to start HTTPS server: tls.cert or tls.key is empty")
		}
		log.Debugf("Starting API server on %s with TLS", s.server.Addr)
		if errServeTLS := s.server.ListenAndServeTLS(cert, key); errServeTLS != nil && !errors.Is(errServeTLS, http.ErrServerClosed) {
			return fmt.Errorf("failed to start HTTPS server: %v", errServeTLS)
		}
		return nil
	}

	log.Debugf("Starting API server on %s", s.server.Addr)
	if errServe := s.server.ListenAndServe(); errServe != nil && !errors.Is(errServe, http.ErrServerClosed) {
		return fmt.Errorf("failed to start HTTP server: %v", errServe)
	}

	return nil
}

// Stop gracefully shuts down the API server without interrupting any
// active connections.
//
// Parameters:
//   - ctx: The context for graceful shutdown
//
// Returns:
//   - error: An error if the server fails to stop
func (s *Server) Stop(ctx context.Context) error {
	log.Debug("Stopping API server...")

	if s.keepAliveEnabled {
		select {
		case s.keepAliveStop <- struct{}{}:
		default:
		}
	}

	// Shutdown the HTTP server.
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %v", err)
	}

	log.Debug("API server stopped")
	return nil
}

// corsMiddleware returns a Gin middleware handler that adds CORS headers
// to every response, allowing cross-origin requests.
//
// Returns:
//   - gin.HandlerFunc: The CORS middleware handler
func corsMiddleware(getCfg func() *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := (*config.Config)(nil)
		if getCfg != nil {
			cfg = getCfg()
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		path := c.Request.URL.Path
		isManagement := strings.HasPrefix(path, "/v0/management")

		allowOrigins := []string{}
		allowMethods := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
		allowHeaders := "*"
		if cfg != nil {
			allowOrigins = cfg.CORS.AllowOrigins
			if isManagement && len(cfg.CORS.ManagementAllowOrigins) > 0 {
				allowOrigins = cfg.CORS.ManagementAllowOrigins
			}
			if len(cfg.CORS.AllowMethods) > 0 {
				allowMethods = strings.Join(cfg.CORS.AllowMethods, ", ")
			}
			if len(cfg.CORS.AllowHeaders) > 0 {
				allowHeaders = strings.Join(cfg.CORS.AllowHeaders, ", ")
			}
		}

		allowedOrigin := ""
		if origin != "" {
			switch {
			case len(allowOrigins) == 0 && !isManagement:
				allowedOrigin = "*"
			case originAllowed(allowOrigins, origin):
				allowedOrigin = origin
			}
		}

		if allowedOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
			c.Header("Access-Control-Allow-Methods", allowMethods)
			c.Header("Access-Control-Allow-Headers", allowHeaders)
			if allowedOrigin != "*" {
				c.Header("Vary", "Origin")
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func originAllowed(allowOrigins []string, origin string) bool {
	if origin == "" || len(allowOrigins) == 0 {
		return false
	}
	for _, allowed := range allowOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

func (s *Server) applyAccessConfig(oldCfg, newCfg *config.Config) {
	if s == nil || s.accessManager == nil || newCfg == nil {
		return
	}
	var oldSDK, newSDK *sdkConfig.SDKConfig
	if oldCfg != nil {
		oldSDK = &oldCfg.SDKConfig
	}
	if newCfg != nil {
		newSDK = &newCfg.SDKConfig
	}
	if _, err := access.ApplyAccessProviders(s.accessManager, oldSDK, newSDK); err != nil {
		return
	}
}

// UpdateClients updates the server's client list and configuration.
// This method is called when the configuration or authentication tokens change.
//
// Parameters:
//   - clients: The new slice of AI service clients
//   - cfg: The new application configuration
func (s *Server) UpdateClients(cfg *config.Config) {
	// Reconstruct old config from YAML snapshot to avoid reference sharing issues
	var oldCfg *config.Config
	if len(s.oldConfigYaml) > 0 {
		_ = yaml.Unmarshal(s.oldConfigYaml, &oldCfg)
	}

	// Update request logger enabled state if it has changed
	previousRequestLog := false
	if oldCfg != nil {
		previousRequestLog = oldCfg.RequestLog
	}
	if s.requestLogger != nil && (oldCfg == nil || previousRequestLog != cfg.RequestLog) {
		if s.loggerToggle != nil {
			s.loggerToggle(cfg.RequestLog)
		} else if toggler, ok := s.requestLogger.(interface{ SetEnabled(bool) }); ok {
			toggler.SetEnabled(cfg.RequestLog)
		}
		if oldCfg != nil {
			log.Debugf("request logging updated from %t to %t", previousRequestLog, cfg.RequestLog)
		} else {
			log.Debugf("request logging toggled to %t", cfg.RequestLog)
		}
	}

	if oldCfg == nil || oldCfg.LoggingToFile != cfg.LoggingToFile || oldCfg.LogsMaxTotalSizeMB != cfg.LogsMaxTotalSizeMB {
		if err := logging.ConfigureLogOutput(cfg.LoggingToFile, cfg.LogsMaxTotalSizeMB); err != nil {
			log.Errorf("failed to reconfigure log output: %v", err)
		} else {
			if oldCfg == nil {
				log.Debug("log output configuration refreshed")
			} else {
				if oldCfg.LoggingToFile != cfg.LoggingToFile {
					log.Debugf("logging_to_file updated from %t to %t", oldCfg.LoggingToFile, cfg.LoggingToFile)
				}
				if oldCfg.LogsMaxTotalSizeMB != cfg.LogsMaxTotalSizeMB {
					log.Debugf("logs_max_total_size_mb updated from %d to %d", oldCfg.LogsMaxTotalSizeMB, cfg.LogsMaxTotalSizeMB)
				}
			}
		}
	}

	if oldCfg == nil || oldCfg.UsageStatisticsEnabled != cfg.UsageStatisticsEnabled {
		usage.SetStatisticsEnabled(cfg.UsageStatisticsEnabled)
		if oldCfg != nil {
			log.Debugf("usage_statistics_enabled updated from %t to %t", oldCfg.UsageStatisticsEnabled, cfg.UsageStatisticsEnabled)
		} else {
			log.Debugf("usage_statistics_enabled toggled to %t", cfg.UsageStatisticsEnabled)
		}
	}

	metricsEnabled := cfg.IsMetricsEnabled()
	requestHistoryEnabled := cfg.IsRequestHistoryEnabled()
	if cfg.CommercialMode {
		metricsEnabled = false
		requestHistoryEnabled = false
	}
	middleware.SetMetricsEnabled(metricsEnabled)
	middleware.SetRequestHistoryEnabled(requestHistoryEnabled)
	middleware.SetRequestHistorySampleRate(cfg.GetRequestHistorySampleRate())
	usage.SetSamplingRate(cfg.GetUsageSampleRate())

	if oldCfg == nil || oldCfg.DisableCooling != cfg.DisableCooling {
		auth.SetQuotaCooldownDisabled(cfg.DisableCooling)
		if oldCfg != nil {
			log.Debugf("disable_cooling updated from %t to %t", oldCfg.DisableCooling, cfg.DisableCooling)
		} else {
			log.Debugf("disable_cooling toggled to %t", cfg.DisableCooling)
		}
	}
	if s.handlers != nil && s.handlers.AuthManager != nil {
		s.handlers.AuthManager.SetRetryConfig(cfg.RequestRetry, time.Duration(cfg.MaxRetryInterval)*time.Second)
	}

	// Update log level dynamically when debug flag changes
	if oldCfg == nil || oldCfg.Debug != cfg.Debug {
		util.SetLogLevel(cfg)
		if oldCfg != nil {
			log.Debugf("debug mode updated from %t to %t", oldCfg.Debug, cfg.Debug)
		} else {
			log.Debugf("debug mode toggled to %t", cfg.Debug)
		}
	}

	prevSecretEmpty := true
	if oldCfg != nil {
		prevSecretEmpty = oldCfg.RemoteManagement.SecretKey == ""
	}
	newSecretEmpty := cfg.RemoteManagement.SecretKey == ""
	if s.envManagementSecret {
		s.registerManagementRoutes()
		if s.managementRoutesEnabled.CompareAndSwap(false, true) {
			log.Info("management routes enabled via MANAGEMENT_PASSWORD")
		} else {
			s.managementRoutesEnabled.Store(true)
		}
	} else {
		switch {
		case prevSecretEmpty && !newSecretEmpty:
			s.registerManagementRoutes()
			if s.managementRoutesEnabled.CompareAndSwap(false, true) {
				log.Info("management routes enabled after secret key update")
			} else {
				s.managementRoutesEnabled.Store(true)
			}
		case !prevSecretEmpty && newSecretEmpty:
			if s.managementRoutesEnabled.CompareAndSwap(true, false) {
				log.Info("management routes disabled after secret key removal")
			} else {
				s.managementRoutesEnabled.Store(false)
			}
		default:
			s.managementRoutesEnabled.Store(!newSecretEmpty)
		}
	}

	s.applyAccessConfig(oldCfg, cfg)
	cache.InitDefaultResponseCache(buildResponseCacheConfig(cfg))
	cache.InitDefaultPromptCache(buildPromptCacheConfig(cfg))
	s.cfg = cfg
	s.cfgHolder.Store(cfg)
	s.wsAuthEnabled.Store(cfg.WebsocketAuth)
	if oldCfg != nil && s.wsAuthChanged != nil && oldCfg.WebsocketAuth != cfg.WebsocketAuth {
		s.wsAuthChanged(oldCfg.WebsocketAuth, cfg.WebsocketAuth)
	}
	// Save YAML snapshot for next comparison
	s.oldConfigYaml, _ = yaml.Marshal(cfg)

	s.handlers.UpdateClients(&cfg.SDKConfig)

	if s.mgmt != nil {
		s.mgmt.SetConfig(cfg)
		s.mgmt.SetAuthManager(s.handlers.AuthManager)
	}

	// Count client sources from configuration and auth directory
	authFiles := util.CountAuthFiles(cfg.AuthDir)
	geminiAPIKeyCount := len(cfg.GeminiKey)
	claudeAPIKeyCount := len(cfg.ClaudeKey)
	codexAPIKeyCount := len(cfg.CodexKey)
	vertexAICompatCount := len(cfg.VertexCompatAPIKey)
	openAICompatCount := 0
	for i := range cfg.OpenAICompatibility {
		entry := cfg.OpenAICompatibility[i]
		openAICompatCount += len(entry.APIKeyEntries)
	}

	total := authFiles + geminiAPIKeyCount + claudeAPIKeyCount + codexAPIKeyCount + vertexAICompatCount + openAICompatCount
	fmt.Printf("server clients and configuration updated: %d clients (%d auth files + %d Gemini API keys + %d Claude API keys + %d Codex keys + %d Vertex-compat + %d OpenAI-compat)\n",
		total,
		authFiles,
		geminiAPIKeyCount,
		claudeAPIKeyCount,
		codexAPIKeyCount,
		vertexAICompatCount,
		openAICompatCount,
	)
}

func (s *Server) SetWebsocketAuthChangeHandler(fn func(bool, bool)) {
	if s == nil {
		return
	}
	s.wsAuthChanged = fn
}

// (management handlers moved to internal/api/handlers/management)

// AuthMiddleware returns a Gin middleware handler that authenticates requests
// using the configured authentication providers. When no providers are available,
// it allows all requests (legacy behaviour).
func AuthMiddleware(manager *sdkaccess.Manager, allowUnauthenticated func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if manager == nil {
			if allowUnauthenticated != nil && allowUnauthenticated() {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing API key"})
			return
		}

		result, err := manager.Authenticate(c.Request.Context(), c.Request)
		if err == nil {
			if result != nil {
				c.Set("apiKey", result.Principal)
				c.Set("accessProvider", result.Provider)
				if len(result.Metadata) > 0 {
					c.Set("accessMetadata", result.Metadata)
				}
			}
			c.Next()
			return
		}

		switch {
		case errors.Is(err, sdkaccess.ErrNoCredentials):
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing API key"})
		case errors.Is(err, sdkaccess.ErrInvalidCredential):
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		default:
			log.Errorf("authentication middleware error: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Authentication service error"})
		}
	}
}

// authManagerAdapter adapts *auth.Manager to the memory.CoreManagerExecutor interface.
// This bridges the typed signature to the interface{} signature expected by the summarizer.
type authManagerAdapter struct {
	manager *auth.Manager
}

// Execute implements memory.CoreManagerExecutor by delegating to the typed manager.
// It converts the interface{} types to/from the concrete executor types.
func (a *authManagerAdapter) Execute(ctx context.Context, providers []string, req interface{}, opts interface{}) (interface{}, error) {
	if a.manager == nil {
		return nil, errors.New("auth manager adapter: manager is nil")
	}

	// Convert the memory package types to cliproxyexecutor types.
	var execReq cliproxyexecutor.Request
	var execOpts cliproxyexecutor.Options

	switch r := req.(type) {
	case nil:
	case cliproxyexecutor.Request:
		execReq = r
	case *cliproxyexecutor.Request:
		if r != nil {
			execReq = *r
		}
	case memory.ExecutorRequest:
		execReq = cliproxyexecutor.Request{
			Model:    r.Model,
			Payload:  r.Payload,
			Metadata: r.Metadata,
		}
	case *memory.ExecutorRequest:
		if r != nil {
			execReq = cliproxyexecutor.Request{
				Model:    r.Model,
				Payload:  r.Payload,
				Metadata: r.Metadata,
			}
		}
	default:
		reqBytes, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("auth manager adapter: failed to marshal request: %w", err)
		}
		if err := json.Unmarshal(reqBytes, &execReq); err != nil {
			return nil, fmt.Errorf("auth manager adapter: failed to unmarshal request: %w", err)
		}
	}

	switch o := opts.(type) {
	case nil:
	case cliproxyexecutor.Options:
		execOpts = o
	case *cliproxyexecutor.Options:
		if o != nil {
			execOpts = *o
		}
	case memory.ExecutorOptions:
		execOpts = cliproxyexecutor.Options{
			Stream:  o.Stream,
			Headers: o.Headers,
		}
	case *memory.ExecutorOptions:
		if o != nil {
			execOpts = cliproxyexecutor.Options{
				Stream:  o.Stream,
				Headers: o.Headers,
			}
		}
	default:
		optsBytes, err := json.Marshal(opts)
		if err != nil {
			return nil, fmt.Errorf("auth manager adapter: failed to marshal options: %w", err)
		}
		if err := json.Unmarshal(optsBytes, &execOpts); err != nil {
			return nil, fmt.Errorf("auth manager adapter: failed to unmarshal options: %w", err)
		}
	}

	resp, err := a.manager.Execute(ctx, providers, execReq, execOpts)
	if err != nil {
		return nil, err
	}

	// Return the response as interface{}
	return resp, nil
}

func (s *Server) getConfig() *config.Config {
	if s == nil {
		return nil
	}
	if v := s.cfgHolder.Load(); v != nil {
		if cfg, ok := v.(*config.Config); ok {
			return cfg
		}
	}
	return s.cfg
}

func (s *Server) allowUnauthenticated() bool {
	cfg := s.getConfig()
	if cfg == nil {
		return false
	}
	return cfg.AllowUnauthenticated
}

func buildResponseCacheConfig(cfg *config.Config) cache.ResponseCacheConfig {
	if cfg == nil {
		return cache.DefaultResponseCacheConfig()
	}
	return cache.ResponseCacheConfig{
		Enabled:       cfg.ResponseCache.Enabled,
		MaxSize:       cfg.ResponseCache.GetMaxSize(),
		MaxBytes:      cfg.ResponseCache.GetMaxBytes(),
		TTL:           cfg.ResponseCache.GetTTL(),
		ExcludeModels: append([]string(nil), cfg.ResponseCache.ExcludeModels...),
		PersistFile:   cfg.ResponseCache.PersistFile,
	}
}

func buildPromptCacheConfig(cfg *config.Config) cache.PromptCacheConfig {
	if cfg == nil {
		return cache.DefaultPromptCacheConfig()
	}
	return cache.PromptCacheConfig{
		Enabled:     cfg.PromptCache.Enabled,
		MaxSize:     cfg.PromptCache.GetMaxSize(),
		MaxBytes:    cfg.PromptCache.GetMaxBytes(),
		TTL:         cfg.PromptCache.GetTTL(),
		PersistFile: cfg.PromptCache.PersistFile,
	}
}
