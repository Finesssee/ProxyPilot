package integrations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ClaudeIntegration struct{}

func (i *ClaudeIntegration) Meta() IntegrationStatus {
	return IntegrationStatus{
		ID:          "claude-code",
		Name:        "Claude Code",
		Description: "Anthropic Claude Code CLI",
	}
}

func (i *ClaudeIntegration) Detect() (bool, error) {
	// Check for npm global install or binary
	// On windows it's often in AppData/Roaming/npm/claude.cmd
	// But let's check for config folder as a proxy for installation
	configDir := filepath.Join(userHomeDir(), ".claude-code")
	return dirExists(configDir), nil
}

func (i *ClaudeIntegration) IsConfigured(proxyURL string) (bool, error) {
	// Check PowerShell profile for ANTHROPIC_BASE_URL
	profile := powerShellProfilePath()
	if profile == "" || !fileExists(profile) {
		return false, nil
	}
	content, _ := os.ReadFile(profile)
	return strings.Contains(string(content), "ANTHROPIC_BASE_URL") && strings.Contains(string(content), proxyURL), nil
}

func (i *ClaudeIntegration) Configure(proxyURL string) error {
	profile := powerShellProfilePath()
	if profile == "" {
		return fmt.Errorf("could not locate PowerShell profile")
	}

	dir := filepath.Dir(profile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	envVar := fmt.Sprintf(`$env:ANTHROPIC_BASE_URL = "%s/v1"`, proxyURL)
	
	var content string
	if fileExists(profile) {
		b, _ := os.ReadFile(profile)
		content = string(b)
	}

	if strings.Contains(content, "ANTHROPIC_BASE_URL") {
		// Replace existing
		lines := strings.Split(content, "\n")
		for idx, line := range lines {
			if strings.Contains(line, "ANTHROPIC_BASE_URL") {
				lines[idx] = envVar
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		content += "\n# ProxyPilot: Claude Code Integration\n" + envVar + "\n"
	}

	return os.WriteFile(profile, []byte(content), 0644)
}

func powerShellProfilePath() string {
	// Documents\PowerShell\Microsoft.PowerShell_profile.ps1
	home := userHomeDir()
	return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
}
