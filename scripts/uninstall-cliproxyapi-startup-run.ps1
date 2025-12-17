Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$valueName = "CLIProxyAPI"

Write-Host "Removing HKCU Run entry '$valueName'..."
Remove-ItemProperty -Path "HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run" -Name $valueName -ErrorAction SilentlyContinue
Write-Host "Done."

