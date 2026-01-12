# Build ProxyPilot MSI installer using WiX v4
# Requires: dotnet tool install --global wix

param(
    [string]$Version = "0.2.9",
    [string]$SourceDir = "..\dist",
    [string]$OutDir = "..\dist"
)

$ErrorActionPreference = "Stop"

# Resolve paths
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceDir = Resolve-Path (Join-Path $ScriptDir $SourceDir)
$OutDir = Join-Path $ScriptDir $OutDir

# Ensure output directory exists
if (-not (Test-Path $OutDir)) {
    New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
}

$WxsFile = Join-Path $ScriptDir "proxypilot.wxs"
$MsiFile = Join-Path $OutDir "ProxyPilot-$Version.msi"

Write-Host "Building ProxyPilot MSI installer v$Version" -ForegroundColor Cyan
Write-Host "Source: $SourceDir"
Write-Host "Output: $MsiFile"

# Check WiX is installed
$wix = Get-Command wix -ErrorAction SilentlyContinue
if (-not $wix) {
    Write-Host "WiX not found. Installing via dotnet..." -ForegroundColor Yellow
    dotnet tool install --global wix
    $env:PATH += ";$env:USERPROFILE\.dotnet\tools"
}

# Build the MSI
wix build `
    -d SourceDir="$SourceDir" `
    -d Version="$Version" `
    -ext WixToolset.UI.wixext `
    -o "$MsiFile" `
    "$WxsFile"

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nMSI built successfully: $MsiFile" -ForegroundColor Green

    # Show file size
    $size = (Get-Item $MsiFile).Length / 1MB
    Write-Host "Size: $([math]::Round($size, 2)) MB"
} else {
    Write-Host "MSI build failed with exit code $LASTEXITCODE" -ForegroundColor Red
    exit 1
}
