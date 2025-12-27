# ProxyPilot Desktop App Build Process

Documentation of the desktop application build and release process established on December 27, 2025.

## Overview

ProxyPilot Desktop is a **single executable** that includes both the system tray UI and the proxy engine:
- **ProxyPilot.exe** - System tray app with embedded WebView2 dashboard and embedded proxy engine

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       ProxyPilot.exe                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ System Tray │  │  WebView2   │  │   Embedded Assets       │  │
│  │   (systray) │  │  Dashboard  │  │     (webui/dist)        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              Embedded Proxy Engine (goroutine)              ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  ││
│  │  │Thinking Proxy│  │   Routing   │  │   Provider Auth     │  ││
│  │  │  (port 8317) │  │   Engine    │  │    Management       │  ││
│  │  └─────────────┘  └─────────────┘  └─────────────────────┘  ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### In-Process Engine

The proxy engine runs as a goroutine within the same process, managed by `EmbeddedEngine`:

```go
// cmd/cliproxytray/engine.go
type EmbeddedEngine struct {
    mu        sync.Mutex
    service   *cliproxy.Service
    cancel    context.CancelFunc
    running   bool
    port      int
    // ...
}

func (e *EmbeddedEngine) Start(cfg *config.Config, configPath, password string) error
func (e *EmbeddedEngine) Stop() error
func (e *EmbeddedEngine) Restart(cfg *config.Config, configPath, password string) error
func (e *EmbeddedEngine) Status() Status
```

Benefits of single-binary architecture:
- **Simpler deployment**: One file to distribute and install
- **Smaller total size**: ~29MB vs ~46MB (shared Go runtime)
- **Faster startup**: No process spawning overhead
- **Easier debugging**: Single process to monitor

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

This generates `webui/dist/` with the dashboard assets.

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

### Step 4: Build Executable

```bash
# Build single-binary tray app (with -H windowsgui to hide console)
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
    "FileDescription": "ProxyPilot Desktop App",
    "ProductName": "ProxyPilot",
    "ProductVersion": "0.1.0"
  },
  "IconPath": "../../static/icon.ico"
}
```

### Embedded Engine

**cmd/cliproxytray/engine.go** - Manages the in-process proxy service:
- Uses `cliproxy.NewBuilder()` SDK to build the service
- Runs in a goroutine with cancellable context
- Thread-safe start/stop/restart operations

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
- Installs ProxyPilot.exe to `%LOCALAPPDATA%\ProxyPilot`
- Creates Start Menu and Desktop shortcuts
- Adds optional Windows startup entry
- Copies config.example.yaml and creates config.yaml on first install
- Uses ProxyPilot icon for installer and shortcuts

## System Tray Menu

Simplified flat menu structure:

```
┌─────────────────────┐
│ Open Dashboard      │  → Opens WebView2 window
├─────────────────────┤
│ Start/Stop          │  → Toggle proxy engine
│ Copy API URL        │  → Copies http://127.0.0.1:8317/v1
├─────────────────────┤
│ Quit                │  → Exit application
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

### WebView2 Dashboard Not Loading

1. Ensure WebView2 runtime is installed on the system
2. Check that webui/dist/ assets are embedded (go:embed directive)
3. Verify the asset server is starting on a free port

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
│   └── cliproxytray/
│       ├── main_windows.go      # Tray app with embedded engine
│       ├── engine.go            # EmbeddedEngine implementation
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
│   └── dist/                    # Built assets (embedded)
├── installer/
│   └── proxypilot.iss           # Inno Setup script
└── dist/                        # Distribution files
    └── ProxyPilot.exe           # Single-binary (~29MB)
```

## Version History

- **v0.2.0** (2025-12-27) - Single-binary architecture
  - Embedded proxy engine runs in-process
  - Removed separate proxypilot-engine.exe
  - Reduced total size from ~46MB to ~29MB
  - Faster startup with no process spawning

- **v0.1.0** (2025-12-27) - Initial desktop app release
  - System tray with minimal menu
  - Embedded WebView2 dashboard
  - Multi-resolution icon support
  - Single-file Windows installer
