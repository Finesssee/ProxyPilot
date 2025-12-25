package integrations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type DroidIntegration struct{}

func (i *DroidIntegration) Meta() IntegrationStatus {
	return IntegrationStatus{
		ID:          "factory-droid",
		Name:        "Factory Droid",
		Description: "Factory.ai Droid CLI",
	}
}

func (i *DroidIntegration) Detect() (bool, error) {
	configDir := filepath.Join(userHomeDir(), ".factory")
	return dirExists(configDir), nil
}

func (i *DroidIntegration) IsConfigured(proxyURL string) (bool, error) {
	// Droid uses settings.json for runtime config, not config.json
	settingsPath := filepath.Join(userHomeDir(), ".factory", "settings.json")
	if !fileExists(settingsPath) {
		return false, nil
	}
	content, _ := os.ReadFile(settingsPath)
	// Check if any custom model uses our proxy URL (note: camelCase "customModels")
	res := gjson.GetBytes(content, "customModels.#.baseUrl")
	for _, match := range res.Array() {
		if strings.Contains(match.String(), proxyURL) {
			return true, nil
		}
	}
	return false, nil
}

func (i *DroidIntegration) Configure(proxyURL string) error {
	configDir := filepath.Join(userHomeDir(), ".factory")
	// Droid uses settings.json for custom models at runtime
	settingsPath := filepath.Join(configDir, "settings.json")

	var jsonStr string
	if fileExists(settingsPath) {
		b, _ := os.ReadFile(settingsPath)
		jsonStr = string(b)
	} else {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}
		jsonStr = "{}"
	}

	// Define models to add - must use thinking models for /v1/responses API
	models := []struct {
		model       string
		displayName string
	}{
		{"claude-opus-4-5-thinking", "ProxyPilot Opus 4.5 Thinking"},
		{"claude-sonnet-4-5-thinking", "ProxyPilot Sonnet 4.5 Thinking"},
	}

	for idx, m := range models {
		// Droid settings.json uses camelCase and requires id/index fields
		newEntry := map[string]interface{}{
			"model":          m.model,
			"id":             fmt.Sprintf("custom:ProxyPilot-%d", idx),
			"index":          idx,
			"baseUrl":        proxyURL + "/v1",
			"apiKey":         "local-dev-key",
			"displayName":    m.displayName,
			"noImageSupport": true,
			"provider":       "openai", // Uses OpenAI Responses API (/v1/responses)
		}

		// Check if model already exists
		existing := gjson.Get(jsonStr, "customModels")
		existingIdx := -1
		if existing.IsArray() {
			for i, entry := range existing.Array() {
				if entry.Get("model").String() == m.model {
					existingIdx = i
					break
				}
			}
		}

		var errSet error
		if existingIdx >= 0 {
			jsonStr, errSet = sjson.Set(jsonStr, fmt.Sprintf("customModels.%d", existingIdx), newEntry)
		} else {
			jsonStr, errSet = sjson.Set(jsonStr, "customModels.-1", newEntry)
		}
		if errSet != nil {
			return errSet
		}
	}

	// Set default model if not set
	if !gjson.Get(jsonStr, "sessionDefaultSettings.model").Exists() {
		jsonStr, _ = sjson.Set(jsonStr, "sessionDefaultSettings.model", "custom:ProxyPilot-1")
		jsonStr, _ = sjson.Set(jsonStr, "sessionDefaultSettings.reasoningEffort", "none")
	}

	return os.WriteFile(settingsPath, []byte(jsonStr), 0644)
}
