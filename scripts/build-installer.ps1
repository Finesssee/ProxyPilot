# Build the custom WebView2-based installer
# 1. Build installer UI (npm)
# 2. Build main ProxyPilot.exe
# 3. Build installer.exe with embedded UI and app files
#
# Usage: .\build-installer.ps1 [-InnoSetup]
#   -InnoSetup: Build the Inno Setup installer instead of custom installer

param(
    [switch]$InnoSetup
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

$repoRoot = Get-RepoRoot

if ($InnoSetup) {
    # Build Inno Setup installer (legacy)
    $ISCC = "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
    if (-not (Test-Path $ISCC)) {
        $ISCC = "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
    }
    $Version = "0.1.6"

    Write-Host "Building ProxyPilot $Version Installer (Inno Setup)..." -ForegroundColor Cyan
    Write-Host ""

    if (-not (Test-Path $ISCC)) {
        Write-Host "ERROR: Inno Setup not found at $ISCC" -ForegroundColor Red
        exit 1
    }

    & $ISCC /DAppVersion="$Version" "$repoRoot\installer\proxypilot.iss"

    if ($LASTEXITCODE -eq 0) {
        Write-Host ""
        Write-Host "Success! Installer created at: $repoRoot\dist\ProxyPilot-$Version-Setup.exe" -ForegroundColor Green
    } else {
        Write-Host ""
        Write-Host "Build failed with exit code $LASTEXITCODE" -ForegroundColor Red
        exit $LASTEXITCODE
    }
} else {
    # Build custom WebView2-based installer
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "Building ProxyPilot Custom Installer" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host ""

    & (Join-Path $repoRoot "scripts\windows\build-custom-installer.ps1")

    if ($LASTEXITCODE -ne 0) {
        Write-Host ""
        Write-Host "Build failed with exit code $LASTEXITCODE" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}
