package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// SwitchMode represents the configuration mode
type SwitchMode string

const (
	ModeProxy  SwitchMode = "proxy"
	ModeNative SwitchMode = "native"
	ModeStatus SwitchMode = "status"
)

// SwitchResult contains the result of a switch operation
type SwitchResult struct {
	Agent      string
	Success    bool
	Mode       SwitchMode
	Message    string
	ConfigPath string
	NativePath string
	ProxyPath  string
}

// AgentSwitchConfig holds paths for an agent's config files
type AgentSwitchConfig struct {
	Name       string
	ConfigPath string // Active config (e.g., settings.json)
	NativePath string // Native/direct API config (e.g., settings.native.json)
	ProxyPath  string // ProxyPilot config (e.g., settings.proxy.json)
}

// getAgentSwitchConfig returns the switch configuration for a given agent
func getAgentSwitchConfig(agent string) (*AgentSwitchConfig, error) {
	switch strings.ToLower(agent) {
	case "claude", "claude-code":
		return &AgentSwitchConfig{
			Name:       "Claude Code",
			ConfigPath: expandPath("~/.claude/settings.json"),
			NativePath: expandPath("~/.claude/settings.native.json"),
			ProxyPath:  expandPath("~/.claude/settings.proxy.json"),
		}, nil
	case "gemini", "gemini-cli":
		return &AgentSwitchConfig{
			Name:       "Gemini CLI",
			ConfigPath: expandPath("~/.gemini/settings.json"),
			NativePath: expandPath("~/.gemini/settings.native.json"),
			ProxyPath:  expandPath("~/.gemini/settings.proxy.json"),
		}, nil
	case "codex", "codex-cli":
		return &AgentSwitchConfig{
			Name:       "Codex CLI",
			ConfigPath: expandPath("~/.codex/config.toml"),
			NativePath: expandPath("~/.codex/config.native.toml"),
			ProxyPath:  expandPath("~/.codex/config.proxy.toml"),
		}, nil
	case "opencode":
		return &AgentSwitchConfig{
			Name:       "OpenCode",
			ConfigPath: expandPath("~/.config/opencode/opencode.json"),
			NativePath: expandPath("~/.config/opencode/opencode.native.json"),
			ProxyPath:  expandPath("~/.config/opencode/opencode.proxy.json"),
		}, nil
	case "droid", "factory-droid":
		return &AgentSwitchConfig{
			Name:       "Factory Droid",
			ConfigPath: expandPath("~/.factory/config.json"),
			NativePath: expandPath("~/.factory/config.native.json"),
			ProxyPath:  expandPath("~/.factory/config.proxy.json"),
		}, nil
	case "cursor":
		configPath := getCursorSettingsPath()
		if configPath == "" {
			return nil, fmt.Errorf("could not find Cursor settings path")
		}
		dir := filepath.Dir(configPath)
		return &AgentSwitchConfig{
			Name:       "Cursor",
			ConfigPath: configPath,
			NativePath: filepath.Join(dir, "settings.native.json"),
			ProxyPath:  filepath.Join(dir, "settings.proxy.json"),
		}, nil
	case "kilo", "kilo-code", "kilocode":
		// Kilo Code is a VS Code extension - config is in globalStorage
		// We can't programmatically switch, but we track it for status
		return &AgentSwitchConfig{
			Name:       "Kilo Code",
			ConfigPath: "", // VS Code extension - manual config required
			NativePath: "",
			ProxyPath:  "",
		}, nil
	case "roo", "roocode", "roo-code":
		// RooCode is a VS Code extension - config is in globalStorage
		// We can't programmatically switch, but we track it for status
		return &AgentSwitchConfig{
			Name:       "RooCode",
			ConfigPath: "", // VS Code extension - manual config required
			NativePath: "",
			ProxyPath:  "",
		}, nil
	default:
		return nil, fmt.Errorf("unknown agent: %s (supported: claude, gemini, codex, opencode, droid, cursor, kilo, roocode)", agent)
	}
}

// getCursorSettingsPath finds the Cursor settings path
func getCursorSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	paths := []string{}
	if appData := os.Getenv("APPDATA"); appData != "" {
		paths = append(paths, filepath.Join(appData, "Cursor", "User", "settings.json"))
	}
	paths = append(paths,
		filepath.Join(home, ".config", "Cursor", "User", "settings.json"),
		filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json"),
	)
	for _, p := range paths {
		if fileExists(p) || dirExists(filepath.Dir(p)) {
			return p
		}
	}
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}

