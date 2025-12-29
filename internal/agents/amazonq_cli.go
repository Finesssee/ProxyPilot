package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// AmazonQCLIHandler handles Amazon Q CLI detection and configuration.
// Amazon Q CLI is only available on Linux/macOS or via WSL on Windows.
// It stores tokens in a SQLite database at ~/.local/share/amazon-q/data.sqlite3
type AmazonQCLIHandler struct {
	// wslDistro is the WSL distribution name (Windows only)
	wslDistro string
	// wslUser is the WSL username (Windows only)
	wslUser string
}

// NewAmazonQCLIHandler creates a new Amazon Q CLI handler
func NewAmazonQCLIHandler() *AmazonQCLIHandler {
	return &AmazonQCLIHandler{}
}

func (h *AmazonQCLIHandler) ID() string {
	return "amazonq-cli"
}

func (h *AmazonQCLIHandler) Name() string {
	return "Amazon Q CLI"
}

func (h *AmazonQCLIHandler) CanAutoConfigure() bool {
	// Amazon Q CLI uses its own auth, no configuration needed from our side
	return false
}

func (h *AmazonQCLIHandler) GetConfigPath() string {
	if runtime.GOOS == "windows" {
		// Return WSL path for display purposes
		return "\\\\wsl$\\<distro>\\home\\<user>\\.local\\share\\amazon-q\\data.sqlite3"
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "share", "amazon-q", "data.sqlite3")
}

// Detect checks if Amazon Q CLI is installed
func (h *AmazonQCLIHandler) Detect() (bool, error) {
	if runtime.GOOS == "windows" {
		return h.detectInWSL()
	}
	return h.detectNative()
}

// detectNative checks for q CLI on Linux/macOS
func (h *AmazonQCLIHandler) detectNative() (bool, error) {
	// Check for 'q' binary in PATH
	if _, err := exec.LookPath("q"); err == nil {
		return true, nil
	}

	// Check common installation locations
	homeDir, _ := os.UserHomeDir()
	locations := []string{
		filepath.Join(homeDir, ".local", "bin", "q"),
		"/usr/local/bin/q",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return true, nil
		}
	}

	// Check for data directory (indicates Q CLI was installed and used)
	dataDir := filepath.Join(homeDir, ".local", "share", "amazon-q")
	if _, err := os.Stat(dataDir); err == nil {
		return true, nil
	}

	return false, nil
}

// detectInWSL checks for q CLI within WSL
func (h *AmazonQCLIHandler) detectInWSL() (bool, error) {
	// Check if WSL is available
	if _, err := exec.LookPath("wsl"); err != nil {
		return false, nil
	}

	// Get default WSL distro
	distro, err := h.getDefaultWSLDistro()
	if err != nil || distro == "" {
		return false, nil
	}
	h.wslDistro = distro

	// Get WSL username
	user, err := h.getWSLUsername(distro)
	if err != nil || user == "" {
		return false, nil
	}
	h.wslUser = user

	// Check for q CLI in WSL
	cmd := exec.Command("wsl", "-d", distro, "--", "which", "q")
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Check for common install locations in WSL
	checkCmd := exec.Command("wsl", "-d", distro, "--", "test", "-f", fmt.Sprintf("/home/%s/.local/bin/q", user))
	if err := checkCmd.Run(); err == nil {
		return true, nil
	}

	// Check for data directory in WSL
	dataCmd := exec.Command("wsl", "-d", distro, "--", "test", "-d", fmt.Sprintf("/home/%s/.local/share/amazon-q", user))
	if err := dataCmd.Run(); err == nil {
		return true, nil
	}

	return false, nil
}

// getDefaultWSLDistro returns the default WSL distribution name
func (h *AmazonQCLIHandler) getDefaultWSLDistro() (string, error) {
	// First try to get default distro
	cmd := exec.Command("wsl", "-l", "-q")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse output - first line is usually the default distro
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		// Clean up the line (remove null bytes from UTF-16 encoding on Windows)
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, "\x00", "")
		if line != "" && line != "docker-desktop" && line != "docker-desktop-data" {
			return line, nil
		}
	}

	return "", fmt.Errorf("no WSL distribution found")
}

// getWSLUsername returns the default username in the WSL distribution
func (h *AmazonQCLIHandler) getWSLUsername(distro string) (string, error) {
	cmd := exec.Command("wsl", "-d", distro, "--", "whoami")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// IsConfigured checks if Amazon Q CLI is authenticated
func (h *AmazonQCLIHandler) IsConfigured(proxyURL string) (bool, error) {
	// Check if the SQLite database exists with auth tokens
	dbPath, err := h.GetDatabasePath()
	if err != nil {
		return false, nil
	}

	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return false, nil
	}

	// Database exists, assume configured (actual token validation happens in auth layer)
	return true, nil
}

// GetDatabasePath returns the path to the Amazon Q SQLite database
func (h *AmazonQCLIHandler) GetDatabasePath() (string, error) {
	if runtime.GOOS == "windows" {
		if h.wslDistro == "" || h.wslUser == "" {
			// Try to detect
			distro, err := h.getDefaultWSLDistro()
			if err != nil {
				return "", fmt.Errorf("no WSL distribution found: %w", err)
			}
			h.wslDistro = distro

			user, err := h.getWSLUsername(distro)
			if err != nil {
				return "", fmt.Errorf("failed to get WSL user: %w", err)
			}
			h.wslUser = user
		}
		// Windows UNC path to WSL filesystem
		return fmt.Sprintf(`\\wsl$\%s\home\%s\.local\share\amazon-q\data.sqlite3`, h.wslDistro, h.wslUser), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".local", "share", "amazon-q", "data.sqlite3"), nil
}

// Enable is not supported for Amazon Q CLI (uses its own auth)
func (h *AmazonQCLIHandler) Enable(proxyURL string) ([]Change, error) {
	return nil, fmt.Errorf("Amazon Q CLI uses its own authentication. Run 'q login' to authenticate")
}

// Disable is not supported for Amazon Q CLI
func (h *AmazonQCLIHandler) Disable(changes []Change) error {
	// Nothing to disable - Q CLI manages its own auth
	return nil
}

// GetInstructions returns instructions for Amazon Q CLI setup
func (h *AmazonQCLIHandler) GetInstructions(proxyURL string) string {
	if runtime.GOOS == "windows" {
		return `Amazon Q CLI is available via WSL on Windows.

To install and authenticate:

1. Open WSL (Ubuntu recommended)
2. Install Amazon Q CLI:
   curl -sS https://desktop-release.q.us-east-1.amazonaws.com/latest/q-x86_64-linux.zip -o q.zip
   unzip q.zip
   ./q/install.sh --no-confirm

3. Authenticate with AWS Builder ID:
   q login

4. ProxyPilot will automatically detect and use your Amazon Q CLI tokens.

Note: Amazon Q CLI has separate usage quota from Kiro IDE.
`
	}

	return `To install and authenticate Amazon Q CLI:

1. Install Amazon Q CLI:
   curl -sS https://desktop-release.q.us-east-1.amazonaws.com/latest/q-x86_64-linux.zip -o q.zip
   unzip q.zip
   ./q/install.sh --no-confirm

2. Authenticate with AWS Builder ID:
   q login

3. ProxyPilot will automatically detect and use your Amazon Q CLI tokens.

Note: Amazon Q CLI has separate usage quota from Kiro IDE.
`
}

// GetWSLInfo returns WSL distro and user information (for external use)
func (h *AmazonQCLIHandler) GetWSLInfo() (distro, user string) {
	return h.wslDistro, h.wslUser
}
