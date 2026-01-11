Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path
}

$repoRoot = Get-RepoRoot

& (Join-Path $repoRoot "scripts\\windows\\build-proxypilot-tray.ps1")

$distDir = Join-Path $repoRoot "dist\\ProxyPilot"
if (Test-Path -LiteralPath $distDir) {
  Remove-Item -Recurse -Force -LiteralPath $distDir
}
New-Item -ItemType Directory -Path $distDir | Out-Null

$exeSrc = Join-Path $repoRoot "bin\\ProxyPilot.exe"
$exeDst = Join-Path $distDir "ProxyPilot.exe"
Copy-Item -Force -LiteralPath $exeSrc -Destination $exeDst

$zipPath = Join-Path $repoRoot "dist\\ProxyPilot.zip"
if (Test-Path -LiteralPath $zipPath) { Remove-Item -Force -LiteralPath $zipPath }

Compress-Archive -Path $distDir\\* -DestinationPath $zipPath

Write-Host "Packaged:"
Write-Host "  $zipPath"
