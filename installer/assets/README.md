# ProxyPilot Installer Assets

This folder contains custom branding assets for the Inno Setup installer.

## Required Images

### 1. wizard-large.bmp (164x314 pixels)
- Displayed on the left side of the installer wizard
- Should feature the ProxyPilot logo/branding
- Must be a 24-bit BMP file (no alpha channel)

### 2. wizard-small.bmp (55x55 pixels)
- Displayed in the top-right corner of wizard pages
- Usually just the ProxyPilot icon
- Must be a 24-bit BMP file (no alpha channel)

## Generating Images

Run the PowerShell script to auto-generate from the existing icon:

```powershell
.\generate-wizard-images.ps1
```

Or manually create them:
1. Open `static/icon.png` in an image editor
2. Create a 164x314 canvas with your brand colors
3. Place the logo centered with product name below
4. Export as 24-bit BMP (no alpha)

## Enabling Custom Images

Once images are created, uncomment these lines in `proxypilot.iss`:

```ini
WizardImageFile={#RepoRoot}\installer\assets\wizard-large.bmp
WizardSmallImageFile={#RepoRoot}\installer\assets\wizard-small.bmp
```
