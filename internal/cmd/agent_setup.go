package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// SetupResult contains the result of a setup operation
type SetupResult struct {
	CLI        string
	Success    bool
	Message    string
	BackupPath string
	ConfigPath string
}

// backupFile creates a timestamped backup of the file if it exists
// Returns the backup path if created, empty string if file didn't exist
func backupFile(path string) (string, error) {
	if !fileExists(path) {
		return "", nil
	}

	// Create backup directory
	backupDir := filepath.Join(filepath.Dir(path), ".proxypilot-backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate timestamped backup filename
	timestamp := time.Now().Format("20060102-150405")
	baseName := filepath.Base(path)
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s.%s.bak", baseName, timestamp))

	// Read original file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read original file: %w", err)
	}

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return backupPath, nil
}

// DoSetupClaude generates Claude Code configuration with backup and safe merge
func DoSetupClaude(cfg *config.Config) {
	result := SetupClaudeSafe(cfg)
	printSetupResult(result)
}

// SetupClaudeSafe configures Claude Code with backup and returns result
func SetupClaudeSafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	settingsPath := expandPath("~/.claude/settings.json")

	// Backup existing config
	backupPath, err := backupFile(settingsPath)
	if err != nil {
		return SetupResult{
			CLI:     "Claude Code",
			Success: false,
			Message: fmt.Sprintf("Failed to backup config: %v", err),
		}
	}

	// Read existing settings or create new
	var settings map[string]any
	if existing := readJSONFile(settingsPath); existing != nil {
		settings = existing
	} else {
		settings = make(map[string]any)
	}

	// Ensure env map exists
	var envMap map[string]any
	if existingEnv, ok := settings["env"].(map[string]any); ok {
		envMap = existingEnv
	} else {
		envMap = make(map[string]any)
	}

	// Only set ProxyPilot-specific values (safe merge - preserve user's other settings)
	envMap["ANTHROPIC_BASE_URL"] = fmt.Sprintf("http://127.0.0.1:%d", port)
	envMap["ANTHROPIC_AUTH_TOKEN"] = "proxypal-local"

	settings["env"] = envMap

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return SetupResult{
			CLI:     "Claude Code",
			Success: false,
			Message: fmt.Sprintf("Failed to create config directory: %v", err),
		}
	}

	// Write settings
	if err := writeJSONFile(settingsPath, settings); err != nil {
		return SetupResult{
			CLI:     "Claude Code",
			Success: false,
			Message: fmt.Sprintf("Failed to write config: %v", err),
		}
	}

	return SetupResult{
		CLI:        "Claude Code",
		Success:    true,
		Message:    "Configured successfully. Restart Claude Code to apply changes.",
		BackupPath: backupPath,
		ConfigPath: settingsPath,
	}
}

// DoSetupCodex generates Codex CLI configuration with backup and safe merge
func DoSetupCodex(cfg *config.Config) {
	result := SetupCodexSafe(cfg)
	printSetupResult(result)
}

// SetupCodexSafe configures Codex CLI with backup and returns result
func SetupCodexSafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	configDir := expandPath("~/.codex")
	configPath := filepath.Join(configDir, "config.toml")
	authPath := filepath.Join(configDir, "auth.json")

	// Backup existing configs
	configBackup, err := backupFile(configPath)
	if err != nil {
		return SetupResult{
			CLI:     "Codex CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to backup config.toml: %v", err),
		}
	}

	authBackup, err := backupFile(authPath)
	if err != nil {
		return SetupResult{
			CLI:     "Codex CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to backup auth.json: %v", err),
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return SetupResult{
			CLI:     "Codex CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to create config directory: %v", err),
		}
	}

	// Read existing config.toml or create new
	existingConfig := make(map[string]any)
	if data, err := os.ReadFile(configPath); err == nil {
		_ = toml.Unmarshal(data, &existingConfig)
	}

	// Safe merge - only set ProxyPilot-specific values
	existingConfig["model_provider"] = "cliproxyapi"
	existingConfig["base_url"] = fmt.Sprintf("http://127.0.0.1:%d/v1", port)

	// Write config.toml preserving other settings
	var configBuilder strings.Builder
	configBuilder.WriteString("# Modified by ProxyPilot (original settings preserved)\n")
	for k, v := range existingConfig {
		switch val := v.(type) {
		case string:
			configBuilder.WriteString(fmt.Sprintf("%s = %q\n", k, val))
		case bool:
			configBuilder.WriteString(fmt.Sprintf("%s = %v\n", k, val))
		case int, int64, float64:
			configBuilder.WriteString(fmt.Sprintf("%s = %v\n", k, val))
		}
	}

	if err := os.WriteFile(configPath, []byte(configBuilder.String()), 0644); err != nil {
		return SetupResult{
			CLI:     "Codex CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to write config.toml: %v", err),
		}
	}

	// Read existing auth.json or create new
	var authData map[string]any
	if existing := readJSONFile(authPath); existing != nil {
		authData = existing
	} else {
		authData = make(map[string]any)
	}

	// Safe merge - only set the API key
	authData["OPENAI_API_KEY"] = "proxypal-local"

	if err := writeJSONFile(authPath, authData); err != nil {
		return SetupResult{
			CLI:     "Codex CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to write auth.json: %v", err),
		}
	}

	backupInfo := ""
	if configBackup != "" || authBackup != "" {
		backupInfo = configBackup
		if authBackup != "" && backupInfo != "" {
			backupInfo += ", " + authBackup
		} else if authBackup != "" {
			backupInfo = authBackup
		}
	}

	return SetupResult{
		CLI:        "Codex CLI",
		Success:    true,
		Message:    "Configured successfully.",
		BackupPath: backupInfo,
		ConfigPath: configPath,
	}
}

