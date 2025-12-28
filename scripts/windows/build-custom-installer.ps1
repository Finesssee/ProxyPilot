# Build ProxyPilot Custom WebView2-based Installer
#
# This script builds the custom installer with embedded UI and application files:
#   1. Build installer UI (React/Vite)
#   2. Copy UI dist to cmd/installer/ui/
#   3. Build main ProxyPilot.exe
#   4. Build ProxyPilot-Installer.exe with embedded UI and app files
#
# Output: dist/ProxyPilot-Installer.exe

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}

function Ensure-Dir([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Write-Step([string]$StepNumber, [string]$Message) {
    Write-Host ""
    Write-Host "[$StepNumber] $Message" -ForegroundColor Cyan
    Write-Host ("-" * 50) -ForegroundColor DarkGray
}

function Write-Success([string]$Message) {
    Write-Host "  [OK] $Message" -ForegroundColor Green
}

function Write-Error-Exit([string]$Message) {
    Write-Host "  [ERROR] $Message" -ForegroundColor Red
    exit 1
}

$repoRoot = Get-RepoRoot
$installerUiDir = Join-Path $repoRoot "installer-ui"
$installerUiDist = Join-Path $installerUiDir "dist"
$cmdInstallerUiDir = Join-Path $repoRoot "cmd\installer\ui"
$distDir = Join-Path $repoRoot "dist"

Write-Host ""
Write-Host "ProxyPilot Custom Installer Build" -ForegroundColor White
Write-Host "==================================" -ForegroundColor White

# -----------------------------------------------------------------------------
# Step 1: Build Installer UI
# -----------------------------------------------------------------------------
Write-Step "1/4" "Building Installer UI (React/Vite)"

if (-not (Test-Path -LiteralPath $installerUiDir)) {
    Write-Error-Exit "installer-ui directory not found at: $installerUiDir"
}

Push-Location $installerUiDir
try {
    Write-Host "  Running npm ci..."
    npm ci --silent 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Error-Exit "npm ci failed"
    }
    Write-Success "Dependencies installed"

    Write-Host "  Running npm run build..."
    npm run build 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Error-Exit "npm run build failed"
    }
    Write-Success "UI built successfully"
} finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $installerUiDist)) {
    Write-Error-Exit "Build output not found at: $installerUiDist"
}

# -----------------------------------------------------------------------------
# Step 2: Copy UI dist to cmd/installer/ui/
# -----------------------------------------------------------------------------
Write-Step "2/4" "Copying UI assets to cmd/installer/ui/"

# Clean and recreate the target directory
if (Test-Path -LiteralPath $cmdInstallerUiDir) {
    Remove-Item -Recurse -Force -LiteralPath $cmdInstallerUiDir
}
Ensure-Dir $cmdInstallerUiDir

# Copy all files from installer-ui/dist to cmd/installer/ui/
Copy-Item -Path "$installerUiDist\*" -Destination $cmdInstallerUiDir -Recurse -Force

$fileCount = (Get-ChildItem -Path $cmdInstallerUiDir -Recurse -File).Count
Write-Success "Copied $fileCount files to cmd/installer/ui/"

# -----------------------------------------------------------------------------
# Step 3: Build ProxyPilot.exe (main application)
# -----------------------------------------------------------------------------
Write-Step "3/4" "Building ProxyPilot.exe"

Ensure-Dir $distDir

$proxyPilotExe = Join-Path $distDir "ProxyPilot.exe"

Push-Location $repoRoot
try {
    Write-Host "  Compiling cmd/cliproxytray..."
    go build -ldflags="-s -w -H=windowsgui" -o $proxyPilotExe .\cmd\cliproxytray

    if ($LASTEXITCODE -ne 0) {
        Write-Error-Exit "Failed to build ProxyPilot.exe"
    }
    Write-Success "Built: $proxyPilotExe"
} finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $proxyPilotExe)) {
    Write-Error-Exit "ProxyPilot.exe not found after build"
}

# -----------------------------------------------------------------------------
# Step 4: Build ProxyPilot-Installer.exe
# -----------------------------------------------------------------------------
Write-Step "4/4" "Building ProxyPilot-Installer.exe"

$installerExe = Join-Path $distDir "ProxyPilot-Installer.exe"

# Check if cmd/installer exists
$cmdInstallerDir = Join-Path $repoRoot "cmd\installer"
if (-not (Test-Path -LiteralPath $cmdInstallerDir)) {
    Write-Error-Exit "cmd/installer directory not found. Please create the installer Go package first."
}

Push-Location $repoRoot
try {
    Write-Host "  Compiling cmd/installer..."
    go build -ldflags="-s -w -H=windowsgui" -o $installerExe .\cmd\installer

    if ($LASTEXITCODE -ne 0) {
        Write-Error-Exit "Failed to build ProxyPilot-Installer.exe"
    }
    Write-Success "Built: $installerExe"
} finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $installerExe)) {
    Write-Error-Exit "ProxyPilot-Installer.exe not found after build"
}

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Build Complete!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Output files:" -ForegroundColor White
Write-Host "  - $proxyPilotExe" -ForegroundColor Yellow
Write-Host "  - $installerExe" -ForegroundColor Yellow
Write-Host ""

# Display file sizes
$proxyPilotSize = [math]::Round((Get-Item $proxyPilotExe).Length / 1MB, 2)
$installerSize = [math]::Round((Get-Item $installerExe).Length / 1MB, 2)
Write-Host "File sizes:" -ForegroundColor White
Write-Host "  - ProxyPilot.exe: $proxyPilotSize MB" -ForegroundColor DarkGray
Write-Host "  - ProxyPilot-Installer.exe: $installerSize MB" -ForegroundColor DarkGray
Write-Host ""
