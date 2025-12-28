$stateDir = Join-Path $Env:LOCALAPPDATA "ProxyPilot"
$statePath = Join-Path $stateDir "ui-state.json"

if (-not (Test-Path $stateDir)) {
    New-Item -ItemType Directory -Path $stateDir -Force | Out-Null
}

$state = @{
    auto_start_proxy = $true
} | ConvertTo-Json

Set-Content -Path $statePath -Value $state
Write-Host "Created state file: $statePath"
Get-Content $statePath
