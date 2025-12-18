Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path
}

function Ensure-Dir([string]$Path) {
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
}

$repoRoot = Get-RepoRoot
$preferredExePath = Join-Path $repoRoot "bin\\proxypilot-engine.exe"
$latestExePath = Join-Path $repoRoot "bin\\cliproxyapi-latest.exe"
$defaultExePath = Join-Path $repoRoot "bin\\cliproxyapi.exe"
$exePath = if (Test-Path -LiteralPath $preferredExePath) { $preferredExePath } elseif (Test-Path -LiteralPath $latestExePath) { $latestExePath } else { $defaultExePath }
$configPath = Join-Path $repoRoot "config.yaml"
$logsDir = Join-Path $repoRoot "logs"
Ensure-Dir $logsDir

$stdoutLog = Join-Path $logsDir "proxypilot-engine.out.log"
$stderrLog = Join-Path $logsDir "proxypilot-engine.err.log"

if (-not (Test-Path -LiteralPath $exePath)) {
  throw "Binary not found: $exePath. Build it with: scripts\\windows\\build-cliproxyapi.ps1"
}
if (-not (Test-Path -LiteralPath $configPath)) {
  throw "Config not found: $configPath"
}

function Get-ConfigPort([string]$Path) {
  try {
    $line = Get-Content -LiteralPath $Path | Where-Object { $_ -match '^\s*port:\s*\d+\s*$' } | Select-Object -First 1
    if ($line -match '(\d+)') { return [int]$Matches[1] }
  } catch {}
  return 8318
}

$port = Get-ConfigPort $configPath
$existing = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($existing) {
  Write-Warning "Port $port is already in use (PID $($existing.OwningProcess))."
  Write-Host "If this is CLIProxyAPI, run: powershell -NoProfile -ExecutionPolicy Bypass -File .\\scripts\\restart-cliproxy.ps1"
  return
}

Write-Host "Starting ProxyPilot Engine..."
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
