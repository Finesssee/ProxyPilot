package integrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CodexIntegration struct{}

func (i *CodexIntegration) Meta() IntegrationStatus {
	return IntegrationStatus{
		ID:          "codex-cli",
		Name:        "Codex CLI",
		Description: "OpenAI Codex CLI tool",
	}
}

func (i *CodexIntegration) Detect() (bool, error) {
	// Check for binary in PATH
	_, err := exec.LookPath("codex")
	if err == nil {
		return true, nil
	}
	// Check for config dir
	configDir := filepath.Join(userHomeDir(), ".codex")
	return dirExists(configDir), nil
}

func (i *CodexIntegration) IsConfigured(proxyURL string) (bool, error) {
	configPath := filepath.Join(userHomeDir(), ".codex", "config.toml")
	if !fileExists(configPath) {
		return false, nil
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	// Naive check
	return strings.Contains(string(content), proxyURL), nil
}

func (i *CodexIntegration) Configure(proxyURL string) error {
	configDir := filepath.Join(userHomeDir(), ".codex")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	configPath := filepath.Join(configDir, "config.toml")

	// Create or append
	// This is a naive implementation; ideally we parse TOML
	var content string
	if fileExists(configPath) {
		b, _ := os.ReadFile(configPath)
		content = string(b)
	}

	// Update base_url
	// If exists, replace line? Or just append?
	// For simplicity, we'll append/overwrite safely if it's simple
	// But robust TOML editing is hard without a library.
	// Let's assume standard format: base_url = "..."

	newLine := fmt.Sprintf(`base_url = "%s/v1"`, proxyURL)

	if strings.Contains(content, "base_url =") {
		lines := strings.Split(content, "\n")
		for idx, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "base_url =") {
				lines[idx] = newLine
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		content += "\n" + newLine + "\n"
	}

	return os.WriteFile(configPath, []byte(content), 0644)
}
