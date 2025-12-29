package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// CursorHandler handles Cursor IDE configuration
type CursorHandler struct {
	configPath string
}

// NewCursorHandler creates a new Cursor handler
func NewCursorHandler() *CursorHandler {
	return &CursorHandler{
		configPath: getCursorSettingsPath(),
	}
}

func (h *CursorHandler) ID() string {
	return "cursor"
}

func (h *CursorHandler) Name() string {
	return "Cursor"
}

func (h *CursorHandler) CanAutoConfigure() bool {
	return true
}

func (h *CursorHandler) GetConfigPath() string {
	return h.configPath
}

// Detect checks if Cursor is installed
func (h *CursorHandler) Detect() (bool, error) {
	if h.configPath == "" {
		return false, nil
	}

	// Check if settings file or its directory exists
	if _, err := os.Stat(h.configPath); err == nil {
		return true, nil
	}
	if _, err := os.Stat(filepath.Dir(h.configPath)); err == nil {
		return true, nil
	}

	return false, nil
}

// IsConfigured checks if Cursor is already configured for ProxyPilot
func (h *CursorHandler) IsConfigured(proxyURL string) (bool, error) {
	config, err := h.readConfig()
	if err != nil {
		return false, nil // Config doesn't exist
	}

	// Check for proxypilot model in models section
	if models, ok := config["models"].(map[string]any); ok {
		if _, hasProxyPilot := models["proxypilot"]; hasProxyPilot {
			return true, nil
		}
	}

	return false, nil
}

// Enable configures Cursor to use ProxyPilot
func (h *CursorHandler) Enable(proxyURL string) ([]Change, error) {
	var changes []Change

	if h.configPath == "" {
		return nil, fmt.Errorf("could not determine Cursor settings path")
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing config or create new one
	config, err := h.readConfig()
	if err != nil {
		config = make(map[string]any)
	}

	// Get or create models map
	var models map[string]any
	if existingModels, ok := config["models"].(map[string]any); ok {
		models = existingModels
	} else {
		models = make(map[string]any)
	}

	// Record original proxypilot model (if any)
	originalModel := models["proxypilot"]

	// Add ProxyPilot model configuration
	models["proxypilot"] = map[string]any{
		"name":          "ProxyPilot",
		"apiKey":        "proxypilot-local",
		"baseUrl":       proxyURL + "/v1",
		"contextLength": 200000,
	}

	changes = append(changes, Change{
		Path:     "models.proxypilot",
		Original: originalModel,
		Applied:  models["proxypilot"],
		Type:     "json",
	})

	config["models"] = models

	// Write config
	if err := h.writeConfig(config); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return changes, nil
}

// Disable removes ProxyPilot configuration from Cursor
func (h *CursorHandler) Disable(changes []Change) error {
	config, err := h.readConfig()
	if err != nil {
		return nil // Config doesn't exist, nothing to disable
	}

	// Get models map
	models, ok := config["models"].(map[string]any)
	if !ok {
		return nil // No models, nothing to remove
	}

	// Restore original values based on recorded changes
	for _, change := range changes {
		if change.Path == "models.proxypilot" {
			if change.Original == nil {
				delete(models, "proxypilot")
			} else {
				models["proxypilot"] = change.Original
			}
		}
	}

	// Clean up empty models map
	if len(models) == 0 {
		delete(config, "models")
	} else {
		config["models"] = models
	}

	// Write config
	if err := h.writeConfig(config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetInstructions returns manual configuration instructions
func (h *CursorHandler) GetInstructions(proxyURL string) string {
	return fmt.Sprintf(`To manually configure Cursor for ProxyPilot:

1. Open Cursor Settings (Cmd/Ctrl + ,)
2. Go to "Models" section
3. Add a new custom model with these settings:
   - Name: ProxyPilot
   - API Key: proxypilot-local
   - Base URL: %s/v1
   - Context Length: 200000

Or edit %s directly and add:

{
  "models": {
    "proxypilot": {
      "name": "ProxyPilot",
      "apiKey": "proxypilot-local",
      "baseUrl": "%s/v1",
      "contextLength": 200000
    }
  }
}

4. Select 'proxypilot' as your model in Cursor AI settings.
`, proxyURL, h.configPath, proxyURL)
}

// readConfig reads the Cursor settings file
func (h *CursorHandler) readConfig() (map[string]any, error) {
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

// writeConfig writes the Cursor settings file
func (h *CursorHandler) writeConfig(config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Add newline at end
	data = append(data, '\n')

	return os.WriteFile(h.configPath, data, 0644)
}

// getCursorSettingsPath returns the Cursor settings path based on platform
func getCursorSettingsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	var paths []string

	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "Cursor", "User", "settings.json"))
		}
	case "darwin":
		paths = append(paths, filepath.Join(homeDir, "Library", "Application Support", "Cursor", "User", "settings.json"))
	default: // Linux and others
		paths = append(paths, filepath.Join(homeDir, ".config", "Cursor", "User", "settings.json"))
	}

	// Return first existing path, or first path as default
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// If parent dir exists, use this path
		if _, err := os.Stat(filepath.Dir(p)); err == nil {
			return p
		}
	}

	if len(paths) > 0 {
		return paths[0]
	}

	return ""
}
