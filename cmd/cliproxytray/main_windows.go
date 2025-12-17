//go:build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/proxypilotupdate"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/trayicon"
)

const autostartAppName = "ProxyPilot"

func main() {
	var repoRoot string
	var configPath string
	var exePath string
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate bin/ and logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.StringVar(&exePath, "exe", "", "Path to CLIProxyAPI binary (defaults to <repo>/bin/cliproxyapi-latest.exe)")
	flag.Parse()

	repoRoot, configPath, exePath = applyDefaults(repoRoot, configPath, exePath)
	run(repoRoot, configPath, exePath)
}

func run(repoRoot, configPath, exePath string) {
	systray.Run(func() {
		if ico := trayicon.ProxyPilotICO(); len(ico) > 0 {
			systray.SetIcon(ico)
		}
		systray.SetTitle("ProxyPilot")
		systray.SetTooltip("ProxyPilot")

		statusItem := systray.AddMenuItem("Status: ...", "Current status")
		statusItem.Disable()
		systray.AddSeparator()

		startItem := systray.AddMenuItem("Start", "Start CLIProxyAPI (proxy server)")
		stopItem := systray.AddMenuItem("Stop", "Stop CLIProxyAPI (proxy server)")
		restartItem := systray.AddMenuItem("Restart", "Restart CLIProxyAPI (proxy server)")
		systray.AddSeparator()

		autoOn, _, _ := desktopctl.IsWindowsRunAutostartEnabled(autostartAppName)
		autoStartItem := systray.AddMenuItemCheckbox("Launch on login", "Start this tray app when you log in", autoOn)
		autoProxyOn, _ := desktopctl.GetAutoStartProxy()
		autoStartProxyItem := systray.AddMenuItemCheckbox("Auto-start proxy", "Start the proxy server automatically when ProxyPilot launches", autoProxyOn)
		systray.AddSeparator()

		openProxyUI := systray.AddMenuItem("Open Proxy UI", "Open CLIProxyAPI built-in management UI")
		openLogs := systray.AddMenuItem("Open Logs", "Open logs folder")
		systray.AddSeparator()

		checkUpdates := systray.AddMenuItem("Check for Updates", "Check GitHub Releases for a newer ProxyPilot")
		updateNow := systray.AddMenuItem("Update ProxyPilot...", "Download and run the latest installer")
		updateNow.Disable()
		systray.AddSeparator()

		quitItem := systray.AddMenuItem("Quit", "Quit")

		lastErr := ""
		var (
			latestVersion string
			assetName     string
			assetURL      string
		)
		refresh := func() {
			st, _ := desktopctl.StatusFor(configPath)
			title := "Stopped"
			if st.Running {
				title = fmt.Sprintf("Running (:%d)", st.Port)
			}
			if st.LastError != "" && lastErr == "" {
				lastErr = st.LastError
			}
			if lastErr != "" {
				title = title + " - " + shorten(lastErr, 80)
			}
			statusItem.SetTitle("Status: " + title)
			systray.SetTooltip("ProxyPilot - " + title)

			if st.Running {
				startItem.Disable()
				stopItem.Enable()
				restartItem.Enable()
			} else {
				startItem.Enable()
				stopItem.Disable()
				restartItem.Disable()
			}
		}

		refresh()

		// Optional: auto-start the proxy server on launch.
		if autoProxyOn {
			go func() {
				st, _ := desktopctl.StatusFor(configPath)
				if st.Running {
					return
				}
				_, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
				if err != nil {
					lastErr = err.Error()
				} else {
					lastErr = ""
				}
				refresh()
			}()
		}

		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for range t.C {
				refresh()
			}
		}()

		go func() {
			for {
				select {
				case <-startItem.ClickedCh:
					_, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
					if err != nil {
						lastErr = err.Error()
						refresh()
						continue
					}
					lastErr = ""
					refresh()
				case <-stopItem.ClickedCh:
					if err := desktopctl.Stop(desktopctl.StopOptions{}); err != nil {
						lastErr = err.Error()
						refresh()
						continue
					}
					lastErr = ""
					refresh()
				case <-restartItem.ClickedCh:
					_, err := desktopctl.Restart(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
					if err != nil {
						lastErr = err.Error()
						refresh()
						continue
					}
					lastErr = ""
					refresh()
				case <-autoStartItem.ClickedCh:
					if autoStartItem.Checked() {
						_ = desktopctl.DisableWindowsRunAutostart(autostartAppName)
						autoStartItem.Uncheck()
						lastErr = ""
						refresh()
						continue
					}
					cmd, err := autostartCommand(repoRoot, configPath, exePath)
					if err != nil {
						lastErr = err.Error()
						refresh()
						continue
					}
					if err := desktopctl.EnableWindowsRunAutostart(autostartAppName, cmd); err != nil {
						lastErr = err.Error()
						refresh()
						continue
					}
					autoStartItem.Check()
					lastErr = ""
					refresh()
				case <-autoStartProxyItem.ClickedCh:
					if autoStartProxyItem.Checked() {
						_ = desktopctl.SetAutoStartProxy(false)
						autoStartProxyItem.Uncheck()
						lastErr = ""
						refresh()
						continue
					}
					_ = desktopctl.SetAutoStartProxy(true)
					autoStartProxyItem.Check()
					lastErr = ""
					refresh()
				case <-openProxyUI.ClickedCh:
					if err := openProxyUIWithAutostart(repoRoot, configPath, exePath); err != nil {
						lastErr = err.Error()
						refresh()
					} else {
						lastErr = ""
						refresh()
					}
				case <-openLogs.ClickedCh:
					if err := desktopctl.OpenLogsFolder(repoRoot, configPath); err != nil {
						lastErr = err.Error()
						refresh()
					}
				case <-checkUpdates.ClickedCh:
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
						defer cancel()
						rel, err := proxypilotupdate.FetchLatestRelease(ctx)
						if err != nil {
							lastErr = err.Error()
							refresh()
							return
						}
						latestVersion = rel.Version()
						assetName, assetURL = rel.FindPreferredAsset()
						if strings.TrimSpace(assetURL) == "" {
							lastErr = "no ProxyPilot asset found in latest release"
							refresh()
							return
						}
						// If this build is already tagged, avoid prompting unnecessarily.
						if buildinfo.Version != "" && buildinfo.Version != "dev" && strings.EqualFold(buildinfo.Version, latestVersion) {
							lastErr = "Up to date (" + latestVersion + ")"
							updateNow.Disable()
							refresh()
							return
						}
						updateNow.SetTitle("Update ProxyPilot... (" + latestVersion + ")")
						updateNow.Enable()
						lastErr = "Update available (" + latestVersion + ")"
						refresh()
					}()
				case <-updateNow.ClickedCh:
					if strings.TrimSpace(assetURL) == "" {
						lastErr = "no update asset URL (run Check for Updates)"
						refresh()
						continue
					}
					go func() {
						dest := filepath.Join(os.TempDir(), "ProxyPilot-"+sanitizeFileName(latestVersion)+"-"+sanitizeFileName(assetName))
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
						defer cancel()
						updateNow.SetTitle("Updating...")
						lastErr = "Downloading update..."
						refresh()
						if err := proxypilotupdate.DownloadToFile(ctx, assetURL, dest); err != nil {
							lastErr = err.Error()
							updateNow.SetTitle("Update ProxyPilot...")
							refresh()
							return
						}
						// Run installer/zip handler and exit ProxyPilot so files can be replaced.
						lastErr = "Launching installer..."
						refresh()
						_ = launchUpdate(dest)
						systray.Quit()
					}()
				case <-quitItem.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}

func openProxyUIWithAutostart(repoRoot, configPath, exePath string) error {
	// Prefer opening the desktop window if the UI binary exists next to this tray app.
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		dir := filepath.Dir(filepath.Clean(exe))
		uiExe := filepath.Join(dir, "ProxyPilotUI.exe")
		if _, err := os.Stat(uiExe); err == nil {
			args := []string{}
			if strings.TrimSpace(repoRoot) != "" {
				args = append(args, "-repo", repoRoot)
			}
			if strings.TrimSpace(configPath) != "" {
				args = append(args, "-config", configPath)
			}
			if strings.TrimSpace(exePath) != "" {
				args = append(args, "-exe", exePath)
			}
			return exec.Command(uiExe, args...).Start()
		}
	}
	// Fallback: open browser to the management UI (starts proxy if needed).
	st, _ := desktopctl.StatusFor(configPath)
	if !st.Running {
		if _, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath}); err != nil {
			return err
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			st2, _ := desktopctl.StatusFor(configPath)
			if st2.Running {
				break
			}
			time.Sleep(150 * time.Millisecond)
		}
	}
	return desktopctl.OpenManagementUI(configPath)
}

func shorten(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	repl := strings.NewReplacer(
		"\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_",
	)
	return repl.Replace(s)
}

func launchUpdate(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".exe":
		// Inno installer (silent, per-user).
		return exec.Command(path, "/SILENT", "/NORESTART", "/CLOSEAPPLICATIONS", "/RESTARTAPPLICATIONS").Start()
	default:
		// Fallback: let Windows handle the file (zip, etc.)
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	}
}

