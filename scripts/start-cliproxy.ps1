Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Ensure-Dir([string]$Path) {
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
}

$repoRoot = Get-RepoRoot
$exePath = Join-Path $repoRoot "bin\\cliproxyapi.exe"
$configPath = Join-Path $repoRoot "config.yaml"
$logsDir = Join-Path $repoRoot "logs"
Ensure-Dir $logsDir

$stdoutLog = Join-Path $logsDir "cliproxyapi.out.log"
$stderrLog = Join-Path $logsDir "cliproxyapi.err.log"

if (-not (Test-Path -LiteralPath $exePath)) {
  throw "Binary not found: $exePath. Run scripts\\setup-droid-cliproxy.ps1 first (or build it with: go build -o bin\\cliproxyapi.exe .\\cmd\\server)."
}
if (-not (Test-Path -LiteralPath $configPath)) {
  throw "Config not found: $configPath"
}

Write-Host "Starting CLIProxyAPI..."
Write-Host "  exe:    $exePath"
Write-Host "  config: $configPath"
Write-Host "  logs:   $logsDir"

Start-Process -FilePath $exePath `
  -ArgumentList @("-config", $configPath) `
  -WorkingDirectory $repoRoot `
  -WindowStyle Hidden `
  -RedirectStandardOutput $stdoutLog `
  -RedirectStandardError $stderrLog | Out-Null

Write-Host "Started. Tail logs:"
Write-Host "  Get-Content -Wait $stdoutLog"

