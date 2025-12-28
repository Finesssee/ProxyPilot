//go:build windows

package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// InstallConfig holds the configuration for the installation process.
type InstallConfig struct {
	// InstallDir is the target installation directory.
	// Defaults to %LOCALAPPDATA%\ProxyPilot.
	InstallDir string

	// CreateDesktopShortcut indicates whether to create a Desktop shortcut.
	CreateDesktopShortcut bool

	// EnableAutostart indicates whether to register the app for autostart.
	EnableAutostart bool
}

// ProgressCallback is called during file copy operations to report progress.
type ProgressCallback func(progress int, status string)

// bundleFiles is the list of files to extract from the embedded bundle.
var bundleFiles = []struct {
	src  string // Path within embeddedBundle.
	dst  string // Destination filename.
	size int64  // Approximate size for progress calculation.
}{
	{"bundle/ProxyPilot.exe", "ProxyPilot.exe", 0},
	{"bundle/config.example.yaml", "config.example.yaml", 0},
	{"bundle/icon.ico", "icon.ico", 0},
	{"bundle/icon.png", "icon.png", 0},
}

// CopyFiles extracts the embedded bundle files to the installation directory.
func CopyFiles(config *InstallConfig, progress ProgressCallback) error {
	if config.InstallDir == "" {
		return fmt.Errorf("install directory not specified")
	}

	// Ensure the installation directory exists.
	if err := os.MkdirAll(config.InstallDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Calculate total number of files for progress.
	totalFiles := len(bundleFiles)
	copiedFiles := 0

	for _, file := range bundleFiles {
		dstPath := filepath.Join(config.InstallDir, file.dst)

		if progress != nil {
			progress((copiedFiles*100)/totalFiles, fmt.Sprintf("Copying %s...", file.dst))
		}

		if err := extractEmbeddedFile(file.src, dstPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", file.dst, err)
		}

		copiedFiles++
	}

	// Create config.yaml from config.example.yaml if it doesn't exist.
	configPath := filepath.Join(config.InstallDir, "config.yaml")
	examplePath := filepath.Join(config.InstallDir, "config.example.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if _, err := os.Stat(examplePath); err == nil {
			if err := copyFile(examplePath, configPath); err != nil {
				// Non-fatal: user can create config manually.
				_ = err
			} else if progress != nil {
				progress(95, "Created default configuration")
			}
		}
	}

	if progress != nil {
		progress(100, "All files copied successfully")
	}

	return nil
}

// extractEmbeddedFile extracts a file from the embedded bundle to the destination path.
func extractEmbeddedFile(src, dst string) error {
	// Open the embedded file.
	srcFile, err := embeddedBundle.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open embedded file %s: %w", src, err)
	}
	defer srcFile.Close()

	// Get file info for permissions.
	info, err := srcFile.(fs.File).Stat()
	if err != nil {
		return fmt.Errorf("failed to stat embedded file: %w", err)
	}

	// Create destination directory if needed.
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dstDir, err)
	}

	// Create the destination file.
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", dst, err)
	}
	defer dstFile.Close()

	// Copy the content.
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to write file %s: %w", dst, err)
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// Cleanup removes all installed files on failure.
func Cleanup(config *InstallConfig) error {
	if config.InstallDir == "" {
		return nil
	}

	// Remove autostart registration.
	_ = UnregisterAutostart(config)

	// Remove uninstall registration.
	_ = UnregisterUninstall()

	// Remove shortcuts.
	_ = RemoveShortcuts(config)

	// Remove the installation directory.
	// Only remove if it's under LocalAppData to prevent accidental deletion.
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" && filepath.HasPrefix(config.InstallDir, localAppData) {
		return os.RemoveAll(config.InstallDir)
	}

	return nil
}

// RemoveShortcuts removes all created shortcuts.
func RemoveShortcuts(config *InstallConfig) error {
	var lastErr error

	// Remove Start Menu shortcut.
	startMenuPath := getStartMenuPath()
	if startMenuPath != "" {
		shortcutPath := filepath.Join(startMenuPath, appName+".lnk")
		if err := os.Remove(shortcutPath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}

	// Remove Desktop shortcut.
	desktopPath := getDesktopPath()
	if desktopPath != "" {
		shortcutPath := filepath.Join(desktopPath, appName+".lnk")
		if err := os.Remove(shortcutPath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}

	return lastErr
}
