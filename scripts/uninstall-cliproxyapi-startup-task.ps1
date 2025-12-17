Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$taskName = "CLIProxyAPI"

Write-Host "Deleting scheduled task '$taskName'..."

try {
  if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction Stop
    Write-Host "Done."
    exit 0
  }
} catch {
  Write-Warning "Unregister-ScheduledTask failed: $($_.Exception.Message)"
  Write-Host "Falling back to schtasks.exe..."
}

& schtasks.exe /Delete /TN $taskName /F | Out-Null
if ($LASTEXITCODE -ne 0) {
  throw "schtasks.exe failed with exit code $LASTEXITCODE"
}
Write-Host "Done."
