Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$valueName = "CLIProxyAPI"
$scriptDir = $PSScriptRoot
$ps1Path = Join-Path $scriptDir "start-cliproxy.ps1"

if (-not (Test-Path -LiteralPath $ps1Path)) {
  throw "Missing starter script: $ps1Path"
}

# Use a hidden PowerShell process at logon.
$command = "powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$ps1Path`""

Write-Host "Installing HKCU Run entry '$valueName'..."
Write-Host "  Command: $command"

New-Item -Path "HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run" -Force | Out-Null
Set-ItemProperty -Path "HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run" -Name $valueName -Value $command

Write-Host "Done."
