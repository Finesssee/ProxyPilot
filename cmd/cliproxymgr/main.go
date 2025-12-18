package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/desktopctl"
)

func main() {
	var (
		uiAddr     string
		repoRoot   string
		configPath string
		exePath    string
		noOpen     bool
	)
	flag.StringVar(&uiAddr, "listen", "127.0.0.1:7331", "UI listen address")
	flag.StringVar(&repoRoot, "repo", "", "Repo root (used to locate bin/ and logs/)")
	flag.StringVar(&configPath, "config", "", "Path to config.yaml (defaults to <repo>/config.yaml)")
	flag.StringVar(&exePath, "exe", "", "Path to ProxyPilot Engine binary (defaults to <repo>/bin/proxypilot-engine.exe)")
	flag.BoolVar(&noOpen, "no-open", false, "Don't auto-open browser")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		st, _ := desktopctl.StatusFor(configPath)
		writeJSON(w, st)
	})
	mux.HandleFunc("/api/start", func(w http.ResponseWriter, r *http.Request) {
		st, err := desktopctl.Start(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, st)
	})
	mux.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := desktopctl.Stop(desktopctl.StopOptions{}); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		st, err := desktopctl.Restart(desktopctl.StartOptions{RepoRoot: repoRoot, ConfigPath: configPath, ExePath: exePath})
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, st)
	})
	mux.HandleFunc("/api/open-ui", func(w http.ResponseWriter, r *http.Request) {
		if err := desktopctl.OpenManagementUI(configPath); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/open-logs", func(w http.ResponseWriter, r *http.Request) {
		if err := desktopctl.OpenLogsFolder(repoRoot, configPath); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	srv := &http.Server{
		Addr:              uiAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	url := "http://" + uiAddr
	if strings.HasPrefix(uiAddr, ":") {
		url = "http://127.0.0.1" + uiAddr
	}
	if !noOpen {
		_ = desktopctl.OpenBrowser(url)
	}

	log.Printf("cliproxymgr listening on %s", url)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	writeJSON(w, map[string]any{"error": err.Error()})
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>CLIProxyAPI Manager</title>
  <style>
    :root { color-scheme: dark; }
    body { font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial, sans-serif; margin: 24px; background:#0b0f17; color:#e6edf3; }
    .card { background:#111827; border:1px solid #223047; border-radius:12px; padding:16px; max-width:820px; }
    .row { display:flex; gap:12px; flex-wrap:wrap; align-items:center; }
    .muted { color:#9aa7b4; }
    button { background:#1f6feb; border:0; color:white; padding:10px 12px; border-radius:10px; cursor:pointer; }
    button.secondary { background:#223047; }
    button.danger { background:#d73a49; }
    pre { background:#0b1220; padding:12px; border-radius:10px; overflow:auto; border:1px solid #223047; }
    code { color:#c9d1d9; }
  </style>
</head>
<body>
  <h1>CLIProxyAPI Manager</h1>
  <div class="card">
    <div class="row">
      <div><strong>Status:</strong> <span id="status">…</span></div>
      <div class="muted" id="detail"></div>
    </div>
    <div style="height:12px"></div>
    <div class="row">
      <button id="start">Start</button>
      <button class="danger" id="stop">Stop</button>
      <button class="secondary" id="restart">Restart</button>
      <button class="secondary" id="openUi">Open Proxy UI</button>
      <button class="secondary" id="openLogs">Open Logs</button>
    </div>
    <div style="height:16px"></div>
    <div><strong>Snippets</strong> <span class="muted">(updates when running)</span></div>
    <pre><code id="snippets">Loading…</code></pre>
    <div class="muted" id="error"></div>
  </div>

<script>
  async function api(path) {
    const res = await fetch(path, { method: "POST" });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || res.statusText);
    return data;
  }
  async function getStatus() {
    const res = await fetch("/api/status");
    return res.json();
  }
  function setText(id, text) { document.getElementById(id).textContent = text; }

  async function refresh() {
    try {
      const st = await getStatus();
      setText("status", st.running ? "Running" : "Stopped");
      setText("detail", st.running ? ("port " + st.port + " | pid " + (st.pid || "?")) : (st.last_error || ""));
      const base = st.base_url || "http://127.0.0.1:8318";
      const key = "local-dev-key";
      setText("snippets",
        "OPENAI_BASE_URL=" + base + "/v1\n" +
        "OPENAI_API_KEY=" + key + "\n\n" +
        "curl -H \"Authorization: Bearer " + key + "\" " + base + "/v1/models"
      );
      setText("error", "");
    } catch (e) {
      setText("error", e.message || String(e));
    }
  }

  document.getElementById("start").onclick = async () => { try { await api("/api/start"); } finally { await refresh(); } };
  document.getElementById("stop").onclick = async () => { try { await api("/api/stop"); } finally { await refresh(); } };
  document.getElementById("restart").onclick = async () => { try { await api("/api/restart"); } finally { await refresh(); } };
  document.getElementById("openUi").onclick = async () => { try { await api("/api/open-ui"); } catch (e) { setText("error", e.message); } };
  document.getElementById("openLogs").onclick = async () => { try { await api("/api/open-logs"); } catch (e) { setText("error", e.message); } };

  refresh();
  setInterval(refresh, 2000);
</script>
</body>
</html>`
