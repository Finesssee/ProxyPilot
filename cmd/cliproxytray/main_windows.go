//go:build windows

package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/getlantern/systray"
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

	run(repoRoot, configPath, exePath)
}

func run(repoRoot, configPath, exePath string) {
	systray.Run(func() {
		systray.SetTitle("CLIProxyAPI")
		systray.SetTooltip("CLIProxyAPI")

		statusItem := systray.AddMenuItem("Status: …", "Current status")
		statusItem.Disable()
		systray.AddSeparator()

		startItem := systray.AddMenuItem("Start", "Start CLIProxyAPI")
		stopItem := systray.AddMenuItem("Stop", "Stop CLIProxyAPI")
		restartItem := systray.AddMenuItem("Restart", "Restart CLIProxyAPI")
		systray.AddSeparator()

		openProxyUI := systray.AddMenuItem("Open Proxy UI", "Open the built-in management UI")
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
			systray.SetTooltip("CLIProxyAPI — " + title)

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
					st, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
					if err != nil {
						refresh(err.Error())
						continue
					}
					_ = st
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
