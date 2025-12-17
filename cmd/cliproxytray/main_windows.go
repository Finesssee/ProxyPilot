//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
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

	run(repoRoot, configPath, exePath)
}

func run(repoRoot, configPath, exePath string) {
	systray.Run(func() {
		systray.SetTitle("ProxyPilot")
		systray.SetTooltip("ProxyPilot")

		statusItem := systray.AddMenuItem("Status: …", "Current status")
		statusItem.Disable()
		systray.AddSeparator()

		startItem := systray.AddMenuItem("Start", "Start CLIProxyAPI (proxy server)")
		stopItem := systray.AddMenuItem("Stop", "Stop CLIProxyAPI (proxy server)")
		restartItem := systray.AddMenuItem("Restart", "Restart CLIProxyAPI (proxy server)")
		systray.AddSeparator()

		autoOn, _, _ := desktopctl.IsWindowsRunAutostartEnabled(autostartAppName)
		autoStartItem := systray.AddMenuItemCheckbox("Launch on login", "Start this tray app when you log in", autoOn)
		systray.AddSeparator()

		openProxyUI := systray.AddMenuItem("Open Proxy UI", "Open CLIProxyAPI built-in management UI")
		openLogs := systray.AddMenuItem("Open Logs", "Open logs folder")
		systray.AddSeparator()

		quitItem := systray.AddMenuItem("Quit", "Quit")

		refresh := func(lastErr string) {
			st, _ := desktopctl.StatusFor(configPath)
			title := "Stopped"
			if st.Running {
				title = fmt.Sprintf("Running (:%d)", st.Port)
			}
			if lastErr != "" {
				title = title + " — " + shorten(lastErr, 80)
			}
			statusItem.SetTitle("Status: " + title)
			systray.SetTooltip("ProxyPilot — " + title)

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

		refresh("")

		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for range t.C {
				refresh("")
			}
		}()

		go func() {
			for {
				select {
				case <-startItem.ClickedCh:
					_, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
					if err != nil {
						refresh(err.Error())
						continue
					}
					refresh("")
				case <-stopItem.ClickedCh:
					if err := desktopctl.Stop(desktopctl.StopOptions{}); err != nil {
						refresh(err.Error())
						continue
					}
					refresh("")
				case <-restartItem.ClickedCh:
					_, err := desktopctl.Restart(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
					if err != nil {
						refresh(err.Error())
						continue
					}
					refresh("")
				case <-autoStartItem.ClickedCh:
					if autoStartItem.Checked() {
						_ = desktopctl.DisableWindowsRunAutostart(autostartAppName)
						autoStartItem.Uncheck()
						refresh("")
						continue
					}
					cmd, err := autostartCommand(repoRoot, configPath, exePath)
					if err != nil {
						refresh(err.Error())
						continue
					}
					if err := desktopctl.EnableWindowsRunAutostart(autostartAppName, cmd); err != nil {
						refresh(err.Error())
						continue
					}
					autoStartItem.Check()
					refresh("")
				case <-openProxyUI.ClickedCh:
					if err := desktopctl.OpenManagementUI(configPath); err != nil {
						refresh(err.Error())
					}
				case <-openLogs.ClickedCh:
					if err := desktopctl.OpenLogsFolder(repoRoot, configPath); err != nil {
						refresh(err.Error())
					}
				case <-quitItem.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}

func shorten(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func autostartCommand(repoRoot, configPath, exePath string) (string, error) {
	exe, err := os.Executable()
	if err == nil && strings.TrimSpace(exe) != "" {
		exe = filepath.Clean(exe)
	} else {
		exe = ""
	}
	if strings.TrimSpace(exePath) != "" {
		exe = filepath.Clean(exePath)
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
