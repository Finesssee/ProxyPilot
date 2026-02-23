#!/bin/sh
# Install git hooks for the project.
# Usage: ./scripts/install-hooks.sh

HOOK_DIR="$(git rev-parse --show-toplevel)/.git/hooks"

cp "$(git rev-parse --show-toplevel)/scripts/pre-commit" "$HOOK_DIR/pre-commit"
chmod +x "$HOOK_DIR/pre-commit"

echo "Git hooks installed."