// DoSetupDroid generates Factory Droid configuration with backup
func DoSetupDroid(cfg *config.Config) {
	result := SetupDroidSafe(cfg)
	printSetupResult(result)
}

// SetupDroidSafe configures Factory Droid with backup and returns result
func SetupDroidSafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	configPath := expandPath("~/.factory/config.json")

	// Backup existing config
	backupPath, err := backupFile(configPath)
	if err != nil {
		return SetupResult{
			CLI:     "Factory Droid",
			Success: false,
			Message: fmt.Sprintf("Failed to backup config: %v", err),
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return SetupResult{
			CLI:     "Factory Droid",
			Success: false,
			Message: fmt.Sprintf("Failed to create config directory: %v", err),
		}
	}

	// Read existing config or create new
	var droidConfig map[string]any
	if existing := readJSONFile(configPath); existing != nil {
		droidConfig = existing
	} else {
		droidConfig = make(map[string]any)
	}

	// Build ProxyPilot custom_models
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
		{
			"name":     "proxypal-claude-haiku",
			"base_url": fmt.Sprintf("http://127.0.0.1:%d/v1", port),
			"api_key":  "proxypal-local",
			"model":    "claude-haiku-4-5-20251001",
		},
	}

	// Merge: keep user's non-proxypal models, replace proxypal models
	var finalModels []map[string]any
	finalModels = append(finalModels, proxypalModels...)

	if existing, ok := droidConfig["custom_models"].([]any); ok {
		for _, entry := range existing {
			if m, ok := entry.(map[string]any); ok {
				name, _ := m["name"].(string)
				if !strings.HasPrefix(name, "proxypal-") {
					finalModels = append(finalModels, m)
				}
			}
		}
	}

	droidConfig["custom_models"] = finalModels

	if err := writeJSONFile(configPath, droidConfig); err != nil {
		return SetupResult{
			CLI:     "Factory Droid",
			Success: false,
			Message: fmt.Sprintf("Failed to write config: %v", err),
		}
	}

	return SetupResult{
		CLI:        "Factory Droid",
		Success:    true,
		Message:    "Configured with ProxyPilot models: proxypal-claude-opus, proxypal-claude-sonnet, proxypal-claude-haiku",
		BackupPath: backupPath,
		ConfigPath: configPath,
	}
}

// DoSetupOpenCode generates OpenCode configuration with backup and safe merge
func DoSetupOpenCode(cfg *config.Config) {
	result := SetupOpenCodeSafe(cfg)
	printSetupResult(result)
}

