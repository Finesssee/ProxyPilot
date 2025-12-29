package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// OpenCodeHandler handles OpenCode CLI configuration
type OpenCodeHandler struct {
	configDir  string
	configPath string
}

// NewOpenCodeHandler creates a new OpenCode handler
func NewOpenCodeHandler() *OpenCodeHandler {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "opencode")
	return &OpenCodeHandler{
		configDir:  configDir,
		configPath: filepath.Join(configDir, "opencode.json"),
	}
}

func (h *OpenCodeHandler) ID() string {
	return "opencode"
}

func (h *OpenCodeHandler) Name() string {
	return "OpenCode"
}

func (h *OpenCodeHandler) CanAutoConfigure() bool {
	return true
}

func (h *OpenCodeHandler) GetConfigPath() string {
	return h.configPath
}

// Detect checks if OpenCode is installed
func (h *OpenCodeHandler) Detect() (bool, error) {
	// Check for opencode binary in PATH
	if _, err := exec.LookPath("opencode"); err == nil {
		return true, nil
	}

	// Check for config directory existence
	if _, err := os.Stat(h.configDir); err == nil {
		return true, nil
	}

	return false, nil
}

// IsConfigured checks if OpenCode is already configured for ProxyPilot
func (h *OpenCodeHandler) IsConfigured(proxyURL string) (bool, error) {
	config, err := h.readConfig()
	if err != nil {
		return false, nil // Config doesn't exist
	}

	// Check for proxypilot provider
	if providers, ok := config["providers"].(map[string]any); ok {
		if _, hasProxyPilot := providers["proxypilot"]; hasProxyPilot {
			return true, nil
		}
	}

	return false, nil
}

// Enable configures OpenCode to use ProxyPilot
func (h *OpenCodeHandler) Enable(proxyURL string) ([]Change, error) {
	var changes []Change

	// Ensure config directory exists
	if err := os.MkdirAll(h.configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing config or create new one
	config, err := h.readConfig()
	if err != nil {
		config = make(map[string]any)
	}

	// Get or create providers map
	var providers map[string]any
	if existingProviders, ok := config["providers"].(map[string]any); ok {
		providers = existingProviders
	} else {
		providers = make(map[string]any)
	}

	// Record original proxypilot provider (if any)
	originalProvider := providers["proxypilot"]

	// Add ProxyPilot provider configuration
	providers["proxypilot"] = map[string]any{
		"name":   "ProxyPilot",
		"apiKey": "proxypilot-local",
		"options": map[string]any{
			"baseURL": proxyURL + "/v1",
		},
		"models": []string{
			"claude-sonnet-4-20250514",
			"claude-opus-4-20250514",
			"gpt-4o",
			"gemini-2.0-flash",
		},
	}

	changes = append(changes, Change{
		Path:     "providers.proxypilot",
		Original: originalProvider,
		Applied:  providers["proxypilot"],
		Type:     "json",
	})

	config["providers"] = providers

	// Write config
	if err := h.writeConfig(config); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return changes, nil
}

// Disable removes ProxyPilot configuration from OpenCode
func (h *OpenCodeHandler) Disable(changes []Change) error {
	config, err := h.readConfig()
	if err != nil {
		return nil // Config doesn't exist, nothing to disable
	}

	// Get providers map
	providers, ok := config["providers"].(map[string]any)
	if !ok {
		return nil // No providers, nothing to remove
	}

	// Restore original values based on recorded changes
	for _, change := range changes {
		if change.Path == "providers.proxypilot" {
			if change.Original == nil {
				delete(providers, "proxypilot")
			} else {
				providers["proxypilot"] = change.Original
			}
		}
	}

	// Clean up empty providers map
	if len(providers) == 0 {
		delete(config, "providers")
	} else {
		config["providers"] = providers
	}

	// Write config
	if err := h.writeConfig(config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetInstructions returns manual configuration instructions
func (h *OpenCodeHandler) GetInstructions(proxyURL string) string {
	return fmt.Sprintf(`To manually configure OpenCode for ProxyPilot:

1. Edit %s
2. Add a "proxypilot" provider to the "providers" section:

{
  "providers": {
    "proxypilot": {
      "name": "ProxyPilot",
      "apiKey": "proxypilot-local",
      "options": {
        "baseURL": "%s/v1"
      },
      "models": [
        "claude-sonnet-4-20250514",
        "claude-opus-4-20250514",
        "gpt-4o"
      ]
    }
  }
}

3. Restart OpenCode and select the ProxyPilot provider.
`, h.configPath, proxyURL)
}

// readConfig reads the OpenCode settings file
func (h *OpenCodeHandler) readConfig() (map[string]any, error) {
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

// writeConfig writes the OpenCode settings file
func (h *OpenCodeHandler) writeConfig(config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Add newline at end
	data = append(data, '\n')

	return os.WriteFile(h.configPath, data, 0644)
}
