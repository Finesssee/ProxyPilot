//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
)

func main() {
	var repoRoot string
	var configPath string
	var exePath string
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate bin/ and logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.StringVar(&exePath, "exe", "", "Path to CLIProxyAPI binary (defaults to <repo>/bin/cliproxyapi-latest.exe)")
	flag.Parse()

	repoRoot, configPath, exePath = applyDefaults(repoRoot, configPath, exePath)

	st, _ := desktopctl.StatusFor(configPath)
	if !st.Running {
		_, _ = desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			st, _ = desktopctl.StatusFor(configPath)
			if st.Running {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
		_ = messageBox("ProxyPilot", "Proxy is not running.\n\nClick Start in the ProxyPilot tray menu, then try again.")
		os.Exit(1)
	}

	target := st.BaseURL + "/management.html"

	edge, err := findEdge()
	if err != nil {
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
		return
	}

	// Open as a desktop app window (no address bar) using Edge app mode.
	_ = exec.Command(edge, "--app="+target, "--window-size=1200,850").Start()
}

func findEdge() (string, error) {
	if p, err := exec.LookPath("msedge.exe"); err == nil && strings.TrimSpace(p) != "" {
		return p, nil
	}
	if p, err := exec.LookPath("msedge"); err == nil && strings.TrimSpace(p) != "" {
		return p, nil
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("msedge.exe not found")
}

func applyDefaults(repoRoot, configPath, exePath string) (string, string, string) {
	repoRoot = strings.TrimSpace(repoRoot)
	configPath = strings.TrimSpace(configPath)
	exePath = strings.TrimSpace(exePath)

	exe, _ := os.Executable()
	exeDir := ""
	if strings.TrimSpace(exe) != "" {
		exeDir = filepath.Dir(filepath.Clean(exe))
	}

	if repoRoot == "" && exeDir != "" {
		repoRoot = exeDir
	}

	if configPath == "" && exeDir != "" {
		configPath = filepath.Join(exeDir, "config.yaml")
	}

	if exePath == "" && exeDir != "" {
		cand := filepath.Join(exeDir, "cliproxyapi-latest.exe")
		if _, err := os.Stat(cand); err == nil {
			exePath = cand
		}
	}

	return repoRoot, configPath, exePath
}

func messageBox(title, text string) error {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	// MB_OK | MB_ICONERROR
	_, _, err := proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00000000|0x00000010)
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}
