package integrations

import (
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
	configPath := filepath.Join(userHomeDir(), ".factory", "config.json")
	if !fileExists(configPath) {
		return false, nil
	}
	content, _ := os.ReadFile(configPath)
	// Check if any custom model uses our proxy URL
	res := gjson.GetBytes(content, "custom_models.#.base_url")
	for _, match := range res.Array() {
		if strings.Contains(match.String(), proxyURL) {
			return true, nil
		}
	}
	return false, nil
}

func (i *DroidIntegration) Configure(proxyURL string) error {
	configDir := filepath.Join(userHomeDir(), ".factory")
	configPath := filepath.Join(configDir, "config.json")
	
	var jsonStr string
	if fileExists(configPath) {
		b, _ := os.ReadFile(configPath)
		jsonStr = string(b)
	} else {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}
		jsonStr = "{}"
	}

	// Define models to add
	models := []string{
		"gpt-5.2",
		"claude-opus-4-5-thinking",
		"gemini-3-pro-preview",
	}

	for _, m := range models {
		// Replicate setup-droid-cliproxy.ps1 logic
		newEntry := map[string]interface{}{
			"model_display_name": "ProxyPilot (local): " + m,
			"model":              m,
			"base_url":           proxyURL + "/v1",
			"api_key":            "local-dev-key", // standard placeholder
			"provider":           "openai",
		}

		// Check if exists
		existing := gjson.Get(jsonStr, "custom_models")
		idx := -1
		if existing.IsArray() {
			for i, entry := range existing.Array() {
				if entry.Get("model").String() == m {
					idx = i
					break
				}
			}
		}

		var errSet error
		if idx >= 0 {
			jsonStr, errSet = sjson.Set(jsonStr, "custom_models."+string(rune(idx)), newEntry)
		} else {
			jsonStr, errSet = sjson.Set(jsonStr, "custom_models.-1", newEntry)
		}
		if errSet != nil {
			return errSet
		}
	}

	return os.WriteFile(configPath, []byte(jsonStr), 0644)
}
