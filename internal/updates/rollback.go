package updates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
)

// RollbackInfo contains metadata about a backup that can be restored.
type RollbackInfo struct {
	Version     string    `json:"version"`
	BackupPath  string    `json:"backup_path"`
	BackupDate  time.Time `json:"backup_date"`
	CurrentExe  string    `json:"current_exe"`
}

// RollbackResult contains the result of a rollback operation.
type RollbackResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	RestoredVersion string `json:"restored_version,omitempty"`
	NeedsRestart   bool   `json:"needs_restart"`
}

// HealthCheckResult contains the result of a startup health check.
type HealthCheckResult struct {
	Healthy       bool   `json:"healthy"`
	StartupCount  int    `json:"startup_count"`
	LastStartup   time.Time `json:"last_startup"`
	ShouldRollback bool  `json:"should_rollback"`
}

const (
	rollbackMetaFile  = "rollback.json"
	healthCheckFile   = "health.json"
	maxFailedStartups = 3 // Auto-rollback after this many rapid failures
	startupWindow     = 30 * time.Second // Consider startups within this window as "rapid"
)

// healthState tracks startup health for auto-rollback detection.
type healthState struct {
	Version       string      `json:"version"`
	StartupTimes  []time.Time `json:"startup_times"`
	LastHealthy   time.Time   `json:"last_healthy"`
}

// getDataDir returns the data directory for storing rollback metadata.
func getDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config dir: %w", err)
	}

	dataDir := filepath.Join(configDir, "ProxyPilot", "updates")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create updates dir: %w", err)
	}

	return dataDir, nil
}

// SaveRollbackInfo saves metadata about the current backup for later restoration.
// Call this after a successful installation before the app exits.
func SaveRollbackInfo(previousVersion string) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable: %w", err)
	}

	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("failed to resolve executable: %w", err)
	}

	backupPath := currentExe + ".old"

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	info := &RollbackInfo{
		Version:    previousVersion,
		BackupPath: backupPath,
		BackupDate: time.Now(),
		CurrentExe: currentExe,
	}

	dataDir, err := getDataDir()
	if err != nil {
		return err
	}

	metaPath := filepath.Join(dataDir, rollbackMetaFile)
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rollback info: %w", err)
	}

	return os.WriteFile(metaPath, data, 0644)
}

// GetRollbackInfo returns information about the available backup, if any.
func GetRollbackInfo() (*RollbackInfo, error) {
	dataDir, err := getDataDir()
	if err != nil {
		return nil, err
	}

	metaPath := filepath.Join(dataDir, rollbackMetaFile)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No rollback available
		}
		return nil, fmt.Errorf("failed to read rollback info: %w", err)
	}

	var info RollbackInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse rollback info: %w", err)
	}

	// Verify backup still exists
	if _, err := os.Stat(info.BackupPath); os.IsNotExist(err) {
		// Backup was deleted, clean up metadata
		os.Remove(metaPath)
		return nil, nil
	}

	return &info, nil
}

// CanRollback returns true if a rollback is possible.
func CanRollback() bool {
	info, err := GetRollbackInfo()
	return err == nil && info != nil
}

// Rollback restores the previous version from backup.
// Returns a result indicating success and whether restart is needed.
func Rollback() (*RollbackResult, error) {
	info, err := GetRollbackInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get rollback info: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("no rollback available")
	}

	// Verify paths
	if _, err := os.Stat(info.BackupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("backup file no longer exists: %s", info.BackupPath)
	}

	// Platform-specific rollback
	switch runtime.GOOS {
	case "windows":
		return rollbackWindows(info)
	default:
		return rollbackUnix(info)
	}
}

func rollbackWindows(info *RollbackInfo) (*RollbackResult, error) {
	// On Windows, we need to use a batch script similar to install
	tempScript := filepath.Join(os.TempDir(), "proxypilot-rollback.bat")

	script := fmt.Sprintf(`@echo off
echo Rolling back ProxyPilot to v%s...
timeout /t 2 /nobreak >nul
:retry
tasklist /FI "IMAGENAME eq %s" 2>NUL | find /I /N "%s">NUL
if "%%ERRORLEVEL%%"=="0" (
    timeout /t 1 /nobreak >nul
    goto retry
)
echo Restoring previous version...
del /f "%s"
copy /y "%s" "%s"
echo Starting restored version...
start "" "%s"
del "%%~f0"
`, info.Version,
		filepath.Base(info.CurrentExe), filepath.Base(info.CurrentExe),
		info.CurrentExe,
		info.BackupPath, info.CurrentExe,
		info.CurrentExe)

	if err := os.WriteFile(tempScript, []byte(script), 0755); err != nil {
		return nil, fmt.Errorf("failed to create rollback script: %w", err)
	}

	// Note: The caller should execute this script and exit
	return &RollbackResult{
		Success:         true,
		Message:         fmt.Sprintf("Rollback to v%s scheduled. Application will restart.", info.Version),
		RestoredVersion: info.Version,
		NeedsRestart:    true,
	}, nil
}

