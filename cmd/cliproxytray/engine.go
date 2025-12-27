//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	log "github.com/sirupsen/logrus"
)

// EmbeddedEngine runs the proxy service in-process within the tray application.
// It provides thread-safe methods for starting, stopping, and querying the service status.
type EmbeddedEngine struct {
	mu sync.Mutex

	// running indicates whether the service is currently active.
	running bool

	// port is the port number the service is listening on.
	port int

	// configPath stores the path to the configuration file.
	configPath string

	// lastError holds the most recent error encountered during operation.
	lastError error

	// startedAt records when the service was started.
	startedAt time.Time

	// cancel is the function to cancel the service context.
	cancel context.CancelFunc

	// wg waits for the service goroutine to complete.
	wg sync.WaitGroup
}

// NewEmbeddedEngine creates a new EmbeddedEngine instance.
func NewEmbeddedEngine() *EmbeddedEngine {
	return &EmbeddedEngine{}
}

// Start builds and runs the proxy service in a background goroutine.
// It returns an error if the service is already running or if building the service fails.
func (e *EmbeddedEngine) Start(cfg *config.Config, configPath, password string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return errors.New("engine is already running")
	}

	if cfg == nil {
		return errors.New("configuration is required")
	}

	if configPath == "" {
		return errors.New("configuration path is required")
	}

	// Build the service using the SDK builder pattern
	builder := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(configPath).
		WithLocalManagementPassword(password)

	service, err := builder.Build()
	if err != nil {
		e.lastError = fmt.Errorf("failed to build proxy service: %w", err)
		return e.lastError
	}

	// Create a cancelable context for the service
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	// Track state
	e.running = true
	e.port = cfg.Port
	e.configPath = configPath
	e.lastError = nil
	e.startedAt = time.Now()

	log.Infof("starting embedded proxy engine on port %d", e.port)

	// Run the service in a goroutine
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			e.running = false
			e.cancel = nil
			e.mu.Unlock()
			log.Info("embedded proxy engine stopped")
		}()

		err := service.Run(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			e.mu.Lock()
			e.lastError = fmt.Errorf("proxy service exited with error: %w", err)
			e.mu.Unlock()
			log.Errorf("embedded proxy engine error: %v", err)
		}
	}()

	return nil
}

// Stop cancels the running service and waits for it to shut down.
// It returns an error if the service is not running.
func (e *EmbeddedEngine) Stop() error {
	e.mu.Lock()

	if !e.running {
		e.mu.Unlock()
		return errors.New("engine is not running")
	}

	if e.cancel != nil {
		log.Info("stopping embedded proxy engine...")
		e.cancel()
	}

	e.mu.Unlock()

	// Wait for the goroutine to complete
	e.wg.Wait()

	return nil
}

// Restart stops the running service (if any) and starts it again with the new configuration.
func (e *EmbeddedEngine) Restart(cfg *config.Config, configPath, password string) error {
	// Stop if running (ignore error if not running)
	if e.IsRunning() {
		if err := e.Stop(); err != nil {
			log.Warnf("error stopping engine during restart: %v", err)
		}
	}

	// Start with new configuration
	return e.Start(cfg, configPath, password)
}

// Status returns the current status of the embedded engine.
// The returned status is compatible with desktopctl.Status for UI integration.
func (e *EmbeddedEngine) Status() desktopctl.Status {
	e.mu.Lock()
	defer e.mu.Unlock()

	status := desktopctl.Status{
		Running:    e.running,
		Managed:    true, // Embedded engine is always managed by the tray
		Port:       e.port,
		ConfigPath: e.configPath,
		StartedAt:  e.startedAt,
	}

	if e.running && e.port > 0 {
		status.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", e.port)
	}

	if e.lastError != nil {
		status.LastError = e.lastError.Error()
	}

	return status
}

// IsRunning returns whether the engine is currently running.
func (e *EmbeddedEngine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// LastError returns the most recent error encountered, or nil if none.
func (e *EmbeddedEngine) LastError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastError
}

// Port returns the port number the engine is configured to listen on.
func (e *EmbeddedEngine) Port() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.port
}
