//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	configaccess "github.com/router-for-me/CLIProxyAPI/v6/internal/access/config_access"
)

const serviceName = "ProxyPilot"
const serviceDisplayName = "ProxyPilot API Proxy"
const serviceDescription = "Local API proxy for AI coding tools"

// proxyService implements svc.Handler
type proxyService struct {
	configPath string
	cancelFunc context.CancelFunc
}

func (s *proxyService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	// Initialize logging
	logging.SetupBaseLogger()

	// Load config
	cfg, err := config.LoadConfigOptional(s.configPath, false)
	if err != nil {
		elog, _ := eventlog.Open(serviceName)
		if elog != nil {
			elog.Error(1, fmt.Sprintf("Failed to load config: %v", err))
			elog.Close()
		}
		return
	}
	if cfg == nil {
		cfg = &config.Config{Port: 8318}
	}

	// Configure logging
	logging.ConfigureLogOutput(cfg)

	// Register token store
	sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())

	// Register access providers
	configaccess.Register(&cfg.SDKConfig)

	// Build and start service using builder pattern
	builder := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(s.configPath)

	service, err := builder.Build()
	if err != nil {
		elog, _ := eventlog.Open(serviceName)
		if elog != nil {
			elog.Error(1, fmt.Sprintf("Failed to build service: %v", err))
			elog.Close()
		}
		return
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	// Start service in background
	go func() {
		if err := service.Run(ctx); err != nil && err != context.Canceled {
			elog, _ := eventlog.Open(serviceName)
			if elog != nil {
				elog.Error(1, fmt.Sprintf("Service error: %v", err))
				elog.Close()
			}
		}
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	// Wait for stop signal
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				return
			case svc.Interrogate:
				changes <- c.CurrentStatus
			}
		}
	}
}

// runService starts the Windows service
func runService(configPath string) error {
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return err
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("Starting %s service", serviceName))

	err = svc.Run(serviceName, &proxyService{configPath: configPath})
	if err != nil {
		elog.Error(1, fmt.Sprintf("Service failed: %v", err))
		return err
	}

	elog.Info(1, fmt.Sprintf("%s service stopped", serviceName))
	return nil
}

// installService installs ProxyPilot as a Windows service
func installService(configPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath = filepath.Clean(exePath)

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	// Check if service exists
	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", serviceName)
	}

	// Build service arguments
	args := []string{"-service"}
	if configPath != "" {
		args = append(args, "-config", configPath)
	}

	s, err = m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName:  serviceDisplayName,
		Description:  serviceDescription,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	}, args...)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	defer s.Close()

	// Set recovery options (restart on failure)
	err = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}, 86400) // Reset failure count after 24 hours
	if err != nil {
		// Non-fatal
	}

	// Install event log source
	err = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		// Non-fatal, may already exist
	}

	fmt.Printf("Service %s installed successfully\n", serviceName)
	return nil
}

// uninstallService removes the Windows service
func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceName, err)
	}
	defer s.Close()

	// Stop the service if running
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		_, _ = s.Control(svc.Stop)
		// Wait for stop
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			status, err = s.Query()
			if err != nil || status.State == svc.Stopped {
				break
			}
		}
	}

	err = s.Delete()
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	// Remove event log source
	_ = eventlog.Remove(serviceName)

	fmt.Printf("Service %s uninstalled successfully\n", serviceName)
	return nil
}

// startService starts the Windows service
func startService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceName, err)
	}
	defer s.Close()

	return s.Start()
}

// stopService stops the Windows service
func stopService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceName, err)
	}
	defer s.Close()

	_, err = s.Control(svc.Stop)
	return err
}

// serviceStatus returns the current service status
func serviceStatus() string {
	m, err := mgr.Connect()
	if err != nil {
		return "unknown"
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return "not installed"
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return "unknown"
	}

	switch status.State {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "starting"
	case svc.StopPending:
		return "stopping"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "resuming"
	case svc.PausePending:
		return "pausing"
	case svc.Paused:
		return "paused"
	default:
		return "unknown"
	}
}

// handleServiceCommand handles service-related CLI commands
func handleServiceCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
	case "install":
		configPath := ""
		if len(args) > 1 {
			configPath = args[1]
		}
		if err := installService(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return true

	case "uninstall", "remove":
		if err := uninstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return true

	case "start":
		if err := startService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service started")
		return true

	case "stop":
		if err := stopService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service stopped")
		return true

	case "status":
		fmt.Printf("Service status: %s\n", serviceStatus())
		return true

	default:
		return false
	}
}
