$paths = @(
    "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
    "C:\Program Files\Inno Setup 6\ISCC.exe",
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
)

foreach ($path in $paths) {
    if (Test-Path $path) {
        Write-Host "FOUND: $path"
        exit 0
    }
}

# Search more broadly
$found = Get-ChildItem -Path "C:\" -Filter "ISCC.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
if ($found) {
    Write-Host "FOUND: $($found.FullName)"
} else {
    Write-Host "NOT FOUND - Please install Inno Setup"
}
