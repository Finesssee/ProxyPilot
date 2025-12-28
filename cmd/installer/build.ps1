# ProxyPilot Installer Build Script
# This script builds the installer with embedded UI and application files.

param(
    [string]$OutputPath = ".\dist\ProxyPilotInstaller.exe",
    [string]$ProxyPilotExe = ".\dist\ProxyPilot.exe",
    [switch]$SkipUI
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $scriptDir)

Write-Host "Building ProxyPilot Installer..." -ForegroundColor Cyan

# Step 1: Build the installer UI (if not skipped)
if (-not $SkipUI) {
    Write-Host "Building installer UI..." -ForegroundColor Yellow
    Push-Location "$repoRoot\installer-ui"
    try {
        npm install
        npm run build
    }
    finally {
        Pop-Location
    }

    # Copy built UI to cmd/installer/ui
    Write-Host "Copying UI assets..." -ForegroundColor Yellow
    $uiSrcDir = "$repoRoot\installer-ui\dist"
    $uiDstDir = "$scriptDir\ui"

    if (Test-Path $uiDstDir) {
        Remove-Item -Recurse -Force $uiDstDir
    }
    Copy-Item -Recurse $uiSrcDir $uiDstDir
}

# Step 2: Copy bundle files
Write-Host "Preparing bundle files..." -ForegroundColor Yellow
$bundleDir = "$scriptDir\bundle"

# Ensure bundle directory exists
if (-not (Test-Path $bundleDir)) {
    New-Item -ItemType Directory -Path $bundleDir | Out-Null
}

# Copy ProxyPilot.exe
if (Test-Path $ProxyPilotExe) {
    Copy-Item $ProxyPilotExe "$bundleDir\ProxyPilot.exe" -Force
    Write-Host "  Copied ProxyPilot.exe" -ForegroundColor Green
} else {
    Write-Host "  Warning: ProxyPilot.exe not found at $ProxyPilotExe" -ForegroundColor Yellow
}

# Copy config.example.yaml
$configExample = "$repoRoot\config.example.yaml"
if (Test-Path $configExample) {
    Copy-Item $configExample "$bundleDir\config.example.yaml" -Force
    Write-Host "  Copied config.example.yaml" -ForegroundColor Green
}

# Copy icons
$iconIco = "$repoRoot\static\icon.ico"
$iconPng = "$repoRoot\static\icon.png"
if (Test-Path $iconIco) {
    Copy-Item $iconIco "$bundleDir\icon.ico" -Force
    Write-Host "  Copied icon.ico" -ForegroundColor Green
}
if (Test-Path $iconPng) {
    Copy-Item $iconPng "$bundleDir\icon.png" -Force
    Write-Host "  Copied icon.png" -ForegroundColor Green
}

# Step 3: Build the installer executable
Write-Host "Building installer executable..." -ForegroundColor Yellow

$outputDir = Split-Path -Parent $OutputPath
if (-not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

Push-Location $repoRoot
try {
    # Build with Windows GUI subsystem (no console window)
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -ldflags="-s -w -H=windowsgui" -o $OutputPath ./cmd/installer
}
finally {
    Pop-Location
}

if (Test-Path $OutputPath) {
    $size = (Get-Item $OutputPath).Length / 1MB
    Write-Host "Build complete: $OutputPath ($([math]::Round($size, 2)) MB)" -ForegroundColor Green
} else {
    Write-Host "Build failed: Output file not created" -ForegroundColor Red
    exit 1
}
