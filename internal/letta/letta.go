package letta

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	log "github.com/sirupsen/logrus"
)

// StartServer starts the Letta server in a headless background process.
// It redirects Letta's output to logs/letta.log.
func StartServer(ctx context.Context) {
	// Check if letta is available in the system PATH
	lettaPath, err := exec.LookPath("letta")
	if err != nil {
		log.Debug("Letta executable not found in PATH; skipping headless sidecar start.")
		return
	}

	logDir := "logs"
	if base := util.WritablePath(); base != "" {
		logDir = filepath.Join(base, "logs")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Errorf("letta: failed to create log directory: %v", err)
		return
	}

	logFile := filepath.Join(logDir, "letta.log")
	
	// Create a goroutine to manage the Letta process lifecycle
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("Letta sidecar shutting down...")
				return
			default:
				// Open log file for appending
				f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
				if err != nil {
					log.Errorf("letta: failed to open log file: %v", err)
					time.Sleep(10 * time.Second)
					continue
				}

				log.Infof("Starting Letta server sidecar (logs: %s)", logFile)
				
				cmd := exec.CommandContext(ctx, lettaPath, "server")
				cmd.Stdout = f
				cmd.Stderr = f
				
				// Ensure the process is hidden on Windows
				if runtime.GOOS == "windows" {
					cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				}

				// Start Letta
				if err := cmd.Start(); err != nil {
					log.Errorf("letta: failed to start server: %v", err)
					f.Close()
					time.Sleep(10 * time.Second)
					continue
				}

				// Wait for process to exit or context to be cancelled
				waitErr := cmd.Wait()
				f.Close()

				if ctx.Err() != nil {
					return
				}

				if waitErr != nil {
					log.Errorf("Letta server exited with error: %v. Restarting in 5s...", waitErr)
				} else {
					log.Warn("Letta server exited normally. Restarting in 5s...")
				}
				
				time.Sleep(5 * time.Second)
			}
		}
	}()
}
