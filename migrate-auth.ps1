# Migrate auth files from ~/.cli-proxy-api to %LOCALAPPDATA%\ProxyPilot\auth

$oldDir = Join-Path $Env:USERPROFILE ".cli-proxy-api"
$newDir = Join-Path $Env:LOCALAPPDATA "ProxyPilot\auth"

Write-Host "Migrating auth files..."
Write-Host "  From: $oldDir"
Write-Host "  To:   $newDir"

# Create new directory
if (-not (Test-Path $newDir)) {
    New-Item -ItemType Directory -Path $newDir -Force | Out-Null
    Write-Host "Created: $newDir"
}

# Copy files
$files = Get-ChildItem $oldDir -Filter "*.json" -ErrorAction SilentlyContinue
foreach ($file in $files) {
    $dest = Join-Path $newDir $file.Name
    Copy-Item $file.FullName $dest -Force
    Write-Host "  Copied: $($file.Name)"
}

Write-Host "`nMigration complete! $($files.Count) files copied."
Write-Host "New auth directory: $newDir"
