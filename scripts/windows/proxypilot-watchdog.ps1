param(
  [int]$Port = 8318,
  [string]$WorkingDir = "D:\\code\\ProxyPilot",
  [string]$ExePath = "D:\\code\\ProxyPilot\\proxypilot.exe",
  [string]$ConfigPath = "D:\\code\\ProxyPilot\\config.yaml",
  [int]$CheckIntervalSeconds = 5
)

$ErrorActionPreference = 'Stop'

$logDir = Join-Path $WorkingDir 'logs'
if (-not (Test-Path $logDir)) {
  New-Item -ItemType Directory -Path $logDir | Out-Null
}

$watchdogLog = Join-Path $logDir 'proxypilot-watchdog.log'

function Write-WatchdogLog([string]$message) {
  $ts = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
  Add-Content -Path $watchdogLog -Value "[$ts] $message"
}

Write-WatchdogLog "watchdog started (port=$Port, interval=${CheckIntervalSeconds}s)"

while ($true) {
  try {
    $listener = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1

    if (-not $listener) {
      if (-not (Test-Path $ExePath)) {
        Write-WatchdogLog "exe not found: $ExePath"
      } elseif (-not (Test-Path $ConfigPath)) {
        Write-WatchdogLog "config not found: $ConfigPath"
      } else {
        Start-Process -FilePath $ExePath -ArgumentList @('-config', $ConfigPath) -WorkingDirectory $WorkingDir -WindowStyle Hidden
        Write-WatchdogLog "started proxypilot (missing listener on $Port)"
      }
    }
  } catch {
    Write-WatchdogLog "error: $($_.Exception.Message)"
  }

  Start-Sleep -Seconds $CheckIntervalSeconds
}
