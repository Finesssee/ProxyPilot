package cmd

import (
	"fmt"
	"os"
	"os/exec"
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
		fmt.Println("  --setup-claude   Configure Claude Code")
		fmt.Println("  --setup-codex    Configure Codex CLI")
		fmt.Println("  --setup-droid    Configure Factory Droid")
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
