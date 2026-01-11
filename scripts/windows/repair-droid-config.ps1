param(
  [Parameter(Mandatory = $false)]
  [string]$ConfigPath = (Join-Path $env:USERPROFILE ".factory\\config.json")
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $ConfigPath)) {
  throw "Droid config not found: $ConfigPath"
}

# Load JSON (tolerates existing UTF-8 BOM)
try {
  $obj = Get-Content -Raw -LiteralPath $ConfigPath | ConvertFrom-Json
} catch {
  throw "config.json is not valid JSON: $($_.Exception.Message)"
}

# Backup existing file
$backupPath = "$ConfigPath.bomfix.$(Get-Date -Format 'yyyyMMdd-HHmmss')"
Copy-Item -LiteralPath $ConfigPath -Destination $backupPath -Force

# Rewrite as UTF-8 WITHOUT BOM to satisfy Droid's JSON parser
$json = $obj | ConvertTo-Json -Depth 50
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($ConfigPath, $json, $utf8NoBom)

Write-Host "Rewrote Droid config as UTF-8 without BOM -> $ConfigPath"
Write-Host "Backup saved to -> $backupPath"