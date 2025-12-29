package integrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	// Check for Claude Code config directory
	configPath := getClaudeCodeConfigPath()
	configDir := filepath.Dir(configPath)
	return dirExists(configDir), nil
}

func (i *ClaudeIntegration) IsConfigured(proxyURL string) (bool, error) {
	configPath := getClaudeCodeConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false, nil
	}

	// Check for ProxyPilot marker or ANTHROPIC_BASE_URL in env
	if _, hasMarker := config["_proxypilot_configured"]; hasMarker {
		return true, nil
	}

	if env, ok := config["env"].(map[string]interface{}); ok {
		if baseURL, ok := env["ANTHROPIC_BASE_URL"].(string); ok {
			if baseURL == proxyURL || baseURL == proxyURL+"/v1" {
				return true, nil
			}
		}
	}

	return false, nil
}

func (i *ClaudeIntegration) Configure(proxyURL string) error {
	return configureClaudeCodeSettings(proxyURL)
}

// getClaudeCodeConfigPath returns the path to Claude Code's settings.json
func getClaudeCodeConfigPath() string {
	home := userHomeDir()
	if runtime.GOOS == "windows" {
		// Windows: ~/.claude/settings.json
		return filepath.Join(home, ".claude", "settings.json")
	}
	// macOS/Linux: ~/.claude/settings.json
	return filepath.Join(home, ".claude", "settings.json")
}

// configureClaudeCodeSettings configures Claude Code via its native settings.json
func configureClaudeCodeSettings(proxyURL string) error {
	configPath := getClaudeCodeConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Read existing config or create new one
	var config map[string]interface{}
	if existingData, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(existingData, &config)
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// Get or create env section
	var env map[string]interface{}
	if existingEnv, ok := config["env"].(map[string]interface{}); ok {
		env = existingEnv
	} else {
		env = make(map[string]interface{})
	}

	// Set ProxyPilot environment variables
	env["ANTHROPIC_BASE_URL"] = proxyURL
	env["ANTHROPIC_AUTH_TOKEN"] = "proxypilot-local"

	config["env"] = env

	// Add ProxyPilot marker
	config["_proxypilot_configured"] = true

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// unconfigureClaudeCodeSettings removes ProxyPilot config from Claude Code settings.json
func unconfigureClaudeCodeSettings() error {
	configPath := getClaudeCodeConfigPath()

	// Read existing config
	existingData, err := os.ReadFile(configPath)
	if err != nil {
		return nil // No config to unconfigure
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingData, &config); err != nil {
		return nil // Invalid JSON, nothing to do
	}

	// Remove ProxyPilot env vars while preserving others
	if env, ok := config["env"].(map[string]interface{}); ok {
		delete(env, "ANTHROPIC_BASE_URL")
		delete(env, "ANTHROPIC_AUTH_TOKEN")
		if len(env) == 0 {
			delete(config, "env")
		}
	}

	// Remove ProxyPilot marker
	delete(config, "_proxypilot_configured")

	// Write back
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}