// DoSwitch handles the switch command
func DoSwitch(cfg *config.Config, agent string, mode string) {
	if agent == "" {
		// Show status for all agents
		DoSwitchStatusAll()
		return
	}

	switchMode := SwitchMode(strings.ToLower(mode))
	if switchMode == "" {
		switchMode = ModeStatus
	}

	var result SwitchResult
	switch switchMode {
	case ModeProxy:
		result = SwitchToProxy(cfg, agent)
	case ModeNative:
		result = SwitchToNative(agent)
	case ModeStatus:
		result = GetSwitchStatus(agent)
	default:
		fmt.Printf("Unknown mode: %s (use: proxy, native, status)\n", mode)
		return
	}

	printSwitchResult(result)
}

// DoSwitchStatusAll shows status for all supported agents
func DoSwitchStatusAll() {
	fmt.Println("ProxyPilot Agent Configuration Status")
	fmt.Println("======================================")
	fmt.Println()

	agents := []string{"claude", "gemini", "codex", "opencode", "droid", "cursor", "kilo", "roocode"}
	for _, agent := range agents {
		result := GetSwitchStatus(agent)
		if result.Success {
			modeStr := string(result.Mode)
			if result.Mode == ModeProxy {
				modeStr = "\033[32mPROXY\033[0m" // Green
			} else if result.Mode == ModeNative {
				modeStr = "\033[33mNATIVE\033[0m" // Yellow
			}
			fmt.Printf("  %-15s %s\n", result.Agent+":", modeStr)
		} else {
			fmt.Printf("  %-15s \033[90m%s\033[0m\n", result.Agent+":", result.Message) // Gray
		}
	}
	fmt.Println()
	fmt.Println("Usage: proxypilot switch <agent> <proxy|native>")
}

// SwitchToProxy switches an agent to ProxyPilot mode
func SwitchToProxy(cfg *config.Config, agent string) SwitchResult {
	agentCfg, err := getAgentSwitchConfig(agent)
	if err != nil {
		return SwitchResult{Agent: agent, Success: false, Message: err.Error()}
	}

	// VS Code extensions (Kilo Code, RooCode) require manual configuration
	if agentCfg.ConfigPath == "" {
		port := cfg.Port
		if port == 0 {
			port = 8317
		}

		// Generate extension-specific instructions using detailed config generators
		var message string
		switch agentCfg.Name {
		case "Kilo Code":
			message = GetKiloCodeInstructions(port)
		case "RooCode":
			message = GetRooCodeInstructions(port)
		default:
			message = fmt.Sprintf("%s requires manual configuration:\n  1. Open settings in the extension\n  2. Select 'OpenAI Compatible' provider\n  3. Set Base URL: http://127.0.0.1:%d/v1\n  4. Set API Key: proxypal-local", agentCfg.Name, port)
		}

		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: true,
			Mode:    ModeProxy,
			Message: message,
		}
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(agentCfg.ConfigPath), 0755); err != nil {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: false,
			Message: fmt.Sprintf("Failed to create config directory: %v", err),
		}
	}

	// If native backup doesn't exist and current config exists, create backup
	if !fileExists(agentCfg.NativePath) && fileExists(agentCfg.ConfigPath) {
		if err := copyFile(agentCfg.ConfigPath, agentCfg.NativePath); err != nil {
			return SwitchResult{
				Agent:   agentCfg.Name,
				Success: false,
				Message: fmt.Sprintf("Failed to backup native config: %v", err),
			}
		}
	}

	// Generate proxy config if it doesn't exist or update it
	if err := generateProxyConfig(cfg, agentCfg); err != nil {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: false,
			Message: fmt.Sprintf("Failed to generate proxy config: %v", err),
		}
	}

	// Copy proxy config to active config
	if err := copyFile(agentCfg.ProxyPath, agentCfg.ConfigPath); err != nil {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: false,
			Message: fmt.Sprintf("Failed to activate proxy config: %v", err),
		}
	}

	return SwitchResult{
		Agent:      agentCfg.Name,
		Success:    true,
		Mode:       ModeProxy,
		Message:    "Switched to PROXY mode. Restart the agent to apply changes.",
		ConfigPath: agentCfg.ConfigPath,
		NativePath: agentCfg.NativePath,
		ProxyPath:  agentCfg.ProxyPath,
	}
}