// SetupOpenCodeSafe configures OpenCode with backup and returns result
func SetupOpenCodeSafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	// OpenCode uses ~/.config/opencode/opencode.json for global config
	configDir := expandPath("~/.config/opencode")
	configPath := filepath.Join(configDir, "opencode.json")

	// Backup existing config
	backupPath, err := backupFile(configPath)
	if err != nil {
		return SetupResult{
			CLI:     "OpenCode",
			Success: false,
			Message: fmt.Sprintf("Failed to backup config: %v", err),
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return SetupResult{
			CLI:     "OpenCode",
			Success: false,
			Message: fmt.Sprintf("Failed to create config directory: %v", err),
		}
	}

	// Read existing config or create new
	var openCodeConfig map[string]any
	if existing := readJSONFile(configPath); existing != nil {
		openCodeConfig = existing
	} else {
		openCodeConfig = map[string]any{
			"$schema": "https://opencode.ai/config.json",
		}
	}

	// Safe merge - configure provider with ProxyPilot endpoint
	// OpenCode uses provider config for custom API base URLs
	provider := make(map[string]any)
	if existingProvider, ok := openCodeConfig["provider"].(map[string]any); ok {
		provider = existingProvider
	}

	// Add ProxyPilot as a custom "local" provider
	provider["local"] = map[string]any{
		"name": "ProxyPilot",
		"options": map[string]any{
			"baseURL": fmt.Sprintf("http://127.0.0.1:%d/v1", port),
			"apiKey":  "proxypal-local",
		},
	}

	openCodeConfig["provider"] = provider

	// Set default model to use ProxyPilot if not already set
	if _, hasModel := openCodeConfig["model"]; !hasModel {
		openCodeConfig["model"] = "local/gpt-4"
	}

	if err := writeJSONFile(configPath, openCodeConfig); err != nil {
		return SetupResult{
			CLI:     "OpenCode",
			Success: false,
			Message: fmt.Sprintf("Failed to write config: %v", err),
		}
	}

	return SetupResult{
		CLI:        "OpenCode",
		Success:    true,
		Message:    "Configured with ProxyPilot provider. Use 'opencode -m local/gpt-4' or select in TUI.",
		BackupPath: backupPath,
		ConfigPath: configPath,
	}
}

// DoSetupGeminiCLI generates Gemini CLI configuration with backup and safe merge
func DoSetupGeminiCLI(cfg *config.Config) {
	result := SetupGeminiCLISafe(cfg)
	printSetupResult(result)
}

// SetupGeminiCLISafe configures Gemini CLI with backup and returns result
func SetupGeminiCLISafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	// Gemini CLI uses ~/.gemini/settings.json
	configPath := expandPath("~/.gemini/settings.json")

	// Backup existing config
	backupPath, err := backupFile(configPath)
	if err != nil {
		return SetupResult{
			CLI:     "Gemini CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to backup config: %v", err),
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return SetupResult{
			CLI:     "Gemini CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to create config directory: %v", err),
		}
	}

	// Read existing config or create new
	var geminiConfig map[string]any
	if existing := readJSONFile(configPath); existing != nil {
		geminiConfig = existing
	} else {
		geminiConfig = make(map[string]any)
	}

	// Safe merge - set ProxyPilot as the API endpoint
	// Gemini CLI uses GOOGLE_API_KEY and can use custom endpoints
	geminiConfig["api_base"] = fmt.Sprintf("http://127.0.0.1:%d", port)

	if err := writeJSONFile(configPath, geminiConfig); err != nil {
		return SetupResult{
			CLI:     "Gemini CLI",
			Success: false,
			Message: fmt.Sprintf("Failed to write config: %v", err),
		}
	}

	return SetupResult{
		CLI:        "Gemini CLI",
		Success:    true,
		Message:    "Configured successfully. You may also need to set GEMINI_API_KEY=proxypal-local",
		BackupPath: backupPath,
		ConfigPath: configPath,
	}
}

// DoSetupCursor generates Cursor configuration with backup and safe merge
func DoSetupCursor(cfg *config.Config) {
	result := SetupCursorSafe(cfg)
	printSetupResult(result)
}

