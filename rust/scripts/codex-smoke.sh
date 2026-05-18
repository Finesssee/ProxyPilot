#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUST_DIR="$ROOT_DIR/rust"
TMP_DIR="$(mktemp -d)"
MOCK_PORT=18318
PROXY_PORT=18319
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

cat > "$TMP_DIR/mock_codex.py" <<'PY'
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

    def do_GET(self):
        if self.path == "/v1/models":
            self._send({"object": "list", "data": [{"id": "gpt-5.1-codex-smoke"}]})
            return
        self.send_error(404)

    def do_POST(self):
        length = int(self.headers.get("content-length", "0"))
        self.rfile.read(length)
        if self.path == "/v1/chat/completions":
            self._send({
                "id": "chatcmpl-smoke",
                "object": "chat.completion",
                "choices": [{"message": {"role": "assistant", "content": "ok"}}],
            })
            return
        if self.path == "/v1/responses":
            self._send({
                "id": "resp_smoke",
                "object": "response",
                "output": [{"type": "message", "content": [{"type": "output_text", "text": "ok"}]}],
            })
            return
        if self.path == "/v1/responses/compact":
            self._send({
                "id": "resp_compact_smoke",
                "object": "response.compaction",
                "usage": {"input_tokens": 1, "output_tokens": 2, "total_tokens": 3},
            })
            return
        self.send_error(404)

    def log_message(self, format, *args):
        return


ThreadingHTTPServer(("127.0.0.1", 18318), Handler).serve_forever()
PY

python3 "$TMP_DIR/mock_codex.py" &
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
active = "codex"

[codex]
upstream_base_url = "http://127.0.0.1:${MOCK_PORT}"
api_key = ""

[claude]
upstream_base_url = "https://api.anthropic.com"
api_key = ""
EOF

(
  "$BIN" account add-codex \
    --config "$TMP_DIR/proxypilot-rs.toml" \
    --name smoke \
    --api-key smoke-token \
    --activate
  "$BIN" run --config "$TMP_DIR/proxypilot-rs.toml"
) &
PROXY_PID="$!"

for _ in {1..100}; do
  if curl -fs "http://127.0.0.1:${PROXY_PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

curl -fsS "http://127.0.0.1:${PROXY_PORT}/healthz" >/dev/null
curl -fsS "http://127.0.0.1:${PROXY_PORT}/health" >/dev/null
curl -fsS "http://127.0.0.1:${PROXY_PORT}/v0/runtime/stats" | grep -q '"active_account_name":"smoke"'
curl -fsS "http://127.0.0.1:${PROXY_PORT}/v1/models" | grep -q "gpt-5.1-codex-smoke"
curl -fsS \
  -H "content-type: application/json" \
  -d '{"model":"gpt-5.1-codex-smoke","messages":[{"role":"user","content":"ping"}]}' \
  "http://127.0.0.1:${PROXY_PORT}/v1/chat/completions" | grep -q "chatcmpl-smoke"
curl -fsS \
  -H "content-type: application/json" \
  -d '{"model":"gpt-5.1-codex-smoke","input":"ping"}' \
  "http://127.0.0.1:${PROXY_PORT}/v1/responses" | grep -q "resp_smoke"
curl -fsS \
  -H "content-type: application/json" \
  -d '{"model":"gpt-5.1-codex-smoke","instructions":"compact this"}' \
  "http://127.0.0.1:${PROXY_PORT}/v1/responses/compact" | grep -q "resp_compact_smoke"
curl -fsS \
  -H "content-type: application/json" \
  -d '{"model":"gpt-5.1-codex-smoke","input":"alias ping"}' \
  "http://127.0.0.1:${PROXY_PORT}/backend-api/codex/responses" | grep -q "resp_smoke"

echo "Codex Rust smoke passed"
