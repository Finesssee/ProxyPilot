# ProxyPilot Premium Installer Graphics Generator
# Creates distinctive, high-quality wizard images

param(
    [string]$SourceIcon = "..\..\static\icon.png",
    [string]$OutputDir = "."
)

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms

$ErrorActionPreference = "Stop"

# Premium color palette extracted from the icon
$Colors = @{
    DeepNavy      = [System.Drawing.Color]::FromArgb(255, 15, 23, 42)      # #0F172A
    RichBlue      = [System.Drawing.Color]::FromArgb(255, 30, 58, 138)     # #1E3A8A
    VibrantBlue   = [System.Drawing.Color]::FromArgb(255, 59, 130, 246)    # #3B82F6
    SkyBlue       = [System.Drawing.Color]::FromArgb(255, 96, 165, 250)    # #60A5FA
    LightBlue     = [System.Drawing.Color]::FromArgb(255, 147, 197, 253)   # #93C5FD
    PureWhite     = [System.Drawing.Color]::FromArgb(255, 255, 255, 255)
    SoftWhite     = [System.Drawing.Color]::FromArgb(255, 248, 250, 252)   # #F8FAFC
    GlowBlue      = [System.Drawing.Color]::FromArgb(80, 96, 165, 250)     # Semi-transparent glow
    SubtleGlow    = [System.Drawing.Color]::FromArgb(30, 147, 197, 253)
}

function Add-Noise {
    param(
        [System.Drawing.Graphics]$g,
        [int]$width,
        [int]$height,
        [int]$intensity = 8
    )

    $random = New-Object System.Random
    for ($i = 0; $i -lt ($width * $height / 50); $i++) {
        $x = $random.Next($width)
        $y = $random.Next($height)
        $alpha = $random.Next(5, $intensity)
        $color = [System.Drawing.Color]::FromArgb($alpha, 255, 255, 255)
        $brush = New-Object System.Drawing.SolidBrush($color)
        $g.FillRectangle($brush, $x, $y, 1, 1)
        $brush.Dispose()
    }
}

function Draw-FlowingLine {
    param(
        [System.Drawing.Graphics]$g,
        [float]$startX,
        [float]$startY,
        [float]$endX,
        [float]$endY,
        [System.Drawing.Color]$color,
        [float]$width = 1.5
    )

    $pen = New-Object System.Drawing.Pen($color, $width)
    $pen.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
    $pen.EndCap = [System.Drawing.Drawing2D.LineCap]::Round

    # Create curved path
    $path = New-Object System.Drawing.Drawing2D.GraphicsPath

    $ctrlX1 = $startX + ($endX - $startX) * 0.3
    $ctrlY1 = $startY + ($endY - $startY) * 0.1
    $ctrlX2 = $startX + ($endX - $startX) * 0.7
    $ctrlY2 = $endY - ($endY - $startY) * 0.1

    $path.AddBezier($startX, $startY, $ctrlX1, $ctrlY1, $ctrlX2, $ctrlY2, $endX, $endY)
    $g.DrawPath($pen, $path)

    # Draw node at end
    $nodeBrush = New-Object System.Drawing.SolidBrush($color)
    $g.FillEllipse($nodeBrush, ($endX - 2), ($endY - 2), 4, 4)

    $pen.Dispose()
    $path.Dispose()
    $nodeBrush.Dispose()
}

