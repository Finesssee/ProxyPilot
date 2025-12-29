# ProxyPilot CLI + Tray Build Process

Documentation of the CLI and system tray application build and release process.

## Overview

ProxyPilot uses a **CLI + system tray architecture**:
- **proxypilot.exe** - CLI executable with all proxy functionality
- **ProxyPilot.exe** - System tray app that manages the CLI process and provides quick access to controls

The dashboard is accessed via web browser at `http://127.0.0.1:<port>/proxypilot.html`.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     ProxyPilot System                           │
│                                                                  │
│  ┌─────────────────────┐      ┌─────────────────────────────┐   │
│  │   ProxyPilot.exe    │      │      proxypilot.exe         │   │
│  │   (System Tray)     │─────▶│      (CLI / Engine)         │   │
│  │                     │      │                             │   │
│  │  - Start/Stop       │      │  - Proxy Engine (port 8317) │   │
│  │  - Open Dashboard   │      │  - Web Dashboard Server     │   │
│  │  - Copy API URL     │      │  - Provider Auth Mgmt       │   │
│  │  - Autostart toggle │      │  - Routing Engine           │   │
│  └─────────────────────┘      └─────────────────────────────┘   │
│                                                                  │
│                         ┌─────────────────────┐                  │
│                         │   Web Browser       │                  │
│                         │  (Dashboard UI)     │                  │
│                         │  proxypilot.html    │                  │
│                         └─────────────────────┘                  │
└─────────────────────────────────────────────────────────────────┘
```

### CLI + Tray Separation

The architecture separates concerns:

**proxypilot.exe (CLI)**:
- All proxy engine functionality
- Serves web dashboard at `/proxypilot.html`
- Can run standalone from command line
- Supports all CLI flags (`--help`, `--setup-claude`, etc.)

**ProxyPilot.exe (Tray)**:
- Lightweight system tray wrapper
- Manages CLI process lifecycle (start/stop/restart)
- Opens dashboard in default browser
- Provides quick-access menu

Benefits of CLI + tray architecture:
- **Flexibility**: Use CLI directly or via tray app
- **No WebView2 dependency**: Dashboard runs in any browser
- **Simpler debugging**: Inspect network/console in browser DevTools
- **Cross-platform dashboard**: Same web UI works on all platforms

## Build Process

### Prerequisites

```bash
# Required tools
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
```

Inno Setup 6 must be installed for creating the Windows installer.

### Step 1: Build Web UI Assets

```bash
cd webui
npm install
npm run build
```

This generates `webui/dist/` with the dashboard assets that are served by the CLI at `/proxypilot.html`.

### Step 2: Create Multi-Resolution Icon

The icon must have multiple sizes for proper Windows display (16px to 256px):

```python
from PIL import Image

img = Image.open('static/icon.png')
if img.mode != 'RGBA':
    img = img.convert('RGBA')

sizes = [(16,16), (24,24), (32,32), (48,48), (64,64), (128,128), (256,256)]
img.save('static/icon.ico', format='ICO', sizes=sizes)
```

Copy to tray icon location:
```bash
cp static/icon.ico internal/trayicon/icon.ico
```

### Step 3: Generate Windows Resource Files

Generate `.syso` files for embedding icons in executables:

```bash
cd cmd/cliproxytray
goversioninfo -icon="../../static/icon.ico"
```

This creates `resource.syso` files that Go automatically includes during build.

### Step 4: Build Executables

```bash
# Build CLI executable
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/proxypilot.exe ./cmd/server

# Build tray app (with -H windowsgui to hide console)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o dist/ProxyPilot.exe ./cmd/cliproxytray
```

### Step 5: Build Installer

```bash
# Using Inno Setup
"C:\Users\FSOS\AppData\Local\Programs\Inno Setup 6\ISCC.exe" installer/proxypilot.iss
```

Output: `dist/ProxyPilot-0.1.0-Setup.exe`

## Key Files

### Version Info Configuration

**cmd/cliproxytray/versioninfo.json**
```json
{
  "FixedFileInfo": {
    "FileVersion": {"Major": 0, "Minor": 1, "Patch": 0, "Build": 0},
    "ProductVersion": {"Major": 0, "Minor": 1, "Patch": 0, "Build": 0}
  },
  "StringFileInfo": {
    "FileDescription": "ProxyPilot System Tray",
    "ProductName": "ProxyPilot",
    "ProductVersion": "0.1.0"
  },
  "IconPath": "../../static/icon.ico"
}
```

### Tray Process Manager

**cmd/cliproxytray/engine.go** - Manages the CLI process:
- Spawns/stops `proxypilot.exe` subprocess
- Monitors process health
- Passes configuration and management password

### Tray Icon Embedding

**internal/trayicon/ico.go**
```go
package trayicon

