Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

$repoRoot = Get-RepoRoot
$outDir = Join-Path $repoRoot "bin"
$outPath = Join-Path $outDir "cliproxyapi-latest.exe"

if (-not (Test-Path -LiteralPath $outDir)) {
  New-Item -ItemType Directory -Path $outDir | Out-Null
}

Write-Host "Building CLIProxyAPI..."
Write-Host "  out: $outPath"

Push-Location $repoRoot
try {
  go build -o $outPath .\cmd\server
} finally {
  Pop-Location
}

Write-Host "Done."

