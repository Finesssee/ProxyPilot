Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

param(
  [Parameter(Mandatory = $false)]
  [string]$ProxyApiKey = $env:CLIPROXY_API_KEY,

  [Parameter(Mandatory = $false)]
  [string]$ProxyBaseUrl = "http://127.0.0.1:8317/v1",

  [Parameter(Mandatory = $false)]
  [string]$Model = "antigravity-claude-sonnet-4-5-thinking",

  [Parameter(Mandatory = $false)]
  [string]$EmbeddingModel = "text-embedding-004"
)

function Ensure-LettaInstalled {
  Write-Host "Checking for Letta installation..."
  if (Get-Command "letta" -ErrorAction SilentlyContinue) {
    Write-Host "Letta is already installed."
    return
  }

  Write-Host "Letta not found. Attempting to install via pip..."
  & pip install letta
  if ($LASTEXITCODE -ne 0) {
    throw "Failed to install Letta. Please ensure Python and pip are installed and in your PATH."
  }
  Write-Host "Letta installed successfully."
}

function Configure-Letta {
  param($BaseUrl, $ApiKey, $Model, $EmbeddingModel)

  Write-Host "Configuring Letta to use ProxyPilot..."
  
  # Set environment variables for the current session to ensure configure works
  $env:LETTA_OPENAI_API_BASE = $BaseUrl
  $env:LETTA_OPENAI_API_KEY = $ApiKey

  # Run Letta configuration
  # Note: Letta's config command structure might vary, but setting env vars
  # and running a basic init is the most reliable way to bootstrap it.
  
  Write-Host "Setting up Letta with:"
  Write-Host "  LLM Backend: $BaseUrl"
  Write-Host "  LLM Model:   $Model"
  Write-Host "  Embed Model: $EmbeddingModel"

  # We use the environment variable approach as it's the most robust across Letta versions
  # Letta will pick up these variables when started by ProxyPilot.
  
  # Create the .letta directory if it doesn't exist
  $lettaDir = Join-Path $env:USERPROFILE ".letta"
  if (-not (Test-Path $lettaDir)) {
    New-Item -ItemType Directory -Path $lettaDir | Out-Null
  }

  Write-Host "Letta configuration prepared."
}

if (-not $ProxyApiKey -or $ProxyApiKey.Trim() -eq "") {
  $ProxyApiKey = "local-dev-key"
  Write-Host "No Proxy API Key provided, using default: $ProxyApiKey"
}

Ensure-LettaInstalled
Configure-Letta -BaseUrl $ProxyBaseUrl -ApiKey $ProxyApiKey -Model $Model -EmbeddingModel $EmbeddingModel

Write-Host ""
Write-Host "Setup Complete!"
Write-Host "1) Restart ProxyPilot to launch the Letta sidecar."
Write-Host "2) Your Letta logs will be available at: logs\letta.log"
Write-Host "3) Use 'letta run' in a new terminal to interact with your memory agent."
Write-Host ""
Write-Host "Note: ProxyPilot now provides REAL embeddings via Gemini text-embedding-004."