// SetupCursorSafe configures Cursor with backup and returns result
func SetupCursorSafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	// Find Cursor settings path based on platform
	var settingsPath string
	if home, err := os.UserHomeDir(); err == nil {
		// Try different platform-specific paths
		paths := []string{}
		// Windows via APPDATA
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "Cursor", "User", "settings.json"))
		}
		paths = append(paths,
			filepath.Join(home, ".config", "Cursor", "User", "settings.json"),                        // Linux
			filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json"), // macOS
		)
		for _, p := range paths {
			if fileExists(p) {
				settingsPath = p
				break
			}
			// If file doesn't exist but parent dir does, use that path
			if dirExists(filepath.Dir(p)) {
				settingsPath = p
				break
			}
		}
		// Fallback to first available path
		if settingsPath == "" && len(paths) > 0 {
			settingsPath = paths[0]
		}
	}

	if settingsPath == "" {
		return SetupResult{
			CLI:     "Cursor",
			Success: false,
			Message: "Could not determine Cursor settings path",
		}
	}

	// Backup existing config
	backupPath, err := backupFile(settingsPath)
	if err != nil {
		return SetupResult{
			CLI:     "Cursor",
			Success: false,
			Message: fmt.Sprintf("Failed to backup config: %v", err),
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return SetupResult{
			CLI:     "Cursor",
			Success: false,
			Message: fmt.Sprintf("Failed to create config directory: %v", err),
		}
	}

	// Read existing settings or create new
	var settings map[string]any
	if existing := readJSONFile(settingsPath); existing != nil {
		settings = existing
	} else {
		settings = make(map[string]any)
	}

	// Safe merge - add ProxyPilot as an OpenAI-compatible custom model
	// Cursor uses models configuration for custom API endpoints
	var models map[string]any
	if existingModels, ok := settings["models"].(map[string]any); ok {
		models = existingModels
	} else {
		models = make(map[string]any)
	}

	// Add ProxyPilot model configuration
	models["proxypilot"] = map[string]any{
		"name":          "ProxyPilot",
		"apiKey":        "proxypal-local",
		"baseUrl":       fmt.Sprintf("http://127.0.0.1:%d/v1", port),
		"contextLength": 200000,
	}

	settings["models"] = models

	if err := writeJSONFile(settingsPath, settings); err != nil {
		return SetupResult{
			CLI:     "Cursor",
			Success: false,
			Message: fmt.Sprintf("Failed to write config: %v", err),
		}
	}

	return SetupResult{
		CLI:        "Cursor",
		Success:    true,
		Message:    "Configured with ProxyPilot model. Select 'proxypilot' in Cursor AI settings.",
		BackupPath: backupPath,
		ConfigPath: settingsPath,
	}
}

// DoSetupKiloCode shows manual configuration instructions for Kilo Code
func DoSetupKiloCode(cfg *config.Config) {
	result := SetupKiloCodeSafe(cfg)
	printSetupResult(result)
}

// SetupKiloCodeSafe returns manual configuration instructions for Kilo Code
// Kilo Code is a VS Code extension that stores config in globalStorage,
// which cannot be reliably configured programmatically
func SetupKiloCodeSafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	instructions := fmt.Sprintf(`Kilo Code requires manual configuration:

1. Open Kilo Code in VS Code/Cursor/Antigravity
2. Click Settings (gear icon)
3. Select "OpenAI Compatible" provider
4. Set Base URL: http://127.0.0.1:%d/v1
5. Set API Key: proxypal-local
6. Choose a model (e.g., gpt-4, claude-sonnet-4-20250514)

Or use Import/Export:
1. Go to Settings > About Kilo Code > Export
2. Edit the JSON file to add ProxyPilot as a provider
3. Import via Settings > About Kilo Code > Import`, port)

	return SetupResult{
		CLI:     "Kilo Code",
		Success: true,
		Message: instructions,
	}
}

// DoSetupRooCode shows manual configuration instructions for RooCode
func DoSetupRooCode(cfg *config.Config) {
	result := SetupRooCodeSafe(cfg)
	printSetupResult(result)
}

// SetupRooCodeSafe returns manual configuration instructions for RooCode
// RooCode is a VS Code extension that stores config in globalStorage,
// which cannot be reliably configured programmatically
func SetupRooCodeSafe(cfg *config.Config) SetupResult {
	port := cfg.Port
	if port == 0 {
		port = 8317
	}

	instructions := fmt.Sprintf(`RooCode requires manual configuration:

1. Open RooCode in VS Code/Cursor/Antigravity
2. Click Settings (gear icon)
3. Select "OpenAI Compatible" provider
4. Set Base URL: http://127.0.0.1:%d/v1
5. Set API Key: proxypal-local
6. Choose a model (e.g., gpt-4, claude-sonnet-4-20250514)

Or use Import/Export:
1. Go to Settings > About Roo Code > Export
2. Edit the JSON file to add ProxyPilot as a provider
3. Import via Settings > About Roo Code > Import`, port)

	return SetupResult{
		CLI:     "RooCode",
		Success: true,
		Message: instructions,
	}
}

