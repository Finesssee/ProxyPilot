package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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
		detectAider(proxyURL),
		detectGeminiCLI(proxyURL),
		detectDroidCLI(proxyURL),
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
	case "aider":
		return configureAider(proxyURL)
	case "gemini-cli":
		return configureGeminiCLI(proxyURL)
	case "droid":
		return configureDroidCLI(proxyURL)
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
	if strings.Contains(string(existing), "# ProxyPilot") {
		lines := strings.Split(string(existing), "\n")
		var newLines []string
		skip := false
		for _, line := range lines {
			if strings.Contains(line, "# ProxyPilot START") {
				skip = true
				continue
			}
			if strings.Contains(line, "# ProxyPilot END") {
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
		ConfigInstructions: "Set ANTHROPIC_BASE_URL environment variable",
		EnvVars: map[string]string{"ANTHROPIC_BASE_URL": proxyURL + "/v1"}, CanAutoConfigure: true,
	}
}

func configureClaudeCode(proxyURL string) error {
	var content string
	if runtime.GOOS == "windows" {
		content = fmt.Sprintf("# ProxyPilot START\n$env:ANTHROPIC_BASE_URL = \"%s/v1\"\n# ProxyPilot END\n", proxyURL)
	} else {
		content = fmt.Sprintf("# ProxyPilot START\nexport ANTHROPIC_BASE_URL=\"%s/v1\"\n# ProxyPilot END\n", proxyURL)
	}
	return appendToShellProfile(content)
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
		EnvVars: map[string]string{"OPENAI_BASE_URL": proxyURL + "/v1"}, CanAutoConfigure: true,
	}
}

func configureCodexCLI(proxyURL string) error {
	home := getHomeDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return err
	}
	configContent := fmt.Sprintf("# Configured by ProxyPilot\nmodel_provider = \"openai\"\n\n[openai]\nbase_url = \"%s/v1\"\n", proxyURL)
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(configContent), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte("{\"OPENAI_API_KEY\": \"proxypilot-local\"}"), 0644)
}

func detectAider(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("aider"); err == nil {
		detected = true
	}
	configured := false
	if detected {
		apiBase := os.Getenv("OPENAI_API_BASE")
		aiderBase := os.Getenv("AIDER_OPENAI_API_BASE")
		if strings.Contains(apiBase, proxyURL) || strings.Contains(aiderBase, proxyURL) {
			configured = true
		}
	}
	return Agent{
		ID: "aider", Name: "Aider", Detected: detected, Configured: configured,
		ConfigInstructions: "Set OPENAI_API_BASE environment variable",
		EnvVars: map[string]string{"OPENAI_API_BASE": proxyURL + "/v1"}, CanAutoConfigure: true,
	}
}

func configureAider(proxyURL string) error {
	var content string
	if runtime.GOOS == "windows" {
		content = fmt.Sprintf("# ProxyPilot START - Aider\n$env:OPENAI_API_BASE = \"%s/v1\"\n$env:OPENAI_API_KEY = \"proxypilot-local\"\n# ProxyPilot END - Aider\n", proxyURL)
	} else {
		content = fmt.Sprintf("# ProxyPilot START - Aider\nexport OPENAI_API_BASE=\"%s/v1\"\nexport OPENAI_API_KEY=\"proxypilot-local\"\n# ProxyPilot END - Aider\n", proxyURL)
	}
	return appendToShellProfile(content)
}

func detectGeminiCLI(proxyURL string) Agent {
	detected := false
	if _, err := exec.LookPath("gemini"); err == nil {
		detected = true
	}
	configured := false
	if detected {
		baseURL := os.Getenv("GOOGLE_GEMINI_BASE_URL")
		if strings.Contains(baseURL, proxyURL) {
			configured = true
		}
	}
	return Agent{
		ID: "gemini-cli", Name: "Gemini CLI", Detected: detected, Configured: configured,
		ConfigInstructions: "Set GOOGLE_GEMINI_BASE_URL environment variable",
		EnvVars: map[string]string{"GOOGLE_GEMINI_BASE_URL": proxyURL}, CanAutoConfigure: true,
	}
}

func configureGeminiCLI(proxyURL string) error {
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
			if strings.Contains(string(data), proxyURL) {
				configured = true
			}
		}
	}
	return Agent{
		ID: "droid", Name: "Droid CLI", Detected: detected, Configured: configured,
		ConfigInstructions: "Configure ~/.factory/config.json",
		EnvVars: map[string]string{}, CanAutoConfigure: true,
	}
}

func configureDroidCLI(proxyURL string) error {
	home := getHomeDir()
	factoryDir := filepath.Join(home, ".factory")
	configPath := filepath.Join(factoryDir, "config.json")
	if err := os.MkdirAll(factoryDir, 0755); err != nil {
		return err
	}
	var config map[string]interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &config)
	}
	if config == nil {
		config = make(map[string]interface{})
	}
	customModels := []map[string]interface{}{
		{"model_display_name": "Claude Sonnet 4 (ProxyPilot)", "model": "claude-sonnet-4-20250514", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000},
		{"model_display_name": "Claude Opus 4 (ProxyPilot)", "model": "claude-opus-4-20250514", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000},
		{"model_display_name": "GPT-4o (ProxyPilot)", "model": "gpt-4o", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000},
		{"model_display_name": "Gemini 2.5 Pro (ProxyPilot)", "model": "gemini-2.5-pro", "base_url": proxyURL + "/v1/", "api_key": "proxypilot-local", "provider": "generic-chat-completion-api", "max_tokens": 128000},
	}
	if existing, ok := config["custom_models"].([]interface{}); ok {
		for _, m := range existing {
			if model, ok := m.(map[string]interface{}); ok {
				if baseURL, ok := model["base_url"].(string); ok {
					if !strings.Contains(baseURL, "localhost:8") {
						customModels = append(customModels, model)
					}
				}
			}
		}
	}
	config["custom_models"] = customModels
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func UnconfigureCLIAgent(agentID string) error {
	switch agentID {
	case "claude-code":
		return removeFromShellProfile("# ProxyPilot START", "# ProxyPilot END")
	case "codex":
		return unconfigureCodexCLI()
	case "aider":
		return removeFromShellProfile("# ProxyPilot START - Aider", "# ProxyPilot END - Aider")
	case "gemini-cli":
		return removeFromShellProfile("# ProxyPilot START - Gemini", "# ProxyPilot END - Gemini")
	case "droid":
		return unconfigureDroidCLI()
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
		if strings.Contains(line, startMarker) {
			skip = true
			continue
		}
		if strings.Contains(line, endMarker) {
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
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	if strings.Contains(string(data), "Configured by ProxyPilot") {
		os.Remove(configPath)
		os.Remove(filepath.Join(codexDir, "auth.json"))
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.Contains(line, "base_url") || !strings.Contains(line, "localhost:8") {
			newLines = append(newLines, line)
		}
	}
	return os.WriteFile(configPath, []byte(strings.Join(newLines, "\n")), 0644)
}

func unconfigureDroidCLI() error {
	home := getHomeDir()
	factoryDir := filepath.Join(home, ".factory")
	configPath := filepath.Join(factoryDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	if existing, ok := config["custom_models"].([]interface{}); ok {
		var filtered []interface{}
		for _, m := range existing {
			if model, ok := m.(map[string]interface{}); ok {
				if baseURL, ok := model["base_url"].(string); ok {
					if !strings.Contains(baseURL, "localhost:8") {
						filtered = append(filtered, model)
					}
				} else {
					filtered = append(filtered, model)
				}
			}
		}
		config["custom_models"] = filtered
	}
	result, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, result, 0644)
}
