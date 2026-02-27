package integrations

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const backupSuffix = ".proxypilot-backup"

// backupConfig creates a backup of the config file if it exists and no backup exists yet.
// Returns true if a new backup was created.
func backupConfig(configPath string) (bool, error) {
	backupPath := configPath + backupSuffix

	// Check if original exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false, nil // Nothing to backup
	}

	// Don't overwrite existing backup - it contains the original user config
	if _, err := os.Stat(backupPath); err == nil {
		return false, nil // Backup already exists
	}

	// Create backup
	src, err := os.Open(configPath)
	if err != nil {
		return false, err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return false, err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return false, err
	}

	return true, nil
}

// restoreConfig restores the config from backup if it exists.
// Returns true if restore was performed.
func restoreConfig(configPath string) (bool, error) {
	backupPath := configPath + backupSuffix

	// Check if backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return false, nil // No backup to restore
	}

	// Read backup
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return false, err
	}

	// Restore original
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return false, err
	}

	// Remove backup file
	os.Remove(backupPath)

	return true, nil
}

// hasBackup checks if a backup file exists for the given config path.
func hasBackup(configPath string) bool {
	_, err := os.Stat(configPath + backupSuffix)
	return err == nil
}

type Agent struct {
	Name               string            `json:"name"`
	ID                 string            `json:"id"`
	Detected           bool              `json:"detected"`
	Configured         bool              `json:"configured"`
	ConfigInstructions string            `json:"configInstructions"`
	EnvVars            map[string]string `json:"envVars"`
	CanAutoConfigure   bool              `json:"canAutoConfigure"`
}

func DetectCLIAgents(proxyURL string) []Agent {
	if proxyURL == "" {
		proxyURL = "http://localhost:8080"
	}
	return []Agent{
		detectClaudeCode(proxyURL),
		detectCodexCLI(proxyURL),
		detectOpenCode(proxyURL),
		detectGeminiCLI(proxyURL),
		detectDroidCLI(proxyURL),
		detectCursor(proxyURL),
		detectKiloCode(proxyURL),
		detectRooCode(proxyURL),
	}
}

func ConfigureCLIAgent(agentID, proxyURL string) error {
	if proxyURL == "" {
		proxyURL = "http://localhost:8080"
	}
	switch agentID {
	case "claude-code":
		return configureClaudeCode(proxyURL)
	case "codex":
		return configureCodexCLI(proxyURL)
	case "opencode":
		return configureOpenCode(proxyURL)
	case "gemini-cli":
		return configureGeminiCLI(proxyURL)
	case "droid":
		return configureDroidCLI(proxyURL)
	case "cursor":
		return configureCursor(proxyURL)
	case "kilo-code":
		return configureKiloCode(proxyURL)
	case "roo-code":
		return configureRooCode(proxyURL)
	default:
		return fmt.Errorf("unknown agent: %s", agentID)
	}
}

func getHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

func getShellProfile() string {
	home := getHomeDir()
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	}
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return filepath.Join(home, ".zshrc")
	}
	if strings.Contains(shell, "fish") {
		return filepath.Join(home, ".config", "fish", "config.fish")
	}
	return filepath.Join(home, ".bashrc")
}

func appendToShellProfile(content string) error {
	profilePath := getShellProfile()
	if err := os.MkdirAll(filepath.Dir(profilePath), 0755); err != nil {
		return err
	}
	existing, _ := os.ReadFile(profilePath)

	// Extract the marker from the content being added (first line)
	contentLines := strings.Split(content, "\n")
	var startMarker, endMarker string
	for _, line := range contentLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ProxyPilot START") {
			startMarker = trimmed
		}
		if strings.HasPrefix(trimmed, "# ProxyPilot END") {
			endMarker = trimmed
		}
	}

	// Remove existing section with the SAME markers (exact match)
	if startMarker != "" && strings.Contains(string(existing), startMarker) {
		lines := strings.Split(string(existing), "\n")
		var newLines []string
		skip := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == startMarker {
				skip = true
				continue
			}
			if trimmed == endMarker {
				skip = false
				continue
			}
			if !skip {
				newLines = append(newLines, line)
			}
		}
		existing = []byte(strings.Join(newLines, "\n"))
	}

	f, err := os.OpenFile(profilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(existing) > 0 {
		f.Write(existing)
		if !strings.HasSuffix(string(existing), "\n") {
			f.WriteString("\n")
		}
	}
	f.WriteString("\n" + content)
	return nil
}

