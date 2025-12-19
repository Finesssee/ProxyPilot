//go:build windows

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/proxypilotupdate"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/trayicon"
)

const autostartAppName = "ProxyPilot"
const thinkingProxyPort = 8317

func main() {
	var repoRoot string
	var configPath string
	var exePath string
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate bin/ and logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.StringVar(&exePath, "exe", "", "Path to ProxyPilot Engine binary (defaults to <repo>/bin/proxypilot-engine.exe)")
	flag.Parse()

	repoRoot, configPath, exePath = applyDefaults(repoRoot, configPath, exePath)
	run(repoRoot, configPath, exePath)
}

func run(repoRoot, configPath, exePath string) {
	systray.Run(func() {
		thinkingProxy := startThinkingProxy(configPath)
		defer thinkingProxy.Close()

		if ico := trayicon.ProxyPilotICO(); len(ico) > 0 {
			systray.SetIcon(ico)
		}
		systray.SetTitle("ProxyPilot")
		systray.SetTooltip("ProxyPilot")

		statusItem := systray.AddMenuItem("Status: ...", "Current status")
		statusItem.Disable()
		systray.AddSeparator()

		openProxyUI := systray.AddMenuItem("Open Dashboard", "Open ProxyPilot Control Center")

		systray.AddSeparator()

		// Engine submenu
		engineMenu := systray.AddMenuItem("Engine", "Engine controls")
		startItem := engineMenu.AddSubMenuItem("Start", "Start the ProxyPilot engine")
		stopItem := engineMenu.AddSubMenuItem("Stop", "Stop the ProxyPilot engine")
		restartItem := engineMenu.AddSubMenuItem("Restart", "Restart the ProxyPilot engine")

		// Settings submenu
		settingsMenu := systray.AddMenuItem("Settings", "ProxyPilot settings")
		autoOn, _, _ := desktopctl.IsWindowsRunAutostartEnabled(autostartAppName)
		autoStartItem := settingsMenu.AddSubMenuItemCheckbox("Launch on login", "Start this tray app when you log in", autoOn)
		autoProxyOn, _ := desktopctl.GetAutoStartProxy()
		autoStartProxyItem := settingsMenu.AddSubMenuItemCheckbox("Auto-start proxy", "Start the proxy server automatically", autoProxyOn)

		// Tools submenu
		toolsMenu := systray.AddMenuItem("Tools", "ProxyPilot tools")
		openLogs := toolsMenu.AddSubMenuItem("Open Logs", "Open logs folder")
		copyDiagnostics := toolsMenu.AddSubMenuItem("Copy Diagnostics", "Copy a support bundle to clipboard")
		openLegacyUI := toolsMenu.AddSubMenuItem("Advanced UI", "Open management UI (advanced)")

		// Updates submenu
		updatesMenu := systray.AddMenuItem("Updates", "Check for updates")
		checkUpdates := updatesMenu.AddSubMenuItem("Check for Updates", "Check GitHub Releases for a newer ProxyPilot")
		updateNow := updatesMenu.AddSubMenuItem("Update ProxyPilot...", "Download and run the latest installer")
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
				case <-openLegacyUI.ClickedCh:
					st, _ := desktopctl.StatusFor(configPath)
					if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
						lastErr = "proxy is not running"
						refresh()
						continue
					}
					_ = desktopctl.OpenBrowser(st.BaseURL + "/management.html")
				case <-openLogs.ClickedCh:
					if err := desktopctl.OpenLogsFolder(repoRoot, configPath); err != nil {
						lastErr = err.Error()
						refresh()
					}
				case <-copyDiagnostics.ClickedCh:
					if err := copyDiagnosticsToClipboard(configPath); err != nil {
						lastErr = err.Error()
					} else {
						lastErr = "Diagnostics copied"
					}
					refresh()
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

type closeFn func() error

func (c closeFn) Close() error { return c() }

func startThinkingProxy(configPath string) ioCloser {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", thinkingProxyPort))
	if err != nil {
		// Best effort: don't crash the tray app if the port is already taken.
		return closeFn(func() error { return nil })
	}

	var (
		mu         sync.Mutex
		lastPort   int
		lastProxy  *httputil.ReverseProxy
		lastTarget *url.URL
	)

	getProxy := func() (*httputil.ReverseProxy, *url.URL) {
		st, _ := desktopctl.StatusFor(configPath)
		port := st.Port
		if port <= 0 {
			port = 8318
		}
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))

		mu.Lock()
		defer mu.Unlock()
		if lastProxy != nil && lastTarget != nil && lastPort == port {
			return lastProxy, lastTarget
		}
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.FlushInterval = 50 * time.Millisecond
		origDirector := rp.Director
		rp.Director = func(r *http.Request) {
			origDirector(r)
			// Preserve original Host header behavior for local forwarding.
			r.Host = target.Host
		}
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"engine unavailable","type":"server_error"}}`))
		}
		lastPort = port
		lastProxy = rp
		lastTarget = target
		return rp, target
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only allow localhost clients to use the thinking proxy.
		host, _, _ := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
		if host != "127.0.0.1" && host != "::1" && !strings.EqualFold(host, "localhost") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		rp, _ := getProxy()
		rp.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()

	return closeFn(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return ln.Close()
	})
}

type ioCloser interface {
	Close() error
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
	st, _ = desktopctl.StatusFor(configPath)
	if strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("proxy base URL not available")
	}
	return desktopctl.OpenBrowser(st.BaseURL + "/proxypilot.html")
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

type authURLResponse struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	State  string `json:"state"`
	Error  string `json:"error"`
}

func startOAuthFlow(configPath string, endpointPath string) error {
	st, _ := desktopctl.StatusFor(configPath)
	if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("proxy is not running")
	}
	key, err := desktopctl.GetManagementPassword()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, st.BaseURL+endpointPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Management-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out authURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if strings.TrimSpace(out.Error) != "" {
			return fmt.Errorf("%s", out.Error)
		}
		return fmt.Errorf("auth url request failed: %s", resp.Status)
	}
	if strings.TrimSpace(out.URL) == "" {
		return fmt.Errorf("missing auth url")
	}

	private, _ := desktopctl.GetOAuthPrivate()
	return openOAuthURL(out.URL, private)
}

func copyDiagnosticsToClipboard(configPath string) error {
	st, _ := desktopctl.StatusFor(configPath)
	if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("proxy is not running")
	}
	key, err := desktopctl.GetManagementPassword()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, st.BaseURL+"/v0/management/proxypilot/diagnostics?lines=200", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Management-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("diagnostics failed: %s", resp.Status)
	}
	if strings.TrimSpace(payload.Text) == "" {
		return fmt.Errorf("empty diagnostics")
	}
	return copyToClipboard(payload.Text)
}

func copyToClipboard(text string) error {
	cmd := exec.Command("cmd", "/c", "clip")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func openOAuthURL(url string, private bool) error {
	if !private {
		return desktopctl.OpenBrowser(url)
	}
	edge, err := findEdge()
	if err != nil {
		return desktopctl.OpenBrowser(url)
	}
	return exec.Command(edge, "--inprivate", url).Start()
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

	// If launched from a repo/app "bin" directory, treat its parent as the root.
	defaultRoot := exeDir
	if strings.EqualFold(filepath.Base(defaultRoot), "bin") {
		defaultRoot = filepath.Dir(defaultRoot)
	}

	if repoRoot == "" && defaultRoot != "" {
		repoRoot = defaultRoot
	}

	if configPath == "" && repoRoot != "" {
		configPath = filepath.Join(repoRoot, "config.yaml")
	}

	if configPath != "" {
		ensureConfig(configPath)
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
