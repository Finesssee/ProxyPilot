# ProxyPilot Installer - Wizard Image Generator
# Generates branded wizard images for Inno Setup installer

param(
    [string]$SourceIcon = "..\..\static\icon.png",
    [string]$OutputDir = "."
)

Add-Type -AssemblyName System.Drawing

$ErrorActionPreference = "Stop"

# ProxyPilot brand colors
$BrandColors = @{
    Primary    = [System.Drawing.Color]::FromArgb(59, 130, 246)    # Blue
    Secondary  = [System.Drawing.Color]::FromArgb(30, 41, 59)      # Dark slate
    Background = [System.Drawing.Color]::FromArgb(15, 23, 42)      # Darker slate
    Text       = [System.Drawing.Color]::FromArgb(248, 250, 252)   # Light text
}

function New-WizardLargeImage {
    param([string]$IconPath, [string]$OutputPath)

    $width = 164
    $height = 314

    $bitmap = New-Object System.Drawing.Bitmap($width, $height)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic

    # Create gradient background
    $gradientBrush = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        (New-Object System.Drawing.Point(0, 0)),
        (New-Object System.Drawing.Point(0, $height)),
        $BrandColors.Background,
        $BrandColors.Secondary
    )
    $graphics.FillRectangle($gradientBrush, 0, 0, $width, $height)

    # Load and draw icon
    if (Test-Path $IconPath) {
        $icon = [System.Drawing.Image]::FromFile((Resolve-Path $IconPath))
        $iconSize = 80
        $iconX = ($width - $iconSize) / 2
        $iconY = 60
        $graphics.DrawImage($icon, $iconX, $iconY, $iconSize, $iconSize)
        $icon.Dispose()
    }

    # Draw product name
    $font = New-Object System.Drawing.Font("Segoe UI", 14, [System.Drawing.FontStyle]::Bold)
    $textBrush = New-Object System.Drawing.SolidBrush($BrandColors.Text)
    $textFormat = New-Object System.Drawing.StringFormat
    $textFormat.Alignment = [System.Drawing.StringAlignment]::Center

    $textRect = New-Object System.Drawing.RectangleF(0, 160, $width, 30)
    $graphics.DrawString("ProxyPilot", $font, $textBrush, $textRect, $textFormat)
    $font.Dispose()

    # Draw tagline
    $smallFont = New-Object System.Drawing.Font("Segoe UI", 8, [System.Drawing.FontStyle]::Regular)
    $taglineRect = New-Object System.Drawing.RectangleF(0, 190, $width, 40)
    $graphics.DrawString("Local AI Proxy", $smallFont, $textBrush, $taglineRect, $textFormat)
    $smallFont.Dispose()

    # Draw decorative line
    $linePen = New-Object System.Drawing.Pen($BrandColors.Primary, 2)
    $graphics.DrawLine($linePen, 30, 240, $width - 30, 240)
    $linePen.Dispose()

    # Draw version indicator area
    $versionFont = New-Object System.Drawing.Font("Segoe UI", 7, [System.Drawing.FontStyle]::Regular)
    $versionRect = New-Object System.Drawing.RectangleF(0, 280, $width, 20)
    $dimBrush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(150, 248, 250, 252))
    $graphics.DrawString("Setup Wizard", $versionFont, $dimBrush, $versionRect, $textFormat)
    $versionFont.Dispose()
    $dimBrush.Dispose()

    # Clean up
    $textBrush.Dispose()
    $textFormat.Dispose()
    $gradientBrush.Dispose()
    $graphics.Dispose()

    # Save as 24-bit BMP (required by Inno Setup)
    $bitmap.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Bmp)
    $bitmap.Dispose()

    Write-Host "Created: $OutputPath" -ForegroundColor Green
}

function New-WizardSmallImage {
    param([string]$IconPath, [string]$OutputPath)

    $size = 55

    $bitmap = New-Object System.Drawing.Bitmap($size, $size)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic

    # Fill with brand background
    $backgroundBrush = New-Object System.Drawing.SolidBrush($BrandColors.Secondary)
    $graphics.FillRectangle($backgroundBrush, 0, 0, $size, $size)
    $backgroundBrush.Dispose()

    # Load and draw icon
    if (Test-Path $IconPath) {
        $icon = [System.Drawing.Image]::FromFile((Resolve-Path $IconPath))
        $iconSize = 45
        $offset = ($size - $iconSize) / 2
        $graphics.DrawImage($icon, $offset, $offset, $iconSize, $iconSize)
        $icon.Dispose()
    }

    $graphics.Dispose()

    # Save as 24-bit BMP
    $bitmap.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Bmp)
    $bitmap.Dispose()

    Write-Host "Created: $OutputPath" -ForegroundColor Green
}

# Main execution
Write-Host ""
Write-Host "ProxyPilot Installer - Wizard Image Generator" -ForegroundColor Cyan
Write-Host "=============================================" -ForegroundColor Cyan
Write-Host ""

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

try {
    $iconFullPath = Resolve-Path $SourceIcon -ErrorAction Stop
    Write-Host "Source icon: $iconFullPath"
    Write-Host ""

    New-WizardLargeImage -IconPath $iconFullPath -OutputPath (Join-Path $OutputDir "wizard-large.bmp")
    New-WizardSmallImage -IconPath $iconFullPath -OutputPath (Join-Path $OutputDir "wizard-small.bmp")

    Write-Host ""
    Write-Host "Done! Wizard images created successfully." -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "1. Uncomment the WizardImageFile lines in proxypilot.iss"
    Write-Host "2. Rebuild the installer"
    Write-Host ""
}
catch {
    Write-Host "Error: $_" -ForegroundColor Red
    exit 1
}
finally {
    Pop-Location
}
