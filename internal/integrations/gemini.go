package integrations

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GeminiIntegration struct{}

func (i *GeminiIntegration) Meta() IntegrationStatus {
	return IntegrationStatus{
		ID:          "gemini-cli",
		Name:        "Gemini CLI",
		Description: "Google Gemini CLI tool",
	}
}

func (i *GeminiIntegration) Detect() (bool, error) {
	// Check for gemini CLI binary
	if _, err := exec.LookPath("gemini"); err == nil {
		return true, nil
	}
	// Check for ~/.gemini directory (config folder)
	home := userHomeDir()
	geminiDir := filepath.Join(home, ".gemini")
	if dirExists(geminiDir) {
		return true, nil
	}
	return false, nil
}

func (i *GeminiIntegration) IsConfigured(proxyURL string) (bool, error) {
	// Check shell profile for ProxyPilot Gemini markers
	profile := getShellProfilePath()
	if profile == "" || !fileExists(profile) {
		return false, nil
	}
	content, _ := os.ReadFile(profile)
	contentStr := string(content)

	// Check for ProxyPilot markers with the proxy URL
	return strings.Contains(contentStr, "# ProxyPilot START - Gemini") &&
		strings.Contains(contentStr, proxyURL), nil
}

func (i *GeminiIntegration) Configure(proxyURL string) error {
	// Delegate to the main configureGeminiCLI in agents.go
	return configureGeminiCLIShellProfile(proxyURL)
}

// getShellProfilePath returns the appropriate shell profile path
func getShellProfilePath() string {
	return getShellProfile()
}
