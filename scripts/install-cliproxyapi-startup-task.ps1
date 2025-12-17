Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

& (Join-Path $PSScriptRoot "windows\\install-cliproxyapi-startup-task.ps1") @args

