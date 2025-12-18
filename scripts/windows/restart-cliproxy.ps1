Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Write-Host "Restarting ProxyPilot Engine..."

try {
  & (Join-Path $PSScriptRoot "stop-cliproxy.ps1")
} catch {
  Write-Warning "Stop failed: $($_.Exception.Message)"
  Write-Host ""
  Write-Host "If you see 'Access is denied', the server is running elevated."
  Write-Host "One-time fix (run in an elevated PowerShell):"
  Write-Host "  taskkill /IM proxypilot-engine.exe /F"
  Write-Host "  taskkill /IM cliproxyapi.exe /F"
  Write-Host "  schtasks /Delete /TN \"CLIProxyAPI-Logon\" /F"
  throw
}

& (Join-Path $PSScriptRoot "start-cliproxy.ps1")