function New-PremiumWizardLarge {
    param([string]$IconPath, [string]$OutputPath)

    $width = 164
    $height = 314

    $bitmap = New-Object System.Drawing.Bitmap($width, $height, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
    $g = [System.Drawing.Graphics]::FromImage($bitmap)
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality

    # === BACKGROUND: Rich multi-stop gradient ===
    $gradientRect = New-Object System.Drawing.Rectangle(0, 0, $width, $height)

    # Primary gradient: Deep navy to rich blue
    $mainGradient = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        (New-Object System.Drawing.Point(0, 0)),
        (New-Object System.Drawing.Point(0, $height)),
        $Colors.DeepNavy,
        $Colors.RichBlue
    )
    $g.FillRectangle($mainGradient, $gradientRect)
    $mainGradient.Dispose()

    # Overlay: Subtle radial-like glow from center-top
    $glowPath = New-Object System.Drawing.Drawing2D.GraphicsPath
    $glowPath.AddEllipse(-50, -100, $width + 100, 300)
    $glowBrush = New-Object System.Drawing.Drawing2D.PathGradientBrush($glowPath)
    $glowBrush.CenterColor = [System.Drawing.Color]::FromArgb(40, 59, 130, 246)
    $glowBrush.SurroundColors = @([System.Drawing.Color]::FromArgb(0, 59, 130, 246))
    $g.FillPath($glowBrush, $glowPath)
    $glowBrush.Dispose()
    $glowPath.Dispose()

    # === DECORATIVE ELEMENTS: Flowing network lines ===
    $lineColor1 = [System.Drawing.Color]::FromArgb(25, 147, 197, 253)
    $lineColor2 = [System.Drawing.Color]::FromArgb(35, 96, 165, 250)
    $lineColor3 = [System.Drawing.Color]::FromArgb(20, 255, 255, 255)

    # Network lines emanating from where icon will be
    $centerX = $width / 2
    $iconBottom = 140

    Draw-FlowingLine $g $centerX $iconBottom 20 200 $lineColor1 1.2
    Draw-FlowingLine $g $centerX $iconBottom 144 210 $lineColor2 1.0
    Draw-FlowingLine $g $centerX $iconBottom 30 240 $lineColor3 0.8
    Draw-FlowingLine $g $centerX $iconBottom 134 250 $lineColor1 1.0
    Draw-FlowingLine $g $centerX $iconBottom 50 270 $lineColor2 0.8
    Draw-FlowingLine $g $centerX $iconBottom 114 280 $lineColor3 0.6

    # === SUBTLE NOISE TEXTURE ===
    Add-Noise $g $width $height 6

    # === ICON: Load and draw with glow effect ===
    if (Test-Path $IconPath) {
        $icon = [System.Drawing.Image]::FromFile((Resolve-Path $IconPath))
        $iconSize = 90
        $iconX = ($width - $iconSize) / 2
        $iconY = 45

        # Glow behind icon
        $glowSize = $iconSize + 30
        $glowX = ($width - $glowSize) / 2
        $glowY = $iconY - 15

        $iconGlowPath = New-Object System.Drawing.Drawing2D.GraphicsPath
        $iconGlowPath.AddEllipse($glowX, $glowY, $glowSize, $glowSize)
        $iconGlowBrush = New-Object System.Drawing.Drawing2D.PathGradientBrush($iconGlowPath)
        $iconGlowBrush.CenterColor = [System.Drawing.Color]::FromArgb(60, 96, 165, 250)
        $iconGlowBrush.SurroundColors = @([System.Drawing.Color]::FromArgb(0, 96, 165, 250))
        $g.FillPath($iconGlowBrush, $iconGlowPath)
        $iconGlowBrush.Dispose()
        $iconGlowPath.Dispose()

        # Draw the icon
        $g.DrawImage($icon, $iconX, $iconY, $iconSize, $iconSize)
        $icon.Dispose()
    }

    # === TYPOGRAPHY: Product name ===
    # Try to use Segoe UI Semibold for modern Windows feel, fall back gracefully
    $titleFont = $null
    try {
        $titleFont = New-Object System.Drawing.Font("Segoe UI Semibold", 16, [System.Drawing.FontStyle]::Regular)
    } catch {
        $titleFont = New-Object System.Drawing.Font("Segoe UI", 16, [System.Drawing.FontStyle]::Bold)
    }

    $titleBrush = New-Object System.Drawing.SolidBrush($Colors.PureWhite)
    $titleFormat = New-Object System.Drawing.StringFormat
    $titleFormat.Alignment = [System.Drawing.StringAlignment]::Center
    $titleFormat.LineAlignment = [System.Drawing.StringAlignment]::Center

    $titleRect = New-Object System.Drawing.RectangleF(0, 150, $width, 30)
    $g.DrawString("ProxyPilot", $titleFont, $titleBrush, $titleRect, $titleFormat)
    $titleFont.Dispose()

    # === TAGLINE ===
    $tagFont = New-Object System.Drawing.Font("Segoe UI", 8.5, [System.Drawing.FontStyle]::Regular)
    $tagBrush = New-Object System.Drawing.SolidBrush($Colors.LightBlue)
    $tagRect = New-Object System.Drawing.RectangleF(0, 178, $width, 20)
    $g.DrawString("Local AI Proxy", $tagFont, $tagBrush, $tagRect, $titleFormat)
    $tagFont.Dispose()
    $tagBrush.Dispose()

    # === DECORATIVE SEPARATOR LINE ===
    $separatorPen = New-Object System.Drawing.Pen([System.Drawing.Color]::FromArgb(60, 96, 165, 250), 1)
    $g.DrawLine($separatorPen, 30, 205, $width - 30, 205)
    $separatorPen.Dispose()

    # === VERSION/SETUP TEXT ===
    $setupFont = New-Object System.Drawing.Font("Segoe UI", 7.5, [System.Drawing.FontStyle]::Regular)
    $setupBrush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(140, 248, 250, 252))
    $setupRect = New-Object System.Drawing.RectangleF(0, 290, $width, 18)
    $g.DrawString("Setup Wizard", $setupFont, $setupBrush, $setupRect, $titleFormat)
    $setupFont.Dispose()
    $setupBrush.Dispose()

    # === BOTTOM ACCENT: Subtle gradient bar ===
    $accentY = $height - 4
    $accentRect = New-Object System.Drawing.Rectangle(0, $accentY, $width, 4)
    $accentGradient = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        (New-Object System.Drawing.Point(0, 0)),
        (New-Object System.Drawing.Point($width, 0)),
        [System.Drawing.Color]::FromArgb(100, 30, 58, 138),
        $Colors.VibrantBlue
    )
    $g.FillRectangle($accentGradient, $accentRect)
    $accentGradient.Dispose()

    # Cleanup
    $titleBrush.Dispose()
    $titleFormat.Dispose()
    $g.Dispose()

    # Save as 24-bit BMP
    $bitmap.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Bmp)
    $bitmap.Dispose()

    Write-Host "  Created: wizard-large.bmp (164x314)" -ForegroundColor Green
}

