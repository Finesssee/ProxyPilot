package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ClaudeCodeHandler handles Claude Code CLI configuration
type ClaudeCodeHandler struct {
	configPath string
}

// NewClaudeCodeHandler creates a new Claude Code handler
func NewClaudeCodeHandler() *ClaudeCodeHandler {
	homeDir, _ := os.UserHomeDir()
	return &ClaudeCodeHandler{
		configPath: filepath.Join(homeDir, ".claude", "settings.json"),
	}
}

func (h *ClaudeCodeHandler) ID() string {
	return "claude-code"
}

func (h *ClaudeCodeHandler) Name() string {
	return "Claude Code"
}

func (h *ClaudeCodeHandler) CanAutoConfigure() bool {
	return true
}

func (h *ClaudeCodeHandler) GetConfigPath() string {
	return h.configPath
}

// Detect checks if Claude Code is installed
func (h *ClaudeCodeHandler) Detect() (bool, error) {
	// Check for claude binary in PATH
	if _, err := exec.LookPath("claude"); err == nil {
		return true, nil
	}

	// Check for config directory existence
	configDir := filepath.Dir(h.configPath)
	if _, err := os.Stat(configDir); err == nil {
		return true, nil
	}

	return false, nil
}

// IsConfigured checks if Claude Code is already configured for ProxyPilot
func (h *ClaudeCodeHandler) IsConfigured(proxyURL string) (bool, error) {
	config, err := h.readConfig()
	if err != nil {
		return false, nil // Config doesn't exist or can't be read
	}

	// Check for ProxyPilot marker or ANTHROPIC_BASE_URL
	if _, hasMarker := config["_proxypilot"]; hasMarker {
		return true, nil
	}

	if env, ok := config["env"].(map[string]any); ok {
		if baseURL, ok := env["ANTHROPIC_BASE_URL"].(string); ok {
			if baseURL == proxyURL || baseURL == proxyURL+"/" {
				return true, nil
			}
		}
	}

	return false, nil
}

// Enable configures Claude Code to use ProxyPilot
func (h *ClaudeCodeHandler) Enable(proxyURL string) ([]Change, error) {
	var changes []Change

	// Ensure config directory exists
	configDir := filepath.Dir(h.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing config or create new one
	config, err := h.readConfig()
	if err != nil {
		config = make(map[string]any)
	}

	// Get or create env map
	env, ok := config["env"].(map[string]any)
	if !ok {
		env = make(map[string]any)
	}

	// Record and apply ANTHROPIC_BASE_URL change
	originalBaseURL := env["ANTHROPIC_BASE_URL"]
	env["ANTHROPIC_BASE_URL"] = proxyURL
	changes = append(changes, Change{
		Path:     "env.ANTHROPIC_BASE_URL",
		Original: originalBaseURL,
		Applied:  proxyURL,
		Type:     "json",
	})

	// Record and apply ANTHROPIC_AUTH_TOKEN change
	originalAuthToken := env["ANTHROPIC_AUTH_TOKEN"]
	env["ANTHROPIC_AUTH_TOKEN"] = "proxypilot-local"
	changes = append(changes, Change{
		Path:     "env.ANTHROPIC_AUTH_TOKEN",
		Original: originalAuthToken,
		Applied:  "proxypilot-local",
		Type:     "json",
	})

	config["env"] = env

	// Add ProxyPilot marker
	originalMarker := config["_proxypilot"]
	config["_proxypilot"] = true
	changes = append(changes, Change{
		Path:     "_proxypilot",
		Original: originalMarker,
		Applied:  true,
		Type:     "json",
	})

	// Write config
	if err := h.writeConfig(config); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return changes, nil
}

// Disable removes ProxyPilot configuration from Claude Code
func (h *ClaudeCodeHandler) Disable(changes []Change) error {
	config, err := h.readConfig()
	if err != nil {
		return nil // Config doesn't exist, nothing to disable
	}

	env, _ := config["env"].(map[string]any)
	if env == nil {
		env = make(map[string]any)
	}

	// Restore original values based on recorded changes
	for _, change := range changes {
		switch change.Path {
		case "env.ANTHROPIC_BASE_URL":
			if change.Original == nil {
				delete(env, "ANTHROPIC_BASE_URL")
			} else {
				env["ANTHROPIC_BASE_URL"] = change.Original
			}
		case "env.ANTHROPIC_AUTH_TOKEN":
			if change.Original == nil {
				delete(env, "ANTHROPIC_AUTH_TOKEN")
			} else {
				env["ANTHROPIC_AUTH_TOKEN"] = change.Original
			}
		case "_proxypilot":
			delete(config, "_proxypilot")
		}
	}

	// Clean up empty env map
	if len(env) == 0 {
		delete(config, "env")
	} else {
		config["env"] = env
	}

	// Write config
	if err := h.writeConfig(config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetInstructions returns manual configuration instructions
func (h *ClaudeCodeHandler) GetInstructions(proxyURL string) string {
	return fmt.Sprintf(`To manually configure Claude Code for ProxyPilot:

1. Edit %s
2. Add or update the "env" section:

{
  "env": {
    "ANTHROPIC_BASE_URL": "%s",
    "ANTHROPIC_AUTH_TOKEN": "proxypilot-local"
  }
}

3. Restart Claude Code for changes to take effect.
`, h.configPath, proxyURL)
}

// readConfig reads the Claude Code settings file
func (h *ClaudeCodeHandler) readConfig() (map[string]any, error) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return nil, err
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// writeConfig writes the Claude Code settings file
func (h *ClaudeCodeHandler) writeConfig(config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Add newline at end
	data = append(data, '\n')

	return os.WriteFile(h.configPath, data, 0644)
}

// GetClaudeConfigDir returns the Claude Code config directory based on platform
func GetClaudeConfigDir() string {
	homeDir, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		// On Windows, Claude might use AppData
		if appData := os.Getenv("APPDATA"); appData != "" {
			claudeAppData := filepath.Join(appData, "Claude")
			if _, err := os.Stat(claudeAppData); err == nil {
				return claudeAppData
			}
		}
	}

	// Default: ~/.claude
	return filepath.Join(homeDir, ".claude")
}
