// ppswitch is a lightweight config switcher for ProxyPilot.
// It provides fast agent configuration switching without logging overhead.
package main

import (
	"fmt"
	"os"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]

	// Handle flags
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			printHelp()
			return
		case "-v", "--version":
			fmt.Println("ppswitch", version)
			return
		}
	}

	// Minimal config with default port
	cfg := &config.Config{Port: 8317}

	switch len(args) {
	case 0:
		// Show status of all agents
		cmd.DoSwitchStatusAll()
	case 1:
		// Show status of specific agent
		cmd.DoSwitch(cfg, args[0], "status")
	case 2:
		// Switch agent to mode (proxy/native)
		cmd.DoSwitch(cfg, args[0], args[1])
	default:
		fmt.Fprintln(os.Stderr, "Error: too many arguments")
		fmt.Fprintln(os.Stderr, "Run 'ppswitch --help' for usage")
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`ppswitch - ProxyPilot Configuration Switcher

Install:
  bun install -g ppswitch

Usage:
  ppswitch                    Show status of all agents
  ppswitch <agent>            Show status of specific agent
  ppswitch <agent> <mode>     Switch agent to mode

Agents:
  claude    Claude Code       gemini    Gemini CLI
  codex     Codex CLI         opencode  OpenCode
  droid     Factory Droid     cursor    Cursor
  kilo      Kilo Code (*)     roocode   RooCode (*)

  (*) VS Code extensions - require manual configuration

Modes:
  proxy     Route through ProxyPilot (http://127.0.0.1:8317)
  native    Use direct API access (restore original config)

Examples:
  ppswitch                    Show all agent statuses
  ppswitch claude proxy       Switch Claude to proxy mode
  ppswitch gemini native      Switch Gemini to native mode
  ppswitch claude             Show Claude status only

Flags:
  -h, --help       Show this help
  -v, --version    Show version

More info: https://github.com/Finesssee/ProxyPilot`)
}
