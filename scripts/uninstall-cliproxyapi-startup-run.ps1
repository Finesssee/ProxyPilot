Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

& (Join-Path $PSScriptRoot "windows\\uninstall-cliproxyapi-startup-run.ps1") @args

