param(
  [string]$BaseUrl = "http://127.0.0.1:9080",
  [string]$ManagementKey = "",
  [switch]$CheckSemantic = $true
)

$ErrorActionPreference = "Stop"

function Invoke-HealthRequest {
  param([string]$Url, [hashtable]$Headers = $null)
  try {
    $resp = Invoke-WebRequest -Uri $Url -Headers $Headers -Method Get -TimeoutSec 5
    return @{ ok = $true; status = $resp.StatusCode; url = $Url }
  } catch {
    return @{ ok = $false; status = $_.Exception.Message; url = $Url }
  }
}

$results = @()
$results += Invoke-HealthRequest "$BaseUrl/healthz"

if ($CheckSemantic) {
  $headers = @{}
  if ($ManagementKey) { $headers["X-Management-Key"] = $ManagementKey }
  $results += Invoke-HealthRequest "$BaseUrl/management/semantic/health" $headers
}

Write-Host "ProxyPilot health check"
foreach ($r in $results) {
  $status = if ($r.ok) { "OK" } else { "FAIL" }
  Write-Host ("[{0}] {1} -> {2}" -f $status, $r.url, $r.status)
}

if ($results | Where-Object { -not $_.ok }) { exit 1 }
