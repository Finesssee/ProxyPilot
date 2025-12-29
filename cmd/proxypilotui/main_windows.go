//go:build windows

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
	"github.com/router-for-me/CLIProxyAPI/v6/cmd/proxypilotui/assets"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"
)

func main() {
	var repoRoot string
	var configPath string
	var exePath string
	var url string
	var standalone bool
	var legacyMode bool
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate bin/ and logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.StringVar(&exePath, "exe", "", "Path to proxy engine binary (legacy mode only)")
	flag.StringVar(&url, "url", "", "Open a specific URL in-app (advanced)")
	flag.BoolVar(&standalone, "standalone", false, "Run standalone UI without connecting to proxy server")
	flag.BoolVar(&legacyMode, "legacy", false, "Use legacy subprocess mode (separate engine binary)")
	flag.Parse()

	// Enable embedded mode by default (single binary)
	if !legacyMode {
		desktopctl.SetEmbeddedMode(true)
		logging.SetupBaseLogger()
	}

	repoRoot, configPath, exePath = applyDefaults(repoRoot, configPath, exePath)

	var assetServerURL string
	if standalone {
		// Start embedded asset server
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			_ = messageBox("ProxyPilot", "Failed to start asset server: "+err.Error())
			os.Exit(1)
		}
		assetServerURL = "http://" + ln.Addr().String()
		go func() {
			fsys, _ := fs.Sub(assets.FS, ".")
			http.Serve(ln, http.FileServer(http.FS(fsys)))
		}()
	} else {
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
	}

	if strings.TrimSpace(url) == "" {
		url = "control"
	}

	var target string
	if standalone {
		if strings.EqualFold(strings.TrimSpace(url), "control") {
			target = assetServerURL + "/index.html"
		} else if strings.HasPrefix(strings.TrimSpace(url), "http://") || strings.HasPrefix(strings.TrimSpace(url), "https://") {
			target = strings.TrimSpace(url)
		} else {
			target = assetServerURL + "/index.html"
		}
	} else {
		st, _ := desktopctl.StatusFor(configPath)
		if strings.EqualFold(strings.TrimSpace(url), "control") {
			target = st.BaseURL + "/proxypilot.html"
		} else if strings.HasPrefix(strings.TrimSpace(url), "http://") || strings.HasPrefix(strings.TrimSpace(url), "https://") {
			target = strings.TrimSpace(url)
		}
	}

	// Prefer an in-app window (WebView2) so this behaves like a native dashboard app.
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
		// Fallback: Edge app mode or default browser.
		if target == "" {
			if standalone {
				target = assetServerURL + "/index.html"
			} else {
				st, _ := desktopctl.StatusFor(configPath)
				target = st.BaseURL + "/proxypilot.html"
			}
		}
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

	// Bindings used by the Control Center UI.
	_ = w.Bind("pp_status", func() (map[string]any, error) {
		cur, _ := desktopctl.StatusFor(configPath)
		return map[string]any{
			"running":          cur.Running,
			"version":          cur.Version,
			"port":             cur.Port,
			"thinking_port":    cur.ThinkingPort,
			"thinking_running": cur.ThinkingRunning,
			"base_url":         cur.BaseURL,
			"last_error":       cur.LastError,
		}, nil
	})
	_ = w.Bind("pp_start", func() error {
		_, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
		return err
	})
	_ = w.Bind("pp_stop", func() error { return desktopctl.Stop(desktopctl.StopOptions{}) })
	_ = w.Bind("pp_restart", func() error {
		_, err := desktopctl.Restart(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
		return err
	})
	_ = w.Bind("pp_open_logs", func() error { return desktopctl.OpenLogsFolder(repoRoot, configPath) })
	_ = w.Bind("pp_open_auth_folder", func() error {
		dir, err := desktopctl.AuthDirFor(configPath)
		if err != nil {
			return err
		}
		return desktopctl.OpenFolder(dir)
	})
	_ = w.Bind("pp_open_legacy_ui", func() error {
		cur, _ := desktopctl.StatusFor(configPath)
		if !cur.Running || strings.TrimSpace(cur.BaseURL) == "" {
			return fmt.Errorf("proxy is not running")
		}
		return openNewWindow(repoRoot, configPath, exePath, cur.BaseURL+"/management.html?legacy=1")
	})
	_ = w.Bind("pp_open_diagnostics", func() error {
		cur, _ := desktopctl.StatusFor(configPath)
		if !cur.Running || strings.TrimSpace(cur.BaseURL) == "" {
			return fmt.Errorf("proxy is not running")
		}
		return openNewWindow(repoRoot, configPath, exePath, cur.BaseURL+"/proxypilot.html")
	})
	_ = w.Bind("pp_get_oauth_private", func() (bool, error) { return desktopctl.GetOAuthPrivate() })
	_ = w.Bind("pp_set_oauth_private", func(enabled bool) error { return desktopctl.SetOAuthPrivate(enabled) })
	_ = w.Bind("pp_oauth", func(provider string) error {
		endpoint := ""
		switch strings.ToLower(strings.TrimSpace(provider)) {
		case "antigravity":
			endpoint = "/v0/management/antigravity-auth-url"
		case "gemini-cli", "geminicli":
			endpoint = "/v0/management/gemini-cli-auth-url"
		case "codex":
			endpoint = "/v0/management/codex-auth-url"
		case "claude", "anthropic":
			endpoint = "/v0/management/anthropic-auth-url"
		case "qwen":
			endpoint = "/v0/management/qwen-auth-url"
		case "kiro":
			endpoint = "/v0/management/kiro-auth-url"
		default:
			return fmt.Errorf("unknown provider: %s", provider)
		}
		return startOAuthFlow(configPath, endpoint)
	})
	_ = w.Bind("pp_import_amazonq", func() error {
		return importAmazonQToken(configPath)
	})
	_ = w.Bind("pp_save_api_key", func(provider, apiKey string) error {
		return saveAPIKey(configPath, provider, apiKey)
	})
	_ = w.Bind("pp_copy_diagnostics", func() error { return copyDiagnosticsToClipboard(configPath) })
	_ = w.Bind("pp_get_management_key", func() (string, error) { return desktopctl.GetManagementPassword() })

	_ = w.Bind("pp_check_updates", func() (map[string]any, error) {
		st, _ := desktopctl.StatusFor(configPath)
		if !st.Running {
			return nil, fmt.Errorf("proxy not running")
		}
		key, _ := desktopctl.GetManagementPassword()
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("GET", st.BaseURL+"/v0/management/updates/check", nil)
		req.Header.Set("X-Management-Key", key)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var res map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}
		return res, nil
	})

	_ = w.Bind("pp_download_update", func(url string) error {
		return desktopctl.OpenBrowser(url)
	})

	// Integration bindings
	_ = w.Bind("pp_get_integrations", func() ([]map[string]any, error) {
		st, _ := desktopctl.StatusFor(configPath)
		if !st.Running {
			return nil, fmt.Errorf("proxy not running")
		}
		key, _ := desktopctl.GetManagementPassword()
		client := &http.Client{Timeout: 5 * time.Second}
		req, _ := http.NewRequest("GET", st.BaseURL+"/v0/management/integrations", nil)
		req.Header.Set("X-Management-Key", key)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var res struct {
			Integrations []map[string]any `json:"integrations"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}
		return res.Integrations, nil
	})

	_ = w.Bind("pp_configure_integration", func(id string) error {
		st, _ := desktopctl.StatusFor(configPath)
		if !st.Running {
			return fmt.Errorf("proxy not running")
		}
		key, _ := desktopctl.GetManagementPassword()
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("POST", st.BaseURL+"/v0/management/integrations/"+id, nil)
		req.Header.Set("X-Management-Key", key)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed: %s", string(b))
		}
		return nil
	})

	// Agent detection bindings
	_ = w.Bind("pp_detect_agents", func() ([]map[string]any, error) {
		agents := detectCLIAgents()
		return agents, nil
	})

	_ = w.Bind("pp_configure_agent", func(agentID string) error {
		st, _ := desktopctl.StatusFor(configPath)
		if !st.Running {
			return fmt.Errorf("proxy not running")
		}
		key, _ := desktopctl.GetManagementPassword()
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("POST", st.BaseURL+"/v0/management/integrations/"+agentID, nil)
		req.Header.Set("X-Management-Key", key)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed: %s", string(b))
		}
		return nil
	})

	// Model mappings bindings
	_ = w.Bind("pp_get_model_mappings", func() ([]map[string]any, error) {
		return getModelMappings(), nil
	})

	_ = w.Bind("pp_save_model_mappings", func(mappings []map[string]any) error {
		return saveModelMappings(mappings)
	})

	if target != "" {
		w.Navigate(target)
		w.Run()
		return
	}

	w.SetHtml(controlCenterHTML())
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

func openNewWindow(repoRoot, configPath, exePath, url string) error {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return desktopctl.OpenBrowser(url)
	}
	args := []string{"-url", url}
	if strings.TrimSpace(repoRoot) != "" {
		args = append(args, "-repo", repoRoot)
	}
	if strings.TrimSpace(configPath) != "" {
		args = append(args, "-config", configPath)
	}
	if strings.TrimSpace(exePath) != "" {
		args = append(args, "-exe", exePath)
	}
	return exec.Command(exe, args...).Start()
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

func importAmazonQToken(configPath string) error {
	st, _ := desktopctl.StatusFor(configPath)
	if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("proxy is not running")
	}
	key, err := desktopctl.GetManagementPassword()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, st.BaseURL+"/v0/management/amazonq-import", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Management-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if errMsg, ok := out["error"].(string); ok && errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}
		if msg, ok := out["message"].(string); ok && msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("import failed: %s", resp.Status)
	}
	return nil
}

func saveAPIKey(configPath, provider, apiKey string) error {
	st, _ := desktopctl.StatusFor(configPath)
	if !st.Running || strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("proxy is not running")
	}
	key, err := desktopctl.GetManagementPassword()
	if err != nil {
		return err
	}

	var endpoint string
	switch provider {
	case "minimax":
		endpoint = "/v0/management/minimax-api-key"
	case "zhipu":
		endpoint = "/v0/management/zhipu-api-key"
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}

	body := fmt.Sprintf(`{"api_key":"%s"}`, apiKey)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodPost, st.BaseURL+endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Management-Key", key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if errMsg, ok := out["error"].(string); ok && errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("save failed: %s", resp.Status)
	}
	return nil
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("diagnostics failed: %s", msg)
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
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

func controlCenterHTML() string {
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>ProxyPilot</title>
  <style>
    :root{
      --bg:#0b0f17;
      --panel:#0f172a;
      --panel2:#111c33;
      --text:#e5e7eb;
      --muted:#9ca3af;
      --line:#1f2a44;
      --brand:#60a5fa;
      --good:#34d399;
      --bad:#f87171;
      --warn:#fbbf24;
      --btn:#1f2a44;
      --btn2:#243152;
    }
    *{box-sizing:border-box}
    body{
      margin:0;
      font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial;
      background: radial-gradient(1000px 600px at 20% 0%, rgba(96,165,250,.18), transparent 60%),
                  radial-gradient(1000px 700px at 80% 10%, rgba(52,211,153,.12), transparent 55%),
                  var(--bg);
      color:var(--text);
    }
    .wrap{max-width:1040px;margin:0 auto;padding:28px 22px 40px}
    .top{display:flex;gap:14px;align-items:center;justify-content:space-between;margin-bottom:18px}
    .brand{display:flex;align-items:center;gap:12px}
    .logo{
      width:42px;height:42px;border-radius:12px;
      background: linear-gradient(135deg, rgba(96,165,250,.95), rgba(34,211,238,.75));
      box-shadow: 0 10px 30px rgba(96,165,250,.18);
    }
    h1{font-size:22px;margin:0}
    .sub{font-size:13px;color:var(--muted);margin-top:3px}
    .pill{
      display:inline-flex;align-items:center;gap:8px;
      padding:8px 12px;border:1px solid var(--line);border-radius:999px;
      background: rgba(15,23,42,.65);
      font-size:13px;color:var(--muted)
    }
    .dot{width:9px;height:9px;border-radius:50%}
    .card{
      border:1px solid var(--line);
      background: linear-gradient(180deg, rgba(15,23,42,.92), rgba(15,23,42,.78));
      border-radius:16px;
      padding:16px;
      box-shadow: 0 20px 60px rgba(0,0,0,.35);
    }
    .grid{display:grid;grid-template-columns:1.4fr .9fr;gap:16px}
    @media (max-width: 980px){.grid{grid-template-columns:1fr}}
    .row{display:flex;flex-wrap:wrap;gap:10px}
    button,.btn{
      appearance:none;border:1px solid var(--line);background:var(--btn);color:var(--text);
      border-radius:12px;padding:10px 12px;font-size:13px;cursor:pointer;
      transition: transform .06s ease, background .12s ease, border-color .12s ease;
      user-select:none;
    }
    button:hover{background:var(--btn2);border-color:#2c3a60}
    button:active{transform: translateY(1px)}
    button[disabled]{opacity:.45;cursor:not-allowed}
    .btnPrimary{background: rgba(96,165,250,.18);border-color: rgba(96,165,250,.45)}
    .btnPrimary:hover{background: rgba(96,165,250,.25);border-color: rgba(96,165,250,.65)}
    .btnDanger{background: rgba(248,113,113,.14);border-color: rgba(248,113,113,.4)}
    .btnDanger:hover{background: rgba(248,113,113,.2);border-color: rgba(248,113,113,.6)}
    .btnGood{background: rgba(52,211,153,.14);border-color: rgba(52,211,153,.38)}
    .btnGood:hover{background: rgba(52,211,153,.2);border-color: rgba(52,211,153,.55)}
    .sectionTitle{font-size:13px;color:var(--muted);margin:0 0 10px}
    .kv{display:grid;grid-template-columns:120px 1fr;gap:8px 10px;font-size:13px;margin-top:8px}
    .k{color:var(--muted)}
    .v{color:var(--text);word-break:break-word}
    .note{font-size:12px;color:var(--muted);margin-top:10px;line-height:1.4}
    .toggle{display:flex;align-items:center;gap:10px;margin-top:10px}
    input[type="checkbox"]{width:16px;height:16px}
    .toast{position:fixed;left:22px;bottom:18px;padding:10px 12px;border-radius:12px;border:1px solid var(--line);background:rgba(15,23,42,.88);color:var(--text);font-size:13px;display:none;max-width: min(620px, calc(100vw - 44px));}
    .toast.show{display:block}
    .hint{font-size:12px;color:var(--muted);margin-top:8px}
  </style>
</head>
<body>
  <div class="wrap">
    <div class="top">
      <div class="brand">
        <div class="logo"></div>
        <div>
          <h1>ProxyPilot</h1>
          <div class="sub">Local proxy controller + dashboard (Windows)</div>
        </div>
      </div>
      <div class="pill" id="statusPill">
        <span class="dot" id="statusDot" style="background: var(--warn)"></span>
        <span id="statusText">Checking…</span>
      </div>
    </div>

    <div class="grid">
      <div class="card">
        <div class="sectionTitle">Engine</div>
        <div class="row">
          <button class="btnGood" id="startBtn">Start</button>
          <button class="btnDanger" id="stopBtn">Stop</button>
          <button id="restartBtn">Restart</button>
          <button class="btnPrimary" id="diagBtn">Open Diagnostics</button>
          <button id="legacyBtn">Advanced UI</button>
        </div>
        <div class="kv">
          <div class="k">Base URL</div><div class="v" id="baseUrl">-</div>
          <div class="k">Port</div><div class="v" id="port">-</div>
          <div class="k">Thinking Port</div><div class="v" id="thinkingPort">-</div>
          <div class="k">Last error</div><div class="v" id="lastErr">-</div>
        </div>
        <div class="row" style="margin-top:12px">
          <button id="logsBtn">Open Logs Folder</button>
          <button id="authBtn">Open Auth Folder</button>
          <button id="copyDiagBtn">Copy Diagnostics</button>
        </div>
        <div class="toggle">
          <input id="privateOAuth" type="checkbox" />
          <label for="privateOAuth">Open OAuth in a private (InPrivate) window</label>
        </div>
        <div class="note">
          Tip: keep the tray menu minimal — use this Control Center for day-to-day actions.
        </div>
      </div>

      <div class="card">
        <div class="sectionTitle">Logins</div>
        <div class="row">
          <button id="oauthAntigravity">Antigravity</button>
          <button id="oauthGeminiCli">Gemini CLI</button>
          <button id="oauthCodex">Codex</button>
          <button id="oauthClaude">Claude</button>
          <button id="oauthQwen">Qwen</button>
          <button id="oauthKiro">Kiro</button>
          <button id="importAmazonQ">Amazon Q</button>
          <button id="apiKeyMiniMax">MiniMax</button>
          <button id="apiKeyZhipu">Zhipu</button>
        </div>
        <div class="hint">OAuth providers open browser login. Amazon Q imports from CLI. MiniMax/Zhipu require API keys.</div>
      </div>

      <div class="card" style="grid-column: 1 / -1">
        <div class="sectionTitle">CLI Agents</div>
        <div class="row" id="agentsList">
          <div style="color:var(--muted);font-size:13px">Detecting agents...</div>
        </div>
        <div class="note">
          Detected CLI agents. Click to configure for ProxyPilot.
        </div>
      </div>

      <div class="card" style="grid-column: 1 / -1">
        <div class="sectionTitle">Model Mappings</div>
        <div id="mappingsList" style="margin-bottom:12px"></div>
        <div class="row" style="gap:8px;flex-wrap:wrap">
          <input type="text" id="mapFrom" placeholder="Alias (e.g. smart)" style="flex:1;min-width:120px;padding:8px 10px;background:rgba(0,0,0,.3);border:1px solid var(--line);border-radius:8px;color:var(--text);font-size:13px" />
          <input type="text" id="mapTo" placeholder="Model (e.g. claude-opus-4-5)" style="flex:2;min-width:180px;padding:8px 10px;background:rgba(0,0,0,.3);border:1px solid var(--line);border-radius:8px;color:var(--text);font-size:13px" />
          <button id="addMappingBtn" class="btnPrimary">Add</button>
        </div>
        <div class="note">
          Route friendly model names to actual models.
        </div>
      </div>

      <div class="card" style="grid-column: 1 / -1">
        <div class="sectionTitle">Integrations</div>
        <div class="row" id="integrationsList">
          <div style="color:var(--muted);font-size:13px">Loading integrations...</div>
        </div>
        <div class="note">
          Automatically configure local tools to use ProxyPilot.
        </div>
      </div>

      <div class="card" style="grid-column: 1 / -1">
        <div class="sectionTitle">Live Logs</div>
        <div id="logContent" style="height:240px;overflow-y:auto;background:rgba(0,0,0,.3);padding:10px;font-family:monospace;font-size:12px;border-radius:8px;border:1px solid var(--line);white-space:pre-wrap;color:var(--muted)">Waiting for logs...</div>
        <div class="row" style="margin-top:10px">
           <button id="toggleLogsBtn">Pause Updates</button>
           <button id="clearLogsBtn">Clear View</button>
        </div>
      </div>
    </div>
  </div>

  <div class="toast" id="toast"></div>
  <script>
    const $ = (id) => document.getElementById(id);
    let logPaused = false;
    const toast = (msg) => {
      const el = $('toast');
      el.textContent = msg;
      el.classList.add('show');
      clearTimeout(window.__toastTimer);
      window.__toastTimer = setTimeout(()=> el.classList.remove('show'), 2400);
    };

    const setRunningUI = (running) => {
      $('startBtn').disabled = !!running;
      $('stopBtn').disabled = !running;
      $('restartBtn').disabled = !running;
      $('diagBtn').disabled = !running;
      $('legacyBtn').disabled = !running;
      const dot = $('statusDot');
      dot.style.background = running ? 'var(--good)' : 'var(--warn)';
      if(running) refreshIntegrations();
    };

    async function refreshStatus() {
      try{
        const s = await window.pp_status();
        $('baseUrl').textContent = s.base_url || '-';
        $('port').textContent = s.port ? String(s.port) : '-';
        
        const tp = $('thinkingPort');
        if (s.thinking_port) {
          tp.textContent = s.thinking_port + (s.thinking_running ? ' (Active)' : ' (Inactive)');
          tp.style.color = s.thinking_running ? 'var(--good)' : 'var(--muted)';
        } else {
          tp.textContent = '-';
          tp.style.color = 'var(--muted)';
        }

        $('lastErr').textContent = (s.last_error && s.last_error.trim()) ? s.last_error : '-';
        $('statusText').textContent = s.running ? ('Running (:' + s.port + ')') : 'Stopped';
        setRunningUI(!!s.running);
        if(!logPaused && s.running) refreshLogs();
      }catch(e){
        $('statusText').textContent = 'Error';
        $('lastErr').textContent = (e && e.message) ? e.message : String(e);
        setRunningUI(false);
      }
    }

    async function refreshLogs() {
      const logEl = $('logContent');
      const key = await window.pp_get_management_key();
      const s = await window.pp_status();
      if(!s.base_url) return;
      try {
        const resp = await fetch(s.base_url + '/v0/management/proxypilot/logs/tail?file=stdout&lines=50', {
          headers: { 'X-Management-Key': key }
        });
        const data = await resp.json();
        if(data.lines) {
          logEl.textContent = data.lines.join('\n');
          logEl.scrollTop = logEl.scrollHeight;
        }
      } catch(e) {}
    }

    async function refreshIntegrations() {
      const list = $('integrationsList');
      try {
        const items = await window.pp_get_integrations();
        if(!items || items.length === 0) {
          list.innerHTML = '<div style="color:var(--muted);font-size:13px">No integrations available</div>';
          return;
        }
        list.innerHTML = '';
        items.forEach(i => {
          const btn = document.createElement('button');
          btn.className = i.configured ? 'btnGood' : 'btn';
          btn.textContent = i.name + (i.configured ? ' (Active)' : '');
          btn.onclick = async () => {
            try {
              await window.pp_configure_integration(i.id);
              toast('Configured ' + i.name);
              refreshIntegrations();
            } catch(e) { toast(e.message||String(e)); }
          };
          if(!i.installed) {
             btn.disabled = true;
             btn.textContent += ' (Not Installed)';
          }
          list.appendChild(btn);
        });
      } catch(e) {
        // quiet fail if proxy not ready
      }
    }

    async function refreshAgents() {
      const list = $('agentsList');
      try {
        const agents = await window.pp_detect_agents();
        if(!agents || agents.length === 0) {
          list.innerHTML = '<div style="color:var(--muted);font-size:13px">No agents detected</div>';
          return;
        }
        list.innerHTML = '';
        agents.forEach(a => {
          const btn = document.createElement('button');
          if (!a.detected) {
            btn.className = 'btn';
            btn.disabled = true;
            btn.textContent = a.name + ' (Not Found)';
          } else if (a.configured) {
            btn.className = 'btnGood';
            btn.textContent = a.name + ' (Configured)';
          } else {
            btn.className = 'btnPrimary';
            btn.textContent = a.name + ' (Configure)';
          }
          btn.onclick = async () => {
            if (!a.detected) return;
            try {
              await window.pp_configure_agent(a.id);
              toast('Configured ' + a.name);
              refreshAgents();
            } catch(e) { toast(e.message||String(e)); }
          };
          list.appendChild(btn);
        });
      } catch(e) {
        list.innerHTML = '<div style="color:var(--muted);font-size:13px">Error: ' + (e.message||e) + '</div>';
      }
    }

    async function refreshMappings() {
      const list = $('mappingsList');
      try {
        const mappings = await window.pp_get_model_mappings();
        if(!mappings || mappings.length === 0) {
          list.innerHTML = '<div style="color:var(--muted);font-size:13px">No mappings configured</div>';
          return;
        }
        list.innerHTML = '<div style="display:flex;flex-direction:column;gap:6px">' +
          mappings.map((m, i) =>
            '<div style="display:flex;align-items:center;gap:8px;padding:8px 10px;background:rgba(0,0,0,.25);border-radius:6px;font-size:13px">' +
              '<code style="color:var(--brand)">' + m.from + '</code>' +
              '<span style="color:var(--muted)">&rarr;</span>' +
              '<code style="color:var(--good)">' + m.to + '</code>' +
              '<button class="btnDanger" style="margin-left:auto;padding:4px 8px;font-size:11px" data-idx="' + i + '">Remove</button>' +
            '</div>'
          ).join('') + '</div>';
        list.querySelectorAll('button[data-idx]').forEach(btn => {
          btn.onclick = async () => {
            const idx = parseInt(btn.dataset.idx);
            const updated = mappings.filter((_, i) => i !== idx);
            try {
              await window.pp_save_model_mappings(updated);
              toast('Mapping removed');
              refreshMappings();
            } catch(e) { toast(e.message||String(e)); }
          };
        });
      } catch(e) {
        list.innerHTML = '<div style="color:var(--muted);font-size:13px">Error loading mappings</div>';
      }
    }

    async function init() {
      try{
        const priv = await window.pp_get_oauth_private();
        $('privateOAuth').checked = !!priv;
      }catch(e){}
      await refreshStatus();
      await refreshAgents();
      await refreshMappings();
      setInterval(refreshStatus, 1200);
    }

    $('startBtn').addEventListener('click', async () => { try{ await window.pp_start(); toast('Started'); }catch(e){ toast(e.message||String(e)); } await refreshStatus(); });
    $('stopBtn').addEventListener('click', async () => { try{ await window.pp_stop(); toast('Stopped'); }catch(e){ toast(e.message||String(e)); } await refreshStatus(); });
    $('restartBtn').addEventListener('click', async () => { try{ await window.pp_restart(); toast('Restarted'); }catch(e){ toast(e.message||String(e)); } await refreshStatus(); });
    $('logsBtn').addEventListener('click', async () => { try{ await window.pp_open_logs(); toast('Opened logs'); }catch(e){ toast(e.message||String(e)); } });
    $('authBtn').addEventListener('click', async () => { try{ await window.pp_open_auth_folder(); toast('Opened auth folder'); }catch(e){ toast(e.message||String(e)); } });
    $('copyDiagBtn').addEventListener('click', async () => { try{ await window.pp_copy_diagnostics(); toast('Copied'); }catch(e){ toast(e.message||String(e)); } });
    $('diagBtn').addEventListener('click', async () => { try{ await window.pp_open_diagnostics(); }catch(e){ toast(e.message||String(e)); } });
    $('legacyBtn').addEventListener('click', async () => { try{ await window.pp_open_legacy_ui(); }catch(e){ toast(e.message||String(e)); } });

    $('toggleLogsBtn').addEventListener('click', () => {
      logPaused = !logPaused;
      $('toggleLogsBtn').textContent = logPaused ? 'Resume Updates' : 'Pause Updates';
    });
    $('clearLogsBtn').addEventListener('click', () => { $('logContent').textContent = ''; });

    $('privateOAuth').addEventListener('change', async (ev) => {
      try{ await window.pp_set_oauth_private(!!ev.target.checked); toast('Saved'); }catch(e){ toast(e.message||String(e)); }
    });

    const oauth = (provider) => async () => { try{ await window.pp_oauth(provider); toast('Opened login'); }catch(e){ toast(e.message||String(e)); } };
    $('oauthAntigravity').addEventListener('click', oauth('antigravity'));
    $('oauthGeminiCli').addEventListener('click', oauth('gemini-cli'));
    $('oauthCodex').addEventListener('click', oauth('codex'));
    $('oauthClaude').addEventListener('click', oauth('claude'));
    $('oauthQwen').addEventListener('click', oauth('qwen'));
    $('oauthKiro').addEventListener('click', oauth('kiro'));
    $('importAmazonQ').addEventListener('click', async () => {
      try { await window.pp_import_amazonq(); toast('Amazon Q token imported'); } catch(e) { toast(e.message||String(e)); }
    });
    $('apiKeyMiniMax').addEventListener('click', async () => {
      const key = prompt('Enter your MiniMax API key:');
      if (!key) return;
      try { await window.pp_save_api_key('minimax', key); toast('MiniMax API key saved'); } catch(e) { toast(e.message||String(e)); }
    });
    $('apiKeyZhipu').addEventListener('click', async () => {
      const key = prompt('Enter your Zhipu AI API key:');
      if (!key) return;
      try { await window.pp_save_api_key('zhipu', key); toast('Zhipu API key saved'); } catch(e) { toast(e.message||String(e)); }
    });

    $('addMappingBtn').addEventListener('click', async () => {
      const from = $('mapFrom').value.trim();
      const to = $('mapTo').value.trim();
      if (!from || !to) { toast('Both fields required'); return; }
      try {
        const existing = await window.pp_get_model_mappings() || [];
        existing.push({ from, to, enabled: true });
        await window.pp_save_model_mappings(existing);
        $('mapFrom').value = '';
        $('mapTo').value = '';
        toast('Mapping added');
        refreshMappings();
      } catch(e) { toast(e.message||String(e)); }
    });

    init();
  </script>
</body>
</html>`
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

// detectCLIAgents returns a list of detected CLI agents
func detectCLIAgents() []map[string]any {
	agents := []map[string]any{}

	// Claude Code
	claude := map[string]any{
		"id":         "claude",
		"name":       "Claude Code",
		"detected":   false,
		"configured": false,
	}
	if path, err := exec.LookPath("claude"); err == nil {
		claude["detected"] = true
		claude["binary_path"] = path
	}
	home, _ := os.UserHomeDir()
	claudeConfig := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.Stat(claudeConfig); err == nil {
		claude["detected"] = true
		claude["config_path"] = claudeConfig
		if content, err := os.ReadFile(claudeConfig); err == nil {
			if strings.Contains(string(content), "127.0.0.1:8317") || strings.Contains(string(content), "127.0.0.1:8318") {
				claude["configured"] = true
			}
		}
	}
	agents = append(agents, claude)

	// Codex CLI
	codex := map[string]any{
		"id":         "codex",
		"name":       "Codex CLI",
		"detected":   false,
		"configured": false,
	}
	if path, err := exec.LookPath("codex"); err == nil {
		codex["detected"] = true
		codex["binary_path"] = path
	}
	codexDir := filepath.Join(home, ".codex")
	if info, err := os.Stat(codexDir); err == nil && info.IsDir() {
		codex["detected"] = true
		codex["config_path"] = codexDir
		codexConfig := filepath.Join(codexDir, "config.toml")
		if content, err := os.ReadFile(codexConfig); err == nil {
			if strings.Contains(string(content), "127.0.0.1:8317") || strings.Contains(string(content), "127.0.0.1:8318") {
				codex["configured"] = true
			}
		}
	}
	agents = append(agents, codex)

	// Factory Droid
	droid := map[string]any{
		"id":         "droid",
		"name":       "Factory Droid",
		"detected":   false,
		"configured": false,
	}
	if path, err := exec.LookPath("droid"); err == nil {
		droid["detected"] = true
		droid["binary_path"] = path
	} else if path, err := exec.LookPath("factory"); err == nil {
		droid["detected"] = true
		droid["binary_path"] = path
	}
	// Droid uses settings.json for runtime config (not config.json)
	droidSettings := filepath.Join(home, ".factory", "settings.json")
	if _, err := os.Stat(droidSettings); err == nil {
		droid["config_path"] = droidSettings
		droid["detected"] = true
		if content, err := os.ReadFile(droidSettings); err == nil {
			// Check for ProxyPilot models in customModels array
			if strings.Contains(string(content), "ProxyPilot") || strings.Contains(string(content), "127.0.0.1:8318") {
				droid["configured"] = true
			}
		}
	}
	agents = append(agents, droid)

	// Gemini CLI
	gemini := map[string]any{
		"id":         "gemini",
		"name":       "Gemini CLI",
		"detected":   false,
		"configured": false,
	}
	if path, err := exec.LookPath("gemini"); err == nil {
		gemini["detected"] = true
		gemini["binary_path"] = path
	}
	agents = append(agents, gemini)

	return agents
}

// getModelMappings reads model mappings from the config file
func getModelMappings() []map[string]any {
	home, _ := os.UserHomeDir()
	mappingsPath := filepath.Join(home, ".proxypilot", "model-mappings.json")

	data, err := os.ReadFile(mappingsPath)
	if err != nil {
		return []map[string]any{}
	}

	var mappings []map[string]any
	if err := json.Unmarshal(data, &mappings); err != nil {
		return []map[string]any{}
	}
	return mappings
}

// saveModelMappings saves model mappings to the config file
func saveModelMappings(mappings []map[string]any) error {
	home, _ := os.UserHomeDir()
	mappingsDir := filepath.Join(home, ".proxypilot")
	if err := os.MkdirAll(mappingsDir, 0755); err != nil {
		return err
	}

	mappingsPath := filepath.Join(mappingsDir, "model-mappings.json")
	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mappingsPath, data, 0644)
}