function New-PremiumWizardSmall {
    param([string]$IconPath, [string]$OutputPath)

    $size = 55

    $bitmap = New-Object System.Drawing.Bitmap($size, $size, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
    $g = [System.Drawing.Graphics]::FromImage($bitmap)
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality

    # Gradient background matching large panel
    $gradient = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        (New-Object System.Drawing.Point(0, 0)),
        (New-Object System.Drawing.Point(0, $size)),
        $Colors.DeepNavy,
        $Colors.RichBlue
    )
    $g.FillRectangle($gradient, 0, 0, $size, $size)
    $gradient.Dispose()

    # Subtle glow in center
    $glowPath = New-Object System.Drawing.Drawing2D.GraphicsPath
    $glowDim = $size - 10
    $glowPath.AddEllipse(5, 5, $glowDim, $glowDim)
    $glowBrush = New-Object System.Drawing.Drawing2D.PathGradientBrush($glowPath)
    $glowBrush.CenterColor = [System.Drawing.Color]::FromArgb(50, 59, 130, 246)
    $glowBrush.SurroundColors = @([System.Drawing.Color]::FromArgb(0, 59, 130, 246))
    $g.FillPath($glowBrush, $glowPath)
    $glowBrush.Dispose()
    $glowPath.Dispose()

    # Draw icon
    if (Test-Path $IconPath) {
        $icon = [System.Drawing.Image]::FromFile((Resolve-Path $IconPath))
        $iconSize = 42
        $offset = ($size - $iconSize) / 2
        $g.DrawImage($icon, $offset, $offset, $iconSize, $iconSize)
        $icon.Dispose()
    }

    # Subtle border
    $borderPen = New-Object System.Drawing.Pen([System.Drawing.Color]::FromArgb(40, 96, 165, 250), 1)
    $borderSize = $size - 1
    $g.DrawRectangle($borderPen, 0, 0, $borderSize, $borderSize)
    $borderPen.Dispose()

    $g.Dispose()

    $bitmap.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Bmp)
    $bitmap.Dispose()

    Write-Host "  Created: wizard-small.bmp (55x55)" -ForegroundColor Green
}

# === MAIN EXECUTION ===
Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  ProxyPilot Premium Installer Graphics" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

try {
    $iconFullPath = Resolve-Path $SourceIcon -ErrorAction Stop
    Write-Host "Source: $iconFullPath" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "Generating premium wizard images..." -ForegroundColor Yellow
    Write-Host ""

    New-PremiumWizardLarge -IconPath $iconFullPath -OutputPath (Join-Path $OutputDir "wizard-large.bmp")
    New-PremiumWizardSmall -IconPath $iconFullPath -OutputPath (Join-Path $OutputDir "wizard-small.bmp")

    Write-Host ""
    Write-Host "Done! Premium graphics generated." -ForegroundColor Green
    Write-Host ""
}
catch {
    Write-Host "Error: $_" -ForegroundColor Red
    Write-Host $_.ScriptStackTrace -ForegroundColor DarkRed
    exit 1
}
finally {
    Pop-Location
}
