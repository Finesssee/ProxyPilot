#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out_dir="$repo_root/bin"
out_path="$out_dir/proxypilot-engine"
compat_path="$out_dir/cliproxyapi-latest"

mkdir -p "$out_dir"

echo "Building ProxyPilot Engine..."
echo "  out: $out_path"
echo "  compat: $compat_path"

(cd "$repo_root" && go build -o "$out_path" ./cmd/server)
cp -f "$out_path" "$compat_path" 2>/dev/null || true

echo "Done."
