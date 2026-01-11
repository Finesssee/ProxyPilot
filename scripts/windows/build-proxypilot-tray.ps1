Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path
}

$repoRoot = Get-RepoRoot
$outDir = Join-Path $repoRoot "bin"
if (-not (Test-Path -LiteralPath $outDir)) {
  New-Item -ItemType Directory -Path $outDir | Out-Null
}

$outPath = Join-Path $outDir "ProxyPilot.exe"

Write-Host "Building ProxyPilot (tray)..."
Write-Host "  out: $outPath"

Push-Location $repoRoot
try {
  # -H windowsgui => no console window when launched normally.
  go build -ldflags "-H windowsgui" -o $outPath .\\cmd\\proxypilot-tray
} finally {
  Pop-Location
}

Write-Host "Done."
