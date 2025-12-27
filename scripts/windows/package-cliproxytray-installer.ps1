Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path
}

function Ensure-Dir([string]$Path) {
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
}

$repoRoot = Get-RepoRoot
$iexpress = Join-Path $env:WINDIR "System32\\iexpress.exe"
if (-not (Test-Path -LiteralPath $iexpress)) {
  throw "IExpress not found at: $iexpress. Install/enable IExpress or use the zip package script instead."
}

# Build single binary (tray app with embedded engine)
& (Join-Path $repoRoot "scripts\\windows\\build-cliproxytray.ps1")

$distRoot = Join-Path $repoRoot "dist"
Ensure-Dir $distRoot

$staging = Join-Path $distRoot "ProxyPilot-Staging"
if (Test-Path -LiteralPath $staging) { Remove-Item -Recurse -Force -LiteralPath $staging }
Ensure-Dir $staging

# Payload files (single binary with embedded engine)
$mgrExe = Join-Path $repoRoot "bin\\ProxyPilot.exe"
$cfgSrc = Join-Path $repoRoot "config.example.yaml"

Copy-Item -Force -LiteralPath $mgrExe -Destination (Join-Path $staging "ProxyPilot.exe")
if (Test-Path -LiteralPath $cfgSrc) {
  Copy-Item -Force -LiteralPath $cfgSrc -Destination (Join-Path $staging "config.example.yaml")
}

# Start script (runs manager with repo-local defaults)
$runCmd = Join-Path $staging "run-manager.cmd"
@"
@echo off
setlocal
start """" ""%~dp0ProxyPilot.exe""
"@ | Set-Content -Encoding ASCII -LiteralPath $runCmd

# IExpress configuration (SED)
$sedPath = Join-Path $staging "package.sed"
$outExe = Join-Path $distRoot "ProxyPilot-Setup.exe"
if (Test-Path -LiteralPath $outExe) { Remove-Item -Force -LiteralPath $outExe }

$escapedStaging = $staging.Replace("\", "\\")
$escapedOutExe = $outExe.Replace("\", "\\")

@"
[Version]
Class=IExpress
SEDVersion=3
[Options]
PackagePurpose=InstallApp
ShowInstallProgramWindow=0
HideExtractAnimation=1
UseLongFileName=1
InsideCompressed=0
CAB_FixedSize=0
CAB_ResvCodeSigning=0
RebootMode=N
InstallPrompt=
DisplayLicense=
FinishMessage=
TargetName=$escapedOutExe
FriendlyName=ProxyPilot
AppLaunched=run-manager.cmd
PostInstallCmd=
AdminQuietInstCmd=
UserQuietInstCmd=
SourceFiles=SourceFiles
[SourceFiles]
SourceFiles0=$escapedStaging
[SourceFiles0]
%FILE0%=ProxyPilot.exe
%FILE1%=config.example.yaml
%FILE2%=run-manager.cmd
[Strings]
FILE0=ProxyPilot.exe
FILE1=config.example.yaml
FILE2=run-manager.cmd
"@ | Set-Content -Encoding ASCII -LiteralPath $sedPath

# If config.example.yaml was absent, remove it from the SED to avoid build failure.
if (-not (Test-Path -LiteralPath (Join-Path $staging "config.example.yaml"))) {
  $sed = Get-Content -LiteralPath $sedPath
  $sed = $sed | Where-Object { $_ -notmatch 'config\\.example\\.yaml' -and $_ -notmatch 'FILE2=' }
  $sed | Set-Content -Encoding ASCII -LiteralPath $sedPath
}

Write-Host "Building installer..."
& $iexpress /n /q $sedPath | Out-Null

if (-not (Test-Path -LiteralPath $outExe)) {
  throw "Installer build failed; expected: $outExe"
}

Write-Host "Built installer:"
Write-Host "  $outExe"
