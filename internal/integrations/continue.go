package integrations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type ContinueIntegration struct{}

func (i *ContinueIntegration) Meta() IntegrationStatus {
	return IntegrationStatus{
		ID:          "continue",
		Name:        "Continue",
		Description: "Continue VS Code Extension",
	}
}

func (i *ContinueIntegration) Detect() (bool, error) {
	configPath := filepath.Join(userHomeDir(), ".continue", "config.json")
	return fileExists(configPath), nil
}

func (i *ContinueIntegration) IsConfigured(proxyURL string) (bool, error) {
	configPath := filepath.Join(userHomeDir(), ".continue", "config.json")
	if !fileExists(configPath) {
		return false, nil
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	// Check if any model uses our proxy URL
	res := gjson.GetBytes(content, "models.#.apiBase")
	for _, match := range res.Array() {
		if strings.Contains(match.String(), proxyURL) {
			return true, nil
		}
	}
	return false, nil
}

func (i *ContinueIntegration) Configure(proxyURL string) error {
	configDir := filepath.Join(userHomeDir(), ".continue")
	configPath := filepath.Join(configDir, "config.json")
	
	if !fileExists(configPath) {
		return fmt.Errorf("continue config not found at %s", configPath)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	jsonStr := string(content)

	// Add ProxyPilot model
	// We'll prepend it to the models array so it appears first/selectable
	// Or we can check if it exists and update it.
	
	// Define the new model entry
	newModel := map[string]interface{}{
		"title":    "ProxyPilot (Auto)",
		"provider": "openai",
		"model":    "gpt-4o", // Default alias, ProxyPilot handles the routing
		"apiBase":  proxyURL + "/v1",
		"apiKey":   "sk-proxypilot-local", 
	}

	// sjson doesn't easily "prepend" or "upsert based on condition" without logic.
	// Let's iterate models to see if we exist.
	
	models := gjson.Get(jsonStr, "models")
	idx := -1
	if models.IsArray() {
		for i, m := range models.Array() {
			if m.Get("title").String() == "ProxyPilot (Auto)" {
				idx = i
				break
			}
		}
	}

	var updated string
	var errSet error
	if idx >= 0 {
		// Update existing
		path := fmt.Sprintf("models.%d", idx)
		updated, errSet = sjson.Set(jsonStr, path, newModel)
	} else {
		// Append (sjson supports -1 for append)
		updated, errSet = sjson.Set(jsonStr, "models.-1", newModel)
	}

	if errSet != nil {
		return errSet
	}

	return os.WriteFile(configPath, []byte(updated), 0644)
}
