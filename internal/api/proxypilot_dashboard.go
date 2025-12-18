package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerProxyPilotDashboardRoutes() {
	if s == nil || s.engine == nil {
		return
	}

	s.engine.GET("/proxypilot", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/proxypilot.html")
	})
	s.engine.GET("/proxypilot.html", s.serveProxyPilotDashboard)
}

func (s *Server) serveProxyPilotDashboard(c *gin.Context) {
	// Never serve the embedded management key to non-local clients.
	clientIP := c.ClientIP()
	if clientIP != "127.0.0.1" && clientIP != "::1" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	key := strings.TrimSpace(os.Getenv("MANAGEMENT_PASSWORD"))
	if key == "" || !s.managementRoutesEnabled.Load() {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, proxyPilotDashboardNoKeyHTML())
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, proxyPilotDashboardHTML(key))
}

func proxyPilotDashboardNoKeyHTML() string {
	return `<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>ProxyPilot Dashboard</title>
    <style>
      body { font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif; background:#0b0f17; color:#e7eaf0; margin:0; padding:24px; }
      .card { max-width: 900px; margin: 0 auto; background:#0f172a; border:1px solid #1f2a44; border-radius:12px; padding:16px; }
      a { color:#7dd3fc; }
      code { background:#111b33; padding:2px 6px; border-radius:6px; }
    </style>
  </head>
  <body>
    <div class="card">
      <h1>ProxyPilot Dashboard</h1>
      <p>This dashboard is available when ProxyPilot starts the engine (it sets a local management key).</p>
      <p>Start the proxy from the ProxyPilot tray app, then reload this page.</p>
      <p>If you started the engine manually, management endpoints may be disabled unless <code>MANAGEMENT_PASSWORD</code> is set.</p>
    </div>
  </body>
</html>`
}