func detectClaudeCode(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("claude"); err == nil {
		detected = true
	}
	if !detected {
		// Check for Claude Code config directory
		configPath := getClaudeCodeConfigPath()
		if _, err := os.Stat(filepath.Dir(configPath)); err == nil {
			detected = true
		}
	}
	if !detected {
		if appData := os.Getenv("APPDATA"); appData != "" {
			if _, err := os.Stat(filepath.Join(appData, "Claude")); err == nil {
				detected = true
			}
		}
	}
	configured := false
	if detected {
		i := &ClaudeIntegration{}
		configured, _ = i.IsConfigured(proxyURL)
	}
	return Agent{
		ID: "claude-code", Name: "Claude Code", Detected: detected, Configured: configured,
		ConfigInstructions: "Configure ~/.claude/settings.json with ProxyPilot env vars",
		EnvVars:            map[string]string{"ANTHROPIC_BASE_URL": proxyURL}, CanAutoConfigure: true,
	}
}

func configureClaudeCode(proxyURL string) error {
	return configureClaudeCodeSettings(proxyURL)
}

func detectCodexCLI(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("codex"); err == nil {
		detected = true
	}
	configured := false
	if detected {
		i := &CodexIntegration{}
		configured, _ = i.IsConfigured(proxyURL)
	}
	return Agent{
		ID: "codex", Name: "Codex CLI", Detected: detected, Configured: configured,
		ConfigInstructions: "Configure ~/.codex/config.toml",
		EnvVars:            map[string]string{"OPENAI_BASE_URL": proxyURL + "/v1"}, CanAutoConfigure: true,
	}
}

