package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AgentInfo describes a detected CLI agent
type AgentInfo struct {
	Name       string
	Detected   bool
	BinaryPath string
	ConfigPath string
	Version    string
}

// DetectAgents checks for installed CLI agents and returns their status
func DetectAgents() []AgentInfo {
	agents := []AgentInfo{
		detectClaudeCode(),
		detectCodex(),
		detectDroid(),
		detectGeminiCLI(),
		detectOpenCode(),
		detectCursor(),
		detectKiloCode(),
		detectRooCode(),
	}
	return agents
}

// DoDetectAgents prints detected agents to console
func DoDetectAgents() {
	fmt.Println("Detecting installed CLI agents...")
	fmt.Println()

	agents := DetectAgents()

	detected := 0
	for _, agent := range agents {
		status := "[-] Not found"
		if agent.Detected {
			status = "[+] Installed"
			detected++
		}

		fmt.Printf("  %-15s %s\n", agent.Name+":", status)
		if agent.Detected {
			if agent.BinaryPath != "" {
				fmt.Printf("  %-15s %s\n", "", "Binary: "+agent.BinaryPath)
			}
			if agent.ConfigPath != "" {
				fmt.Printf("  %-15s %s\n", "", "Config: "+agent.ConfigPath)
			}
			if agent.Version != "" {
				fmt.Printf("  %-15s %s\n", "", "Version: "+agent.Version)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Found %d/%d agents installed.\n", detected, len(agents))

	if detected > 0 {
		fmt.Println()
		fmt.Println("To configure an agent, run:")
		fmt.Println("  --setup-claude    Configure Claude Code")
		fmt.Println("  --setup-codex     Configure Codex CLI")
		fmt.Println("  --setup-droid     Configure Factory Droid")
		fmt.Println("  --setup-opencode  Configure OpenCode")
		fmt.Println("  --setup-gemini    Configure Gemini CLI")
		fmt.Println("  --setup-cursor    Configure Cursor")
		fmt.Println("  --setup-kilo      Configure Kilo Code CLI")
		fmt.Println("  --setup-roocode   Configure RooCode (VS Code)")
		fmt.Println()
		fmt.Println("Or configure all detected agents at once:")
		fmt.Println("  --setup-all       Configure all detected agents (with backup)")
	}
}

func detectClaudeCode() AgentInfo {
	info := AgentInfo{Name: "Claude Code"}

	// Check for claude binary
	if path, err := exec.LookPath("claude"); err == nil {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check config file
	configPath := expandPath("~/.claude/settings.json")
	if fileExists(configPath) {
		info.ConfigPath = configPath
		info.Detected = true
	}

	return info
}

func detectCodex() AgentInfo {
	info := AgentInfo{Name: "Codex CLI"}

	// Check for codex binary
	if path, err := exec.LookPath("codex"); err == nil {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check config directory
	configDir := expandPath("~/.codex")
	if dirExists(configDir) {
		info.ConfigPath = configDir
		info.Detected = true
	}

	return info
}

func detectDroid() AgentInfo {
	info := AgentInfo{Name: "Factory Droid"}

	// Check for droid or factory binary
	if path, err := exec.LookPath("droid"); err == nil {
		info.Detected = true
		info.BinaryPath = path
	} else if path, err := exec.LookPath("factory"); err == nil {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check config file
	configPath := expandPath("~/.factory/config.json")
	if fileExists(configPath) {
		info.ConfigPath = configPath
		info.Detected = true
	}

	return info
}

func detectGeminiCLI() AgentInfo {
	info := AgentInfo{Name: "Gemini CLI"}

	// Check for gemini binary
	if path, err := exec.LookPath("gemini"); err == nil {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check config file
	configPath := expandPath("~/.gemini/settings.json")
	if fileExists(configPath) {
		info.ConfigPath = configPath
		info.Detected = true
	}

	return info
}

func detectOpenCode() AgentInfo {
	info := AgentInfo{Name: "OpenCode"}

	// Check for opencode binary
	if path, err := exec.LookPath("opencode"); err == nil {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check config file (global) - OpenCode uses ~/.config/opencode/opencode.json
	configPath := expandPath("~/.config/opencode/opencode.json")
	if fileExists(configPath) {
		info.ConfigPath = configPath
		info.Detected = true
	}

	return info
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func detectCursor() AgentInfo {
	info := AgentInfo{Name: "Cursor"}

	// Check for cursor binary
	if path, err := exec.LookPath("cursor"); err == nil {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check config file - Cursor is a VS Code fork
	// Windows: %APPDATA%\Cursor\User\settings.json
	// macOS: ~/Library/Application Support/Cursor/User/settings.json
	// Linux: ~/.config/Cursor/User/settings.json
	var configPath string
	if home, err := os.UserHomeDir(); err == nil {
		// Try different platform-specific paths
		paths := []string{
			filepath.Join(home, ".config", "Cursor", "User", "settings.json"),                        // Linux
			filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json"), // macOS
		}
		// Windows via APPDATA
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append([]string{filepath.Join(appData, "Cursor", "User", "settings.json")}, paths...)
		}
		for _, p := range paths {
			if fileExists(p) {
				configPath = p
				break
			}
		}
	}

	if configPath != "" {
		info.ConfigPath = configPath
		info.Detected = true
	}

	return info
}

// getIDEExtensionsDirs returns extension directories for VS Code and forks
func getIDEExtensionsDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".vscode", "extensions"),
		filepath.Join(home, ".cursor", "extensions"),
		filepath.Join(home, ".antigravity", "extensions"),
		filepath.Join(home, ".windsurf", "extensions"),
		filepath.Join(home, ".vscode-insiders", "extensions"),
	}
}

// hasIDEExtension checks if an extension matching pattern exists in any IDE
func hasIDEExtension(pattern string) (bool, string) {
	pattern = strings.ToLower(pattern)
	for _, dir := range getIDEExtensionsDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), pattern) {
				return true, filepath.Join(dir, entry.Name())
			}
		}
	}
	return false, ""
}

func detectKiloCode() AgentInfo {
	info := AgentInfo{Name: "Kilo Code"}

	// Check for VS Code/Cursor/Antigravity extension
	if found, path := hasIDEExtension("kilo-code"); found {
		info.Detected = true
		info.BinaryPath = path
	} else if found, path := hasIDEExtension("kilocode"); found {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check for kilocode CLI binary as fallback
	if !info.Detected {
		if path, err := exec.LookPath("kilocode"); err == nil {
			info.Detected = true
			info.BinaryPath = path
		}
	}

	// Check for settings file
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".config", "kilocode", "kilo-code-settings.json")
	if fileExists(settingsPath) {
		info.ConfigPath = settingsPath
	}

	return info
}

func detectRooCode() AgentInfo {
	info := AgentInfo{Name: "RooCode"}

	// Check for roo-cline extension in VS Code/Cursor/Antigravity
	if found, path := hasIDEExtension("roo-cline"); found {
		info.Detected = true
		info.BinaryPath = path
	} else if found, path := hasIDEExtension("roocode"); found {
		info.Detected = true
		info.BinaryPath = path
	}

	// Check for settings file
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".config", "roocode", "roo-code-settings.json")
	if fileExists(settingsPath) {
		info.ConfigPath = settingsPath
	}

	return info
}
