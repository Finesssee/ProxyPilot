package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FactoryDroidHandler handles Factory Droid CLI configuration
type FactoryDroidHandler struct {
	configDir    string
	settingsPath string // Droid uses settings.json for customModels
}

// NewFactoryDroidHandler creates a new Factory Droid handler
func NewFactoryDroidHandler() *FactoryDroidHandler {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".factory")
	return &FactoryDroidHandler{
		configDir:    configDir,
		settingsPath: filepath.Join(configDir, "settings.json"),
	}
}

func (h *FactoryDroidHandler) ID() string {
	return "factory-droid"
}

func (h *FactoryDroidHandler) Name() string {
	return "Factory Droid"
}

func (h *FactoryDroidHandler) CanAutoConfigure() bool {
	return true
}

func (h *FactoryDroidHandler) GetConfigPath() string {
	return h.settingsPath
}

// Detect checks if Factory Droid is installed
func (h *FactoryDroidHandler) Detect() (bool, error) {
	if _, err := exec.LookPath("droid"); err == nil {
		return true, nil
	}
	if _, err := exec.LookPath("factory"); err == nil {
		return true, nil
	}
	if _, err := os.Stat(h.configDir); err == nil {
		return true, nil
	}
	return false, nil
}

// IsConfigured checks if Factory Droid is already configured for ProxyPilot
func (h *FactoryDroidHandler) IsConfigured(proxyURL string) (bool, error) {
	settings, err := h.readSettings()
	if err != nil {
		return false, nil
	}

	if customModels, ok := settings["customModels"].([]any); ok {
		for _, entry := range customModels {
			if m, ok := entry.(map[string]any); ok {
				displayName, _ := m["displayName"].(string)
				if strings.HasPrefix(displayName, "proxypilot-") {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// Enable configures Factory Droid to use ProxyPilot
func (h *FactoryDroidHandler) Enable(proxyURL string) ([]Change, error) {
	var changes []Change

	if err := os.MkdirAll(h.configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	settings, err := h.readSettings()
	if err != nil {
		settings = make(map[string]any)
	}

	// Build ProxyPilot models in Droid's format
	proxyPilotModels := buildDroidModels(proxyURL)

	originalCustomModels := settings["customModels"]

	// Merge: proxypilot models first, then user's non-proxypilot models
	var finalModels []any
	for _, m := range proxyPilotModels {
		finalModels = append(finalModels, m)
	}

	// Keep user's non-proxypilot models
	if existing, ok := settings["customModels"].([]any); ok {
		for _, entry := range existing {
			if m, ok := entry.(map[string]any); ok {
				displayName, _ := m["displayName"].(string)
				if !isProxyPilotModel(displayName) {
					// Re-index user models after proxypilot models
					m["index"] = len(finalModels)
					finalModels = append(finalModels, m)
				}
			}
		}
	}

	settings["customModels"] = finalModels

	changes = append(changes, Change{
		Path:     "customModels",
		Original: originalCustomModels,
		Applied:  finalModels,
		Type:     "json",
	})

	if err := h.writeSettings(settings); err != nil {
		return nil, fmt.Errorf("failed to write settings: %w", err)
	}

	return changes, nil
}

// Disable removes ProxyPilot configuration from Factory Droid
func (h *FactoryDroidHandler) Disable(changes []Change) error {
	settings, err := h.readSettings()
	if err != nil {
		return nil
	}

	var filteredModels []any
	idx := 0
	if customModels, ok := settings["customModels"].([]any); ok {
		for _, entry := range customModels {
			if m, ok := entry.(map[string]any); ok {
				displayName, _ := m["displayName"].(string)
				// Remove all ProxyPilot models (various naming conventions)
				if !isProxyPilotModel(displayName) {
					m["index"] = idx
					filteredModels = append(filteredModels, m)
					idx++
				}
			}
		}
	}

	if len(filteredModels) == 0 {
		delete(settings, "customModels")
	} else {
		settings["customModels"] = filteredModels
	}

	if err := h.writeSettings(settings); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// isProxyPilotModel checks if a model name belongs to ProxyPilot
func isProxyPilotModel(displayName string) bool {
	lower := strings.ToLower(displayName)
	return strings.HasPrefix(lower, "proxypilot-") || strings.HasPrefix(lower, "proxypilot ")
}

// GetInstructions returns manual configuration instructions
func (h *FactoryDroidHandler) GetInstructions(proxyURL string) string {
	return fmt.Sprintf(`To manually configure Factory Droid for ProxyPilot:

1. Edit %s
2. Add ProxyPilot models to the "customModels" array with this format:

{
  "customModels": [
    {
      "model": "claude-opus-4-5-20251101",
      "id": "custom:proxypilot-Claude-4.5-Opus-0",
      "index": 0,
      "baseUrl": "%s/v1",
      "apiKey": "proxypilot-local",
      "displayName": "proxypilot-Claude 4.5 Opus",
      "provider": "openai"
    }
  ]
}

3. Restart Factory Droid and select a proxypilot model.
`, h.settingsPath, proxyURL)
}

// GetShellConfig returns shell profile configuration for Factory Droid
func (h *FactoryDroidHandler) GetShellConfig(proxyURL string) string {
	return fmt.Sprintf(`# ProxyPilot - Factory Droid Configuration
# Factory Droid uses settings.json for configuration, not environment variables.
# Use the auto-configure feature or edit %s directly.

# Optional: For other OpenAI-compatible tools
export OPENAI_BASE_URL="%s/v1"
export OPENAI_API_KEY="proxypilot-local"
`, h.settingsPath, proxyURL)
}

func (h *FactoryDroidHandler) readSettings() (map[string]any, error) {
	data, err := os.ReadFile(h.settingsPath)
	if err != nil {
		return nil, err
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func (h *FactoryDroidHandler) writeSettings(settings map[string]any) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(h.settingsPath, data, 0644)
}

// buildDroidModels builds models in Droid's expected format
func buildDroidModels(proxyURL string) []map[string]any {
	curatedModels := []struct {
		displayName string
		modelID     string
		provider    string
	}{
		// Claude 4.5 Direct (Anthropic API)
		{"Claude 4.5 Opus", "claude-opus-4-5-20251101", "anthropic"},
		{"Claude 4.5 Sonnet", "claude-sonnet-4-5-20250929", "anthropic"},
		{"Claude 4.5 Haiku", "claude-haiku-4-5-20251001", "anthropic"},

		// Antigravity (Claude via Gemini with extended thinking)
		{"Claude 4.5 Opus Thinking", "gemini-claude-opus-4-5-thinking", "antigravity"},
		{"Claude 4.5 Sonnet Thinking", "gemini-claude-sonnet-4-5-thinking", "antigravity"},

		// Gemini latest
		{"Gemini 2.5 Pro", "gemini-2.5-pro", "google"},
		{"Gemini 2.5 Flash", "gemini-2.5-flash", "google"},
		{"Gemini 3 Pro Preview", "gemini-3-pro-preview", "google"},
		{"Gemini 3 Flash Preview", "gemini-3-flash-preview", "google"},

		// OpenAI latest
		{"GPT 5.2", "gpt-5.2", "openai"},
		{"GPT 5.2 Codex", "gpt-5.2-codex", "openai"},
		{"GPT 5.1 Codex Max", "gpt-5.1-codex-max", "openai"},

		// Kiro (AWS)
		{"Kiro Claude Opus 4.5", "kiro-claude-opus-4-5", "kiro"},
		{"Kiro Claude Sonnet 4.5", "kiro-claude-sonnet-4-5", "kiro"},
		{"Kiro Claude Haiku 4.5", "kiro-claude-haiku-4-5", "kiro"},

		// Amazon Q
		{"Amazon Q Auto", "amazonq-auto", "amazonq"},
		{"Amazon Q Claude Opus 4.5", "amazonq-claude-opus-4.5", "amazonq"},
		{"Amazon Q Claude Sonnet 4.5", "amazonq-claude-sonnet-4.5", "amazonq"},

		// GitHub Copilot
		{"Copilot GPT 5.2", "gpt-5.2", "github-copilot"},
		{"Copilot Claude Opus 4.5", "claude-opus-4.5", "github-copilot"},
		{"Copilot Claude Sonnet 4.5", "claude-sonnet-4.5", "github-copilot"},
		{"Copilot Gemini 3 Pro", "gemini-3-pro", "github-copilot"},

		// Qwen
		{"Qwen3 Coder Plus", "qwen3-coder-plus", "qwen"},
		{"Qwen3 Coder Flash", "qwen3-coder-flash", "qwen"},

		// MiniMax
		{"MiniMax M2.1", "minimax-m2.1", "minimax"},

		// Zhipu
		{"GLM 4.7", "glm-4.7", "zhipu"},
		{"GLM 4.6V", "glm-4.6v", "zhipu"},
	}

	var models []map[string]any
	for i, m := range curatedModels {
		displayName := "proxypilot-" + m.displayName
		// Create ID in Droid's format: custom:<name>-<index>
		id := fmt.Sprintf("custom:%s-%d", strings.ReplaceAll(displayName, " ", "-"), i)

		models = append(models, map[string]any{
			"model":          m.modelID,
			"id":             id,
			"index":          i,
			"baseUrl":        proxyURL + "/v1",
			"apiKey":         "proxypilot-local",
			"displayName":    displayName,
			"noImageSupport": false,
			"provider":       "openai", // Droid uses "openai" for OpenAI-compatible endpoints
		})
	}

	return models
}