// DoSetupAll configures all detected CLI agents
func DoSetupAll(cfg *config.Config) {
	fmt.Println("ProxyPilot Unified Setup Wizard")
	fmt.Println("================================")
	fmt.Println()
	fmt.Println("This wizard will configure all detected AI CLI tools to use ProxyPilot.")
	fmt.Println("Your existing configurations will be backed up before any changes.")
	fmt.Println()

	// Detect installed agents
	agents := DetectAgents()
	detected := 0
	for _, agent := range agents {
		if agent.Detected {
			detected++
		}
	}

	if detected == 0 {
		fmt.Println("No CLI agents detected. Install one of the supported tools first:")
		fmt.Println("  - Claude Code (claude)")
		fmt.Println("  - Codex CLI (codex)")
		fmt.Println("  - Factory Droid (droid)")
		fmt.Println("  - OpenCode (opencode)")
		fmt.Println("  - Gemini CLI (gemini)")
		fmt.Println("  - Cursor (cursor)")
		fmt.Println("  - Kilo Code (kilocode)")
		fmt.Println("  - RooCode (VS Code extension)")
		return
	}

	fmt.Printf("Detected %d CLI agent(s). Configuring...\n", detected)
	fmt.Println()

	var results []SetupResult

	for _, agent := range agents {
		if !agent.Detected {
			continue
		}

		var result SetupResult
		switch agent.Name {
		case "Claude Code":
			result = SetupClaudeSafe(cfg)
		case "Codex CLI":
			result = SetupCodexSafe(cfg)
		case "Factory Droid":
			result = SetupDroidSafe(cfg)
		case "OpenCode":
			result = SetupOpenCodeSafe(cfg)
		case "Gemini CLI":
			result = SetupGeminiCLISafe(cfg)
		case "Cursor":
			result = SetupCursorSafe(cfg)
		case "Kilo Code":
			result = SetupKiloCodeSafe(cfg)
		case "RooCode":
			result = SetupRooCodeSafe(cfg)
		default:
			continue
		}
		results = append(results, result)
	}

	// Print summary
	fmt.Println()
	fmt.Println("Setup Summary")
	fmt.Println("=============")
	fmt.Println()

	successCount := 0
	for _, result := range results {
		printSetupResult(result)
		if result.Success {
			successCount++
		}
	}

	fmt.Println()
	fmt.Printf("Configured %d/%d agents successfully.\n", successCount, len(results))

	if successCount > 0 {
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Start ProxyPilot: proxypilot")
		fmt.Println("  2. Login to your providers: proxypilot --claude-login, --codex-login, etc.")
		fmt.Println("  3. Restart your CLI tools to apply the configuration")
	}
}

// printSetupResult prints a formatted setup result
func printSetupResult(result SetupResult) {
	status := "[OK]"
	if !result.Success {
		status = "[FAIL]"
	}

	fmt.Printf("%s %s\n", status, result.CLI)
	if result.Success {
		fmt.Printf("    Config: %s\n", result.ConfigPath)
		if result.BackupPath != "" {
			fmt.Printf("    Backup: %s\n", result.BackupPath)
		}
		fmt.Printf("    %s\n", result.Message)
	} else {
		fmt.Printf("    Error: %s\n", result.Message)
	}
	fmt.Println()
}

// RestoreBackup restores a configuration file from its backup
func RestoreBackup(backupPath string) error {
	if !fileExists(backupPath) {
		return fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	// Determine original path from backup path
	// Backup format: config.json.20240101-120000.bak -> config.json
	dir := filepath.Dir(filepath.Dir(backupPath)) // Go up from .proxypilot-backups
	baseName := filepath.Base(backupPath)

	// Remove timestamp and .bak suffix
	parts := strings.Split(baseName, ".")
	if len(parts) < 3 {
		return fmt.Errorf("invalid backup filename format: %s", baseName)
	}

	// Reconstruct original filename (everything before the timestamp)
	// e.g., "config.json.20240101-120000.bak" -> "config.json"
	var originalParts []string
	for _, part := range parts {
		// Stop before timestamp (which looks like 20XXXXXX-XXXXXX)
		if len(part) == 15 && strings.Contains(part, "-") {
			break
		}
		// Stop at .bak
		if part == "bak" {
			break
		}
		originalParts = append(originalParts, part)
	}

	originalName := strings.Join(originalParts, ".")
	originalPath := filepath.Join(dir, originalName)

	// Read backup
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Write to original location
	if err := os.WriteFile(originalPath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore: %w", err)
	}

	return nil
}

// ListBackups lists all ProxyPilot backups in a directory
func ListBackups(configDir string) ([]string, error) {
	backupDir := filepath.Join(configDir, ".proxypilot-backups")
	if !dirExists(backupDir) {
		return nil, nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".bak") {
			backups = append(backups, filepath.Join(backupDir, entry.Name()))
		}
	}

	return backups, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func readJSONFile(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func writeJSONFile(path string, data any) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}
