package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GeminiCLIHandler handles Gemini CLI configuration
type GeminiCLIHandler struct {
	configDir  string
	configPath string
}

// NewGeminiCLIHandler creates a new Gemini CLI handler
func NewGeminiCLIHandler() *GeminiCLIHandler {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".gemini")
	return &GeminiCLIHandler{
		configDir:  configDir,
		configPath: filepath.Join(configDir, "settings.json"),
	}
}

func (h *GeminiCLIHandler) ID() string {
	return "gemini-cli"
}

func (h *GeminiCLIHandler) Name() string {
	return "Gemini CLI"
}

func (h *GeminiCLIHandler) CanAutoConfigure() bool {
	return true
}

func (h *GeminiCLIHandler) GetConfigPath() string {
	return h.configPath
}

// Detect checks if Gemini CLI is installed
func (h *GeminiCLIHandler) Detect() (bool, error) {
	// Check for gemini binary in PATH
	if _, err := exec.LookPath("gemini"); err == nil {
		return true, nil
	}

	// Check for config directory existence
	if _, err := os.Stat(h.configDir); err == nil {
		return true, nil
	}

	return false, nil
}

// IsConfigured checks if Gemini CLI is already configured for ProxyPilot
func (h *GeminiCLIHandler) IsConfigured(proxyURL string) (bool, error) {
	config, err := h.readConfig()
	if err != nil {
		return false, nil // Config doesn't exist
	}

	// Check for ProxyPilot marker
	if _, hasMarker := config["_proxypilot"]; hasMarker {
		return true, nil
	}

	// Check for api_base pointing to proxy
	if apiBase, ok := config["api_base"].(string); ok {
		if apiBase == proxyURL || apiBase == proxyURL+"/" {
			return true, nil
		}
	}

	return false, nil
}

// Enable configures Gemini CLI to use ProxyPilot
func (h *GeminiCLIHandler) Enable(proxyURL string) ([]Change, error) {
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

	// Record and apply api_base change
	originalAPIBase := config["api_base"]
	config["api_base"] = proxyURL
	changes = append(changes, Change{
		Path:     "api_base",
		Original: originalAPIBase,
		Applied:  proxyURL,
		Type:     "json",
	})

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

// Disable removes ProxyPilot configuration from Gemini CLI
func (h *GeminiCLIHandler) Disable(changes []Change) error {
	config, err := h.readConfig()
	if err != nil {
		return nil // Config doesn't exist, nothing to disable
	}

	// Restore original values based on recorded changes
	for _, change := range changes {
		switch change.Path {
		case "api_base":
			if change.Original == nil {
				delete(config, "api_base")
			} else {
				config["api_base"] = change.Original
			}
		case "_proxypilot":
			delete(config, "_proxypilot")
		}
	}

	// Write config
	if err := h.writeConfig(config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetInstructions returns manual configuration instructions
func (h *GeminiCLIHandler) GetInstructions(proxyURL string) string {
	return fmt.Sprintf(`To manually configure Gemini CLI for ProxyPilot:

Option 1: Edit settings.json
1. Edit %s
2. Add or update the "api_base" field:

{
  "api_base": "%s"
}

Option 2: Use environment variables
Add to your shell profile (~/.bashrc or ~/.zshrc):

export GOOGLE_GEMINI_BASE_URL="%s"
export GEMINI_API_KEY="proxypilot-local"

Then restart your shell or run: source ~/.bashrc
`, h.configPath, proxyURL, proxyURL)
}

// readConfig reads the Gemini CLI settings file
func (h *GeminiCLIHandler) readConfig() (map[string]any, error) {
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

// writeConfig writes the Gemini CLI settings file
func (h *GeminiCLIHandler) writeConfig(config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Add newline at end
	data = append(data, '\n')

	return os.WriteFile(h.configPath, data, 0644)
}

// GetShellConfig returns shell profile configuration for Gemini CLI
func (h *GeminiCLIHandler) GetShellConfig(proxyURL string) string {
	return fmt.Sprintf(`# ProxyPilot - Gemini CLI Configuration
# Add these lines to your shell profile (~/.bashrc, ~/.zshrc, or ~/.profile)

# Option 1: OAuth mode (local only)
export CODE_ASSIST_ENDPOINT="%s"

# Option 2: API Key mode (works with any IP/domain)
# export GOOGLE_GEMINI_BASE_URL="%s"
# export GEMINI_API_KEY="proxypilot-local"
`, proxyURL, proxyURL)
}