func rollbackUnix(info *RollbackInfo) (*RollbackResult, error) {
	// On Unix, we can swap files directly
	currentExe := info.CurrentExe
	backupPath := info.BackupPath

	// Create a temporary backup of the current (failed) version
	failedBackup := currentExe + ".failed"
	os.Remove(failedBackup)

	// Move current (failed) version to .failed
	if err := os.Rename(currentExe, failedBackup); err != nil {
		return nil, fmt.Errorf("failed to move current executable: %w", err)
	}

	// Move backup to current
	if err := os.Rename(backupPath, currentExe); err != nil {
		// Try to restore failed version
		os.Rename(failedBackup, currentExe)
		return nil, fmt.Errorf("failed to restore backup: %w", err)
	}

	// Clean up failed version (optional - keep for debugging)
	// os.Remove(failedBackup)

	// Clear rollback metadata
	dataDir, _ := getDataDir()
	os.Remove(filepath.Join(dataDir, rollbackMetaFile))

	return &RollbackResult{
		Success:         true,
		Message:         fmt.Sprintf("Rolled back to v%s. Please restart the application.", info.Version),
		RestoredVersion: info.Version,
		NeedsRestart:    true,
	}, nil
}

// RecordStartup should be called at application startup to track health.
// It returns a HealthCheckResult indicating if auto-rollback should occur.
func RecordStartup() (*HealthCheckResult, error) {
	dataDir, err := getDataDir()
	if err != nil {
		return nil, err
	}

	healthPath := filepath.Join(dataDir, healthCheckFile)
	currentVersion := buildinfo.Version

	var state healthState

	// Load existing health state
	if data, err := os.ReadFile(healthPath); err == nil {
		json.Unmarshal(data, &state)
	}

	now := time.Now()

	// If version changed, reset tracking
	if state.Version != currentVersion {
		state = healthState{
			Version:      currentVersion,
			StartupTimes: []time.Time{now},
		}
	} else {
		// Filter to only recent startups within the window
		recentStartups := make([]time.Time, 0)
		for _, t := range state.StartupTimes {
			if now.Sub(t) < startupWindow {
				recentStartups = append(recentStartups, t)
			}
		}
		recentStartups = append(recentStartups, now)
		state.StartupTimes = recentStartups
	}

	// Determine if we should rollback
	shouldRollback := len(state.StartupTimes) >= maxFailedStartups && CanRollback()

	// Save updated state
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(healthPath, data, 0644)

	return &HealthCheckResult{
		Healthy:        len(state.StartupTimes) < maxFailedStartups,
		StartupCount:   len(state.StartupTimes),
		LastStartup:    now,
		ShouldRollback: shouldRollback,
	}, nil
}

// MarkHealthy should be called once the application has started successfully
// and been running for a reasonable amount of time.
func MarkHealthy() error {
	dataDir, err := getDataDir()
	if err != nil {
		return err
	}

	healthPath := filepath.Join(dataDir, healthCheckFile)
	currentVersion := buildinfo.Version

	state := healthState{
		Version:      currentVersion,
		StartupTimes: []time.Time{},
		LastHealthy:  time.Now(),
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(healthPath, data, 0644)
}

// ClearRollbackInfo removes the rollback metadata.
// Call this after a successful update has been verified.
func ClearRollbackInfo() error {
	dataDir, err := getDataDir()
	if err != nil {
		return err
	}

	metaPath := filepath.Join(dataDir, rollbackMetaFile)
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// CleanupAfterSuccessfulUpdate should be called after confirming
// the new version is working correctly. It removes the backup file
// and clears rollback metadata.
func CleanupAfterSuccessfulUpdate() error {
	info, err := GetRollbackInfo()
	if err != nil {
		return err
	}
	if info == nil {
		return nil // Nothing to clean up
	}

	// Remove backup file
	if err := os.Remove(info.BackupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup: %w", err)
	}

	// Clear metadata
	return ClearRollbackInfo()
}
