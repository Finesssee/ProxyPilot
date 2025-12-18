#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
default_exe="$repo_root/bin/cliproxyapi"
preferred_exe="$repo_root/bin/proxypilot-engine"
latest_exe="$repo_root/bin/cliproxyapi-latest"
exe="$default_exe"
if [[ -f "$preferred_exe" ]]; then
  exe="$preferred_exe"
elif [[ -f "$latest_exe" ]]; then
  exe="$latest_exe"
fi

config_path="$repo_root/config.yaml"
logs_dir="$repo_root/logs"
stdout_log="$logs_dir/proxypilot-engine.out.log"
stderr_log="$logs_dir/proxypilot-engine.err.log"
pid_file="$logs_dir/cliproxyapi.pid"

mkdir -p "$logs_dir"

if [[ ! -f "$exe" ]]; then
  echo "Binary not found: $exe" >&2
  echo "Build it with: ./scripts/build.sh" >&2
  exit 1
fi
chmod +x "$exe" 2>/dev/null || true

if [[ ! -f "$config_path" ]]; then
  echo "Config not found: $config_path" >&2
  exit 1
fi

port="$(
  grep -E '^[[:space:]]*port:[[:space:]]*[0-9]+[[:space:]]*$' "$config_path" 2>/dev/null \
    | head -n 1 \
    | sed -E 's/[^0-9]*([0-9]+).*/\1/'
)"
if [[ -z "${port:-}" ]]; then port="8318"; fi

if [[ -f "$pid_file" ]]; then
  existing_pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [[ -n "${existing_pid:-}" ]] && kill -0 "$existing_pid" 2>/dev/null; then
    echo "ProxyPilot Engine already running (PID $existing_pid)."
    exit 0
  fi
fi

if command -v lsof >/dev/null 2>&1; then
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "Port $port is already in use." >&2
    echo "If this is ProxyPilot Engine, run: ./scripts/restart.sh" >&2
    exit 0
  fi
fi

echo "Starting ProxyPilot Engine..."
echo "  exe:    $exe"
echo "  config: $config_path"
echo "  logs:   $logs_dir"

nohup "$exe" -config "$config_path" >"$stdout_log" 2>"$stderr_log" &
pid="$!"
echo "$pid" >"$pid_file"

echo "Started (PID $pid). Tail logs:"
echo "  tail -f \"$stdout_log\""
