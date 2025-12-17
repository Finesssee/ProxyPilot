Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

& (Join-Path $PSScriptRoot "windows\\uninstall-cliproxyapi-startup-task.ps1") @args