// SwitchToNative switches an agent back to native/direct API mode
func SwitchToNative(agent string) SwitchResult {
	agentCfg, err := getAgentSwitchConfig(agent)
	if err != nil {
		return SwitchResult{Agent: agent, Success: false, Message: err.Error()}
	}

	// VS Code extensions (Kilo Code, RooCode) require manual configuration
	if agentCfg.ConfigPath == "" {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: true,
			Mode:    ModeNative,
			Message: fmt.Sprintf("%s requires manual configuration:\n  1. Open settings in the extension\n  2. Change provider back to your preferred API\n  3. Update API key as needed", agentCfg.Name),
		}
	}

	// Check if native config exists
	if !fileExists(agentCfg.NativePath) {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: false,
			Message: "No native config backup found. Run 'switch proxy' first to create a backup.",
		}
	}

	// Copy native config to active config
	if err := copyFile(agentCfg.NativePath, agentCfg.ConfigPath); err != nil {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: false,
			Message: fmt.Sprintf("Failed to restore native config: %v", err),
		}
	}

	return SwitchResult{
		Agent:      agentCfg.Name,
		Success:    true,
		Mode:       ModeNative,
		Message:    "Switched to NATIVE mode. Restart the agent to apply changes.",
		ConfigPath: agentCfg.ConfigPath,
		NativePath: agentCfg.NativePath,
		ProxyPath:  agentCfg.ProxyPath,
	}
}

// GetSwitchStatus returns the current mode of an agent
func GetSwitchStatus(agent string) SwitchResult {
	agentCfg, err := getAgentSwitchConfig(agent)
	if err != nil {
		return SwitchResult{Agent: agent, Success: false, Message: err.Error()}
	}

	// VS Code extensions (Kilo Code, RooCode) - can't detect status automatically
	if agentCfg.ConfigPath == "" {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: false,
			Message: "manual config (VS Code extension)",
		}
	}

	// Check if config exists
	if !fileExists(agentCfg.ConfigPath) {
		return SwitchResult{
			Agent:   agentCfg.Name,
			Success: false,
			Message: "not installed",
		}
	}

	// Determine current mode by comparing configs
	mode := detectCurrentMode(agentCfg)

	return SwitchResult{
		Agent:      agentCfg.Name,
		Success:    true,
		Mode:       mode,
		ConfigPath: agentCfg.ConfigPath,
		NativePath: agentCfg.NativePath,
		ProxyPath:  agentCfg.ProxyPath,
	}
}

// detectCurrentMode determines if the current config is proxy or native
func detectCurrentMode(agentCfg *AgentSwitchConfig) SwitchMode {
	currentData, err := os.ReadFile(agentCfg.ConfigPath)
	if err != nil {
		return ModeNative // Default to native if can't read
	}

	// Check if proxy config exists and matches
	if fileExists(agentCfg.ProxyPath) {
		proxyData, err := os.ReadFile(agentCfg.ProxyPath)
		if err == nil && string(currentData) == string(proxyData) {
			return ModeProxy
		}
	}

	// Check for ProxyPilot markers in the config
	content := string(currentData)
	if strings.Contains(content, "127.0.0.1:8317") ||
		strings.Contains(content, "127.0.0.1:8318") ||
		strings.Contains(content, "proxypal-local") ||
		strings.Contains(content, "ANTHROPIC_BASE_URL") && strings.Contains(content, "127.0.0.1") {
		return ModeProxy
	}

	return ModeNative
}

