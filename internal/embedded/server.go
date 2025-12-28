// Package embedded provides an in-process proxy server that can be embedded
// directly into the tray application, eliminating the need for a separate engine binary.
package embedded

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	configaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	log "github.com/sirupsen/logrus"
)

// Server represents an embedded proxy server instance.
type Server struct {
	mu         sync.Mutex
	running    bool
	cancel     context.CancelFunc
	done       chan struct{}
	configPath string
	password   string
	port       int
	startedAt  time.Time
}

// NewServer creates a new embedded server instance.
func NewServer() *Server {
	return &Server{}
}

// Start starts the embedded proxy server with the given configuration.
func (s *Server) Start(configPath, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return errors.New("server is already running")
	}

	// Resolve config path
	resolvedPath := strings.TrimSpace(configPath)
	if resolvedPath == "" {
		return errors.New("config path is required")
	}

	// Load .env if present
	if wd, err := os.Getwd(); err == nil {
		_ = godotenv.Load(filepath.Join(wd, ".env"))
	}

	// Load configuration
	cfg, err := config.LoadConfig(resolvedPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve auth-dir to apply default if empty
	if resolvedAuthDir, errResolve := util.ResolveAuthDir(cfg.AuthDir); errResolve == nil {
		cfg.AuthDir = resolvedAuthDir
	}

	s.configPath = resolvedPath
	s.password = password
	s.port = cfg.Port

	// Setup logging with config-driven output
	logging.SetupBaseLogger()
	logging.ConfigureLogOutput(cfg.LoggingToFile, cfg.LogsMaxTotalSizeMB)

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})

	// Build service
	builder := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(resolvedPath).
		WithLocalManagementPassword(password)

	// Add keep-alive endpoint if password is set
	if password != "" {
		builder = builder.WithServerOptions(api.WithKeepAliveEndpoint(10*time.Second, func() {
			log.Warn("keep-alive endpoint idle for 10s (embedded mode - ignoring)")
			// In embedded mode, we don't auto-shutdown on keep-alive timeout
		}))
	}

	// Register SDK providers
	sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())
	configaccess.Register()

	service, err := builder.Build()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to build service: %w", err)
	}

	s.running = true
	s.startedAt = time.Now()

	// Run service in goroutine
	go func() {
		defer close(s.done)
		err := service.Run(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Errorf("embedded proxy service exited with error: %v", err)
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	// Wait briefly for server to start
	time.Sleep(500 * time.Millisecond)

	return nil
}

// Stop stops the embedded proxy server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
	}

	// Wait for shutdown with timeout
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		log.Warn("embedded server shutdown timeout")
	}

	s.running = false
	return nil
}

// Restart restarts the embedded proxy server.
func (s *Server) Restart() error {
	configPath := s.configPath
	password := s.password

	if err := s.Stop(); err != nil {
		return err
	}

	time.Sleep(200 * time.Millisecond)

	return s.Start(configPath, password)
}

// IsRunning returns whether the server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Port returns the port the server is running on.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// StartedAt returns when the server was started.
func (s *Server) StartedAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startedAt
}

// ConfigPath returns the configuration file path.
func (s *Server) ConfigPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configPath
}

// Global embedded server instance
var globalServer = NewServer()

// StartGlobal starts the global embedded server.
func StartGlobal(configPath, password string) error {
	return globalServer.Start(configPath, password)
}

// StopGlobal stops the global embedded server.
func StopGlobal() error {
	return globalServer.Stop()
}

// RestartGlobal restarts the global embedded server.
func RestartGlobal() error {
	return globalServer.Restart()
}

// GlobalIsRunning returns whether the global server is running.
func GlobalIsRunning() bool {
	return globalServer.IsRunning()
}

// GlobalServer returns the global server instance.
func GlobalServer() *Server {
	return globalServer
}
