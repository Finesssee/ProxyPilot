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

	"github.com/jchv/go-webview2"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
)

func main() {
	var repoRoot string
	var configPath string
	var exePath string
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate bin/ and logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.StringVar(&exePath, "exe", "", "Path to proxy engine binary (defaults to <repo>/bin/proxypilot-engine.exe)")
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

	target := st.BaseURL + "/proxypilot.html"

	// Prefer an in-app window (WebView2) so this behaves like a native dashboard app.
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "ProxyPilot",
			Width:  1200,
			Height: 850,
			Center: true,
		},
	})
	if w == nil {
		// Fallback: Edge app mode or default browser.
		edge, err := findEdge()
		if err != nil {
			_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
			return
		}
		_ = exec.Command(edge, "--app="+target, "--window-size=1200,850", "--lang=en-US").Start()
		return
	}
	defer w.Destroy()
	w.SetSize(1200, 850, webview2.HintNone)
	w.Navigate(target)
	w.Run()
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

	// If launched from a repo/app "bin" directory, treat its parent as the root.
	defaultRoot := exeDir
	if strings.EqualFold(filepath.Base(defaultRoot), "bin") {
		defaultRoot = filepath.Dir(defaultRoot)
	}

	if repoRoot == "" && defaultRoot != "" {
		repoRoot = defaultRoot
	}

	if configPath == "" && exeDir != "" {
		configPath = filepath.Join(repoRoot, "config.yaml")
	}

	if exePath == "" && exeDir != "" {
		for _, name := range []string{"proxypilot-engine.exe", "cliproxyapi-latest.exe"} {
			cand := filepath.Join(exeDir, name)
			if _, err := os.Stat(cand); err == nil {
				exePath = cand
				break
			}
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