// generateProxyConfig creates the proxy configuration for an agent
func generateProxyConfig(cfg *config.Config, agentCfg *AgentSwitchConfig) error {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	switch agentCfg.Name {
	case "Claude Code":
		return generateClaudeProxyConfig(agentCfg, port)
	case "Gemini CLI":
		return generateGeminiProxyConfig(agentCfg, port)
	case "Codex CLI":
		return generateCodexProxyConfig(agentCfg, port)
	case "OpenCode":
		return generateOpenCodeProxyConfig(agentCfg, port)
	case "Factory Droid":
		return generateDroidProxyConfig(agentCfg, port)
	case "Cursor":
		return generateCursorProxyConfig(agentCfg, port)
	case "Kilo Code", "RooCode":
		// VS Code extensions require manual configuration - return instructions as error
		// This case is typically not reached due to early return in SwitchToProxy,
		// but is included for completeness and programmatic access
		return fmt.Errorf("VS Code extension %s requires manual configuration. Use 'proxypilot switch %s proxy' for instructions",
			agentCfg.Name, strings.ToLower(strings.ReplaceAll(agentCfg.Name, " ", "")))
	default:
		return fmt.Errorf("proxy config generation not implemented for %s", agentCfg.Name)
	}
}

func generateClaudeProxyConfig(agentCfg *AgentSwitchConfig, port int) error {
	// Start with native config if it exists, otherwise empty
	var settings map[string]any
	if fileExists(agentCfg.NativePath) {
		if data, err := os.ReadFile(agentCfg.NativePath); err == nil {
			_ = json.Unmarshal(data, &settings)
		}
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	// Ensure env map exists
	var envMap map[string]any
	if existingEnv, ok := settings["env"].(map[string]any); ok {
		envMap = existingEnv
	} else {
		envMap = make(map[string]any)
	}

	// Set ProxyPilot values
	envMap["ANTHROPIC_BASE_URL"] = fmt.Sprintf("http://127.0.0.1:%d", port)
	envMap["ANTHROPIC_AUTH_TOKEN"] = "proxypal-local"
	settings["env"] = envMap

	return writeJSONFile(agentCfg.ProxyPath, settings)
}

func generateGeminiProxyConfig(agentCfg *AgentSwitchConfig, port int) error {
	var settings map[string]any
	if fileExists(agentCfg.NativePath) {
		if data, err := os.ReadFile(agentCfg.NativePath); err == nil {
			_ = json.Unmarshal(data, &settings)
		}
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	settings["api_base"] = fmt.Sprintf("http://127.0.0.1:%d", port)

	return writeJSONFile(agentCfg.ProxyPath, settings)
}

func generateCodexProxyConfig(agentCfg *AgentSwitchConfig, port int) error {
	// For TOML files, we'll create a simple config
	content := fmt.Sprintf(`# ProxyPilot Configuration
model_provider = "cliproxyapi"
base_url = "http://127.0.0.1:%d/v1"
`, port)

	return os.WriteFile(agentCfg.ProxyPath, []byte(content), 0644)
}

func generateOpenCodeProxyConfig(agentCfg *AgentSwitchConfig, port int) error {
	var settings map[string]any
	if fileExists(agentCfg.NativePath) {
		if data, err := os.ReadFile(agentCfg.NativePath); err == nil {
			_ = json.Unmarshal(data, &settings)
		}
	}
	if settings == nil {
		settings = map[string]any{
			"$schema": "https://opencode.ai/config.json",
		}
	}

	provider := make(map[string]any)
	if existingProvider, ok := settings["provider"].(map[string]any); ok {
		provider = existingProvider
	}

	provider["local"] = map[string]any{
		"name": "ProxyPilot",
		"options": map[string]any{
			"baseURL": fmt.Sprintf("http://127.0.0.1:%d/v1", port),
			"apiKey":  "proxypal-local",
		},
	}
	settings["provider"] = provider

	if _, hasModel := settings["model"]; !hasModel {
		settings["model"] = "local/gpt-4"
	}

	return writeJSONFile(agentCfg.ProxyPath, settings)
}

func generateDroidProxyConfig(agentCfg *AgentSwitchConfig, port int) error {
	var settings map[string]any
	if fileExists(agentCfg.NativePath) {
		if data, err := os.ReadFile(agentCfg.NativePath); err == nil {
			_ = json.Unmarshal(data, &settings)
		}
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	proxypalModels := []map[string]any{
		{
			"name":     "proxypal-claude-opus",
			"base_url": fmt.Sprintf("http://127.0.0.1:%d/v1", port),
			"api_key":  "proxypal-local",
			"model":    "claude-opus-4-5-20251101",
		},
		{
			"name":     "proxypal-claude-sonnet",
			"base_url": fmt.Sprintf("http://127.0.0.1:%d/v1", port),
			"api_key":  "proxypal-local",
			"model":    "claude-sonnet-4-5-20250929",
		},
	}

	var finalModels []map[string]any
	finalModels = append(finalModels, proxypalModels...)

	if existing, ok := settings["custom_models"].([]any); ok {
		for _, entry := range existing {
			if m, ok := entry.(map[string]any); ok {
				name, _ := m["name"].(string)
				if !strings.HasPrefix(name, "proxypal-") {
					finalModels = append(finalModels, m)
				}
			}
		}
	}

	settings["custom_models"] = finalModels

	return writeJSONFile(agentCfg.ProxyPath, settings)
}

func generateCursorProxyConfig(agentCfg *AgentSwitchConfig, port int) error {
	var settings map[string]any
	if fileExists(agentCfg.NativePath) {
		if data, err := os.ReadFile(agentCfg.NativePath); err == nil {
			_ = json.Unmarshal(data, &settings)
		}
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	var models map[string]any
	if existingModels, ok := settings["models"].(map[string]any); ok {
		models = existingModels
	} else {
		models = make(map[string]any)
	}

	models["proxypilot"] = map[string]any{
		"name":          "ProxyPilot",
		"apiKey":        "proxypal-local",
		"baseUrl":       fmt.Sprintf("http://127.0.0.1:%d/v1", port),
		"contextLength": 200000,
	}
	settings["models"] = models

	return writeJSONFile(agentCfg.ProxyPath, settings)
}

// KiloCodeConfig holds the configuration instructions for Kilo Code VS Code extension
type KiloCodeConfig struct {
	BaseURL          string
	APIKey           string
	SettingsPath     string
	VSCodeConfigPath string
	Instructions     string
}

// generateKiloCodeProxyConfig generates configuration instructions for Kilo Code VS Code extension.
// Since Kilo Code stores its settings in VS Code's globalStorage, we cannot auto-configure it.
// Instead, this function returns detailed instructions for manual configuration.
func generateKiloCodeProxyConfig(port int) KiloCodeConfig {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", port)
	apiKey := "proxypal-local"

	// Determine VS Code settings paths based on OS
	home, _ := os.UserHomeDir()
	var vscodePath, settingsPath string

	// Check for VS Code settings.json path
	if appData := os.Getenv("APPDATA"); appData != "" {
		// Windows
		vscodePath = filepath.Join(appData, "Code", "User", "settings.json")
		settingsPath = filepath.Join(appData, "Code", "User", "globalStorage", "kilocode.kilo-code", "settings", "cline_mcp_settings.json")
	} else if home != "" {
		// macOS/Linux
		if _, err := os.Stat(filepath.Join(home, "Library")); err == nil {
			// macOS
			vscodePath = filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
			settingsPath = filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", "kilocode.kilo-code", "settings", "cline_mcp_settings.json")
		} else {
			// Linux
			vscodePath = filepath.Join(home, ".config", "Code", "User", "settings.json")
			settingsPath = filepath.Join(home, ".config", "Code", "User", "globalStorage", "kilocode.kilo-code", "settings", "cline_mcp_settings.json")
		}
	}

	instructions := fmt.Sprintf(`Kilo Code VS Code Extension Configuration
==========================================

Kilo Code is a VS Code extension that requires manual configuration.
Follow these steps to configure it for ProxyPilot:

1. Open VS Code and go to the Kilo Code extension settings
   - Click the Kilo Code icon in the sidebar
   - Click the settings gear icon

2. Add a new API Provider:
   - Click "Edit API Configuration" or similar
   - Select "OpenAI Compatible" or "OpenRouter" as the provider type

3. Configure the provider with these settings:
   - Base URL: %s
   - API Key: %s

4. Alternatively, add this to your VS Code settings.json:

   "kilocode.apiProvider": "openai-compatible",
   "kilocode.openaiCompatible.baseUrl": "%s",
   "kilocode.openaiCompatible.apiKey": "%s"

5. Select a model from the available list or configure a custom model

Note: The exact settings keys may vary by Kilo Code version.
Check the extension documentation for the latest configuration options.

Settings file locations:
- VS Code settings: %s
- Kilo Code storage: %s
`, baseURL, apiKey, baseURL, apiKey, vscodePath, settingsPath)

	return KiloCodeConfig{
		BaseURL:          baseURL,
		APIKey:           apiKey,
		SettingsPath:     settingsPath,
		VSCodeConfigPath: vscodePath,
		Instructions:     instructions,
	}
}

// GetKiloCodeInstructions returns formatted instructions for configuring Kilo Code
func GetKiloCodeInstructions(port int) string {
	cfg := generateKiloCodeProxyConfig(port)
	return cfg.Instructions
}

// RooCodeConfig holds the configuration instructions for RooCode VS Code extension
type RooCodeConfig struct {
	BaseURL          string
	APIKey           string
	SettingsPath     string
	VSCodeConfigPath string
	Instructions     string
}

// generateRooCodeProxyConfig generates configuration instructions for RooCode VS Code extension.
// Since RooCode stores its settings in VS Code's globalStorage, we cannot auto-configure it.
// Instead, this function returns detailed instructions for manual configuration.
func generateRooCodeProxyConfig(port int) RooCodeConfig {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", port)
	apiKey := "proxypal-local"

	// Determine VS Code settings paths based on OS
	home, _ := os.UserHomeDir()
	var vscodePath, settingsPath string

	// Check for VS Code settings.json path
	if appData := os.Getenv("APPDATA"); appData != "" {
		// Windows
		vscodePath = filepath.Join(appData, "Code", "User", "settings.json")
		settingsPath = filepath.Join(appData, "Code", "User", "globalStorage", "rooveterinaryinc.roo-cline", "settings", "cline_mcp_settings.json")
	} else if home != "" {
		// macOS/Linux
		if _, err := os.Stat(filepath.Join(home, "Library")); err == nil {
			// macOS
			vscodePath = filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
			settingsPath = filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", "rooveterinaryinc.roo-cline", "settings", "cline_mcp_settings.json")
		} else {
			// Linux
			vscodePath = filepath.Join(home, ".config", "Code", "User", "settings.json")
			settingsPath = filepath.Join(home, ".config", "Code", "User", "globalStorage", "rooveterinaryinc.roo-cline", "settings", "cline_mcp_settings.json")
		}
	}

	instructions := fmt.Sprintf(`RooCode VS Code Extension Configuration
========================================

RooCode is a VS Code extension that requires manual configuration.
Follow these steps to configure it for ProxyPilot:

1. Open VS Code and go to the RooCode extension settings
   - Click the RooCode icon in the sidebar
   - Click the settings gear icon

2. Add a new API Provider:
   - Click "Edit API Configuration" or similar
   - Select "OpenAI Compatible" or "OpenRouter" as the provider type

3. Configure the provider with these settings:
   - Base URL: %s
   - API Key: %s

4. Alternatively, add this to your VS Code settings.json:

   "roo-cline.apiProvider": "openai-compatible",
   "roo-cline.openaiCompatible.baseUrl": "%s",
   "roo-cline.openaiCompatible.apiKey": "%s"

5. Select a model from the available list or configure a custom model

Note: The exact settings keys may vary by RooCode version.
Check the extension documentation for the latest configuration options.

Settings file locations:
- VS Code settings: %s
- RooCode storage: %s
`, baseURL, apiKey, baseURL, apiKey, vscodePath, settingsPath)

	return RooCodeConfig{
		BaseURL:          baseURL,
		APIKey:           apiKey,
		SettingsPath:     settingsPath,
		VSCodeConfigPath: vscodePath,
		Instructions:     instructions,
	}
}

// GetRooCodeInstructions returns formatted instructions for configuring RooCode
func GetRooCodeInstructions(port int) string {
	cfg := generateRooCodeProxyConfig(port)
	return cfg.Instructions
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// printSwitchResult prints a formatted switch result
func printSwitchResult(result SwitchResult) {
	if !result.Success {
		fmt.Printf("\033[31m✗\033[0m %s: %s\n", result.Agent, result.Message)
		return
	}

	modeColor := "\033[33m" // Yellow for native
	if result.Mode == ModeProxy {
		modeColor = "\033[32m" // Green for proxy
	}

	fmt.Printf("\033[32m✓\033[0m %s: %s%s\033[0m\n", result.Agent, modeColor, strings.ToUpper(string(result.Mode)))
	if result.Message != "" {
		fmt.Printf("  %s\n", result.Message)
	}
	if result.ConfigPath != "" {
		fmt.Printf("  Config: %s\n", result.ConfigPath)
	}
}
