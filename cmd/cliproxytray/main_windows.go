//go:build windows

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
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
	"github.com/jchv/go-webview2"
	"github.com/router-for-me/CLIProxyAPI/v6/cmd/proxypilotui/assets"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/trayicon"
)

const autostartAppName = "ProxyPilot"
const thinkingProxyPort = 8317

var assetServerURL string

func main() {
	// Start embedded asset server for the dashboard UI
	assetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		assetServerURL = "http://" + assetLn.Addr().String()
		go func() {
			fsys, _ := fs.Sub(assets.FS, ".")
			http.Serve(assetLn, http.FileServer(http.FS(fsys)))
		}()
	}

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

		// Minimal tray menu
		openDashboard := systray.AddMenuItem("Open Dashboard", "Open ProxyPilot Dashboard")
		systray.AddSeparator()

		toggleItem := systray.AddMenuItem("Start", "Start/Stop proxy")
		copyURLItem := systray.AddMenuItem("Copy API URL", "Copy http://127.0.0.1:8317/v1")
		systray.AddSeparator()

		quitItem := systray.AddMenuItem("Quit", "Quit ProxyPilot")

		// Auto-start proxy on launch if enabled
		autoProxyOn, _ := desktopctl.GetAutoStartProxy()
		if autoProxyOn {
			go func() {
				st, _ := desktopctl.StatusFor(configPath)
				if !st.Running {
					desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
				}
			}()
		}

		// Update UI based on status
		refresh := func() {
			st, _ := desktopctl.StatusFor(configPath)
			if st.Running {
				systray.SetTooltip(fmt.Sprintf("ProxyPilot - Running (:%d)", st.Port))
				toggleItem.SetTitle("Stop")
				toggleItem.SetTooltip("Stop the proxy")
			} else {
				systray.SetTooltip("ProxyPilot - Stopped")
				toggleItem.SetTitle("Start")
				toggleItem.SetTooltip("Start the proxy")
			}
		}
		refresh()

		// Refresh status periodically
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for range t.C {
				refresh()
			}
		}()

		// Handle clicks
		go func() {
			for {
				select {
				case <-openDashboard.ClickedCh:
					openProxyUIWithAutostart(repoRoot, configPath, exePath)
				case <-toggleItem.ClickedCh:
					st, _ := desktopctl.StatusFor(configPath)
					if st.Running {
						desktopctl.Stop(desktopctl.StopOptions{})
					} else {
						desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
					}
					refresh()
				case <-copyURLItem.ClickedCh:
					copyToClipboard(fmt.Sprintf("http://127.0.0.1:%d/v1", thinkingProxyPort))
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
	// Start proxy if not running
	st, _ := desktopctl.StatusFor(configPath)
	if !st.Running {
		if _, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath}); err != nil {
			// Continue anyway to show UI
		}
	}

	// Open embedded WebView2 dashboard
	go func() {
		openEmbeddedDashboard(configPath)
	}()
	return nil
}

func openEmbeddedDashboard(configPath string) {
	target := assetServerURL + "/index.html"
	if assetServerURL == "" {
		// Fallback to browser if asset server failed
		st, _ := desktopctl.StatusFor(configPath)
		if st.Running && strings.TrimSpace(st.BaseURL) != "" {
			_ = desktopctl.OpenBrowser(st.BaseURL + "/proxypilot.html")
		}
		return
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     true,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "ProxyPilot",
			Width:  1200,
			Height: 850,
			Center: true,
		},
	})
	if w == nil {
		// Fallback to browser
		_ = desktopctl.OpenBrowser(target)
		return
	}
	defer w.Destroy()

	// Bind desktop functions for the React UI
	_ = w.Bind("pp_status", func() (map[string]any, error) {
		cur, _ := desktopctl.StatusFor(configPath)
		return map[string]any{
			"running":    cur.Running,
			"port":       cur.Port,
			"base_url":   cur.BaseURL,
			"last_error": cur.LastError,
		}, nil
	})
	_ = w.Bind("pp_start", func() error {
		_, err := desktopctl.Start(desktopctl.StartOptions{ConfigPath: configPath})
		return err
	})
	_ = w.Bind("pp_stop", func() error { return desktopctl.Stop(desktopctl.StopOptions{}) })
	_ = w.Bind("pp_restart", func() error {
		_, err := desktopctl.Restart(desktopctl.StartOptions{ConfigPath: configPath})
		return err
	})
	_ = w.Bind("pp_open_logs", func() error { return desktopctl.OpenLogsFolder("", configPath) })
	_ = w.Bind("pp_open_auth_folder", func() error {
		dir, err := desktopctl.AuthDirFor(configPath)
		if err != nil {
			return err
		}
		return desktopctl.OpenFolder(dir)
	})
	_ = w.Bind("pp_get_oauth_private", func() (bool, error) { return desktopctl.GetOAuthPrivate() })
	_ = w.Bind("pp_set_oauth_private", func(enabled bool) error { return desktopctl.SetOAuthPrivate(enabled) })
	_ = w.Bind("pp_oauth", func(provider string) error { return startOAuthFlow(configPath, getOAuthEndpoint(provider)) })
	_ = w.Bind("pp_copy_diagnostics", func() error { return copyDiagnosticsToClipboard(configPath) })
	_ = w.Bind("pp_get_management_key", func() (string, error) { return desktopctl.GetManagementPassword() })
	_ = w.Bind("pp_open_legacy_ui", func() error {
		cur, _ := desktopctl.StatusFor(configPath)
		if !cur.Running {
			return fmt.Errorf("proxy not running")
		}
		return desktopctl.OpenBrowser(cur.BaseURL + "/management.html?legacy=1")
	})
	_ = w.Bind("pp_open_diagnostics", func() error {
		cur, _ := desktopctl.StatusFor(configPath)
		if !cur.Running {
			return fmt.Errorf("proxy not running")
		}
		return desktopctl.OpenBrowser(cur.BaseURL + "/proxypilot.html")
	})

	w.Navigate(target)
	w.Run()
}

func getOAuthEndpoint(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "antigravity":
		return "/v0/management/antigravity-auth-url"
	case "gemini-cli", "geminicli":
		return "/v0/management/gemini-cli-auth-url"
	case "codex":
		return "/v0/management/codex-auth-url"
	case "claude", "anthropic":
		return "/v0/management/anthropic-auth-url"
	case "qwen":
		return "/v0/management/qwen-auth-url"
	case "iflow":
		return "/v0/management/iflow-auth-url"
	default:
		return ""
	}
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