func autostartCommand(repoRoot, configPath, exePath string) (string, error) {
	exe, err := os.Executable()
	if err == nil && strings.TrimSpace(exe) != "" {
		exe = filepath.Clean(exe)
	} else {
		exe = ""
	}
	if exe == "" {
		return "", fmt.Errorf("unable to resolve tray executable path")
	}
	args := make([]string, 0, 6)
	if strings.TrimSpace(repoRoot) != "" {
		args = append(args, "-repo", repoRoot)
	}
	if strings.TrimSpace(configPath) != "" {
		args = append(args, "-config", configPath)
	}
	if strings.TrimSpace(exePath) != "" {
		args = append(args, "-exe", exePath)
	}
	return quoteWindowsCommand(exe, args), nil
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

	if configPath != "" {
		ensureConfig(configPath)
	}

	if exePath == "" && exeDir != "" {
		cand := filepath.Join(exeDir, "cliproxyapi-latest.exe")
		if _, err := os.Stat(cand); err == nil {
			exePath = cand
		}
	}

	return repoRoot, configPath, exePath
}

func ensureConfig(configPath string) {
	if _, err := os.Stat(configPath); err == nil {
		return
	}
	dir := filepath.Dir(configPath)
	example := filepath.Join(dir, "config.example.yaml")
	if _, err := os.Stat(example); err != nil {
		return
	}
	b, err := os.ReadFile(example)
	if err != nil {
		return
	}
	b = bootstrapLocalConfig(b)
	_ = os.WriteFile(configPath, b, 0o644)
}

func bootstrapLocalConfig(b []byte) []byte {
	// Best-effort: make the packaged default usable without editing.
	// Keep it simple (string-based) so we don't need YAML parsing in the tray binary.
	s := string(b)
	s = strings.ReplaceAll(s, "- \"your-api-key-1\"", "- \"local-dev-key\"")
	s = strings.ReplaceAll(s, "- \"your-api-key-2\"\r\n", "")
	s = strings.ReplaceAll(s, "- \"your-api-key-2\"\n", "")
	s = strings.ReplaceAll(s, "secret-key: \"\"\r\n", "secret-key: \"local-dev-key\"\r\n")
	s = strings.ReplaceAll(s, "secret-key: \"\"\n", "secret-key: \"local-dev-key\"\n")
	return []byte(s)
}

func quoteWindowsCommand(exe string, args []string) string {
	quoted := make([]string, 0, 1+len(args))
	quoted = append(quoted, `"`+strings.ReplaceAll(exe, `"`, `\"`)+`"`)
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if strings.ContainsAny(a, " \t") {
			quoted = append(quoted, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
		} else {
			quoted = append(quoted, a)
		}
	}
	return strings.Join(quoted, " ")
}
