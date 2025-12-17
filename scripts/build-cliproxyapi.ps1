Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

& (Join-Path $PSScriptRoot "windows\\build-cliproxyapi.ps1") @args

