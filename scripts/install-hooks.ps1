# Install git hooks for the project (Windows).
# Usage: .\scripts\install-hooks.ps1

$repoRoot = git rev-parse --show-toplevel
$hookDir = Join-Path $repoRoot ".git" "hooks"

Copy-Item (Join-Path $repoRoot "scripts" "pre-commit") (Join-Path $hookDir "pre-commit") -Force

Write-Host "Git hooks installed."
