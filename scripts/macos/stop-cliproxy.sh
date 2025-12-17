#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
pid_file="$repo_root/logs/cliproxyapi.pid"

stop_pid() {
  local pid="$1"
  if ! kill -0 "$pid" 2>/dev/null; then
    return 1
  fi
  echo "Stopping CLIProxyAPI (PID $pid)"
  kill "$pid" 2>/dev/null || true
  for _ in {1..20}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 0.25
  done
  echo "Force killing CLIProxyAPI (PID $pid)"
  kill -9 "$pid" 2>/dev/null || true
  return 0
}

stopped_any=false

if [[ -f "$pid_file" ]]; then
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [[ -n "${pid:-}" ]] && stop_pid "$pid"; then
    stopped_any=true
  fi
  rm -f "$pid_file"
fi

for name in cliproxyapi-latest cliproxyapi; do
  if command -v pkill >/dev/null 2>&1; then
    if pkill -x "$name" 2>/dev/null; then
      echo "Stopping $name"
      stopped_any=true
    fi
  fi
done

if [[ "$stopped_any" == "false" ]]; then
  echo "No running CLIProxyAPI process found."
fi

