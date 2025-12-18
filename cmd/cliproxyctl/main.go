package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := strings.ToLower(strings.TrimSpace(os.Args[1]))

	fs := flag.NewFlagSet("cliproxyctl "+cmd, flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.yaml")
	repoRoot := fs.String("repo", "", "Repo root (used to locate bin/ and logs/)")
	exePath := fs.String("exe", "", "Path to ProxyPilot engine binary")
	jsonOut := fs.Bool("json", true, "Output JSON when applicable")
	_ = fs.Parse(os.Args[2:])

	switch cmd {
	case "status":
		st, err := desktopctl.StatusFor(*configPath)
		if err != nil {
			fatal(err)
		}
		printStatus(st, *jsonOut)
	case "start":
		st, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: *repoRoot, ConfigPath: *configPath, ExePath: *exePath})
		if err != nil {
			fatal(err)
		}
		printStatus(st, *jsonOut)
	case "stop":
		if err := desktopctl.Stop(desktopctl.StopOptions{}); err != nil {
			fatal(err)
		}
		fmt.Println("stopped")
	case "restart":
		st, err := desktopctl.Restart(desktopctl.StartOptions{RepoRoot: *repoRoot, ConfigPath: *configPath, ExePath: *exePath})
		if err != nil {
			fatal(err)
		}
		printStatus(st, *jsonOut)
	case "open-ui":
		if err := desktopctl.OpenManagementUI(*configPath); err != nil {
			fatal(err)
		}
	case "open-logs":
		if err := desktopctl.OpenLogsFolder(*repoRoot, *configPath); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: cliproxyctl <status|start|stop|restart|open-ui|open-logs> [flags]")
	fmt.Fprintln(os.Stderr, "Flags: -config <path> -repo <path> -exe <path>")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func printStatus(st desktopctl.Status, jsonOut bool) {
	if jsonOut {
		data, _ := json.MarshalIndent(st, "", "  ")
		fmt.Println(string(data))
		return
	}
	if st.Running {
		fmt.Printf("running pid=%d port=%d base=%s\n", st.PID, st.Port, st.BaseURL)
	} else {
		fmt.Printf("stopped last_error=%s\n", st.LastError)
	}
}
