Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$procNames = @("cliproxyapi", "cliproxyapi-latest")
$procs = @()
foreach ($name in $procNames) {
  $procs += @(Get-Process -Name $name -ErrorAction SilentlyContinue)
}

if (-not $procs) {
  Write-Host "No running CLIProxyAPI process found."
  exit 0
}

$hadError = $false
foreach ($proc in $procs) {
  $procId = $proc.Id
  Write-Host "Stopping $($proc.ProcessName) (PID $procId)"
  try {
    Stop-Process -Id $procId -Force -ErrorAction Stop
  } catch {
    $hadError = $true
    Write-Warning "Failed to stop PID ${procId}: $($_.Exception.Message)"
    Write-Warning "If CLIProxyAPI was started elevated (e.g. via a 'HighestAvailable' scheduled task), run this script from an elevated PowerShell or stop it via Task Manager (Run as administrator)."
  }
}

if ($hadError) {
  throw "Failed to stop one or more CLIProxyAPI processes."
}
