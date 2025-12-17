#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out_dir="$repo_root/bin"
out_path="$out_dir/cliproxyapi-latest"

mkdir -p "$out_dir"

echo "Building CLIProxyAPI..."
echo "  out: $out_path"

(cd "$repo_root" && go build -o "$out_path" ./cmd/server)

echo "Done."

