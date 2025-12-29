package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CodexHandler handles Codex CLI configuration
type CodexHandler struct {
	configDir  string
	configPath string
}

// NewCodexHandler creates a new Codex CLI handler
func NewCodexHandler() *CodexHandler {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".codex")
	return &CodexHandler{
		configDir:  configDir,
		configPath: filepath.Join(configDir, "config.toml"),
	}
}

func (h *CodexHandler) ID() string {
	return "codex"
}

func (h *CodexHandler) Name() string {
	return "Codex CLI"
}

func (h *CodexHandler) CanAutoConfigure() bool {
	return true
}

func (h *CodexHandler) GetConfigPath() string {
	return h.configPath
}

// Detect checks if Codex CLI is installed
func (h *CodexHandler) Detect() (bool, error) {
	// Check for codex binary in PATH
	if _, err := exec.LookPath("codex"); err == nil {
		return true, nil
	}

	// Check for config directory existence
	if _, err := os.Stat(h.configDir); err == nil {
		return true, nil
	}

	return false, nil
}

// IsConfigured checks if Codex CLI is already configured for ProxyPilot
func (h *CodexHandler) IsConfigured(proxyURL string) (bool, error) {
	content, err := os.ReadFile(h.configPath)
	if err != nil {
		return false, nil // Config doesn't exist
	}

	// Check for ProxyPilot section marker
	if strings.Contains(string(content), "# ProxyPilot START") {
		return true, nil
	}

	// Check for proxy URL in config
	if strings.Contains(string(content), proxyURL) {
		return true, nil
	}

	return false, nil
}

// Enable configures Codex CLI to use ProxyPilot
func (h *CodexHandler) Enable(proxyURL string) ([]Change, error) {
	var changes []Change

	// Ensure config directory exists
	if err := os.MkdirAll(h.configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing config
	var existingContent string
	if data, err := os.ReadFile(h.configPath); err == nil {
		existingContent = string(data)
	}

	// Check if already has ProxyPilot section
	if strings.Contains(existingContent, "# ProxyPilot START") {
		// Already configured, update it
		existingContent = h.removeProxyPilotSection(existingContent)
	}

	// Record original content
	changes = append(changes, Change{
		Path:     "config.toml",
		Original: existingContent,
		Applied:  "", // Will be set below
		Type:     "toml",
		File:     h.configPath,
	})

	// Create ProxyPilot section
	proxyPilotSection := fmt.Sprintf(`
# ProxyPilot START
# ProxyPilot model provider for local proxy routing
[model_providers.proxypilot]
name = "ProxyPilot"
base_url = "%s/v1"
env_key = "PROXYPILOT_API_KEY"
wire_api = "openai"
# ProxyPilot END
`, proxyURL)

	// Append to existing content
	newContent := strings.TrimRight(existingContent, "\n\r\t ") + "\n" + proxyPilotSection

	// Update the change record
	changes[0].Applied = newContent

	// Write config
	if err := os.WriteFile(h.configPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return changes, nil
}

// Disable removes ProxyPilot configuration from Codex CLI
func (h *CodexHandler) Disable(changes []Change) error {
	content, err := os.ReadFile(h.configPath)
	if err != nil {
		return nil // Config doesn't exist, nothing to disable
	}

	// Remove ProxyPilot section
	newContent := h.removeProxyPilotSection(string(content))

	// If we have recorded original content, we could restore it
	// But for additive changes, just removing the section is cleaner
	for _, change := range changes {
		if change.Path == "config.toml" && change.Original != nil {
			if origStr, ok := change.Original.(string); ok && origStr != "" {
				// Restore original if it was recorded
				newContent = origStr
				break
			}
		}
	}

	// Write config
	if err := os.WriteFile(h.configPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// removeProxyPilotSection removes the ProxyPilot section from config content
func (h *CodexHandler) removeProxyPilotSection(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inProxyPilotSection := false

	for _, line := range lines {
		if strings.Contains(line, "# ProxyPilot START") {
			inProxyPilotSection = true
			continue
		}
		if strings.Contains(line, "# ProxyPilot END") {
			inProxyPilotSection = false
			continue
		}
		if !inProxyPilotSection {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// GetInstructions returns manual configuration instructions
func (h *CodexHandler) GetInstructions(proxyURL string) string {
	return fmt.Sprintf(`To manually configure Codex CLI for ProxyPilot:

1. Edit %s
2. Add the following section:

[model_providers.proxypilot]
name = "ProxyPilot"
base_url = "%s/v1"
env_key = "PROXYPILOT_API_KEY"
wire_api = "openai"

3. Set the environment variable:
   export PROXYPILOT_API_KEY="proxypilot-local"

4. Restart Codex CLI for changes to take effect.
`, h.configPath, proxyURL)
}
