Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$exeName = "cliproxyapi"
$procs = Get-Process -Name $exeName -ErrorAction SilentlyContinue

if (-not $procs) {
  Write-Host "No running $exeName process found."
  exit 0
}

$procs | ForEach-Object {
  Write-Host "Stopping $exeName (PID $($_.Id))"
  Stop-Process -Id $_.Id -Force
}

