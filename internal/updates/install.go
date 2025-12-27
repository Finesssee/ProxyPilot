package updates

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallResult contains the result of an installation attempt.
type InstallResult struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	RestartCmd  string `json:"restart_cmd,omitempty"`
	NeedsRestart bool  `json:"needs_restart"`
}

// PrepareInstall extracts and prepares the update for installation.
// Returns the path to the extracted executable.
func PrepareInstall(downloadResult *DownloadResult) (string, error) {
	if downloadResult == nil || downloadResult.FilePath == "" {
		return "", fmt.Errorf("no download result provided")
	}

	filePath := downloadResult.FilePath
	ext := strings.ToLower(filepath.Ext(filePath))

	// Handle different archive formats
	switch ext {
	case ".zip":
		return extractZip(filePath)
	case ".gz":
		if strings.HasSuffix(strings.ToLower(filePath), ".tar.gz") {
			return extractTarGz(filePath)
		}
		return "", fmt.Errorf("unsupported archive format: %s", ext)
	case ".exe", ".msi":
		// Already an executable or installer
		return filePath, nil
	default:
		// Assume it's a raw binary (Linux/macOS)
		// Make it executable
		if err := os.Chmod(filePath, 0755); err != nil {
			return "", fmt.Errorf("failed to make binary executable: %w", err)
		}
		return filePath, nil
	}
}

// InstallUpdate installs the update by replacing the current binary.
// On Windows, this schedules a replacement for the next restart.
// On Unix, this replaces the binary directly.
func InstallUpdate(executablePath string) (*InstallResult, error) {
	currentExe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Resolve any symlinks
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return installWindows(executablePath, currentExe)
	default:
		return installUnix(executablePath, currentExe)
	}
}

func installWindows(newExe, currentExe string) (*InstallResult, error) {
	// On Windows, we can't replace a running executable directly.
	// We use a batch script that waits for the process to exit, then replaces the file.

	backupPath := currentExe + ".old"
	tempScript := filepath.Join(os.TempDir(), "proxypilot-update.bat")

	// Create update script
	script := fmt.Sprintf(`@echo off
echo Waiting for ProxyPilot to exit...
timeout /t 2 /nobreak >nul
:retry
tasklist /FI "IMAGENAME eq %s" 2>NUL | find /I /N "%s">NUL
if "%%ERRORLEVEL%%"=="0" (
    timeout /t 1 /nobreak >nul
    goto retry
)
echo Backing up old version...
if exist "%s" del /f "%s"
move /y "%s" "%s"
echo Installing new version...
copy /y "%s" "%s"
echo Starting new version...
start "" "%s"
del "%%~f0"
`, filepath.Base(currentExe), filepath.Base(currentExe),
		backupPath, backupPath,
		currentExe, backupPath,
		newExe, currentExe,
		currentExe)

	if err := os.WriteFile(tempScript, []byte(script), 0755); err != nil {
		return nil, fmt.Errorf("failed to create update script: %w", err)
	}

	// Start the update script
	cmd := exec.Command("cmd", "/c", "start", "/min", tempScript)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start update script: %w", err)
	}

	return &InstallResult{
		Success:      true,
		Message:      "Update scheduled. The application will restart shortly.",
		RestartCmd:   tempScript,
		NeedsRestart: true,
	}, nil
}

func installUnix(newExe, currentExe string) (*InstallResult, error) {
	// On Unix, we can replace the binary while it's running.
	// The new binary will be used on next execution.

	backupPath := currentExe + ".old"

	// Remove old backup if exists
	os.Remove(backupPath)

	// Rename current to backup
	if err := os.Rename(currentExe, backupPath); err != nil {
		return nil, fmt.Errorf("failed to backup current executable: %w", err)
	}

	// Copy new executable to current location
	if err := copyFile(newExe, currentExe); err != nil {
		// Try to restore backup
		os.Rename(backupPath, currentExe)
		return nil, fmt.Errorf("failed to install new executable: %w", err)
	}

	// Make executable
	if err := os.Chmod(currentExe, 0755); err != nil {
		return nil, fmt.Errorf("failed to set permissions: %w", err)
	}

	return &InstallResult{
		Success:      true,
		Message:      "Update installed successfully. Please restart the application.",
		NeedsRestart: true,
	}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func extractZip(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	extractDir := filepath.Dir(zipPath)
	var executablePath string

	for _, f := range r.File {
		fpath := filepath.Join(extractDir, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(extractDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("invalid file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return "", err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return "", err
		}

		// Look for the main executable
		if isExecutable(f.Name) {
			executablePath = fpath
		}
	}

	if executablePath == "" {
		return "", fmt.Errorf("no executable found in archive")
	}

	return executablePath, nil
}

func extractTarGz(tarGzPath string) (string, error) {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return "", fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	extractDir := filepath.Dir(tarGzPath)
	var executablePath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		fpath := filepath.Join(extractDir, header.Name)

		// Check for path traversal
		if !strings.HasPrefix(fpath, filepath.Clean(extractDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("invalid file path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
				return "", err
			}

			outFile, err := os.Create(fpath)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()

			if err := os.Chmod(fpath, os.FileMode(header.Mode)); err != nil {
				return "", err
			}

			if isExecutable(header.Name) {
				executablePath = fpath
			}
		}
	}

	if executablePath == "" {
		return "", fmt.Errorf("no executable found in archive")
	}

	return executablePath, nil
}

func isExecutable(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	// Look for common executable patterns
	if strings.HasSuffix(base, ".exe") {
		return true
	}
	if strings.Contains(base, "proxypilot") && !strings.Contains(base, ".") {
		return true
	}
	if base == "proxypilot" || base == "cliproxyapi" {
		return true
	}
	return false
}

// CleanupOldVersions removes backup files from previous updates.
func CleanupOldVersions() error {
	currentExe, err := os.Executable()
	if err != nil {
		return err
	}

	currentExe, _ = filepath.EvalSymlinks(currentExe)
	backupPath := currentExe + ".old"

	if _, err := os.Stat(backupPath); err == nil {
		return os.Remove(backupPath)
	}

	return nil
}