func configureCodexCLI(proxyURL string) error {
	home := getHomeDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(codexDir, "config.toml")
	authPath := filepath.Join(codexDir, "auth.json")

	// Update config.toml - preserve existing content, add ProxyPilot model provider
	existingConfig, _ := os.ReadFile(configPath)
	configStr := string(existingConfig)

	// Remove any existing ProxyPilot section
	startMarker := "# ProxyPilot START"
	endMarker := "# ProxyPilot END"
	if idx := strings.Index(configStr, startMarker); idx != -1 {
		if endIdx := strings.Index(configStr, endMarker); endIdx != -1 {
			configStr = configStr[:idx] + configStr[endIdx+len(endMarker):]
			configStr = strings.TrimSpace(configStr)
		}
	}

	// Add ProxyPilot model provider section at the end (proper Codex format)
	// Format: [model_providers.<id>] with name, base_url, env_key, wire_api
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

	if configStr != "" && !strings.HasSuffix(configStr, "\n") {
		configStr += "\n"
	}
	configStr += proxyPilotSection

	if err := os.WriteFile(configPath, []byte(configStr), 0644); err != nil {
		return err
	}

	// Update auth.json - preserve existing keys, add proxypilot key
	var authData map[string]interface{}
	if existingAuth, err := os.ReadFile(authPath); err == nil {
		json.Unmarshal(existingAuth, &authData)
	}
	if authData == nil {
		authData = make(map[string]interface{})
	}
	authData["PROXYPILOT_API_KEY"] = "proxypilot-local"

	authBytes, err := json.MarshalIndent(authData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(authPath, authBytes, 0644)
}

// ---- OpenCode ----

func getOpenCodeConfigPath() string {
	home := getHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

func detectOpenCode(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("opencode"); err == nil {
		detected = true
	}
	configPath := getOpenCodeConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		detected = true
	}
	configured := false
	if detected {
		if data, err := os.ReadFile(configPath); err == nil {
			// Check for proxypilot provider in config
			var config map[string]interface{}
			if json.Unmarshal(data, &config) == nil {
				if providers, ok := config["provider"].(map[string]interface{}); ok {
					if _, hasProxyPilot := providers["proxypilot"]; hasProxyPilot {
						configured = true
					}
				}
				// Also check marker flag
				if _, hasMarker := config["_proxypilot_configured"]; hasMarker {
					configured = true
				}
			}
		}
	}
	return Agent{
		ID: "opencode", Name: "OpenCode", Detected: detected, Configured: configured,
		ConfigInstructions: "Configure ~/.config/opencode/opencode.json with ProxyPilot provider",
		EnvVars:            map[string]string{}, CanAutoConfigure: true,
	}
}

func configureOpenCode(proxyURL string) error {
	configPath := getOpenCodeConfigPath()
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

	// Ensure $schema is present
	if _, ok := config["$schema"]; !ok {
		config["$schema"] = "https://opencode.ai/config.json"
	}

	// Add/update ProxyPilot provider while preserving others
	var providers map[string]interface{}
	if existingProviders, ok := config["provider"].(map[string]interface{}); ok {
		providers = existingProviders
	} else {
		providers = make(map[string]interface{})
	}

	// Add proxypilot provider (uses @ai-sdk/openai-compatible format)
	providers["proxypilot"] = map[string]interface{}{
		"name": "ProxyPilot",
		"npm":  "@ai-sdk/openai-compatible",
		"options": map[string]interface{}{
			"baseURL": proxyURL + "/v1",
			"name":    "proxypilot",
		},
		"models": map[string]interface{}{
			"gpt-4":                    map[string]interface{}{},
			"claude-sonnet-4-20250514": map[string]interface{}{},
			"claude-opus-4-20250514":   map[string]interface{}{},
			"gemini-2.5-pro":           map[string]interface{}{},
		},
	}
	config["provider"] = providers

	// Add ProxyPilot marker comment
	config["_proxypilot_configured"] = true

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func unconfigureOpenCode() error {
	configPath := getOpenCodeConfigPath()

	// Read existing config
	existingData, err := os.ReadFile(configPath)
	if err != nil {
		return nil // No config to unconfigure
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingData, &config); err != nil {
		return nil // Invalid JSON, nothing to do
	}

	// Remove proxypilot provider while preserving others
	if providers, ok := config["provider"].(map[string]interface{}); ok {
		delete(providers, "proxypilot")
		if len(providers) == 0 {
			delete(config, "provider")
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

func detectGeminiCLI(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("gemini"); err == nil {
		detected = true
	}
	// Also check for ~/.gemini directory
	if !detected {
		home := getHomeDir()
		geminiDir := filepath.Join(home, ".gemini")
		if _, err := os.Stat(geminiDir); err == nil {
			detected = true
		}
	}
	configured := false
	if detected {
		i := &GeminiIntegration{}
		configured, _ = i.IsConfigured(proxyURL)
	}
	return Agent{
		ID: "gemini-cli", Name: "Gemini CLI", Detected: detected, Configured: configured,
		ConfigInstructions: "Set GOOGLE_GEMINI_BASE_URL environment variable in shell profile",
		EnvVars:            map[string]string{"GOOGLE_GEMINI_BASE_URL": proxyURL}, CanAutoConfigure: true,
	}
}

func configureGeminiCLI(proxyURL string) error {
	return configureGeminiCLIShellProfile(proxyURL)
}

// configureGeminiCLIShellProfile configures Gemini CLI via shell profile env vars
// Note: Gemini CLI doesn't have a native settings.json base URL option,
// so shell profile is the correct approach
func configureGeminiCLIShellProfile(proxyURL string) error {
	var content string
	if runtime.GOOS == "windows" {
		content = fmt.Sprintf("# ProxyPilot START - Gemini\n$env:GOOGLE_GEMINI_BASE_URL = \"%s\"\n$env:GEMINI_API_KEY = \"proxypilot-local\"\n# ProxyPilot END - Gemini\n", proxyURL)
	} else {
		content = fmt.Sprintf("# ProxyPilot START - Gemini\nexport GOOGLE_GEMINI_BASE_URL=\"%s\"\nexport GEMINI_API_KEY=\"proxypilot-local\"\n# ProxyPilot END - Gemini\n", proxyURL)
	}
	return appendToShellProfile(content)
}

func detectDroidCLI(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("droid"); err == nil {
		detected = true
	}
	home := getHomeDir()
	factoryDir := filepath.Join(home, ".factory")
	if _, err := os.Stat(factoryDir); err == nil {
		detected = true
	}
	configured := false
	if detected {
		configPath := filepath.Join(factoryDir, "config.json")
		if data, err := os.ReadFile(configPath); err == nil {
			// Check for ProxyPilot models in config
			var config map[string]interface{}
			if json.Unmarshal(data, &config) == nil {
				// Check marker flag
				if _, hasMarker := config["_proxypilot_configured"]; hasMarker {
					configured = true
				}
				// Also check for ProxyPilot models in custom_models
				if !configured {
					if models, ok := config["custom_models"].([]interface{}); ok {
						for _, m := range models {
							if model, ok := m.(map[string]interface{}); ok {
								if _, isProxyPilot := model["_proxypilot"]; isProxyPilot {
									configured = true
									break
								}
								if displayName, ok := model["model_display_name"].(string); ok {
									if strings.Contains(displayName, "(ProxyPilot)") {
										configured = true
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return Agent{
		ID: "droid", Name: "Droid CLI", Detected: detected, Configured: configured,
		ConfigInstructions: "Configure ~/.factory/config.json",
		EnvVars:            map[string]string{}, CanAutoConfigure: true,
	}
}

func configureDroidCLI(proxyURL string) error {
	home := getHomeDir()
	factoryDir := filepath.Join(home, ".factory")
	configPath := filepath.Join(factoryDir, "config.json")
	if err := os.MkdirAll(factoryDir, 0755); err != nil {
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

	// ProxyPilot models to add
	proxyPilotModels := []map[string]interface{}{
		{"model_display_name": "Claude Sonnet 4 (ProxyPilot)", "model": "claude-sonnet-4-20250514", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000, "_proxypilot": true},
		{"model_display_name": "Claude Opus 4 (ProxyPilot)", "model": "claude-opus-4-20250514", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000, "_proxypilot": true},
		{"model_display_name": "GPT-4o (ProxyPilot)", "model": "gpt-4o", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000, "_proxypilot": true},
		{"model_display_name": "Gemini 2.5 Pro (ProxyPilot)", "model": "gemini-2.5-pro", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000, "_proxypilot": true},
	}

	// Get existing custom_models and filter out old ProxyPilot models
	var existingModels []interface{}
	if models, ok := config["custom_models"].([]interface{}); ok {
		for _, m := range models {
			if model, ok := m.(map[string]interface{}); ok {
				// Skip existing ProxyPilot models (identified by marker or display name)
				if _, isProxyPilot := model["_proxypilot"]; isProxyPilot {
					continue
				}
				if displayName, ok := model["model_display_name"].(string); ok {
					if strings.Contains(displayName, "(ProxyPilot)") {
						continue
					}
				}
				existingModels = append(existingModels, model)
			}
		}
	}

	// Combine existing models with ProxyPilot models
	var allModels []interface{}
	for _, m := range existingModels {
		allModels = append(allModels, m)
	}
	for _, m := range proxyPilotModels {
		allModels = append(allModels, m)
	}
	config["custom_models"] = allModels

	// Add ProxyPilot marker
	config["_proxypilot_configured"] = true

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func UnconfigureCLIAgent(agentID string) error {
	switch agentID {
	case "claude-code":
		// Use native settings.json unconfigure
		// Also clean up legacy shell profile entries for backwards compatibility
		removeFromShellProfile("# ProxyPilot START - Claude", "# ProxyPilot END - Claude")
		removeFromShellProfile("# ProxyPilot START", "# ProxyPilot END")
		return unconfigureClaudeCodeSettings()
	case "codex":
		return unconfigureCodexCLI()
	case "opencode":
		return unconfigureOpenCode()
	case "gemini-cli":
		return removeFromShellProfile("# ProxyPilot START - Gemini", "# ProxyPilot END - Gemini")
	case "droid":
		return unconfigureDroidCLI()
	case "cursor":
		return unconfigureCursor()
	case "kilo-code":
		return unconfigureKiloCode()
	case "roo-code":
		return unconfigureRooCode()
	default:
		return fmt.Errorf("unknown agent: %s", agentID)
	}
}

func removeFromShellProfile(startMarker, endMarker string) error {
	profilePath := getShellProfile()
	existing, err := os.ReadFile(profilePath)
	if err != nil {
		return nil
	}
	if !strings.Contains(string(existing), startMarker) {
		return nil
	}
	lines := strings.Split(string(existing), "\n")
	var newLines []string
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Use exact match for markers to avoid matching similar markers
		// e.g., "# ProxyPilot START" should not match "# ProxyPilot START - Gemini"
		if trimmed == startMarker || trimmed == startMarker+" " {
			skip = true
			continue
		}
		if trimmed == endMarker || trimmed == endMarker+" " {
			skip = false
			continue
		}
		if !skip {
			newLines = append(newLines, line)
		}
	}
	result := strings.TrimSpace(strings.Join(newLines, "\n")) + "\n"
	return os.WriteFile(profilePath, []byte(result), 0644)
}

func unconfigureCodexCLI() error {
	home := getHomeDir()
	codexDir := filepath.Join(home, ".codex")
	configPath := filepath.Join(codexDir, "config.toml")
	authPath := filepath.Join(codexDir, "auth.json")

	// Remove ProxyPilot section from config.toml (preserve rest)
	if configData, err := os.ReadFile(configPath); err == nil {
		configStr := string(configData)
		startMarker := "# ProxyPilot START"
		endMarker := "# ProxyPilot END"
		if idx := strings.Index(configStr, startMarker); idx != -1 {
			if endIdx := strings.Index(configStr, endMarker); endIdx != -1 {
				configStr = configStr[:idx] + configStr[endIdx+len(endMarker):]
				configStr = strings.TrimSpace(configStr) + "\n"
				os.WriteFile(configPath, []byte(configStr), 0644)
			}
		}
	}

	// Remove PROXYPILOT_API_KEY from auth.json (preserve rest)
	if authData, err := os.ReadFile(authPath); err == nil {
		var auth map[string]interface{}
		if json.Unmarshal(authData, &auth) == nil {
			delete(auth, "PROXYPILOT_API_KEY")
			if len(auth) > 0 {
				authBytes, _ := json.MarshalIndent(auth, "", "  ")
				os.WriteFile(authPath, authBytes, 0644)
			}
		}
	}

	// Clean up any old backup files
	os.Remove(configPath + backupSuffix)
	os.Remove(authPath + backupSuffix)

	return nil
}

func unconfigureDroidCLI() error {
	home := getHomeDir()
	factoryDir := filepath.Join(home, ".factory")
	configPath := filepath.Join(factoryDir, "config.json")

	// Read existing config
	existingData, err := os.ReadFile(configPath)
	if err != nil {
		return nil // No config to unconfigure
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingData, &config); err != nil {
		return nil // Invalid JSON, nothing to do
	}

	// Filter out ProxyPilot models while preserving others
	if models, ok := config["custom_models"].([]interface{}); ok {
		var filteredModels []interface{}
		for _, m := range models {
			if model, ok := m.(map[string]interface{}); ok {
				// Skip ProxyPilot models (identified by marker or display name)
				if _, isProxyPilot := model["_proxypilot"]; isProxyPilot {
					continue
				}
				if displayName, ok := model["model_display_name"].(string); ok {
					if strings.Contains(displayName, "(ProxyPilot)") {
						continue
					}
				}
				filteredModels = append(filteredModels, model)
			}
		}
		if len(filteredModels) == 0 {
			delete(config, "custom_models")
		} else {
			config["custom_models"] = filteredModels
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

// ---- Cursor ----

func getCursorConfigPath() string {
	home := getHomeDir()
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Cursor", "User", "settings.json")
		}
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json")
	}
	return filepath.Join(home, ".config", "Cursor", "User", "settings.json")
}

func detectCursor(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("cursor"); err == nil {
		detected = true
	}
	configPath := getCursorConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		detected = true
	}
	configured := false
	if detected {
		if data, err := os.ReadFile(configPath); err == nil {
			// Check for proxypilot model in config
			var config map[string]interface{}
			if json.Unmarshal(data, &config) == nil {
				if models, ok := config["models"].(map[string]interface{}); ok {
					if _, hasProxyPilot := models["proxypilot"]; hasProxyPilot {
						configured = true
					}
				}
				// Also check marker flag
				if _, hasMarker := config["_proxypilot_configured"]; hasMarker {
					configured = true
				}
			}
		}
	}
	return Agent{
		ID: "cursor", Name: "Cursor", Detected: detected, Configured: configured,
		ConfigInstructions: "Configure Cursor settings.json with ProxyPilot model",
		EnvVars:            map[string]string{}, CanAutoConfigure: true,
	}
}

func configureCursor(proxyURL string) error {
	configPath := getCursorConfigPath()
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

	// Add/update models section while preserving others
	var models map[string]interface{}
	if existingModels, ok := config["models"].(map[string]interface{}); ok {
		models = existingModels
	} else {
		models = make(map[string]interface{})
	}

	// Add proxypilot model
	models["proxypilot"] = map[string]interface{}{
		"name":          "ProxyPilot",
		"apiKey":        "proxypilot-local",
		"baseUrl":       proxyURL + "/v1",
		"contextLength": 200000,
	}
	config["models"] = models

	// Add ProxyPilot marker
	config["_proxypilot_configured"] = true

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func unconfigureCursor() error {
	configPath := getCursorConfigPath()

	// Read existing config
	existingData, err := os.ReadFile(configPath)
	if err != nil {
		return nil // No config to unconfigure
	}

	var config map[string]interface{}
	if err := json.Unmarshal(existingData, &config); err != nil {
		return nil // Invalid JSON, nothing to do
	}

	// Remove proxypilot model while preserving others
	if models, ok := config["models"].(map[string]interface{}); ok {
		delete(models, "proxypilot")
		if len(models) == 0 {
			delete(config, "models")
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

// ---- Kilo Code ----

// getIDEExtensionsDirs returns all known IDE extension directories
// (VS Code, Cursor, Antigravity, Windsurf, etc.)
func getIDEExtensionsDirs() []string {
	home := getHomeDir()
	dirs := []string{
		filepath.Join(home, ".vscode", "extensions"),          // VS Code
		filepath.Join(home, ".cursor", "extensions"),          // Cursor
		filepath.Join(home, ".antigravity", "extensions"),     // Antigravity
		filepath.Join(home, ".windsurf", "extensions"),        // Windsurf
		filepath.Join(home, ".vscode-insiders", "extensions"), // VS Code Insiders
	}
	return dirs
}

func hasVSCodeExtension(pattern string) bool {
	pattern = strings.ToLower(pattern)
	for _, extensionsDir := range getIDEExtensionsDirs() {
		entries, err := os.ReadDir(extensionsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), pattern) {
				return true
			}
		}
	}
	return false
}

func detectKiloCode(proxyURL string) Agent {
	detected := false
	// Check for VS Code extension (kilocode.kilo-code or similar)
	if hasVSCodeExtension("kilo-code") || hasVSCodeExtension("kilocode") {
		detected = true
	}
	// Also check for CLI binary as fallback
	if _, err := exec.LookPath("kilocode"); err == nil {
		detected = true
	}
	// Note: We can't reliably check if Kilo Code is configured for ProxyPilot
	// because it stores settings in VS Code's globalStorage, not a simple config file
	configured := false

	// Kilo Code requires manual configuration via its extension UI
	instructions := fmt.Sprintf(`Kilo Code requires manual configuration:
1. Open Kilo Code in VS Code/Cursor
2. Click Settings (gear icon)
3. Select "OpenAI Compatible" provider
4. Set Base URL: %s/v1
5. Set API Key: proxypilot-local
6. Choose a model (e.g., gpt-4, claude-sonnet-4-20250514)

Or use Import/Export:
1. Go to Settings > About Kilo Code > Export
2. Edit the JSON file to add ProxyPilot provider
3. Import via Settings > About Kilo Code > Import`, proxyURL)

	return Agent{
		ID: "kilo-code", Name: "Kilo Code", Detected: detected, Configured: configured,
		ConfigInstructions: instructions,
		EnvVars:            map[string]string{}, CanAutoConfigure: false,
	}
}

func configureKiloCode(proxyURL string) error {
	// Kilo Code stores config in VS Code globalStorage and requires manual UI configuration
	return fmt.Errorf(`Kilo Code requires manual configuration:
1. Open Kilo Code in VS Code/Cursor
2. Click Settings (gear icon)
3. Select "OpenAI Compatible" provider
4. Set Base URL: %s/v1
5. Set API Key: proxypilot-local
6. Choose a model (e.g., gpt-4, claude-sonnet-4-20250514)

Or use Import/Export:
1. Go to Settings > About Kilo Code > Export
2. Edit the JSON file to add ProxyPilot provider
3. Import via Settings > About Kilo Code > Import`, proxyURL)
}

func unconfigureKiloCode() error {
	// Kilo Code requires manual unconfiguration via its extension UI
	// Nothing to do here since we don't auto-configure
	return nil
}

// ---- RooCode ----

func detectRooCode(proxyURL string) Agent {
	detected := false
	// Check for roo-cline/roocode VS Code extension
	if hasVSCodeExtension("roo-cline") || hasVSCodeExtension("roocode") {
		detected = true
	}
	// Note: We can't reliably check if RooCode is configured for ProxyPilot
	// because it stores settings in VS Code's globalStorage, not a simple config file
	configured := false

	// RooCode requires manual configuration via its extension UI
	instructions := fmt.Sprintf(`RooCode requires manual configuration:
1. Open RooCode in VS Code/Cursor
2. Click Settings (gear icon)
3. Select "OpenAI Compatible" provider
4. Set Base URL: %s/v1
5. Set API Key: proxypilot-local
6. Choose a model (e.g., gpt-4, claude-sonnet-4-20250514)

Or use Import/Export:
1. Go to Settings > About Roo Code > Export
2. Edit the JSON file to add ProxyPilot provider
3. Import via Settings > About Roo Code > Import`, proxyURL)

	return Agent{
		ID: "roo-code", Name: "RooCode", Detected: detected, Configured: configured,
		ConfigInstructions: instructions,
		EnvVars:            map[string]string{}, CanAutoConfigure: false,
	}
}

func configureRooCode(proxyURL string) error {
	// RooCode stores config in VS Code globalStorage and requires manual UI configuration
	return fmt.Errorf(`RooCode requires manual configuration:
1. Open RooCode in VS Code/Cursor
2. Click Settings (gear icon)
3. Select "OpenAI Compatible" provider
4. Set Base URL: %s/v1
5. Set API Key: proxypilot-local
6. Choose a model (e.g., gpt-4, claude-sonnet-4-20250514)

Or use Import/Export:
1. Go to Settings > About Roo Code > Export
2. Edit the JSON file to add ProxyPilot provider
3. Import via Settings > About Roo Code > Import`, proxyURL)
}

func unconfigureRooCode() error {
	// RooCode requires manual unconfiguration via its extension UI
	// Nothing to do here since we don't auto-configure
	return nil
}
