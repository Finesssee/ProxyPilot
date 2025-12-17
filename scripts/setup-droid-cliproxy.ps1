Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

& (Join-Path $PSScriptRoot "windows\\setup-droid-cliproxy.ps1") @args

