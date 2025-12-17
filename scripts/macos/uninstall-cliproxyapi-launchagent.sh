#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "launchd autostart is macOS-only." >&2
  exit 1
fi

label="com.cliproxyapi"
plist_path="$HOME/Library/LaunchAgents/${label}.plist"
uid="$(id -u)"

if [[ -f "$plist_path" ]]; then
  launchctl bootout "gui/${uid}" "$plist_path" >/dev/null 2>&1 || true
  rm -f "$plist_path"
  echo "Removed LaunchAgent: $plist_path"
else
  echo "LaunchAgent not found: $plist_path"
fi