import (
    _ "embed"
)

//go:embed icon.ico
var proxyPilotICO []byte

func ProxyPilotICO() []byte {
    return proxyPilotICO
}
```

### Installer Script

**installer/proxypilot.iss** - Inno Setup script that:
- Installs `proxypilot.exe` and `ProxyPilot.exe` to `%LOCALAPPDATA%\ProxyPilot`
- Creates Start Menu shortcut
- Adds optional Windows startup entry
- Copies config.example.yaml and creates config.yaml on first install
- Uses ProxyPilot icon for installer and shortcuts

## System Tray Menu

Simplified flat menu structure:

```
┌─────────────────────┐
│ Open Dashboard      │  → Opens browser to proxypilot.html
├─────────────────────┤
│ Start/Stop          │  → Toggle proxy engine (CLI process)
│ Copy API URL        │  → Copies http://127.0.0.1:8317/v1
├─────────────────────┤
│ Quit                │  → Stop engine and exit
└─────────────────────┘
```

## Release Process

### Create Release on GitHub

```bash
# Tag the release
git tag v0.1.0
git push origin v0.1.0

# Create release with assets
gh release create v0.1.0 \
  --title "ProxyPilot v0.1.0" \
  --notes "Release notes here" \
  dist/ProxyPilot-0.1.0-Setup.exe \
  dist/ProxyPilot.exe \
  config.example.yaml
```

### Update Existing Release

```bash
# Delete old asset
gh release delete-asset v0.1.0 ProxyPilot.exe --yes

# Upload new asset
gh release upload v0.1.0 dist/ProxyPilot.exe
```

## Troubleshooting

### Icon Not Showing in File Explorer

1. Ensure icon.ico has 256x256 size (required for Windows)
2. Clear Windows icon cache: `ie4uinit.exe -show`
3. Verify resource.syso was generated after icon update
4. Rebuild the executable

### Dashboard Not Loading in Browser

1. Ensure the CLI process is running (check tray status)
2. Verify the port is correct: `http://127.0.0.1:8317/proxypilot.html`
3. Check for port conflicts with another application

### Tray Icon Shows Wrong Image

1. Check internal/trayicon/icon.ico is the correct file
2. Rebuild the tray app to re-embed the icon
3. Restart the application

### Engine Not Starting

1. Check if port 8317/8318 are already in use
2. Review logs in `%LOCALAPPDATA%\ProxyPilot\logs\`
3. Ensure config.yaml is valid YAML

## File Structure

```
ProxyPilot/
├── cmd/
│   ├── server/
│   │   └── main.go              # CLI entry point
│   └── cliproxytray/
│       ├── main_windows.go      # Tray app entry point
│       ├── engine.go            # CLI process manager
│       ├── versioninfo.json     # Version/icon config
│       └── resource.syso        # Compiled resources
├── internal/
│   └── trayicon/
│       ├── ico.go               # Embeds icon.ico
│       └── icon.ico             # ProxyPilot logo (multi-res)
├── static/
│   ├── icon.ico                 # Source icon (7 sizes)
│   └── icon.png                 # Original PNG logo
├── webui/
│   ├── src/                     # React dashboard source
│   └── dist/                    # Built assets (served by CLI)
├── installer/
│   └── proxypilot.iss           # Inno Setup script
└── dist/                        # Distribution files
    ├── proxypilot.exe           # CLI executable
    └── ProxyPilot.exe           # Tray app
```

## Version History

- **v0.3.0** (2025-12-30) - CLI + Tray architecture
  - Removed WebView2 dependency
  - Dashboard served by CLI, opened in browser
  - Tray app manages CLI subprocess
  - Simplified deployment and debugging

- **v0.2.0** (2025-12-27) - Single-binary architecture
  - Embedded proxy engine runs in-process
  - Removed separate proxypilot-engine.exe
  - Reduced total size from ~46MB to ~29MB
  - Faster startup with no process spawning

- **v0.1.0** (2025-12-27) - Initial release
  - System tray with minimal menu
  - Multi-resolution icon support
  - Single-file Windows installer
