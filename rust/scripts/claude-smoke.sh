#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUST_DIR="$ROOT_DIR/rust"
TMP_DIR="$(mktemp -d)"
MOCK_PORT=18320
PROXY_PORT=18321
MOCK_PID=""
PROXY_PID=""
BIN="$RUST_DIR/target/debug/proxypilot-rs"

cleanup() {
  if [[ -n "$PROXY_PID" ]]; then
    kill "$PROXY_PID" 2>/dev/null || true
    wait "$PROXY_PID" 2>/dev/null || true
  fi
  if [[ -n "$MOCK_PID" ]]; then
    kill "$MOCK_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

cat > "$TMP_DIR/mock_claude.py" <<'PY'
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def _send(self, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _headers(self):
        return {
            "x_api_key": self.headers.get("x-api-key", ""),
            "authorization": self.headers.get("authorization", ""),
            "anthropic_version": self.headers.get("anthropic-version", ""),
        }

    def do_GET(self):
        if self.path == "/v1/models":
            payload = {
                "object": "list",
                "data": [{"id": "claude-3-5-sonnet-smoke"}],
                "headers": self._headers(),
            }
            self._send(payload)
            return
        self.send_error(404)

    def do_POST(self):
        length = int(self.headers.get("content-length", "0"))
        self.rfile.read(length)
        if self.path == "/v1/messages":
            payload = {
                "id": "msg_smoke",
                "type": "message",
                "role": "assistant",
                "content": [{"type": "text", "text": "ok"}],
                "headers": self._headers(),
            }
            self._send(payload)
            return
        self.send_error(404)

    def log_message(self, format, *args):
        return


ThreadingHTTPServer(("127.0.0.1", 18320), Handler).serve_forever()
PY

python3 "$TMP_DIR/mock_claude.py" &
MOCK_PID="$!"

(
  cd "$RUST_DIR"
  cargo build -q -p proxypilot-rs
)

cat > "$TMP_DIR/proxypilot-rs.toml" <<EOF
[server]
bind = "127.0.0.1:${PROXY_PORT}"

[state]
path = "$TMP_DIR/proxypilot-rs.state.toml"

[providers]
active = "claude"

[codex]
upstream_base_url = "https://api.openai.com"
api_key = ""

[claude]
upstream_base_url = "http://127.0.0.1:${MOCK_PORT}"
api_key = "claude-smoke-token"
EOF

"$BIN" run --config "$TMP_DIR/proxypilot-rs.toml" &
PROXY_PID="$!"

for _ in {1..100}; do
  if curl -fs "http://127.0.0.1:${PROXY_PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

curl -fsS "http://127.0.0.1:${PROXY_PORT}/healthz" | grep -q '"provider":"claude"'
curl -fsS "http://127.0.0.1:${PROXY_PORT}/v0/runtime/stats" | grep -q '"source":"config_fallback_key"'
curl -fsS "http://127.0.0.1:${PROXY_PORT}/v1/models" | grep -q "claude-3-5-sonnet-smoke"
curl -fsS "http://127.0.0.1:${PROXY_PORT}/v1/models" | grep -q "claude-smoke-token"
curl -fsS \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-5-sonnet-smoke","max_tokens":16,"messages":[{"role":"user","content":"ping"}]}' \
  "http://127.0.0.1:${PROXY_PORT}/v1/messages" | grep -q "msg_smoke"
curl -fsS \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-5-sonnet-smoke","max_tokens":16,"messages":[{"role":"user","content":"ping"}]}' \
  "http://127.0.0.1:${PROXY_PORT}/v1/messages" | grep -q "claude-smoke-token"

echo "Claude Rust smoke passed"
