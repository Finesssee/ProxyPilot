#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
cargo fetch --manifest-path rust/Cargo.toml
if [ ! -f rust/proxypilot-rs.toml ]; then
  cargo run --manifest-path rust/Cargo.toml -p proxypilot-rs -- init --config rust/proxypilot-rs.toml
fi
