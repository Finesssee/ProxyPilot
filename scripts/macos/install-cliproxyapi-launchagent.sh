#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "launchd autostart is macOS-only." >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
default_exe="$repo_root/bin/cliproxyapi"
latest_exe="$repo_root/bin/cliproxyapi-latest"
exe="$default_exe"
if [[ -f "$latest_exe" ]]; then exe="$latest_exe"; fi

config_path="$repo_root/config.yaml"
logs_dir="$repo_root/logs"
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

label="com.cliproxyapi"
plist_dir="$HOME/Library/LaunchAgents"
plist_path="$plist_dir/${label}.plist"
mkdir -p "$plist_dir"

cat >"$plist_path" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>${label}</string>

    <key>ProgramArguments</key>
    <array>
      <string>${exe}</string>
      <string>-config</string>
      <string>${config_path}</string>
    </array>

    <key>WorkingDirectory</key>
    <string>${repo_root}</string>

    <key>RunAtLoad</key>
    <true/>

    <key>StandardOutPath</key>
    <string>${logs_dir}/cliproxyapi.out.log</string>
    <key>StandardErrorPath</key>
    <string>${logs_dir}/cliproxyapi.err.log</string>
  </dict>
</plist>
EOF

uid="$(id -u)"
launchctl bootout "gui/${uid}" "$plist_path" >/dev/null 2>&1 || true
launchctl bootstrap "gui/${uid}" "$plist_path"
launchctl kickstart -k "gui/${uid}/${label}" >/dev/null 2>&1 || true

echo "Installed LaunchAgent: $plist_path"

