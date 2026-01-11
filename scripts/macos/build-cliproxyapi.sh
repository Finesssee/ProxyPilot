#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out_dir="$repo_root/bin"
out_path="$out_dir/proxypilot-engine"
compat_path="$out_dir/cliproxyapi-latest"

mkdir -p "$out_dir"

# Version info from git
VERSION="${VERSION:-$(git -C "$repo_root" describe --tags --always 2>/dev/null || echo "dev")}"
COMMIT="${COMMIT:-$(git -C "$repo_root" rev-parse --short HEAD 2>/dev/null || echo "none")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

LDFLAGS="-X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildDate=$BUILD_DATE"

echo "Building ProxyPilot Engine..."
echo "  version: $VERSION"
echo "  commit:  $COMMIT"
echo "  built:   $BUILD_DATE"
echo "  out:     $out_path"

(cd "$repo_root" && go build -ldflags "$LDFLAGS" -o "$out_path" ./cmd/server)
cp -f "$out_path" "$compat_path" 2>/dev/null || true

echo "Done."
