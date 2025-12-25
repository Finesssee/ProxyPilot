package integrations

import (
	"fmt"
	"os"
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
	// Check for gemini command or config
	// Usually gemini-cli doesn't have a strict config file, relies on env vars
	// But we can check for installation via npm or pip if known, or just assume "not installed" if we can't find binary
	// For now, let's just check if we can write to the profile
	return true, nil // Always show available to configure env vars
}

func (i *GeminiIntegration) IsConfigured(proxyURL string) (bool, error) {
	profile := powerShellProfilePath()
	if profile == "" || !fileExists(profile) {
		return false, nil
	}
	content, _ := os.ReadFile(profile)
	return strings.Contains(string(content), "GEMINI_BASE_URL") && strings.Contains(string(content), proxyURL), nil
}

func (i *GeminiIntegration) Configure(proxyURL string) error {
	profile := powerShellProfilePath()
	if profile == "" {
		return fmt.Errorf("could not locate PowerShell profile")
	}

	dir := filepath.Dir(profile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// We set both API Key and Base URL
	envVars := []string{
		fmt.Sprintf(`$env:GEMINI_BASE_URL = "%s/v1beta/models"`, proxyURL),
		`$env:GOOGLE_API_KEY = "local-dev-key"`,
	}
	
	block := "\n# ProxyPilot: Gemini CLI Integration\n" + strings.Join(envVars, "\n") + "\n"

	var content string
	if fileExists(profile) {
		b, _ := os.ReadFile(profile)
		content = string(b)
	}

	// Simple check to avoid duplication
	if strings.Contains(content, "GEMINI_BASE_URL") {
		// Heuristic replace
		lines := strings.Split(content, "\n")
		for idx, line := range lines {
			if strings.Contains(line, "GEMINI_BASE_URL") {
				lines[idx] = envVars[0]
			}
			if strings.Contains(line, "GOOGLE_API_KEY") && strings.Contains(line, "local-dev-key") {
				lines[idx] = envVars[1]
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		content += block
	}

	return os.WriteFile(profile, []byte(content), 0644)
}