func proxyPilotDashboardHTML(managementKey string) string {
	// This is served to localhost only.
	escaped := strings.ReplaceAll(managementKey, `"`, "")
	return `<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="pp-mgmt-key" content="` + escaped + `">
    <title>ProxyPilot Dashboard</title>
    <style>
      :root{
        --bg:#0b0f17; --panel:#0f172a; --muted:#9aa4b2; --text:#e7eaf0; --border:#1f2a44;
        --btn:#1e293b; --btn2:#0b2a3a; --accent:#7dd3fc; --bad:#fb7185; --good:#34d399; --warn:#fbbf24;
        --mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
      }
      body { font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif; background:var(--bg); color:var(--text); margin:0; padding:20px; }
      .wrap{ max-width:1100px; margin:0 auto; display:grid; gap:12px; }
      .row{ display:grid; grid-template-columns: 1fr 1fr; gap:12px; }
      @media (max-width: 980px){ .row { grid-template-columns: 1fr; } }
      .card{ background:var(--panel); border:1px solid var(--border); border-radius:12px; padding:14px; }
      h1{ font-size:18px; margin:0 0 10px 0;}
      h2{ font-size:14px; margin:0 0 10px 0; color:#c7d2fe;}
      .muted{ color:var(--muted); font-size:12px; }
      .btn{ background:var(--btn); border:1px solid var(--border); color:var(--text); padding:6px 10px; border-radius:10px; cursor:pointer; }
      .btn:hover{ border-color:#2e3b59; }
      .btn2{ background:var(--btn2); }
      .pill{ display:inline-block; font-size:12px; padding:2px 8px; border-radius:999px; border:1px solid var(--border); }
      .good{ color:var(--good); border-color:#1b4b3e; background:#0b1f1a; }
      .bad{ color:var(--bad); border-color:#5b2132; background:#221017; }
      .warn{ color:var(--warn); border-color:#5a4a1a; background:#1b160a; }
      table{ width:100%; border-collapse:collapse; font-size:12px; }
      th, td{ border-top:1px solid var(--border); padding:8px 6px; text-align:left; vertical-align:top; }
      th{ color:#cbd5e1; font-weight:600; }
      code{ font-family:var(--mono); font-size:12px; }
      pre{ font-family:var(--mono); font-size:12px; white-space:pre-wrap; background:#0b1226; border:1px solid var(--border); border-radius:12px; padding:10px; margin:8px 0 0 0; }
      input[type="number"]{ width:110px; background:#0b1226; border:1px solid var(--border); color:var(--text); border-radius:10px; padding:6px 8px; }
    </style>
  </head>
  <body>
    <div class="wrap">
      <div class="card">
        <h1>ProxyPilot Dashboard</h1>
        <div class="muted">Local-only dashboard for managing the ProxyPilot engine. Management key is injected server-side for localhost.</div>
      </div>

      <div class="row">
        <div class="card">
          <h2>Status</h2>
          <div id="statusLine" class="pill warn">Checking...</div>
          <div class="muted" style="margin-top:8px">Base URL: <code id="baseUrl"></code></div>
          <div style="margin-top:10px; display:flex; gap:8px; flex-wrap:wrap">
            <button class="btn" id="refreshBtn">Refresh</button>
            <button class="btn btn2" id="openMgmtBtn">Open legacy management UI</button>
          </div>
        </div>

        <div class="card">
          <h2>Quick Config</h2>
          <div class="muted">Edits are persisted (YAML comments preserved).</div>
          <div style="margin-top:10px; display:grid; gap:10px">
            <div>
              <span class="muted">Debug</span>
              <button class="btn" id="toggleDebugBtn">Toggle</button>
              <span id="debugValue" class="pill warn">...</span>
            </div>
            <div>
              <span class="muted">Request retry</span>
              <input type="number" min="0" max="10" id="retryInput" />
              <button class="btn" id="saveRetryBtn">Save</button>
            </div>
            <div>
              <span class="muted">Max retry interval (seconds)</span>
              <input type="number" min="0" max="3600" id="maxRetryInput" />
              <button class="btn" id="saveMaxRetryBtn">Save</button>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Accounts</h2>
        <div class="muted">From <code>/v0/management/auth-files</code>. Reset clears local cooldown/blocked flags so the next request can probe again.</div>
        <div style="margin-top:10px; display:flex; gap:8px; flex-wrap:wrap">
          <button class="btn" id="refreshAuthBtn">Refresh accounts</button>
          <button class="btn" id="resetAllCooldownsBtn">Reset all cooldowns</button>
        </div>
        <div id="authTableWrap" style="margin-top:10px"></div>
      </div>

      <div class="row">
        <div class="card">
          <h2>Diagnostics</h2>
          <div style="display:flex; gap:8px; flex-wrap:wrap">
            <button class="btn" id="loadDiagBtn">Load</button>
            <button class="btn" id="copyDiagBtn">Copy</button>
          </div>
          <pre id="diagPre"></pre>
        </div>
        <div class="card">
          <h2>Logs (tail)</h2>
          <div style="display:flex; gap:8px; flex-wrap:wrap">
            <button class="btn" data-log="stdout" id="tailStdoutBtn">Stdout</button>
            <button class="btn" data-log="stderr" id="tailStderrBtn">Stderr</button>
          </div>
          <pre id="logPre"></pre>
        </div>
      </div>
    </div>

    <script>
      const mgmtKey = document.querySelector('meta[name="pp-mgmt-key"]').content;
      const baseUrl = location.origin;
      document.getElementById('baseUrl').textContent = baseUrl;

      async function api(path, opts = {}) {
        const headers = Object.assign({}, opts.headers || {}, { 'X-Management-Key': mgmtKey });
        const res = await fetch(path, Object.assign({}, opts, { headers }));
        const ct = (res.headers.get('content-type') || '').toLowerCase();
        const body = ct.includes('application/json') ? await res.json() : await res.text();
        if (!res.ok) {
          const msg = typeof body === 'string' ? body : (body && body.error) ? body.error : JSON.stringify(body);
          throw new Error(res.status + ' ' + res.statusText + ': ' + msg);
        }
        return body;
      }

      function setPill(el, kind, text) {
        el.className = 'pill ' + kind;
        el.textContent = text;
      }

      async function refreshStatus() {
        const pill = document.getElementById('statusLine');
        setPill(pill, 'warn', 'Checking...');
        try {
          const res = await fetch('/healthz');
          if (res.ok) setPill(pill, 'good', 'Running');
          else setPill(pill, 'bad', 'Stopped (' + res.status + ')');
        } catch (e) {
          setPill(pill, 'bad', 'Stopped (' + e.message + ')');
        }
      }

      async function loadConfig() {
        const cfg = await api('/v0/management/config');
        const debug = !!cfg.debug;
        setPill(document.getElementById('debugValue'), debug ? 'good' : 'warn', debug ? 'On' : 'Off');
        document.getElementById('retryInput').value = cfg['request-retry'] ?? cfg.request_retry ?? cfg.requestRetry ?? 0;
        const max = cfg['max-retry-interval'] ?? cfg.max_retry_interval ?? cfg.maxRetryInterval ?? 0;
        document.getElementById('maxRetryInput').value = max;
      }

      async function toggleDebug() {
        const current = (await api('/v0/management/debug')).value;
        await api('/v0/management/debug', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ value: !current })});
        await loadConfig();
      }

      async function saveRetry() {
        const v = parseInt(document.getElementById('retryInput').value || '0', 10);
        await api('/v0/management/request-retry', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ value: v })});
        await loadConfig();
      }

      async function saveMaxRetry() {
        const v = parseInt(document.getElementById('maxRetryInput').value || '0', 10);
        await api('/v0/management/max-retry-interval', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ value: v })});
        await loadConfig();
      }

      function renderAuthTable(files) {
        if (!Array.isArray(files) || files.length === 0) {
          return '<div class="muted">No auth files loaded.</div>';
        }
        let html = '<table><thead><tr>' +
          '<th>Provider</th><th>Email/Label</th><th>Status</th><th>Notes</th><th></th>' +
          '</tr></thead><tbody>';
        for (const f of files) {
          const prov = (f.provider || f.type || '').toString();
          const email = (f.email || '').toString();
          const label = (f.label || '').toString();
          const status = (f.status || '').toString();
          const msg = (f.status_message || '').toString();
          const id = (f.id || '').toString();
          const unavailable = !!f.unavailable;
          const disabled = !!f.disabled;
          const badge = disabled ? '<span class="pill bad">Disabled</span>' : unavailable ? '<span class="pill warn">Unavailable</span>' : '<span class="pill good">OK</span>';
          html += '<tr>' +
            '<td><code>' + escapeHtml(prov) + '</code></td>' +
            '<td>' + escapeHtml(email || label || '-') + '</td>' +
            '<td>' + escapeHtml(status || '-') + '<div class="muted">' + escapeHtml(msg) + '</div></td>' +
            '<td>' + badge + '<div class="muted"><code>' + escapeHtml(id) + '</code></div></td>' +
            '<td><button class="btn" data-reset="' + escapeAttr(id) + '">Reset</button></td>' +
          '</tr>';
        }
        html += '</tbody></table>';
        return html;
      }

      function escapeHtml(s) {
        return (s || '').toString().replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('\"','&quot;').replaceAll(\"'\",'&#39;');
      }
      function escapeAttr(s){ return escapeHtml(s).replaceAll('\"',''); }

      async function refreshAuth() {
        const res = await api('/v0/management/auth-files');
        const wrap = document.getElementById('authTableWrap');
        wrap.innerHTML = renderAuthTable(res.files || []);
        wrap.querySelectorAll('button[data-reset]').forEach(btn => {
          btn.addEventListener('click', async () => {
            const id = btn.getAttribute('data-reset');
            try {
              await api('/v0/management/auth/reset-cooldown', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ auth_id: id })});
              await refreshAuth();
            } catch (e) { alert(e.message); }
          });
        });
      }

      async function resetAllCooldowns() {
        await api('/v0/management/auth/reset-cooldown', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({})});
        await refreshAuth();
      }

      async function loadDiagnostics() {
        const res = await api('/v0/management/proxypilot/diagnostics?lines=120');
        document.getElementById('diagPre').textContent = res.text || '';
      }

      async function copyDiagnostics() {
        const text = document.getElementById('diagPre').textContent || '';
        if (!text) return;
        try { await navigator.clipboard.writeText(text); }
        catch(e) { alert('Copy failed: ' + e.message); }
      }

      async function tailLogs(kind) {
        const res = await api('/v0/management/proxypilot/logs/tail?file=' + encodeURIComponent(kind) + '&lines=200');
        document.getElementById('logPre').textContent = (res.lines || []).join('\\n');
      }

      document.getElementById('refreshBtn').addEventListener('click', () => { refreshStatus(); loadConfig(); refreshAuth(); });
      document.getElementById('openMgmtBtn').addEventListener('click', () => { location.href = baseUrl + '/management.html'; });
      document.getElementById('toggleDebugBtn').addEventListener('click', () => toggleDebug().catch(e => alert(e.message)));
      document.getElementById('saveRetryBtn').addEventListener('click', () => saveRetry().catch(e => alert(e.message)));
      document.getElementById('saveMaxRetryBtn').addEventListener('click', () => saveMaxRetry().catch(e => alert(e.message)));
      document.getElementById('refreshAuthBtn').addEventListener('click', () => refreshAuth().catch(e => alert(e.message)));
      document.getElementById('resetAllCooldownsBtn').addEventListener('click', () => resetAllCooldowns().catch(e => alert(e.message)));
      document.getElementById('loadDiagBtn').addEventListener('click', () => loadDiagnostics().catch(e => alert(e.message)));
      document.getElementById('copyDiagBtn').addEventListener('click', () => copyDiagnostics().catch(e => alert(e.message)));
      document.getElementById('tailStdoutBtn').addEventListener('click', () => tailLogs('stdout').catch(e => alert(e.message)));
      document.getElementById('tailStderrBtn').addEventListener('click', () => tailLogs('stderr').catch(e => alert(e.message)));

      (async () => {
        await refreshStatus();
        await loadConfig();
        await refreshAuth();
        await tailLogs('stdout');
      })().catch(e => alert(e.message));
    </script>
  </body>
</html>`
}
