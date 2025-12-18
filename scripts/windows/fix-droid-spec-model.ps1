param(
  [Parameter(Mandatory = $false)]
  [string]$Model = "custom:gpt-5.2(high)"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Ensure-CustomModelExists([string]$ModelId) {
  if (-not $ModelId.StartsWith("custom:")) {
    throw "Model must start with 'custom:'. Got: $ModelId"
  }
  $inner = $ModelId.Substring("custom:".Length)

  $configPath = Join-Path $env:USERPROFILE ".factory\\config.json"
  if (-not (Test-Path -LiteralPath $configPath)) {
    throw "Droid BYOK config not found: $configPath"
  }
  $cfg = Get-Content -Raw -LiteralPath $configPath | ConvertFrom-Json
  $models = @($cfg.custom_models | ForEach-Object { [string]$_.model })
  if (-not ($models -contains $inner)) {
    throw "Custom model '$ModelId' not found in $configPath. Available: $($models -join ', ')"
  }
}

function Update-JsonFile([string]$Path, [scriptblock]$Mutator) {
  $obj = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json
  & $Mutator $obj | Out-Null
  $json = $obj | ConvertTo-Json -Depth 50
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($Path, $json, $utf8NoBom)
}

function Update-JsoncModelLine([string]$Path, [string]$ModelId) {
  $text = Get-Content -Raw -LiteralPath $Path
  if ($text -notmatch '"model"\s*:\s*"[^"]*"') {
    throw "Couldn't find a JSON 'model' field in: $Path"
  }
  $updated = [regex]::Replace($text, '"model"\s*:\s*"[^"]*"', ('"model": "' + $ModelId + '"'), 1)
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($Path, $updated, $utf8NoBom)
}

Ensure-CustomModelExists -ModelId $Model

# Update global settings.json (JSONC with comments)
$globalSettingsPath = Join-Path $env:USERPROFILE ".factory\\settings.json"
if (Test-Path -LiteralPath $globalSettingsPath) {
  Update-JsoncModelLine -Path $globalSettingsPath -ModelId $Model
  Write-Host "Updated global model -> $globalSettingsPath"
} else {
  Write-Host "Skipped (not found): $globalSettingsPath"
}

# Update the latest session settings for this folder (pure JSON)
$sessionDirNew = Join-Path $env:USERPROFILE ".factory\\sessions\\-D-code-ProxyPilot"
$sessionDirOld = Join-Path $env:USERPROFILE ".factory\\sessions\\-D-code-CLIProxyAPI"
$sessionDir = if (Test-Path -LiteralPath $sessionDirNew) { $sessionDirNew } else { $sessionDirOld }
if (Test-Path -LiteralPath $sessionDir) {
  $latest = Get-ChildItem -LiteralPath $sessionDir -Filter "*.settings.json" -File |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1
  if ($latest) {
    Update-JsonFile -Path $latest.FullName -Mutator {
      param($o)
      $o.model = $Model
      if ($o.PSObject.Properties.Name -contains "providerLock") { $o.PSObject.Properties.Remove("providerLock") }
      if ($o.PSObject.Properties.Name -contains "providerLockTimestamp") { $o.PSObject.Properties.Remove("providerLockTimestamp") }
      if ($o.PSObject.Properties.Name -contains "apiProviderLock") { $o.PSObject.Properties.Remove("apiProviderLock") }
      if ($o.PSObject.Properties.Name -contains "apiProviderLockTimestamp") { $o.PSObject.Properties.Remove("apiProviderLockTimestamp") }
    }
    Write-Host "Updated session model -> $($latest.FullName)"
  } else {
    Write-Host "No session settings found in: $sessionDir"
  }
} else {
  Write-Host "Skipped (not found): $sessionDir"
}

Write-Host ""
Write-Host "Restart droid, then try Spec mode again."
