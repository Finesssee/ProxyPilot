Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$taskName = "CLIProxyAPI"
$scriptDir = $PSScriptRoot
$vbsPath = Join-Path $scriptDir "run-cliproxyapi-hidden.vbs"

if (-not (Test-Path -LiteralPath $vbsPath)) {
  throw "Missing launcher: $vbsPath"
}

Write-Host "Creating scheduled task '$taskName' (At log on)..."
Write-Host "  Launcher: $vbsPath"

try {
  $action = New-ScheduledTaskAction -Execute "wscript.exe" -Argument "`"$vbsPath`""
  $trigger = New-ScheduledTaskTrigger -AtLogOn
  $userId = if ($env:USERDOMAIN) { "$($env:USERDOMAIN)\\$($env:USERNAME)" } else { $env:USERNAME }
  $principal = New-ScheduledTaskPrincipal -UserId $userId -LogonType Interactive -RunLevel Limited
  $settings = New-ScheduledTaskSettingsSet -StartWhenAvailable

  Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
  Write-Host "Done."
  Write-Host "Verify:"
  Write-Host "  Get-ScheduledTask -TaskName $taskName | Format-List *"
  exit 0
} catch {
  Write-Warning "Register-ScheduledTask failed: $($_.Exception.Message)"
  Write-Host "Falling back to schtasks.exe..."
}

$taskAction = "wscript.exe `"$vbsPath`""
& schtasks.exe /Create /TN $taskName /SC ONLOGON /F /RL LIMITED /TR $taskAction | Out-Null
if ($LASTEXITCODE -ne 0) {
  throw "schtasks.exe failed with exit code $LASTEXITCODE. Try running this script in an elevated PowerShell."
}

Write-Host "Done."
Write-Host "Verify:"
Write-Host "  schtasks /Query /TN $taskName /V /FO LIST"
