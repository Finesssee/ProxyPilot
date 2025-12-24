Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

param(
  [Parameter(Mandatory = $false)]
  [string]$ProxyApiKey = $env:CLIPROXY_API_KEY,

  [Parameter(Mandatory = $false)]
  [string]$ProxyBaseUrl = "http://127.0.0.1:8318/v1",

  [Parameter(Mandatory = $false)]
  [string]$ProxyModel = "gpt-5.2"
)

function Get-RepoRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path
}

function Ensure-Dir([string]$Path) {
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
}

function Build-CLIProxy([string]$RepoRoot) {
  $binDir = Join-Path $RepoRoot "bin"
  Ensure-Dir $binDir

  $exePath = Join-Path $binDir "proxypilot-engine.exe"
  $compatPath = Join-Path $binDir "cliproxyapi-latest.exe"

  Write-Host "Building ProxyPilot Engine -> $exePath"
  Push-Location $RepoRoot
  try {
    & go build -o $exePath .\cmd\server
    Copy-Item -Force -LiteralPath $exePath -Destination $compatPath
  } finally {
    Pop-Location
  }

  return $exePath
}

function Ensure-DroidConfig([string]$BaseUrl, [string]$ApiKey, [string]$Model) {
  $factoryDir = Join-Path $env:USERPROFILE ".factory"
  Ensure-Dir $factoryDir

  $configPath = Join-Path $factoryDir "config.json"
  $backupPath = Join-Path $factoryDir ("config.json.bak." + (Get-Date -Format "yyyyMMdd-HHmmss"))

  $existingConfig = $null
  if (Test-Path -LiteralPath $configPath) {
    Copy-Item -LiteralPath $configPath -Destination $backupPath -Force
    Write-Host "Backed up existing Droid config -> $backupPath"
    try {
      $existingConfig = Get-Content -Raw -LiteralPath $configPath | ConvertFrom-Json
    } catch {
      $existingConfig = $null
    }
  }
  if (-not $existingConfig) {
    $existingConfig = [PSCustomObject]@{}
  }

  $modelsToAdd = @(
    "gpt-5.2",
    "gpt-5.2(low)",
    "gpt-5.2(medium)",
    "gpt-5.2(high)",
    "gpt-5.2(xhigh)",
    "gpt-5.1-codex-max",
    "gpt-5.1-codex-max(low)",
    "gpt-5.1-codex-max(medium)",
    "gpt-5.1-codex-max(high)",
    "gpt-5.1-codex-max(xhigh)",
    "gemini-3-pro-preview",
    "antigravity-claude-sonnet-4-5-thinking",
    "antigravity-claude-opus-4-5-thinking"
  )

  $modelsToRemove = @(
    "gpt-5-codex",
    "gpt-5-codex-mini",
    "gpt-5",
    "gemini-3-pro-image-preview",
    "gemini-2.5-flash-lite",
    "gemini-claude-sonnet-4-5",
    "gpt-5.1",
    "gemini-2.5-flash",
    "gemini-2.5-computer-use-preview-10-2025",
    "gpt-5.1-codex-mini",
    "gpt-5.1-codex"
  )

  $newEntries = $modelsToAdd | ForEach-Object {
    $modelId = $_
    $displayModelId = $modelId
    if ($modelId -match '\((low|medium|high|xhigh|none|auto)\)$') {
      $displayModelId = ($modelId -replace '\((low|medium|high|xhigh|none|auto)\)$', ' (reasoning: $1)')
    }
    [PSCustomObject]@{
      model_display_name = "ProxyPilot (local): $displayModelId"
      model              = $modelId
      base_url           = $BaseUrl
      api_key            = $ApiKey
      provider           = "openai"
    }
  }

  $existingModels = @()
  if ($existingConfig.PSObject.Properties.Name -contains "custom_models" -and $existingConfig.custom_models) {
    $existingModels = @($existingConfig.custom_models)
  }

  $filteredExisting = $existingModels | Where-Object {
    $m = $_
    -not (
      $m -and
      ($m.PSObject.Properties.Name -contains "base_url") -and
      ($m.PSObject.Properties.Name -contains "model") -and
      ($m.base_url -eq $BaseUrl) -and
      (
        ($modelsToAdd -contains [string]$m.model) -or
        ($modelsToRemove -contains [string]$m.model)
      )
    )
  }

  $existingConfig | Add-Member -NotePropertyName "custom_models" -NotePropertyValue (@($filteredExisting) + @($newEntries)) -Force

  $json = $existingConfig | ConvertTo-Json -Depth 20
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($configPath, $json, $utf8NoBom)
  Write-Host "Wrote Droid BYOK config -> $configPath"
}

function Write-UsageHint([string]$ApiKey, [string]$BaseUrl) {
  Write-Host ""
  Write-Host "Next:"
  Write-Host "  1) Start the proxy:   scripts\\start-cliproxy.ps1"
  Write-Host "  2) In Droid:          /model  -> select 'CLIProxy (local)'"
  Write-Host ""
  Write-Host "Proxy auth (Droid api_key): $ApiKey"
  Write-Host "Proxy base_url:            $BaseUrl"
}

$repoRoot = Get-RepoRoot
$configPath = Join-Path $repoRoot "config.yaml"

if (-not (Test-Path -LiteralPath $configPath)) {
  throw "Expected config file not found: $configPath"
}

if (-not $ProxyApiKey -or $ProxyApiKey.Trim() -eq "") {
  throw "Missing proxy API key. Provide -ProxyApiKey <key> or set env var CLIPROXY_API_KEY."
}

Build-CLIProxy -RepoRoot $repoRoot | Out-Null
Ensure-DroidConfig -BaseUrl $ProxyBaseUrl -ApiKey $ProxyApiKey -Model $ProxyModel
Write-UsageHint -ApiKey $ProxyApiKey -BaseUrl $ProxyBaseUrl
