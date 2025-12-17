#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Restarting CLIProxyAPI..."
"$script_dir/stop-cliproxy.sh" || true
"$script_dir/start-cliproxy.sh"

