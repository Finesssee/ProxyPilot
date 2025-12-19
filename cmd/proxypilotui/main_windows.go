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
)

func main() {
	var repoRoot string
	var configPath string
	var exePath string
	var url string
	var standalone bool
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate bin/ and logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.StringVar(&exePath, "exe", "", "Path to proxy engine binary (defaults to <repo>/bin/proxypilot-engine.exe)")
	flag.StringVar(&url, "url", "", "Open a specific URL in-app (advanced)")
	flag.BoolVar(&standalone, "standalone", false, "Run standalone UI without connecting to proxy server")
	flag.Parse()

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
			"running":    cur.Running,
			"port":       cur.Port,
			"base_url":   cur.BaseURL,
			"last_error": cur.LastError,
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
		case "iflow":
			endpoint = "/v0/management/iflow-auth-url"
		default:
			return fmt.Errorf("unknown provider: %s", provider)
		}
		return startOAuthFlow(configPath, endpoint)
	})
	_ = w.Bind("pp_copy_diagnostics", func() error { return copyDiagnosticsToClipboard(configPath) })
	_ = w.Bind("pp_get_management_key", func() (string, error) { return desktopctl.GetManagementPassword() })

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
          <button id="oauthIflow">iFlow</button>
        </div>
        <div class="hint">These open the provider login flow in your browser and save auth files for ProxyPilot.</div>
      </div>
    </div>
  </div>

  <div class="toast" id="toast"></div>
  <script>
    const $ = (id) => document.getElementById(id);
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
    };

    async function refreshStatus() {
      try{
        const s = await window.pp_status();
        $('baseUrl').textContent = s.base_url || '-';
        $('port').textContent = s.port ? String(s.port) : '-';
        $('lastErr').textContent = (s.last_error && s.last_error.trim()) ? s.last_error : '-';
        $('statusText').textContent = s.running ? ('Running (:' + s.port + ')') : 'Stopped';
        setRunningUI(!!s.running);
      }catch(e){
        $('statusText').textContent = 'Error';
        $('lastErr').textContent = (e && e.message) ? e.message : String(e);
        setRunningUI(false);
      }
    }

    async function init() {
      try{
        const priv = await window.pp_get_oauth_private();
        $('privateOAuth').checked = !!priv;
      }catch(e){}
      await refreshStatus();
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

    $('privateOAuth').addEventListener('change', async (ev) => {
      try{ await window.pp_set_oauth_private(!!ev.target.checked); toast('Saved'); }catch(e){ toast(e.message||String(e)); }
    });

    const oauth = (provider) => async () => { try{ await window.pp_oauth(provider); toast('Opened login'); }catch(e){ toast(e.message||String(e)); } };
    $('oauthAntigravity').addEventListener('click', oauth('antigravity'));
    $('oauthGeminiCli').addEventListener('click', oauth('gemini-cli'));
    $('oauthCodex').addEventListener('click', oauth('codex'));
    $('oauthClaude').addEventListener('click', oauth('claude'));
    $('oauthQwen').addEventListener('click', oauth('qwen'));
    $('oauthIflow').addEventListener('click', oauth('iflow'));

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
